# SPEC: go-signals

Status: Draft v2.0
Author(s): Steve Rozzo
Audience: Module users, forkers, and maintainers
Scope: Define precise behavior, API, and guarantees for a minimal, zero-dependency signal helper in Go.

---

## 1. Overview

`go-signals` is a small, deterministic abstraction for handling OS signals in Go. It focuses on orderly shutdown, predictable handler ordering, bounded coalescing when multiple signals arrive, and a clear **force** path that cancels work via context when grace elapses or a second terminating signal is received.

The package exposes both an instance API (`*Registry`) and a convenience singleton (`signals.Default` with top-level helpers). All public APIs are safe for concurrent use. Global configuration setters are concurrency-safe and may take effect on subsequent dispatch cycles (values are snapshotted per cycle).

---

## 2. Goals

1. Provide a tiny, explicit API for registering process-signal handlers.
2. Make shutdown behavior predictable: explicit ordering, grace timeout, and second-signal policy.
3. Integrate cleanly with `context.Context` and typical server shutdown flows.
4. Be testable and race-free with clear failure modes and minimal logging hooks.
5. Keep zero external dependencies and work on major OSes supported by Go.

### Non-goals
- Competing with the standard library for raw `os/signal` primitives.
- Providing a lifecycle framework or service container.
- Cross-process orchestration or supervision trees.
- Reliable counting/queueing of signals (signals are notifications, not messages).

---

## 3. Key Concepts
- **Signal**: An `os.Signal` delivered by the OS to the process.
- **Handler**: A function invoked by the library in response to a signal, receiving a cancelable `context.Context`.
- **Registry**: Owns handlers and policy; manages subscription and the dispatch loop once started.
- **Grace period**: Duration for orderly shutdown before forcing cancellation.
- **Second-signal policy**: Whether a second terminating signal during shutdown forces immediately.

---

## 4. Supported Platforms
- **Unix**: Linux, macOS, BSD. Any platform signal supported by Go can be registered.
- **Windows**: Limited by Go’s runtime; `os.Interrupt` and `syscall.SIGTERM` are meaningful (and treated as “terminating”). Others are ignored.
- If a requested signal is not supported on the current platform, registration is a no-op; a debug log may be emitted if debug is enabled.

---

## 5. Public API

### 5.1 Types

```go
package signals

import "time"

// Handler is the function signature for signal callbacks.
type Handler func(ctx context.Context)

type Policy struct {
    // If > 0, starting a shutdown arms a grace timer that will force-cancel
    // handler contexts when it fires.
    GracePeriod time.Duration

    // If true, receiving a second terminating signal while already shutting down
    // forces cancellation immediately.
    ForceOnSecondSignal bool

    // If true, panics in handlers are recovered and logged via the logger.
    // When false, panics are still recovered but not logged.
    LogPanics bool
}

// LoggerFunc is a printf-style logger.
type LoggerFunc func(format string, args ...any)

// Token is an opaque handle for a single registration. Pass it to Unregister
type Token struct { /* opaque */ }

// Registry owns handlers and dispatch for a set of signals.
type Registry struct { /* unexported */ }
```

**Default Policy**
- `GracePeriod`: 30s
- `ForceOnSecondSignal`: true
- `LogPanics`: true

Configure per instance with `WithPolicy(Policy)` or globally for the default registry with `SetPolicy(Policy)`.

### 5.2 Construction

```go
func NewRegistry(opts ...Option) *Registry

// Options
func WithPolicy(p Policy) Option
func WithLogger(l LoggerFunc) Option // default is no-op logger
func WithDebug(enabled bool) Option
```

### 5.3 Registration

```go
// Register adds h to the handler list for each signal in sigs and returns
// a Token per signal. Handlers for a given signal execute FIFO in the order
// registered. Safe before or after Start.
func (r *Registry) Register(h Handler, sigs ...os.Signal) []Token

// Unregister removes the handler associated with the token. Unknown tokens are a no-op.
func (r *Registry) Unregister(t Token)

// Reset removes all handlers and clears internal shutdown state (term counters,
// coalescing and pending flags). In-flight handlers are not interrupted.
func (r *Registry) Reset()
```

**Notes**
- Multiple registrations of the same function are allowed and are treated as distinct entries.
- Registration and unregistration are safe for concurrent use.

### 5.4 Lifecycle

```go
// Start subscribes to the registered signals and launches the dispatch loop.
// Errors:
//  - ErrNoSignals           if no signals are registered
//  - ErrAlreadyStarted      if called twice without Stop
//  - "signals: Start after Stop" if called after Stop on this registry
func (r *Registry) Start() error

// StartWithContext delegates to Start. The context does not scope the registry;
// use Stop to end it explicitly.
func (r *Registry) StartWithContext(ctx context.Context) error

// Stop unsubscribes from OS signals, ends the dispatch loop, and force-cancels
// any in-flight handler contexts.
// Semantics:
//  - Stop before Start is a no-op.
//  - After a successful Start and Stop, a later Start returns "signals: Start after Stop".
func (r *Registry) Stop()
```

### 5.5 Context convenience

```go
// ContextFor returns a context canceled when any of the provided signals
// is observed for this process. Independent of Start/Stop; implemented via
// os/signal.NotifyContext.
func (r *Registry) ContextFor(parent context.Context, sigs ...os.Signal) (context.Context, context.CancelFunc)
```

### 5.6 Package-level convenience (Default registry)

```go
var Default *Registry // initialized eagerly at package load

func Register(h Handler, sigs ...os.Signal) []Token
func Unregister(t Token)
func Reset()
func Start() error
func StartWithContext(ctx context.Context) error
func Stop()

// Policy and logging
func SetPolicy(p Policy)
func SetLogger(fn LoggerFunc)
func SetDebug(on bool)
```

---

## 6. Behavioral Semantics

### 6.1 Signal classes
- **Terminating signals** initiate shutdown: `SIGINT`, `SIGTERM`, `SIGQUIT`, and `os.Interrupt`.
  - Windows: `os.Interrupt` and `syscall.SIGTERM` are treated as terminating.
- **Non-terminating signals** such as `SIGUSR1/2` do not start shutdown but dispatch registered handlers.

### 6.2 Dispatch & coalescing
- When a signal arrives and no dispatch for that signal is running, a **dispatch cycle** begins:
  - All handlers for that signal run sequentially (FIFO) in that cycle.
  - Each handler receives a `context.Context` that will be canceled if the force path triggers.
- While a cycle is running for a signal, further arrivals of the same signal set a per-signal **pending** flag (bounded coalescing: at most one pending at any time).
  After the current cycle finishes, if pending was set, one additional cycle runs and clears the flag. If more arrivals occur during that pending cycle, the flag is set again and another cycle runs after the pending cycle completes. This keeps coalescing bounded without queueing individual arrivals.

### 6.3 Shutdown and force
- The first terminating signal transitions the registry to **shutting down**.
- If `Policy.GracePeriod > 0`, a timer starts. When it fires, all handler contexts are canceled (force).
- If `Policy.ForceOnSecondSignal` is true and a second terminating signal arrives while shutting down, cancellation occurs immediately.
- `Stop` always force-cancels contexts.

### 6.4 Panics in handlers
- Panics are recovered so subsequent handlers still run.
- If `Policy.LogPanics` is true, the panic and stack are logged via the configured logger.

### 6.5 Ordering guarantees
- Within a signal, handlers execute in registration order.
- Across different signals, no relative ordering is guaranteed.

### 6.6 Start/Stop state machine
- `Stop()` before `Start()` is a no-op.
- `Start()` twice without `Stop()` returns `ErrAlreadyStarted`.
- `Start()` after `Stop()` on the same registry returns `"signals: Start after Stop"`.

Concurrency note: global configuration (`SetPolicy`, `SetLogger`, `SetDebug`) is safe to call concurrently; values are read under a mutex or snapshotted at dispatch cycle start.

---

## 7. Logging & Debug

```go
SetLogger(func(format string, args ...any))
SetDebug(true|false)
SetPolicy(Policy) // for Default
```

- The logger is an optional printf-style function. By default there is no logging.
- `SetDebug(true)` enables verbose logs for lifecycle, coalescing, shutdown, force, and panic recovery.
- Per-instance logging/debug can be set via options or direct fields where applicable.

---

## 8. Examples

### 8.1 Basic graceful shutdown
```go
r := signals.NewRegistry()
r.Register(func(ctx context.Context) {
    // cleanup work, flushing, closing resources
    <-ctx.Done() // optional: wait for force cancellation
}, os.Interrupt, syscall.SIGTERM)

if err := r.Start(); err != nil { log.Fatal(err) }
defer r.Stop()
```

### 8.2 Second signal forces immediately
```go
r := signals.NewRegistry(signals.WithPolicy(signals.Policy{
    GracePeriod:         30 * time.Second,
    ForceOnSecondSignal: true,
    LogPanics:           true,
}))
r.Register(cleanup, os.Interrupt, syscall.SIGTERM)
_ = r.Start()
```

### 8.3 Using the Default registry
```go
signals.SetPolicy(signals.Policy{GracePeriod: 10 * time.Second, ForceOnSecondSignal: true})
signals.Register(cleanup, os.Interrupt)
if err := signals.Start(); err != nil { log.Fatal(err) }
defer signals.Stop()
```

---

## 9. Errors
- `ErrNoSignals`: `Start` called with zero registered signals.
- `ErrAlreadyStarted`: `Start` called twice without `Stop`.
- `"signals: Start after Stop"`: `Start` called after `Stop` on the same registry.

---

## 10. Testing & Quality

### 10.1 Unit tests
- Cross-platform tests in `signals_test.go`.
- Unix-only tests in `signals_unix_test.go` (`//go:build !windows`).
- Windows-only tests in `signal_windows_test.go` (`//go:build windows`).

Run with coverage:
```bash
go test -coverpkg=github.com/srozzo/go-signals/v2/signals \
  -covermode=atomic -coverprofile=coverage.out ./signals
go tool cover -func=coverage.out | tail -n1
```

### 10.2 Race detector
```bash
go test -race ./signals
```

### 10.3 Fuzz tests
- Deterministic fuzz harnesses avoid real OS signals and drive shutdown via internal methods.
```bash
# Run all Fuzz* targets for ~30s
go test -run=^$ -fuzz=Fuzz -fuzztime=30s ./signals

# Race + fuzz
go test -race -run=^$ -fuzz=Fuzz -fuzztime=30s ./signals
```
- If fuzzing finds a failure, Go writes a minimal reproducer under `testdata/fuzz/<FuzzName>/`. Keep those checked in to prevent regressions.

### 10.4 Coverage targets
- Repository overall: ≥ 85%
- `signals` package: ≥ 95% preferred; acceptability depends on remaining lines and practicality

---

## 11. Performance Considerations
- Handlers execute sequentially per signal to preserve deterministic order. Keep handlers short and non-blocking where possible.
- Coalescing reduces redundant dispatch during long-running cycles.
- The dispatcher runs on a goroutine and consumes from a buffered signal channel to handle bursts.

---

## 12. Limitations
- Signals are notifications, not a reliable queue. Coalescing can intentionally drop intermediate arrivals during an active cycle.
- On Windows the signal model is limited by the runtime. Only `os.Interrupt` and `SIGTERM` participate in termination logic.

---

## 13. Future Extensions
- Per-signal policy overrides (custom grace per signal)
- Pluggable metrics hooks
- Optional parallel handler execution per signal with explicit ordering barriers

---

## 14. Compatibility
- Requires a modern Go toolchain as specified in `go.mod`.
- No external dependencies for core behavior.
