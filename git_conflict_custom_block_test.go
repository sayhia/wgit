package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newCustomBlockRepo(t *testing.T) (string, GitService) {
	t.Helper()
	repo := t.TempDir()
	svc := GitService{}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@example.com")
	return repo, svc
}

func writeCustomBlockFile(t *testing.T, repo, name, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(repo, name)
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

const threeBlockConflictFile = `before
<<<<<<< HEAD
ours-one
=======
theirs-one
>>>>>>> side
middle
<<<<<<< HEAD
ours-two
=======
theirs-two
>>>>>>> side
tail
<<<<<<< HEAD
ours-three
=======
theirs-three
>>>>>>> side
after
`

func TestResolveConflictBlockWithCustomContentKeepsOtherBlocks(t *testing.T) {
	repo, svc := newCustomBlockRepo(t)
	path := writeCustomBlockFile(t, repo, "conflict.txt", threeBlockConflictFile, 0o644)

	res, err := svc.ResolveConflictBlockWithCustomContent(
		repo, "conflict.txt", 1, "merged-two\n", "ours-two", "theirs-two", "",
	)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if res.Stale || !res.Success || res.BlockIndex != 1 {
		t.Fatalf("result = %#v", res)
	}
	if res.RemainingConflicts != 2 || res.FileAutoStaged {
		t.Fatalf("remaining/staged = %d/%v", res.RemainingConflicts, res.FileAutoStaged)
	}

	data, _ := os.ReadFile(path)
	got := string(data)
	if !strings.Contains(got, "ours-one") || !strings.Contains(got, "theirs-one") ||
		!strings.Contains(got, "ours-three") || !strings.Contains(got, "theirs-three") {
		t.Fatalf("other blocks were modified:\n%s", got)
	}
	if !strings.Contains(got, "middle\nmerged-two\ntail") {
		t.Fatalf("target block was not replaced cleanly:\n%s", got)
	}
	if strings.Count(got, "<<<<<<<") != 2 {
		t.Fatalf("expected 2 markers left, got %d", strings.Count(got, "<<<<<<<"))
	}
}

func TestResolveConflictBlockWithCustomContentPreservesCRLFAndMode(t *testing.T) {
	repo, svc := newCustomBlockRepo(t)
	raw := strings.ReplaceAll("head\n<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> side\ntail\n", "\n", "\r\n")
	path := writeCustomBlockFile(t, repo, "crlf.txt", raw, 0o750)

	res, err := svc.ResolveConflictBlockWithCustomContent(
		repo, "crlf.txt", 0, "merged\r\nsecond", "ours", "theirs", "",
	)
	if err != nil || !res.Success || res.Stale {
		t.Fatalf("resolve = %#v err=%v", res, err)
	}

	data, _ := os.ReadFile(path)
	got := string(data)
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Fatalf("file contains bare LF after CRLF resolution: %q", got)
	}
	if !strings.Contains(got, "head\r\nmerged\r\nsecond\r\ntail\r\n") {
		t.Fatalf("unexpected CRLF output: %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Fatalf("mode = %o, want 750", info.Mode().Perm())
	}
}

func TestResolveConflictBlockWithCustomContentRejectsStaleAdvice(t *testing.T) {
	repo, svc := newCustomBlockRepo(t)
	path := writeCustomBlockFile(t, repo, "conflict.txt", threeBlockConflictFile, 0o644)
	before, _ := os.ReadFile(path)

	res, err := svc.ResolveConflictBlockWithCustomContent(
		repo, "conflict.txt", 1, "merged", "different-ours", "theirs-two", "",
	)
	if err != nil {
		t.Fatalf("stale check should not be an error: %v", err)
	}
	if !res.Stale || res.Success {
		t.Fatalf("result = %#v, want stale", res)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("stale advice modified the file")
	}
}

func TestResolveConflictBlockWithCustomContentRejectsShiftedIndex(t *testing.T) {
	repo, svc := newCustomBlockRepo(t)
	path := writeCustomBlockFile(t, repo, "conflict.txt", threeBlockConflictFile, 0o644)

	if res, err := svc.ResolveConflictBlockWithCustomContent(
		repo, "conflict.txt", 0, "merged-one", "ours-one", "theirs-one", "",
	); err != nil || !res.Success {
		t.Fatalf("first resolve = %#v err=%v", res, err)
	}
	before, _ := os.ReadFile(path)

	// Block two has shifted to index one after the first block was removed.
	// The stale expectations must be rejected instead of applying to the wrong block.
	res, err := svc.ResolveConflictBlockWithCustomContent(
		repo, "conflict.txt", 1, "merged-two", "ours-two", "theirs-two", "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Stale {
		t.Fatalf("result = %#v, want stale for shifted index", res)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("shifted-index advice modified the file")
	}
}

func TestResolveConflictBlockWithCustomContentVerifiesDiff3Base(t *testing.T) {
	repo, svc := newCustomBlockRepo(t)
	content := "start\n<<<<<<< HEAD\nours\n||||||| base\ncommon\n=======\ntheirs\n>>>>>>> side\nend\n"
	path := writeCustomBlockFile(t, repo, "diff3.txt", content, 0o644)

	res, err := svc.ResolveConflictBlockWithCustomContent(
		repo, "diff3.txt", 0, "merged", "ours", "theirs", "wrong-base",
	)
	if err != nil || !res.Stale {
		t.Fatalf("base mismatch = %#v err=%v, want stale", res, err)
	}

	res, err = svc.ResolveConflictBlockWithCustomContent(
		repo, "diff3.txt", 0, "merged\n", "ours", "theirs", "common",
	)
	if err != nil || !res.Success || res.Stale {
		t.Fatalf("base match = %#v err=%v", res, err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "start\nmerged\nend\n" {
		t.Fatalf("unexpected resolved content: %q", data)
	}
}

func TestResolveConflictBlockWithCustomContentAutoStagesOnlyWhenClean(t *testing.T) {
	repo, svc := newCustomBlockRepo(t)
	writeCustomBlockFile(t, repo, "one.txt", "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> side\n", 0o644)

	res, err := svc.ResolveConflictBlockWithCustomContent(
		repo, "one.txt", 0, "merged\n", "ours", "theirs", "",
	)
	if err != nil || !res.Success || res.Stale {
		t.Fatalf("resolve = %#v err=%v", res, err)
	}
	if !res.FileAutoStaged || res.RemainingConflicts != 0 {
		t.Fatalf("result = %#v, want auto-staged with no conflicts", res)
	}
	index, err := ExecGit(repo, "ls-files", "--stage", "--", "one.txt")
	if err != nil || strings.TrimSpace(index) == "" {
		t.Fatalf("file was not staged: %q err=%v", index, err)
	}

	writeCustomBlockFile(t, repo, "two.txt", threeBlockConflictFile, 0o644)
	res, err = svc.ResolveConflictBlockWithCustomContent(
		repo, "two.txt", 0, "merged\n", "ours-one", "theirs-one", "",
	)
	if err != nil || !res.Success || res.Stale {
		t.Fatalf("second resolve = %#v err=%v", res, err)
	}
	if res.FileAutoStaged {
		t.Fatal("file with remaining markers was auto-staged")
	}
	index, _ = ExecGit(repo, "ls-files", "--stage", "--", "two.txt")
	if strings.TrimSpace(index) != "" {
		t.Fatalf("unresolved file was staged: %q", index)
	}
}

func TestResolveConflictBlockWithCustomContentRejectsInvalidInputs(t *testing.T) {
	repo, svc := newCustomBlockRepo(t)
	writeCustomBlockFile(t, repo, "conflict.txt", threeBlockConflictFile, 0o644)
	before, _ := os.ReadFile(filepath.Join(repo, "conflict.txt"))

	res, err := svc.ResolveConflictBlockWithCustomContent(
		repo, "conflict.txt", 99, "merged", "ours-one", "theirs-one", "",
	)
	if err != nil {
		t.Fatalf("out-of-range index should be a stale result: %v", err)
	}
	if !res.Stale || res.Success {
		t.Fatalf("out-of-range result = %#v", res)
	}

	if _, err := svc.ResolveConflictBlockWithCustomContent(
		repo, "../escape.txt", 0, "merged", "", "", "",
	); err == nil {
		t.Fatal("path escape was accepted")
	}

	after, _ := os.ReadFile(filepath.Join(repo, "conflict.txt"))
	if string(before) != string(after) {
		t.Fatal("invalid input modified the file")
	}
}
