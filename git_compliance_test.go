package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoComplianceAndCommitPolicy(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "dev@company.com")

	// 1. Inspect without rules file -> defaults used
	report, err := svc.InspectRepoCompliance(repo)
	if err != nil {
		t.Fatalf("InspectRepoCompliance failed: %v", err)
	}
	if report.HasRulesFile {
		t.Error("expected hasRulesFile to be false before saving")
	}

	// 2. Save custom compliance rules
	customRules := ComplianceRules{
		EnforceSignedCommits:         true,
		RequireCommitMessagePattern:  `^(feat|fix|docs|chore): .+`,
		DisallowDirectCommitBranches: []string{"main", "master"},
		MaxFileSizeMb:                100,
		RequireEmailDomain:           "@company.com",
	}
	if err := svc.SaveRepoComplianceRules(repo, customRules); err != nil {
		t.Fatalf("SaveRepoComplianceRules failed: %v", err)
	}

	rulesPath := filepath.Join(repo, ".wgit-rules.json")
	if _, err := os.Stat(rulesPath); err != nil {
		t.Fatalf(".wgit-rules.json should exist, stat err: %v", err)
	}

	// 3. Inspect with rules file -> rules loaded
	reportAfter, err := svc.InspectRepoCompliance(repo)
	if err != nil {
		t.Fatalf("InspectRepoCompliance after save failed: %v", err)
	}
	if !reportAfter.HasRulesFile {
		t.Error("expected hasRulesFile to be true after saving")
	}
	if reportAfter.Rules.MaxFileSizeMb != 100 {
		t.Errorf("expected MaxFileSizeMb=100, got %d", reportAfter.Rules.MaxFileSizeMb)
	}

	// 4. Validate Commit Policy - Direct branch violation
	resDirect, err := svc.ValidateCommitPolicy(repo, "feat: valid commit message", "dev@company.com", "main")
	if err != nil {
		t.Fatalf("ValidateCommitPolicy failed: %v", err)
	}
	if resDirect.Valid || len(resDirect.Violations) == 0 {
		t.Error("expected violation for direct commit to main")
	}

	// 5. Validate Commit Policy - Message format violation
	resBadMsg, err := svc.ValidateCommitPolicy(repo, "random bad message without prefix", "dev@company.com", "feature/awesome")
	if err != nil {
		t.Fatalf("ValidateCommitPolicy failed: %v", err)
	}
	if resBadMsg.Valid || len(resBadMsg.Violations) == 0 {
		t.Error("expected violation for invalid commit message format")
	}

	// 6. Validate Commit Policy - Email domain violation
	resBadEmail, err := svc.ValidateCommitPolicy(repo, "feat: valid message", "hacker@evil.com", "feature/awesome")
	if err != nil {
		t.Fatalf("ValidateCommitPolicy failed: %v", err)
	}
	if resBadEmail.Valid || len(resBadEmail.Violations) == 0 {
		t.Error("expected violation for non-matching email domain")
	}

	// 7. Validate Commit Policy - All valid
	resValid, err := svc.ValidateCommitPolicy(repo, "feat: perfectly valid message", "dev@company.com", "feature/awesome")
	if err != nil {
		t.Fatalf("ValidateCommitPolicy failed: %v", err)
	}
	if !resValid.Valid || len(resValid.Violations) > 0 {
		t.Errorf("expected valid policy check, got violations: %v", resValid.Violations)
	}
}
