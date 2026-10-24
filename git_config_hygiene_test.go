package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigProxyIgnoreAndRepoHygieneInTempRepo(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("tracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "tracked.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	head, _ := ExecGit(repo, "rev-parse", "HEAD")
	head = strings.TrimSpace(head)

	entries, err := svc.GetDetailedGitConfig(repo)
	if err != nil {
		t.Fatalf("GetDetailedGitConfig failed: %v", err)
	}
	localName := false
	for _, entry := range entries {
		if entry.Key == "user.name" && entry.Value == "Tester" && entry.Scope == "local" {
			localName = true
		}
	}
	if !localName {
		t.Fatalf("expected local user.name config, got %+v", entries)
	}
	if err := svc.SetCustomConfigKey(repo, "wgit.testKey", "local-value", false); err != nil {
		t.Fatalf("SetCustomConfigKey local failed: %v", err)
	}
	if err := svc.SetCustomConfigKey(repo, "", "x", false); err == nil {
		t.Fatal("expected error for empty config key")
	}
	globalHome := t.TempDir()
	globalConfig := filepath.Join(globalHome, "isolated-gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	if err := svc.SetCustomConfigKey(repo, "wgit.globalKey", "global-value", true); err != nil {
		t.Fatalf("SetCustomConfigKey global failed: %v", err)
	}
	entries, err = svc.GetDetailedGitConfig(repo)
	if err != nil {
		t.Fatalf("GetDetailedGitConfig after global failed: %v", err)
	}
	var localValue, globalValue bool
	for _, entry := range entries {
		if entry.Key == "wgit.testkey" && entry.Value == "local-value" && entry.Scope == "local" {
			localValue = true
		}
		if entry.Key == "wgit.globalkey" && entry.Value == "global-value" && entry.Scope == "global" {
			globalValue = true
		}
	}
	if !localValue || !globalValue {
		t.Fatalf("expected local and global config scopes, got %+v", entries)
	}

	if err := svc.SetGitProxy(repo, "http://127.0.0.1:7890", false); err != nil {
		t.Fatalf("SetGitProxy failed: %v", err)
	}
	proxy, err := svc.GetGitProxy(repo)
	if err != nil || proxy.HttpProxy != "http://127.0.0.1:7890" || proxy.HttpsProxy != "http://127.0.0.1:7890" || proxy.IsGlobal {
		t.Fatalf("GetGitProxy failed: %+v err=%v", proxy, err)
	}
	if err := svc.SetGitProxy(repo, "", false); err == nil {
		t.Fatal("expected error for empty proxy URL")
	}
	if err := svc.ClearGitProxy(repo, false); err != nil {
		t.Fatalf("ClearGitProxy failed: %v", err)
	}
	if proxy, _ := svc.GetGitProxy(repo); proxy.HttpProxy != "" || proxy.HttpsProxy != "" {
		t.Fatalf("expected proxy to be cleared, got %+v", proxy)
	}
	if err := svc.SetGitProxy(repo, "http://127.0.0.1:1080", true); err != nil {
		t.Fatalf("SetGitProxy global failed: %v", err)
	}
	if proxy, _ := svc.GetGitProxy(repo); proxy.HttpProxy != "http://127.0.0.1:1080" || !proxy.IsGlobal {
		t.Fatalf("expected global proxy, got %+v", proxy)
	}
	if err := svc.ClearGitProxy(repo, true); err != nil {
		t.Fatalf("ClearGitProxy global failed: %v", err)
	}

	if attrs, err := svc.GetGitAttributes(repo); err != nil || attrs != "" {
		t.Fatalf("expected empty gitattributes, got %q err=%v", attrs, err)
	}
	if err := svc.SaveGitAttributes(repo, "*.txt text\n*.bin binary\n"); err != nil {
		t.Fatalf("SaveGitAttributes failed: %v", err)
	}
	if attrs, _ := svc.GetGitAttributes(repo); !strings.Contains(attrs, "*.txt text") {
		t.Fatalf("GetGitAttributes did not read saved content: %q", attrs)
	}
	if err := svc.SaveGitAttributes(repo, ""); err != nil {
		t.Fatalf("SaveGitAttributes empty failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignored, err := svc.CheckIgnoreRule(repo, "debug.log")
	if err != nil || ignored == nil || !ignored.IsIgnored || ignored.Pattern != "*.log" || ignored.LineNumber != 1 || ignored.SourceFile != ".gitignore" {
		t.Fatalf("CheckIgnoreRule ignored failed: %+v err=%v", ignored, err)
	}
	if ignored, _ := svc.CheckIgnoreRule(repo, "tracked.txt"); ignored.IsIgnored {
		t.Fatalf("tracked.txt should not be ignored: %+v", ignored)
	}
	if _, err := svc.CheckIgnoreRule(repo, ""); err == nil {
		t.Fatal("expected error for empty ignore path")
	}
	if _, err := svc.CheckIgnoreRule(t.TempDir(), "debug.log"); err == nil {
		t.Fatal("a non-git directory must surface an error instead of reporting 'not ignored'")
	}

	outside := filepath.Join(filepath.Dir(repo), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("temp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.DiscardUntrackedFile(repo, "tracked.txt"); err == nil {
		t.Fatal("DiscardUntrackedFile must refuse tracked files")
	}
	if _, err := os.Stat(filepath.Join(repo, "tracked.txt")); err != nil {
		t.Fatalf("tracked file should be preserved: %v", err)
	}
	if err := svc.DiscardUntrackedFile(repo, "../outside.txt"); err == nil {
		t.Fatal("DiscardUntrackedFile must refuse paths outside the repository")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file should be preserved: %v", err)
	}
	if err := svc.DiscardUntrackedFile(repo, ""); err == nil {
		t.Fatal("expected error for empty untracked path")
	}
	if err := svc.DiscardUntrackedFile(repo, "untracked.txt"); err != nil {
		t.Fatalf("DiscardUntrackedFile failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "untracked.txt")); !os.IsNotExist(err) {
		t.Fatalf("untracked file should be removed: %v", err)
	}

	topology, err := svc.GetCommitTopologyStats(repo)
	if err != nil || topology.TotalCommits != 1 || topology.MergeCommits != 0 || topology.DirectCommits != 1 || topology.MergeRatio != 0 {
		t.Fatalf("GetCommitTopologyStats failed: %+v err=%v", topology, err)
	}
	batch, err := svc.GetCommitDetailsBatch(repo, []string{head, "", "missing"})
	if err != nil || len(batch) != 1 || batch[0].Subject != "chore: base" {
		t.Fatalf("GetCommitDetailsBatch failed: %+v err=%v", batch, err)
	}

	if err := os.MkdirAll(filepath.Join(repo, "empty", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	emptyDirs, err := svc.ScanEmptyDirectories(repo)
	if err != nil {
		t.Fatalf("ScanEmptyDirectories failed: %v", err)
	}
	var sawEmptySub bool
	for _, dir := range emptyDirs {
		if dir == "empty/sub" {
			sawEmptySub = true
		}
	}
	if !sawEmptySub {
		t.Fatalf("expected empty/sub in empty directories, got %+v", emptyDirs)
	}
	if count, err := svc.GenerateGitkeeps(repo, []string{"empty/sub", "missing-dir", ""}); err != nil || count != 1 {
		t.Fatalf("GenerateGitkeeps failed: %d err=%v", count, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "empty", "sub", ".gitkeep")); err != nil {
		t.Fatalf("expected .gitkeep to be created: %v", err)
	}
	if status, _ := svc.GetRepoStatus(repo); len(status.StagedFiles) != 1 || status.StagedFiles[0].Path != "empty/sub/.gitkeep" {
		t.Fatalf("expected .gitkeep staged, got %+v", status.StagedFiles)
	}
	emptyDirs, _ = svc.ScanEmptyDirectories(repo)
	for _, dir := range emptyDirs {
		if dir == "empty/sub" {
			t.Fatalf("empty/sub should no longer be empty: %+v", emptyDirs)
		}
	}

	health, err := svc.VerifyRepoHealth(repo)
	if err != nil || health == nil || !health.IsClean || len(health.RawErrors) != 0 {
		t.Fatalf("healthy repo reported as broken: %+v err=%v", health, err)
	}
	blobHash, err := ExecGit(repo, "rev-parse", "HEAD:tracked.txt")
	if err != nil {
		t.Fatalf("resolve blob failed: %v", err)
	}
	blobHash = strings.TrimSpace(blobHash)
	objectPath := filepath.Join(repo, ".git", "objects", blobHash[:2], blobHash[2:])
	if err := os.Remove(objectPath); err != nil {
		t.Fatalf("remove object to simulate corruption failed: %v", err)
	}
	health, err = svc.VerifyRepoHealth(repo)
	if err != nil {
		t.Fatalf("VerifyRepoHealth corrupted failed: %v", err)
	}
	if health.IsClean || len(health.RawErrors) == 0 {
		t.Fatalf("corrupted repo must not be reported healthy: %+v", health)
	}
}
