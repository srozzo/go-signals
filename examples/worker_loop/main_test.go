package main

import (
    "bytes"
    "context"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
    "time"
)

func TestWorkerLoopExample_StopsOnSignal(t *testing.T) {
    t.Parallel()

    tmp := t.TempDir()
    bin := filepath.Join(tmp, "worker-loop-example")
    if runtime.GOOS == "windows" {
        bin += ".exe"
    }

    build := exec.Command("go", "build", "-o", bin, ".")
    build.Dir = "."
    if out, err := build.CombinedOutput(); err != nil {
        t.Fatalf("build failed: %v\n%s", err, string(out))
    }

    ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
    defer cancel()
    run := exec.CommandContext(ctx, bin)
    var buf bytes.Buffer
    run.Stdout = &buf
    run.Stderr = &buf
    if err := run.Start(); err != nil {
        t.Fatalf("start failed: %v", err)
    }

    time.Sleep(300 * time.Millisecond)
    if err := run.Process.Signal(os.Interrupt); err != nil {
        t.Fatalf("signal failed: %v", err)
    }

    err := run.Wait()
    if ctx.Err() == context.DeadlineExceeded {
        t.Fatalf("example timed out; output:\n%s", buf.String())
    }
    _ = err // a non-zero exit is acceptable

    // Treat timely exit as success; output can vary across platforms and buffering.
    _ = strings.Contains // keep import for potential future assertions
}
