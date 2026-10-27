package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// oversizedDiffFixtureBytes stays above maxSourceTextBytes (5 MB) so the guard
// trips while remaining small enough for a fast test.
const oversizedDiffFixtureBytes = 6 * 1024 * 1024

// setupSourceDiffSizeRepo builds an isolated git repo so host-level config
// (signing, hooks, templates) cannot influence these size-boundary tests.
func setupSourceDiffSizeRepo(t *testing.T) (string, func(args ...string) string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := ExecGit(repo, args...)
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
		return out
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")
	run("config", "commit.gpgsign", "false")
	return repo, run
}

// largeTextFile renders at least targetBytes of newline-terminated text. The
// fill character differs per version so the two blobs share no lines and a
// real diff would be as large as the files themselves.
func largeTextFile(targetBytes int, fill byte) string {
	const lineLen = 64
	line := strings.Repeat(string(fill), lineLen-1) + "\n"
	lines := targetBytes / lineLen
	return strings.Repeat(line, lines)
}

func writeSourceDiffFile(t *testing.T, repo string, relPath string, content string) {
	t.Helper()
	fullPath := filepath.Join(repo, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func requireSkippedDiffError(t *testing.T, diff string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected oversized diff to be rejected, got %d bytes of patch", len(diff))
	}
	if !strings.Contains(err.Error(), "跳过差异") {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != "" {
		t.Fatalf("rejected diff must not return payload, got %d bytes", len(diff))
	}
}

// TestGetSourceFileDiffRejectsOversizedBlobs covers both entry points: the
// working-tree mode (pinned by the on-disk size and the HEAD blob) and the
// revision mode (pinned by the commit blob and its parent).
func TestGetSourceFileDiffRejectsOversizedBlobs(t *testing.T) {
	repo, run := setupSourceDiffSizeRepo(t)
	const relPath = "Assets/Big.txt"

	writeSourceDiffFile(t, repo, relPath, largeTextFile(oversizedDiffFixtureBytes, 'a'))
	run("add", "-A")
	run("commit", "-qm", "big v1")

	// Same size, different content: both the worktree copy and HEAD exceed the
	// cap, so neither diff may be materialized.
	writeSourceDiffFile(t, repo, relPath, largeTextFile(oversizedDiffFixtureBytes, 'b'))
	run("add", "-A")
	run("commit", "-qm", "big v2")

	// Modify the working tree again so the working-mode check sees a huge
	// on-disk file on top of the huge HEAD blob.
	writeSourceDiffFile(t, repo, relPath, largeTextFile(oversizedDiffFixtureBytes, 'c'))

	svc := GitService{}
	diff, err := svc.GetSourceFileDiff(repo, relPath, "")
	requireSkippedDiffError(t, diff, err)

	diff, err = svc.GetSourceFileDiff(repo, relPath, "working")
	requireSkippedDiffError(t, diff, err)

	// HEAD has a parent here, so the revision guard is exercised with both
	// sides known.
	diff, err = svc.GetSourceFileDiff(repo, relPath, "HEAD")
	requireSkippedDiffError(t, diff, err)
}

// TestGetSourceFileDiffRejectsOversizedUntrackedFile makes sure the working
// size check alone is enough when there is no HEAD blob to consult.
func TestGetSourceFileDiffRejectsOversizedUntrackedFile(t *testing.T) {
	repo, run := setupSourceDiffSizeRepo(t)
	writeSourceDiffFile(t, repo, "notes.txt", "tracked\n")
	run("add", "-A")
	run("commit", "-qm", "base")

	writeSourceDiffFile(t, repo, "Untracked.txt", largeTextFile(oversizedDiffFixtureBytes, 'x'))

	diff, err := GitService{}.GetSourceFileDiff(repo, "Untracked.txt", "")
	requireSkippedDiffError(t, diff, err)
}

// TestGetSourceFileDiffRejectsOversizedParentBlob checks that the best-effort
// parent lookup still blocks a patch that would have to delete a huge blob.
func TestGetSourceFileDiffRejectsOversizedParentBlob(t *testing.T) {
	repo, run := setupSourceDiffSizeRepo(t)
	const relPath = "Assets/Shrunk.txt"

	writeSourceDiffFile(t, repo, relPath, largeTextFile(oversizedDiffFixtureBytes, 'p'))
	run("add", "-A")
	run("commit", "-qm", "big")

	writeSourceDiffFile(t, repo, relPath, "now tiny\n")
	run("add", "-A")
	run("commit", "-qm", "tiny")

	diff, err := GitService{}.GetSourceFileDiff(repo, relPath, "HEAD")
	requireSkippedDiffError(t, diff, err)
}

// TestGetSourceFileDiffSmallFileStillReturnsPatch is the regression guard: the
// size probe must not disturb the normal working-tree or revision patches.
func TestGetSourceFileDiffSmallFileStillReturnsPatch(t *testing.T) {
	repo, run := setupSourceDiffSizeRepo(t)
	const relPath = "src/main.go"

	writeSourceDiffFile(t, repo, relPath, "package main\n\nfunc main() {\n\tprintln(\"old\")\n}\n")
	run("add", "-A")
	run("commit", "-qm", "add main")

	svc := GitService{}
	unchangedDiff, err := svc.GetSourceFileDiff(repo, relPath, "")
	if err != nil {
		t.Fatalf("unchanged working diff must not fail: %v", err)
	}
	if unchangedDiff != "" {
		t.Fatalf("expected no working diff for an unchanged file, got %q", unchangedDiff)
	}

	writeSourceDiffFile(t, repo, relPath, "package main\n\nfunc main() {\n\tprintln(\"new\")\n}\n")
	workingDiff, err := svc.GetSourceFileDiff(repo, relPath, "")
	if err != nil {
		t.Fatalf("working diff after edit failed: %v", err)
	}
	if !strings.Contains(workingDiff, "diff --git") || !strings.Contains(workingDiff, "+") {
		t.Fatalf("expected working modification patch, got %q", workingDiff)
	}

	revDiff, err := svc.GetSourceFileDiff(repo, relPath, "HEAD")
	if err != nil {
		t.Fatalf("revision diff failed: %v", err)
	}
	if !strings.Contains(revDiff, "diff --git") || !strings.Contains(revDiff, "+package main") {
		t.Fatalf("expected HEAD patch for the commit that added the file, got %q", revDiff)
	}
}
