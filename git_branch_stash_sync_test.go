package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchStashSyncWorkflowsInTempRepo(t *testing.T) {
	svc := GitService{}
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@example.com")

	writeAndCommit := func(path, content, message string) string {
		t.Helper()
		full := filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := svc.StageFile(repo, path); err != nil {
			t.Fatalf("StageFile(%s) failed: %v", path, err)
		}
		if err := svc.Commit(repo, message, false); err != nil {
			t.Fatalf("Commit(%s) failed: %v", message, err)
		}
		hash, err := ExecGit(repo, "rev-parse", "HEAD")
		if err != nil {
			t.Fatalf("rev-parse HEAD failed: %v", err)
		}
		return strings.TrimSpace(hash)
	}
	currentBranch := func() string {
		t.Helper()
		name, err := ExecGit(repo, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			t.Fatalf("resolve current branch failed: %v", err)
		}
		return strings.TrimSpace(name)
	}

	writeAndCommit("base.txt", "base\n", "chore: base")
	baseBranch := currentBranch()
	baseHash, _ := ExecGit(repo, "rev-parse", "HEAD")
	baseHash = strings.TrimSpace(baseHash)

	// Branch create / checkout / rename / delete.
	if err := svc.CreateBranch(repo, "feature", ""); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	branches, err := svc.GetBranches(repo)
	if err != nil {
		t.Fatalf("GetBranches failed: %v", err)
	}
	var featureFound bool
	for _, b := range branches {
		if b.Name == "feature" && !b.IsCurrent {
			featureFound = true
		}
	}
	if !featureFound {
		t.Fatalf("expected a non-current feature branch, got %+v", branches)
	}
	if err := svc.CheckoutBranch(repo, "feature", false); err != nil {
		t.Fatalf("CheckoutBranch(feature) failed: %v", err)
	}
	if got := currentBranch(); got != "feature" {
		t.Fatalf("expected current branch feature, got %q", got)
	}
	featureHash := writeAndCommit("feature.txt", "feature\n", "feat: feature work")
	if err := svc.RenameBranch(repo, "feature", "feature-renamed"); err != nil {
		t.Fatalf("RenameBranch failed: %v", err)
	}
	if got := currentBranch(); got != "feature-renamed" {
		t.Fatalf("expected current branch feature-renamed, got %q", got)
	}

	// Annotated tag creation and listing.
	if err := svc.CreateTag(repo, "v1.0", featureHash, "release 1"); err != nil {
		t.Fatalf("CreateTag failed: %v", err)
	}
	tags, err := svc.GetTags(repo)
	if err != nil {
		t.Fatalf("GetTags failed: %v", err)
	}
	var tagFound bool
	for _, tag := range tags {
		if tag.Name == "v1.0" {
			tagFound = tag.IsAnnotated && strings.HasPrefix(featureHash, tag.CommitHash) && strings.Contains(tag.Message, "release 1")
		}
	}
	if !tagFound {
		t.Fatalf("expected annotated tag v1.0 at %s, got %+v", featureHash, tags)
	}

	if err := svc.CheckoutBranch(repo, baseBranch, false); err != nil {
		t.Fatalf("checkout base failed: %v", err)
	}
	if err := svc.DeleteBranch(repo, "feature-renamed", true); err != nil {
		t.Fatalf("DeleteBranch failed: %v", err)
	}

	// Stash save / apply / pop / branch.
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("temp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StashSave(repo, "wip-one", false, true); err != nil {
		t.Fatalf("StashSave failed: %v", err)
	}
	stashes, err := svc.GetStashes(repo)
	if err != nil || len(stashes) != 1 || !strings.Contains(stashes[0].Message, "wip-one") {
		t.Fatalf("expected stash message wip-one, got %+v (err=%v)", stashes, err)
	}
	if status, err := svc.GetRepoStatus(repo); err != nil || !status.IsClean {
		t.Fatalf("workspace should be clean after stash, status=%+v err=%v", status, err)
	}
	if err := svc.StashApply(repo, 0); err != nil {
		t.Fatalf("StashApply failed: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(repo, "base.txt")); err != nil || !strings.Contains(string(data), "base changed") {
		t.Fatalf("StashApply did not restore tracked change: %q err=%v", data, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "untracked.txt")); err != nil {
		t.Fatalf("StashApply did not restore untracked file: %v", err)
	}
	if stashes, _ := svc.GetStashes(repo); len(stashes) != 1 {
		t.Fatalf("StashApply should keep the stash, got %+v", stashes)
	}
	if err := svc.DiscardAll(repo); err != nil {
		t.Fatalf("DiscardAll failed: %v", err)
	}
	if err := svc.StashPop(repo, 0); err != nil {
		t.Fatalf("StashPop failed: %v", err)
	}
	if stashes, _ := svc.GetStashes(repo); len(stashes) != 0 {
		t.Fatalf("StashPop should drop the stash, got %+v", stashes)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "base.txt")); !strings.Contains(string(data), "base changed") {
		t.Fatalf("StashPop did not restore tracked change: %q", data)
	}

	if err := svc.StashSave(repo, "wip-two", false, true); err != nil {
		t.Fatalf("second StashSave failed: %v", err)
	}
	if err := svc.StashBranch(repo, 0, "stash-recovery"); err != nil {
		t.Fatalf("StashBranch failed: %v", err)
	}
	if got := currentBranch(); got != "stash-recovery" {
		t.Fatalf("expected current branch stash-recovery, got %q", got)
	}
	if stashes, _ := svc.GetStashes(repo); len(stashes) != 0 {
		t.Fatalf("StashBranch should consume the stash, got %+v", stashes)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "base.txt")); !strings.Contains(string(data), "base changed") {
		t.Fatalf("StashBranch did not restore tracked change: %q", data)
	}
	if err := svc.DiscardAll(repo); err != nil {
		t.Fatalf("DiscardAll after stash branch failed: %v", err)
	}
	if err := svc.CheckoutBranch(repo, baseBranch, false); err != nil {
		t.Fatalf("checkout base after stash branch failed: %v", err)
	}
	if err := svc.DeleteBranch(repo, "stash-recovery", true); err != nil {
		t.Fatalf("DeleteBranch(stash-recovery) failed: %v", err)
	}

	// Local bare remote: add / push / fetch / pull / upstream / tag sync.
	remote := filepath.Join(parent, "remote.git")
	if _, err := ExecGit(parent, "init", "--bare", remote); err != nil {
		t.Fatalf("init bare remote failed: %v", err)
	}
	if err := svc.AddRemote(repo, "origin", remote); err != nil {
		t.Fatalf("AddRemote failed: %v", err)
	}
	remotes, err := svc.GetRemoteDetails(repo)
	if err != nil || len(remotes) < 2 {
		t.Fatalf("expected fetch/push entries for origin, got %+v (err=%v)", remotes, err)
	}
	for _, remoteDetail := range remotes {
		if remoteDetail.Name != "origin" || remoteDetail.URL != remote {
			t.Fatalf("unexpected remote entry %+v", remoteDetail)
		}
	}
	if _, err := svc.Push(repo, "origin", baseBranch, false); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if err := svc.SetUpstream(repo, baseBranch, "origin/"+baseBranch); err != nil {
		t.Fatalf("SetUpstream failed: %v", err)
	}
	branches, _ = svc.GetBranches(repo)
	var tracked bool
	for _, b := range branches {
		if b.Name == baseBranch && b.IsCurrent && b.Upstream == "origin/"+baseBranch {
			tracked = true
		}
	}
	if !tracked {
		t.Fatalf("expected %s to track origin/%s, got %+v", baseBranch, baseBranch, branches)
	}
	if _, err := svc.Fetch(repo, "origin"); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if out, err := svc.Pull(repo, "origin", baseBranch); err != nil {
		t.Fatalf("Pull failed: %v (%s)", err, out)
	}
	if _, err := svc.PushTag(repo, "origin", "v1.0"); err != nil {
		t.Fatalf("PushTag failed: %v", err)
	}
	lsTags, err := ExecGit(repo, "ls-remote", "--tags", "origin")
	if err != nil || !strings.Contains(lsTags, "refs/tags/v1.0") {
		t.Fatalf("expected remote tag v1.0, got %q err=%v", lsTags, err)
	}
	if _, err := svc.DeleteRemoteTag(repo, "origin", "v1.0"); err != nil {
		t.Fatalf("DeleteRemoteTag failed: %v", err)
	}
	lsTags, _ = ExecGit(repo, "ls-remote", "--tags", "origin")
	if strings.Contains(lsTags, "refs/tags/v1.0") {
		t.Fatalf("remote tag v1.0 should be deleted, got %q", lsTags)
	}
	if err := svc.DeleteTag(repo, "v1.0"); err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}
	if err := svc.RemoveRemote(repo, "origin"); err != nil {
		t.Fatalf("RemoveRemote failed: %v", err)
	}
	if remotes, _ := svc.GetRemoteDetails(repo); len(remotes) != 0 {
		t.Fatalf("expected no remotes after removal, got %+v", remotes)
	}

	// Revert and reset.
	revertTarget := writeAndCommit("base.txt", "to be reverted\n", "change: revert me")
	if err := svc.RevertCommit(repo, revertTarget); err != nil {
		t.Fatalf("RevertCommit failed: %v", err)
	}
	subject, err := ExecGit(repo, "log", "-1", "--format=%s")
	if err != nil || !strings.Contains(subject, `Revert "change: revert me"`) {
		t.Fatalf("expected revert commit, got %q err=%v", subject, err)
	}
	if err := svc.ResetBranch(repo, baseHash, "hard"); err != nil {
		t.Fatalf("ResetBranch failed: %v", err)
	}
	if head, _ := ExecGit(repo, "rev-parse", "HEAD"); strings.TrimSpace(head) != strings.TrimSpace(baseHash) {
		t.Fatalf("expected HEAD at %s after reset, got %q", baseHash, head)
	}
	if status, _ := svc.GetRepoStatus(repo); !status.IsClean {
		t.Fatalf("workspace should be clean after hard reset, got %+v", status)
	}
}

func TestFastCommitWithoutRemoteReportsLocalOnly(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@example.com")
	if err := os.WriteFile(filepath.Join(repo, "fast.txt"), []byte("fast\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := svc.FastCommit(repo, "feat: local only")
	if err != nil || res == nil {
		t.Fatalf("FastCommit failed: %v res=%+v", err, res)
	}
	if !res.Success || res.Pushed {
		t.Fatalf("expected local success without push, got %+v", res)
	}
	if strings.Contains(res.Message, "rebase has conflict") {
		t.Fatalf("no-upstream failure must not be reported as a conflict: %q", res.Message)
	}
	if !strings.Contains(res.Message, "pull --rebase did not complete") {
		t.Fatalf("expected an accurate pull failure message, got %q", res.Message)
	}
	subject, err := ExecGit(repo, "log", "-1", "--format=%s")
	if err != nil || strings.TrimSpace(subject) != "feat: local only" {
		t.Fatalf("expected local commit to exist, subject=%q err=%v", subject, err)
	}
	if status, _ := svc.GetRepoStatus(repo); !status.IsClean {
		t.Fatalf("workspace should be clean after FastCommit, got %+v", status)
	}
}

func TestDiscardAllHandlesEmptyRepoAndRejectsNonRepo(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "loose.txt"), []byte("untracked"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.DiscardAll(repo); err != nil {
		t.Fatalf("DiscardAll in an empty repository should still clean untracked files: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "loose.txt")); !os.IsNotExist(err) {
		t.Fatalf("untracked file was not cleaned: %v", err)
	}
	if err := svc.DiscardAll(t.TempDir()); err == nil {
		t.Fatal("expected error when discarding outside a Git repository")
	}
	if err := svc.DiscardAll(""); err == nil {
		t.Fatal("expected error for empty repository path")
	}
}

func TestDiscardAllPropagatesRestoreFailure(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "sub/a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	if err := svc.DiscardAll(repo); err == nil {
		t.Fatal("DiscardAll must fail when tracked files cannot be restored")
	}
	if data, err := os.ReadFile(filepath.Join(sub, "a.txt")); err != nil || string(data) != "new\n" {
		t.Fatalf("failed restore must not be reported as success: %q err=%v", data, err)
	}
}

func TestFastCommitReportsPushRejectionReason(t *testing.T) {
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	svc := GitService{}
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "base.txt"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatal(err)
	}

	bare := filepath.Join(parent, "remote.git")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(bare, "init", "--bare"); err != nil {
		t.Fatalf("init bare remote: %v", err)
	}
	if _, err := ExecGit(repo, "remote", "add", "origin", bare); err != nil {
		t.Fatal(err)
	}
	branchOut, err := ExecGit(repo, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		t.Fatalf("resolve current branch: %v", err)
	}
	if _, err := ExecGit(repo, "push", "-u", "origin", strings.TrimSpace(branchOut)); err != nil {
		t.Fatalf("initial push failed: %v", err)
	}
	hook := filepath.Join(bare, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho 'rejected by test hook' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := svc.FastCommit(repo, "feat: rejected push")
	if err != nil {
		t.Fatalf("FastCommit failed: %v", err)
	}
	if res == nil || !res.Success || res.Pushed {
		t.Fatalf("expected local success without push: %+v", res)
	}
	if !strings.Contains(res.Message, "rejected by test hook") {
		t.Fatalf("push failure message must include the remote reason: %q", res.Message)
	}
}
