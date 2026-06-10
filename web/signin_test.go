package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/mbriggs/go-bootstrap/auth"
	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/webtest"
)

func TestMain(m *testing.M) { webtest.Main(m) }

func signinForm(email, password string) url.Values {
	return url.Values{"email": {email}, "password": {password}}
}

func deleteUser(t *testing.T, id int64) {
	t.Helper()

	err := db.ExecInTx(t.Context(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(t.Context(), "DELETE FROM users WHERE id = $1", id); err != nil {
			return fmt.Errorf("deleting user %d: %w", id, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}
}

func TestSigninFlowEndToEnd(t *testing.T) {
	ctx := t.Context()

	user, err := auth.Create(ctx, auth.CreateInput{
		Email:    "flow@example.com",
		Password: "flow-pw-1",
		Name:     "Flow",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	client := webtest.NewClient(t, webtest.Server(ctx))

	// Signed out, the protected page bounces to signin.
	if rec := client.Get("/"); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/signin" {
		t.Fatalf("GET / signed out = %d -> %q, want 303 -> /signin", rec.Code, rec.Header().Get("Location"))
	}

	if rec := client.Get("/signin"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Sign in") {
		t.Fatalf("GET /signin = %d, want 200 with signin form", rec.Code)
	}

	// Bad password re-renders the form with a flash and keeps the email.
	rec := client.PostForm("/signin", signinForm("flow@example.com", "wrong"))
	if rec.Code != http.StatusOK {
		t.Fatalf("bad password = %d, want 200 re-render", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid email or password") {
		t.Fatal("bad password response missing flash message")
	}
	if !strings.Contains(rec.Body.String(), "flow@example.com") {
		t.Fatal("bad password response should keep the typed email")
	}

	// Good credentials redirect back to the page that demanded signin.
	rec = client.PostForm("/signin", signinForm("flow@example.com", "flow-pw-1"))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("signin = %d -> %q, want 303 -> /", rec.Code, rec.Header().Get("Location"))
	}

	rec = client.Get("/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), user.Name) {
		t.Fatalf("GET / signed in = %d, want 200 greeting %q", rec.Code, user.Name)
	}

	// Signout destroys the session.
	if rec = client.PostForm("/signout", nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("signout = %d, want 303", rec.Code)
	}
	if rec = client.Get("/"); rec.Code != http.StatusSeeOther {
		t.Fatalf("GET / after signout = %d, want 303 to signin", rec.Code)
	}
}

func TestSigninRedirectsBackToOriginalPath(t *testing.T) {
	ctx := t.Context()

	user, err := auth.Make(ctx)
	if err != nil {
		t.Fatalf("make user: %v", err)
	}

	client := webtest.NewClient(t, webtest.Server(ctx))

	// Health is public but / is protected; ask for / first so after_signin
	// records it, then signin and land back on it.
	if rec := client.Get("/"); rec.Code != http.StatusSeeOther {
		t.Fatalf("GET / = %d, want 303", rec.Code)
	}
	rec := client.PostForm("/signin", signinForm(user.Email, auth.MakePassword))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("signin = %d -> %q, want 303 -> /", rec.Code, rec.Header().Get("Location"))
	}
}

func TestCrossOriginPostIsForbidden(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))

	rec := client.Do(http.MethodPost, "/signin",
		signinForm("any@example.com", "pw"),
		map[string]string{"Sec-Fetch-Site": "cross-site"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site POST = %d, want 403", rec.Code)
	}
}

func TestVanishedSessionUserIsClearedNotFatal(t *testing.T) {
	ctx := t.Context()

	user, err := auth.Create(ctx, auth.CreateInput{Email: "vanish@example.com", Password: "vanish-pw"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	client := webtest.NewClient(t, webtest.Server(ctx))
	if rec := client.PostForm("/signin", signinForm("vanish@example.com", "vanish-pw")); rec.Code != http.StatusSeeOther {
		t.Fatalf("signin = %d, want 303", rec.Code)
	}

	deleteUser(t, user.ID)

	// The stale session id should clear and the request continue as
	// signed-out, not 500.
	if rec := client.Get("/"); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/signin" {
		t.Fatalf("GET / with vanished user = %d -> %q, want 303 -> /signin", rec.Code, rec.Header().Get("Location"))
	}
}
