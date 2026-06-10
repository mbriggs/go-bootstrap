package web

import (
	"net/http"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mbriggs/go-bootstrap/env"
)

// Sessions is the process-wide scs session manager, backed by the sessions
// table. Like db.Conn it is a thread-safe single-instance resource, so it
// lives as a global; main and webtest call Configure once at boot before
// building the router.
var Sessions *scs.SessionManager

// devMode loosens the error page (full detail + copy button) in
// development only; prodMode hides development surfaces like /design.
var (
	devMode  bool
	prodMode bool
)

// Configure wires the web package to its process-wide resources: sessions
// on the given pool (cookie Secure in production) and the environment's
// debugging posture.
func Configure(pool *pgxpool.Pool, appEnv env.AppEnv) {
	devMode = appEnv == env.Development
	prodMode = appEnv == env.Production

	s := scs.New()
	s.Store = pgxstore.New(pool)
	s.Lifetime = 30 * 24 * time.Hour
	s.IdleTimeout = 7 * 24 * time.Hour
	s.Cookie.Name = "session"
	s.Cookie.HttpOnly = true
	s.Cookie.SameSite = http.SameSiteLaxMode
	s.Cookie.Secure = appEnv == env.Production
	Sessions = s
}
