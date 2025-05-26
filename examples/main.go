//go:build ignore

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/srozzo/go-signals"
)

func main() {
	// Set up shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Optional: enable internal debug logging
	signals.SetDebug(true)

	// Optional: set a custom logger
	signals.SetLogger(func(format string, args ...any) {
		log.Printf("[custom-logger] "+format, args...)
	})

	// Optional: configure fallback behavior
	signals.SetConfig(&signals.Config{
		CancelOnUnhandled: false, // Set true to cancel ctx on unhandled signals
		DefaultHandler: func(sig os.Signal) {
			log.Printf("default handler: received unregistered signal %s", sig)
		},
	})

	// Register graceful shutdown for SIGINT and SIGTERM
	signals.RegisterMany([]os.Signal{syscall.SIGINT, syscall.SIGTERM}, signals.HandlerFunc(func(sig os.Signal) {
		log.Printf("graceful shutdown signal received: %s", sig)
		cancel()
	}))

	// Register a reload handler (e.g., SIGHUP)
	signals.Register(syscall.SIGHUP, signals.HandlerFunc(func(sig os.Signal) {
		log.Printf("received %s: reload configuration", sig)
		// Add config reload logic here if needed
	}))

	// Start signal dispatcher
	if err := signals.Start(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP); err != nil {
		log.Fatalf("failed to start signal dispatcher: %v", err)
	}

	// HTTP server setup
	server := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("handling request: %s", r.URL.Path)
			_, _ = w.Write([]byte("Hello from go-signals example!\n"))
		}),
	}

	// Start HTTP server in background
	go func() {
		log.Println("HTTP server listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for signal-triggered context cancellation
	<-ctx.Done()
	log.Println("shutting down server...")

	// Gracefully shut down the server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("error during server shutdown: %v", err)
	} else {
		log.Println("server shut down cleanly")
	}
}
