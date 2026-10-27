package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newTagDeleteRepo creates an isolated repository with one commit so tags can
// be created without an explicit target. All git config is local so the
// developer's global/system settings never leak into the assertions.
func newTagDeleteRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageAll(repo); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	return repo
}

func localTagNames(t *testing.T, repo string) []string {
	t.Helper()
	out, err := ExecGit(repo, "tag", "--list")
	if err != nil {
		t.Fatalf("git tag --list failed: %v", err)
	}
	names := []string{}
	for _, line := range strings.Split(out, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// TestDeleteBatchTagsParity pins the previous per-tag semantics on the batched
// path: only locally removed tags are reported, absent/invalid names surface in
// the aggregate error, and option-like names are never treated as flags.
func TestDeleteBatchTagsParity(t *testing.T) {
	svc := GitService{}
	repo := newTagDeleteRepo(t)

	for _, tag := range []string{"t1", "t2"} {
		if err := svc.CreateTag(repo, tag, "", ""); err != nil {
			t.Fatalf("CreateTag(%s) failed: %v", tag, err)
		}
	}
	deleted, err := svc.DeleteBatchTags(repo, []string{"t1", "t2"}, false, "")
	if err != nil {
		t.Fatalf("DeleteBatchTags failed: %v", err)
	}
	if len(deleted) != 2 || deleted[0] != "t1" || deleted[1] != "t2" {
		t.Fatalf("deleted = %+v, want [t1 t2] in input order", deleted)
	}
	if tags := localTagNames(t, repo); len(tags) != 0 {
		t.Fatalf("expected no tags after batch delete, got %+v", tags)
	}

	if err := svc.CreateTag(repo, "keep-me", "", ""); err != nil {
		t.Fatalf("CreateTag(keep-me) failed: %v", err)
	}
	deleted, err = svc.DeleteBatchTags(repo, []string{"keep-me", "missing-tag", "bad..tag"}, false, "")
	if len(deleted) != 1 || deleted[0] != "keep-me" {
		t.Fatalf("deleted = %+v, want [keep-me]", deleted)
	}
	if err == nil || !strings.Contains(err.Error(), "missing-tag") || !strings.Contains(err.Error(), "bad..tag") {
		t.Fatalf("aggregate error must name both failures: %v", err)
	}
	if !strings.Contains(err.Error(), "部分标签未能删除") {
		t.Fatalf("aggregate error wording changed: %v", err)
	}
	if tags := localTagNames(t, repo); len(tags) != 0 {
		t.Fatalf("successfully deleted tag should be gone, got %+v", tags)
	}

	if err := svc.CreateTag(repo, "keep-safe", "", ""); err != nil {
		t.Fatalf("CreateTag(keep-safe) failed: %v", err)
	}
	if err := svc.DeleteTag(repo, "--all"); err == nil {
		t.Fatal("option-like tag name must be treated as a literal tag, not a flag")
	}
	deleted, err = svc.DeleteBatchTags(repo, []string{"--all"}, false, "")
	if err == nil || len(deleted) != 0 {
		t.Fatalf("batch delete must not accept option-like tag names: %+v err=%v", deleted, err)
	}
	if tags := localTagNames(t, repo); len(tags) != 1 || tags[0] != "keep-safe" {
		t.Fatalf("option-like deletes must not remove real tags: %+v", tags)
	}
}

// TestDeleteBatchTagsDeduplicatesInput guarantees a repeated name collapses to
// one deletion instead of producing a bogus "not found" failure on the second
// occurrence.
func TestDeleteBatchTagsDeduplicatesInput(t *testing.T) {
	svc := GitService{}
	repo := newTagDeleteRepo(t)

	if err := svc.CreateTag(repo, "t1", "", ""); err != nil {
		t.Fatalf("CreateTag(t1) failed: %v", err)
	}
	deleted, err := svc.DeleteBatchTags(repo, []string{"t1", "t1"}, false, "")
	if err != nil {
		t.Fatalf("duplicate input must not fail: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "t1" {
		t.Fatalf("deleted = %+v, want exactly [t1]", deleted)
	}
	if tags := localTagNames(t, repo); len(tags) != 0 {
		t.Fatalf("expected no tags left, got %+v", tags)
	}
}

// TestDeleteBatchTagsSingleRemotePush proves the remote cleanup uses one
// `git push --delete` for the whole batch. A git shim delegates to the real
// binary but logs every push invocation.
func TestDeleteBatchTagsSingleRemotePush(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil || realGit == "" {
		t.Fatalf("real git is required for the shim: %v", err)
	}

	svc := GitService{}
	repo := newTagDeleteRepo(t)
	parent := filepath.Dir(repo)
	remoteDir := filepath.Join(parent, "remote.git")
	if _, err := ExecGit(parent, "init", "--bare", "-q", remoteDir); err != nil {
		t.Fatalf("init bare remote failed: %v", err)
	}
	run := func(dir string, args ...string) string {
		t.Helper()
		out, err := ExecGit(dir, args...)
		if err != nil {
			t.Fatalf("git %v in %s failed: %v (%s)", args, dir, err, out)
		}
		return out
	}
	run(repo, "remote", "add", "origin", remoteDir)
	branch := strings.TrimSpace(run(repo, "rev-parse", "--abbrev-ref", "HEAD"))
	run(repo, "push", "-q", "-u", "origin", branch)
	for _, tag := range []string{"r1", "r2"} {
		if err := svc.CreateTag(repo, tag, "", ""); err != nil {
			t.Fatalf("CreateTag(%s) failed: %v", tag, err)
		}
	}
	run(repo, "push", "-q", "origin", "--tags")

	shimDir := t.TempDir()
	logPath := filepath.Join(shimDir, "push.log")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "push" ]; then
  echo "$(date +%%s) push" >> %q
fi
exec %q "$@"
`, logPath, realGit)
	installFakeGit(t, shimDir, script)

	deleted, err := svc.DeleteBatchTags(repo, []string{"r1", "r2"}, true, "origin")
	if err != nil {
		t.Fatalf("remote batch delete failed: %v", err)
	}
	if len(deleted) != 2 || deleted[0] != "r1" || deleted[1] != "r2" {
		t.Fatalf("deleted = %+v, want [r1 r2]", deleted)
	}
	if tags := localTagNames(t, repo); len(tags) != 0 {
		t.Fatalf("expected no local tags left, got %+v", tags)
	}
	remoteTags, err := exec.Command(realGit, "--git-dir", remoteDir, "tag", "--list").CombinedOutput()
	if err != nil {
		t.Fatalf("list remote tags failed: %v (%s)", err, remoteTags)
	}
	if left := strings.TrimSpace(string(remoteTags)); left != "" {
		t.Fatalf("expected remote tags removed, still present: %q", left)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected one push invocation, log missing: %v", err)
	}
	pushes := 0
	for _, line := range strings.Split(strings.TrimSpace(string(logData)), "\n") {
		if strings.HasSuffix(line, " push") {
			pushes++
		}
	}
	if pushes != 1 {
		t.Fatalf("expected exactly 1 push for 2 tags, got %d: %q", pushes, string(logData))
	}
}
