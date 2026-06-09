// Package views owns the templ components, render DTOs, and layout data for
// server-rendered pages. Views render already-shaped data — no DB calls,
// sessions, or domain commands in here.
package views

import (
	"github.com/a-h/templ"

	"github.com/mbriggs/gesso/ui"
	"github.com/mbriggs/go-bootstrap/auth"
)

// Flash is a one-shot message popped from the session by the next render.
// Kind is "ok" or "error". Key optionally scopes an error to a form field
// so pages can place it next to the input.
type Flash struct {
	Kind    string
	Message string
	Key     string
}

// LayoutData is the request-scoped data every page render receives.
type LayoutData struct {
	Title      string
	User       *auth.User
	Flash      *Flash
	HeadExtras templ.Component
}

func flashTone(kind string) ui.Tone {
	if kind == "error" {
		return ui.ToneDanger
	}

	return ui.ToneSuccess
}

// ErrorPageData feeds the error page. Detail is set in development only.
type ErrorPageData struct {
	Status     int
	StatusText string
	Detail     string
}
