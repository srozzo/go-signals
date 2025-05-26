package signals

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// mockSignalSource is a test implementation of SignalSource.
type mockSignalSource struct {
	SignalChan chan os.Signal
}

// Notify forwards signals from the mock's internal channel to the dispatcher.
func (m *mockSignalSource) Notify(c chan<- os.Signal, _ ...os.Signal) {
	go func() {
		for sig := range m.SignalChan {
			c <- sig
		}
	}()
}

func TestStartDefaultSource(t *testing.T) {
	Reset()
	err := Start(syscall.SIGINT)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
}

func TestStartWithSource_NoSignals(t *testing.T) {
	err := StartWithSource(&mockSignalSource{SignalChan: make(chan os.Signal)}, []os.Signal{}...)
	if err == nil {
		t.Error("expected error for no signals provided")
	}
}

func TestRegisterAndTriggerHandler(t *testing.T) {
	Reset()

	var called int32
	var wg sync.WaitGroup
	wg.Add(1)

	Register(syscall.SIGINT, HandlerFunc(func(sig os.Signal) {
		defer wg.Done()
		t.Logf("handler triggered for signal: %v", sig)
		atomic.StoreInt32(&called, 1)
	}))

	mockSrc := &mockSignalSource{SignalChan: make(chan os.Signal, 1)}
	if err := StartWithSource(mockSrc, syscall.SIGINT); err != nil {
		t.Fatalf("StartWithSource failed: %v", err)
	}

	mockSrc.SignalChan <- syscall.SIGINT
	wg.Wait()

	if atomic.LoadInt32(&called) != 1 {
		t.Fatal("handler was not called")
	}
}

func TestRegisterMany(t *testing.T) {
	Reset()
	ForceResetStartOnce()

	var sigintCalled atomic.Bool
	var sigtermCalled atomic.Bool

	intDone := make(chan struct{})
	termDone := make(chan struct{})

	Register(syscall.SIGINT, HandlerFunc(func(sig os.Signal) {
		t.Log("SIGINT handler triggered")
		sigintCalled.Store(true)
		select {
		case <-intDone:
		default:
			close(intDone)
		}
	}))

	Register(syscall.SIGTERM, HandlerFunc(func(sig os.Signal) {
		t.Log("SIGTERM handler triggered")
		sigtermCalled.Store(true)
		select {
		case <-termDone:
		default:
			close(termDone)
		}
	}))

	mockSrc := &mockSignalSource{SignalChan: make(chan os.Signal, 4)}
	if err := StartWithSource(mockSrc, syscall.SIGINT, syscall.SIGTERM); err != nil {
		t.Fatalf("StartWithSource failed: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	mockSrc.SignalChan <- syscall.SIGINT
	mockSrc.SignalChan <- syscall.SIGTERM

	select {
	case <-intDone:
	case <-time.After(500 * time.Millisecond):
		t.Error("SIGINT handler did not run")
	}

	select {
	case <-termDone:
	case <-time.After(500 * time.Millisecond):
		t.Error("SIGTERM handler did not run")
	}

	if !sigintCalled.Load() {
		t.Error("SIGINT handler not triggered")
	}
	if !sigtermCalled.Load() {
		t.Error("SIGTERM handler not triggered")
	}
}

func TestStartTwice(t *testing.T) {
	Reset()

	mockSrc := &mockSignalSource{SignalChan: make(chan os.Signal, 1)}
	if err := StartWithSource(mockSrc, syscall.SIGINT); err != nil {
		t.Fatalf("first StartWithSource failed: %v", err)
	}

	if err := StartWithSource(mockSrc, syscall.SIGTERM); err != nil {
		t.Logf("second StartWithSource correctly ignored: %v", err)
	}
}

func TestReset(t *testing.T) {
	Reset()

	var called int32
	Register(syscall.SIGINT, HandlerFunc(func(sig os.Signal) {
		atomic.StoreInt32(&called, 1)
	}))

	Reset()

	mockSrc := &mockSignalSource{SignalChan: make(chan os.Signal, 1)}
	if err := StartWithSource(mockSrc, syscall.SIGINT); err != nil {
		t.Fatalf("StartWithSource after reset failed: %v", err)
	}

	mockSrc.SignalChan <- syscall.SIGINT
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&called) != 0 {
		t.Fatal("handler should not have been called after Reset")
	}
}

func TestResetClearsGlobals(t *testing.T) {
	SetLogger(func(string, ...any) {})
	SetDebug(true)
	SetConfig(&Config{CancelOnUnhandled: true})

	Reset()

	if isDebug() {
		t.Error("expected debug mode to be false after reset")
	}
	if getConfig().CancelOnUnhandled {
		t.Error("expected config to be reset to default")
	}
}

func TestDoubleReset(t *testing.T) {
	Reset()
	Reset() // should not panic
}

func TestLogger(t *testing.T) {
	var called atomic.Bool
	SetLogger(func(format string, args ...any) {
		called.Store(true)
	})
	logf("test message")

	if !called.Load() {
		t.Error("custom logger was not called")
	}

	SetLogger(nil)
}

func TestDebugToggle(t *testing.T) {
	SetDebug(false)
	if isDebug() {
		t.Error("debug should be false")
	}

	SetDebug(true)
	if !isDebug() {
		t.Error("debug should be true")
	}
}

func TestDefaultHandlerWhenNoHandlerRegistered(t *testing.T) {
	Reset()
	ForceResetStartOnce()

	var called atomic.Bool
	done := make(chan struct{})

	SetConfig(&Config{
		DefaultHandler: func(sig os.Signal) {
			t.Logf("default handler received: %v", sig)
			called.Store(true)
			close(done)
		},
	})

	mockSrc := &mockSignalSource{SignalChan: make(chan os.Signal, 1)}
	if err := StartWithSource(mockSrc, syscall.SIGINT); err != nil {
		t.Fatalf("StartWithSource failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	mockSrc.SignalChan <- syscall.SIGUSR1

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Error("default handler was not invoked for unregistered signal")
	}
}

func TestCancelOnUnhandled(t *testing.T) {
	Reset()
	ForceResetStartOnce()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	SetConfig(&Config{
		CancelOnUnhandled: true,
		DefaultHandler: func(sig os.Signal) {
			t.Logf("default handler: %v", sig)
			cancel()
		},
	})

	mockSrc := &mockSignalSource{SignalChan: make(chan os.Signal, 1)}
	if err := StartWithSource(mockSrc, syscall.SIGINT); err != nil {
		t.Fatalf("StartWithSource failed: %v", err)
	}

	go func() {
		select {
		case <-ctx.Done():
			close(done)
		case <-time.After(500 * time.Millisecond):
			t.Error("context not canceled on unhandled signal")
		}
	}()

	mockSrc.SignalChan <- syscall.SIGUSR2

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Error("context cancellation not observed")
	}
}
