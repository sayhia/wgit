package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectAssetCyclicDependenciesUsesUnityGUIDs(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "Assets", "Cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	guidA := strings.Repeat("a", 32)
	guidB := strings.Repeat("b", 32)
	writeAsset := func(name, guid, referenced string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name+".prefab"), []byte("%YAML 1.1\n--- !u!1 &1\nGameObject:\n  m_Script: {fileID: 11500000, guid: "+referenced+", type: 3}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".prefab.meta"), []byte("fileFormatVersion: 2\nguid: "+guid+"\nPrefabImporter:\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeAsset("PrefabA", guidA, guidB)
	writeAsset("PrefabB", guidB, guidA)

	res, err := (GitService{}).DetectAssetCyclicDependencies(repo, "Assets/Cycles")
	if err != nil {
		t.Fatalf("DetectAssetCyclicDependencies failed: %v", err)
	}
	if res.TotalAssetsScanned != 2 || res.CyclesCount != 1 || !res.HasCycles {
		t.Fatalf("expected one detected cycle: %+v", res)
	}
	cycle := res.Cycles[0]
	if cycle.Length != 2 || cycle.RiskLevel != "HIGH" {
		t.Fatalf("unexpected cycle metadata: %+v", cycle)
	}
	joined := strings.Join(cycle.CyclePath, "->")
	if !strings.Contains(joined, "PrefabA.prefab") || !strings.Contains(joined, "PrefabB.prefab") {
		t.Fatalf("unexpected cycle path: %+v", cycle.CyclePath)
	}

	if _, err := (GitService{}).DetectAssetCyclicDependencies(repo, "Assets/Missing"); err == nil {
		t.Fatal("missing scan directory must fail")
	}
}
