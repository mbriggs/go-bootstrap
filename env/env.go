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

var ErrBadAppEnv = errors.New("APP_ENV must be development, test, or production")

type Env struct {
	AppEnv    AppEnv
	Port      string // dev-server port; worktree.env overrides it per worktree
	PublicDir string // static asset root served at /public
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

	return Env{
		AppEnv:    appEnv,
		Port:      getenv("PORT", "8080"),
		PublicDir: getenv("PUBLIC_DIR", "public"),
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
