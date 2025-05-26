# signals

[![License](https://img.shields.io/github/license/srozzo/go-signals?style=flat)](https://github.com/srozzo/go-signals/blob/main/LICENSE)
[![Go Test](https://img.shields.io/github/actions/workflow/status/srozzo/go-signals/test.yml?branch=main)](https://github.com/srozzo/go-signals/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/srozzo/go-signals/branch/main/graph/badge.svg)](https://codecov.io/gh/srozzo/go-signals)
[![Go Report Card](https://goreportcard.com/badge/github.com/srozzo/go-signals)](https://goreportcard.com/report/github.com/srozzo/go-signals)


> A lightweight, testable, and thread-safe signal handling library for Go.
A lightweight, thread-safe Unix signal handling module for Go. Designed for clean shutdowns, reloadable config patterns, and diagnostic triggers (e.g., `SIGINT`, `SIGHUP`, `SIGQUIT`) — all with clean abstractions and safe concurrency.

## ✨ Features

- Register multiple handlers per signal
- Caller-controlled context cancellation (flexible lifecycle)
- Debug-friendly with optional structured logging

## 📦 Install

```bash
go get github.com/srozzo/go-signals
```

## 🚀 Quick Start
```golang 
// See examples/main.go
```

## 🧪 Test

To run the tests with verbose output and race detection:

```bash
go test -v -race ./...
```
The signals package includes full test coverage for:
* Signal registration and dispatch
* Custom logging
* Debug mode toggling 
* Safe reset and sync.Once behavior

---

## 🛠️ API

| Function                          | Description                                                  |
|----------------------------------|--------------------------------------------------------------|
| `Register(signal, handler)`      | Registers a handler for a single signal.                    |
| `RegisterMany([]signal, handler)`| Registers a handler for multiple signals.                   |
| `Start(signals...)`              | Starts listening for the specified signals (once only).     |
| `Reset()`                        | Clears all registered handlers and resets state.            |
| `SetDebug(bool)`                 | Enables or disables internal debug logging.                 |
| `SetLogger(func(format, ...any))`| Sets a custom logger (e.g., `log.Printf`, `logrus.Infof`).  |

## 🧱 Contributing

Contributions are welcome! Please:

- Open issues for bugs, ideas, or feature requests
- Submit pull requests with tests and clean commits
- Follow idiomatic Go and avoid unnecessary dependencies

If you're unsure about anything, open a draft PR or start a discussion.

## 📄 License

MIT License © 2025 [Steve Rozzo](https://github.com/srozzo)

See the [LICENSE](LICENSE) file for details.