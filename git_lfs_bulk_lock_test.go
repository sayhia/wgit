package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bulkLockFakeGitScript is a stub `git` that owns only the LFS surface used by
// LockAllModifiedLfs: a configurable remote lock inventory plus successful
// `lfs lock` calls. Every other command (status, rev-parse, check-attr, config,
// ...) is forwarded to the real binary, which was resolved before the stub went
// on PATH. WGIT_BULK_LOCKS_JSON supplies the inventory (empty means no locks).
// Successful lock paths are appended to WGIT_BULK_LOCK_LOG; a path matching
// WGIT_BULK_FAIL_LOCK is rejected so the per-file failure path can be asserted.
const bulkLockFakeGitScript = `#!/bin/sh
real="$WGIT_BULK_REAL_GIT"

if [ "$1" = "lfs" ]; then
  case "$2" in
    locks)
      if [ "$3" = "--json" ]; then
        if [ -n "$WGIT_BULK_LOCKS_JSON" ]; then
          printf '%s\n' "$WGIT_BULK_LOCKS_JSON"
        else
          printf '[]\n'
        fi
      fi
      exit 0
      ;;
    lock)
      target="$3"
      if [ -n "$WGIT_BULK_FAIL_LOCK" ] && [ "$target" = "$WGIT_BULK_FAIL_LOCK" ]; then
        echo "Lock failed: $target is already locked by another user" >&2
        exit 1
      fi
      if [ -n "$WGIT_BULK_LOCK_LOG" ]; then
        printf '%s\n' "$target" >> "$WGIT_BULK_LOCK_LOG"
      fi
      exit 0
      ;;
  esac
fi

exec "$real" "$@"
`

// installBulkLockFakeGit puts the stub git ahead of the real one. realGit must
// have been resolved with exec.LookPath before this is called.
func installBulkLockFakeGit(t *testing.T, realGit, logPath, failPath, locksJSON string) {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(bulkLockFakeGitScript), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("WGIT_BULK_REAL_GIT", realGit)
	t.Setenv("WGIT_BULK_LOCK_LOG", logPath)
	t.Setenv("WGIT_BULK_FAIL_LOCK", failPath)
	t.Setenv("WGIT_BULK_LOCKS_JSON", locksJSON)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// bulkLockGit runs the real git binary directly so fixture setup never depends
// on the stub installed later. Callers isolate GIT_CONFIG_GLOBAL/NOSYSTEM first.
func bulkLockGit(t *testing.T, realGit, repo string, args ...string) string {
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

// bulkLockAssetPaths returns Assets/A01.uasset .. Assets/A20.uasset.
func bulkLockAssetPaths() []string {
	paths := make([]string, 0, 20)
	for i := 1; i <= 20; i++ {
		paths = append(paths, fmt.Sprintf("Assets/A%02d.uasset", i))
	}
	return paths
}

// newBulkLockRepo commits .gitattributes, 20 LFS-tracked assets and readme.txt,
// then modifies all 21 files so every one of them becomes a lock candidate.
func newBulkLockRepo(t *testing.T, realGit string) (string, []string) {
	t.Helper()
	repo := t.TempDir()
	assets := bulkLockAssetPaths()
	all := append(append([]string{}, assets...), "readme.txt")

	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"),
		[]byte("*.uasset filter=lfs diff=lfs merge=lfs -text\n"), 0o644); err != nil {
		t.Fatalf("write .gitattributes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "Assets"), 0o755); err != nil {
		t.Fatalf("mkdir Assets: %v", err)
	}
	for _, rel := range all {
		if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(rel)), []byte("seed\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	bulkLockGit(t, realGit, repo, "init", "-q", "-b", "main")
	bulkLockGit(t, realGit, repo, "config", "user.name", "me")
	bulkLockGit(t, realGit, repo, "config", "user.email", "me@studio.dev")
	bulkLockGit(t, realGit, repo, "add", "-A")
	bulkLockGit(t, realGit, repo, "commit", "-q", "-m", "chore: seed assets")

	for _, rel := range all {
		if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(rel)), []byte("modified seed\n"), 0o644); err != nil {
			t.Fatalf("modify %s: %v", rel, err)
		}
	}
	return repo, assets
}

// bulkLockLogPaths reads the paths appended by the stub for successful locks.
func bulkLockLogPaths(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read fake lock log: %v", err)
	}
	var paths []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

func bulkLockIsolateGitConfig(t *testing.T) {
	t.Helper()
	// Isolate from the developer's global/system config (git-lfs filters, signing,
	// identity) so the fixture repo behaves identically on every machine.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func TestLockAllModifiedLfsLocksOnlyLfsCandidates(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	bulkLockIsolateGitConfig(t)
	repo, assets := newBulkLockRepo(t, realGit)

	logPath := filepath.Join(t.TempDir(), "locks.log")
	installBulkLockFakeGit(t, realGit, logPath, "", "")

	res, err := (GitService{}).LockAllModifiedLfs(repo)
	if err != nil {
		t.Fatalf("LockAllModifiedLfs failed: %v", err)
	}
	if res == nil {
		t.Fatal("LockAllModifiedLfs returned nil result")
	}
	if res.TotalCount != 20 {
		t.Errorf("TotalCount = %d, want 20 (readme.txt must be skipped)", res.TotalCount)
	}
	if len(res.LockedFiles) != 20 {
		t.Errorf("locked %d files, want 20: %v", len(res.LockedFiles), res.LockedFiles)
	}
	if len(res.SkippedFiles) != 0 || len(res.FailedFiles) != 0 {
		t.Errorf("unexpected skipped=%v failed=%v", res.SkippedFiles, res.FailedFiles)
	}
	if !strings.Contains(res.Message, "成功锁定 20 个 LFS 资产") {
		t.Errorf("message = %q, want it to mention 成功锁定 20 个 LFS 资产", res.Message)
	}

	expected := make(map[string]bool, len(assets))
	for _, a := range assets {
		expected[a] = true
	}
	seen := make(map[string]bool, len(res.LockedFiles))
	for _, f := range res.LockedFiles {
		if !expected[f] {
			t.Errorf("locked non-LFS-candidate %q", f)
		}
		if seen[f] {
			t.Errorf("duplicate lock entry %q", f)
		}
		seen[f] = true
	}
	if len(seen) != 20 {
		t.Errorf("locked %d distinct assets, want 20", len(seen))
	}

	logged := bulkLockLogPaths(t, logPath)
	if len(logged) != 20 {
		t.Errorf("issued %d lfs lock calls, want 20: %v", len(logged), logged)
	}
	loggedSet := make(map[string]bool, len(logged))
	for _, p := range logged {
		if !expected[p] {
			t.Errorf("unexpected lfs lock target %q", p)
		}
		loggedSet[p] = true
	}
	for _, a := range assets {
		if !loggedSet[a] {
			t.Errorf("missing lfs lock invocation for %q", a)
		}
	}
}

func TestLockAllModifiedLfsReportsIndividualLockFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	bulkLockIsolateGitConfig(t)
	repo, _ := newBulkLockRepo(t, realGit)

	failPath := "Assets/A05.uasset"
	logPath := filepath.Join(t.TempDir(), "locks.log")
	installBulkLockFakeGit(t, realGit, logPath, failPath, "")

	res, err := (GitService{}).LockAllModifiedLfs(repo)
	if err != nil {
		t.Fatalf("LockAllModifiedLfs failed: %v", err)
	}
	if res == nil {
		t.Fatal("LockAllModifiedLfs returned nil result")
	}
	if len(res.LockedFiles) != 19 {
		t.Errorf("locked %d files, want 19: %v", len(res.LockedFiles), res.LockedFiles)
	}
	if len(res.FailedFiles) != 1 || !strings.Contains(res.FailedFiles[0], failPath) {
		t.Errorf("failed files = %v, want exactly one entry for %s", res.FailedFiles, failPath)
	}
	if !strings.Contains(res.Message, "锁定失败") {
		t.Errorf("message = %q, want it to mention 锁定失败", res.Message)
	}
	if res.TotalCount != 20 {
		t.Errorf("TotalCount = %d, want 20", res.TotalCount)
	}
	for _, f := range res.LockedFiles {
		if f == failPath {
			t.Errorf("failed asset %q must not be reported as locked", failPath)
		}
	}

	logged := bulkLockLogPaths(t, logPath)
	if len(logged) != 19 {
		t.Errorf("recorded %d successful lfs lock calls, want 19: %v", len(logged), logged)
	}
	for _, p := range logged {
		if p == failPath {
			t.Errorf("failed lock %q must not be logged as successful", failPath)
		}
	}
	_ = assets // assets documents the fixture; membership covered via locked count
}

// bulkLockInventoryJSON builds a lock inventory covering every fixture asset,
// all held by the given owner name.
func bulkLockInventoryJSON(owner string) string {
	assets := bulkLockAssetPaths()
	var sb strings.Builder
	sb.WriteString("[")
	for i, rel := range assets {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"id":"lock-%d","path":%q,"owner":{"name":%q},"locked_at":"2026-09-01T00:00:00Z"}`, i+1, rel, owner)
	}
	sb.WriteString("]")
	return sb.String()
}

// TestLockAllModifiedLfsSkipsLocksOwnedByCurrentUser guards against a lock
// owner that differs from user.name/user.email only by case being treated as a
// teammate conflict (and failing the whole batch).
func TestLockAllModifiedLfsSkipsLocksOwnedByCurrentUser(t *testing.T) {
	cases := []struct {
		name      string
		userEmail string
		owner     string
	}{
		{name: "name differs only by case", userEmail: "me@studio.dev", owner: "ME"},
		{name: "email differs only by case", userEmail: "me@x.com", owner: "ME@X.COM"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatalf("locate git: %v", err)
			}
			bulkLockIsolateGitConfig(t)
			repo, assets := newBulkLockRepo(t, realGit)
			bulkLockGit(t, realGit, repo, "config", "user.email", tc.userEmail)

			logPath := filepath.Join(t.TempDir(), "locks.log")
			installBulkLockFakeGit(t, realGit, logPath, "", bulkLockInventoryJSON(tc.owner))

			res, err := (GitService{}).LockAllModifiedLfs(repo)
			if err != nil {
				t.Fatalf("LockAllModifiedLfs failed: %v", err)
			}
			if res == nil {
				t.Fatal("LockAllModifiedLfs returned nil result")
			}
			if len(res.LockedFiles) != 0 {
				t.Errorf("locked %d files, want 0: %v", len(res.LockedFiles), res.LockedFiles)
			}
			if len(res.FailedFiles) != 0 {
				t.Errorf("failed %d files, want 0: %v", len(res.FailedFiles), res.FailedFiles)
			}
			if len(res.SkippedFiles) != len(assets) {
				t.Errorf("skipped %d files, want %d: %v", len(res.SkippedFiles), len(assets), res.SkippedFiles)
			}
			if res.TotalCount != len(assets) {
				t.Errorf("TotalCount = %d, want %d", res.TotalCount, len(assets))
			}
			if !strings.Contains(res.Message, "成功锁定 0 个 LFS 资产") {
				t.Errorf("message = %q, want it to report 成功锁定 0 个 LFS 资产", res.Message)
			}
			if !strings.Contains(res.Message, fmt.Sprintf("%d 个已预先锁定", len(assets))) {
				t.Errorf("message = %q, want it to report %d 个已预先锁定", res.Message, len(assets))
			}
			if strings.Contains(res.Message, "锁定失败") {
				t.Errorf("message = %q, must not report failures for own locks", res.Message)
			}
			if logged := bulkLockLogPaths(t, logPath); len(logged) != 0 {
				t.Errorf("issued %d lfs lock calls for already-owned locks: %v", len(logged), logged)
			}

			skipped := make(map[string]bool, len(res.SkippedFiles))
			for _, f := range res.SkippedFiles {
				skipped[f] = true
			}
			for _, a := range assets {
				if !skipped[a] {
					t.Errorf("asset %q was not reported as skipped", a)
				}
			}
		})
	}
}
