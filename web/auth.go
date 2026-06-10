package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/mbriggs/go-bootstrap/auth"
	"github.com/mbriggs/go-bootstrap/db"
)

type ctxUserKey struct{}

// CurrentUser returns the user attached by LoadUser, or nil.
func CurrentUser(c *echo.Context) *auth.User {
	u, _ := c.Request().Context().Value(ctxUserKey{}).(*auth.User)
	return u
}

// LoadUser hydrates the user from the session-bound user id on every
// request. Cheap one-row read; no caching beyond the request scope.
func LoadUser(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		ctx := c.Request().Context()

		id := Sessions.GetInt64(ctx, "user_id")
		if id == 0 {
			return next(c)
		}

		user, err := auth.ByID(ctx, id)
		if errors.Is(err, db.ErrNotFound) {
			// Session points at a vanished user; clear and continue.
			Sessions.Remove(ctx, "user_id")
			return next(c)
		}
		if err != nil {
			return fmt.Errorf("loading session user %d: %w", id, err)
		}

		c.SetRequest(c.Request().WithContext(context.WithValue(ctx, ctxUserKey{}, &user)))

		return next(c)
	}
}

// RequireUser enforces sign-in. Stores the original path so signin can
// bounce the user back after success.
func RequireUser(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if CurrentUser(c) == nil {
			Sessions.Put(c.Request().Context(), "after_signin", c.Request().RequestURI)
			return SafeRedirect(c, "/signin")
		}

		return next(c)
	}
}

// RequirePolicy gates a group on a predicate over the signed-in user.
// Policies live next to the domain they protect and return an error
// explaining the denial (logged, never shown to the client).
func RequirePolicy(policy func(auth.User) error) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return RequireUser(func(c *echo.Context) error {
			if err := policy(*CurrentUser(c)); err != nil {
				return echo.NewHTTPError(http.StatusForbidden, "forbidden").Wrap(err)
			}

			return next(c)
		})
	}
}

var errMissingRole = errors.New("missing role")

// RequireRole gates a group on a role, e.g. web.RequireRole(auth.RoleAdmin).
func RequireRole(role string) echo.MiddlewareFunc {
	return RequirePolicy(func(u auth.User) error {
		if !u.HasRole(role) {
			return fmt.Errorf("%w: %s", errMissingRole, role)
		}

		return nil
	})
}
