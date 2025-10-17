package signals

import "time"

type Policy struct {
	GracePeriod         time.Duration
	ForceOnSecondSignal bool
	LogPanics           bool
}

func defaultPolicy() Policy {
    return Policy{
        GracePeriod:         30 * time.Second,
        ForceOnSecondSignal: true,
        LogPanics:           true,
    }
}
