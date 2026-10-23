package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergedBranchesCherryPickRangeAndCommitTemplate(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	writeCommit := func(name, message string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(message+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := svc.StageFile(repo, name); err != nil {
			t.Fatalf("StageFile(%s) failed: %v", name, err)
		}
		if err := svc.Commit(repo, message, false); err != nil {
			t.Fatalf("Commit(%s) failed: %v", message, err)
		}
		hash, _ := ExecGit(repo, "rev-parse", "HEAD")
		return strings.TrimSpace(hash)
	}
	baseHash := writeCommit("base.txt", "chore: base")
	baseBranch, _ := ExecGit(repo, "symbolic-ref", "--short", "HEAD")
	baseBranch = strings.TrimSpace(baseBranch)

	if err := svc.CreateBranch(repo, "merged-topic", ""); err != nil {
		t.Fatalf("CreateBranch merged-topic failed: %v", err)
	}
	if err := svc.CheckoutBranch(repo, "merged-topic", false); err != nil {
		t.Fatalf("checkout merged-topic failed: %v", err)
	}
	writeCommit("merged.txt", "feat: merged")
	if err := svc.CheckoutBranch(repo, baseBranch, false); err != nil {
		t.Fatalf("checkout base failed: %v", err)
	}
	if _, err := ExecGit(repo, "merge", "--no-ff", "-m", "merge: merged-topic", "merged-topic"); err != nil {
		t.Fatalf("merge merged-topic failed: %v", err)
	}

	if err := svc.CreateBranch(repo, "unmerged-topic", ""); err != nil {
		t.Fatalf("CreateBranch unmerged-topic failed: %v", err)
	}
	if err := svc.CheckoutBranch(repo, "unmerged-topic", false); err != nil {
		t.Fatalf("checkout unmerged-topic failed: %v", err)
	}
	writeCommit("unmerged.txt", "feat: unmerged")
	if err := svc.CheckoutBranch(repo, baseBranch, false); err != nil {
		t.Fatalf("checkout base failed: %v", err)
	}
	protectedBranches := []string{"release/1.0", "hotfix/urgent", "develop/next", "support/2.x"}
	for _, name := range protectedBranches {
		if _, err := ExecGit(repo, "branch", name, "HEAD"); err != nil {
			t.Fatalf("create protected branch %s failed: %v", name, err)
		}
	}

	merged, err := svc.GetMergedBranches(repo, "HEAD")
	if err != nil {
		t.Fatalf("GetMergedBranches failed: %v", err)
	}
	var sawMerged, sawUnmerged bool
	for _, branch := range merged {
		if branch == "merged-topic" {
			sawMerged = true
		}
		if branch == "unmerged-topic" {
			sawUnmerged = true
		}
		if protectedBranchName(branch) {
			t.Fatalf("long-lived branch %q must not be listed for cleanup: %+v", branch, merged)
		}
	}
	if !sawMerged || sawUnmerged {
		t.Fatalf("unexpected merged branch list: %+v", merged)
	}
	deleteRequest := append([]string{"merged-topic", "unmerged-topic", "", "-D", "main", "master"}, protectedBranches...)
	deleted, err := svc.DeleteMergedBranches(repo, deleteRequest)
	if err != nil || len(deleted) != 1 || deleted[0] != "merged-topic" {
		t.Fatalf("DeleteMergedBranches failed: %+v err=%v", deleted, err)
	}
	branches, _ := svc.GetBranches(repo)
	stillExists := map[string]bool{}
	for _, branch := range branches {
		stillExists[branch.Name] = true
	}
	if !stillExists["unmerged-topic"] {
		t.Fatal("unmerged branch must not be deleted by branch -d")
	}
	for _, name := range protectedBranches {
		if !stillExists[name] {
			t.Fatalf("protected branch %s must not be deleted", name)
		}
	}

	if err := svc.CreateBranch(repo, "range-topic", baseHash); err != nil {
		t.Fatalf("CreateBranch range-topic failed: %v", err)
	}
	if err := svc.CheckoutBranch(repo, "range-topic", false); err != nil {
		t.Fatalf("checkout range-topic failed: %v", err)
	}
	writeCommit("range-a.txt", "feat: range a")
	rangeEnd := writeCommit("range-b.txt", "feat: range b")
	if err := svc.CheckoutBranch(repo, baseBranch, false); err != nil {
		t.Fatalf("checkout base before cherry-pick failed: %v", err)
	}
	if out, err := svc.CherryPickRange(repo, baseHash, rangeEnd); err != nil {
		t.Fatalf("CherryPickRange failed: %v (%s)", err, out)
	}
	for _, file := range []string{"range-a.txt", "range-b.txt"} {
		if _, err := os.Stat(filepath.Join(repo, file)); err != nil {
			t.Fatalf("cherry-picked file %s missing: %v", file, err)
		}
	}
	if out, err := svc.CherryPickRange(repo, "", rangeEnd); err == nil {
		t.Fatalf("expected error for empty cherry-pick range, got %q", out)
	}

	if err := svc.SaveCommitTemplate(repo, "type(scope): subject\n\nbody\n", true); err != nil {
		t.Fatalf("SaveCommitTemplate failed: %v", err)
	}
	tmpl, err := svc.GetCommitTemplate(repo)
	if err != nil || tmpl == nil || !tmpl.HasTemplate || tmpl.ConfiguredPath != ".gitmessage" || !strings.Contains(tmpl.Content, "type(scope)") {
		t.Fatalf("GetCommitTemplate failed: %+v err=%v", tmpl, err)
	}
	configured, _ := ExecGit(repo, "config", "--get", "commit.template")
	if strings.TrimSpace(configured) != ".gitmessage" {
		t.Fatalf("expected commit.template config, got %q", configured)
	}
	if err := svc.SaveCommitTemplate(repo, "second template\n", false); err != nil {
		t.Fatalf("SaveCommitTemplate without config failed: %v", err)
	}
	if tmpl, _ := svc.GetCommitTemplate(repo); !strings.Contains(tmpl.Content, "second template") {
		t.Fatalf("template content was not updated: %+v", tmpl)
	}
}
