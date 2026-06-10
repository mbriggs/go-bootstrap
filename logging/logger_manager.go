package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/charmbracelet/log"
)

// Manager is a logging Manager that holds the configuration and loggers.
type Manager struct {
	sync.RWMutex
	config  LoggingConfig
	loggers map[string]*slog.Logger
	// handlers keeps the charm handlers addressable for reconfiguration;
	// the slog.Logger's own handler is wrapped in traceHandler.
	handlers map[string]*log.Logger
}

func NewManager(settings string, defaultLevel string) (*Manager, error) {
	m := &Manager{
		loggers:  make(map[string]*slog.Logger),
		handlers: make(map[string]*log.Logger),
	}

	err := m.Configure(settings, defaultLevel)

	return m, err
}

func (lm *Manager) Configure(settings string, defaultLevel string) error {
	lm.Lock()
	defer lm.Unlock()

	config, err := NewConfig(settings, defaultLevel)
	if err != nil {
		return err
	}

	lm.config = config

	// Iterate over any existing loggers, and set the output and level.
	for name, handler := range lm.handlers {
		handler.SetOutput(lm.device(name))
		handler.SetLevel(lm.getLevel(name))
	}

	return err
}

func (lm *Manager) Logger(name string) *slog.Logger {
	lm.RLock()
	defer lm.RUnlock()

	logger, exists := lm.loggers[name]

	if exists {
		return logger
	}

	device := lm.device(name)

	// Even though we are using slog, we are using charmbracelet/log for the handler.
	// The handler is the part of the logger that actually writes the log messages.
	handler := log.NewWithOptions(device, log.Options{
		Prefix: fmt.Sprintf("[%s] ", name),
	})
	handler.SetLevel(lm.getLevel(name))

	// The trace wrapper stamps trace_id/span_id from ctx-carrying calls.
	logger = slog.New(traceHandler{handler})

	lm.loggers[name] = logger
	lm.handlers[name] = handler

	return logger
}

func (lm *Manager) device(name string) io.Writer {
	if lm.config.IsEnabled(name) {
		return os.Stderr
	}

	return io.Discard
}

func (lm *Manager) getLevel(name string) log.Level {
	return lm.config.GetLevel(name)
}
