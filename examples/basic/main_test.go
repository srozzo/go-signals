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

func TestBasicExample_ExitsOnSignal(t *testing.T) {
    t.Parallel()

    tmp := t.TempDir()
    bin := filepath.Join(tmp, "basic-example")
    if runtime.GOOS == "windows" {
        bin += ".exe"
    }

    cmd := exec.Command("go", "build", "-o", bin, ".")
    cmd.Dir = "."
    if out, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("build failed: %v\n%s", err, string(out))
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    run := exec.CommandContext(ctx, bin)
    var buf bytes.Buffer
    run.Stdout = &buf
    run.Stderr = &buf
    if err := run.Start(); err != nil {
        t.Fatalf("start failed: %v", err)
    }

    // Give the program a moment to initialize then send Interrupt.
    time.Sleep(300 * time.Millisecond)
    if err := run.Process.Signal(os.Interrupt); err != nil {
        t.Fatalf("signal failed: %v", err)
    }

    err := run.Wait()
    // On timeout, CommandContext will kill the process; treat as failure.
    if ctx.Err() == context.DeadlineExceeded {
        t.Fatalf("example did not exit in time; output:\n%s", buf.String())
    }
    if err != nil && ctx.Err() == nil {
        // Nonzero exit is acceptable; just continue to checking output
    }

    // Treat successful and timely exit as pass; output may vary by platform buffering.
    _ = strings.Contains // keep import for future assertions if needed
}
