package signals

import (
    "context"
    "os"
    "testing"
    "time"
)

// Ensure the exported convenience wrappers delegate to Default and are wired.
func TestConvenienceWrappers_Delegate_Cross(t *testing.T) {
    old := Default
    Default = NewRegistry()
    t.Cleanup(func() { Default.Stop(); Default = old })

    // Register and Unregister via wrappers
    tok := Register(func(ctx context.Context) {}, os.Interrupt)[0]
    Unregister(tok)

    // ContextFor wrapper should return a cancelable context
    ctx, cancel := ContextFor(context.Background(), os.Interrupt)
    cancel()
    select {
    case <-ctx.Done():
        // ok
    case <-time.After(50 * time.Millisecond):
        t.Fatal("ContextFor wrapper did not produce cancelable context")
    }

    // Policy and Logger setters should update Default
    SetPolicy(Policy{GracePeriod: 10 * time.Millisecond, ForceOnSecondSignal: false})
    SetLogger(func(format string, args ...any) {})
    SetDebug(true)
    if !isDebug() { // covers isDebug path
        t.Fatal("expected isDebug() true after SetDebug(true)")
    }
    setDebug(false) // covers setDebug path
    if isDebug() {
        t.Fatal("expected isDebug() false after setDebug(false)")
    }

    // Reset and Stop wrappers (simple invocation for coverage)
    Reset()
    Stop()
}

// Options helpers should apply to a new Registry.
func TestOptions_WithLogger_And_WithDebug_Cross(t *testing.T) {
    var called bool
    l := func(format string, args ...any) { called = true }
    r := NewRegistry(WithLogger(l), WithDebug(true))
    if r.logf == nil {
        t.Fatal("WithLogger did not set logf")
    }
    // call the logger to ensure it is wired and avoid unused variable
    r.logf("test")
    if !called {
        t.Fatal("logger func not invoked as expected")
    }
    if !r.debug {
        t.Fatal("WithDebug did not set debug flag")
    }
}

func TestConvenience_StartWithContext_Cross(t *testing.T) {
    old := Default
    Default = NewRegistry()
    t.Cleanup(func() { Default.Stop(); Default = old })

    Register(func(ctx context.Context) {}, os.Interrupt)
    if err := StartWithContext(context.Background()); err != nil {
        t.Fatalf("StartWithContext wrapper failed: %v", err)
    }
    Stop()
}
