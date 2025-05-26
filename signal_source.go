package signals

import (
	"os"
	"os/signal"
)

// SignalSource is an interface used to abstract signal registration.
// It is primarily useful for injecting mocks during testing.
type SignalSource interface {
	// Notify registers the provided channel to receive the given signals.
	Notify(chan<- os.Signal, ...os.Signal)
}

// defaultSignalSource is the production implementation of SignalSource.
// It delegates to the standard library's signal.Notify function.
type defaultSignalSource struct{}

// Notify registers the provided channel to receive the specified OS signals.
func (d *defaultSignalSource) Notify(c chan<- os.Signal, sig ...os.Signal) {
	signal.Notify(c, sig...)
}
