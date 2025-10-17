package signals

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "runtime"
    "runtime/debug"
    "sync"
    "syscall"
    "time"
)

type Handler func(ctx context.Context)

// Token is an opaque handle for unregistering a specific handler registration.
type Token struct {
	sig os.Signal
	id  uint64
}

type entry struct {
	id      uint64
	handler Handler
}

// Registry implements the SPEC’s ordered dispatch with panic recovery, coalescing,
// grace period handling, and optional second-signal force path.
type Registry struct {
    mu sync.Mutex

    // configuration
    policy Policy
    logf   LoggerFunc
    debug  bool

    // state
    started     bool
    stopped     bool
    nextID      uint64
    handlers    map[os.Signal][]entry // per-signal ordered handlers
    sigch       chan os.Signal
    stopCh      chan struct{} // closes when Stop is called
    forceCtx    context.Context
    forceCancel context.CancelFunc

    // shutdown/force tracking
    shuttingDown bool
    // count of “terminating” signals seen since shutdown began
    termCount int

    // coalescing state: track in-flight processing and a single pending cycle per signal
    processing map[os.Signal]bool
    pending    map[os.Signal]bool
}

func NewRegistry(opts ...Option) *Registry {
    r := &Registry{
        policy:   defaultPolicy(),
        logf:     func(string, ...any) {},
        handlers: make(map[os.Signal][]entry),
        stopCh:   make(chan struct{}),
        processing: make(map[os.Signal]bool),
        pending:    make(map[os.Signal]bool),
    }
    for _, opt := range opts {
        opt(r)
    }
    r.forceCtx, r.forceCancel = context.WithCancel(context.Background())
    return r
}

var Default = NewRegistry()

func (r *Registry) Register(h Handler, sigs ...os.Signal) (tokens []Token) {
    r.mu.Lock()
    defer r.mu.Unlock()

    for _, s := range sigs {
        if !isSupportedSignal(s) {
            if r.debug {
                r.logf("signals: ignoring unsupported signal %v on %s", s, runtime.GOOS)
            }
            continue
        }
        r.nextID++
        id := r.nextID
        r.handlers[s] = append(r.handlers[s], entry{id: id, handler: h})
        tokens = append(tokens, Token{sig: s, id: id})
        if r.debug {
			r.logf("signals: register id=%d for %v", id, s)
		}
	}
	return
}

func (r *Registry) Unregister(t Token) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.handlers[t.sig]
	for i := range list {
		if list[i].id == t.id {
			// remove while preserving order
			copy(list[i:], list[i+1:])
			list = list[:len(list)-1]
			r.handlers[t.sig] = list
			if r.debug {
				r.logf("signals: unregister id=%d for %v", t.id, t.sig)
			}
			return
		}
	}
}

func (r *Registry) Reset() {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.handlers = make(map[os.Signal][]entry)
    // Clear shutdown state and counters.
    r.shuttingDown = false
    r.termCount = 0
    // Refresh force context so future handlers do not inherit a canceled ctx.
    r.forceCtx, r.forceCancel = context.WithCancel(context.Background())
    // Clear coalescing flags.
    r.processing = make(map[os.Signal]bool)
    r.pending = make(map[os.Signal]bool)
    if r.debug {
        r.logf("signals: reset all handlers")
    }
}

// Start wires os/signal and begins dispatch loop.
// It is idempotent per registry.
func (r *Registry) Start() error {
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return fmt.Errorf("signals: Start after Stop")
	}
	if r.started {
		r.mu.Unlock()
		return ErrAlreadyStarted
	}
	if r.totalHandlerCountLocked() == 0 {
		r.mu.Unlock()
		return ErrNoSignals
	}
	// snapshot keys to subscribe
	var all []os.Signal
	for s := range r.handlers {
		all = append(all, s)
	}
    r.sigch = make(chan os.Signal, 16)
    signal.Notify(r.sigch, all...)
    r.started = true
    r.mu.Unlock()

    // Pass stable copies of channels to avoid races with Stop mutating fields.
    go r.loop(r.sigch, r.stopCh)
    return nil
}

func (r *Registry) Stop() {
    r.mu.Lock()
    // If Stop is called before Start, do nothing (true no-op).
    if !r.started {
        r.mu.Unlock()
        return
    }
    if r.stopped {
        r.mu.Unlock()
        return
    }
    r.stopped = true
    sigch := r.sigch
    stopCh := r.stopCh
    cancel := r.forceCancel
    r.sigch = nil
    r.started = false
    r.mu.Unlock()

    if sigch != nil {
        signal.Stop(sigch)
    }
    // Close stopCh to end the dispatch loop.
    if stopCh != nil {
        close(stopCh)
    }
    // Cancel handler contexts to trigger force-path exits.
    if cancel != nil {
        cancel()
    }
}

func (r *Registry) ContextFor(parent context.Context, sigs ...os.Signal) (context.Context, context.CancelFunc) {
	ctx, cancel := signal.NotifyContext(parent, sigs...)
	return ctx, cancel
}

func (r *Registry) StartWithContext(parent context.Context) error {
	// Parent cancellation does not trigger handlers; this is sugar for lifecycle plumbing.
	// We still must Start the registry to handle OS signals.
	return r.Start()
}

func (r *Registry) loop(sigch chan os.Signal, stopCh chan struct{}) {
    // Uses r.processing and r.pending under lock to coalesce arrivals and schedule one pending cycle.

    for {
        select {
        case <-stopCh:
            return
        case s, ok := <-sigch:
            if !ok {
                return
            }
            r.mu.Lock()
            // If this signal is already being processed, set pending and still respect second-signal policy.
            if r.processing[s] {
                r.pending[s] = true
                if isTerminating(s) && r.shuttingDown && r.policy.ForceOnSecondSignal {
                    r.termCount++
                    doForce := r.termCount >= 2
                    r.mu.Unlock()
                    if doForce {
                        r.force()
                    }
                } else {
                    r.mu.Unlock()
                }
                continue
            }

            // Mark as in-flight to coalesce re-entrancy while handlers run.
            r.processing[s] = true

            // Shutdown bookkeeping is done under the lock to avoid races with coalesced signals.
            startGrace := false
            grace := r.policy.GracePeriod
            doForce := false

            if isTerminating(s) {
                if !r.shuttingDown {
                    r.shuttingDown = true
                    r.termCount = 1
                    if grace > 0 {
                        startGrace = true
                    }
                } else {
                    r.termCount++
                    if r.policy.ForceOnSecondSignal && r.termCount >= 2 {
                        doForce = true
                    }
                }
            }

            // Snapshot handlers for this signal.
            hlist := append([]entry(nil), r.handlers[s]...)
            r.mu.Unlock()

            if startGrace {
                // Start grace timer outside the lock.
                go func(d time.Duration) {
                    timer := time.NewTimer(d)
                    defer timer.Stop()
                    select {
                    case <-timer.C:
                        r.force()
                    case <-r.stopCh:
                        return
                    }
                }(grace)
            }
            if doForce {
                r.force()
            }

            go func(sig os.Signal, snapshot []entry) {
                // snapshot logger/debug flags to avoid races with Set* during dispatch
                r.mu.Lock()
                localLogf := r.logf
                localDebug := r.debug
                logPanics := r.policy.LogPanics
                localCtx := r.forceCtx
                r.mu.Unlock()

                process := func(list []entry) {
                    start := time.Now()
                    for i, e := range list {
                        func(idx int, en entry) {
                            defer func() {
                                if rec := recover(); rec != nil {
                                    if logPanics {
                                        localLogf("signals: panic in handler %d for %v: %v\n%s", idx, sig, rec, string(debug.Stack()))
                                    }
                                }
                            }()
                            en.handler(localCtx)
                        }(i, e)
                    }
                    if localDebug {
                        localLogf("signals: handled %v with %d handlers in %s", sig, len(list), time.Since(start))
                    }
                }

                // Run at least one cycle, then any pending one-at-a-time cycles.
                current := snapshot
                for {
                    process(current)
                    r.mu.Lock()
                    if r.pending[sig] {
                        r.pending[sig] = false
                        current = append([]entry(nil), r.handlers[sig]...)
                        r.mu.Unlock()
                        continue
                    }
                    delete(r.processing, sig)
                    r.mu.Unlock()
                    break
                }
            }(s, hlist)
        }
    }
}

func (r *Registry) handlerContext() context.Context {
    // returns a context canceled by the force path
    return r.forceCtx
}

func (r *Registry) beginShutdown() {
    r.mu.Lock()
    if r.shuttingDown {
        r.mu.Unlock()
        return
    }
    r.shuttingDown = true
    r.termCount = 1
    grace := r.policy.GracePeriod
    forceOnSecond := r.policy.ForceOnSecondSignal
    // snapshot logger/debug under lock
    localDebug := r.debug
    localLogf := r.logf
    r.mu.Unlock()

    if localDebug {
        localLogf("signals: shutdown started; grace=%s second-signal=%v", grace, forceOnSecond)
    }

	// grace timer
	if grace > 0 {
		go func() {
			timer := time.NewTimer(grace)
			defer timer.Stop()
			select {
			case <-timer.C:
				r.force()
			case <-r.stopCh:
				return
			}
		}()
	}
}

func (r *Registry) incrementTermAndMaybeForce() {
	r.mu.Lock()
	r.termCount++
	doForce := r.policy.ForceOnSecondSignal && r.termCount >= 2
	r.mu.Unlock()
	if doForce {
		r.force()
	}
}

func (r *Registry) force() {
    // idempotent
    r.mu.Lock()
    alreadyShutting := r.shuttingDown
    localDebug := r.debug
    localLogf := r.logf
    r.mu.Unlock()
    if !alreadyShutting {
        return
    }
    if localDebug {
        localLogf("signals: entering force path; canceling handler contexts")
    }
    r.forceCancel()
}

func (r *Registry) totalHandlerCountLocked() int {
	n := 0
	for _, list := range r.handlers {
		n += len(list)
	}
	return n
}

func isTerminating(s os.Signal) bool {
    // Treat common termination signals as “terminating.”
    if runtime.GOOS == "windows" {
        switch s {
        case os.Interrupt, syscall.SIGTERM:
            return true
        default:
            return false
        }
    }
    switch s {
    case os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT:
        return true
    default:
        return false
    }
}

func isSupportedSignal(s os.Signal) bool {
    if runtime.GOOS == "windows" {
        switch s {
        case os.Interrupt, syscall.SIGTERM:
            return true
        default:
            return false
        }
    }
    // On Unix-like platforms, we assume all provided signals are supported.
    return true
}
