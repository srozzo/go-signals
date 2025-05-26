package signals

import (
	"os"
)

// Config controls how the signal dispatcher behaves.
// It allows setting whether the context should be canceled when an unhandled signal is received,
// and optionally specifies a default handler to invoke when no specific handler is registered.
type Config struct {
	// CancelOnUnhandled indicates whether to cancel the context when a signal is received
	// that does not have a specific handler registered.
	CancelOnUnhandled bool

	// DefaultHandler is invoked if a signal is received without a registered handler.
	// If nil, the signal is ignored unless CancelOnUnhandled is true.
	DefaultHandler func(signal os.Signal)
}
