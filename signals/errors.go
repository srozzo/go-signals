package signals

import "errors"

var (
	ErrAlreadyStarted = errors.New("signals: already started")
	ErrNoSignals      = errors.New("signals: no signals registered")
)
