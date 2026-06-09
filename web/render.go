package web

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"

	"github.com/mbriggs/go-bootstrap/views"
)

// PageMeta is the per-page input to RenderPage.
type PageMeta struct {
	Title      string
	HeadExtras templ.Component
}

// RenderPage renders a full page inside the layout. User and Flash are
// attached automatically.
func RenderPage(c echo.Context, meta PageMeta, body templ.Component) error {
	return RenderPageData(c, meta, func(views.LayoutData) templ.Component { return body })
}

// RenderPageData is RenderPage for components that need the request-scoped
// layout data themselves. It renders into a buffer first so a mid-stream
// component failure produces a 500, not a partial 200.
func RenderPageData(c echo.Context, meta PageMeta, body func(views.LayoutData) templ.Component) error {
	data := views.LayoutData{
		Title:      meta.Title,
		User:       CurrentUser(c),
		Flash:      TakeFlash(c),
		HeadExtras: meta.HeadExtras,
	}
	if data.Title == "" {
		data.Title = "app"
	}

	var buf bytes.Buffer
	if err := views.Layout(data, body(data)).Render(c.Request().Context(), &buf); err != nil {
		return fmt.Errorf("rendering %s: %w", data.Title, err)
	}

	if err := c.HTMLBlob(http.StatusOK, buf.Bytes()); err != nil {
		return fmt.Errorf("writing %s: %w", data.Title, err)
	}

	return nil
}
