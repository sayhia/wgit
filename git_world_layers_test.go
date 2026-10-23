package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffWorldDataLayersUsesTrackedAssets(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := ExecGit(repo, args...)
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
		return out
	}
	write := func(name string, content string) {
		t.Helper()
		full := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")

	write("Content/DataLayers/DL_Environment.uasset", "env-v1")
	write("Content/DataLayers/DL_NPC.uasset", "npc-v1")
	write("Content/Textures/T_Rock.uasset", "texture")
	write("Content/DataLayers/Notes.txt", "not an asset")
	run("add", "-A")
	run("commit", "-qm", "data layers base")

	write("Content/DataLayers/DL_Environment.uasset", "env-v2")
	if err := os.Remove(filepath.Join(repo, "Content/DataLayers/DL_NPC.uasset")); err != nil {
		t.Fatal(err)
	}
	write("Content/DataLayers/DL_Boss.uasset", "boss-v1")
	run("add", "-A")
	run("commit", "-qm", "data layers target")

	res, err := GitService{}.DiffWorldDataLayers(repo, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("diff world data layers: %v", err)
	}
	if res.TotalDataLayers != 2 {
		t.Fatalf("expected 2 data layer assets in target, got %d", res.TotalDataLayers)
	}
	if len(res.Layers) != 3 {
		t.Fatalf("expected added/modified/removed entries, got %+v", res.Layers)
	}
	changes := make(map[string]DataLayerDiffItem, len(res.Layers))
	for _, layer := range res.Layers {
		changes[layer.Name] = layer
	}
	if layer := changes["Content/DataLayers/DL_Environment"]; layer.ChangeType != "MODIFIED" {
		t.Fatalf("unexpected modified layer: %+v", layer)
	}
	if layer := changes["Content/DataLayers/DL_NPC"]; layer.ChangeType != "REMOVED" {
		t.Fatalf("unexpected removed layer: %+v", layer)
	}
	if layer := changes["Content/DataLayers/DL_Boss"]; layer.ChangeType != "ADDED" {
		t.Fatalf("unexpected added layer: %+v", layer)
	}
	for _, layer := range res.Layers {
		if layer.InitialRuntimeState != "Unknown" || layer.ActorCount != 0 || layer.IsRuntime {
			t.Fatalf("runtime state must not be fabricated: %+v", layer)
		}
		if layer.AddedActors == nil || layer.RemovedActors == nil {
			t.Fatalf("actor lists must be non-nil: %+v", layer)
		}
	}
	if !strings.Contains(res.Summary, "新增 1 / 修改 1 / 移除 1") || !strings.Contains(res.Summary, "UE Editor") {
		t.Fatalf("summary should state real counts and limitations: %q", res.Summary)
	}
	if strings.Contains(res.Summary, "TownHall") || strings.Contains(res.Summary, "Merchant") {
		t.Fatalf("summary must not contain former sample actors: %q", res.Summary)
	}
	if res.RuntimeBudgetOverrun || res.StreamingConflicts == nil {
		t.Fatalf("streaming evaluation must be empty, not fabricated: %+v", res)
	}
}

func TestDiffWorldDataLayersEmptyRepoIsHonest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("init: %v (%s)", err, out)
	}
	if out, err := ExecGit(repo, "config", "user.name", "Tester"); err != nil {
		t.Fatalf("config: %v (%s)", err, out)
	}
	if out, err := ExecGit(repo, "config", "user.email", "tester@example.com"); err != nil {
		t.Fatalf("config: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "readme.txt"), []byte("no data layers"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := ExecGit(repo, "add", "-A"); err != nil {
		t.Fatalf("add: %v (%s)", err, out)
	}
	if out, err := ExecGit(repo, "commit", "-qm", "base"); err != nil {
		t.Fatalf("commit: %v (%s)", err, out)
	}

	res, err := GitService{}.DiffWorldDataLayers(repo, "HEAD", "HEAD")
	if err != nil {
		t.Fatalf("empty world layers: %v", err)
	}
	if res.TotalDataLayers != 0 || len(res.Layers) != 0 || res.Layers == nil {
		t.Fatalf("empty repo should return no layers: %+v", res)
	}
	if !strings.Contains(res.Summary, "未发现 Unreal Data Layer 资产") {
		t.Fatalf("empty summary should be explicit: %q", res.Summary)
	}

	if _, err := (GitService{}).DiffWorldDataLayers(repo, "missing-ref", "HEAD"); err == nil {
		t.Fatal("invalid ref must return an error")
	}
}
