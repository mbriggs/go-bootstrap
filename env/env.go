// Package env is the single trust boundary for process configuration: read
// once at startup, validated, then passed as values. Nothing else reads
// os.Getenv for app-level settings. (PG* connection variables are the
// exception — pgx consumes those directly.)
package env

import (
	"errors"
	"fmt"
	"os"
)

type AppEnv string

const (
	Development AppEnv = "development"
	Test        AppEnv = "test"
	Production  AppEnv = "production"
)

var (
	ErrBadAppEnv = errors.New("APP_ENV must be development, test, or production")

	// ErrAppURLRequired: the localhost fallback would silently put dead
	// links in production email, so production must say its origin.
	ErrAppURLRequired = errors.New("APP_URL is required in production (emailed links use it as their origin)")

	// ErrMailFromRequired: without a sender the log Outbox stays in place
	// and production email silently goes nowhere but the logs.
	ErrMailFromRequired = errors.New("MAIL_FROM is required in production (the SES sender address)")
)

type Env struct {
	AppEnv    AppEnv
	Port      string // dev-server port; worktree.env overrides it per worktree
	PublicDir string // static asset root served at /public
	BaseURL   string // externally reachable origin, used in emailed links
	MailFrom  string // SES sender address; empty leaves the log Outbox in place
}

func (e Env) Dev() bool        { return e.AppEnv == Development }
func (e Env) Production() bool { return e.AppEnv == Production }

// Load reads and validates process configuration. Call it once, at the top
// of main.
func Load() (Env, error) {
	appEnv := AppEnv(getenv("APP_ENV", string(Development)))
	switch appEnv {
	case Development, Test, Production:
	default:
		return Env{}, fmt.Errorf("%w: got %q", ErrBadAppEnv, appEnv)
	}

	port := getenv("PORT", "8080")

	baseURL := os.Getenv("APP_URL")
	if baseURL == "" {
		if appEnv == Production {
			return Env{}, ErrAppURLRequired
		}
		baseURL = "http://localhost:" + port
	}

	mailFrom := os.Getenv("MAIL_FROM")
	if mailFrom == "" && appEnv == Production {
		return Env{}, ErrMailFromRequired
	}

	return Env{
		AppEnv:    appEnv,
		Port:      port,
		PublicDir: getenv("PUBLIC_DIR", "public"),
		BaseURL:   baseURL,
		MailFrom:  mailFrom,
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
