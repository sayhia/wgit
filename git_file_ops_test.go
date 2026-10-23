package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newFileOpsTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	run("config", "user.email", "tester@example.com")
	run("config", "user.name", "Tester")
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "a.txt", "b.txt")
	run("commit", "-qm", "init")
	return dir
}

func porcelainStatus(t *testing.T, repo string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status failed: %v (%s)", err, string(out))
	}
	return string(out)
}

func TestRenameRepoFileStagesTrackedRename(t *testing.T) {
	repo := newFileOpsTestRepo(t)
	svc := GitService{}
	if err := svc.RenameRepoFile(repo, "a.txt", "docs/renamed.txt"); err != nil {
		t.Fatalf("RenameRepoFile failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("old path should be gone, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "docs", "renamed.txt")); err != nil {
		t.Fatalf("new path missing: %v", err)
	}
	if status := porcelainStatus(t, repo); !strings.Contains(status, "R  a.txt -> docs/renamed.txt") {
		t.Fatalf("rename should be staged, status=%q", status)
	}
}

func TestRenameRepoFileRejectsExistingTargetAndEscape(t *testing.T) {
	repo := newFileOpsTestRepo(t)
	svc := GitService{}
	if err := svc.RenameRepoFile(repo, "a.txt", "b.txt"); err == nil {
		t.Fatal("expected error when target exists")
	}
	if err := svc.RenameRepoFile(repo, "a.txt", "../escape.txt"); err == nil {
		t.Fatal("expected error for escaping target path")
	}
}

func TestDeleteRepoFileHandlesTrackedAndUntracked(t *testing.T) {
	repo := newFileOpsTestRepo(t)
	svc := GitService{}

	// Untracked file: plain filesystem delete.
	untracked := filepath.Join(repo, "scratch.txt")
	if err := os.WriteFile(untracked, []byte("scratch"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteRepoFile(repo, "scratch.txt"); err != nil {
		t.Fatalf("DeleteRepoFile(untracked) failed: %v", err)
	}
	if _, err := os.Stat(untracked); !os.IsNotExist(err) {
		t.Fatalf("untracked file should be deleted, stat err=%v", err)
	}

	// Tracked file: git rm stages the deletion.
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteRepoFile(repo, "a.txt"); err != nil {
		t.Fatalf("DeleteRepoFile(tracked) failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("tracked file should be removed, stat err=%v", err)
	}
	if status := porcelainStatus(t, repo); !strings.Contains(status, "D  a.txt") {
		t.Fatalf("deletion should be staged, status=%q", status)
	}
}

func TestUntrackRepoFileKeepsWorkingTreeFile(t *testing.T) {
	repo := newFileOpsTestRepo(t)
	svc := GitService{}
	if err := svc.UntrackRepoFile(repo, "b.txt"); err != nil {
		t.Fatalf("UntrackRepoFile failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "b.txt")); err != nil {
		t.Fatalf("file should remain on disk: %v", err)
	}
	status := porcelainStatus(t, repo)
	if !strings.Contains(status, "?? b.txt") || !strings.Contains(status, "D  b.txt") {
		t.Fatalf("expected staged deletion + untracked file, status=%q", status)
	}
}

func TestAddGitignorePatternIsIdempotent(t *testing.T) {
	repo := newFileOpsTestRepo(t)
	svc := GitService{}
	if err := svc.AddGitignorePattern(repo, "/build/"); err != nil {
		t.Fatalf("AddGitignorePattern failed: %v", err)
	}
	if err := svc.AddGitignorePattern(repo, "/build/"); err != nil {
		t.Fatalf("AddGitignorePattern second call failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "/build/"); got != 1 {
		t.Fatalf("expected one pattern entry, got %d: %q", got, string(data))
	}
	// Appending to a file without trailing newline must not glue patterns together.
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("/dist"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddGitignorePattern(repo, "/cache/"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(repo, ".gitignore"))
	if !strings.Contains(string(data), "/dist\n/cache/\n") {
		t.Fatalf("newline handling broken: %q", string(data))
	}
}

func TestGitignorePatternRoundTrip(t *testing.T) {
	repo := newFileOpsTestRepo(t)
	svc := GitService{}
	for _, pattern := range []string{"/build/", "/draft-notes.md", "# comment"} {
		if err := svc.AddGitignorePattern(repo, pattern); err != nil {
			t.Fatalf("AddGitignorePattern(%q) failed: %v", pattern, err)
		}
	}
	patterns, err := svc.GetGitignorePatterns(repo)
	if err != nil {
		t.Fatalf("GetGitignorePatterns failed: %v", err)
	}
	if len(patterns) != 3 || patterns[0] != "/build/" || patterns[2] != "# comment" {
		t.Fatalf("unexpected patterns: %#v", patterns)
	}
	if err := svc.RemoveGitignorePattern(repo, "/draft-notes.md"); err != nil {
		t.Fatalf("RemoveGitignorePattern failed: %v", err)
	}
	patterns, _ = svc.GetGitignorePatterns(repo)
	if len(patterns) != 2 || patterns[0] != "/build/" || patterns[1] != "# comment" {
		t.Fatalf("pattern not removed cleanly: %#v", patterns)
	}
	// Removing a missing pattern is a no-op.
	if err := svc.RemoveGitignorePattern(repo, "/missing/"); err != nil {
		t.Fatalf("RemoveGitignorePattern(missing) failed: %v", err)
	}
	// Missing .gitignore file yields an empty list.
	emptyRepo := newFileOpsTestRepo(t)
	if patterns, err := svc.GetGitignorePatterns(emptyRepo); err != nil || len(patterns) != 0 {
		t.Fatalf("expected empty patterns, got %#v err=%v", patterns, err)
	}
}

func TestStageAndUnstageRepoPath(t *testing.T) {
	repo := newFileOpsTestRepo(t)
	svc := GitService{}
	if err := os.MkdirAll(filepath.Join(repo, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "pkg", "one.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageRepoPath(repo, "pkg"); err != nil {
		t.Fatalf("StageRepoPath failed: %v", err)
	}
	if status := porcelainStatus(t, repo); !strings.Contains(status, "A  pkg/one.txt") {
		t.Fatalf("directory changes should be staged, status=%q", status)
	}
	if err := svc.UnstageRepoPath(repo, "pkg"); err != nil {
		t.Fatalf("UnstageRepoPath failed: %v", err)
	}
	if status := porcelainStatus(t, repo); !strings.Contains(status, "?? pkg/") {
		t.Fatalf("directory changes should be unstaged, status=%q", status)
	}
	if err := svc.StageRepoPath(repo, "../escape"); err == nil {
		t.Fatal("expected validation error for escaping path")
	}
	if err := svc.UnstageRepoPath(repo, ""); err == nil {
		t.Fatal("expected validation error for empty path")
	}
}

func TestUpdateGitignorePattern(t *testing.T) {
	repo := newFileOpsTestRepo(t)
	svc := GitService{}
	for _, pattern := range []string{"/build/", "*.log"} {
		if err := svc.AddGitignorePattern(repo, pattern); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.UpdateGitignorePattern(repo, "*.log", "*.tmp"); err != nil {
		t.Fatalf("UpdateGitignorePattern failed: %v", err)
	}
	patterns, _ := svc.GetGitignorePatterns(repo)
	if len(patterns) != 2 || patterns[1] != "*.tmp" {
		t.Fatalf("pattern not updated: %#v", patterns)
	}
	if err := svc.UpdateGitignorePattern(repo, "*.tmp", "/build/"); err == nil {
		t.Fatal("expected duplicate-rule error")
	}
	if err := svc.UpdateGitignorePattern(repo, "/missing/", "x"); err == nil {
		t.Fatal("expected not-found error")
	}
	if err := svc.UpdateGitignorePattern(repo, "*.tmp", "a\nb"); err == nil {
		t.Fatal("expected single-line validation error")
	}
}

func TestSwapGitignorePatterns(t *testing.T) {
	repo := newFileOpsTestRepo(t)
	svc := GitService{}
	for _, pattern := range []string{"*.log", "!keep.log"} {
		if err := svc.AddGitignorePattern(repo, pattern); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.SwapGitignorePatterns(repo, "*.log", "!keep.log"); err != nil {
		t.Fatalf("SwapGitignorePatterns failed: %v", err)
	}
	patterns, _ := svc.GetGitignorePatterns(repo)
	if len(patterns) != 2 || patterns[0] != "!keep.log" || patterns[1] != "*.log" {
		t.Fatalf("patterns not swapped: %#v", patterns)
	}
	if err := svc.SwapGitignorePatterns(repo, "*.log", "missing"); err == nil {
		t.Fatal("expected not-found error for missing second pattern")
	}
	if err := svc.SwapGitignorePatterns(repo, "", "*.log"); err == nil {
		t.Fatal("expected empty-pattern validation error")
	}
}
