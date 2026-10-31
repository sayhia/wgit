package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// batch3FakeGitScript is a stub `git` covering the flight-recorder and
// three-way-merge surfaces whose ExecGit error handling batch 3 of the swallow
// audit triaged in git_flight_recorder.go and git_three_way_merge.go:
//
//   - `update-ref refs/wgit/snapshots/...` (CreateFlightRecorderSnapshot) — a
//     failed ref pin must surface: git gc would prune the snapshot while the
//     manifest still advertises it, so restore would fail weeks later
//   - `ls-tree` file counting (CreateFlightRecorderSnapshot) — display
//     metadata only, a failure must not discard the captured snapshot (keep)
//   - stage reads `git show :N:<path>` (GetThreeWayMergeData) — a failed stage
//     read degrades to the conflict-block synthesis instead of failing the
//     merge editor (keep)
//
// Every other command is forwarded to the real binary, which is resolved
// before the stub goes on PATH.
const batch3FakeGitScript = `#!/bin/sh
real="$WGIT_B3_REAL_GIT"
mode="$WGIT_B3_MODE"

if [ "$1" = "update-ref" ] && [ "$2" != "-d" ] && [ "$mode" = "update-ref-fail" ]; then
  echo "fatal: cannot lock ref 'refs/wgit/snapshots/snap_test'" >&2
  exit 1
fi

if [ "$1" = "ls-tree" ] && [ "$mode" = "ls-tree-fail" ]; then
  echo "fatal: tree object unreadable" >&2
  exit 1
fi

if [ "$1" = "show" ] && [ "$mode" = "show-fail" ]; then
  echo "fatal: object unreadable" >&2
  exit 1
fi

exec "$real" "$@"
`

// installBatch3FakeGit puts the stub git ahead of the real one; realGit must
// have been resolved with exec.LookPath before this is called.
func installBatch3FakeGit(t *testing.T, realGit, mode string) {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(batch3FakeGitScript), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("WGIT_B3_REAL_GIT", realGit)
	t.Setenv("WGIT_B3_MODE", mode)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// batch3SeedRepo builds a committed repository so the flight recorder has an
// index and a HEAD to snapshot; fixture setup always runs the real git binary
// (shared helpers come from git_lfs_lock_swallow_test.go).
func batch3SeedRepo(t *testing.T, realGit string) string {
	t.Helper()
	repo := t.TempDir()
	runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")
	runSwallowGit(t, realGit, repo, "config", "user.name", "Alice")
	runSwallowGit(t, realGit, repo, "config", "user.email", "alice@studio.dev")
	if err := os.WriteFile(filepath.Join(repo, "level.txt"), []byte("level v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "add", "-A")
	runSwallowGit(t, realGit, repo, "commit", "-q", "-m", "level: seed")
	return repo
}

// batch3SkipForRoot guards the permission-based failure injections: root opens
// unreadable/read-only files regardless of mode, so the injection would not
// fire there.
func batch3SkipForRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission-based failure injection does not apply to root")
	}
}

// TestCreateFlightRecorderSnapshotPropagatesRefPinFailure pins the triaged
// propagation for the snapshot ref pin: a failed `update-ref` must fail the
// capture and leave no manifest entry, because an unpinned snapshot would be
// pruned by git gc while the manifest still advertised it.
func TestCreateFlightRecorderSnapshotPropagatesRefPinFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch3SeedRepo(t, realGit)

	installBatch3FakeGit(t, realGit, "update-ref-fail")

	info, err := (GitService{}).CreateFlightRecorderSnapshot(repo, "pre-reset", "pin fail")
	if err == nil {
		t.Fatalf("failed ref pin must surface, got snapshot %+v", info)
	}
	if !strings.Contains(err.Error(), "failed to pin flight snapshot ref") {
		t.Fatalf("error = %v, want wrapped flight snapshot ref pin failure", err)
	}
	if info != nil {
		t.Errorf("expected no result alongside the error, got %+v", info)
	}
	if _, statErr := os.Stat(getFlightStorePath(repo)); !os.IsNotExist(statErr) {
		t.Errorf("manifest must not be written when the ref pin fails, stat err = %v", statErr)
	}
}

// TestCreateFlightRecorderSnapshotPropagatesManifestReadFailure pins the
// manifest read guard: a real IO error (the store path is a directory here, so
// os.IsNotExist is false) must fail the capture instead of appending to an
// empty list, which would overwrite the manifest and silently drop every
// earlier snapshot entry.
func TestCreateFlightRecorderSnapshotPropagatesManifestReadFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch3SeedRepo(t, realGit)

	if err := os.MkdirAll(getFlightStorePath(repo), 0o755); err != nil {
		t.Fatal(err)
	}

	info, err := (GitService{}).CreateFlightRecorderSnapshot(repo, "manual", "read fail")
	if err == nil {
		t.Fatalf("manifest read failure must surface, got snapshot %+v", info)
	}
	if !strings.Contains(err.Error(), "failed to read flight recorder manifest") {
		t.Fatalf("error = %v, want wrapped manifest read failure", err)
	}
	if info != nil {
		t.Errorf("expected no result alongside the error, got %+v", info)
	}
}

// TestCreateFlightRecorderSnapshotPropagatesManifestWriteFailure pins the
// manifest write guard: a snapshot that never lands in the manifest is
// invisible to the UI and to PurgeFlightSnapshots, so the write failure must
// surface instead of reporting a successful capture.
func TestCreateFlightRecorderSnapshotPropagatesManifestWriteFailure(t *testing.T) {
	batch3SkipForRoot(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch3SeedRepo(t, realGit)

	storePath := getFlightStorePath(repo)
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	// A valid (empty) manifest that reads fine but can no longer be written.
	if err := os.WriteFile(storePath, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(storePath, 0o444); err != nil {
		t.Fatal(err)
	}

	info, err := (GitService{}).CreateFlightRecorderSnapshot(repo, "manual", "write fail")
	if err == nil {
		t.Fatalf("manifest write failure must surface, got snapshot %+v", info)
	}
	if !strings.Contains(err.Error(), "failed to save flight recorder manifest") {
		t.Fatalf("error = %v, want wrapped manifest write failure", err)
	}
	if info != nil {
		t.Errorf("expected no result alongside the error, got %+v", info)
	}
}

// TestCreateFlightRecorderSnapshotPropagatesTempIndexCopyFailure pins the
// temp-index seeding guard: a failed copy starts `add -A` from an empty index,
// so staged deletions and tracked-but-ignored files would silently drop out of
// the snapshot tree. The failure is injected Go-side (unreadable real index),
// so no git stub is involved.
func TestCreateFlightRecorderSnapshotPropagatesTempIndexCopyFailure(t *testing.T) {
	batch3SkipForRoot(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch3SeedRepo(t, realGit)

	if err := os.Chmod(filepath.Join(repo, ".git", "index"), 0o000); err != nil {
		t.Fatal(err)
	}

	info, err := (GitService{}).CreateFlightRecorderSnapshot(repo, "manual", "copy fail")
	if err == nil {
		t.Fatalf("temp index copy failure must surface, got snapshot %+v", info)
	}
	if !strings.Contains(err.Error(), "failed to prepare flight recorder temp index") {
		t.Fatalf("error = %v, want wrapped temp index preparation failure", err)
	}
	if info != nil {
		t.Errorf("expected no result alongside the error, got %+v", info)
	}
}

// TestCreateFlightRecorderSnapshotToleratesLsTreeFailure keeps the triaged
// best-effort behavior: FileCount is display metadata, so a failed `ls-tree`
// must not discard the captured snapshot — FileCount degrades to 0, the entry
// still lands in the manifest, and the pinning ref is still written.
func TestCreateFlightRecorderSnapshotToleratesLsTreeFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch3SeedRepo(t, realGit)

	installBatch3FakeGit(t, realGit, "ls-tree-fail")

	info, err := (GitService{}).CreateFlightRecorderSnapshot(repo, "pre-checkout", "count may fail")
	if err != nil {
		t.Fatalf("file-count failure is display-only and must not fail the capture: %v", err)
	}
	if info == nil || info.CommitHash == "" || info.TreeHash == "" {
		t.Fatalf("snapshot must still be captured, got %+v", info)
	}
	if info.FileCount != 0 {
		t.Errorf("failed ls-tree must degrade FileCount to 0, got %d", info.FileCount)
	}

	list, listErr := (GitService{}).ListFlightSnapshots(repo)
	if listErr != nil || len(list) != 1 || list[0].ID != info.ID {
		t.Fatalf("snapshot must land in the manifest: list=%+v err=%v", list, listErr)
	}
	// The ref pin passes through the stub, so gc cannot prune the snapshot.
	runSwallowGit(t, realGit, repo, "rev-parse", "--verify", "refs/wgit/snapshots/"+info.ID)
}

// TestGetThreeWayMergeDataSynthesizesStreamsWhenStageReadsFail keeps the
// triaged best-effort behavior for the three stage reads: even with real
// merge stages in the index, unreadable stage content degrades to the
// conflict-block synthesis instead of failing the merge editor, while
// CurrentContent stays the on-disk ground truth.
func TestGetThreeWayMergeDataSynthesizesStreamsWhenStageReadsFail(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := t.TempDir()
	runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")
	runSwallowGit(t, realGit, repo, "config", "user.name", "Alice")
	runSwallowGit(t, realGit, repo, "config", "user.email", "alice@studio.dev")
	if err := os.WriteFile(filepath.Join(repo, "level.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "add", "-A")
	runSwallowGit(t, realGit, repo, "commit", "-q", "-m", "base")
	runSwallowGit(t, realGit, repo, "checkout", "-q", "-b", "feature")
	if err := os.WriteFile(filepath.Join(repo, "level.txt"), []byte("theirs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "commit", "-qam", "theirs")
	runSwallowGit(t, realGit, repo, "checkout", "-q", "main")
	if err := os.WriteFile(filepath.Join(repo, "level.txt"), []byte("ours\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "commit", "-qam", "ours")

	// Real merge conflict: stages 1/2/3 all exist in the index.
	mergeCmd := exec.Command(realGit, "merge", "feature")
	mergeCmd.Dir = repo
	mergeCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "LANG=C")
	if out, mergeErr := mergeCmd.CombinedOutput(); mergeErr == nil || !strings.Contains(string(out), "CONFLICT") {
		t.Fatalf("fixture expects a conflicting merge, got err=%v out=%s", mergeErr, out)
	}

	installBatch3FakeGit(t, realGit, "show-fail")

	data, err := (GitService{}).GetThreeWayMergeData(repo, "level.txt")
	if err != nil {
		t.Fatalf("stage read failures degrade to block synthesis, not an error: %v", err)
	}
	if data == nil || data.TotalConflicts != 1 {
		t.Fatalf("conflicts = %+v, want exactly 1 block", data)
	}
	if strings.TrimSpace(data.OursContent) != "ours" {
		t.Errorf("oursContent = %q, want the synthesized ours side", data.OursContent)
	}
	if strings.TrimSpace(data.TheirsContent) != "theirs" {
		t.Errorf("theirsContent = %q, want the synthesized theirs side", data.TheirsContent)
	}
	if !strings.Contains(data.CurrentContent, "<<<<<<<") {
		t.Errorf("currentContent must stay the on-disk conflict file, got %q", data.CurrentContent)
	}
}

// TestCreateFlightRecorderSnapshotWorksOnUnbornHead pins the corrected HEAD
// read: rev-parse exits non-zero on an unborn branch but echoes the literal
// "HEAD" on stdout, which used to flow into `commit-tree -p HEAD` and break
// the capture with an unrelated error. The hash is now cleared on failure so
// the designed no-parent root-commit snapshot is taken.
func TestCreateFlightRecorderSnapshotWorksOnUnbornHead(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := t.TempDir()
	runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")

	info, err := (GitService{}).CreateFlightRecorderSnapshot(repo, "manual", "first snapshot")
	if err != nil {
		t.Fatalf("snapshot on unborn HEAD must succeed as a root commit: %v", err)
	}
	if info == nil || info.CommitHash == "" || info.TreeHash == "" {
		t.Fatalf("root-commit snapshot must be captured, got %+v", info)
	}
	if info.HeadHash != "" {
		t.Errorf("headHash = %q, want empty on an unborn branch", info.HeadHash)
	}
	runSwallowGit(t, realGit, repo, "rev-parse", "--verify", "refs/wgit/snapshots/"+info.ID)
}

// TestGetStudioCIPipelinesToleratesUnbornHead keeps the triaged best-effort
// branch probe: on an unborn HEAD `rev-parse --abbrev-ref HEAD` exits non-zero
// yet git still echoes the literal "HEAD" on stdout, which ExecGit preserves
// next to the error — the panel must degrade to the friendly "未检测到 CI 配置"
// message instead of erroring (older git versions print an empty branch, which
// falls back to "main"; both are the designed degradation).
func TestGetStudioCIPipelinesToleratesUnbornHead(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := t.TempDir()
	runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")

	res, err := (GitService{}).GetStudioCIPipelines(repo)
	if err != nil {
		t.Fatalf("unborn HEAD must degrade, not error: %v", err)
	}
	if res == nil {
		t.Fatal("result must not be nil")
	}
	if res.ActiveBranch != "HEAD" && res.ActiveBranch != "main" {
		t.Errorf("active branch = %q, want the literal HEAD echo or the \"main\" fallback", res.ActiveBranch)
	}
	if !strings.Contains(res.Message, "未检测到 CI 配置") {
		t.Errorf("message = %q, want the friendly no-CI-config degradation", res.Message)
	}
}
