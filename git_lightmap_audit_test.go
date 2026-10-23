package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodLightmapScene = `%YAML 1.1
--- !u!157 &1
LightmapSettings:
  m_ObjectHideFlags: 0
  m_GIWorkflowMode: 1
  m_LightmapEditorSettings:
    m_Resolution: 2
    m_BakeResolution: 40
    m_AtlasSize: 1024
    m_TextureCompression: 1
    m_LightmapsMode: 1
  m_EnableBakedLightmaps: 1
--- !u!1 &2
GameObject:
  m_Name: Main
`

const badLightmapScene = `%YAML 1.1
--- !u!157 &1
LightmapSettings:
  m_GIWorkflowMode: 1
  m_LightmapEditorSettings:
    m_BakeResolution: 300
    m_AtlasSize: 8192
    m_TextureCompression: 0
    m_LightmapsMode: 1
  m_EnableBakedLightmaps: 1
--- !u!1 &2
GameObject:
  m_Name: Bad
`

func TestAuditLightmapBakeSettingsParsesUnityScenes(t *testing.T) {
	repo := t.TempDir()
	levels := filepath.Join(repo, "Assets/Levels")
	if err := os.MkdirAll(levels, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(levels, "Good.unity"), []byte(goodLightmapScene), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(levels, "Bad.unity"), []byte(badLightmapScene), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.AuditLightmapBakeSettings(repo, "Assets/Levels")
	if err != nil {
		t.Fatalf("audit lightmap settings: %v", err)
	}
	if res.TotalMeshes != 2 || res.ExcessiveResolutionCount != 1 {
		t.Fatalf("unexpected scene counts: %+v", res)
	}
	if res.HealthScore != 75 {
		t.Fatalf("expected one CRITICAL warning to score 75, got %d", res.HealthScore)
	}
	if res.EstimatedBakeMinutes <= 0 {
		t.Fatalf("atlas-based bake estimate should be positive, got %d", res.EstimatedBakeMinutes)
	}
	if len(res.OverlappingMeshes) != 1 {
		t.Fatalf("expected one warning item, got %+v", res.OverlappingMeshes)
	}
	warning := res.OverlappingMeshes[0]
	if warning.MeshPath != "Assets/Levels/Bad.unity" || warning.Resolution != 300 || warning.Severity != "CRITICAL" {
		t.Fatalf("unexpected warning item: %+v", warning)
	}
	if !strings.Contains(warning.Recommendation, "Bake Resolution") || !strings.Contains(warning.Recommendation, "Atlas") {
		t.Fatalf("warning should explain resolution and atlas issues: %q", warning.Recommendation)
	}
	if !strings.Contains(res.BakeSettingsSummary, "40-300") || !strings.Contains(res.BakeSettingsSummary, "1024-8192") {
		t.Fatalf("summary should show real setting ranges: %q", res.BakeSettingsSummary)
	}
	if !strings.Contains(res.BakeSettingsSummary, "纹理压缩 1/2") {
		t.Fatalf("summary should report compression state: %q", res.BakeSettingsSummary)
	}
	if !strings.Contains(res.Summary, "UV2") || !strings.Contains(res.Summary, "未评估") {
		t.Fatalf("summary must state that UV2 overlap was not evaluated: %q", res.Summary)
	}
}

func TestAuditLightmapBakeSettingsFallsBackToLightingAsset(t *testing.T) {
	repo := t.TempDir()
	settingsDir := filepath.Join(repo, "Assets/Settings")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	asset := `%YAML 1.1
--- !u!850595691 &1
LightingSettings:
  m_BakeResolution: 50
  m_AtlasSize: 2048
  m_TextureCompression: 1
  m_LightmapsMode: 1
`
	if err := os.WriteFile(filepath.Join(settingsDir, "MainLighting.lighting"), []byte(asset), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.AuditLightmapBakeSettings(repo, "")
	if err != nil {
		t.Fatalf("audit lighting asset: %v", err)
	}
	if res.TotalMeshes != 1 || res.HealthScore != 100 || len(res.OverlappingMeshes) != 0 {
		t.Fatalf("unexpected lighting asset audit: %+v", res)
	}
	if !strings.Contains(res.BakeSettingsSummary, "Bake Resolution 50") || !strings.Contains(res.BakeSettingsSummary, "Atlas 2048") {
		t.Fatalf("summary should reflect real lighting asset values: %q", res.BakeSettingsSummary)
	}
}

func TestAuditLightmapBakeSettingsEmptyAndMissingPath(t *testing.T) {
	res, err := GitService{}.AuditLightmapBakeSettings(t.TempDir(), "")
	if err != nil {
		t.Fatalf("empty repo should not error: %v", err)
	}
	if res.TotalMeshes != 0 || res.HealthScore != 0 || !strings.Contains(res.Summary, "未检测到") {
		t.Fatalf("empty repo should be honest about missing settings: %+v", res)
	}

	if _, err := (GitService{}).AuditLightmapBakeSettings(t.TempDir(), "Assets/NoLevel"); err == nil {
		t.Fatal("missing target level path must return an error")
	}
}
