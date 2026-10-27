package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSparsePresetsLifecycle(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	// Create test folder tree
	_ = os.MkdirAll(filepath.Join(repo, "src"), 0755)
	_ = os.WriteFile(filepath.Join(repo, "src", "main.go"), []byte("package main"), 0644)
	_ = os.MkdirAll(filepath.Join(repo, "Assets", "Scripts"), 0755)
	_ = os.WriteFile(filepath.Join(repo, "Assets", "Scripts", "Player.cs"), []byte("class Player {}"), 0644)
	_ = os.MkdirAll(filepath.Join(repo, "Assets", "Scenes"), 0755)

	// Create scene with mock asset references
	sceneContent := `--- !u!1 &1
GameObject:
  m_Component:
  - component: {fileID: 100}
--- !u!21 &100
Material:
  m_Shader: {fileID: 4800000, guid: 0000000000000000f000000000000000, type: 3}
  path: Assets/Textures/Hero.png
`
	_ = os.WriteFile(filepath.Join(repo, "Assets", "Scenes", "Level1.unity"), []byte(sceneContent), 0644)

	// Initial commit so git commands succeed
	_, _ = ExecGit(repo, "config", "user.name", "Test")
	_, _ = ExecGit(repo, "config", "user.email", "test@test.com")
	_, _ = ExecGit(repo, "add", ".")
	_, _ = ExecGit(repo, "commit", "-m", "chore: setup test repo")

	// 1. Get presets
	presets, err := svc.GetSparsePresets(repo)
	if err != nil {
		t.Fatalf("GetSparsePresets failed: %v", err)
	}
	if len(presets) < 4 {
		t.Errorf("expected at least 4 presets, got %d", len(presets))
	}

	// 2. Hydrate scene LFS assets before sparse cone hides it
	sceneRes, err := svc.HydrateSceneLfsAssets(repo, filepath.Join("Assets", "Scenes", "Level1.unity"))
	if err != nil {
		t.Fatalf("HydrateSceneLfsAssets failed: %v", err)
	}
	if !sceneRes.Success || sceneRes.HydratedCount == 0 {
		t.Errorf("expected references extracted from scene, got %+v", sceneRes)
	}

	// 3. Apply role preset "programmer"
	applyRes, err := svc.ApplyRoleSparsePreset(repo, "programmer", []string{"extra/tools"})
	if err != nil {
		t.Fatalf("ApplyRoleSparsePreset failed: %v", err)
	}
	if !applyRes.Success {
		t.Errorf("expected Success=true, got %+v", applyRes)
	}

	// 4. Disable sparse with "full"
	fullRes, err := svc.ApplyRoleSparsePreset(repo, "full", nil)
	if err != nil {
		t.Fatalf("ApplyRoleSparsePreset(full) failed: %v", err)
	}
	if !fullRes.Success || len(fullRes.ActiveFolders) != 0 {
		t.Errorf("expected activeFolders empty after full checkout, got %+v", fullRes)
	}
}
