package main

import (
    "context"
    "fmt"
    "os"
    "syscall"
    "time"

    "github.com/srozzo/go-signals/v2/signals"
)

// Basic example using the Default helpers. It registers a shutdown handler
// and waits for os.Interrupt or SIGTERM.
func main() {
    // Optional: configure policy (30s grace by default)
    signals.SetPolicy(signals.Policy{GracePeriod: 10 * time.Second, ForceOnSecondSignal: true, LogPanics: true})

    // Handler runs in FIFO order per signal. It will observe cancellation
    // if force occurs (grace timer or second signal).
    signals.Register(func(ctx context.Context) {
        fmt.Println("basic: cleanup started")
        select {
        case <-ctx.Done():
            fmt.Println("basic: forced cancellation")
        case <-time.After(500 * time.Millisecond):
            fmt.Println("basic: graceful completion")
        }
    }, os.Interrupt, syscall.SIGTERM)

    if err := signals.Start(); err != nil {
        panic(err)
    }
    defer signals.Stop()

    // Wait for either Interrupt or SIGTERM
    ctx, cancel := signals.ContextFor(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()
    <-ctx.Done()
    fmt.Println("basic: signal observed; exiting")
}
