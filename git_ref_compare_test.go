package main

import (
	"fmt"
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

func TestBuildDiffComparisonReportsPatchFailure(t *testing.T) {
	repo := newRefCompareRepo(t)

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "--patch" ]; then
    echo "simulated patch failure" >&2
    exit 128
  fi
done
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := (GitService{}).GetMergeBaseDiff(repo, "main", "feature"); err == nil {
		t.Fatal("patch read failure must return an error instead of an empty diff")
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

func newPatchRangeRepo(t *testing.T) (string, []string) {
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
	run("checkout", "-q", "-b", "main")

	var hashes []string
	write("file1.txt", "line1\n")
	run("add", "-A")
	run("commit", "-qm", "feat: commit 1")
	hashes = append(hashes, run("rev-parse", "HEAD"))

	write("file2.txt", "line2\n")
	run("add", "-A")
	run("commit", "-qm", "feat: commit 2")
	hashes = append(hashes, run("rev-parse", "HEAD"))

	write("file3.txt", "line3\n")
	run("add", "-A")
	run("commit", "-qm", "feat: commit 3")
	hashes = append(hashes, run("rev-parse", "HEAD"))

	return dir, hashes
}

func TestGeneratePatchRange(t *testing.T) {
	repo, hashes := newPatchRangeRepo(t)
	svc := GitService{}

	// Range from commit 1 to commit 3 (should include commit 2 and commit 3 changes)
	diff, err := svc.GeneratePatchRange(repo, hashes[0], hashes[2])
	if err != nil {
		t.Fatalf("GeneratePatchRange failed: %v", err)
	}
	if !strings.Contains(diff, "file2.txt") || !strings.Contains(diff, "+line2") {
		t.Fatalf("expected diff to include file2.txt changes, got:\n%s", diff)
	}
	if !strings.Contains(diff, "file3.txt") || !strings.Contains(diff, "+line3") {
		t.Fatalf("expected diff to include file3.txt changes, got:\n%s", diff)
	}
	if strings.Contains(diff, "file1.txt") {
		t.Fatalf("diff should not include file1.txt: %s", diff)
	}

	// Range from commit 2 to commit 3
	diff2, err := svc.GeneratePatchRange(repo, hashes[1], hashes[2])
	if err != nil {
		t.Fatalf("GeneratePatchRange(c2..c3) failed: %v", err)
	}
	if strings.Contains(diff2, "file2.txt") {
		t.Fatalf("diff2 should not include file2.txt: %s", diff2)
	}
	if !strings.Contains(diff2, "file3.txt") || !strings.Contains(diff2, "+line3") {
		t.Fatalf("expected diff2 to include file3.txt changes, got:\n%s", diff2)
	}

	// Validation tests
	if _, err := svc.GeneratePatchRange("", hashes[0], hashes[2]); err == nil {
		t.Fatal("expected error for empty repoPath")
	}
	if _, err := svc.GeneratePatchRange(repo, "", hashes[2]); err == nil {
		t.Fatal("expected error for empty startHash")
	}
	if _, err := svc.GeneratePatchRange(repo, hashes[0], ""); err == nil {
		t.Fatal("expected error for empty endHash")
	}
	if _, err := svc.GeneratePatchRange(repo, "--flag", hashes[2]); err == nil {
		t.Fatal("expected error for flag starting with -")
	}
	if _, err := svc.GeneratePatchRange(repo, hashes[0], "-invalid"); err == nil {
		t.Fatal("expected error for flag starting with -")
	}
	if _, err := svc.GeneratePatchRange(repo, "nonexistent999", hashes[2]); err == nil {
		t.Fatal("expected error for non-existent commit ref")
	}
}

func TestExportPatchRange(t *testing.T) {
	repo, hashes := newPatchRangeRepo(t)
	svc := GitService{}

	tmpDir := t.TempDir()
	patchPath := filepath.Join(tmpDir, "nested", "commits.patch")

	// Export commits 2 and 3 (hashes[0]..hashes[2])
	err := svc.ExportPatchRange(repo, hashes[0], hashes[2], patchPath)
	if err != nil {
		t.Fatalf("ExportPatchRange failed: %v", err)
	}

	patchBytes, err := os.ReadFile(patchPath)
	if err != nil {
		t.Fatalf("failed to read exported patch: %v", err)
	}
	patchStr := string(patchBytes)
	if !strings.Contains(patchStr, "feat: commit 2") || !strings.Contains(patchStr, "feat: commit 3") {
		t.Fatalf("patch content missing commit subjects:\n%s", patchStr)
	}
	if !strings.Contains(patchStr, "file2.txt") || !strings.Contains(patchStr, "file3.txt") {
		t.Fatalf("patch content missing changed files:\n%s", patchStr)
	}

	// Verify the exported patch can be applied cleanly on a fresh clone at commit 1
	cloneDir := filepath.Join(tmpDir, "clone")
	if _, err := ExecGit(tmpDir, "clone", repo, cloneDir); err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	if err := svc.ResetBranch(cloneDir, hashes[0], "hard"); err != nil {
		t.Fatalf("reset to commit 1 failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cloneDir, "file2.txt")); !os.IsNotExist(err) {
		t.Fatalf("file2.txt should not exist before patch apply")
	}
	if err := svc.ApplyPatch(cloneDir, patchPath); err != nil {
		t.Fatalf("ApplyPatch with exported range patch failed: %v", err)
	}
	data2, err := os.ReadFile(filepath.Join(cloneDir, "file2.txt"))
	if err != nil || string(data2) != "line2\n" {
		t.Fatalf("file2.txt content mismatch: %q, err=%v", string(data2), err)
	}
	data3, err := os.ReadFile(filepath.Join(cloneDir, "file3.txt"))
	if err != nil || string(data3) != "line3\n" {
		t.Fatalf("file3.txt content mismatch: %q, err=%v", string(data3), err)
	}

	// Validation tests
	if err := svc.ExportPatchRange("", hashes[0], hashes[2], patchPath); err == nil {
		t.Fatal("expected error for empty repoPath")
	}
	if err := svc.ExportPatchRange(repo, "", hashes[2], patchPath); err == nil {
		t.Fatal("expected error for empty startHash")
	}
	if err := svc.ExportPatchRange(repo, hashes[0], "", patchPath); err == nil {
		t.Fatal("expected error for empty endHash")
	}
	if err := svc.ExportPatchRange(repo, hashes[0], hashes[2], ""); err == nil {
		t.Fatal("expected error for empty outputPath")
	}
	if err := svc.ExportPatchRange(repo, "--flag", hashes[2], patchPath); err == nil {
		t.Fatal("expected error for flag ref")
	}
	if err := svc.ExportPatchRange(repo, hashes[0], "-flag", patchPath); err == nil {
		t.Fatal("expected error for flag ref")
	}
	if err := svc.ExportPatchRange(repo, hashes[0], hashes[2], tmpDir); err == nil {
		t.Fatal("expected error when outputPath is an existing directory")
	}
}

func TestResetBranchValidation(t *testing.T) {
	repo, hashes := newPatchRangeRepo(t)
	svc := GitService{}

	if err := svc.ResetBranch("", hashes[0], "hard"); err == nil {
		t.Fatal("expected error for empty repoPath")
	}
	if err := svc.ResetBranch(repo, "", "hard"); err == nil {
		t.Fatal("expected error for empty commitHash")
	}
	if err := svc.ResetBranch(repo, "--bad-flag", "hard"); err == nil {
		t.Fatal("expected error for flag commitHash")
	}

	// Valid reset
	if err := svc.ResetBranch(repo, hashes[1], "hard"); err != nil {
		t.Fatalf("ResetBranch failed: %v", err)
	}
	head, err := ExecGit(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	if strings.TrimSpace(head) != hashes[1] {
		t.Fatalf("HEAD not reset to hashes[1]: got %q, want %q", head, hashes[1])
	}
}
