package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// lfsSafetyFakeGitScript is a stub `git` that only owns the LFS surface used by
// CheckRemoteLocksConflict / UnlockStaleLfsLocks. Everything else (status,
// rev-parse, commits, config user.email, ...) is forwarded to the real binary,
// which is resolved before the stub is put on PATH.
//
// Modes (WGIT_FAKE_LFS_MODE):
//   - "" / "default": healthy lock inventory, unlocks succeed
//   - "bad-json":      `lfs locks --json` prints non-JSON, plain fallback is empty
//   - "unavailable":   every `lfs` call fails like a missing git-lfs install
//   - "query-fail":    every `lfs locks` call fails with a remote error
//   - "locks-hard-fail": `lfs locks --json` exits 1 with non-JSON and the plain
//     fallback exits 1 too, so no lock inventory can be read at all
//   - "version-fail":  `lfs version` fails, everything else healthy
//
// When WGIT_FAKE_GIT_LOG is set, every `lfs ...` invocation is appended to it
// so tests can assert that mutating lock commands were never attempted.
const lfsSafetyFakeGitScript = `#!/bin/sh
real="$WGIT_REAL_GIT"
mode="$WGIT_FAKE_LFS_MODE"

if [ "$1" = "lfs" ]; then
  if [ -n "$WGIT_FAKE_GIT_LOG" ]; then
    echo "$*" >> "$WGIT_FAKE_GIT_LOG"
  fi
  case "$2" in
    version)
      if [ "$mode" = "version-fail" ]; then
        echo "git-lfs: command not found" >&2
        exit 127
      fi
      exit 0
      ;;
    locks)
      if [ "$mode" = "unavailable" ]; then
        echo "git: 'lfs' is not a git command. See 'git --help'." >&2
        exit 1
      fi
      if [ "$mode" = "query-fail" ]; then
        echo "fatal: unable to read lock list from remote" >&2
        exit 128
      fi
      if [ "$mode" = "locks-hard-fail" ]; then
        if [ "$3" = "--json" ]; then
          echo "lock inventory unavailable"
        else
          echo "fatal: unable to read lock list from remote" >&2
        fi
        exit 1
      fi
      if [ "$3" = "--json" ]; then
        if [ "$mode" = "bad-json" ]; then
          echo "lock inventory unavailable"
        else
          printf '%s\n' "$WGIT_FAKE_LOCKS_JSON"
        fi
        exit 0
      fi
      # Plain-text fallback: report no locks.
      exit 0
      ;;
    unlock)
      target=""
      for arg in "$@"; do
        target="$arg"
      done
      case "$target" in
        *fail*)
          echo "Unable to unlock $target: lock is owned by another user" >&2
          exit 1
          ;;
      esac
      exit 0
      ;;
  esac
fi

if [ "$1" = "config" ] && [ "$2" = "user.name" ]; then
  echo "me"
  exit 0
fi

exec "$real" "$@"
`

// installLfsSafetyFakeGit puts the stub git ahead of the real one. realGit must
// have been resolved with exec.LookPath before this is called.
func installLfsSafetyFakeGit(t *testing.T, realGit, mode, locksJSON string) {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(lfsSafetyFakeGitScript), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("WGIT_REAL_GIT", realGit)
	t.Setenv("WGIT_FAKE_LFS_MODE", mode)
	t.Setenv("WGIT_FAKE_LOCKS_JSON", locksJSON)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// lfsSafetyGit runs the real git binary directly so repository setup never
// depends on the stub that is installed later.
func lfsSafetyGit(t *testing.T, realGit, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command(realGit, args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=Alice",
		"GIT_AUTHOR_EMAIL=alice@studio.dev",
		"GIT_COMMITTER_NAME=Alice",
		"GIT_COMMITTER_EMAIL=alice@studio.dev",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

// newLfsSafetyRepo creates a committed repo with the given tracked files.
func newLfsSafetyRepo(t *testing.T, realGit string, tracked ...string) string {
	t.Helper()
	repo := t.TempDir()
	lfsSafetyGit(t, realGit, repo, "init", "-q", "-b", "main")
	lfsSafetyGit(t, realGit, repo, "config", "user.name", "me")
	lfsSafetyGit(t, realGit, repo, "config", "user.email", "me@studio.dev")
	for _, rel := range tracked {
		full := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte("seed\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	lfsSafetyGit(t, realGit, repo, "add", "-A")
	lfsSafetyGit(t, realGit, repo, "commit", "-q", "--allow-empty", "-m", "chore: seed")
	return repo
}

func TestCheckRemoteLocksConflictFlagsOtherOwnersOnModifiedFiles(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}

	files := []string{
		"Art/locked_by_other.uasset",
		"Art/locked_by_me.uasset",
		"Art/unlocked_modified.uasset",
		"Art/clean.uasset",
	}
	repo := newLfsSafetyRepo(t, realGit, files...)
	for _, rel := range files[:3] {
		if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(rel)), []byte("modified\n"), 0o644); err != nil {
			t.Fatalf("modify %s: %v", rel, err)
		}
	}

	locks := `[` +
		`{"id":"1","path":"Art/locked_by_other.uasset","owner":{"name":"bob"},"locked_at":"2026-09-01T00:00:00Z"},` +
		`{"id":"2","path":"Art/locked_by_me.uasset","owner":{"name":"me"},"locked_at":"2026-09-02T00:00:00Z"}` +
		`]`
	installLfsSafetyFakeGit(t, realGit, "default", locks)

	conflicts, err := (GitService{}).CheckRemoteLocksConflict(repo)
	if err != nil {
		t.Fatalf("CheckRemoteLocksConflict failed: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected only the teammate lock to conflict, got %+v", conflicts)
	}
	got := conflicts[0]
	if got.Path != "Art/locked_by_other.uasset" {
		t.Errorf("conflict path = %q, want Art/locked_by_other.uasset", got.Path)
	}
	if got.LockOwner != "bob" {
		t.Errorf("conflict owner = %q, want bob", got.LockOwner)
	}
	if got.IsStaged || got.UnstagedStatus != "M" {
		t.Errorf("conflict must keep the unstaged status, got %+v", got)
	}
}

// TestCheckRemoteLocksConflictMatchesCurrentUserCaseInsensitive guards against
// a lock owner that differs from user.name/user.email only by case being
// misreported as a teammate conflict.
func TestCheckRemoteLocksConflictMatchesCurrentUserCaseInsensitive(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}

	files := []string{
		"Art/owned_by_name.uasset",
		"Art/owned_by_email.uasset",
		"Art/owned_by_other.uasset",
	}
	repo := newLfsSafetyRepo(t, realGit, files...)
	// user.name stays "me" (stubbed); user.email is overridden so the email
	// fallback can be exercised with an address that does not match the owner name.
	lfsSafetyGit(t, realGit, repo, "config", "user.email", "me@x.com")
	for _, rel := range files {
		if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(rel)), []byte("modified\n"), 0o644); err != nil {
			t.Fatalf("modify %s: %v", rel, err)
		}
	}

	locks := `[` +
		`{"id":"1","path":"Art/owned_by_name.uasset","owner":{"name":"ME"},"locked_at":"2026-09-01T00:00:00Z"},` +
		`{"id":"2","path":"Art/owned_by_email.uasset","owner":{"name":"ME@X.COM"},"locked_at":"2026-09-02T00:00:00Z"},` +
		`{"id":"3","path":"Art/owned_by_other.uasset","owner":{"name":"OTHER"},"locked_at":"2026-09-03T00:00:00Z"}` +
		`]`
	installLfsSafetyFakeGit(t, realGit, "default", locks)

	conflicts, err := (GitService{}).CheckRemoteLocksConflict(repo)
	if err != nil {
		t.Fatalf("CheckRemoteLocksConflict failed: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected only the other owner's lock to conflict, got %+v", conflicts)
	}
	got := conflicts[0]
	if got.Path != "Art/owned_by_other.uasset" {
		t.Errorf("conflict path = %q, want Art/owned_by_other.uasset", got.Path)
	}
	if got.LockOwner != "OTHER" {
		t.Errorf("conflict owner = %q, want OTHER", got.LockOwner)
	}
	if got.IsStaged || got.UnstagedStatus != "M" {
		t.Errorf("conflict must keep the unstaged status, got %+v", got)
	}
}

func TestCheckRemoteLocksConflictDegradesToEmptyOnMissingLockData(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}

	cases := []struct {
		name    string
		mode    string
		locks   string
		wantErr string
	}{
		{name: "no locks", mode: "default", locks: `[]`},
		{name: "unparseable json falls back to empty plain list", mode: "bad-json"},
		// No git-lfs means there is no lock source at all; the tab must stay usable.
		{name: "git-lfs unavailable", mode: "unavailable"},
		// A real remote/auth/network failure must be reported, not hidden as "no conflicts".
		{name: "lock query failure is surfaced", mode: "query-fail", wantErr: "检查远端 LFS 锁失败"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newLfsSafetyRepo(t, realGit, "Art/hero.uasset")
			if err := os.WriteFile(filepath.Join(repo, "Art", "hero.uasset"), []byte("modified\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			installLfsSafetyFakeGit(t, realGit, tc.mode, tc.locks)

			conflicts, err := (GitService{}).CheckRemoteLocksConflict(repo)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected the lock query failure to surface, got conflicts=%#v", conflicts)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
				}
				if conflicts != nil {
					t.Errorf("expected no conflicts alongside the error, got %#v", conflicts)
				}
				return
			}
			if err != nil {
				t.Fatalf("degradation paths must not surface an error: %v", err)
			}
			if conflicts == nil || len(conflicts) != 0 {
				t.Fatalf("expected a non-nil empty conflict list, got %#v", conflicts)
			}
		})
	}
}

func TestUnlockStaleLfsLocksRejectsEmptyInput(t *testing.T) {
	svc := GitService{}

	res, err := svc.UnlockStaleLfsLocks("", []string{"Art/hero.uasset"}, true)
	if err == nil || res != nil {
		t.Fatalf("empty repo path must fail, got res=%+v err=%v", res, err)
	}
	if !strings.Contains(err.Error(), "repository path") {
		t.Errorf("unexpected error for empty repo path: %v", err)
	}

	res, err = svc.UnlockStaleLfsLocks(t.TempDir(), nil, true)
	if err == nil || res != nil {
		t.Fatalf("empty path list must fail, got res=%+v err=%v", res, err)
	}
	if !strings.Contains(err.Error(), "没有选择要破除的锁") {
		t.Errorf("unexpected error for empty path list: %v", err)
	}
}

func TestUnlockStaleLfsLocksMixedBatchReportsPerPathStatus(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	repo := newLfsSafetyRepo(t, realGit)

	locks := `[` +
		`{"id":"1","path":"Art/hero.uasset","owner":{"name":"alice"},"locked_at":"2026-09-20T00:00:00Z"},` +
		`{"id":"2","path":"locked/fail.uasset","owner":{"name":"bob"},"locked_at":"2026-09-21T00:00:00Z"},` +
		`{"id":"3","path":"Art/prop.uasset","owner":{"name":"carol"},"locked_at":"2026-09-22T00:00:00Z"}` +
		`]`
	installLfsSafetyFakeGit(t, realGit, "default", locks)

	paths := []string{"Art/hero.uasset", "locked/fail.uasset", "Art/prop.uasset"}
	res, err := (GitService{}).UnlockStaleLfsLocks(repo, paths, true)
	if err != nil {
		t.Fatalf("UnlockStaleLfsLocks failed: %v", err)
	}
	if res.Success {
		t.Error("a partially failed batch must not report Success")
	}
	if res.Unlocked != 2 || res.Failed != 1 || res.Skipped != 0 {
		t.Fatalf("unexpected counts: unlocked=%d failed=%d skipped=%d", res.Unlocked, res.Failed, res.Skipped)
	}
	if len(res.Items) != len(paths) {
		t.Fatalf("expected %d per-path items, got %+v", len(paths), res.Items)
	}

	wantStatus := []string{"unlocked", "failed", "unlocked"}
	wantOwner := []string{"alice", "bob", "carol"}
	for i, item := range res.Items {
		if item.Path != paths[i] {
			t.Errorf("item %d path = %q, want %q (input order must be preserved)", i, item.Path, paths[i])
		}
		if item.Status != wantStatus[i] {
			t.Errorf("item %d status = %q, want %q", i, item.Status, wantStatus[i])
		}
		if item.Owner != wantOwner[i] {
			t.Errorf("item %d owner = %q, want %q", i, item.Owner, wantOwner[i])
		}
		if item.Age == "" || item.Age == "持有时间未知" {
			t.Errorf("item %d should be enriched with a lock age, got %q", i, item.Age)
		}
	}
	if res.Items[1].Detail == "" || !strings.Contains(res.Items[1].Detail, "owned by another user") {
		t.Errorf("failed item must carry the unlock error, got %q", res.Items[1].Detail)
	}
	if res.Items[0].Detail != "" || res.Items[2].Detail != "" {
		t.Errorf("successful items must not carry a detail: %+v", res.Items)
	}

	if res.MessageKey != "closed.loop.lock.partial" {
		t.Errorf("messageKey = %q, want closed.loop.lock.partial", res.MessageKey)
	}
	if res.Params["unlocked"] != "2" || res.Params["failed"] != "1" {
		t.Errorf("params must match the counts, got %+v", res.Params)
	}
	if res.Message == "" {
		t.Error("expected a human-readable partial message")
	}
}

func TestUnlockStaleLfsLocksAllSuccess(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	repo := newLfsSafetyRepo(t, realGit)

	locks := `[` +
		`{"id":"1","path":"Art/hero.uasset","owner":{"name":"alice"},"locked_at":"2026-09-20T00:00:00Z"},` +
		`{"id":"2","path":"Art/prop.uasset","owner":{"name":"bob"},"locked_at":"2026-09-21T00:00:00Z"}` +
		`]`
	installLfsSafetyFakeGit(t, realGit, "default", locks)

	paths := []string{"Art/hero.uasset", "Art/prop.uasset"}
	res, err := (GitService{}).UnlockStaleLfsLocks(repo, paths, false)
	if err != nil {
		t.Fatalf("UnlockStaleLfsLocks failed: %v", err)
	}
	if !res.Success {
		t.Error("an all-success batch must report Success")
	}
	if res.Unlocked != len(paths) || res.Failed != 0 {
		t.Fatalf("unexpected counts: unlocked=%d failed=%d", res.Unlocked, res.Failed)
	}
	for i, item := range res.Items {
		if item.Path != paths[i] || item.Status != "unlocked" || item.Detail != "" {
			t.Errorf("item %d = %+v, want an unlocked entry for %q", i, item, paths[i])
		}
		if item.Owner == "" || item.Age == "" {
			t.Errorf("item %d should be enriched from the lock inventory: %+v", i, item)
		}
	}
	if res.MessageKey != "closed.loop.lock.done" {
		t.Errorf("messageKey = %q, want closed.loop.lock.done", res.MessageKey)
	}
	if res.Params["unlocked"] != "2" {
		t.Errorf("params must match the counts, got %+v", res.Params)
	}
}

func TestUnlockStaleLfsLocksRequiresGitLfs(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	repo := newLfsSafetyRepo(t, realGit)
	installLfsSafetyFakeGit(t, realGit, "version-fail", `[]`)

	res, err := (GitService{}).UnlockStaleLfsLocks(repo, []string{"Art/hero.uasset"}, true)
	if err == nil {
		t.Fatalf("expected an error when git-lfs is missing, got %+v", res)
	}
	if res != nil {
		t.Errorf("expected no result on git-lfs failure, got %+v", res)
	}
	if !strings.Contains(err.Error(), "git-lfs") {
		t.Errorf("error must mention git-lfs, got: %v", err)
	}
}

func TestUnlockStaleLfsLocksRejectsBlankOnlyPaths(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	repo := newLfsSafetyRepo(t, realGit)
	// version-fail proves the blank filter runs before the git-lfs probe.
	logPath := filepath.Join(t.TempDir(), "git-calls.log")
	t.Setenv("WGIT_FAKE_GIT_LOG", logPath)
	installLfsSafetyFakeGit(t, realGit, "version-fail", `[]`)

	res, err := (GitService{}).UnlockStaleLfsLocks(repo, []string{"  ", "\t"}, true)
	if err == nil || res != nil {
		t.Fatalf("blank paths must be rejected, got res=%+v err=%v", res, err)
	}
	if !strings.Contains(err.Error(), "没有选择要破除的锁") {
		t.Errorf("error = %v, want it to contain 没有选择要破除的锁", err)
	}
	for _, line := range readFakeGitLog(t, logPath) {
		if strings.HasPrefix(line, "lfs unlock") || strings.HasPrefix(line, "lfs version") {
			t.Errorf("blank-only selection must not touch git-lfs, saw %q", line)
		}
	}
}

func readFakeGitLog(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read fake git log: %v", err)
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestCheckRemoteLocksConflictFlagsLockedUntrackedFile(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	repo := newLfsSafetyRepo(t, realGit, "Art/tracked.uasset")
	// Untracked drafts: one held by a teammate, one free.
	for _, rel := range []string{"Art/new_locked.uasset", "Art/new_unlocked.uasset"} {
		if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(rel)), []byte("draft\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	locks := `[` +
		`{"id":"1","path":"Art/new_locked.uasset","owner":{"name":"bob"},"locked_at":"2026-09-01T00:00:00Z"}` +
		`]`
	installLfsSafetyFakeGit(t, realGit, "default", locks)

	conflicts, err := (GitService{}).CheckRemoteLocksConflict(repo)
	if err != nil {
		t.Fatalf("CheckRemoteLocksConflict failed: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected only the locked untracked draft to conflict, got %+v", conflicts)
	}
	got := conflicts[0]
	if got.Path != "Art/new_locked.uasset" {
		t.Errorf("conflict path = %q, want Art/new_locked.uasset", got.Path)
	}
	if got.LockOwner != "bob" {
		t.Errorf("conflict owner = %q, want bob", got.LockOwner)
	}
	if got.IsStaged || got.UnstagedStatus != "?" {
		t.Errorf("conflict must keep the untracked status, got %+v", got)
	}
}

func TestLockAllModifiedLfsSurfacesLockQueryFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	repo := newLfsSafetyRepo(t, realGit, "Art/hero.uasset")
	if err := os.WriteFile(filepath.Join(repo, "Art", "hero.uasset"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("*.uasset filter=lfs diff=lfs merge=lfs -text lockable\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(t.TempDir(), "git-calls.log")
	t.Setenv("WGIT_FAKE_GIT_LOG", logPath)
	installLfsSafetyFakeGit(t, realGit, "locks-hard-fail", "")

	res, err := (GitService{}).LockAllModifiedLfs(repo)
	if err == nil || res != nil {
		t.Fatalf("a failed lock inventory must abort the batch, got res=%+v err=%v", res, err)
	}
	if !strings.Contains(err.Error(), "读取现有 LFS 锁失败") {
		t.Fatalf("error = %v, want it to contain 读取现有 LFS 锁失败", err)
	}
	for _, line := range readFakeGitLog(t, logPath) {
		if strings.HasPrefix(line, "lfs lock ") {
			t.Errorf("no lock command may run when the inventory is unknown, saw %q", line)
		}
	}
}

func TestSyncLockableFilePermissionsSurfacesLockQueryFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	repo := newLfsSafetyRepo(t, realGit, "Art/hero.uasset")
	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("*.uasset filter=lfs diff=lfs merge=lfs -text lockable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	installLfsSafetyFakeGit(t, realGit, "locks-hard-fail", "")

	res, err := (GitService{}).SyncLockableFilePermissions(repo)
	if err == nil || res != nil {
		t.Fatalf("a failed lock inventory must abort the sync, got res=%+v err=%v", res, err)
	}
	if !strings.Contains(err.Error(), "读取 LFS 锁失败，无法同步只读保护") {
		t.Fatalf("error = %v, want it to contain 读取 LFS 锁失败，无法同步只读保护", err)
	}
}
