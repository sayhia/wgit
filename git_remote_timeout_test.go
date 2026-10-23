package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRemoteConnectionHonorsTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX git wrapper")
	}
	binDir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = \"ls-remote\" ]; then /bin/sleep 5; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	oldTimeout := remoteTestTimeout
	remoteTestTimeout = 150 * time.Millisecond
	defer func() { remoteTestTimeout = oldTimeout }()

	start := time.Now()
	res, err := (GitService{}).TestRemoteConnection(t.TempDir(), "origin")
	if err != nil {
		t.Fatalf("TestRemoteConnection failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("remote test should time out quickly, took %v", elapsed)
	}
	if res.Success || !strings.Contains(res.Error, "timed out") {
		t.Fatalf("timeout should be reported without success: %+v", res)
	}
}
