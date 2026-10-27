package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ignoreSanitizerTrackedArtifacts are committed fixtures that AuditIgnoreRules
// classifies as dangerous build artifacts: Library/*, Temp/* and the *.apk
// suffix matcher.
var ignoreSanitizerTrackedArtifacts = []string{
	"Library/cache.bin",
	"Temp/foo.log",
	"Builds/game.apk",
}

// newIgnoreSanitizerRepo builds a throwaway repository with isolated git config
// (no user/system config is consulted) and commits the given fixture paths.
func newIgnoreSanitizerRepo(t *testing.T, files []string) string {
	t.Helper()

	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Ignore Sanitizer Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "sanitizer@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Ignore Sanitizer Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "sanitizer@example.invalid")

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "Assets"), 0o755); err != nil {
		t.Fatalf("create Assets dir: %v", err)
	}
	for _, rel := range files {
		full := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("create dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte("fixture\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	ignoreSanitizerGit(t, repo, "init", "-q", "-b", "main")
	ignoreSanitizerGit(t, repo, "add", "-A")
	ignoreSanitizerGit(t, repo, "commit", "-q", "-m", "seed build artifacts")
	return repo
}

func ignoreSanitizerGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestApplyIgnoreSanitizerBatchesUntrack(t *testing.T) {
	repo := newIgnoreSanitizerRepo(t, ignoreSanitizerTrackedArtifacts)

	res, err := (GitService{}).ApplyIgnoreSanitizer(repo, "Unity 3D", true)
	if err != nil {
		t.Fatalf("apply ignore sanitizer: %v", err)
	}
	if !res.Success {
		t.Fatalf("full success expected, got success=%v failed=%v message=%q", res.Success, res.FailedFiles, res.Message)
	}
	if len(res.FailedFiles) != 0 {
		t.Fatalf("no failures expected, got %v", res.FailedFiles)
	}
	untracked := make(map[string]bool, len(res.UntrackedFiles))
	for _, p := range res.UntrackedFiles {
		untracked[p] = true
	}
	for _, want := range ignoreSanitizerTrackedArtifacts {
		if !untracked[want] {
			t.Fatalf("%s missing from untrackedFiles %v", want, res.UntrackedFiles)
		}
	}

	remaining := ignoreSanitizerGit(t, repo, "ls-files")
	for _, want := range ignoreSanitizerTrackedArtifacts {
		if strings.Contains(remaining, want) {
			t.Fatalf("%s is still tracked after sanitizing:\n%s", want, remaining)
		}
	}
}

func TestApplyIgnoreSanitizerReportsBatchFailure(t *testing.T) {
	repo := newIgnoreSanitizerRepo(t, ignoreSanitizerTrackedArtifacts)

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate real git: %v", err)
	}
	wrapperDir := t.TempDir()
	wrapper := fmt.Sprintf(`#!/bin/sh
real_git='%s'
has_rm=0
has_cached=0
for arg in "$@"; do
  case "$arg" in
    rm) has_rm=1 ;;
    --cached) has_cached=1 ;;
  esac
done
if [ "$has_rm" -eq 1 ] && [ "$has_cached" -eq 1 ]; then
  echo "fatal: simulated rm --cached failure" >&2
  exit 1
fi
exec "$real_git" "$@"
`, realGit)
	wrapperPath := filepath.Join(wrapperDir, "git")
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0o755); err != nil {
		t.Fatalf("write fake git wrapper: %v", err)
	}
	t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	res, err := (GitService{}).ApplyIgnoreSanitizer(repo, "Unity 3D", true)
	if err != nil {
		t.Fatalf("apply ignore sanitizer should not fail the RPC on untrack errors: %v", err)
	}
	if res.Success {
		t.Fatalf("partial failure must not report success: %+v", res)
	}
	if len(res.UntrackedFiles) != 0 {
		t.Fatalf("nothing was removed, untrackedFiles must stay empty: %v", res.UntrackedFiles)
	}
	if len(res.FailedFiles) != len(ignoreSanitizerTrackedArtifacts) {
		t.Fatalf("expected %d failures, got %d: %v", len(ignoreSanitizerTrackedArtifacts), len(res.FailedFiles), res.FailedFiles)
	}
	for _, msg := range res.FailedFiles {
		if !strings.Contains(msg, "simulated rm --cached failure") {
			t.Fatalf("failure entry must carry the batch error: %q", msg)
		}
	}
	if !strings.Contains(res.Message, "失败") {
		t.Fatalf("message must mention failures: %q", res.Message)
	}

	remaining := ignoreSanitizerGit(t, repo, "ls-files")
	for _, want := range ignoreSanitizerTrackedArtifacts {
		if !strings.Contains(remaining, want) {
			t.Fatalf("%s must remain tracked after the failed batch:\n%s", want, remaining)
		}
	}
}
