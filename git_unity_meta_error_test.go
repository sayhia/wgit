package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installUnityMetaGitWrapper puts a fake `git` first on PATH. The fake delegates
// every invocation to the real binary, except the ones rejected by failScript
// (a /bin/sh fragment that may exit 128). realGit must be resolved by the
// caller before PATH is mutated.
func installUnityMetaGitWrapper(t *testing.T, realGit string, failScript string) {
	t.Helper()
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
%s
exec %q "$@"
`, failScript, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func newUnityMetaControlRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-q")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	runGitTest(t, repo, nil, "commit", "-q", "--allow-empty", "-m", "base")
	return repo
}

func TestCheckUnityMetaSyncReportsTrackedSetFailure(t *testing.T) {
	repo := newUnityMetaControlRepo(t)

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	// Only the tracked-set probe (`git ls-files -z`) fails; every other call,
	// including `git status` and the Assets/Packages enumeration, is real.
	installUnityMetaGitWrapper(t, realGit, `if [ "$#" -eq 2 ] && [ "$1" = "ls-files" ] && [ "$2" = "-z" ]; then
  echo "simulated tracked-set read failure" >&2
  exit 128
fi`)

	mismatches, err := GitService{}.CheckUnityMetaSync(repo)
	if err == nil || !strings.Contains(err.Error(), "读取已跟踪文件失败") {
		t.Fatalf("tracked-set read failure must surface, got err=%v mismatches=%+v", err, mismatches)
	}
	if mismatches != nil {
		t.Errorf("failed read must not return partial mismatches: %+v", mismatches)
	}
}

func TestCheckUnityMetaSyncReportsAssetEnumerationFailure(t *testing.T) {
	repo := newUnityMetaControlRepo(t)

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	// Fail only the enumeration call (the one combining --cached and --others).
	installUnityMetaGitWrapper(t, realGit, `has_cached=0
has_others=0
for arg in "$@"; do
  [ "$arg" = "--cached" ] && has_cached=1
  [ "$arg" = "--others" ] && has_others=1
done
if [ "$has_cached" = "1" ] && [ "$has_others" = "1" ]; then
  echo "simulated asset enumeration failure" >&2
  exit 128
fi`)

	mismatches, err := GitService{}.CheckUnityMetaSync(repo)
	if err == nil || !strings.Contains(err.Error(), "读取 Unity 资产列表失败") {
		t.Fatalf("enumeration failure must surface, got err=%v mismatches=%+v", err, mismatches)
	}
	if mismatches != nil {
		t.Errorf("failed enumeration must not be reported as no mismatches: %+v", mismatches)
	}
}

func TestCheckUnityMetaSyncRealRepoControl(t *testing.T) {
	t.Run("asset missing meta", func(t *testing.T) {
		repo := newUnityMetaControlRepo(t)
		assetPath := filepath.Join(repo, "Assets", "hero.png")
		if err := os.MkdirAll(filepath.Dir(assetPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(assetPath, []byte("png"), 0o644); err != nil {
			t.Fatal(err)
		}

		mismatches, err := GitService{}.CheckUnityMetaSync(repo)
		if err != nil {
			t.Fatalf("CheckUnityMetaSync: %v", err)
		}
		if len(mismatches) != 1 {
			t.Fatalf("got %d mismatches, want 1: %+v", len(mismatches), mismatches)
		}
		if mismatches[0].AssetPath != "Assets/hero.png" || mismatches[0].IssueType != "missing_meta" {
			t.Errorf("unexpected mismatch: %+v", mismatches[0])
		}
	})

	t.Run("empty repo", func(t *testing.T) {
		repo := t.TempDir()
		if _, err := ExecGit(repo, "init", "-q"); err != nil {
			t.Fatalf("git init: %v", err)
		}
		mismatches, err := GitService{}.CheckUnityMetaSync(repo)
		if err != nil {
			t.Fatalf("empty repo must succeed: %v", err)
		}
		if mismatches == nil || len(mismatches) != 0 {
			t.Fatalf("empty repo must return an empty slice, got %+v", mismatches)
		}
	})
}
