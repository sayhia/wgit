package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareMaterialProperties(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@example.com")

	oldMat := `%YAML 1.1
%TAG !u! tag:unity3d.com,2011:
--- !u!21 &2100000
Material:
  m_Shader: {fileID: 4800000, guid: 0000000000000000f000000000000000, type: 3}
  m_SavedProperties:
    m_TexEnvs:
    - _MainTex:
        m_Texture: {fileID: 2800000, guid: 11111111111111111111111111111111, type: 3}
    m_Floats:
    - _Glossiness: 0.5
    - _Metallic: 0.0
    m_Colors:
    - _Color: {r: 1, g: 0.5, b: 0, a: 1}
`

	newMat := `%YAML 1.1
%TAG !u! tag:unity3d.com,2011:
--- !u!21 &2100000
Material:
  m_Shader: {fileID: 4800000, guid: 99999999999999999999999999999999, type: 3}
  m_SavedProperties:
    m_TexEnvs:
    - _MainTex:
        m_Texture: {fileID: 2800000, guid: 22222222222222222222222222222222, type: 3}
    m_Floats:
    - _Glossiness: 0.8
    - _Metallic: 1.0
    m_Colors:
    - _Color: {r: 0, g: 1, b: 0, a: 1}
`

	matPath := filepath.Join(repo, "StandardMat.mat")
	_ = os.WriteFile(matPath, []byte(oldMat), 0644)
	_, _ = ExecGit(repo, "add", "StandardMat.mat")
	_, _ = ExecGit(repo, "commit", "-m", "initial material")

	_ = os.WriteFile(matPath, []byte(newMat), 0644)
	_, _ = ExecGit(repo, "add", "StandardMat.mat")
	_, _ = ExecGit(repo, "commit", "-m", "update material roughness and shader")

	res, err := svc.CompareMaterialProperties(repo, "StandardMat.mat", "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("CompareMaterialProperties failed: %v", err)
	}

	if !res.ShaderChanged {
		t.Error("expected ShaderChanged to be true")
	}
	if res.TotalChanges == 0 {
		t.Error("expected TotalChanges > 0")
	}

	foundColor := false
	for _, prop := range res.Properties {
		if prop.Name == "_Color" {
			foundColor = true
			if prop.ColorHexOld != "#FF7F00" && prop.ColorHexOld != "#FF8000" {
				t.Errorf("expected orange hex for old color, got %s", prop.ColorHexOld)
			}
			if prop.ColorHexNew != "#00FF00" {
				t.Errorf("expected green hex for new color, got %s", prop.ColorHexNew)
			}
		}
	}
	if !foundColor {
		t.Error("_Color property not found in parsed diff")
	}
}

func TestInspectTextureMetadata(t *testing.T) {
	svc := GitService{}
	meta, err := svc.InspectTextureMetadata("/fake/repo", "Assets/Textures/NormalMap.tga")
	if err != nil {
		t.Fatalf("InspectTextureMetadata failed: %v", err)
	}
	if meta.Format != "tga" || !meta.HasAlpha {
		t.Errorf("unexpected meta: %+v", meta)
	}
}
