package signals

import (
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

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

	err := Start(syscall.SIGINT)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	process, _ := os.FindProcess(os.Getpid())
	_ = process.Signal(syscall.SIGINT)

	wg.Wait()

	if atomic.LoadInt32(&called) != 1 {
		t.Fatal("handler was not called")
	}
}

func TestRegisterMany(t *testing.T) {
	Reset()

	var sigintCalls atomic.Int32
	var sigtermCalls atomic.Int32
	intDone := make(chan struct{})
	termDone := make(chan struct{})

	var intOnce sync.Once
	var termOnce sync.Once

	Register(syscall.SIGINT, HandlerFunc(func(sig os.Signal) {
		t.Log("SIGINT handler triggered")
		sigintCalls.Add(1)
		intOnce.Do(func() { close(intDone) })
	}))

	Register(syscall.SIGTERM, HandlerFunc(func(sig os.Signal) {
		t.Log("SIGTERM handler triggered")
		sigtermCalls.Add(1)
		termOnce.Do(func() { close(termDone) })
	}))

	err := Start(syscall.SIGINT, syscall.SIGTERM)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	process, _ := os.FindProcess(os.Getpid())

	if err := process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

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

	if sigintCalls.Load() < 1 {
		t.Error("SIGINT handler not triggered")
	}
	if sigtermCalls.Load() < 1 {
		t.Error("SIGTERM handler not triggered")
	}
}

func TestStartTwice(t *testing.T) {
	Reset()

	err := Start(syscall.SIGINT)
	if err != nil {
		t.Fatalf("first Start failed: %v", err)
	}

	err = Start(syscall.SIGTERM)
	if err != nil {
		t.Logf("second Start correctly ignored: %v", err)
	}
}

func TestReset(t *testing.T) {
	Reset()

	var called int32
	Register(syscall.SIGINT, HandlerFunc(func(sig os.Signal) {
		atomic.StoreInt32(&called, 1)
	}))

	Reset()

	err := Start(syscall.SIGINT)
	if err != nil {
		t.Fatalf("Start after reset failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	process, _ := os.FindProcess(os.Getpid())
	_ = process.Signal(syscall.SIGINT)

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&called) != 0 {
		t.Fatal("handler should not have been called after Reset")
	}
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
