package main

import (
    "context"
    "fmt"
    "os"
    "syscall"
    "time"

    "github.com/srozzo/go-signals/v2/signals"
)

// Demonstrates a worker loop that exits when a signal is received, using
// ContextFor for cancellation and keeping the handler body small.
func main() {
    // Configure a modest grace; force on second signal.
    signals.SetPolicy(signals.Policy{GracePeriod: 5 * time.Second, ForceOnSecondSignal: true, LogPanics: true})
    // Register a tiny handler so Start has at least one signal to subscribe.
    signals.Register(func(hctx context.Context) { <-hctx.Done() }, os.Interrupt, syscall.SIGTERM)

    if err := signals.Start(); err != nil {
        panic(err)
    }
    defer signals.Stop()

    // Context that cancels on Interrupt or SIGTERM.
    ctx, cancel := signals.ContextFor(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    // Kick off a worker loop that respects ctx.Done().
    done := make(chan struct{}, 1)
    go func() {
        ticker := time.NewTicker(50 * time.Millisecond)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                fmt.Println("worker_loop: stopping")
                done <- struct{}{}
                return
            case <-ticker.C:
                // simulate unit of work
                _ = time.Now()
            }
        }
    }()

    // Wait for cancellation and worker to exit.
    <-ctx.Done()
    <-done
    fmt.Println("worker_loop: exited")
}
