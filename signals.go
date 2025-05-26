// Package signals provides a thread-safe, singleton-based signal handling mechanism
// that allows registering handlers for Unix signals. Unlike traditional designs,
// this implementation gives the application control over context cancellation,
// allowing signal handlers to decide if and when to terminate the program.
package signals

import (
	"errors"
	"os"
	"os/signal"
	"sync"
)

// Handler defines the interface for handling a signal.
// Implementers receive the signal and can take appropriate action.
type Handler interface {
	HandleSignal(sig os.Signal)
}

// HandlerFunc is an adapter to allow the use of ordinary functions as signal handlers.
// It satisfies the Handler interface.
type HandlerFunc func(sig os.Signal)

// HandleSignal calls the underlying function with the given signal.
func (h HandlerFunc) HandleSignal(sig os.Signal) {
	h(sig)
}

// defaultSignalSource is the production implementation of SignalSource.
// It delegates to the standard library's signal.Notify function.
type defaultSignalSource struct{}

// Notify registers the provided channel to receive the specified OS signals.
// This wraps signal.Notify for use in the signal handling subsystem.
func (d *defaultSignalSource) Notify(c chan<- os.Signal, sig ...os.Signal) {
	signal.Notify(c, sig...)
}

// signalManager manages signal-to-handler mappings and ensures signal dispatch starts only once.
type signalManager struct {
	mu        sync.RWMutex
	handlers  map[os.Signal][]Handler
	startOnce sync.Once
}

// manager is the global singleton instance of the signalManager.
var manager = &signalManager{
	handlers: make(map[os.Signal][]Handler),
}

// Register adds a handler for a specific signal. Handlers are invoked in the
// order they were registered. It is safe to call concurrently.
func Register(sig os.Signal, h Handler) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.handlers[sig] = append(manager.handlers[sig], h)
}

// RegisterMany adds the same handler for multiple signals. It is safe to call concurrently.
func RegisterMany(sigs []os.Signal, h Handler) {
	for _, sig := range sigs {
		Register(sig, h)
	}
}

// StartWithSource begins listening for the specified OS signals using the provided
// SignalSource implementation. When a signal is received, all registered handlers
// for that signal are invoked in separate goroutines.
//
// This function is useful for injecting mock signal sources during testing.
// It must be called only once per process lifetime; subsequent calls are no-ops.
func StartWithSource(src SignalSource, signals ...os.Signal) error {
	if len(signals) == 0 {
		return errors.New("signals: no signals provided")
	}

	manager.startOnce.Do(func() {
		sigChan := make(chan os.Signal, 1)
		src.Notify(sigChan, signals...)

		go func() {
			for sig := range sigChan {
				manager.mu.RLock()
				handlers := manager.handlers[sig]
				manager.mu.RUnlock()

				for _, h := range handlers {
					go h.HandleSignal(sig)
				}
			}
		}()
	})

	return nil
}

// Start begins listening for the specified OS signals using the default signal source.
// When a signal is received, all registered handlers for that signal are invoked
// concurrently in separate goroutines.
//
// This function is safe to call only once per process lifetime. Subsequent calls
// have no effect. It is equivalent to calling StartWithSource with a default
// implementation that wraps signal.Notify.
func Start(signals ...os.Signal) error {
	return StartWithSource(&defaultSignalSource{}, signals...)
}

// Reset clears all registered signal handlers and resets the internal state.
// It is intended for use in testing or controlled reinitialization.
func Reset() {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	manager.handlers = make(map[os.Signal][]Handler)
	manager.startOnce = sync.Once{}
}
