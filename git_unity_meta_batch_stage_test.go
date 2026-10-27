package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newUnityMetaBatchRepo creates an isolated repository: the user/system git
// config is neutralized so host settings (hooks, signing, autocrlf) cannot
// influence staging behavior.
func newUnityMetaBatchRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-q")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	runGitTest(t, repo, nil, "commit", "-q", "--allow-empty", "-m", "base")
	return repo
}

func writeUnityMetaBatchFile(t *testing.T, repo, relPath, content string) {
	t.Helper()
	full := filepath.Join(repo, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// installUnityMetaBatchGitShim puts a fake `git` first on PATH. Every `add`
// invocation is appended to logPath; when rejectAdd is set the shim fails that
// invocation instead of delegating, exactly like a broken index lock would.
// All other invocations (status, ls-files, diff --cached, ...) delegate to the
// real binary resolved by the caller before PATH is mutated.
func installUnityMetaBatchGitShim(t *testing.T, realGit, logPath string, rejectAdd bool) {
	t.Helper()
	reject := ""
	if rejectAdd {
		reject = `echo "simulated batch add rejection" >&2
    exit 1`
	}
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "add" ]; then
  printf '%%s\n' "$*" >> %q
  %s
fi
exec %q "$@"
`, logPath, reject, realGit)
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func readUnityMetaBatchAddLog(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	lines := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// TestUnityMetaBatchStageSingleAdd proves that ten unpaired asset/meta gaps are
// fixed by exactly one `git add` process, with every path staged and reported
// in deterministic (first-seen) order.
func TestUnityMetaBatchStageSingleAdd(t *testing.T) {
	repo := newUnityMetaBatchRepo(t)

	const assetCount = 10
	assets := make([]string, 0, assetCount)
	expected := make([]string, 0, assetCount*2)
	for i := 0; i < assetCount; i++ {
		asset := fmt.Sprintf("Assets/asset%02d.png", i)
		writeUnityMetaBatchFile(t, repo, asset, "png")
		writeUnityMetaBatchFile(t, repo, asset+".meta", "guid: asset\n")
		// Stage the asset only; the untracked meta makes the pair unpaired.
		assets = append(assets, asset)
		expected = append(expected, asset+".meta", asset)
	}
	runGitTest(t, repo, nil, append([]string{"add", "--"}, assets...)...)

	mismatches, err := GitService{}.CheckUnityMetaSync(repo)
	if err != nil {
		t.Fatalf("CheckUnityMetaSync: %v", err)
	}
	if len(mismatches) != assetCount {
		t.Fatalf("got %d mismatches, want %d: %+v", len(mismatches), assetCount, mismatches)
	}
	for _, m := range mismatches {
		if m.IssueType != "unpaired_staged" {
			t.Fatalf("unexpected mismatch type: %+v", m)
		}
	}

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "add.log")
	installUnityMetaBatchGitShim(t, realGit, logPath, false)

	fixed, err := GitService{}.AutoFixUnityMeta(repo)
	if err != nil {
		t.Fatalf("AutoFixUnityMeta: %v", err)
	}
	if len(fixed) != len(expected) {
		t.Fatalf("fixed=%v, want %v", fixed, expected)
	}
	for i := range expected {
		if fixed[i] != expected[i] {
			t.Fatalf("fixed order = %v, want %v", fixed, expected)
		}
	}

	cached := runGitTest(t, repo, nil, "ls-files", "-z")
	staged := make(map[string]bool)
	for _, p := range strings.Split(cached, "\x00") {
		if p != "" {
			staged[p] = true
		}
	}
	for _, p := range expected {
		if !staged[p] {
			t.Errorf("%s was not staged; index=%v", p, staged)
		}
	}

	adds := readUnityMetaBatchAddLog(t, logPath)
	if len(adds) != 1 {
		t.Fatalf("expected exactly one git add invocation for %d gaps, got %d: %v", assetCount, len(adds), adds)
	}
	if !strings.HasPrefix(adds[0], "add -- ") {
		t.Errorf("unexpected add invocation: %q", adds[0])
	}

	after, err := GitService{}.CheckUnityMetaSync(repo)
	if err != nil {
		t.Fatalf("CheckUnityMetaSync after fix: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("expected no mismatches after batch fix, got %+v", after)
	}
}

// TestUnityMetaBatchStageFailureVerification proves the fail-closed path: when
// the batch `add` is rejected, the `diff --cached` verification decides what is
// fixed, and nothing that the failed batch did not stage may be reported.
func TestUnityMetaBatchStageFailureVerification(t *testing.T) {
	repo := newUnityMetaBatchRepo(t)

	asset := "Assets/broken.png"
	meta := asset + ".meta"
	writeUnityMetaBatchFile(t, repo, asset, "png")
	writeUnityMetaBatchFile(t, repo, meta, "guid: broken\n")
	runGitTest(t, repo, nil, "add", "--", meta)
	// Remove the staged meta from the worktree: the pair stays unpaired_staged
	// (staged meta, untracked asset) but the only path the batch add receives is
	// the asset, which the rejected add must leave untracked.
	if err := os.Remove(filepath.Join(repo, meta)); err != nil {
		t.Fatal(err)
	}

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "add.log")
	installUnityMetaBatchGitShim(t, realGit, logPath, true)

	fixed, err := GitService{}.AutoFixUnityMeta(repo)
	if err == nil || !strings.Contains(err.Error(), "部分配对未能暂存") {
		t.Fatalf("batch add failure must surface, fixed=%v err=%v", fixed, err)
	}
	if len(fixed) != 0 {
		t.Fatalf("failed batch must not report paths as fixed: %v", fixed)
	}

	cached := runGitTest(t, repo, nil, "ls-files", "-z")
	for _, p := range strings.Split(cached, "\x00") {
		if p == asset {
			t.Fatalf("%s must stay untracked after a rejected add; index=%q", asset, cached)
		}
	}
	if _, statErr := os.Stat(filepath.Join(repo, asset)); statErr != nil {
		t.Fatalf("asset must remain on disk: %v", statErr)
	}

	adds := readUnityMetaBatchAddLog(t, logPath)
	if len(adds) != 1 {
		t.Fatalf("expected exactly one git add invocation, got %d: %v", len(adds), adds)
	}
}
