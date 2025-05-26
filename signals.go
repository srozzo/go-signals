// Package signals provides a thread-safe, singleton-based signal handling mechanism
// that allows registering handlers for Unix signals. Unlike traditional designs,
// this implementation gives the application control over context cancellation,
// allowing signal handlers to decide if and when to terminate the program.
package signals

import (
	"errors"
	"os"
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
// SignalSource. These signals are passed to the source's Notify() function. When a
// signal is received, all registered handlers for that signal are invoked in separate
// goroutines. If no handler is registered, the DefaultHandler from Config is invoked (if set).
//
// This function is useful for injecting mock signal sources during testing.
// It must be called only once per process lifetime; subsequent calls are ignored.
func StartWithSource(src SignalSource, signals ...os.Signal) error {
	if len(signals) == 0 {
		return errors.New("signals: no signals provided")
	}

	manager.startOnce.Do(func() {
		sigChan := make(chan os.Signal, 1)
		src.Notify(sigChan, signals...)

		go func() {
			for sig := range sigChan {
				if isDebug() {
					logf("received signal: %v", sig)
				}

				manager.mu.RLock()
				handlers := manager.handlers[sig]
				manager.mu.RUnlock()

				if len(handlers) == 0 {
					cfg := getConfig()

					if isDebug() {
						logf("no handlers registered for signal: %v", sig)
					}

					if cfg.DefaultHandler != nil {
						go cfg.DefaultHandler(sig)
					}

					// Optional: CancelOnUnhandled logic could go here
					continue
				}

				for _, h := range handlers {
					go h.HandleSignal(sig)
				}
			}
		}()
	})

	return nil
}

// Start begins listening for the specified OS signals using the default signal source.
// It is equivalent to calling StartWithSource with a default source.
// Subsequent calls are ignored.
func Start(signals ...os.Signal) error {
	return StartWithSource(&defaultSignalSource{}, signals...)
}

// Reset clears all registered signal handlers, configuration, and internal state.
// It is intended for use in testing or controlled reinitialization.
func Reset() {
	manager.mu.Lock()
	manager.handlers = make(map[os.Signal][]Handler)
	manager.startOnce = sync.Once{}
	manager.mu.Unlock()

	SetLogger(nil)
	SetDebug(false)
	SetConfig(nil)
}

// ForceResetStartOnce forcibly resets the singleton dispatcher state.
// This is exported for test use.
func ForceResetStartOnce() {
	manager.startOnce = sync.Once{}
}
