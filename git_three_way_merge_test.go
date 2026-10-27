package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseThreeWayBlocks(t *testing.T) {
	lines := []string{
		"header line",
		"<<<<<<< HEAD",
		"local change 1",
		"local change 2",
		"||||||| base",
		"common ancestor",
		"=======",
		"remote change 1",
		">>>>>>> branch-b",
		"footer line",
	}

	blocks := parseThreeWayBlocks(lines)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 conflict block, got %d", len(blocks))
	}

	b := blocks[0]
	if b.StartLine != 1 || b.EndLine != 8 {
		t.Errorf("expected block range 1..8, got %d..%d", b.StartLine, b.EndLine)
	}

	if strings.Join(b.OursLines, "\n") != "local change 1\nlocal change 2" {
		t.Errorf("unexpected ours: %v", b.OursLines)
	}

	if strings.Join(b.BaseLines, "\n") != "common ancestor" {
		t.Errorf("unexpected base: %v", b.BaseLines)
	}

	if strings.Join(b.TheirsLines, "\n") != "remote change 1" {
		t.Errorf("unexpected theirs: %v", b.TheirsLines)
	}
}

func TestThreeWayMergeDataAndSaveInTempRepo(t *testing.T) {
	tempDir := t.TempDir()
	svc := GitService{}

	// Initialize git repo
	if _, err := ExecGit(tempDir, "init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	_ = os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("line 1\n<<<<<<< HEAD\nours text\n=======\ntheirs text\n>>>>>>> feature\nline 3"), 0o644)

	data, err := svc.GetThreeWayMergeData(tempDir, "test.txt")
	if err != nil {
		t.Fatalf("GetThreeWayMergeData failed: %v", err)
	}

	if data.TotalConflicts != 1 {
		t.Fatalf("expected 1 conflict, got %d", data.TotalConflicts)
	}

	if !strings.Contains(data.OursContent, "ours text") {
		t.Errorf("expected ours content to contain 'ours text', got %q", data.OursContent)
	}

	if !strings.Contains(data.TheirsContent, "theirs text") {
		t.Errorf("expected theirs content to contain 'theirs text', got %q", data.TheirsContent)
	}

	// Save resolved content
	resolved := "line 1\nresolved text\nline 3"
	ok, err := svc.SaveThreeWayMergeResult(tempDir, "test.txt", resolved, true)
	if err != nil || !ok {
		t.Fatalf("SaveThreeWayMergeResult failed: %v", err)
	}

	saved, _ := os.ReadFile(filepath.Join(tempDir, "test.txt"))
	if string(saved) != resolved {
		t.Fatalf("expected file to have resolved content, got %q", string(saved))
	}
}

func TestCreatePartialStashInTempRepo(t *testing.T) {
	tempDir := t.TempDir()
	svc := GitService{}

	if _, err := ExecGit(tempDir, "init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	_ = os.WriteFile(filepath.Join(tempDir, "a.txt"), []byte("initial a"), 0o644)
	_ = os.WriteFile(filepath.Join(tempDir, "b.txt"), []byte("initial b"), 0o644)
	_, _ = ExecGit(tempDir, "add", ".")
	_, _ = ExecGit(tempDir, "-c", "user.name=Test", "-c", "user.email=t@e.st", "commit", "-m", "init")

	// Modify both
	_ = os.WriteFile(filepath.Join(tempDir, "a.txt"), []byte("modified a"), 0o644)
	_ = os.WriteFile(filepath.Join(tempDir, "b.txt"), []byte("modified b"), 0o644)

	_, err := svc.CreatePartialStash(tempDir, "stash only a", []string{"a.txt"})
	if err != nil {
		t.Fatalf("CreatePartialStash failed: %v", err)
	}

	// a.txt should be reverted to initial
	curA, _ := os.ReadFile(filepath.Join(tempDir, "a.txt"))
	if string(curA) != "initial a" {
		t.Errorf("expected a.txt to be stashed back to initial, got %q", string(curA))
	}

	// b.txt should still be modified
	curB, _ := os.ReadFile(filepath.Join(tempDir, "b.txt"))
	if string(curB) != "modified b" {
		t.Errorf("expected b.txt to remain modified, got %q", string(curB))
	}
}

func TestRepoWatcherLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	svc := GitService{}

	started, err := svc.StartRepoWatcher(tempDir)
	if err != nil || !started {
		t.Fatalf("StartRepoWatcher failed: %v", err)
	}

	// Idempotent start
	started2, _ := svc.StartRepoWatcher(tempDir)
	if !started2 {
		t.Errorf("expected re-start to be idempotent")
	}

	stopped := svc.StopRepoWatcher()
	if !stopped {
		t.Errorf("expected StopRepoWatcher to succeed")
	}
}
