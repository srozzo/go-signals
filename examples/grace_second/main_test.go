package main

import (
    "bytes"
    "context"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
    "time"
)

func TestGraceSecondExample_ForcesOnSecondSignal(t *testing.T) {
    t.Parallel()

    tmp := t.TempDir()
    bin := filepath.Join(tmp, "grace-second-example")
    if runtime.GOOS == "windows" {
        bin += ".exe"
    }

    cmd := exec.Command("go", "build", "-o", bin, ".")
    cmd.Dir = "."
    if out, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("build failed: %v\n%s", err, string(out))
    }

    ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
    defer cancel()
    run := exec.CommandContext(ctx, bin)
    var buf bytes.Buffer
    run.Stdout = &buf
    run.Stderr = &buf
    if err := run.Run(); err != nil && ctx.Err() == nil {
        // Non-zero exit acceptable; we only validate output
    }
    if ctx.Err() == context.DeadlineExceeded {
        t.Fatalf("example timed out; output:\n%s", buf.String())
    }

    out := buf.String()
    if !strings.Contains(out, "grace_second: sent first signal") ||
        !strings.Contains(out, "grace_second: sent second signal") ||
        !strings.Contains(out, "grace_second: forced cancel") {
        t.Fatalf("expected grace_second output, got:\n%s", out)
    }
}

