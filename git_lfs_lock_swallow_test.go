package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// swallowFakeGitScript is a stub `git` covering the LFS lock surfaces whose
// ExecGit error handling git_lfs_lock.go triaged in the swallow audit:
//
//   - check-attr probes (CheckStagedNonLfsLargeFiles, the staged-large-file
//     push gate) — a failed probe must surface, not masquerade as "not LFS"
//   - the `lfs locks` JSON/plain reads behind CheckFileLockStatus, whose
//     broken-JSON fallback is a designed degradation for the lock-release flow
//   - identity reads (user.name/user.email) feeding lock-ownership matching —
//     failures must degrade fail-safe (conflicts still reported), never silent
//   - per-revision `git show` pointer enrichment in InspectLfsFileHistory —
//     best-effort by design, revisions survive without Oid/Size
//   - VerifyPrePushLfs reads (rev-parse/log/ls-tree, and `git show` beyond the
//     "path absent at rev" skip) — a failed check must surface as an error,
//     never as a "fully synced / safe to push" verdict
//
// Every other command (status, rev-parse, log, commit, ...) is forwarded to
// the real binary, which is resolved before the stub goes on PATH.
const swallowFakeGitScript = `#!/bin/sh
real="$WGIT_SWALLOW_REAL_GIT"
mode="$WGIT_SWALLOW_MODE"

if [ "$1" = "check-attr" ]; then
  if [ "$mode" = "attr-fail" ]; then
    echo "fatal: attribute engine unavailable" >&2
    exit 1
  fi
  if [ "$mode" = "attr-lfs" ]; then
    echo "$4: filter: lfs"
    exit 0
  fi
fi

if [ "$1" = "config" ] && [ "$mode" = "config-fail" ]; then
  case "$2" in
    user.name|user.email)
      echo "fatal: unable to read identity" >&2
      exit 128
      ;;
  esac
fi

if [ "$1" = "show" ] && [ "$mode" = "show-fail" ]; then
  echo "fatal: object unreadable" >&2
  exit 128
fi

if [ "$1" = "log" ] && [ "$mode" = "log-fail" ]; then
  echo "fatal: simulated log failure" >&2
  exit 128
fi

if [ "$1" = "ls-tree" ] && [ "$mode" = "ls-tree-fail" ]; then
  echo "fatal: simulated ls-tree failure" >&2
  exit 128
fi

if [ "$1" = "rev-parse" ] && [ "$mode" = "rev-parse-fail" ]; then
  echo "fatal: simulated rev-parse failure" >&2
  exit 128
fi

if [ "$1" = "lfs" ] && [ "$2" = "locks" ]; then
  if [ "$3" = "--json" ]; then
    case "$mode" in
      json-garbage)
        echo '<<<not json>>>'
        ;;
      config-fail)
        printf '%s\n' "$WGIT_SWALLOW_LOCKS_JSON"
        ;;
      *)
        echo '[]'
        ;;
    esac
    exit 0
  fi
  if [ "$mode" = "json-garbage" ]; then
    printf 'Assets/hero.png\tBob\tID:101\n'
  fi
  exit 0
fi

exec "$real" "$@"
`

// installSwallowFakeGit puts the stub git ahead of the real one; realGit must
// have been resolved with exec.LookPath before this is called.
func installSwallowFakeGit(t *testing.T, realGit, mode, locksJSON string) {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(swallowFakeGitScript), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("WGIT_SWALLOW_REAL_GIT", realGit)
	t.Setenv("WGIT_SWALLOW_MODE", mode)
	t.Setenv("WGIT_SWALLOW_LOCKS_JSON", locksJSON)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// runSwallowGit runs the real git binary directly so fixture setup never
// depends on the stub installed later.
func runSwallowGit(t *testing.T, realGit, repo string, args ...string) {
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
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func swallowIsolateGitConfig(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// TestCheckStagedNonLfsLargeFilesPropagatesCheckAttrFailure guards the staged
// oversized-file push gate: when the LFS attribute probe fails, the error must
// surface instead of classifying the file as non-LFS (a false alarm for an
// asset LFS actually protects).
func TestCheckStagedNonLfsLargeFilesPropagatesCheckAttrFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)

	repo := t.TempDir()
	runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")
	runSwallowGit(t, realGit, repo, "config", "user.name", "Alice")
	runSwallowGit(t, realGit, repo, "config", "user.email", "alice@studio.dev")
	runSwallowGit(t, realGit, repo, "commit", "-q", "--allow-empty", "-m", "base")

	const size = 1024*1024 + 1 // above the 1 MiB threshold used below
	if err := os.WriteFile(filepath.Join(repo, "big.bin"), make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "add", "big.bin")

	t.Run("probe failure propagates", func(t *testing.T) {
		installSwallowFakeGit(t, realGit, "attr-fail", "")
		files, err := (GitService{}).CheckStagedNonLfsLargeFiles(repo, 1)
		if err == nil {
			t.Fatalf("check-attr failure must surface, got files=%+v", files)
		}
		if !strings.Contains(err.Error(), "LFS 属性失败") || !strings.Contains(err.Error(), "big.bin") {
			t.Fatalf("error = %v, want contextual 检查暂存文件 big.bin 的 LFS 属性失败", err)
		}
		if files != nil {
			t.Errorf("expected no result alongside the error, got %+v", files)
		}
	})

	t.Run("lfs-filtered file is not flagged", func(t *testing.T) {
		installSwallowFakeGit(t, realGit, "attr-lfs", "")
		files, err := (GitService{}).CheckStagedNonLfsLargeFiles(repo, 1)
		if err != nil {
			t.Fatalf("healthy probe must not fail: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("LFS-managed asset must not be reported as oversized non-LFS: %+v", files)
		}
	})
}

// TestCheckFileLockStatusFallsBackToPlainWhenJSONBroken covers the lock-status
// read behind the lock-release flow: `lfs locks --json` output that fails to
// parse falls back to the plain listing instead of silently reporting an empty
// (unlocked) state.
func TestCheckFileLockStatusFallsBackToPlainWhenJSONBroken(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)

	repo := t.TempDir()
	runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")
	installSwallowFakeGit(t, realGit, "json-garbage", "")

	owner, err := (GitService{}).CheckFileLockStatus(repo, "Assets/hero.png")
	if err != nil {
		t.Fatalf("broken JSON must fall back to plain parsing, got: %v", err)
	}
	if owner != "Bob" {
		t.Fatalf("owner = %q, want Bob from the plain lock listing", owner)
	}
}

// TestInspectLfsFileHistoryToleratesShowFailure pins the triaged keep case: the
// per-revision `git show` probe is best-effort pointer enrichment, so a failure
// must not fail the history and revisions stay listed without Oid/Size.
func TestInspectLfsFileHistoryToleratesShowFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)

	repo := t.TempDir()
	runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(repo, "Assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Assets", "hero.png"), []byte("png\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "add", "-A")
	runSwallowGit(t, realGit, repo, "commit", "-q", "-m", "art: hero")

	installSwallowFakeGit(t, realGit, "show-fail", "")

	revs, err := (GitService{}).InspectLfsFileHistory(repo, "Assets/hero.png")
	if err != nil {
		t.Fatalf("show failure is best-effort and must not fail the history: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("revisions = %d, want 1: %+v", len(revs), revs)
	}
	if revs[0].CommitHash == "" || revs[0].Subject != "art: hero" {
		t.Errorf("revision metadata must survive the show failure: %+v", revs[0])
	}
	if revs[0].Oid != "" || revs[0].SizeBytes != 0 {
		t.Errorf("failed show probe must leave Oid/Size empty, got %+v", revs[0])
	}
}

// TestCheckRemoteLocksConflictKeepsFailSafeWhenIdentityUnreadable pins the
// triaged keep case for the user.name/user.email reads: when the identity
// cannot be read, own-lock matching matches nothing, so a teammate's lock on a
// locally modified file must still be reported as a conflict (fail-safe
// direction) instead of the check silently passing.
func TestCheckRemoteLocksConflictKeepsFailSafeWhenIdentityUnreadable(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)

	repo := t.TempDir()
	runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(repo, "Art"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Art", "hero.uasset"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "add", "-A")
	runSwallowGit(t, realGit, repo, "commit", "-q", "-m", "art: hero")
	if err := os.WriteFile(filepath.Join(repo, "Art", "hero.uasset"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bobLockJSON := `[{"id":"101","path":"Art/hero.uasset","owner":{"name":"Bob"},"locked_at":"2026-10-01T00:00:00Z"}]`
	installSwallowFakeGit(t, realGit, "config-fail", bobLockJSON)

	conflicts, err := (GitService{}).CheckRemoteLocksConflict(repo)
	if err != nil {
		t.Fatalf("identity read failure must degrade fail-safe, not error: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %d, want 1: %+v", len(conflicts), conflicts)
	}
	if conflicts[0].Path != "Art/hero.uasset" || conflicts[0].LockOwner != "Bob" {
		t.Errorf("conflict = %+v, want Art/hero.uasset locked by Bob", conflicts[0])
	}
}

// swallowVerifyRepo builds a real single-commit repository so every git command
// except the deliberately failed one succeeds during VerifyPrePushLfs.
func swallowVerifyRepo(t *testing.T, realGit string) string {
	t.Helper()
	repo := t.TempDir()
	runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")
	runSwallowGit(t, realGit, repo, "config", "user.name", "Alice")
	runSwallowGit(t, realGit, repo, "config", "user.email", "alice@studio.dev")
	if err := os.MkdirAll(filepath.Join(repo, "art"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "art", "asset.bin"), []byte("plain payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "add", "-A")
	runSwallowGit(t, realGit, repo, "commit", "-q", "-m", "feat: asset")
	return repo
}

// assertSwallowNotPassed pins the repair contract for VerifyPrePushLfs: when a
// repository read fails, the caller gets an error naming the broken step, never
// a result whose summary claims the branch is synced or safe to push.
func assertSwallowNotPassed(t *testing.T, err error, wantSub string) {
	t.Helper()
	if err == nil {
		t.Fatalf("failed check must not be reported as a passing verdict, want error containing %q", wantSub)
	}
	if !strings.Contains(err.Error(), wantSub) {
		t.Fatalf("error = %v, want it to mention %q", err, wantSub)
	}
	for _, leaked := range []string{"完全同步", "就绪", "可安全推送"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("error = %v, still leaks the optimistic verdict %q", err, leaked)
		}
	}
}

func TestVerifyPrePushLfsReportsLogFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := swallowVerifyRepo(t, realGit)

	installSwallowFakeGit(t, realGit, "log-fail", "")
	_, err = (GitService{}).VerifyPrePushLfs(repo, "", "")
	assertSwallowNotPassed(t, err, "读取待推送提交列表失败")
}

func TestVerifyPrePushLfsReportsLsTreeFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := swallowVerifyRepo(t, realGit)

	installSwallowFakeGit(t, realGit, "ls-tree-fail", "")
	_, err = (GitService{}).VerifyPrePushLfs(repo, "", "")
	assertSwallowNotPassed(t, err, "读取 HEAD 文件清单失败")
}

func TestVerifyPrePushLfsReportsShowFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := swallowVerifyRepo(t, realGit)

	// "fatal: object unreadable" is not the "path does not exist in 'HEAD'"
	// skip case, so the pointer check cannot complete and must surface.
	installSwallowFakeGit(t, realGit, "show-fail", "")
	_, err = (GitService{}).VerifyPrePushLfs(repo, "", "")
	assertSwallowNotPassed(t, err, "未完成 LFS 完整性检查")
}

func TestVerifyPrePushLfsReportsBranchResolveFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := swallowVerifyRepo(t, realGit)

	installSwallowFakeGit(t, realGit, "rev-parse-fail", "")
	_, err = (GitService{}).VerifyPrePushLfs(repo, "", "")
	assertSwallowNotPassed(t, err, "读取当前分支失败")
}

// TestVerifyPrePushLfsHealthyPathKeepsSafeVerdict is the regression guard for
// the repair: a file deleted inside the unpushed range is legitimately absent
// from HEAD (the "path does not exist in 'HEAD'" skip) and must not turn into
// an error, while the clean verdict stays unchanged.
func TestVerifyPrePushLfsHealthyPathKeepsSafeVerdict(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := swallowVerifyRepo(t, realGit)

	if err := os.Remove(filepath.Join(repo, "art", "asset.bin")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "art", "other.bin"), []byte("second payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "add", "-A")
	runSwallowGit(t, realGit, repo, "commit", "-q", "-m", "feat: replace asset")

	res, err := (GitService{}).VerifyPrePushLfs(repo, "", "")
	if err != nil {
		t.Fatalf("healthy path must not fail: %v", err)
	}
	if !res.IsSafe {
		t.Fatalf("healthy path should stay safe: %+v", res)
	}
	if !strings.Contains(res.StatusSummary, "就绪") {
		t.Fatalf("healthy path verdict changed: %q", res.StatusSummary)
	}
	if len(res.UnpushedCommits) != 2 {
		t.Fatalf("unpushed commits = %d, want 2", len(res.UnpushedCommits))
	}
	if res.MissingLfsCount != 0 || len(res.OversizedBlobs) != 0 {
		t.Fatalf("healthy path reported risks: %+v", res)
	}
}
