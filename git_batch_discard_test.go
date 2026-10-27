package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newBatchDiscardRepo creates a throwaway repository whose git configuration is
// isolated from the developer's global/system settings.
func newBatchDiscardRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Tester")
	t.Setenv("GIT_AUTHOR_EMAIL", "tester@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Tester")
	t.Setenv("GIT_COMMITTER_EMAIL", "tester@example.com")

	repo := t.TempDir()
	batchDiscardGit(t, repo, "init", "-b", "main")
	batchDiscardGit(t, repo, "config", "commit.gpgsign", "false")
	return repo
}

func batchDiscardGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat", "LC_ALL=en_US.UTF-8")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func writeBatchDiscardFile(t *testing.T, repo, relPath, content string) {
	t.Helper()
	full := filepath.Join(repo, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s failed: %v", relPath, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s failed: %v", relPath, err)
	}
}

func readBatchDiscardFile(t *testing.T, repo, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(relPath)))
	if err != nil {
		t.Fatalf("read %s failed: %v", relPath, err)
	}
	return string(data)
}

func commitBatchDiscardFiles(t *testing.T, repo string, message string, paths ...string) {
	t.Helper()
	args := append([]string{"add", "--"}, paths...)
	batchDiscardGit(t, repo, args...)
	batchDiscardGit(t, repo, "commit", "-m", message)
}

func assertBatchDiscardMissing(t *testing.T, repo, relPath string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(relPath))); !os.IsNotExist(err) {
		t.Fatalf("%s should have been removed, stat err = %v", relPath, err)
	}
}

func TestBatchDiscardFilesRestoresTrackedAndRemovesUntracked(t *testing.T) {
	repo := newBatchDiscardRepo(t)
	writeBatchDiscardFile(t, repo, "tracked.txt", "v1")
	writeBatchDiscardFile(t, repo, "clean.txt", "keep\n")
	commitBatchDiscardFiles(t, repo, "chore: base", "tracked.txt", "clean.txt")

	writeBatchDiscardFile(t, repo, "tracked.txt", "v2")
	writeBatchDiscardFile(t, repo, "untracked.txt", "temp\n")

	svc := GitService{}
	if err := svc.BatchDiscardFiles(repo, []string{"tracked.txt", "untracked.txt", "clean.txt", "   "}); err != nil {
		t.Fatalf("BatchDiscardFiles failed: %v", err)
	}
	if got := readBatchDiscardFile(t, repo, "tracked.txt"); got != "v1" {
		t.Fatalf("tracked file should be restored to v1, got %q", got)
	}
	assertBatchDiscardMissing(t, repo, "untracked.txt")
	if got := readBatchDiscardFile(t, repo, "clean.txt"); got != "keep\n" {
		t.Fatalf("tracked clean file should be untouched, got %q", got)
	}
}

func TestBatchDiscardFilesHandlesPathsWithSpaces(t *testing.T) {
	repo := newBatchDiscardRepo(t)
	const spaced = "Assets/my file.png"
	writeBatchDiscardFile(t, repo, spaced, "v1")
	commitBatchDiscardFiles(t, repo, "chore: asset", spaced)

	writeBatchDiscardFile(t, repo, spaced, "v2")

	svc := GitService{}
	if err := svc.BatchDiscardFiles(repo, []string{spaced}); err != nil {
		t.Fatalf("BatchDiscardFiles failed: %v", err)
	}
	if got := readBatchDiscardFile(t, repo, spaced); got != "v1" {
		t.Fatalf("spaced path should be restored to v1, got %q", got)
	}
}

func TestBatchDiscardFilesTreatsTrackedDirectoryAsTracked(t *testing.T) {
	repo := newBatchDiscardRepo(t)
	writeBatchDiscardFile(t, repo, "Assets/hero.png", "v1")
	commitBatchDiscardFiles(t, repo, "chore: asset", "Assets/hero.png")

	writeBatchDiscardFile(t, repo, "Assets/hero.png", "v2")
	writeBatchDiscardFile(t, repo, "Assets/new.txt", "draft\n")

	svc := GitService{}
	if err := svc.BatchDiscardFiles(repo, []string{"Assets"}); err != nil {
		t.Fatalf("BatchDiscardFiles failed: %v", err)
	}
	if got := readBatchDiscardFile(t, repo, "Assets/hero.png"); got != "v1" {
		t.Fatalf("tracked file under directory should be restored, got %q", got)
	}
	if got := readBatchDiscardFile(t, repo, "Assets/new.txt"); got != "draft\n" {
		t.Fatalf("untracked file under directory must survive restore, got %q", got)
	}
}

func TestBatchDiscardFilesEmptyInputIsNoOp(t *testing.T) {
	repo := newBatchDiscardRepo(t)
	writeBatchDiscardFile(t, repo, "tracked.txt", "v1")
	commitBatchDiscardFiles(t, repo, "chore: base", "tracked.txt")

	writeBatchDiscardFile(t, repo, "tracked.txt", "v2")
	writeBatchDiscardFile(t, repo, "untracked.txt", "temp\n")

	svc := GitService{}
	for _, input := range [][]string{nil, {}, {"", "   "}} {
		if err := svc.BatchDiscardFiles(repo, input); err != nil {
			t.Fatalf("BatchDiscardFiles(%v) should be a no-op, got %v", input, err)
		}
	}
	if got := readBatchDiscardFile(t, repo, "tracked.txt"); got != "v2" {
		t.Fatalf("empty batch must not restore tracked file, got %q", got)
	}
	if got := readBatchDiscardFile(t, repo, "untracked.txt"); got != "temp\n" {
		t.Fatalf("empty batch must not delete untracked file, got %q", got)
	}
}
