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
		_, _ = w.Write([]byte("Hello, world!"))
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
			os.Exit(1)
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
