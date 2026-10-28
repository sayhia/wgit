package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetFileCommitRangeDiffInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-tl-range-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runCmd := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v (output: %s)", args, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	runCmd("init", "-b", "main")
	runCmd("config", "user.name", "Test User")
	runCmd("config", "user.email", "test@example.com")

	codeFile := filepath.Join(tempDir, "source.py")
	_ = os.WriteFile(codeFile, []byte("def hello():\n    return 'v1'\n"), 0644)
	runCmd("add", ".")
	hash1 := runCmd("commit", "-m", "v1")
	_ = hash1
	hash1 = runCmd("rev-parse", "HEAD")

	_ = os.WriteFile(codeFile, []byte("def hello():\n    return 'v2'\n"), 0644)
	runCmd("commit", "-am", "v2")
	hash2 := runCmd("rev-parse", "HEAD")

	svc := &GitService{}
	diffRes, err := svc.GetFileCommitRangeDiff(tempDir, "source.py", hash1, hash2)
	if err != nil {
		t.Fatalf("GetFileCommitRangeDiff failed: %v", err)
	}
	if len(diffRes.Files) != 1 || diffRes.Files[0].Path != "source.py" {
		t.Fatalf("expected 1 file (source.py), got: %+v", diffRes.Files)
	}
	if !strings.Contains(diffRes.Diff, "v2") {
		t.Fatalf("expected diff to contain v2, got: %s", diffRes.Diff)
	}
}
