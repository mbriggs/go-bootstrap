package logging

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
)

var ErrUnknownLevel = errors.New("unknown log level")

// LoggingConfig holds the configuration for the loggers.
type LoggingConfig struct {
	value        string
	defaultLevel log.Level
	includeAll   bool
	include      map[string]*log.Level
	level        map[string]*log.Level
	exclude      map[string]*log.Level
}

func NewConfig(config string, defaultLevel string) (LoggingConfig, error) {
	if defaultLevel == "" {
		defaultLevel = "INFO"
	}

	dl, err := parseLogLevel(defaultLevel)
	if err != nil {
		return LoggingConfig{}, err
	}

	c := LoggingConfig{
		value:        config,
		defaultLevel: dl,
		includeAll:   false,
		include:      make(map[string]*log.Level),
		exclude:      make(map[string]*log.Level),
		level:        make(map[string]*log.Level),
	}

	// Split the config string by comma.
	// "MyTag:debug,MyOtherTag,-MyExcludedTag" => ["MyTag:debug", "MyOtherTag", "-MyExcludedTag"]
	for tag := range strings.SplitSeq(config, ",") {
		tag = strings.TrimSpace(tag)

		// Split the tag by colon.
		// "MyTag:debug" => ["MyTag", "debug"]
		parts := strings.SplitN(tag, ":", 2)

		tagName := parts[0]

		// If the tag is "_all", include all logs.
		includeAll := tagName == "_all"

		// If a level is provided, parse it.
		var level *log.Level
		if len(parts) == 2 {
			l, err := parseLogLevel(parts[1])
			level = &l
			if err != nil {
				return c, fmt.Errorf("error parsing log level for tag (%s): %w", tag, err)
			}
			// If a level is provided for _all, set the default level.
			if includeAll {
				c.defaultLevel = l
			}
		}

		// If the tag is "_all", set the includeAll flag, and continue to the next tag.
		if includeAll {
			c.includeAll = true
			continue
		}

		// If the tag starts with a dash, it's an exclusion.
		if strings.HasPrefix(tagName, "-") {
			c.exclude[tagName[1:]] = level
			continue
		}

		// If the tag is not excluded, include it.
		c.include[tagName] = level
	}

	return c, nil
}

func (lc *LoggingConfig) IsIncluded(tag string) bool {
	_, exists := lc.include[tag]
	return exists || lc.includeAll
}

func (lc *LoggingConfig) IsExcluded(tag string) bool {
	_, exists := lc.exclude[tag]
	return exists
}

func (lc *LoggingConfig) IsEnabled(tag string) bool {
	return lc.IsIncluded(tag) && !lc.IsExcluded(tag)
}

func (lc *LoggingConfig) GetLevel(tag string) log.Level {
	if level, exists := lc.level[tag]; exists && level != nil {
		return *level
	}

	return lc.defaultLevel
}

// parseLogLevel converts a string to a slog.Level.
func parseLogLevel(level string) (log.Level, error) {
	level = strings.ToUpper(level)
	switch level {
	case "DEBUG":
		return log.DebugLevel, nil
	case "INFO":
		return log.InfoLevel, nil
	case "WARN":
		return log.WarnLevel, nil
	case "ERROR":
		return log.ErrorLevel, nil
	default:
		return log.InfoLevel, fmt.Errorf("%w: %s", ErrUnknownLevel, level)
	}
}
