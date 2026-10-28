package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSafetyNetDiscardAndRestoreInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-safety-net-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (output: %s)", args, err, string(out))
		}
	}

	runCmd("init", "-b", "main")
	runCmd("config", "user.name", "Test User")
	runCmd("config", "user.email", "test@example.com")

	// 1. Initial commit
	file1 := filepath.Join(tempDir, "file1.txt")
	_ = os.WriteFile(file1, []byte("version 1\n"), 0644)
	runCmd("add", ".")
	runCmd("commit", "-m", "init")

	// 2. Modify file1, and create untracked file2
	_ = os.WriteFile(file1, []byte("version 1 modified uncommitted\n"), 0644)
	file2 := filepath.Join(tempDir, "file2.txt")
	_ = os.WriteFile(file2, []byte("untracked file2 content\n"), 0644)

	svc := &GitService{}

	// 3. Discard with safety net
	snap, err := svc.DiscardWithSafetyNet(tempDir, []string{"file1.txt", "file2.txt"})
	if err != nil {
		t.Fatalf("DiscardWithSafetyNet failed: %v", err)
	}
	if snap.Count != 2 {
		t.Fatalf("expected 2 files backed up, got: %d", snap.Count)
	}

	// 4. Verify file1 restored to HEAD and file2 is gone
	content1, _ := os.ReadFile(file1)
	if string(content1) != "version 1\n" {
		t.Fatalf("expected file1 to be reverted to version 1, got: %s", string(content1))
	}
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Fatalf("expected file2 to be removed by clean, but it exists")
	}

	// 5. Restore from safety net
	if err := svc.RestoreFromSafetyNet(tempDir, snap.ID); err != nil {
		t.Fatalf("RestoreFromSafetyNet failed: %v", err)
	}

	// 6. Verify contents restored!
	content1Restored, _ := os.ReadFile(file1)
	if string(content1Restored) != "version 1 modified uncommitted\n" {
		t.Fatalf("expected file1 to be restored to modified content, got: %s", string(content1Restored))
	}
	content2Restored, err := os.ReadFile(file2)
	if err != nil {
		t.Fatalf("expected file2 to be restored, but read failed: %v", err)
	}
	if string(content2Restored) != "untracked file2 content\n" {
		t.Fatalf("expected file2 content restored, got: %s", string(content2Restored))
	}
}
