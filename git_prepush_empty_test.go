package main

import (
	"os"
	"strings"
	"testing"
)

func TestAuditPrePushChecklistNoCommitsIsNotPass(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", t.TempDir()+"/gitconfig")
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("init: %v (%s)", err, out)
	}

	res, err := GitService{}.AuditPrePushChecklist(repo, "main")
	if err != nil {
		t.Fatalf("audit empty pre-push repo: %v", err)
	}
	if res.AnalysisKnown || res.IsBlocked || res.PassReadinessScore != 0 || res.ReadinessGrade != "NO_COMMITS" {
		t.Fatalf("repository without commits must not pass with 100/100: %+v", res)
	}
	if !strings.Contains(res.Summary, "没有待推送提交") || !strings.Contains(res.Summary, "评分不适用") {
		t.Fatalf("no-commit summary should be explicit: %q", res.Summary)
	}
	if res.RulesChecked == nil {
		t.Fatal("rulesChecked must serialize as an array, not null")
	}
}
