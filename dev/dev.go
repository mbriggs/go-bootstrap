// Package dev holds development-time conveniences that are inert in
// production. DevMode flips the switch (webtest does this for test runs);
// helpers like PP check it themselves, so call sites never branch on
// environment.
package dev

import (
	"encoding/json"
	"fmt"

	"github.com/mbriggs/go-bootstrap/logging"
)

var (
	logger  = logging.Logger("dev")
	devMode = false
)

// DevMode enables the package's helpers and forces colored log output.
func DevMode() {
	devMode = true
	logging.ForceColor()
}

// PP pretty-prints val as indented JSON for log lines; outside dev mode it
// returns val untouched.
func PP(val any) any {
	if !devMode {
		return val
	}

	prettyJSON, err := json.MarshalIndent(val, "", "    ")
	if err != nil {
		logger.Error("Failed to generate json", "error", err)
		panic(fmt.Errorf("from dev.PP: %w", err))
	}

	return string(prettyJSON)
}
