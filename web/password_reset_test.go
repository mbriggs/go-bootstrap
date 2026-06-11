package web_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/mbriggs/go-bootstrap/auth"
	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/webtest"
)

func resetRequestForm(email string) url.Values {
	return url.Values{"email": {email}}
}

type resetEmailJob struct {
	Email string `json:"email"`
}

// enqueuedResetEmails returns the password_reset_email jobs enqueued for
// email, newest last. Workers never run in webtest, so rows stay put.
func enqueuedResetEmails(t *testing.T, email string) []resetEmailJob {
	t.Helper()

	type jobRow struct {
		Args []byte `db:"args"`
	}

	rows, err := db.FindAll[jobRow](t.Context(),
		"SELECT args FROM river_job WHERE kind = 'password_reset_email' ORDER BY id")
	if err != nil {
		t.Fatalf("reading enqueued jobs: %v", err)
	}

	var matched []resetEmailJob
	for _, row := range rows {
		var job resetEmailJob
		if err := json.Unmarshal(row.Args, &job); err != nil {
			t.Fatalf("decoding job args: %v", err)
		}
		if job.Email == email {
			matched = append(matched, job)
		}
	}

	return matched
}

func TestPasswordResetFlowEndToEnd(t *testing.T) {
	ctx := t.Context()

	user, err := auth.Make(ctx)
	if err != nil {
		t.Fatalf("make user: %v", err)
	}

	client := webtest.NewClient(t, webtest.Server(ctx))

	if rec := client.Get("/password-reset"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Reset password") {
		t.Fatalf("GET /password-reset = %d, want 200 with request form", rec.Code)
	}

	// Requesting a reset enqueues the email job and bounces to signin
	// without confirming the account exists. The args carry no token — the
	// worker mints it at send time — so the rest of the flow drives off a
	// token from the auth API, the same call the worker makes.
	rec := client.PostForm("/password-reset", resetRequestForm(user.Email))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/signin" {
		t.Fatalf("reset request = %d -> %q, want 303 -> /signin", rec.Code, rec.Header().Get("Location"))
	}

	if enqueued := enqueuedResetEmails(t, user.Email); len(enqueued) != 1 {
		t.Fatalf("enqueued jobs = %+v, want exactly one", enqueued)
	}

	token, _, err := auth.CreatePasswordReset(ctx, user.Email)
	if err != nil {
		t.Fatalf("create reset token: %v", err)
	}

	// The emailed link shows the new-password form.
	confirmPath := "/password-reset/confirm?token=" + token
	if rec := client.Get(confirmPath); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "new password") {
		t.Fatalf("GET %s = %d, want 200 with confirm form", confirmPath, rec.Code)
	}

	rec = client.PostForm("/password-reset/confirm",
		url.Values{"token": {token}, "password": {"brand-new-pw"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/signin" {
		t.Fatalf("confirm = %d -> %q, want 303 -> /signin", rec.Code, rec.Header().Get("Location"))
	}

	// Old password is dead, new one signs in.
	if rec := client.PostForm("/signin", signinForm(user.Email, auth.MakePassword)); rec.Code != http.StatusOK {
		t.Fatalf("signin with old password = %d, want 200 re-render", rec.Code)
	}
	if rec := client.PostForm("/signin", signinForm(user.Email, "brand-new-pw")); rec.Code != http.StatusSeeOther {
		t.Fatalf("signin with new password = %d, want 303", rec.Code)
	}

	// The token is single-use: the link is dead after a successful reset.
	if rec := client.Get(confirmPath); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/password-reset" {
		t.Fatalf("GET %s after use = %d -> %q, want 303 -> /password-reset", confirmPath, rec.Code, rec.Header().Get("Location"))
	}
}

func TestPasswordResetUnknownEmailLooksIdentical(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))

	// Same 303 -> /signin as a real account, and nothing enqueued.
	rec := client.PostForm("/password-reset", resetRequestForm("nobody-reset@example.com"))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/signin" {
		t.Fatalf("unknown email = %d -> %q, want 303 -> /signin", rec.Code, rec.Header().Get("Location"))
	}

	if enqueued := enqueuedResetEmails(t, "nobody-reset@example.com"); len(enqueued) != 0 {
		t.Fatalf("unknown email enqueued %d jobs, want none", len(enqueued))
	}
}

func TestPasswordResetConfirmRejectsShortPassword(t *testing.T) {
	ctx := t.Context()

	user, err := auth.Make(ctx)
	if err != nil {
		t.Fatalf("make user: %v", err)
	}
	token, _, err := auth.CreatePasswordReset(ctx, user.Email)
	if err != nil {
		t.Fatalf("create reset token: %v", err)
	}

	client := webtest.NewClient(t, webtest.Server(ctx))
	rec := client.PostForm("/password-reset/confirm",
		url.Values{"token": {token}, "password": {"seven77"}})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "at least 8 characters") {
		t.Fatalf("short password = %d, want 200 re-render with policy message", rec.Code)
	}

	// The rejected attempt must not burn the token.
	if err := auth.CheckResetToken(ctx, token); err != nil {
		t.Fatalf("token after rejected reset = %v, want still live", err)
	}
}

func TestPasswordResetSignsOutLiveSessions(t *testing.T) {
	ctx := t.Context()

	user, err := auth.Make(ctx)
	if err != nil {
		t.Fatalf("make user: %v", err)
	}

	// The attacker scenario: a session signed in before the reset (a stolen
	// cookie behaves the same as this client).
	stale := webtest.NewClient(t, webtest.Server(ctx))
	if rec := stale.PostForm("/signin", signinForm(user.Email, auth.MakePassword)); rec.Code != http.StatusSeeOther {
		t.Fatalf("signin = %d, want 303", rec.Code)
	}
	if rec := stale.Get("/"); rec.Code != http.StatusOK {
		t.Fatalf("GET / signed in = %d, want 200", rec.Code)
	}

	token, _, err := auth.CreatePasswordReset(ctx, user.Email)
	if err != nil {
		t.Fatalf("create reset token: %v", err)
	}
	fresh := webtest.NewClient(t, webtest.Server(ctx))
	rec := fresh.PostForm("/password-reset/confirm",
		url.Values{"token": {token}, "password": {"post-reset-pw"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("confirm = %d, want 303", rec.Code)
	}

	// The pre-reset session is dead: its password epoch no longer matches.
	if rec := stale.Get("/"); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/signin" {
		t.Fatalf("GET / with pre-reset session = %d -> %q, want 303 -> /signin", rec.Code, rec.Header().Get("Location"))
	}

	// Signing in again with the new password works as normal.
	if rec := stale.PostForm("/signin", signinForm(user.Email, "post-reset-pw")); rec.Code != http.StatusSeeOther {
		t.Fatalf("signin after reset = %d, want 303", rec.Code)
	}
	if rec := stale.Get("/"); rec.Code != http.StatusOK {
		t.Fatalf("GET / after re-signin = %d, want 200", rec.Code)
	}
}

func TestPasswordResetBadTokenBouncesToRequestForm(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))

	rec := client.Get("/password-reset/confirm?token=not-a-real-token")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/password-reset" {
		t.Fatalf("bad token = %d -> %q, want 303 -> /password-reset", rec.Code, rec.Header().Get("Location"))
	}
}
