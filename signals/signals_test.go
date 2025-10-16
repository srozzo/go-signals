package signals

import (
    "context"
    "errors"
    "fmt"
    "os"
    "sync/atomic"
    "testing"
    "time"
    "syscall"
)

// Cross-platform: only uses os.Interrupt.

func TestStartNoSignals_Cross(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()
	if err := r.Start(); !errors.Is(err, ErrNoSignals) {
		t.Fatalf("expected ErrNoSignals, got %v", err)
	}
}

func TestResetClearsShutdownState_Cross(t *testing.T) {
	r := NewRegistry(WithPolicy(Policy{GracePeriod: time.Minute, ForceOnSecondSignal: true}))
	defer r.Stop()

	// Simulate prior shutdown, then reset and ensure state is clean.
	r.beginShutdown()
	r.Reset()

	ctx := r.handlerContext()
	r.beginShutdown()

	// With a 1-minute grace, there should be no immediate force.
	select {
	case <-ctx.Done():
		t.Fatal("unexpected force after Reset with long grace period")
	case <-time.After(50 * time.Millisecond):
		// ok
	}
}

func TestHandlerOrderWithPanic_Cross(t *testing.T) {
	r := NewRegistry(WithPolicy(Policy{GracePeriod: time.Second, LogPanics: true}))
	defer r.Stop()

    // Use a channel to avoid data races on shared slice under -race.
    seqCh := make(chan int, 2)
    r.Register(func(ctx context.Context) { seqCh <- 1; panic("boom") }, os.Interrupt)
    r.Register(func(ctx context.Context) { seqCh <- 2 }, os.Interrupt)

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(os.Interrupt)
    // Collect exactly two results or time out
    got := make([]int, 0, 2)
    deadline := time.After(500 * time.Millisecond)
    for len(got) < 2 {
        select {
        case v := <-seqCh:
            got = append(got, v)
        case <-deadline:
            t.Fatalf("timeout waiting for handlers; got %v", got)
        }
    }
    if got[0] != 1 || got[1] != 2 {
        t.Fatalf("expected order [1,2], got %v", got)
    }
}

// Unregistering during dispatch should not affect the current snapshot: both
// handlers on the current signal delivery must run, and the removal only
// impacts subsequent deliveries.
func TestUnregisterDuringDispatch_SnapshotIsolation_Cross(t *testing.T) {
    r := NewRegistry()
    defer r.Stop()

    var first, second int64

    // Placeholder for token to allow h1 to unregister h2 during dispatch.
    var tok2 Token

    h1 := func(ctx context.Context) {
        // Unregister h2 while we are handling the signal.
        r.Unregister(tok2)
        // Small delay to increase chance h2 was already in the snapshot.
        time.Sleep(20 * time.Millisecond)
        atomic.AddInt64(&first, 1)
    }
    h2 := func(ctx context.Context) {
        atomic.AddInt64(&second, 1)
    }

    r.Register(h1, os.Interrupt)
    tok2 = r.Register(h2, os.Interrupt)[0]

    if err := r.Start(); err != nil {
        t.Fatal(err)
    }

    p, _ := os.FindProcess(os.Getpid())
    _ = p.Signal(os.Interrupt)
    time.Sleep(120 * time.Millisecond)

    // Both handlers from the original snapshot should have run once.
    if atomic.LoadInt64(&first) != 1 || atomic.LoadInt64(&second) != 1 {
        t.Fatalf("expected first=1 second=1 after first signal; got %d and %d", first, second)
    }

    // Send another signal; h2 should have been removed and not run again.
    _ = p.Signal(os.Interrupt)
    time.Sleep(120 * time.Millisecond)

    if atomic.LoadInt64(&first) != 2 || atomic.LoadInt64(&second) != 1 {
        t.Fatalf("expected first=2 second=1 after second signal; got %d and %d", first, second)
    }
}

func TestCoalesceLogs_Cross(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()

	var logs int64
	r.policy.LogPanics = true
	r.debug = true
	r.logf = func(format string, args ...any) { atomic.AddInt64(&logs, 1) }

	r.Register(func(ctx context.Context) { time.Sleep(80 * time.Millisecond) }, os.Interrupt)
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(os.Interrupt)
	for i := 0; i < 5; i++ {
		_ = p.Signal(os.Interrupt)
	}

	time.Sleep(150 * time.Millisecond)
	if atomic.LoadInt64(&logs) == 0 {
		t.Fatal("expected debug logs for coalescing path")
	}
}

func TestUnregisterSingleToken_Cross(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()

	var a, b int64
	t1 := r.Register(func(ctx context.Context) { atomic.AddInt64(&a, 1) }, os.Interrupt)[0]
	_ = r.Register(func(ctx context.Context) { atomic.AddInt64(&b, 1) }, os.Interrupt)

	r.Unregister(t1)
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(os.Interrupt)
	time.Sleep(90 * time.Millisecond)

	if atomic.LoadInt64(&a) != 0 || atomic.LoadInt64(&b) != 1 {
		t.Fatalf("expected a=0,b=1; got a=%d,b=%d", a, b)
	}
}

func TestBeginShutdownZeroGrace_NoForce_Cross(t *testing.T) {
	r := NewRegistry(WithPolicy(Policy{GracePeriod: 0, ForceOnSecondSignal: false}))
	defer r.Stop()
	ctx := r.handlerContext()
	r.beginShutdown()
	select {
	case <-ctx.Done():
		t.Fatal("unexpected force with zero grace")
	case <-time.After(80 * time.Millisecond):
		// ok
	}
}

func TestForceIdempotent_Cross(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()
	ctx := r.handlerContext()
	r.beginShutdown()
	r.force()
	r.force()
	select {
	case <-ctx.Done(): // ok
	case <-time.After(100 * time.Millisecond):
		t.Fatal("force did not cancel")
	}
}

func TestStartWithContext_Cross(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()
	r.Register(func(ctx context.Context) {}, os.Interrupt)

	// Should delegate to Start and succeed.
	if err := r.StartWithContext(context.Background()); err != nil {
		t.Fatalf("StartWithContext: %v", err)
	}
	r.Stop()
}

// Unregister with a token whose signal does not exist in the map should be a no-op.
func TestUnregisterUnknownSignal_Cross(t *testing.T) {
    r := NewRegistry()
    defer r.Stop()
    // Register for os.Interrupt only
    _ = r.Register(func(ctx context.Context) {}, os.Interrupt)
    // Attempt to unregister a token for a different signal with arbitrary id
    r.Unregister(Token{sig: syscall.SIGTERM, id: 42})
}

// force() should be a no-op if shutdown hasn't begun (ctx should NOT be canceled).
func TestForceEarlyReturn_Cross(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()

	ctx := r.handlerContext()
	r.force() // no shutdown yet → early return

	select {
	case <-ctx.Done():
		t.Fatal("force() canceled context without shutdown")
	case <-time.After(50 * time.Millisecond):
		// ok
	}
}

// beginShutdown should start the grace timer and eventually cancel handler contexts.
func TestBeginShutdown_Grace_Cross(t *testing.T) {
	r := NewRegistry(WithPolicy(Policy{
		GracePeriod:         25 * time.Millisecond,
		ForceOnSecondSignal: false,
		LogPanics:           true,
	}))
	defer r.Stop()

	ctx := r.handlerContext()
	r.beginShutdown() // sets shuttingDown and starts grace timer

	// Call again to hit the early-return branch (already shutting down).
	r.beginShutdown()

	select {
	case <-ctx.Done():
		// ok: grace timer fired and forced
	case <-time.After(300 * time.Millisecond):
		t.Fatal("context was not canceled by grace timer after beginShutdown")
	}
}

// Reset should emit a debug log when debug is enabled.
func TestReset_LogsWhenDebug_Cross(t *testing.T) {
    r := NewRegistry()
    defer r.Stop()
    r.debug = true
    var logs int64
    r.logf = func(format string, args ...any) { atomic.AddInt64(&logs, 1) }
    r.Reset()
    if atomic.LoadInt64(&logs) == 0 {
        t.Fatal("expected a debug log from Reset when debug enabled")
    }
}

// beginShutdown should log when debug is enabled.
func TestBeginShutdown_LogsWhenDebug_Cross(t *testing.T) {
    r := NewRegistry(WithPolicy(Policy{GracePeriod: 0, ForceOnSecondSignal: false}))
    defer r.Stop()
    r.debug = true
    var logs int64
    r.logf = func(format string, args ...any) { atomic.AddInt64(&logs, 1) }
    r.beginShutdown()
    // give the goroutine (if any) a tiny moment; no timer at grace=0 though
    time.Sleep(10 * time.Millisecond)
    if atomic.LoadInt64(&logs) == 0 {
        t.Fatal("expected a debug log from beginShutdown when debug enabled")
    }
}

// incrementTermAndMaybeForce should cancel contexts on the second terminating signal.
func TestIncrementTermAndMaybeForce_Cross(t *testing.T) {
	r := NewRegistry(WithPolicy(Policy{
		GracePeriod:         0,    // no timer
		ForceOnSecondSignal: true, // force on 2nd signal
		LogPanics:           true,
	}))
	defer r.Stop()

	ctx := r.handlerContext()
	r.beginShutdown()              // sets termCount=1
	r.incrementTermAndMaybeForce() // termCount=2 -> force

	select {
	case <-ctx.Done():
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Fatal("context was not canceled by second-signal force")
	}
}

func TestStartIdempotent_Cross(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()
	r.Register(func(ctx context.Context) {}, os.Interrupt)
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("expected ErrAlreadyStarted, got %v", err)
	}
}

func TestStartAfterStop_Cross(t *testing.T) {
	r := NewRegistry()
	r.Register(func(ctx context.Context) {}, os.Interrupt)
	// Stop before Start should be a true no-op now.
	r.Stop()
	if err := r.Start(); err != nil {
		t.Fatalf("expected Start to succeed after Stop-before-Start, got %v", err)
	}
	r.Stop()
}

func TestStartAfterStop_AfterStarted_Cross(t *testing.T) {
	r := NewRegistry()
	r.Register(func(ctx context.Context) {}, os.Interrupt)
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	r.Stop()
	if err := r.Start(); err == nil || err.Error() != "signals: Start after Stop" {
		t.Fatalf("expected 'signals: Start after Stop', got %v", err)
	}
}

func TestStopIdempotentAndBeforeStart_Cross(t *testing.T) {
	r := NewRegistry()
	// Stop before Start is a no-op
	r.Stop()
	r.Register(func(ctx context.Context) {}, os.Interrupt)
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	r.Stop()
	r.Stop()
}

func TestContextForCancelOnSignal_Cross(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()

	ctx, cancel := r.ContextFor(context.Background(), os.Interrupt)
	defer cancel()

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(os.Interrupt)

	select {
	case <-ctx.Done():
		// ok
	case <-time.After(400 * time.Millisecond):
		t.Fatal("ContextFor did not cancel on os.Interrupt")
	}
}

func TestConvenience_SetPolicy_And_Logger_Cross(t *testing.T) {
	// Fresh Default instance
	old := Default
	Default = NewRegistry()
	t.Cleanup(func() { Default.Stop(); Default = old })

	// Short grace so we see force without second signal
	SetPolicy(Policy{GracePeriod: 50 * time.Millisecond, ForceOnSecondSignal: false, LogPanics: true})

	var logs int64
	SetLogger(func(format string, args ...any) { _ = fmt.Sprintf(format, args...); atomic.AddInt64(&logs, 1) })
	SetDebug(true)

	forced := make(chan struct{}, 1)
	Register(func(ctx context.Context) { <-ctx.Done(); forced <- struct{}{} }, os.Interrupt)

	if err := Start(); err != nil {
		t.Fatal(err)
	}

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(os.Interrupt)

	select {
	case <-forced:
		// ok
	case <-time.After(600 * time.Millisecond):
		t.Fatal("expected force via grace timer from SetPolicy")
	}

	// We should have emitted at least some debug logs
	if atomic.LoadInt64(&logs) == 0 {
		t.Fatal("expected debug logs when SetDebug(true)")
	}
}
