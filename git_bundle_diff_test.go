package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeBundleDiffParsesJSONManifests(t *testing.T) {
	repo := t.TempDir()
	base := `{
  "bundles": [
    {"name": "hero.bundle", "sizeBytes": 1000, "hash": "aaa", "assets": ["Assets/Hero.prefab", "Assets/Shared.mat"]},
    {"name": "ui.bundle", "sizeBytes": 500, "hash": "bbb", "assets": ["Assets/UI.png", "Assets/Shared.mat"]},
    {"name": "legacy.bundle", "sizeBytes": 100, "hash": "old", "assets": ["Assets/Legacy.prefab"]}
  ]
}`
	target := `{
  "bundles": [
    {"name": "hero.bundle", "sizeBytes": 1200, "hash": "ccc", "assets": ["Assets/Hero.prefab", "Assets/Shared.mat"]},
    {"name": "ui.bundle", "sizeBytes": 500, "hash": "bbb", "assets": ["Assets/UI.png", "Assets/Shared.mat"]},
    {"name": "new.bundle", "sizeBytes": 300, "hash": "ddd", "assets": ["Assets/New.fbx"]}
  ]
}`
	if err := os.WriteFile(filepath.Join(repo, "base.json"), []byte(base), 0o644); err != nil {
		t.Fatalf("write base manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "target.json"), []byte(target), 0o644); err != nil {
		t.Fatalf("write target manifest: %v", err)
	}

	res, err := GitService{}.AnalyzeBundleDiff(repo, "base.json", "target.json")
	if err != nil {
		t.Fatalf("analyze bundle diff: %v", err)
	}
	if res.TotalBundlesBase != 3 || res.TotalBundlesTarget != 3 {
		t.Fatalf("unexpected bundle totals: %+v", res)
	}
	if res.InitialInstallDeltaBytes != 400 || res.PatchDownloadBytes != 1500 {
		t.Fatalf("unexpected size estimates: initial=%d patch=%d", res.InitialInstallDeltaBytes, res.PatchDownloadBytes)
	}
	if res.InitialInstallDeltaStr != "+400 B" || res.PatchDownloadStr != "1.5 KB" {
		t.Fatalf("unexpected size strings: %q / %q", res.InitialInstallDeltaStr, res.PatchDownloadStr)
	}
	if len(res.ChangedBundles) != 3 {
		t.Fatalf("expected added/modified/removed bundles, got %+v", res.ChangedBundles)
	}
	changes := make(map[string]BundleDiffItem, len(res.ChangedBundles))
	for _, item := range res.ChangedBundles {
		changes[item.BundleName] = item
	}
	if item := changes["hero.bundle"]; item.ChangeType != "MODIFIED" || item.DeltaBytes != 200 || item.AssetCount != 2 {
		t.Fatalf("unexpected hero diff: %+v", item)
	}
	if item := changes["new.bundle"]; item.ChangeType != "ADDED" || item.DeltaBytes != 300 {
		t.Fatalf("unexpected new bundle diff: %+v", item)
	}
	if item := changes["legacy.bundle"]; item.ChangeType != "REMOVED" || item.DeltaBytes != -100 || item.DeltaSizeStr != "-100 B" {
		t.Fatalf("unexpected removed bundle diff: %+v", item)
	}
	if len(res.DuplicateAssetLeaks) != 1 || res.DuplicateAssetLeaks[0].AssetPath != "Assets/Shared.mat" {
		t.Fatalf("expected one duplicate asset leak: %+v", res.DuplicateAssetLeaks)
	}
	if got := res.DuplicateAssetLeaks[0].Bundles; len(got) != 2 || got[0] != "hero.bundle" || got[1] != "ui.bundle" {
		t.Fatalf("unexpected duplicate owners: %+v", got)
	}
	if !strings.Contains(res.Summary, "新增 1 / 修改 1 / 移除 1") || !strings.Contains(res.Summary, "重复打包") {
		t.Fatalf("summary should report real counts and duplicate state: %q", res.Summary)
	}
}

func TestAnalyzeBundleDiffSupportsNameMapping(t *testing.T) {
	repo := t.TempDir()
	base := `{"hero.bundle": {"size": 100, "hash": "a"}, "old.bundle": {"size": 50}}`
	target := `{"hero.bundle": {"size": 100, "hash": "b"}}`
	if err := os.WriteFile(filepath.Join(repo, "base.json"), []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "target.json"), []byte(target), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.AnalyzeBundleDiff(repo, "base.json", "target.json")
	if err != nil {
		t.Fatalf("analyze mapping manifest: %v", err)
	}
	if len(res.ChangedBundles) != 2 {
		t.Fatalf("expected hash modification and removal, got %+v", res.ChangedBundles)
	}
	if !strings.Contains(res.Summary, "清单未提供资产列表") {
		t.Fatalf("summary must not claim duplicate-free without asset lists: %q", res.Summary)
	}
}

func TestParseUnityBundleManifestHashesAndAssets(t *testing.T) {
	data := []byte(`ManifestFileVersion: 0
Hashes:
  hero.bundle: 0123456789abcdef
Assets:
- Assets/Hero.prefab
- Assets/Shared.mat
Dependencies: {}
`)
	entries, err := parseUnityBundleManifest(data)
	if err != nil {
		t.Fatalf("parse unity manifest: %v", err)
	}
	entry, ok := entries["hero.bundle"]
	if !ok {
		t.Fatalf("expected hero.bundle entry, got %+v", entries)
	}
	if entry.version != "0123456789abcdef" || entry.assetCount != 2 || len(entry.assets) != 2 {
		t.Fatalf("unexpected unity entry: %+v", entry)
	}

	same := entry
	if bundleManifestEntryChanged(entry, same) {
		t.Fatal("identical manifest entries must not be reported as changed")
	}
	changed := entry
	changed.version = "fedcba9876543210"
	if !bundleManifestEntryChanged(entry, changed) {
		t.Fatal("hash change must be detected")
	}
}

func TestAnalyzeBundleDiffRejectsMissingManifest(t *testing.T) {
	if _, err := (GitService{}).AnalyzeBundleDiff(t.TempDir(), "missing-base.json", "missing-target.json"); err == nil {
		t.Fatal("missing manifests must return an error")
	}
}
