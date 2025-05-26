# signals

![GitHub License](https://img.shields.io/github/license/srozzo/signals?style=flat&link=https%3A%2F%2Fraw.githubusercontent.com%2Fsrozzo%2Fsignals%2Frefs%2Fheads%2Fmain%2FLICENSE)
![Go Test](https://img.shields.io/github/actions/workflow/status/srozzo/signals/test.yml?branch=main)
![Coverage](https://img.shields.io/badge/coverage-92.1%25-brightgreen)
![Go Report Card](https://img.shields.io/badge/go%20report-A%2B-brightgreen?logo=go)



A lightweight, thread-safe Unix signal handling module for Go. Designed for clean shutdowns, reloadable config patterns, and diagnostic triggers (e.g., `SIGINT`, `SIGHUP`, `SIGQUIT`) — all with clean abstractions and safe concurrency.

## ✨ Features

- Register multiple handlers per signal
- Caller-controlled context cancellation (flexible lifecycle)
- Debug-friendly with optional structured logging

## 📦 Install

```bash
go get github.com/srozzo/signals
```

## 🚀 Quick Start
```golang 
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/srozzo/signals"
)

func main() {
	// Set up context with cancellation for shutdown coordination
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Enable debug logging (optional)
	signals.SetDebug(true)

	// Handle shutdown signals
	signals.RegisterMany([]os.Signal{syscall.SIGINT, syscall.SIGTERM}, signals.HandlerFunc(func(sig os.Signal) {
		log.Printf("received shutdown signal: %s", sig)
		cancel()
	}))

	// Handle reload (no cancel)
	signals.Register(syscall.SIGHUP, signals.HandlerFunc(func(sig os.Signal) {
		log.Printf("received reload signal: %s", sig)
		// Reload config, re-open logs, etc.
	}))

	// Start signal handling
	if err := signals.Start(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP); err != nil {
		log.Fatalf("failed to start signal dispatcher: %v", err)
	}

	// Define a simple HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("handling request:", r.URL.Path)
		w.Write([]byte("Hello, world!"))
	})

	// Set up HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Run server in background
	go func() {
		log.Println("server starting on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for cancellation
	<-ctx.Done()
	log.Println("shutting down server...")

	// Shutdown HTTP server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	} else {
		log.Println("server shut down cleanly")
	}
}
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