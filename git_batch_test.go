package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBatchStageAndUnstage(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	writeAndCommit(t, dir, "a.cs", "a\n", "feat: a", time.Now(), "Alice")
	writeAndCommit(t, dir, "b.cs", "b\n", "feat: b", time.Now(), "Alice")
	writeAndCommit(t, dir, "c.cs", "c\n", "feat: c", time.Now(), "Alice")

	if err := os.WriteFile(filepath.Join(dir, "a.cs"), []byte("a2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.cs"), []byte("b2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "d.cs"), []byte("d\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := GitService{}
	if err := svc.BatchStageFiles(dir, []string{"a.cs", "b.cs", "d.cs"}); err != nil {
		t.Fatalf("BatchStageFiles failed: %v", err)
	}
	status, err := svc.GetRepoStatus(dir)
	if err != nil {
		t.Fatalf("GetRepoStatus failed: %v", err)
	}
	if len(status.StagedFiles) != 3 {
		t.Fatalf("expected 3 staged files, got %d", len(status.StagedFiles))
	}
	if len(status.UnstagedFiles) != 0 || len(status.UntrackedFiles) != 0 {
		t.Fatalf("expected clean unstaged lists, got %+v / %+v", status.UnstagedFiles, status.UntrackedFiles)
	}

	if err := svc.BatchUnstageFiles(dir, []string{"a.cs", "d.cs"}); err != nil {
		t.Fatalf("BatchUnstageFiles failed: %v", err)
	}
	status2, err := svc.GetRepoStatus(dir)
	if err != nil {
		t.Fatalf("second GetRepoStatus failed: %v", err)
	}
	if len(status2.StagedFiles) != 1 || status2.StagedFiles[0].Path != "b.cs" {
		t.Fatalf("expected only b.cs staged, got %+v", status2.StagedFiles)
	}
	if len(status2.UnstagedFiles) != 1 || len(status2.UntrackedFiles) != 1 {
		t.Fatalf("expected a.cs unstaged and d.cs untracked, got %+v / %+v", status2.UnstagedFiles, status2.UntrackedFiles)
	}

	// Empty input is a no-op.
	if err := svc.BatchUnstageFiles(dir, nil); err != nil {
		t.Fatalf("empty batch unstage should not fail: %v", err)
	}
}
