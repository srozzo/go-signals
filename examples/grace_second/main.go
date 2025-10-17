package main

import (
    "context"
    "fmt"
    "os"
    "syscall"
    "time"

    "github.com/srozzo/go-signals/v2/signals"
)

// Shows grace timer and second-signal force. First signal begins shutdown,
// second signal (of a terminating kind) forces immediate cancellation.
func main() {
    r := signals.NewRegistry(signals.WithPolicy(signals.Policy{
        GracePeriod:         5 * time.Second,
        ForceOnSecondSignal: true,
        LogPanics:           true,
    }))

    r.Register(func(ctx context.Context) {
        fmt.Println("grace_second: handler running; waiting for cancel or work to finish...")
        select {
        case <-ctx.Done():
            fmt.Println("grace_second: forced cancel")
        case <-time.After(10 * time.Second):
            fmt.Println("grace_second: finished before force")
        }
    }, os.Interrupt, syscall.SIGTERM)

    if err := r.Start(); err != nil {
        panic(err)
    }
    defer r.Stop()

    // Start shutdown via os.Interrupt, then force with SIGTERM.
    time.Sleep(200 * time.Millisecond)
    p, _ := os.FindProcess(os.Getpid())
    _ = p.Signal(os.Interrupt)
    fmt.Println("grace_second: sent first signal (Interrupt)")

    time.Sleep(1 * time.Second)
    _ = p.Signal(syscall.SIGTERM)
    fmt.Println("grace_second: sent second signal (SIGTERM) to force")

    // Allow time to observe cancellation in this demo.
    time.Sleep(800 * time.Millisecond)
}
