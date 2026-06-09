package web

import (
	"github.com/labstack/echo/v4"

	"github.com/mbriggs/gesso/gallery"
)

// DesignShowcase renders the gesso design-system gallery — every ui
// component with its states, tokens, and responsive behavior. It is a
// development reference, hidden in production.
func DesignShowcase(c echo.Context) error {
	if prodMode {
		return echo.ErrNotFound
	}

	page := gallery.PageData{Groups: gallery.Sections()}

	return RenderPage(c, PageMeta{
		Title:      "Design system",
		HeadExtras: gallery.HeadExtras(),
	}, gallery.Page(page))
}
