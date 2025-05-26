package signals

import (
	"os"
	"sync"
)

// Config defines optional global behavior for the signal dispatcher.
type Config struct {
	// CancelOnUnhandled indicates whether a context should be cancelled
	// when a signal with no registered handler is received.
	CancelOnUnhandled bool

	// DefaultHandler is called when an unregistered signal is received.
	// If nil, the signal is ignored unless CancelOnUnhandled is true.
	DefaultHandler func(signal os.Signal)
}

var (
	configMu sync.RWMutex
	config   *Config
)

// SetConfig replaces the global signal dispatcher configuration.
// It is safe to call concurrently. Use nil to clear configuration.
func SetConfig(cfg *Config) {
	configMu.Lock()
	defer configMu.Unlock()
	config = cfg
}

// getConfig returns the current configuration. If none is set, it returns
// a zero-value Config. This function never returns nil.
func getConfig() *Config {
	configMu.RLock()
	defer configMu.RUnlock()
	if config != nil {
		return config
	}
	return &Config{}
}
