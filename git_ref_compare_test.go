package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newRefCompareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	run("config", "user.email", "tester@example.com")
	run("config", "user.name", "Tester")
	// Normalize the default branch name so tests do not depend on init.defaultBranch.
	run("checkout", "-q", "-b", "main")
	write("base.txt", "base\n")
	run("add", "-A")
	run("commit", "-qm", "base")
	run("branch", "feature")
	write("main-only.txt", "main\n")
	run("add", "-A")
	run("commit", "-qm", "main only")
	run("checkout", "-q", "feature")
	write("feature-only.txt", "feature\n")
	run("add", "-A")
	run("commit", "-qm", "feature only")
	return dir
}

func diffPaths(res *DiffComparisonResult) map[string]bool {
	out := map[string]bool{}
	for _, file := range res.Files {
		out[file.Path] = true
	}
	return out
}

func TestGetMergeBaseDiffUsesThreeDotRange(t *testing.T) {
	repo := newRefCompareRepo(t)
	svc := GitService{}

	threeDot, err := svc.GetMergeBaseDiff(repo, "main", "feature")
	if err != nil {
		t.Fatalf("GetMergeBaseDiff failed: %v", err)
	}
	paths := diffPaths(threeDot)
	if !paths["feature-only.txt"] {
		t.Fatalf("three-dot diff should include feature-only.txt: %#v", paths)
	}
	if paths["main-only.txt"] {
		t.Fatalf("three-dot diff must not include main-only.txt: %#v", paths)
	}

	twoDot, err := svc.GetDiffBetween(repo, "main", "feature")
	if err != nil {
		t.Fatalf("GetDiffBetween failed: %v", err)
	}
	twoPaths := diffPaths(twoDot)
	if !twoPaths["feature-only.txt"] || !twoPaths["main-only.txt"] {
		t.Fatalf("two-dot diff should include both sides: %#v", twoPaths)
	}
	if !strings.Contains(threeDot.Diff, "feature-only.txt") {
		t.Fatalf("three-dot patch preview missing feature file")
	}

	if _, err := svc.GetMergeBaseDiff(repo, "", "feature"); err == nil {
		t.Fatal("expected validation error for empty ref")
	}
}

func TestGetFileDiffBetween(t *testing.T) {
	repo := newRefCompareRepo(t)
	svc := GitService{}

	patch, err := svc.GetFileDiffBetween(repo, "main", "feature", "feature-only.txt", false)
	if err != nil {
		t.Fatalf("GetFileDiffBetween failed: %v", err)
	}
	if !strings.Contains(patch, "+feature") {
		t.Fatalf("expected feature-only patch, got: %q", patch)
	}

	patch, err = svc.GetFileDiffBetween(repo, "main", "feature", "feature-only.txt", true)
	if err != nil {
		t.Fatalf("GetFileDiffBetween(mergeBase) failed: %v", err)
	}
	if !strings.Contains(patch, "feature-only.txt") {
		t.Fatalf("expected merge-base patch, got: %q", patch)
	}

	if _, err := svc.GetFileDiffBetween(repo, "", "feature", "x", false); err == nil {
		t.Fatal("expected validation error for empty ref")
	}
	if _, err := svc.GetFileDiffBetween(repo, "main", "feature", "", false); err == nil {
		t.Fatal("expected validation error for empty path")
	}
}
