package signals

import (
    "context"
    "os"
)

// Register adds a handler for the given signals to the Default registry.
func Register(h Handler, sigs ...os.Signal) (tokens []Token) {
    return Default.Register(h, sigs...)
}

// Unregister removes a handler by token from the Default registry.
func Unregister(t Token) { Default.Unregister(t) }

// Reset clears all handlers and internal state on the Default registry.
func Reset() { Default.Reset() }

// Start begins signal processing on the Default registry.
func Start() error { return Default.Start() }

// Stop ends processing and cancels handler contexts on the Default registry.
func Stop() { Default.Stop() }

// ContextFor returns a context canceled when any of sigs is observed.
func ContextFor(parent context.Context, sigs ...os.Signal) (context.Context, context.CancelFunc) {
    return Default.ContextFor(parent, sigs...)
}

// StartWithContext is a convenience wrapper over Default.StartWithContext.
func StartWithContext(parent context.Context) error { return Default.StartWithContext(parent) }

// SetLogger sets the logger for the Default registry. Safe for concurrent use; the
// value is snapshotted at the start of each dispatch cycle.
func SetLogger(l LoggerFunc) {
    Default.mu.Lock()
    Default.logf = l
    Default.mu.Unlock()
}

// SetPolicy sets the policy for the Default registry. Safe for concurrent use; the
// value is snapshotted at the start of each dispatch cycle.
func SetPolicy(p Policy) {
    Default.mu.Lock()
    Default.policy = p
    Default.mu.Unlock()
}

// SetDebug toggles debug logging for the Default registry. Safe for concurrent use; the
// value is snapshotted at the start of each dispatch cycle.
func SetDebug(enabled bool) {
    Default.mu.Lock()
    Default.debug = enabled
    Default.mu.Unlock()
}
