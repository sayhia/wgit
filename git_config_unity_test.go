package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigPersistenceInTempHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := LoadConfig()
	if cfg.Theme != "dark" || len(cfg.RecentRepos) != 0 {
		t.Fatalf("unexpected default config: %+v", cfg)
	}
	cfg.Theme = "light"
	cfg.LastRepo = "/tmp/example"
	cfg.RecentRepos = []string{"/tmp/example"}
	SaveConfig(cfg)
	loaded := LoadConfig()
	if loaded.Theme != "light" || loaded.LastRepo != "/tmp/example" || len(loaded.RecentRepos) != 1 {
		t.Fatalf("LoadConfig did not return saved config: %+v", loaded)
	}
	if !strings.HasSuffix(getConfigPath(), filepath.Join(".config", "wgit", "config.json")) {
		t.Fatalf("unexpected config path: %s", getConfigPath())
	}

	parent := t.TempDir()
	repoA := filepath.Join(parent, "a")
	repoB := filepath.Join(parent, "b")
	for _, repo := range []string{repoA, repoB} {
		if err := os.MkdirAll(repo, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := ExecGit(repo, "init"); err != nil {
			t.Fatal(err)
		}
	}
	AddRecentRepo(repoA)
	AddRecentRepo(repoB)
	recent := LoadConfig()
	if len(recent.RecentRepos) != 2 || recent.RecentRepos[0] != repoB || recent.RecentRepos[1] != repoA {
		t.Fatalf("unexpected recent repos order: %+v", recent.RecentRepos)
	}
	AddRecentRepo(repoA)
	recent = LoadConfig()
	if len(recent.RecentRepos) != 2 || recent.RecentRepos[0] != repoA {
		t.Fatalf("AddRecentRepo did not deduplicate/prepend: %+v", recent.RecentRepos)
	}
	AddRecentRepo(filepath.Join(parent, "missing"))
	valid := (GitService{}).GetRecentRepos()
	if len(valid) != 2 {
		t.Fatalf("GetRecentRepos should filter missing paths, got %+v", valid)
	}
	for _, repo := range valid {
		if repo != repoA && repo != repoB {
			t.Fatalf("unexpected recent repo: %+v", valid)
		}
	}
}

func TestLoadConfigPreservesCorruptFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := getConfigPath()
	corrupt := []byte(`{"recentRepos":[`)
	if err := os.WriteFile(path, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadConfig()
	if cfg.Theme != "dark" || len(cfg.RecentRepos) != 0 {
		t.Fatalf("corrupt config must fall back to defaults: %+v", cfg)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("corrupt config should be moved aside, stat err=%v", err)
	}
	backups, err := filepath.Glob(path + ".corrupt-*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("corrupt config must be preserved once: %v %v", backups, err)
	}
	preserved, err := os.ReadFile(backups[0])
	if err != nil || !bytes.Equal(preserved, corrupt) {
		t.Fatalf("preserved config content mismatch: %q err=%v", preserved, err)
	}

	cfg.Theme = "light"
	cfg.RecentRepos = []string{"/tmp/restored"}
	SaveConfig(cfg)
	reloaded := LoadConfig()
	if reloaded.Theme != "light" || len(reloaded.RecentRepos) != 1 {
		t.Fatalf("config should be writable again after backup: %+v", reloaded)
	}
}

func TestConfigMigratesLegacyUgitFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	oldDir := filepath.Join(home, ".config", "ugit")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"recentRepos":["/tmp/legacy"],"lastRepo":"/tmp/legacy","theme":"light"}`
	if err := os.WriteFile(filepath.Join(oldDir, "config.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadConfig()
	if cfg.Theme != "light" || cfg.LastRepo != "/tmp/legacy" {
		t.Fatalf("legacy config was not migrated: %+v", cfg)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "wgit", "config.json")); err != nil {
		t.Fatalf("migrated config file missing: %v", err)
	}
}

func TestUnityYamlMergeConfiguration(t *testing.T) {
	repo := t.TempDir()
	if _, err := ExecGit(repo, "init"); err != nil {
		t.Fatal(err)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@example.com")

	svc := GitService{}
	status, err := svc.DetectUnityYamlMerge(repo)
	if err != nil || status.IsConfigured || status.HasGitattributes {
		t.Fatalf("unexpected initial UnityYAMLMerge status: %v, status=%+v", err, status)
	}

	toolPath := "/tmp/FakeUnity/UnityYAMLMerge"
	if err := svc.ConfigureUnityYamlMerge(repo, toolPath, true); err != nil {
		t.Fatalf("ConfigureUnityYamlMerge failed: %v", err)
	}
	driver, err := ExecGit(repo, "config", "--get", "merge.unityyamlmerge.driver")
	if err != nil || !strings.Contains(driver, toolPath) {
		t.Fatalf("merge driver not configured: %v, driver=%q", err, driver)
	}
	attrs, err := os.ReadFile(filepath.Join(repo, ".gitattributes"))
	if err != nil || !strings.Contains(string(attrs), "*.unity merge=unityyamlmerge") || !strings.Contains(string(attrs), "*.prefab merge=unityyamlmerge") {
		t.Fatalf("Unity merge rules missing: %v, attrs=%q", err, string(attrs))
	}
	status, err = svc.DetectUnityYamlMerge(repo)
	if err != nil || !status.IsConfigured || !status.HasGitattributes || !strings.Contains(status.ConfiguredToolPath, toolPath) {
		t.Fatalf("UnityYAMLMerge status not detected after configure: %v, status=%+v", err, status)
	}
}
