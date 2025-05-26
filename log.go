package signals

import (
	"log"
	"sync"
)

// LoggerFunc defines the function signature for a logger used in the signals package.
// It should behave like log.Printf (e.g., log.Printf, fmt.Printf, or a structured logger).
type LoggerFunc func(format string, args ...any)

var (
	loggerMu sync.RWMutex
	logger   LoggerFunc
)

// SetLogger allows you to provide a custom logger function for internal diagnostics.
// This logger will be used for debug messages and internal tracing.
// Pass nil to revert to the default logger (log.Printf).
func SetLogger(logFn LoggerFunc) {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	logger = logFn
}

// logf logs a formatted message using the currently configured logger.
// If no custom logger is set, it defaults to log.Printf.
// This is used internally throughout the signals package.
func logf(format string, args ...any) {
	loggerMu.RLock()
	defer loggerMu.RUnlock()

	if logger != nil {
		logger(format, args...)
	} else {
		log.Printf(format, args...)
	}
}
