package logging

import (
	"fmt"
	"log/slog"

	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
)

var loggers *Manager

// Initialize the loggers manager, ensure that it is initialized before any other logging functions are called.
// init is a special function that is called when the package is loaded.
func init() {
	var err error
	loggers, err = NewManager("_all", "INFO")
	if err != nil {
		panic(fmt.Sprintf("error initializing logging: %v", err))
	}
}

// Configure the loggers with the provided settings and default level.
// The settings string is a comma-separated list of tags and levels in the following format:
//
//	"MyClass:debug,MyOtherClass,-DisabledClass,_all"
//
// In this example:
// - MyClass logger will be set to debug
// - MyOtherClass logger will be set to whatever Rails.logger is set to
// - DisabledClass will have its logger disabled. This happens via the - prefix
// - all other loggers will be enabled. _all is a magic logger name to allow for this
// - without specifying _all, all unmentioned loggers will be disabled by default
//
// The defaultLevel is the level that will be used for any loggers that do not have a level specified.
// Levels are case-insensitive and can be one of the following:
// - DEBUG
// - INFO
// - WARN
// - ERROR
func Configure(settings string, defaultLevel string) error {
	err := loggers.Configure(settings, defaultLevel)
	return err
}

// Logger returns a logger with the provided name.
func Logger(name string) *slog.Logger {
	return loggers.Logger(name)
}

func ForceColor() {
	log.SetColorProfile(termenv.TrueColor)
}
