package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanFixCenterDetectsStagedSecret(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "README.md", "hi\n", "chore: init", time.Now(), "Alice")

	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "-f", ".env")

	svc := GitService{}
	res, err := svc.ScanFixCenter(dir)
	if err != nil {
		t.Fatalf("ScanFixCenter failed: %v", err)
	}

	var secret *FixCenterItem
	for i := range res.Items {
		if res.Items[i].ID == "secret-unstage" {
			secret = &res.Items[i]
		}
	}
	if secret == nil {
		t.Fatalf("expected a secret-unstage item, got %+v", res.Items)
	}
	if secret.Severity != "fail" {
		t.Fatalf("an AWS key should escalate to fail, got %q", secret.Severity)
	}
	if !secret.Fixable || !secret.NeedsReview {
		t.Fatalf("secret remediation must be fixable and reviewable: %+v", secret)
	}
	found := false
	for _, target := range secret.Targets {
		if target == ".env" {
			found = true
		}
	}
	if !found {
		t.Fatalf(".env missing from targets: %+v", secret.Targets)
	}
	if res.TotalFixes == 0 {
		t.Fatal("expected at least one fixable item")
	}

	// Every item must carry catalog keys so the desktop UI can localize the
	// card; the rendered text stays as the fallback.
	for _, item := range res.Items {
		if item.TitleKey == "" || item.SummaryKey == "" || item.FixLabelKey == "" {
			t.Fatalf("item %q is missing i18n keys: %+v", item.ID, item)
		}
		if item.SummaryKey == "fix.item.secret.summary" {
			for _, name := range []string{"hits", "files", "critical"} {
				if item.Params[name] == "" {
					t.Fatalf("secret summary is missing param %q: %+v", name, item.Params)
				}
			}
		}
	}
}

func TestRemediateStagedSecretsUnstagesAndIgnores(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "README.md", "hi\n", "chore: init", time.Now(), "Alice")

	if err := os.WriteFile(filepath.Join(dir, "secret.pem"), []byte("-----BEGIN PRIVATE KEY-----\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "-f", "secret.pem")

	svc := GitService{}
	res, err := svc.RemediateStagedSecrets(dir, []string{"secret.pem"}, true, false)
	if err != nil {
		t.Fatalf("RemediateStagedSecrets failed: %v", err)
	}
	if res.Unstaged != 1 || res.Failed != 0 {
		t.Fatalf("expected one unstaged file, got %+v", res)
	}

	staged := strings.TrimSpace(runGitTest(t, dir, nil, "diff", "--cached", "--name-only"))
	if strings.Contains(staged, "secret.pem") {
		t.Fatalf("secret.pem is still staged: %q", staged)
	}
	// The working-tree file must survive: unstaging is not deletion.
	if _, err := os.Stat(filepath.Join(dir, "secret.pem")); err != nil {
		t.Fatalf("working tree file was removed: %v", err)
	}
	ignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(ignore), "secret.pem") {
		t.Fatalf(".gitignore missing the pattern: %s", ignore)
	}
}

func TestRemediateStagedSecretsIgnoresDuplicatePatterns(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "README.md", "hi\n", "chore: init", time.Now(), "Alice")
	if err := os.WriteFile(filepath.Join(dir, "a.key"), []byte("k"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "-f", "a.key")

	svc := GitService{}
	if _, err := svc.RemediateStagedSecrets(dir, []string{"a.key"}, true, false); err != nil {
		t.Fatalf("first remediation failed: %v", err)
	}
	first, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))

	// Stage again and remediate a second time: the pattern must not duplicate.
	runGitTest(t, dir, nil, "add", "-f", "a.key")
	if _, err := svc.RemediateStagedSecrets(dir, []string{"a.key"}, true, false); err != nil {
		t.Fatalf("second remediation failed: %v", err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Count(string(second), "a.key") != 1 {
		t.Fatalf("pattern duplicated: %s", second)
	}
	if string(first) != string(second) {
		t.Fatalf(".gitignore changed unexpectedly:\nfirst=%s\nsecond=%s", first, second)
	}
}

func TestApplyAuditFixIgnoreRules(t *testing.T) {
	dir := initClosedLoopRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "Assets"), 0755); err != nil {
		t.Fatal(err)
	}
	writeAndCommit(t, dir, "Assets/Player.cs", "// player\n", "feat: player", time.Now(), "Alice")

	svc := GitService{}
	res, err := svc.ApplyAuditFix(dir, "ignore-rules", nil)
	if err != nil {
		t.Fatalf("ApplyAuditFix failed: %v", err)
	}
	if !res.Success || res.Applied == 0 {
		t.Fatalf("expected rules to be applied, got %+v", res)
	}

	ignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(ignore), "[Ll]ibrary/") {
		t.Fatalf("Unity Library rule missing: %s", ignore)
	}
}

func TestApplyAuditFixRejectsUnknownFix(t *testing.T) {
	dir := initClosedLoopRepo(t)
	svc := GitService{}
	if _, err := svc.ApplyAuditFix(dir, "does-not-exist", nil); err == nil {
		t.Fatal("expected an error for an unknown fix id")
	}
}

func TestApplyAuditFixRoutesStagedLargeLfs(t *testing.T) {
	dir := initClosedLoopRepo(t)
	requireGitLFS(t, dir)

	if err := os.MkdirAll(filepath.Join(dir, "Assets"), 0755); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 1<<20)
	if err := os.WriteFile(filepath.Join(dir, "Assets", "big.fbx"), payload, 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "Assets/big.fbx")

	svc := GitService{}
	res, err := svc.ApplyAuditFix(dir, "staged-large-lfs", []string{"Assets/big.fbx"})
	if err != nil {
		t.Fatalf("ApplyAuditFix failed: %v", err)
	}
	if !res.Success || res.Applied != 1 {
		t.Fatalf("expected one conversion, got %+v", res)
	}
}

func TestBackupRepoFilesSnapshotsContent(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "note.txt", "original\n", "chore: note", time.Now(), "Alice")

	backupDir, backed := backupRepoFiles(dir, []string{"note.txt", "missing.txt"})
	if backupDir == "" || len(backed) != 1 || backed[0] != "note.txt" {
		t.Fatalf("unexpected backup result: dir=%q backed=%v", backupDir, backed)
	}
	data, err := os.ReadFile(filepath.Join(backupDir, "note.txt"))
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(data) != "original\n" {
		t.Fatalf("backup content mismatch: %q", data)
	}

	// Two fixes in quick succession must not share a snapshot directory.
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	secondDir, _ := backupRepoFiles(dir, []string{"note.txt"})
	if secondDir == "" || secondDir == backupDir {
		t.Fatalf("consecutive backups must use distinct directories: %q vs %q", backupDir, secondDir)
	}
	first, err := os.ReadFile(filepath.Join(backupDir, "note.txt"))
	if err != nil || string(first) != "original\n" {
		t.Fatalf("first backup must stay intact: %q err=%v", first, err)
	}
	second, err := os.ReadFile(filepath.Join(secondDir, "note.txt"))
	if err != nil || string(second) != "changed\n" {
		t.Fatalf("second backup content mismatch: %q err=%v", second, err)
	}
}

func TestScanFixCenterNormalizesEmptyTargets(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "README.md", "hi\n", "chore: init", time.Now(), "Alice")

	res, err := (GitService{}).ScanFixCenter(dir)
	if err != nil {
		t.Fatalf("ScanFixCenter failed: %v", err)
	}
	found := false
	for _, item := range res.Items {
		if item.ID != "ignore-rules" {
			continue
		}
		found = true
		if item.Targets == nil {
			t.Fatal("targets must serialize as [] rather than null")
		}
	}
	if !found {
		t.Fatalf("expected an ignore-rules item, got %+v", res.Items)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal scan result: %v", err)
	}
	if strings.Contains(string(raw), `"targets":null`) {
		t.Fatalf("targets must not serialize as null: %s", raw)
	}
}

func TestScanFixCenterReportsIncompleteChecks(t *testing.T) {
	binDir := t.TempDir()
	fakeLfs := filepath.Join(binDir, "git-lfs")
	if err := os.WriteFile(fakeLfs, []byte("#!/bin/sh\necho 'fatal: could not read from remote repository' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "README.md", "hi\n", "chore: init", time.Now(), "Alice")

	res, err := (GitService{}).ScanFixCenter(dir)
	if err != nil {
		t.Fatalf("ScanFixCenter failed: %v", err)
	}
	if res.AnalysisComplete {
		t.Fatal("a failing LFS lock query must mark the scan incomplete")
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected the failed check to be reported")
	}
	if strings.Contains(res.Message, "仓库状态良好") {
		t.Fatalf("must not claim the repo is clean after a failed check: %q", res.Message)
	}
}

func TestApplyAuditFixSecretUnstageBacksUpGitignore(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "README.md", "hi\n", "chore: init", time.Now(), "Alice")

	ignorePath := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(ignorePath, []byte("*.log\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "-f", ".env")

	res, err := (GitService{}).ApplyAuditFix(dir, "secret-unstage", []string{".env"})
	if err != nil {
		t.Fatalf("ApplyAuditFix failed: %v", err)
	}
	if !res.Success || res.BackupDir == "" {
		t.Fatalf("expected a successful fix with a backup, got %+v", res)
	}
	backup, err := os.ReadFile(filepath.Join(res.BackupDir, ".gitignore"))
	if err != nil || string(backup) != "*.log\n" {
		t.Fatalf("backup must preserve the original .gitignore: %q err=%v", backup, err)
	}
	updated, err := os.ReadFile(ignorePath)
	if err != nil || !strings.Contains(string(updated), ".env") {
		t.Fatalf("expected the secret path to be ignored: %q err=%v", updated, err)
	}
}

func TestApplyAuditFixAbortsWhenBackupFails(t *testing.T) {
	dir := initClosedLoopRepo(t)
	ignorePath := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(ignorePath, []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ignorePath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ignorePath, 0o644) })

	svc := GitService{}
	res, err := svc.ApplyAuditFix(dir, "ignore-rules", nil)
	if err != nil {
		t.Fatalf("ApplyAuditFix should report the backup failure in its result: %v", err)
	}
	if res.Failed == 0 || !strings.Contains(res.Message, "无法备份") {
		t.Fatalf("failed backup must abort the fix: %+v", res)
	}
	if err := os.Chmod(ignorePath, 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(ignorePath); err != nil || string(data) != "*.log\n" {
		t.Fatalf("original .gitignore must be preserved: %q err=%v", data, err)
	}
}
