package web_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mbriggs/go-bootstrap/auth"
	"github.com/mbriggs/go-bootstrap/webtest"
)

func TestSigninThrottlesRepeatedFailures(t *testing.T) {
	ctx := t.Context()

	if _, err := auth.Create(ctx, auth.CreateInput{Email: "throttle@example.com", Password: "right-pw"}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	client := webtest.NewClient(t, webtest.Server(ctx))

	for i := range 5 {
		rec := client.PostForm("/signin", signinForm("throttle@example.com", fmt.Sprintf("wrong-%d", i)))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "invalid email or password") {
			t.Fatalf("attempt %d = %d, want invalid-credentials re-render", i, rec.Code)
		}
	}

	// Sixth attempt is blocked even with the correct password.
	rec := client.PostForm("/signin", signinForm("throttle@example.com", "right-pw"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "too many failed attempts") {
		t.Fatalf("throttled attempt = %d %q, want throttle message", rec.Code, rec.Body.String())
	}

	// A different email from the same address is unaffected (keyed per
	// IP+email, not per IP).
	rec = client.PostForm("/signin", signinForm("other@example.com", "whatever"))
	if !strings.Contains(rec.Body.String(), "invalid email or password") {
		t.Fatal("throttle leaked across emails")
	}
}
