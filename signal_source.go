package signals

import "os"

// SignalSource defines an abstraction for signal notification.
type SignalSource interface {
	Notify(c chan<- os.Signal, sig ...os.Signal)
}
