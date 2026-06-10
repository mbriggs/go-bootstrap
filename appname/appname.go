// Package appname derives process identity from the module path, so a
// project bootstrapped via gonew renames everything — databases, telemetry
// service name, Inngest app id — by renaming the module. Callers choose
// their own fallback when build info is unavailable: webtest fails loud
// (wrong database names corrupt test isolation), telemetry and flows fall
// back to a generic name.
package appname

import (
	"path"
	"runtime/debug"
	"strings"
)

// Base returns the module path's final element ("go-bootstrap"), or ""
// when build info is unavailable.
func Base() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Path == "" {
		return ""
	}

	return path.Base(info.Main.Path)
}

// Postgres returns Base lowercased with every non-identifier rune
// flattened to underscore ("go_bootstrap"), or "".
func Postgres() string {
	var b strings.Builder

	for _, r := range strings.ToLower(Base()) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}

	return b.String()
}
