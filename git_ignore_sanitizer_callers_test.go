package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installFailingRmCachedGit puts a git wrapper at the front of PATH that
// delegates everything to the real git except `git rm --cached`, which fails
// like a locked index would. It mirrors the wrapper used by
// TestApplyIgnoreSanitizerReportsBatchFailure.
func installFailingRmCachedGit(t *testing.T) {
	t.Helper()

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
}

func TestIgnoreSanitizerCallersRunHealthFixReportsFailure(t *testing.T) {
	repo := newIgnoreSanitizerRepo(t, ignoreSanitizerTrackedArtifacts)
	installFailingRmCachedGit(t)

	res, err := (GitService{}).RunHealthFix(repo, "ignore")
	if err != nil {
		t.Fatalf("RunHealthFix must not fail the RPC on partial untrack errors: %v", err)
	}
	if !res.Executed {
		t.Fatalf("expected the ignore fix to execute, got %+v", res)
	}
	if res.Success {
		t.Fatalf("partial untrack failure must not be reported as success: %+v", res)
	}
	if res.Output == "" || !strings.Contains(res.Output, "移出失败") {
		t.Fatalf("output must mention the failed untracks, got %q", res.Output)
	}
	if !strings.Contains(res.Output, fmt.Sprintf("%d 个移出失败", len(ignoreSanitizerTrackedArtifacts))) {
		t.Fatalf("output must carry the failure count, got %q", res.Output)
	}
	if res.Hint == "" || !strings.Contains(res.Hint, "失败") {
		t.Fatalf("hint must explain the partial failure and retry path, got %q", res.Hint)
	}

	remaining := ignoreSanitizerGit(t, repo, "ls-files")
	for _, want := range ignoreSanitizerTrackedArtifacts {
		if !strings.Contains(remaining, want) {
			t.Fatalf("%s must remain tracked after the failed batch:\n%s", want, remaining)
		}
	}
}

func TestIgnoreSanitizerCallersRunEngineBootstrapReportsFailure(t *testing.T) {
	repo := newIgnoreSanitizerRepo(t, ignoreSanitizerTrackedArtifacts)
	installFailingRmCachedGit(t)

	res, err := (GitService{}).RunEngineBootstrap(repo, EngineBootstrapOptions{
		Engine:             "unity",
		WriteGitattributes: true,
		WriteGitignore:     true,
		UntrackArtifacts:   true,
	})
	if err != nil {
		t.Fatalf("RunEngineBootstrap failed: %v", err)
	}

	steps := make(map[string]EngineBootstrapStep, len(res.Steps))
	for _, step := range res.Steps {
		steps[step.ID] = step
	}
	for _, id := range []string{"gitattributes", "gitignore", "unity-merge", "lfs"} {
		if _, ok := steps[id]; !ok {
			t.Fatalf("bootstrap must keep running every step, missing %q in %+v", id, res.Steps)
		}
	}

	gitignore, ok := steps["gitignore"]
	if !ok {
		t.Fatalf("missing gitignore step: %+v", res.Steps)
	}
	if gitignore.Status != "failed" {
		t.Fatalf("partial untrack failure must mark the gitignore step failed, got %q (%s)", gitignore.Status, gitignore.Detail)
	}
	if !strings.Contains(gitignore.Detail, "移出失败") {
		t.Fatalf("gitignore detail must mention the failure count, got %q", gitignore.Detail)
	}
	if !strings.Contains(gitignore.Detail, "simulated rm --cached failure") {
		t.Fatalf("gitignore detail must carry the first failure context, got %q", gitignore.Detail)
	}
	if steps["gitattributes"].Status != "applied" {
		t.Fatalf("earlier steps must still apply, got %q", steps["gitattributes"].Status)
	}
	if res.Failed == 0 || res.Success {
		t.Fatalf("bootstrap result must reflect the failed step: failed=%d success=%v", res.Failed, res.Success)
	}

	remaining := ignoreSanitizerGit(t, repo, "ls-files")
	for _, want := range ignoreSanitizerTrackedArtifacts {
		if !strings.Contains(remaining, want) {
			t.Fatalf("%s must remain tracked after the failed batch:\n%s", want, remaining)
		}
	}
}
