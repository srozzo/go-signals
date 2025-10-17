# signals

[![License](https://img.shields.io/github/license/srozzo/go-signals?style=flat)](https://github.com/srozzo/go-signals/blob/main/LICENSE)
[![Go Test](https://img.shields.io/github/actions/workflow/status/srozzo/go-signals/test.yml?branch=main)](https://github.com/srozzo/go-signals/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/srozzo/go-signals/branch/main/graph/badge.svg)](https://codecov.io/gh/srozzo/go-signals)
[![Go Report Card](https://goreportcard.com/badge/github.com/srozzo/go-signals)](https://goreportcard.com/report/github.com/srozzo/go-signals)

A small, deterministic, dependency‑free signal helper for Go. It provides ordered handler dispatch, bounded coalescing, and clear shutdown semantics with a force path on grace timeout or a second terminating signal.

## Features

- FIFO handler order per signal (deterministic)
- Grace period and second‑signal force cancellation
- Bounded coalescing (at most one pending cycle at a time)
- Panic recovery with optional logging
- `context.Context` handed to handlers for cancellation
- Instance API plus `Default` convenience helpers
- Cross‑platform: Unix and Windows

## Install

```bash
go get github.com/srozzo/go-signals/v2
```

## Versioning

This module uses semantic import versioning. For v2 and later, import paths must include `/v2`:

```go
import "github.com/srozzo/go-signals/v2/signals"
```
If you need the older v1 series, depend on a v1 tag and import without `/v2`.

## Quick Start

Instance API
```go
package main

import (
    "context"
    "os"
    "syscall"
    "time"
    "github.com/srozzo/go-signals/v2/signals"
)

func main() {
    r := signals.NewRegistry(signals.WithPolicy(signals.Policy{
        GracePeriod:         30 * time.Second,
        ForceOnSecondSignal: true,
        LogPanics:           true,
    }))
    r.Register(func(ctx context.Context) { <-ctx.Done() /* cleanup */ }, os.Interrupt, syscall.SIGTERM)
    _ = r.Start()
    defer r.Stop()
}
```

Default helpers
```go
signals.SetPolicy(signals.Policy{GracePeriod: 10 * time.Second, ForceOnSecondSignal: true})
signals.Register(func(ctx context.Context) { /* ... */ }, os.Interrupt)
if err := signals.Start(); err != nil { panic(err) }
defer signals.Stop()
```

## Examples

- examples/basic: Minimal Default usage; register a shutdown handler and wait on `ContextFor`.
- examples/multihandler: Demonstrates FIFO ordering and panic recovery.
- examples/grace_second: Shows grace timer and second-signal forcing.
- examples/httpserver: Graceful HTTP shutdown with signal-triggered `Server.Shutdown`.
- examples/worker_loop: Worker loop that exits cleanly on signal using `ContextFor`.

## API (selected)

Instance
```go
type Handler func(ctx context.Context)
type Token struct{ /* opaque */ }

func NewRegistry(opts ...Option) *Registry
func (*Registry) Register(h Handler, sigs ...os.Signal) []Token
func (*Registry) Unregister(t Token)
func (*Registry) Reset()
func (*Registry) Start() error
func (*Registry) StartWithContext(ctx context.Context) error
func (*Registry) Stop()
func (*Registry) ContextFor(parent context.Context, sigs ...os.Signal) (context.Context, context.CancelFunc)
```

Default convenience
```go
func Register(h Handler, sigs ...os.Signal) []Token
func Unregister(t Token)
func Reset()
func Start() error
func StartWithContext(ctx context.Context) error
func Stop()
func SetPolicy(p Policy)
func SetLogger(fn LoggerFunc)
func SetDebug(on bool)
```

## Test & Fuzz

With the Makefile:
```bash
make test           # race tests for ./signals
make cover          # writes coverage.out and prints summary
make fuzz FUZZTIME=20s  # fuzz all fuzzers sequentially
```

Manual examples:
```bash
GOCACHE="$(pwd)/.gocache" go test -race ./signals
GOCACHE="$(pwd)/.gocache" go test ./signals -run FuzzStateMachine -fuzz=StateMachine -fuzztime=30s
```

## Design

- Dispatch loop
  - Per-signal FIFO execution. Handlers are snapshotted at cycle start to preserve order even if registrations change during dispatch.
  - Panic recovery around each handler; optional logging via `Policy.LogPanics`.
- Bounded coalescing
  - While a signal is being processed, further arrivals set a per-signal pending flag (no unbounded queue).
  - After the current cycle, one additional cycle runs if pending was set. If more arrivals occur during that pending cycle, another cycle will run afterward. This keeps coalescing bounded but responsive under bursts.
- Shutdown and force
  - First terminating signal begins shutdown; optional grace timer cancels handler contexts when it fires.
  - If `ForceOnSecondSignal` is true, a second terminating signal forces cancellation immediately.
  - `Stop` always cancels handler contexts and ends the loop.
- Contexts
  - Each handler receives a context canceled on the force path. `ContextFor` uses `os/signal.NotifyContext` and is independent of Start/Stop.
- Concurrency
  - All APIs are safe for concurrent use. Global setters (`SetPolicy`, `SetLogger`, `SetDebug`) are synchronized and their values are snapshotted at dispatch cycle start to avoid races.

## Platform Notes (Windows)

- On Windows, only `os.Interrupt` and `syscall.SIGTERM` are treated as meaningful/terminating.
- Registrations for other signals are ignored (a debug log may be emitted if `SetDebug(true)` is enabled).
- On Unix-like systems, the common terminating set includes `os.Interrupt`, `SIGTERM`, `SIGINT`, and `SIGQUIT`.

## Contributing

Contributions are welcome!
- Open issues for bugs, ideas, or feature requests
- Submit PRs with tests and clean commits
- Follow idiomatic Go; avoid unnecessary dependencies

## License

MIT License © 2025 [Steve Rozzo](https://github.com/srozzo)

See the [LICENSE](LICENSE) file for details.
