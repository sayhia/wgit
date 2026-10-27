package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// submoduleTestGit runs git with an isolated identity/config for fixture setup.
func submoduleTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{
		"-c", "user.name=Submodule Tester",
		"-c", "user.email=submodule@example.com",
		"-c", "protocol.file.allow=always",
	}, args...)
	out, err := ExecGit(dir, full...)
	if err != nil {
		t.Fatalf("git %s failed: %v", strings.Join(args, " "), err)
	}
	return out
}

// submoduleTestSeedRemote creates a bare remote containing one commit.
func submoduleTestSeedRemote(t *testing.T, bareDir string) {
	t.Helper()
	seed := t.TempDir()
	submoduleTestGit(t, seed, "init")
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	submoduleTestGit(t, seed, "add", "README.md")
	submoduleTestGit(t, seed, "commit", "-m", "seed")
	submoduleTestGit(t, seed, "push", bareDir, "HEAD:refs/heads/main")
	// Point the bare remote's HEAD at the pushed branch so clones get a work tree.
	submoduleTestGit(t, bareDir, "symbolic-ref", "HEAD", "refs/heads/main")
}

func TestGetSubmodulesPrefetchesURLsAndKeepsOrder(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	remotes := map[string]string{}
	for _, name := range []string{"SubA", "SubB"} {
		bareDir := filepath.Join(t.TempDir(), name+".git")
		submoduleTestGit(t, t.TempDir(), "init", "--bare", bareDir)
		submoduleTestSeedRemote(t, bareDir)
		remotes[name] = bareDir
	}

	repo := t.TempDir()
	submoduleTestGit(t, repo, "init")

	submoduleTestGit(t, repo, "submodule", "add", remotes["SubA"], "Assets/SubA")
	submoduleTestGit(t, repo, "submodule", "add", remotes["SubB"], "Assets/SubB")
	submoduleTestGit(t, repo, "commit", "-m", "add submodules")

	list, err := GitService{}.GetSubmodules(repo)
	if err != nil {
		t.Fatalf("GetSubmodules failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 submodules, got %d: %+v", len(list), list)
	}

	wantPaths := []string{"Assets/SubA", "Assets/SubB"}
	wantNames := []string{"SubA", "SubB"}
	for i, sub := range list {
		if sub.Path != wantPaths[i] {
			t.Errorf("entry %d: expected path %q, got %q", i, wantPaths[i], sub.Path)
		}
		if sub.Name != wantNames[i] {
			t.Errorf("entry %d: expected name %q, got %q", i, wantNames[i], sub.Name)
		}
		if len(sub.Commit) != 40 {
			t.Errorf("entry %d: expected 40-char commit hash, got %q", i, sub.Commit)
		}
		if sub.Status != "synced" {
			t.Errorf("entry %d: expected synced status, got %q", i, sub.Status)
		}
	}

	// Compare URLs against the values actually recorded in .gitmodules.
	configOut, err := ExecGit(repo, "config", "-f", ".gitmodules", "--get-regexp", `^submodule\..*\.(url|path)$`)
	if err != nil {
		t.Fatalf("reading .gitmodules config failed: %v", err)
	}
	wantURL := map[string]string{}
	for _, line := range strings.Split(configOut, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimPrefix(parts[0], "submodule.")
		if strings.HasSuffix(key, ".url") {
			wantURL[strings.TrimSuffix(key, ".url")] = parts[1]
		}
	}

	for _, sub := range list {
		want, ok := wantURL[sub.Path]
		if !ok {
			t.Fatalf("no .gitmodules url recorded for %q", sub.Path)
		}
		if sub.URL == "" {
			t.Errorf("expected non-empty URL for %q", sub.Path)
		}
		if sub.URL != want {
			t.Errorf("entry %q: expected URL %q, got %q", sub.Path, want, sub.URL)
		}
	}
}

func TestGetSubmodulesWithoutGitmodulesReturnsEmpty(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	repo := t.TempDir()
	submoduleTestGit(t, repo, "init")

	list, err := GitService{}.GetSubmodules(repo)
	if err != nil {
		t.Fatalf("GetSubmodules without .gitmodules failed: %v", err)
	}
	if list == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %+v", list)
	}
}
