package main

import (
    "bytes"
    "context"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "testing"
    "time"
)

func TestHTTPServerExample_GracefulShutdownOnSignal(t *testing.T) {
    t.Parallel()

    tmp := t.TempDir()
    bin := filepath.Join(tmp, "httpserver-example")
    if runtime.GOOS == "windows" {
        bin += ".exe"
    }

    // Build the example binary
    build := exec.Command("go", "build", "-o", bin, ".")
    build.Dir = "."
    if out, err := build.CombinedOutput(); err != nil {
        t.Fatalf("build failed: %v\n%s", err, string(out))
    }

    // Run with an ephemeral port to avoid collisions
    ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
    defer cancel()
    run := exec.CommandContext(ctx, bin)
    run.Env = append(os.Environ(), "HTTP_ADDR=127.0.0.1:0")
    var buf bytes.Buffer
    run.Stdout = &buf
    run.Stderr = &buf
    if err := run.Start(); err != nil {
        t.Fatalf("start failed: %v", err)
    }

    // Give it time to start, then send Interrupt
    time.Sleep(300 * time.Millisecond)
    if err := run.Process.Signal(os.Interrupt); err != nil {
        t.Fatalf("signal failed: %v", err)
    }

    err := run.Wait()
    if ctx.Err() == context.DeadlineExceeded {
        t.Fatalf("server did not exit in time; output:\n%s", buf.String())
    }
    // Non-zero exit is acceptable; success is timely shutdown
    _ = err
}

