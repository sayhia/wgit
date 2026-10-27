package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// setupPruneErrorRepo creates a real repository with one commit, isolated from
// the user's global git configuration.
func setupPruneErrorRepo(t *testing.T) (GitService, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := filepath.Join(t.TempDir(), "main-repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	steps := [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "tester@example.com"},
		{"config", "user.name", "Tester"},
		{"add", "base.txt"},
		{"commit", "-qm", "base"},
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range steps {
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	return GitService{}, repo
}

// newOrphanWorktreeTarget creates a non-empty directory that is not a git
// worktree under a dedicated parent, then applies perm to that parent.
func newOrphanWorktreeTarget(t *testing.T, perm os.FileMode) (parent, target string) {
	t.Helper()
	parent = filepath.Join(t.TempDir(), "worktrees")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	target = filepath.Join(parent, "orphan-wt")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "leftover.txt"), []byte("leftover\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, perm); err != nil {
		t.Fatal(err)
	}
	// Registered after t.TempDir's cleanup, so it runs first and unblocks removal.
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
	return parent, target
}

func TestPruneSingleWorktreeReportsDirectoryRemovalFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions are not enforced on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses read-only directory permissions")
	}

	svc, repo := setupPruneErrorRepo(t)
	parent, target := newOrphanWorktreeTarget(t, 0o555)

	err := svc.PruneSingleWorktree(repo, target)
	if err == nil {
		t.Fatalf("expected error when the directory cannot be removed, got nil")
	}
	if msg := err.Error(); !strings.Contains(msg, "删除目录失败") {
		t.Fatalf("error should mention directory deletion failure, got %q", msg)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("target should still exist after failed removal: %v", statErr)
	}

	// The main worktree must still be rejected.
	if err := svc.PruneSingleWorktree(repo, repo); err == nil || !strings.Contains(err.Error(), "main worktree") {
		t.Fatalf("expected main worktree rejection, got %v", err)
	}

	// Restoring permissions makes a retry succeed and actually remove the directory.
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := svc.PruneSingleWorktree(repo, target); err != nil {
		t.Fatalf("retry after restoring permissions failed: %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("target should be gone after successful retry, stat err=%v", statErr)
	}
}

func TestPruneSingleWorktreeWritableParentRemovesOrphanDirectory(t *testing.T) {
	svc, repo := setupPruneErrorRepo(t)
	_, target := newOrphanWorktreeTarget(t, 0o755)

	if err := svc.PruneSingleWorktree(repo, target); err != nil {
		t.Fatalf("expected success with writable parent, got %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("orphan directory should be removed, stat err=%v", statErr)
	}
}
