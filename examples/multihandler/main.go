package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/srozzo/go-signals/v2/signals"
)

// Demonstrates FIFO ordering across multiple handlers and panic recovery.
func main() {
    signals.SetLogger(func(format string, args ...any) { log.Printf(format, args...) })
    signals.SetDebug(true)
    signals.SetPolicy(signals.Policy{GracePeriod: 0, ForceOnSecondSignal: true, LogPanics: true})

    signals.Register(func(ctx context.Context) {
        fmt.Println("multihandler: handler #1 (will panic)")
        panic("boom in handler #1")
    }, os.Interrupt)

    signals.Register(func(ctx context.Context) {
        fmt.Println("multihandler: handler #2 (continues after panic)")
    }, os.Interrupt)

    if err := signals.Start(); err != nil {
        log.Fatal(err)
    }
    defer signals.Stop()

    // Trigger Interrupt to self to demonstrate ordering + recovery.
    time.Sleep(200 * time.Millisecond)
    p, _ := os.FindProcess(os.Getpid())
    _ = p.Signal(os.Interrupt)

    // Give handlers time to run in this simple demo.
    time.Sleep(500 * time.Millisecond)
}
