package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteWebInfoAndExternalToolConfigInTempRepo(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "base.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if err := svc.AddRemote(repo, "origin", "https://github.com/acme/demo.git"); err != nil {
		t.Fatalf("AddRemote failed: %v", err)
	}
	info, err := svc.GetRemoteWebInfo(repo)
	if err != nil || info == nil || !info.HasRemote || info.Provider != "github" || info.WebBaseURL != "https://github.com/acme/demo" {
		t.Fatalf("GetRemoteWebInfo failed: %+v err=%v", info, err)
	}
	if !strings.Contains(info.CommitURLTpl, "/commit/{hash}") || !strings.Contains(info.FileURLTpl, "/blob/{branch}/{path}") {
		t.Fatalf("unexpected GitHub URL templates: %+v", info)
	}
	if err := svc.RenameRemote(repo, "origin", "upstream"); err != nil {
		t.Fatalf("RenameRemote failed: %v", err)
	}
	info, err = svc.GetRemoteWebInfo(repo)
	if err != nil || info == nil || !info.HasRemote || info.RemoteName != "upstream" {
		t.Fatalf("GetRemoteWebInfo should fall back to the first remote: %+v err=%v", info, err)
	}
	if err := svc.RemoveRemote(repo, "upstream"); err != nil {
		t.Fatalf("RemoveRemote failed: %v", err)
	}
	info, err = svc.GetRemoteWebInfo(repo)
	if err != nil || info == nil || info.HasRemote {
		t.Fatalf("expected no remote web info, got %+v err=%v", info, err)
	}

	parsed := ParseRemoteWebURL("git@gitlab.com:group/sub/repo.git")
	if !parsed.HasRemote || parsed.Provider != "gitlab" || parsed.WebBaseURL != "https://gitlab.com/group/sub/repo" {
		t.Fatalf("unexpected GitLab SSH parse: %+v", parsed)
	}
	parsed = ParseRemoteWebURL("ssh://git@gitea.example.com:2222/team/repo.git")
	if !parsed.HasRemote || parsed.Provider != "gitea" || parsed.WebBaseURL != "https://gitea.example.com/team/repo" {
		t.Fatalf("unexpected Gitea SSH parse: %+v", parsed)
	}
	parsed = ParseRemoteWebURL("https://bitbucket.org/team/repo.git")
	if !parsed.HasRemote || parsed.Provider != "bitbucket" || !strings.Contains(parsed.FileURLTpl, "/src/{branch}/") {
		t.Fatalf("unexpected Bitbucket parse: %+v", parsed)
	}
	parsed = ParseRemoteWebURL("https://gitee.com/team/repo")
	if !parsed.HasRemote || parsed.Provider != "gitee" {
		t.Fatalf("unexpected Gitee parse: %+v", parsed)
	}
	parsed = ParseRemoteWebURL("https://example.com/team/repo.git")
	if !parsed.HasRemote || parsed.Provider != "generic" {
		t.Fatalf("unexpected generic parse: %+v", parsed)
	}
	if parsed := ParseRemoteWebURL(""); parsed.HasRemote || parsed.RemoteURL != "" {
		t.Fatalf("empty remote URL should not be web-capable: %+v", parsed)
	}
	if parsed := ParseRemoteWebURL("/tmp/local.git"); parsed.HasRemote {
		t.Fatalf("local path should not be web-capable: %+v", parsed)
	}

	globalConfig := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	if err := svc.SetExternalToolConfig(repo, "meld", "vimdiff", "meld $LOCAL $REMOTE", "vimdiff $LOCAL $REMOTE $BASE $MERGED", false); err != nil {
		t.Fatalf("SetExternalToolConfig local failed: %v", err)
	}
	tools, err := svc.GetExternalToolConfig(repo)
	if err != nil || tools.DiffTool != "meld" || tools.MergeTool != "vimdiff" || !strings.Contains(tools.CustomDiffCmd, "meld") || tools.IsGlobal {
		t.Fatalf("GetExternalToolConfig local failed: %+v err=%v", tools, err)
	}
	for _, key := range []string{"diff.tool", "merge.tool", "wgit.customDiffCmd", "wgit.customMergeCmd"} {
		if _, err := ExecGit(repo, "config", "--unset", key); err != nil {
			t.Fatalf("unset %s failed: %v", key, err)
		}
	}
	if err := svc.SetExternalToolConfig(repo, "bc", "p4merge", "bc $LOCAL $REMOTE", "p4merge $LOCAL $REMOTE $BASE $MERGED", true); err != nil {
		t.Fatalf("SetExternalToolConfig global failed: %v", err)
	}
	tools, err = svc.GetExternalToolConfig(repo)
	if err != nil || tools.DiffTool != "bc" || tools.MergeTool != "p4merge" || !strings.Contains(tools.CustomMergeCmd, "p4merge") || !tools.IsGlobal {
		t.Fatalf("GetExternalToolConfig global failed: %+v err=%v", tools, err)
	}
	if data, err := os.ReadFile(globalConfig); err != nil || !strings.Contains(string(data), "p4merge") {
		t.Fatalf("expected isolated global config to contain tool settings, got %q err=%v", data, err)
	}
	if err := svc.SetExternalToolConfig(repo, "bc", "p4merge", "", "", true); err != nil {
		t.Fatalf("clearing custom commands failed: %v", err)
	}
	tools, err = svc.GetExternalToolConfig(repo)
	if err != nil || tools.CustomDiffCmd != "" || tools.CustomMergeCmd != "" {
		t.Fatalf("custom commands should be cleared, got %+v err=%v", tools, err)
	}
	if data, err := os.ReadFile(globalConfig); err == nil && (strings.Contains(string(data), "customDiffCmd") || strings.Contains(string(data), "customMergeCmd")) {
		t.Fatalf("global config still contains cleared custom commands: %q", data)
	}

	if err := svc.SetExternalToolConfig(repo, "meld", "vimdiff", "meld $LOCAL $REMOTE", "vimdiff $LOCAL $REMOTE $BASE $MERGED", false); err != nil {
		t.Fatalf("SetExternalToolConfig local failed: %v", err)
	}
	gitDir := filepath.Join(repo, ".git")
	if err := os.Chmod(gitDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gitDir, 0o755) })
	if err := svc.SetExternalToolConfig(repo, "", "", "", "", false); err == nil {
		t.Fatal("failed unset must not be reported as cleared")
	}
	if err := os.Chmod(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := ExecGit(repo, "config", "--local", "--get", "diff.tool"); err != nil || strings.TrimSpace(out) != "meld" {
		t.Fatalf("tool config should remain after failed clear: %q err=%v", out, err)
	}
	if err := svc.SetExternalToolConfig(repo, "", "", "", "", false); err != nil {
		t.Fatalf("clearing tool config after restoring permissions failed: %v", err)
	}
	if out, _ := ExecGit(repo, "config", "--local", "--get", "diff.tool"); strings.TrimSpace(out) != "" {
		t.Fatalf("local diff.tool was not cleared: %q", out)
	}
}

func TestStashDropApplyPatchContentAndGitVersion(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	patchFile := filepath.Join(repo, "patch.txt")
	if err := os.WriteFile(patchFile, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "patch.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: patch base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	patch := "diff --git a/patch.txt b/patch.txt\n--- a/patch.txt\n+++ b/patch.txt\n@@ -1 +1 @@\n-old\n+new\n"
	if out, err := svc.ApplyPatchContent(repo, patch, true); err != nil {
		t.Fatalf("ApplyPatchContent check failed: %q err=%v", out, err)
	}
	if data, _ := os.ReadFile(patchFile); string(data) != "old\n" {
		t.Fatalf("check-only patch must not modify the file: %q", data)
	}
	if out, err := svc.ApplyPatchContent(repo, patch, false); err != nil {
		t.Fatalf("ApplyPatchContent failed: %q err=%v", out, err)
	}
	if data, _ := os.ReadFile(patchFile); string(data) != "new\n" {
		t.Fatalf("ApplyPatchContent did not apply the patch: %q", data)
	}
	if _, err := svc.ApplyPatchContent(repo, "   ", false); err == nil {
		t.Fatal("expected error for empty patch content")
	}
	if _, err := ExecGit(repo, "reset", "--hard", "HEAD"); err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	if err := os.WriteFile(patchFile, []byte("stashed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StashSave(repo, "drop me", false, false); err != nil {
		t.Fatalf("StashSave failed: %v", err)
	}
	if stashes, _ := svc.GetStashes(repo); len(stashes) != 1 {
		t.Fatalf("expected one stash before drop, got %+v", stashes)
	}
	if err := svc.StashDrop(repo, 0); err != nil {
		t.Fatalf("StashDrop failed: %v", err)
	}
	if stashes, _ := svc.GetStashes(repo); len(stashes) != 0 {
		t.Fatalf("expected no stashes after drop, got %+v", stashes)
	}
	if data, _ := os.ReadFile(patchFile); string(data) != "old\n" {
		t.Fatalf("StashDrop should keep the working tree clean, got %q", data)
	}

	version, err := svc.GetGitVersion()
	if err != nil || version == nil || !strings.Contains(version.Version, "git version") || version.System == "" {
		t.Fatalf("GetGitVersion failed: %+v err=%v", version, err)
	}
}
