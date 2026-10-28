package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newRulesRepo(t *testing.T) string {
	t.Helper()
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "chore: init", time.Now(), "Alice")
	return dir
}

func TestCommitSafetyRulesDefaultWhenFileMissing(t *testing.T) {
	dir := newRulesRepo(t)
	svc := GitService{}

	rules, err := svc.LoadCommitSafetyRules(dir)
	if err != nil {
		t.Fatalf("LoadCommitSafetyRules failed: %v", err)
	}
	if rules.Source != "default" {
		t.Fatalf("expected default source, got %q", rules.Source)
	}
	if len(rules.Terms) != len(defaultCommitSafetyTerms()) {
		t.Fatalf("expected %d default terms, got %d", len(defaultCommitSafetyTerms()), len(rules.Terms))
	}
	if rules.Message == "" {
		t.Fatal("expected an explanatory message")
	}
}

func TestSaveCommitSafetyRulesRoundTrip(t *testing.T) {
	dir := newRulesRepo(t)
	svc := GitService{}

	saved, err := svc.SaveCommitSafetyRules(dir, []string{"内部代号", "SECRET_TOKEN"})
	if err != nil {
		t.Fatalf("SaveCommitSafetyRules failed: %v", err)
	}
	if saved.Source != "custom" || len(saved.Terms) != 2 {
		t.Fatalf("unexpected save result: %+v", saved)
	}

	// The file must live under the git dir, not the work tree.
	expected := filepath.Join(dir, ".git", commitSafetyRulesFile)
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("rules file not written where expected: %v", err)
	}

	loaded, err := svc.LoadCommitSafetyRules(dir)
	if err != nil {
		t.Fatalf("LoadCommitSafetyRules failed: %v", err)
	}
	if loaded.Source != "custom" {
		t.Fatalf("expected custom source, got %q", loaded.Source)
	}
	if len(loaded.Terms) != 2 || loaded.Terms[0] != "内部代号" {
		t.Fatalf("terms not preserved in order: %+v", loaded.Terms)
	}
}

func TestSaveCommitSafetyRulesNormalizesInput(t *testing.T) {
	dir := newRulesRepo(t)
	svc := GitService{}

	saved, err := svc.SaveCommitSafetyRules(dir, []string{"  spaced  ", "", "spaced", "other"})
	if err != nil {
		t.Fatalf("SaveCommitSafetyRules failed: %v", err)
	}
	// Trimmed, blank dropped, duplicate collapsed, original order kept.
	if len(saved.Terms) != 2 {
		t.Fatalf("expected 2 normalized terms, got %+v", saved.Terms)
	}
	if saved.Terms[0] != "spaced" || saved.Terms[1] != "other" {
		t.Fatalf("unexpected order: %+v", saved.Terms)
	}
}

func TestSaveCommitSafetyRulesRejectsEmpty(t *testing.T) {
	dir := newRulesRepo(t)
	svc := GitService{}
	if _, err := svc.SaveCommitSafetyRules(dir, []string{"   ", ""}); err == nil {
		t.Fatal("expected an error for an empty rule set")
	}
}

func TestSaveCommitSafetyRulesRejectsOversizedTerm(t *testing.T) {
	dir := newRulesRepo(t)
	svc := GitService{}
	long := make([]byte, maxCommitSafetyTermBytes+1)
	for i := range long {
		long[i] = 'x'
	}
	if _, err := svc.SaveCommitSafetyRules(dir, []string{string(long)}); err == nil {
		t.Fatal("expected an error for an oversized term")
	}
}

func TestLoadCommitSafetyRulesFallsBackOnCorruptFile(t *testing.T) {
	dir := newRulesRepo(t)
	svc := GitService{}

	path := filepath.Join(dir, ".git", commitSafetyRulesFile)
	if err := os.WriteFile(path, []byte("{ this is not json"), 0644); err != nil {
		t.Fatal(err)
	}

	rules, err := svc.LoadCommitSafetyRules(dir)
	if err != nil {
		t.Fatalf("a corrupt file must not fail the audit: %v", err)
	}
	if rules.Source != "default" {
		t.Fatalf("expected fallback to default, got %q", rules.Source)
	}
	if len(rules.Terms) == 0 {
		t.Fatal("fallback must still provide terms")
	}
	if rules.Message == "" {
		t.Fatal("fallback must explain itself")
	}
}

func TestLoadCommitSafetyRulesFallsBackOnEmptyFileContent(t *testing.T) {
	dir := newRulesRepo(t)
	svc := GitService{}

	path := filepath.Join(dir, ".git", commitSafetyRulesFile)
	if err := os.WriteFile(path, []byte(`{"terms": []}`), 0644); err != nil {
		t.Fatal(err)
	}

	rules, err := svc.LoadCommitSafetyRules(dir)
	if err != nil {
		t.Fatalf("empty terms must not fail: %v", err)
	}
	if rules.Source != "default" {
		t.Fatalf("expected default fallback, got %q", rules.Source)
	}
}

func TestResetCommitSafetyRulesRestoresDefaults(t *testing.T) {
	dir := newRulesRepo(t)
	svc := GitService{}

	if _, err := svc.SaveCommitSafetyRules(dir, []string{"临时规则"}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	reset, err := svc.ResetCommitSafetyRules(dir)
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}
	if reset.Source != "default" {
		t.Fatalf("expected default after reset, got %q", reset.Source)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", commitSafetyRulesFile)); !os.IsNotExist(err) {
		t.Fatal("rules file should be gone after reset")
	}
}

func TestAuditCommitContentSafetyUsesCustomRules(t *testing.T) {
	dir := newRulesRepo(t)
	svc := GitService{}

	// A word the default list does not know about.
	writeAndCommit(t, dir, "b.txt", "b\n", "feat: 内部代号蓝鲸", time.Now(), "Alice")

	before, err := svc.AuditCommitContentSafety(dir, 10)
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if before.RulesSource != "default" {
		t.Fatalf("expected default rules, got %q", before.RulesSource)
	}
	if before.ViolationsCount != 0 {
		t.Fatalf("default rules should not flag the custom word: %+v", before.Violations)
	}

	if _, err := svc.SaveCommitSafetyRules(dir, []string{"蓝鲸"}); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	after, err := svc.AuditCommitContentSafety(dir, 10)
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if after.RulesSource != "custom" {
		t.Fatalf("expected custom rules, got %q", after.RulesSource)
	}
	if after.RuleCount != 1 {
		t.Fatalf("expected 1 rule, got %d", after.RuleCount)
	}
	if after.ViolationsCount != 1 {
		t.Fatalf("custom rule should flag the commit, got %+v", after.Violations)
	}
}
