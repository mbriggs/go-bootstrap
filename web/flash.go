package web

import (
	"github.com/labstack/echo/v4"

	"github.com/mbriggs/go-bootstrap/views"
)

// SetFlash stores a one-shot message for the next render. kind is "ok" or
// "error".
func SetFlash(c echo.Context, kind, msg string) {
	SetKeyedFlash(c, kind, "", msg)
}

// SetKeyedFlash is SetFlash with a field key, so the next page can place
// the message next to the input it belongs to.
func SetKeyedFlash(c echo.Context, kind, key, msg string) {
	ctx := c.Request().Context()
	Sessions.Put(ctx, "flash_kind", kind)
	Sessions.Put(ctx, "flash_msg", msg)
	if key == "" {
		Sessions.Remove(ctx, "flash_key")
	} else {
		Sessions.Put(ctx, "flash_key", key)
	}
}

// TakeFlash pops the pending flash, or returns nil. RenderPage calls this;
// handlers rarely need it directly.
func TakeFlash(c echo.Context) *views.Flash {
	ctx := c.Request().Context()
	kind := Sessions.PopString(ctx, "flash_kind")
	msg := Sessions.PopString(ctx, "flash_msg")
	key := Sessions.PopString(ctx, "flash_key")
	if msg == "" {
		return nil
	}
	if kind == "" {
		kind = "ok"
	}

	return &views.Flash{Kind: kind, Message: msg, Key: key}
}
