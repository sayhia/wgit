package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDiagnoseGitAuthHonorsSSHTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX ssh wrapper")
	}
	t.Setenv("HOME", t.TempDir())
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "ssh"), []byte("#!/bin/sh\n/bin/sleep 5\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	oldTimeout := gitAuthSSHTimeout
	gitAuthSSHTimeout = 150 * time.Millisecond
	defer func() { gitAuthSSHTimeout = oldTimeout }()

	start := time.Now()
	res, err := (GitService{}).DiagnoseGitAuth(t.TempDir(), "example.com")
	if err != nil {
		t.Fatalf("DiagnoseGitAuth failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("SSH diagnosis should time out quickly, took %v", elapsed)
	}
	if res.SSHAuthSuccess || !strings.Contains(res.SSHAuthOutput, "超时") {
		t.Fatalf("timeout should be reported without success: %+v", res)
	}
}
