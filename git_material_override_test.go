package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMaterialTestFile(t *testing.T, repo string, rel string, content string) {
	t.Helper()
	path := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAuditMaterialInstanceOverridesParsesUnityAssets(t *testing.T) {
	repo := t.TempDir()
	writeMaterialTestFile(t, repo, "Assets/Shaders/Lit.shader", `Shader "Universal Render Pipeline/Lit" {
  Properties {
    _BaseMap ("Base Map", 2D) = "white" {}
    _Metallic ("Metallic", Range(0,1)) = 0
    _BaseColor ("Color", Color) = (1,1,1,1)
  }
}`)
	writeMaterialTestFile(t, repo, "Assets/Shaders/Lit.shader.meta", "fileFormatVersion: 2\nguid: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n")
	writeMaterialTestFile(t, repo, "Assets/Textures/T_Rock_Base.png", "png")
	writeMaterialTestFile(t, repo, "Assets/Textures/T_Rock_Base.png.meta", "fileFormatVersion: 2\nguid: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n")
	writeMaterialTestFile(t, repo, "Assets/Materials/M_Rock.mat", `%YAML 1.1
Material:
  m_Shader: {fileID: 4800000, guid: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa, type: 3}
  m_SavedProperties:
    serializedVersion: 3
    m_TexEnvs:
    - _BaseMap:
        m_Texture: {fileID: 2800000, guid: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb, type: 3}
        m_Scale: {x: 1, y: 1}
    - _DetailMap:
        m_Texture: {fileID: 0}
    m_Floats:
    - _Metallic: 0.65
    - _LegacyShininess: 0.2
    m_Colors:
    - _BaseColor: {r: 1, g: 0.5, b: 0.25, a: 1}
`)
	writeMaterialTestFile(t, repo, "Assets/Materials/M_Rock.mat.meta", "fileFormatVersion: 2\nguid: cccccccccccccccccccccccccccccccc\n")
	writeMaterialTestFile(t, repo, "Assets/Materials/M_Rock_Variant.mat", `%YAML 1.1
Material:
  m_Parent: {fileID: 2100000, guid: cccccccccccccccccccccccccccccccc, type: 2}
  m_Shader: {fileID: 4800000, guid: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa, type: 3}
  m_SavedProperties:
    m_Floats:
    - _Metallic: 0.2
`)
	writeMaterialTestFile(t, repo, "Assets/Materials/MI_UE.uasset", "binary")

	res, err := GitService{}.AuditMaterialInstanceOverrides(repo, "Assets/Materials")
	if err != nil {
		t.Fatalf("audit material overrides: %v", err)
	}
	if res.TotalInstances != 2 || res.OverriddenParametersCount != 6 || res.DeadParametersCount != 2 {
		t.Fatalf("unexpected material audit totals: %+v", res)
	}
	instances := map[string]MaterialInstanceItem{}
	for _, item := range res.Instances {
		instances[item.InstancePath] = item
	}
	rock, ok := instances["Assets/Materials/M_Rock.mat"]
	if !ok {
		t.Fatalf("missing parsed material: %+v", res.Instances)
	}
	if rock.ParentShader != "Assets/Shaders/Lit.shader" || rock.ParentMaterial != "" {
		t.Fatalf("shader/parent should resolve from GUIDs: %+v", rock)
	}
	overrides := map[string]MaterialParamOverride{}
	for _, override := range rock.Overrides {
		overrides[override.ParamName] = override
	}
	if got := overrides["_BaseMap"].Value; got != "Assets/Textures/T_Rock_Base.png" {
		t.Fatalf("texture path should resolve through .meta GUIDs, got %q", got)
	}
	if overrides["_Metallic"].Value != "0.65" || overrides["_BaseColor"].Value != "{r: 1, g: 0.5, b: 0.25, a: 1}" {
		t.Fatalf("scalar/color values should come from the material: %+v", overrides)
	}
	if !overrides["_DetailMap"].IsDead || !overrides["_LegacyShininess"].IsDead {
		t.Fatalf("properties absent from the shader should be dead: %+v", overrides)
	}
	if overrides["_BaseMap"].IsDead || overrides["_Metallic"].IsDead {
		t.Fatalf("valid shader properties must not be marked dead: %+v", overrides)
	}

	variant := instances["Assets/Materials/M_Rock_Variant.mat"]
	if variant.ParentMaterial != "Assets/Materials/M_Rock.mat" || variant.DeadCount != 0 || !variant.IsHealthy {
		t.Fatalf("material variant parent should resolve and stay healthy: %+v", variant)
	}
	if !strings.Contains(res.Summary, "审计 2 个") || !strings.Contains(res.Summary, "跳过 1 个 UE .uasset") {
		t.Fatalf("summary should state real counts and unsupported assets: %q", res.Summary)
	}
}

func TestAuditMaterialInstanceOverridesEmptyRepoIsHonest(t *testing.T) {
	res, err := GitService{}.AuditMaterialInstanceOverrides(t.TempDir(), "")
	if err != nil {
		t.Fatalf("audit empty material repo: %v", err)
	}
	if res.TotalInstances != 0 || len(res.Instances) != 0 {
		t.Fatalf("empty repo must not fabricate MI_Hero_Armor: %+v", res)
	}
	if !strings.Contains(res.Summary, "未发现") {
		t.Fatalf("empty repo summary should be explicit: %q", res.Summary)
	}
}
