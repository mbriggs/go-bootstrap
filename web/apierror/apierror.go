// Package apierror renders flat {"message","status"} JSON error responses
// that clients can parse and branch on. Internal detail never crosses the
// boundary — it goes to the logger instead.
package apierror

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/mbriggs/go-bootstrap/logging"
)

var logger = logging.Logger("apierror")

type Response struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// JSON renders an Algolia-shaped error with the given status and message.
func JSON(c *echo.Context, status int, message string) error {
	if err := c.JSON(status, Response{Message: message, Status: status}); err != nil {
		return fmt.Errorf("rendering api error: %w", err)
	}

	return nil
}

// Internal logs err with request context and renders a generic 500,
// keeping the internal detail out of the response body.
func Internal(c *echo.Context, err error) error {
	logger.Error(
		"internal error",
		"method", c.Request().Method,
		"uri", c.Request().RequestURI,
		"error", err,
	)

	return JSON(c, http.StatusInternalServerError, "Internal Server Error")
}
