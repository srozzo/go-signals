//go:build !windows
// +build !windows

package signals

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestResetThenStartNoSignals_Unix(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()
	r.Register(func(ctx context.Context) {}, syscall.SIGUSR1)
	r.Reset()
	if err := r.Start(); !errors.Is(err, ErrNoSignals) {
		t.Fatalf("expected ErrNoSignals after Reset, got %v", err)
	}
}

func TestUnregister_Unix(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()

	var sum int64
	tok1 := r.Register(func(ctx context.Context) { atomic.AddInt64(&sum, 1) }, syscall.SIGUSR1)[0]
	tok2 := r.Register(func(ctx context.Context) { atomic.AddInt64(&sum, 10) }, syscall.SIGUSR1)[0]
	r.Unregister(tok2) // remove second

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGUSR1)
	time.Sleep(90 * time.Millisecond)

	if got := atomic.LoadInt64(&sum); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}

	r.Unregister(tok1)
	_ = p.Signal(syscall.SIGUSR1)
	time.Sleep(60 * time.Millisecond)
	if got := atomic.LoadInt64(&sum); got != 1 {
		t.Fatalf("expected still 1 after unregistering all, got %d", got)
	}
}

func TestUnregisterNonexistent_Unix(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()
	r.Unregister(Token{sig: syscall.SIGUSR1, id: 999}) // no panic
}

func TestIsTerminatingSwitch_Unix(t *testing.T) {
	if !isTerminating(syscall.SIGTERM) {
		t.Fatal("SIGTERM should be terminating")
	}
	if !isTerminating(syscall.SIGINT) {
		t.Fatal("SIGINT should be terminating")
	}
	if !isTerminating(os.Interrupt) {
		t.Fatal("os.Interrupt should be terminating")
	}
	if !isTerminating(syscall.SIGQUIT) {
		t.Fatal("SIGQUIT should be terminating")
	}
	if isTerminating(syscall.SIGUSR1) {
		t.Fatal("SIGUSR1 should not be terminating")
	}
}

func TestMultipleRegistriesIsolation_Unix(t *testing.T) {
	r1 := NewRegistry()
	r2 := NewRegistry()
	defer r1.Stop()
	defer r2.Stop()

	var c1, c2 int64
	r1.Register(func(ctx context.Context) { atomic.AddInt64(&c1, 1) }, syscall.SIGUSR1)
	r2.Register(func(ctx context.Context) { atomic.AddInt64(&c2, 1) }, syscall.SIGUSR2)

	if err := r1.Start(); err != nil {
		t.Fatal(err)
	}
	if err := r2.Start(); err != nil {
		t.Fatal(err)
	}

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGUSR1)
	_ = p.Signal(syscall.SIGUSR2)
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt64(&c1) != 1 || atomic.LoadInt64(&c2) != 1 {
		t.Fatalf("expected c1=1 c2=1, got c1=%d c2=%d", c1, c2)
	}
}

// Second terminating signal should force while a handler is still processing,
// even with a long grace period, exercising the coalescing + force path.
func TestSecondSignalForcesWhileProcessing_Unix(t *testing.T) {
    r := NewRegistry(WithPolicy(Policy{GracePeriod: time.Minute, ForceOnSecondSignal: true, LogPanics: true}))
    defer r.Stop()

    done := make(chan struct{}, 1)
    r.Register(func(ctx context.Context) {
        select {
        case <-ctx.Done():
            // Force path observed
        case <-time.After(2 * time.Second):
            // Would indicate force never arrived
        }
        done <- struct{}{}
    }, os.Interrupt)

    if err := r.Start(); err != nil {
        t.Fatal(err)
    }

    p, _ := os.FindProcess(os.Getpid())
    // First signal starts processing; second should trigger force.
    _ = p.Signal(os.Interrupt)
    time.Sleep(25 * time.Millisecond)
    _ = p.Signal(os.Interrupt)

    select {
    case <-done:
        // ok, handler observed cancellation and returned
    case <-time.After(750 * time.Millisecond):
        t.Fatal("expected force via second signal while processing")
    }
}

func TestCoalesceDuringProcessing_Unix(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()

	var calls int64
    r.Register(func(ctx context.Context) {
        time.Sleep(200 * time.Millisecond)
        atomic.AddInt64(&calls, 1)
    }, syscall.SIGUSR1)

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGUSR1)
	for i := 0; i < 10; i++ {
		_ = p.Signal(syscall.SIGUSR1)
	}

    // After the first cycle finishes (~200ms), there should be exactly 1 call,
    // even though we fired many signals during processing (they coalesce).
    time.Sleep(250 * time.Millisecond)
    if got := atomic.LoadInt64(&calls); got != 1 {
        t.Fatalf("expected exactly 1 call after first cycle, got %d", got)
    }

    // Trigger a second cycle, then while it is running, send another signal to
    // schedule a third (bounded coalescing: one pending at a time).
    _ = p.Signal(syscall.SIGUSR1)
    time.Sleep(60 * time.Millisecond) // within the second cycle's 200ms window
    _ = p.Signal(syscall.SIGUSR1)
    time.Sleep(600 * time.Millisecond)
    if got := atomic.LoadInt64(&calls); got != 3 {
        t.Fatalf("expected 3 calls after scheduling pending during second cycle, got %d", got)
    }
}

func TestPanicRecovery_WithAndWithoutLog_Unix(t *testing.T) {
	// LogPanics=true
	r1 := NewRegistry(WithPolicy(Policy{GracePeriod: time.Second, ForceOnSecondSignal: true, LogPanics: true}))
	defer r1.Stop()
	var ran1 int64
	r1.Register(func(ctx context.Context) { panic("boom") }, syscall.SIGUSR1)
	r1.Register(func(ctx context.Context) { atomic.AddInt64(&ran1, 1) }, syscall.SIGUSR1)
	if err := r1.Start(); err != nil {
		t.Fatal(err)
	}
	p1, _ := os.FindProcess(os.Getpid())
	_ = p1.Signal(syscall.SIGUSR1)
	time.Sleep(120 * time.Millisecond)
	if atomic.LoadInt64(&ran1) != 1 {
		t.Fatal("second handler did not run after panic (LogPanics=true)")
	}

	// LogPanics=false
	r2 := NewRegistry(WithPolicy(Policy{GracePeriod: time.Second, ForceOnSecondSignal: true, LogPanics: false}))
	defer r2.Stop()
	var ran2 int64
	r2.Register(func(ctx context.Context) { panic("boom2") }, syscall.SIGUSR1)
	r2.Register(func(ctx context.Context) { atomic.AddInt64(&ran2, 1) }, syscall.SIGUSR1)
	if err := r2.Start(); err != nil {
		t.Fatal(err)
	}
	p2, _ := os.FindProcess(os.Getpid())
	_ = p2.Signal(syscall.SIGUSR1)
	time.Sleep(120 * time.Millisecond)
	if atomic.LoadInt64(&ran2) != 1 {
		t.Fatal("second handler did not run after panic (LogPanics=false)")
	}
}

func TestGraceTimerForces_Unix(t *testing.T) {
	r := NewRegistry(WithPolicy(Policy{
		GracePeriod:         60 * time.Millisecond,
		ForceOnSecondSignal: false,
		LogPanics:           true,
	}))
	defer r.Stop()

	forced := make(chan struct{}, 1)
	r.Register(func(ctx context.Context) { <-ctx.Done(); forced <- struct{}{} }, syscall.SIGTERM)

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)

	select {
	case <-forced:
		// ok
	case <-time.After(600 * time.Millisecond):
		t.Fatal("expected force via grace timer")
	}
}

func TestSecondSignalForces_WhileProcessing_Unix(t *testing.T) {
	r := NewRegistry(WithPolicy(Policy{GracePeriod: time.Minute, ForceOnSecondSignal: true, LogPanics: true}))
	defer r.Stop()

	forced := make(chan struct{}, 1)
	r.Register(func(ctx context.Context) {
		select {
		case <-ctx.Done():
		case <-time.After(500 * time.Millisecond):
		}
		forced <- struct{}{}
	}, syscall.SIGTERM)

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)
	time.Sleep(25 * time.Millisecond)
	_ = p.Signal(syscall.SIGTERM)

	select {
	case <-forced:
		// ok
	case <-time.After(650 * time.Millisecond):
		t.Fatal("expected force path after second SIGTERM while processing")
	}
}

func TestSecondSignalDifferentKindsAlsoForces_Unix(t *testing.T) {
	r := NewRegistry(WithPolicy(Policy{
		GracePeriod:         time.Minute,
		ForceOnSecondSignal: true,
		LogPanics:           true,
	}))
	defer r.Stop()

	forced := make(chan struct{}, 1)
	r.Register(func(ctx context.Context) { <-ctx.Done(); forced <- struct{}{} }, os.Interrupt, syscall.SIGTERM)

	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(os.Interrupt)
	time.Sleep(25 * time.Millisecond)
	_ = p.Signal(syscall.SIGTERM)

	select {
	case <-forced:
		// ok
	case <-time.After(600 * time.Millisecond):
		t.Fatal("force path not observed after INT then TERM")
	}
}

// Stress test: hammer register/unregister and dispatch concurrently for a short budget.
func TestStressDispatch_Unix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	r := NewRegistry(WithPolicy(Policy{GracePeriod: 100 * time.Millisecond, ForceOnSecondSignal: true, LogPanics: true}))
	defer r.Stop()

	var total int64
	base := func(ctx context.Context) { atomic.AddInt64(&total, 1) }
	r.Register(base, syscall.SIGUSR1)
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})

	// signal pumper
	go func() {
		p, _ := os.FindProcess(os.Getpid())
		for {
			select {
			case <-stop:
				return
			default:
				_ = p.Signal(syscall.SIGUSR1)
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// registrar/unregistrar churn
	go func() {
		toks := make([]Token, 0, 64)
		for {
			select {
			case <-stop:
				return
			default:
				tok := r.Register(func(ctx context.Context) { atomic.AddInt64(&total, 1) }, syscall.SIGUSR1)[0]
				toks = append(toks, tok)
				if len(toks) > 32 {
					r.Unregister(toks[0])
					toks = toks[1:]
				}
			}
		}
	}()

	time.Sleep(400 * time.Millisecond)
	close(stop)
	runtime.Gosched()
	time.Sleep(30 * time.Millisecond)

	if atomic.LoadInt64(&total) == 0 {
		t.Fatal("stress: expected some handler executions, got 0")
	}
}
