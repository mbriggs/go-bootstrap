package dev

import (
	"encoding/json"
	"fmt"

	"github.com/mbriggs/go-bootstrap/logging"
)

var logger = logging.Logger("dev")
var devMode = false

func DevMode() {
	devMode = true
	logging.ForceColor()
}

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
