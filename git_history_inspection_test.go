package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoryInspectionAndRecoveryInTempRepo(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(files []string, message string) string {
		t.Helper()
		for _, file := range files {
			if err := svc.StageFile(repo, file); err != nil {
				t.Fatalf("StageFile(%s) failed: %v", file, err)
			}
		}
		if err := svc.Commit(repo, message, false); err != nil {
			t.Fatalf("Commit(%s) failed: %v", message, err)
		}
		hash, _ := ExecGit(repo, "rev-parse", "HEAD")
		return strings.TrimSpace(hash)
	}

	write("app.txt", "one\ntwo\n")
	c1 := commit([]string{"app.txt"}, "feat: first")
	baseBranch, _ := ExecGit(repo, "symbolic-ref", "--short", "HEAD")
	baseBranch = strings.TrimSpace(baseBranch)

	write("app.txt", "one\ntwo changed\n")
	write("second.txt", "second\n")
	c2 := commit([]string{"app.txt", "second.txt"}, "feat: second")

	write("third.txt", "third\n")
	c3 := commit([]string{"third.txt"}, "feat: third")

	blame, err := svc.GetFileBlame(repo, "app.txt")
	if err != nil || len(blame) != 2 {
		t.Fatalf("GetFileBlame failed: %+v err=%v", blame, err)
	}
	if blame[0].Content != "one" || blame[1].Content != "two changed" || blame[1].Author != "Tester" || blame[1].ShortHash == "" {
		t.Fatalf("unexpected blame data: %+v", blame)
	}
	if _, err := svc.GetFileBlame(repo, ""); err == nil {
		t.Fatal("expected error for empty blame path")
	}

	oldContent, err := svc.GetFileContentAtCommit(repo, c1, "app.txt")
	if err != nil || oldContent != "one\ntwo" {
		t.Fatalf("GetFileContentAtCommit failed: %q err=%v", oldContent, err)
	}
	if _, err := svc.GetFileContentAtCommit(repo, c1, "missing.txt"); err == nil {
		t.Fatal("expected error reading missing file at commit")
	}
	if _, err := svc.GetFileContentAtCommit(repo, c1, ""); err == nil {
		t.Fatal("expected error for empty file path")
	}

	diff, err := svc.GetDiffBetween(repo, c1, c3)
	if err != nil || diff.RefA != c1 || diff.RefB != c3 || diff.Additions == 0 || !strings.Contains(diff.Diff, "third.txt") {
		t.Fatalf("GetDiffBetween failed: %+v err=%v", diff, err)
	}
	var sawApp, sawSecond bool
	for _, f := range diff.Files {
		if f.Path == "app.txt" {
			sawApp = true
		}
		if f.Path == "second.txt" {
			sawSecond = true
		}
	}
	if !sawApp || !sawSecond {
		t.Fatalf("diff should include app.txt and second.txt: %+v", diff.Files)
	}
	if _, err := svc.GetDiffBetween(repo, "", c3); err == nil {
		t.Fatal("expected error comparing empty ref")
	}

	write("app.txt", "one\ntwo changed again\n")
	wordDiff, err := svc.GetWorkingWordDiff(repo, "app.txt", false)
	if err != nil || !strings.Contains(wordDiff, "{+") {
		t.Fatalf("GetWorkingWordDiff failed: %q err=%v", wordDiff, err)
	}
	if err := svc.DiscardFile(repo, "app.txt"); err != nil {
		t.Fatalf("DiscardFile failed: %v", err)
	}

	overview, err := svc.GetRepoOverview(repo)
	if err != nil || overview.TotalCommits != 3 || overview.TotalBranches < 1 || overview.TotalTags != 0 {
		t.Fatalf("GetRepoOverview failed: %+v err=%v", overview, err)
	}
	var testerCommits int
	for _, c := range overview.Contributors {
		if c.Name == "Tester" {
			testerCommits = c.Commits
		}
	}
	if testerCommits != 3 {
		t.Fatalf("expected Tester to have 3 commits, got %+v", overview.Contributors)
	}

	if err := svc.CreateAnnotatedTag(repo, "v1", "release one", false); err != nil {
		t.Fatalf("CreateAnnotatedTag failed: %v", err)
	}
	tags, _ := svc.GetTags(repo)
	var annotated bool
	for _, tag := range tags {
		if tag.Name == "v1" && tag.IsAnnotated && strings.Contains(tag.Message, "release one") {
			annotated = true
		}
	}
	if !annotated {
		t.Fatalf("expected annotated tag v1, got %+v", tags)
	}
	if err := svc.CreateAnnotatedTag(repo, "", "bad", false); err == nil {
		t.Fatal("expected error for empty tag name")
	}

	if err := svc.GenerateGitignoreTemplate(repo, "go"); err != nil {
		t.Fatalf("GenerateGitignoreTemplate failed: %v", err)
	}
	ignore, _ := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if !strings.Contains(string(ignore), "# Go") || !strings.Contains(string(ignore), "bin/") {
		t.Fatalf("unexpected .gitignore template: %q", ignore)
	}
	if err := svc.GenerateGitignoreTemplate(repo, "unknown"); err == nil {
		t.Fatal("expected error for unknown gitignore template")
	}

	storage, err := svc.GetRepoStorageBreakdown(repo)
	if err != nil || len(storage) == 0 {
		t.Fatalf("GetRepoStorageBreakdown failed: %+v err=%v", storage, err)
	}
	var txt *StorageBreakdownItem
	for i := range storage {
		if storage[i].Extension == ".txt" {
			txt = &storage[i]
		}
	}
	if txt == nil || txt.Count != 3 || txt.SizeBytes == 0 || txt.SizeHuman == "" {
		t.Fatalf("expected three .txt files in storage breakdown, got %+v", storage)
	}

	signature, err := svc.GetCommitSignatureDetails(repo, c2)
	if err != nil || signature.Status != "N" || signature.Date != "" {
		t.Fatalf("expected unsigned signature status N without signature date, got %+v err=%v", signature, err)
	}
	if sig, err := svc.GetCommitSignatureDetails(repo, ""); err != nil || sig.Status != "N" || sig.Date != "" {
		t.Fatalf("empty hash should default to HEAD without signature date, got %+v err=%v", sig, err)
	}

	reflog, err := svc.GetReflog(repo, 20)
	if err != nil || len(reflog) < 3 {
		t.Fatalf("GetReflog failed: %+v err=%v", reflog, err)
	}
	if reflog[0].Selector == "" || reflog[0].CommitHash == "" || reflog[0].Message == "" {
		t.Fatalf("unexpected reflog entry: %+v", reflog[0])
	}
	if err := svc.CreateBranchFromReflog(repo, "recovered", c2); err != nil {
		t.Fatalf("CreateBranchFromReflog failed: %v", err)
	}
	branches, _ := svc.GetBranches(repo)
	var recovered bool
	for _, b := range branches {
		if b.Name == "recovered" && strings.HasPrefix(b.CommitHash, c2[:7]) {
			recovered = true
		}
	}
	if !recovered {
		t.Fatalf("expected recovered branch at %s, got %+v", c2, branches)
	}
	if err := svc.CreateBranchFromReflog(repo, "", c2); err == nil {
		t.Fatal("expected error for empty reflog branch name")
	}
	if err := svc.CheckoutReflog(repo, ""); err == nil {
		t.Fatal("expected error for empty reflog selector")
	}

	rebaseCommits, err := svc.GetRebaseCommits(repo, c1)
	if err != nil || len(rebaseCommits) != 2 {
		t.Fatalf("GetRebaseCommits failed: %+v err=%v", rebaseCommits, err)
	}
	todos, err := svc.InteractiveRebasePlan(repo, c1)
	if err != nil || len(todos) != 2 {
		t.Fatalf("InteractiveRebasePlan failed: %+v err=%v", todos, err)
	}
	if todos[0].Action != "pick" || len(todos[0].Hash) != 40 || todos[0].Subject == "" {
		t.Fatalf("unexpected rebase todo: %+v", todos[0])
	}

	if err := svc.CheckoutCommitDetached(repo, c1); err != nil {
		t.Fatalf("CheckoutCommitDetached failed: %v", err)
	}
	if current, _ := ExecGit(repo, "rev-parse", "--abbrev-ref", "HEAD"); strings.TrimSpace(current) != "HEAD" {
		t.Fatalf("expected detached HEAD, got %q", current)
	}
	if err := svc.CheckoutBranch(repo, baseBranch, false); err != nil {
		t.Fatalf("checkout base failed: %v", err)
	}

	write("app.txt", "one\ntwo changed again\n")
	write("other.txt", "other\n")
	if err := svc.SaveStashWithFiles(repo, "only app", []string{"app.txt"}); err != nil {
		t.Fatalf("SaveStashWithFiles failed: %v", err)
	}
	stashes, _ := svc.GetStashes(repo)
	if len(stashes) != 1 || !strings.Contains(stashes[0].Message, "only app") {
		t.Fatalf("expected selective stash, got %+v", stashes)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "app.txt")); string(data) != "one\ntwo changed\n" {
		t.Fatalf("selective stash should restore app.txt, got %q", data)
	}
	if _, err := os.Stat(filepath.Join(repo, "other.txt")); err != nil {
		t.Fatalf("selective stash should keep other.txt: %v", err)
	}
	if err := svc.SaveStashWithFiles(repo, "bad", nil); err == nil {
		t.Fatal("expected error for empty stash file list")
	}
	if err := svc.StashPop(repo, 0); err != nil {
		t.Fatalf("StashPop failed: %v", err)
	}

	if err := svc.CheckoutCommitDetached(repo, ""); err == nil {
		t.Fatal("expected error for empty detached checkout hash")
	}
}

func TestSquashCommitsInTempRepo(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	for i, message := range []string{"feat: one", "feat: two", "feat: three"} {
		path := filepath.Join(repo, "file.txt")
		if err := os.WriteFile(path, []byte(message+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := svc.StageFile(repo, "file.txt"); err != nil {
			t.Fatalf("StageFile failed: %v", err)
		}
		if err := svc.Commit(repo, message, false); err != nil {
			t.Fatalf("Commit #%d failed: %v", i+1, err)
		}
	}

	if _, err := svc.SquashCommits(repo, 1, "bad"); err == nil {
		t.Fatal("expected error squashing fewer than two commits")
	}
	if _, err := svc.SquashCommits(repo, 2, "  "); err == nil {
		t.Fatal("expected error with empty squash message")
	}
	if _, err := svc.SquashCommits(repo, 2, "squash: last two"); err != nil {
		t.Fatalf("SquashCommits failed: %v", err)
	}
	log, err := ExecGit(repo, "log", "--format=%s")
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(log), "\n")
	if len(lines) != 2 || lines[0] != "squash: last two" || lines[1] != "feat: one" {
		t.Fatalf("unexpected history after squash: %q", lines)
	}
	if status, _ := svc.GetRepoStatus(repo); !status.IsClean {
		t.Fatalf("workspace should be clean after squash, got %+v", status)
	}
}
