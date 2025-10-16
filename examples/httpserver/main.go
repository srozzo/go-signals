package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "syscall"
    "time"

    "github.com/srozzo/go-signals/v2/signals"
)

func main() {
    signals.SetLogger(func(format string, args ...any) {
        log.Printf(format, args...)
    })
    signals.SetPolicy(signals.Policy{
        GracePeriod:         15 * time.Second,
        ForceOnSecondSignal: true,
        LogPanics:           true,
    })

    addr := os.Getenv("HTTP_ADDR")
    if addr == "" {
        addr = ":8080"
    }

    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
        _, _ = w.Write([]byte("ok"))
    })
    srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
        log.Println("listening on ", addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

	if err := signals.Start(); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := signals.ContextFor(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	signals.Register(func(hctx context.Context) {
		sdCtx, sdCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer sdCancel()
		_ = srv.Shutdown(sdCtx)
	}, os.Interrupt, syscall.SIGTERM)

	<-ctx.Done()

	// Optional: exit on force path
	signals.Register(func(hctx context.Context) {
		<-hctx.Done()
		os.Exit(1)
	}, os.Interrupt, syscall.SIGTERM)
}
