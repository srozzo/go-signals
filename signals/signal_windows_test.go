//go:build windows
// +build windows

package signals

import (
    "context"
    "errors"
    "os"
    "sync/atomic"
    "syscall"
    "testing"
    "time"
)

func TestWindows_StartAndStop(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()
	r.Register(func(ctx context.Context) {}, os.Interrupt)
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	r.Stop()
	r.Stop()
}

func TestWindows_ContextFor_Interrupt(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()

	ctx, cancel := r.ContextFor(context.Background(), os.Interrupt)
	defer cancel()

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(os.Interrupt)

	select {
	case <-ctx.Done():
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ContextFor did not cancel on os.Interrupt")
	}
}

func TestWindows_SecondSignalForces_INT_then_TERM(t *testing.T) {
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
	case <-time.After(750 * time.Millisecond):
		t.Fatal("force path not observed after os.Interrupt then SIGTERM")
	}
}

func TestWindows_SecondSignalForces_TERM_then_INT(t *testing.T) {
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
    _ = p.Signal(syscall.SIGTERM)
    time.Sleep(25 * time.Millisecond)
    _ = p.Signal(os.Interrupt)

    select {
    case <-forced:
        // ok
    case <-time.After(750 * time.Millisecond):
        t.Fatal("force path not observed after SIGTERM then os.Interrupt")
    }
}

func TestWindows_StartNoSignalsAndIdempotent(t *testing.T) {
	r := NewRegistry()
	defer r.Stop()
	if err := r.Start(); !errors.Is(err, ErrNoSignals) {
		t.Fatalf("expected ErrNoSignals, got %v", err)
	}
	r.Register(func(ctx context.Context) {}, os.Interrupt)
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("expected ErrAlreadyStarted, got %v", err)
	}
}

func TestWindows_ConvenienceLoggerAndPolicy(t *testing.T) {
	old := Default
	Default = NewRegistry()
	t.Cleanup(func() { Default.Stop(); Default = old })

	var logs int64
	SetPolicy(Policy{GracePeriod: 50 * time.Millisecond, ForceOnSecondSignal: false, LogPanics: true})
	SetLogger(func(format string, args ...any) { atomic.AddInt64(&logs, 1) })
	SetDebug(true)

	done := make(chan struct{}, 1)
	Register(func(ctx context.Context) { <-ctx.Done(); done <- struct{}{} }, os.Interrupt)

	if err := Start(); err != nil {
		t.Fatal(err)
	}

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(os.Interrupt)

	select {
	case <-done:
		// ok
	case <-time.After(800 * time.Millisecond):
		t.Fatal("expected force via grace timer on Windows")
	}

	if atomic.LoadInt64(&logs) == 0 {
		t.Fatal("expected some debug logs")
	}
}

func TestWindows_IsTerminating_Classification(t *testing.T) {
    if !isTerminating(os.Interrupt) {
        t.Fatal("os.Interrupt should be terminating on Windows")
    }
    if !isTerminating(syscall.SIGTERM) {
        t.Fatal("SIGTERM should be terminating on Windows")
    }
    if isTerminating(syscall.SIGINT) {
        t.Fatal("SIGINT should not be terminating on Windows")
    }
    if isTerminating(syscall.SIGQUIT) {
        t.Fatal("SIGQUIT should not be terminating on Windows")
    }
}

func TestWindows_UnsupportedSignalsIgnored(t *testing.T) {
    r := NewRegistry()
    defer r.Stop()
    // SIGQUIT should be ignored (unsupported) on Windows
    toks := r.Register(func(ctx context.Context) {}, syscall.SIGQUIT)
    if len(toks) != 0 {
        t.Fatalf("expected no tokens for unsupported signal; got %d", len(toks))
    }
    if err := r.Start(); !errors.Is(err, ErrNoSignals) {
        t.Fatalf("expected ErrNoSignals after only unsupported registrations, got %v", err)
    }
}
