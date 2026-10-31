package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// initPolicyAuditRepo creates a repository with fully isolated git config so
// global hooks/signing settings cannot perturb the policy audit.
func initPolicyAuditRepo(t *testing.T, name string) (string, func(dir string, args ...string) string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	repo := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, args ...string) string {
		t.Helper()
		out, err := ExecGit(dir, args...)
		if err != nil {
			t.Fatalf("git %v in %s failed: %v", args, dir, err)
		}
		return out
	}
	run(repo, "init", "-q", "-b", "main")
	run(repo, "config", "user.email", "tester@example.com")
	run(repo, "config", "user.name", "Tester")
	return repo, run
}

func writePolicyAuditFile(t *testing.T, repo, relPath, content string) {
	t.Helper()
	full := filepath.Join(repo, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func filterPolicyViolations(items []PolicyViolationItem, ruleID string) []PolicyViolationItem {
	out := []PolicyViolationItem{}
	for _, item := range items {
		if item.RuleID == ruleID {
			out = append(out, item)
		}
	}
	return out
}

func findPolicyRule(items []PolicyRuleCheckResult, ruleID string) (PolicyRuleCheckResult, bool) {
	for _, item := range items {
		if item.RuleID == ruleID {
			return item, true
		}
	}
	return PolicyRuleCheckResult{}, false
}

// TestAuditPrePushChecklistParallelDeterministic audits 12 commits through the
// parallel Rule 2/Rule 3 path. A base commit is pushed to a local bare remote so
// AuditPrePushChecklist("HEAD") audits the remote-tracking range rather than
// falling back to the ten-commit cap, keeping the worker pool fully exercised.
func TestAuditPrePushChecklistParallelDeterministic(t *testing.T) {
	repo, run := initPolicyAuditRepo(t, "work")

	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remoteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(remoteDir, "init", "--bare", "-q")

	writePolicyAuditFile(t, repo, "base.txt", "base\n")
	run(repo, "add", "base.txt")
	run(repo, "commit", "-qm", "chore: base [ISSUE-100000]")
	run(repo, "remote", "add", "origin", remoteDir)
	run(repo, "push", "-q", "-u", "origin", "main")

	pointerOids := map[int]string{
		3: strings.Repeat("a1", 32), // 64 hex chars, never stored in .git/lfs/objects
		8: strings.Repeat("b2", 32),
	}
	pointerFiles := map[int]string{
		3: "assets/arena_map_a.bin",
		8: "assets/arena_map_b.bin",
	}
	const commitCount = 12
	for i := 1; i <= commitCount; i++ {
		if file, ok := pointerFiles[i]; ok {
			pointer := "version https://git-lfs.github.com/spec/v1\noid sha256:" + pointerOids[i] + "\nsize 1024\n"
			writePolicyAuditFile(t, repo, file, pointer)
			run(repo, "add", file)
		}
		note := fmt.Sprintf("docs/notes_%02d.txt", i)
		writePolicyAuditFile(t, repo, note, fmt.Sprintf("note %d\n", i))
		run(repo, "add", note)
		run(repo, "commit", "-qm", fmt.Sprintf("feat: add studio note %02d [ISSUE-%06d]", i, 100000+i))
	}

	// Map each audited subject to its full hash and preserve log order.
	hashBySubject := map[string]string{}
	orderedSubjects := []string{}
	for _, line := range strings.Split(run(repo, "log", "--pretty=format:%H%x09%s"), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		hashBySubject[parts[1]] = parts[0]
		orderedSubjects = append(orderedSubjects, parts[1])
	}
	if len(orderedSubjects) != commitCount+1 {
		t.Fatalf("expected %d commits in repo, got %d", commitCount+1, len(orderedSubjects))
	}

	svc := GitService{}
	first, err := svc.AuditPrePushChecklist(repo, "HEAD")
	if err != nil {
		t.Fatalf("AuditPrePushChecklist failed: %v", err)
	}
	second, err := svc.AuditPrePushChecklist(repo, "HEAD")
	if err != nil {
		t.Fatalf("second AuditPrePushChecklist failed: %v", err)
	}

	if !first.AnalysisKnown {
		t.Fatal("analysisKnown should be true")
	}
	if first.CommitsAudited != commitCount {
		t.Fatalf("expected %d audited commits, got %d (summary=%s)", commitCount, first.CommitsAudited, first.Summary)
	}

	lfsViolations := filterPolicyViolations(first.Violations, "RULE_LFS_POINTER_INTEGRITY")
	if len(lfsViolations) != 2 {
		t.Fatalf("expected 2 LFS pointer violations, got %d: %+v", len(lfsViolations), lfsViolations)
	}
	// Commits are audited newest-first, so commit 8 precedes commit 3.
	expected := []struct {
		commitIndex int
		file        string
	}{
		{8, pointerFiles[8]},
		{3, pointerFiles[3]},
	}
	for i, want := range expected {
		got := lfsViolations[i]
		subject := fmt.Sprintf("feat: add studio note %02d [ISSUE-%06d]", want.commitIndex, 100000+want.commitIndex)
		if got.CommitHash != hashBySubject[subject] {
			t.Fatalf("violation %d commit = %s, want %s (%s)", i, got.CommitHash, hashBySubject[subject], subject)
		}
		if got.FilePath != want.file {
			t.Fatalf("violation %d file = %s, want %s", i, got.FilePath, want.file)
		}
		if got.Severity != "BLOCKING" || got.Title != "LFS 指针对象本地缺失" {
			t.Fatalf("violation %d unexpected severity/title: %+v", i, got)
		}
	}
	if first.Violations[0].RuleID != "RULE_LFS_POINTER_INTEGRITY" {
		t.Fatalf("expected LFS violation first after a clean Rule 1, got %s", first.Violations[0].RuleID)
	}

	rule, ok := findPolicyRule(first.RulesChecked, "RULE_LFS_POINTER_INTEGRITY")
	if !ok {
		t.Fatal("RULE_LFS_POINTER_INTEGRITY check missing")
	}
	if rule.Passed || rule.Severity != "BLOCKING" || rule.Details != "LFS 指针扫描完成，缺失本地缓存对象: 2 个" {
		t.Fatalf("unexpected LFS rule result: %+v", rule)
	}
	if _, ok := findPolicyRule(first.RulesChecked, "RULE_NO_LARGE_BLOBS"); !ok {
		t.Fatal("RULE_NO_LARGE_BLOBS check missing")
	}
	if !first.IsBlocked || first.ReadinessGrade != "BLOCKED" {
		t.Fatalf("expected blocked audit, got blocked=%v grade=%s", first.IsBlocked, first.ReadinessGrade)
	}

	if !reflect.DeepEqual(first.Violations, second.Violations) {
		t.Fatalf("violations are not deterministic:\nfirst:  %+v\nsecond: %+v", first.Violations, second.Violations)
	}
	if !reflect.DeepEqual(first.RulesChecked, second.RulesChecked) {
		t.Fatalf("rule checks are not deterministic:\nfirst:  %+v\nsecond: %+v", first.RulesChecked, second.RulesChecked)
	}
	if first.Summary != second.Summary || first.Message != second.Message {
		t.Fatalf("summary/message are not deterministic: %q vs %q, %q vs %q", first.Summary, second.Summary, first.Message, second.Message)
	}
}

// TestAuditPrePushChecklistCleanRepo verifies a repo with only valid commits
// reports no Rule 2 findings while all five rule checks remain present.
func TestAuditPrePushChecklistCleanRepo(t *testing.T) {
	repo, run := initPolicyAuditRepo(t, "clean")
	for i := 1; i <= 3; i++ {
		note := fmt.Sprintf("notes_%d.txt", i)
		writePolicyAuditFile(t, repo, note, fmt.Sprintf("clean note %d\n", i))
		run(repo, "add", note)
		run(repo, "commit", "-qm", fmt.Sprintf("fix: correct note %d [ISSUE-%06d]", i, 200000+i))
	}

	svc := GitService{}
	res, err := svc.AuditPrePushChecklist(repo, "HEAD")
	if err != nil {
		t.Fatalf("AuditPrePushChecklist failed: %v", err)
	}
	if !res.AnalysisKnown || res.CommitsAudited != 3 {
		t.Fatalf("expected 3 audited commits, got %d (analysisKnown=%v)", res.CommitsAudited, res.AnalysisKnown)
	}
	if got := filterPolicyViolations(res.Violations, "RULE_LFS_POINTER_INTEGRITY"); len(got) != 0 {
		t.Fatalf("expected no LFS pointer violations, got %+v", got)
	}
	if got := filterPolicyViolations(res.Violations, "RULE_NO_LARGE_BLOBS"); len(got) != 0 {
		t.Fatalf("expected no large blob violations, got %+v", got)
	}

	wantRules := []string{
		"RULE_COMMIT_CONVENTION",
		"RULE_LFS_POINTER_INTEGRITY",
		"RULE_NO_LARGE_BLOBS",
		"RULE_COMMIT_SIGNATURE",
		"RULE_BRANCH_STATUS",
	}
	if len(res.RulesChecked) != len(wantRules) {
		t.Fatalf("expected %d rule checks, got %d: %+v", len(wantRules), len(res.RulesChecked), res.RulesChecked)
	}
	for i, want := range wantRules {
		if res.RulesChecked[i].RuleID != want {
			t.Fatalf("rule check %d = %s, want %s", i, res.RulesChecked[i].RuleID, want)
		}
	}
	lfsRule, _ := findPolicyRule(res.RulesChecked, "RULE_LFS_POINTER_INTEGRITY")
	if !lfsRule.Passed || lfsRule.Details != "LFS 指针扫描完成，缺失本地缓存对象: 0 个" {
		t.Fatalf("unexpected clean LFS rule result: %+v", lfsRule)
	}
}
