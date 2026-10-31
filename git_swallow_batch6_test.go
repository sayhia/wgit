package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// batch6FakeGitScript is a stub `git` covering the swallow-audit batch 6
// surfaces in git_lfs_migrate.go, git_stash_insights.go and git_insights.go:
//
//   - the pre-rewrite `rev-parse HEAD` read (ExecuteLfsMigration) — a failed
//     read must abort before the bundle and the rewrite, not flow into an
//     empty RewrittenFrom on a "successful" migration (propagate)
//   - the third-parent `ls-tree` read (GetStashFiles) — after the ^3 parent
//     was verified to exist, an unreadable tree must surface instead of
//     silently dropping the stash's untracked files from the list (propagate)
//   - the `stash show --numstat` line counts (GetStashFiles) — display-only,
//     the authoritative list comes from the --name-status pass (keep)
//
// The kept `git config user.name/user.email` reads in GetTeamActivity need no
// stub: an unconfigured identity is the natural exit-1 case, covered by
// TestSwallowBatch6GetTeamActivityToleratesUnconfiguredIdentity with an
// isolated HOME. `lfs version` is answered by the stub so the migration
// preflight works where git-lfs is not installed; `lfs migrate` only marks its
// invocation so tests can prove the rewrite never started. Every other
// command is forwarded to the real binary, which is resolved before the stub
// goes on PATH.
const batch6FakeGitScript = `#!/bin/sh
real="$WGIT_B6_REAL_GIT"
mode="$WGIT_B6_MODE"

if [ "$1" = "lfs" ] && [ "$2" = "version" ]; then
  echo "git-lfs/3.7.0 (batch6-stub)"
  exit 0
fi

if [ "$1" = "lfs" ] && [ "$2" = "migrate" ]; then
  touch "$WGIT_B6_MARKER"
  exec "$real" "$@"
fi

if [ "$1" = "rev-parse" ] && [ "$2" = "HEAD" ] && [ "$mode" = "head-fail" ]; then
  echo "fatal: HEAD unreadable (batch6 injection)" >&2
  exit 128
fi

if [ "$1" = "stash" ] && [ "$2" = "show" ] && [ "$3" = "--numstat" ] && [ "$mode" = "numstat-fail" ]; then
  echo "fatal: numstat unavailable (batch6 injection)" >&2
  exit 1
fi

if [ "$1" = "ls-tree" ] && [ "$mode" = "l3tree-fail" ]; then
  echo "fatal: third-parent tree unreadable (batch6 injection)" >&2
  exit 1
fi

exec "$real" "$@"
`

// installBatch6FakeGit puts the stub git ahead of the real one; realGit must
// have been resolved with exec.LookPath before this is called.
func installBatch6FakeGit(t *testing.T, realGit, mode string) {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(batch6FakeGitScript), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("WGIT_B6_REAL_GIT", realGit)
	t.Setenv("WGIT_B6_MODE", mode)
	t.Setenv("WGIT_B6_MARKER", filepath.Join(t.TempDir(), "lfs-migrate-invoked"))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestSwallowBatch6LfsMigrateAbortsWhenHeadUnreadable pins the propagation for
// the pre-rewrite HEAD read in ExecuteLfsMigration: a failed `rev-parse HEAD`
// must abort before the bundle and the rewrite. The old swallow produced a
// "successful" migration with an empty RewrittenFrom — no recovery anchor and
// no before/after verification — on an irreversible history rewrite.
func TestSwallowBatch6LfsMigrateAbortsWhenHeadUnreadable(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch3SeedRepo(t, realGit)

	installBatch6FakeGit(t, realGit, "head-fail")

	res, err := (GitService{}).ExecuteLfsMigration(repo, LfsMigrateOptions{
		Patterns:   []string{"*.psd"},
		Everything: true,
	})
	if err == nil {
		t.Fatalf("an unreadable pre-rewrite HEAD must abort the migration, got %+v", res)
	}
	if !strings.Contains(err.Error(), "读取当前 HEAD 失败") || !strings.Contains(err.Error(), "HEAD unreadable (batch6 injection)") {
		t.Fatalf("error = %v, want the abort reason carrying the root cause from the failed read", err)
	}
	if res != nil {
		t.Errorf("expected no result alongside the error, got %+v", res)
	}
	if _, statErr := os.Stat(os.Getenv("WGIT_B6_MARKER")); !os.IsNotExist(statErr) {
		t.Error("git lfs migrate must not run when the pre-rewrite HEAD read fails")
	}
	// The rewrite never started, so the repository must be intact.
	runSwallowGit(t, realGit, repo, "rev-parse", "--verify", "HEAD")
}

// TestSwallowBatch6GetStashFilesPropagatesUntrackedTreeFailure pins the
// propagation for the third-parent tree read in GetStashFiles: the ^3 parent
// has just been verified to exist, so a failed `ls-tree` means the stash's
// untracked files would silently drop out of the list — an incomplete list
// must surface as an error, not present itself as complete.
func TestSwallowBatch6GetStashFilesPropagatesUntrackedTreeFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch3SeedRepo(t, realGit)

	if err := os.WriteFile(filepath.Join(repo, "level.txt"), []byte("level v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "scratch.tmp"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "stash", "push", "-u", "-m", "wip with untracked")

	installBatch6FakeGit(t, realGit, "l3tree-fail")

	files, err := (GitService{}).GetStashFiles(repo, "0")
	if err == nil {
		t.Fatalf("an unreadable third-parent tree must surface, got files %+v", files)
	}
	if !strings.Contains(err.Error(), "读取 stash 的未跟踪文件列表失败") || !strings.Contains(err.Error(), "third-parent tree unreadable") {
		t.Fatalf("error = %v, want the wrapped ls-tree failure", err)
	}
	if files != nil {
		t.Errorf("expected no result alongside the error, got %+v", files)
	}
}

// TestSwallowBatch6GetStashFilesToleratesNumstatFailure keeps the triaged
// best-effort behavior of the numstat line counts: a failed `stash show
// --numstat` degrades Additions/Deletions to 0 while the authoritative file
// list still comes from the --name-status pass, so the entries survive.
func TestSwallowBatch6GetStashFilesToleratesNumstatFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch3SeedRepo(t, realGit)

	if err := os.WriteFile(filepath.Join(repo, "level.txt"), []byte("level v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "stash", "push", "-m", "wip tracked only")

	installBatch6FakeGit(t, realGit, "numstat-fail")

	files, err := (GitService{}).GetStashFiles(repo, "0")
	if err != nil {
		t.Fatalf("line-count failure is display-only and must not fail the listing: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %+v, want exactly the stashed modification", files)
	}
	entry := files[0]
	if entry.Path != "level.txt" || entry.Status != "M" {
		t.Errorf("entry = %+v, want the name-status record for level.txt", entry)
	}
	if entry.Additions != 0 || entry.Deletions != 0 {
		t.Errorf("counts = +%d/-%d, want the honest 0/0 degradation from the failed numstat read", entry.Additions, entry.Deletions)
	}
}

// TestSwallowBatch6GetTeamActivityToleratesUnconfiguredIdentity keeps the
// triaged behavior of the identity reads in GetTeamActivity: with no
// user.name/user.email configured anywhere (the natural exit-1 case), the
// panel still renders normally and nobody is flagged as "me" — fail-safe
// degradation, never a false "this is you" badge.
func TestSwallowBatch6GetTeamActivityToleratesUnconfiguredIdentity(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	// No `git config user.*` anywhere: the commit identity comes from the env
	// vars runSwallowGit sets, so both config reads exit non-zero.
	repo := t.TempDir()
	runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "level.txt"), []byte("level v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "add", "-A")
	runSwallowGit(t, realGit, repo, "commit", "-q", "-m", "level: seed")

	res, err := (GitService{}).GetTeamActivity(repo, 30)
	if err != nil {
		t.Fatalf("unconfigured identity must not fail the panel: %v", err)
	}
	if res == nil {
		t.Fatal("result must not be nil")
	}
	if res.TotalCommits != 1 {
		t.Fatalf("totalCommits = %d, want the seed commit to be counted", res.TotalCommits)
	}
	if len(res.Contributors) != 1 {
		t.Fatalf("contributors = %+v, want exactly Alice", res.Contributors)
	}
	c := res.Contributors[0]
	if c.Name != "Alice" {
		t.Errorf("name = %q, want the log-read author", c.Name)
	}
	if c.IsMe {
		t.Error("an unread identity must not flag anyone as \"me\"")
	}
}
