package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// batch4FakeGitScript is a stub `git` covering the git_service.go surfaces
// whose ExecGit error handling batch 4 of the swallow audit triaged:
//
//   - the `git status --porcelain` probes that route DiscardFile and
//     DiscardUntrackedFile between the destructive `clean -fd` and
//     `restore`/refusal paths — a failed probe must surface and must not
//     execute the destructive branch
//   - the `git remote` listing in getRepoInfo — a failed read must propagate
//     instead of degrading into an empty Remotes list that hides push targets
//   - the `rev-parse --git-dir` repository gates added around the swallowed
//     `git config` reads (GetGitConfig, GetGitProxy, GetCommitSigningStatus) —
//     a broken repository must surface, while the per-key swallows keep
//     masking only the benign "key not configured" exit 1
//
// Every other command is forwarded to the real binary, which is resolved
// before the stub goes on PATH (shared helpers come from
// git_lfs_lock_swallow_test.go).
const batch4FakeGitScript = `#!/bin/sh
real="$WGIT_BATCH4_REAL_GIT"
mode="$WGIT_BATCH4_MODE"

case "$mode" in
  status-fail)
    if [ "$1" = "status" ]; then
      echo "fatal: unable to read working tree status" >&2
      exit 128
    fi
    ;;
  remote-fail)
    if [ "$1" = "remote" ]; then
      echo "fatal: simulated remote listing failure" >&2
      exit 128
    fi
    ;;
  rev-parse-fail)
    if [ "$1" = "rev-parse" ]; then
      echo "fatal: simulated rev-parse failure" >&2
      exit 128
    fi
    ;;
esac

exec "$real" "$@"
`

// installBatch4FakeGit puts the stub git ahead of the real one; realGit must
// have been resolved with exec.LookPath before this is called.
func installBatch4FakeGit(t *testing.T, realGit, mode string) {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(batch4FakeGitScript), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("WGIT_BATCH4_REAL_GIT", realGit)
	t.Setenv("WGIT_BATCH4_MODE", mode)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// batch4SeedRepo builds a real single-commit repository with a tracked file
// and a local identity, so fixture setup never depends on the stub installed
// later.
func batch4SeedRepo(t *testing.T, realGit string) string {
	t.Helper()
	repo := t.TempDir()
	runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")
	runSwallowGit(t, realGit, repo, "config", "user.name", "Alice")
	runSwallowGit(t, realGit, repo, "config", "user.email", "alice@studio.dev")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSwallowGit(t, realGit, repo, "add", "-A")
	runSwallowGit(t, realGit, repo, "commit", "-q", "-m", "chore: seed")
	return repo
}

// TestBatch4DiscardFileSurfacesStatusCheckFailure pins the DiscardFile repair:
// the `git status` probe routes between `clean -fd` (removes the file) and
// `restore` (discards tracked changes), so a failed probe must surface and
// neither destructive command may run.
func TestBatch4DiscardFileSurfacesStatusCheckFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch4SeedRepo(t, realGit)

	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	installBatch4FakeGit(t, realGit, "status-fail")
	err = (GitService{}).DiscardFile(repo, "tracked.txt")
	if err == nil {
		t.Fatal("failed status probe must surface from DiscardFile")
	}
	if !strings.Contains(err.Error(), "status check failed") || !strings.Contains(err.Error(), "tracked.txt") {
		t.Fatalf("error = %v, want the contextual status check failure for tracked.txt", err)
	}
	// The destructive path must not have run on a failed probe: the working
	// tree modification survives untouched.
	data, readErr := os.ReadFile(filepath.Join(repo, "tracked.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "v2\n" {
		t.Fatalf("tracked.txt was discarded despite the failed probe: %q", data)
	}
}

// TestBatch4DiscardUntrackedFileSurfacesStatusCheckFailure pins the
// DiscardUntrackedFile repair: a failed `git status` probe must surface with
// the real cause instead of the false "not an untracked path" conclusion, and
// the `clean -fd` guard must stay closed.
func TestBatch4DiscardUntrackedFileSurfacesStatusCheckFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch4SeedRepo(t, realGit)

	untracked := filepath.Join(repo, "notes.txt")
	if err := os.WriteFile(untracked, []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	installBatch4FakeGit(t, realGit, "status-fail")
	err = (GitService{}).DiscardUntrackedFile(repo, "notes.txt")
	if err == nil {
		t.Fatal("failed status probe must surface from DiscardUntrackedFile")
	}
	if !strings.Contains(err.Error(), "status check failed") || strings.Contains(err.Error(), "not an untracked path") {
		t.Fatalf("error = %v, want the status check failure and not the misleading conclusion", err)
	}
	if _, statErr := os.Stat(untracked); statErr != nil {
		t.Fatalf("untracked file must survive the failed probe: %v", statErr)
	}
}

// TestBatch4GetRepoInfoSurfacesRemoteListingFailure pins the getRepoInfo
// repair: a failed `git remote` read must propagate instead of degrading into
// an empty Remotes list that would present a repository with push targets as
// having none.
func TestBatch4GetRepoInfoSurfacesRemoteListingFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch4SeedRepo(t, realGit)
	runSwallowGit(t, realGit, repo, "remote", "add", "origin", "/tmp/batch4-origin.git")

	installBatch4FakeGit(t, realGit, "remote-fail")
	info, err := getRepoInfo(repo)
	if err == nil {
		t.Fatalf("failed remote listing must surface, got info=%+v", info)
	}
	if !strings.Contains(err.Error(), "read remotes") {
		t.Fatalf("error = %v, want the wrapped read remotes failure", err)
	}
	if info != nil {
		t.Errorf("expected no result alongside the error, got %+v", info)
	}
}

// TestBatch4GitConfigSurfacesBrokenRepoKeepsUnsetKeysBenign pins the
// GetGitConfig repair pair: a broken repository read must surface through the
// rev-parse gate, while the ordinary "user.name/user.email not configured"
// exit 1 stays a benign empty config.
func TestBatch4GitConfigSurfacesBrokenRepoKeepsUnsetKeysBenign(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)

	t.Run("broken repo surfaces", func(t *testing.T) {
		repo := batch4SeedRepo(t, realGit)
		installBatch4FakeGit(t, realGit, "rev-parse-fail")
		cfg, err := (GitService{}).GetGitConfig(repo)
		if err == nil {
			t.Fatalf("broken repository read must surface, got %+v", cfg)
		}
		if !strings.Contains(err.Error(), "read git user config") {
			t.Fatalf("error = %v, want the wrapped read git user config failure", err)
		}
	})

	t.Run("unset keys stay benign", func(t *testing.T) {
		repo := t.TempDir()
		runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")
		cfg, err := (GitService{}).GetGitConfig(repo)
		if err != nil {
			t.Fatalf("unset identity is the ordinary not-configured exit and must not error: %v", err)
		}
		if cfg == nil || cfg.UserName != "" || cfg.UserEmail != "" {
			t.Fatalf("unset identity must render as an empty config, got %+v", cfg)
		}
	})
}

// TestBatch4GitProxySurfacesBrokenRepoKeepsUnsetProxyBenign pins the
// GetGitProxy repair pair: a broken repository must not silently render as
// "no local proxy configured", while an ordinary unset proxy key keeps the
// error-free empty result.
func TestBatch4GitProxySurfacesBrokenRepoKeepsUnsetProxyBenign(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)

	t.Run("broken repo surfaces", func(t *testing.T) {
		repo := batch4SeedRepo(t, realGit)
		installBatch4FakeGit(t, realGit, "rev-parse-fail")
		cfg, err := (GitService{}).GetGitProxy(repo)
		if err == nil {
			t.Fatalf("broken repository read must surface, got %+v", cfg)
		}
		if !strings.Contains(err.Error(), "read git proxy config") {
			t.Fatalf("error = %v, want the wrapped read git proxy config failure", err)
		}
	})

	t.Run("unset proxy stays benign", func(t *testing.T) {
		repo := t.TempDir()
		runSwallowGit(t, realGit, repo, "init", "-q", "-b", "main")
		cfg, err := (GitService{}).GetGitProxy(repo)
		if err != nil {
			t.Fatalf("unset proxy is the ordinary not-configured exit and must not error: %v", err)
		}
		if cfg == nil || cfg.HttpProxy != "" || cfg.HttpsProxy != "" || cfg.IsGlobal {
			t.Fatalf("unset proxy must render as an empty local config, got %+v", cfg)
		}
	})
}

// TestBatch4CommitSigningSurfacesBrokenRepo pins the GetCommitSigningStatus
// repair: a broken repository read must surface instead of reporting "signing
// disabled" with 0% compliance — a failed check presented as an affirmative
// verdict.
func TestBatch4CommitSigningSurfacesBrokenRepo(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch4SeedRepo(t, realGit)

	installBatch4FakeGit(t, realGit, "rev-parse-fail")
	status, err := (GitService{}).GetCommitSigningStatus(repo)
	if err == nil {
		t.Fatalf("broken repository must surface instead of reporting signing disabled, got %+v", status)
	}
	if !strings.Contains(err.Error(), "read commit signing config") {
		t.Fatalf("error = %v, want the wrapped read commit signing config failure", err)
	}
	if status != nil {
		t.Errorf("expected no result alongside the error, got %+v", status)
	}
}

// TestBatch4SignatureDetailsKeepUnsignedVerifyBenign pins the triaged keep
// case in GetCommitSignatureDetails: `git verify-commit` exits non-zero
// exactly for unsigned or bad signatures, so the swallow is what keeps an
// ordinary unsigned commit rendering as Status "N" instead of failing the
// details view.
func TestBatch4SignatureDetailsKeepUnsignedVerifyBenign(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	swallowIsolateGitConfig(t)
	repo := batch4SeedRepo(t, realGit) // single unsigned commit

	detail, err := (GitService{}).GetCommitSignatureDetails(repo, "")
	if err != nil {
		t.Fatalf("unsigned commit must not fail the details view: %v", err)
	}
	if detail == nil || detail.Status != "N" {
		t.Fatalf("unsigned commit status = %+v, want Status \"N\"", detail)
	}
}
