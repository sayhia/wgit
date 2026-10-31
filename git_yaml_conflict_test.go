package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSemanticYamlConflictResolver(t *testing.T) {
	// InitRepo records the repository as recent; keep the test out of the real
	// ~/.config/wgit config.
	t.Setenv("HOME", t.TempDir())
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	testFile := filepath.Join(repo, "Hero.prefab")

	// Craft a Unity YAML file with Git conflict markers
	conflictContent := `--- !u!1 &100
GameObject:
  m_Name: Hero
  m_Component:
<<<<<<< HEAD
  - component: {fileID: 200}
=======
  - component: {fileID: 300}
>>>>>>> feature/audio
--- !u!4 &101
Transform:
<<<<<<< HEAD
  m_LocalPosition: {x: 1, y: 2, z: 3}
=======
  m_LocalPosition: {x: 5, y: 5, z: 5}
>>>>>>> feature/audio
`
	if err := os.WriteFile(testFile, []byte(conflictContent), 0644); err != nil {
		t.Fatalf("failed to write test prefab: %v", err)
	}

	// 1. Analyze YAML 3-way conflict
	analysis, err := svc.AnalyzeYaml3WayConflict(repo, "Hero.prefab")
	if err != nil {
		t.Fatalf("AnalyzeYaml3WayConflict failed: %v", err)
	}

	if !analysis.IsUnityOrUeAsset {
		t.Errorf("expected Hero.prefab to be recognized as game asset")
	}
	if analysis.TotalConflictBlocks != 2 {
		t.Fatalf("expected 2 conflict blocks, got %d", analysis.TotalConflictBlocks)
	}
	if analysis.Blocks[0].ConflictType != "both_added_component" {
		t.Errorf("expected block 0 both_added_component, got %s", analysis.Blocks[0].ConflictType)
	}
	if analysis.Blocks[1].ConflictType != "transform_modified" {
		t.Errorf("expected block 1 transform_modified, got %s", analysis.Blocks[1].ConflictType)
	}

	// 2. Auto resolve using smart_auto strategy
	resolveRes, err := svc.AutoResolveSemanticYamlConflict(repo, "Hero.prefab", "smart_auto")
	if err != nil {
		t.Fatalf("AutoResolveSemanticYamlConflict failed: %v", err)
	}

	if !resolveRes.Success {
		t.Fatalf("expected resolution success: %s", resolveRes.Message)
	}
	if resolveRes.RemainingConflicts != 0 {
		t.Errorf("expected 0 remaining conflicts, got %d", resolveRes.RemainingConflicts)
	}

	// Verify both components are kept in output
	resolvedContent, _ := os.ReadFile(testFile)
	resStr := string(resolvedContent)
	if !strings.Contains(resStr, "fileID: 200") || !strings.Contains(resStr, "fileID: 300") {
		t.Errorf("expected both component fileIDs preserved in output, got:\n%s", resStr)
	}
	if strings.Contains(resStr, "<<<<<<<") {
		t.Errorf("output should not contain conflict markers")
	}
}
