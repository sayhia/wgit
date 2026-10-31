package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// batch5FakeGitScript is a stub `git` covering the closed-loop LFS conversion
// and the notes inspection surface triaged in batch 5 of the swallow audit.
//
// The primary files (git_notes.go, git_ref_compare.go) had zero swallowed
// ExecGit errors, so the sweep moved to the secondary targets:
// git_workspace.go (zero) and git_closed_loop.go (two propagation points in
// TrackStagedLargeFilesAsLfs):
//
//   - `check-attr filter` (TrackStagedLargeFilesAsLfs) — a failed probe must
//     fail the item, not read as "not LFS-tracked" and re-run the conversion
//   - the post-conversion `cat-file -s` verification read — an unreadable
//     staged size must fail the item, not report a "converted" 0 B pointer
//
// The kept site is the `cat-file -p` note-content read in GetCommitNotes: a
// failed read degrades the display fields while the note itself still surfaces.
//
// git-lfs is optional in the sandbox, so `lfs version` is answered by the stub
// and `lfs track` is stood in for: it appends the pattern to .gitattributes so
// the fixture's filter.lfs.clean shrinks the staged blob like the real filter
// would. Every other command is forwarded to the real binary.
const batch5FakeGitScript = `#!/bin/sh
real="$WGIT_B5_REAL_GIT"
mode="$WGIT_B5_MODE"

if [ "$1" = "lfs" ] && [ "$2" = "version" ]; then
  echo "git-lfs/3.7.0 (batch5-stub)"
  exit 0
fi

if [ "$1" = "lfs" ] && [ "$2" = "track" ]; then
  shift 2
  pattern=""
  take=0
  for a in "$@"; do
    if [ "$take" = "1" ]; then
      pattern="$a"
    fi
    if [ "$a" = "--" ]; then
      take=1
    fi
  done
  if [ -n "$pattern" ]; then
    printf '%s filter=lfs diff=lfs merge=lfs -text\n' "$pattern" >> .gitattributes
  fi
  echo "Tracking \"$pattern\""
  if [ "$mode" = "check-attr-fail" ]; then
    # Marker proving the conversion write path ran despite the failed probe.
    touch "$WGIT_B5_MARKER"
  fi
  exit 0
fi

if [ "$1" = "check-attr" ] && [ "$mode" = "check-attr-fail" ]; then
  echo "fatal: attributes probe unavailable (batch5 injection)" >&2
  exit 1
fi

if [ "$1" = "cat-file" ] && [ "$2" = "-s" ] && [ "$mode" = "catfile-second-fail" ]; then
  n=$(cat "$WGIT_B5_COUNT" 2>/dev/null)
  [ -z "$n" ] && n=0
  n=$((n + 1))
  echo "$n" > "$WGIT_B5_COUNT"
  if [ "$n" -ge 2 ]; then
    echo "fatal: size probe unavailable (batch5 injection)" >&2
    exit 1
  fi
fi

if [ "$1" = "cat-file" ] && [ "$2" = "-p" ] && [ "$mode" = "catfile-p-fail" ]; then
  echo "fatal: note blob unreadable (batch5 injection)" >&2
  exit 1
fi

exec "$real" "$@"
`

// installBatch5FakeGit puts the stub git ahead of the real one; realGit must
// have been resolved with exec.LookPath before this is called.
func installBatch5FakeGit(t *testing.T, realGit, mode string) {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(batch5FakeGitScript), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("WGIT_B5_REAL_GIT", realGit)
	t.Setenv("WGIT_B5_MODE", mode)
	t.Setenv("WGIT_B5_MARKER", filepath.Join(t.TempDir(), "lfs-track-invoked"))
	t.Setenv("WGIT_B5_COUNT", filepath.Join(t.TempDir(), "catfile-s-count"))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestSwallowBatch5TrackLfsPropagatesCheckAttrFailure pins the propagation for
// the LFS attribute probe in TrackStagedLargeFilesAsLfs: when `check-attr
// filter` fails, the item must fail with the root cause instead of being
// treated as "not LFS-tracked" — the old swallow would have re-run the whole
// conversion (lfs track / rm --cached / add) on an already-tracked file and
// reported a false "converted".
func TestSwallowBatch5TrackLfsPropagatesCheckAttrFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch3SeedRepo(t, realGit)

	if err := os.WriteFile(filepath.Join(repo, "level.bin"), bytes.Repeat([]byte{0xA5}, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "add", "level.bin")

	installBatch5FakeGit(t, realGit, "check-attr-fail")

	res, err := (GitService{}).TrackStagedLargeFilesAsLfs(repo, []string{"level.bin"}, false)
	if err != nil {
		t.Fatalf("per-item failure must not fail the batch: %v", err)
	}
	if res == nil {
		t.Fatal("result must not be nil")
	}
	if res.Failed != 1 || res.Converted != 0 || res.Skipped != 0 {
		t.Fatalf("counts = converted:%d failed:%d skipped:%d, want 0/1/0", res.Converted, res.Failed, res.Skipped)
	}
	if res.Success {
		t.Error("a failed attribute probe must not mark the batch successful")
	}
	if len(res.Items) != 1 {
		t.Fatalf("items = %+v, want exactly the failed item", res.Items)
	}
	item := res.Items[0]
	if item.Status != "failed" {
		t.Errorf("status = %q, want failed (probe failure must not read as not-tracked)", item.Status)
	}
	if !strings.Contains(item.Detail, "读取 LFS 过滤属性失败") || !strings.Contains(item.Detail, "attributes probe unavailable") {
		t.Errorf("detail = %q, want the root cause from the failed check-attr probe", item.Detail)
	}
	if _, statErr := os.Stat(os.Getenv("WGIT_B5_MARKER")); !os.IsNotExist(statErr) {
		t.Error("lfs track must not run when the attribute probe fails")
	}
	if _, statErr := os.Stat(filepath.Join(repo, ".gitattributes")); !os.IsNotExist(statErr) {
		t.Error(".gitattributes must stay untouched when the attribute probe fails")
	}
}

// TestSwallowBatch5TrackLfsFailsWhenPostConvertSizeUnreadable pins the
// propagation for the post-conversion verification read: a failed
// `cat-file -s` after re-adding must fail the item instead of reporting
// "converted" with a 0 B pointer — an unverified conversion must never be
// presented as a success.
func TestSwallowBatch5TrackLfsFailsWhenPostConvertSizeUnreadable(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch3SeedRepo(t, realGit)

	if err := os.WriteFile(filepath.Join(repo, "level.bin"), bytes.Repeat([]byte{0x5A}, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "add", "level.bin")
	// The stub's `lfs track` registers the pattern; this local clean filter
	// stands in for git-lfs and shrinks the staged blob to a pointer-sized one.
	runSwallowGit(t, realGit, repo, "config", "filter.lfs.clean", "echo fake-lfs-pointer")

	// The first cat-file -s (pre-conversion size) passes through to real git;
	// the second one (post-conversion verification) is injected to fail.
	installBatch5FakeGit(t, realGit, "catfile-second-fail")

	res, err := (GitService{}).TrackStagedLargeFilesAsLfs(repo, []string{"level.bin"}, false)
	if err != nil {
		t.Fatalf("per-item failure must not fail the batch: %v", err)
	}
	if res == nil {
		t.Fatal("result must not be nil")
	}
	if res.Failed != 1 || res.Converted != 0 {
		t.Fatalf("counts = converted:%d failed:%d, want 0/1", res.Converted, res.Failed)
	}
	if res.Success {
		t.Error("an unverified conversion must not mark the batch successful")
	}
	if len(res.Items) != 1 {
		t.Fatalf("items = %+v, want exactly the failed item", res.Items)
	}
	item := res.Items[0]
	if item.Status != "failed" {
		t.Errorf("status = %q, want failed (unreadable verification must not read as converted)", item.Status)
	}
	if item.OldBytes != 2048 {
		t.Errorf("oldBytes = %d, want 2048 (the pre-conversion read must have succeeded)", item.OldBytes)
	}
	if !strings.Contains(item.Detail, "无法确认 LFS 指针") || !strings.Contains(item.Detail, "size probe unavailable") {
		t.Errorf("detail = %q, want the root cause from the failed verification read", item.Detail)
	}
}

// TestSwallowBatch5GetCommitNotesToleratesNoteContentReadFailure keeps the
// triaged best-effort behavior of GetCommitNotes (git_notes.go): a failed
// `cat-file -p` degrades only the Content/Preview display fields, while the
// note itself still surfaces with its hash and metadata — honest degradation,
// not an error and not a fabricated "note is empty" claim.
func TestSwallowBatch5GetCommitNotesToleratesNoteContentReadFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch3SeedRepo(t, realGit)
	runSwallowGit(t, realGit, repo, "notes", "add", "-m", "QA: verified this level", "HEAD")

	installBatch5FakeGit(t, realGit, "catfile-p-fail")

	res, err := (GitService{}).GetCommitNotes(repo, 10)
	if err != nil {
		t.Fatalf("note content read failure is display-only and must not fail the listing: %v", err)
	}
	if res == nil {
		t.Fatal("result must not be nil")
	}
	if res.Total != 1 || len(res.Notes) != 1 {
		t.Fatalf("notes = total:%d len:%d, want the single annotated commit", res.Total, len(res.Notes))
	}
	item := res.Notes[0]
	if item.CommitHash == "" {
		t.Error("commit hash must survive the failed content read")
	}
	if item.ShortHash == "" || !strings.HasPrefix(item.CommitHash, item.ShortHash) {
		t.Errorf("shortHash = %q, want a prefix of %q", item.ShortHash, item.CommitHash)
	}
	if item.Author != "Alice" || item.Subject != "level: seed" {
		t.Errorf("metadata = author:%q subject:%q, want the log-read fields filled", item.Author, item.Subject)
	}
	if item.Content != "" || item.Preview != "" {
		t.Errorf("content/preview = %q/%q, want honest degradation to empty", item.Content, item.Preview)
	}
	if !strings.Contains(res.Summary, "共 1 条提交批注") {
		t.Errorf("summary = %q, want the normal listing summary", res.Summary)
	}
}
