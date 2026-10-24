package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalizationAuditEmptyRepoIsHonest(t *testing.T) {
	res, err := GitService{}.AuditLocalizationParity(t.TempDir(), "")
	if err != nil {
		t.Fatalf("audit empty localization repo: %v", err)
	}
	if res.AnalysisKnown || res.Severity != "UNKNOWN" || res.OverallParityScore != 0 {
		t.Fatalf("empty localization audit must not claim coverage: %+v", res)
	}
	if res.LanguagesFound == nil || res.LanguageStats == nil || res.UntranslatedList == nil || res.AuditedFiles == nil {
		t.Fatalf("empty localization slices must serialize as arrays, not null: %+v", res)
	}
	if len(res.Advice) == 0 || !strings.Contains(strings.Join(res.Advice, " "), "未作覆盖率结论") {
		t.Fatalf("empty localization advice should be explicit: %+v", res.Advice)
	}
	if strings.Contains(strings.Join(res.Advice, " "), "全覆盖合规") || !strings.Contains(res.Message, "未发现") {
		t.Fatalf("empty localization audit must not claim full coverage: %+v", res)
	}
}

func setupDataTableAuditRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.name", "Tester"},
		{"config", "user.email", "tester@example.com"},
	} {
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "readme.txt"), []byte("no tables"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := ExecGit(repo, "add", "-A"); err != nil {
		t.Fatalf("add: %v (%s)", err, out)
	}
	if out, err := ExecGit(repo, "commit", "-qm", "base"); err != nil {
		t.Fatalf("commit: %v (%s)", err, out)
	}
	return repo
}

func TestDataTableAuditEmptyRepoIsHonest(t *testing.T) {
	repo := setupDataTableAuditRepo(t)
	res, err := GitService{}.AuditDataTableDiff(repo, "", "HEAD", "WORKTREE")
	if err != nil {
		t.Fatalf("audit empty data table repo: %v", err)
	}
	if res.AnalysisKnown || res.TotalTables != 0 || res.IsValid {
		t.Fatalf("empty data table audit must not be valid-by-default: %+v", res)
	}
	if res.AuditedTables == nil {
		t.Fatalf("empty data table list must serialize as an array, not null: %+v", res)
	}
	if !strings.Contains(res.OverallSummary, "未作结构与数值结论") {
		t.Fatalf("empty data table summary should be explicit: %q", res.OverallSummary)
	}
}

func TestDataTableAuditRejectsInvalidRefsAndMissingTable(t *testing.T) {
	repo := setupDataTableAuditRepo(t)
	if _, err := (GitService{}).AuditDataTableDiff(repo, "", "missing-base", "WORKTREE"); err == nil {
		t.Fatal("invalid base ref must return an error")
	}
	if _, err := (GitService{}).AuditDataTableDiff(repo, "", "HEAD", "missing-target"); err == nil {
		t.Fatal("invalid target ref must return an error")
	}
	if _, err := (GitService{}).AuditDataTableDiff(repo, "Design/Missing.csv", "HEAD", "WORKTREE"); err == nil {
		t.Fatal("explicitly requested missing table must return an error")
	}
}

func TestDataTableAuditRejectsNonRepoAndIndexFailure(t *testing.T) {
	if _, err := (GitService{}).AuditDataTableDiff(t.TempDir(), "", "WORKTREE", "WORKTREE"); err == nil {
		t.Fatal("non-git directory must return an error")
	}

	repo := setupDataTableAuditRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".git", "index"), []byte("not a git index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (GitService{}).AuditDataTableDiff(repo, "", "HEAD", "WORKTREE"); err == nil {
		t.Fatal("unreadable index must surface the diff failure")
	}
}
