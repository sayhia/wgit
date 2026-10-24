package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShaderSelfHealingUsesRealIncludesMaterialsAndCache(t *testing.T) {
	repo := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(repo, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("ProjectSettings/ProjectVersion.txt", "m_EditorVersion: 2022.3.10f1\n")
	write("Packages/manifest.json", `{"dependencies":{"com.unity.render-pipelines.universal":"14.0.0"}}`)
	write("Assets/Shaders/Common.cginc", "// common\n")
	write("Assets/Shaders/Good.shader", "#include \"Common.cginc\"\nShader \"Good\" {}\n")
	write("Assets/Shaders/Broken.shader", "#include \"Missing.cginc\"\nShader \"Broken\" {}\n")
	write("Assets/Shaders/Valid.shader.meta", "fileFormatVersion: 2\nguid: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n")
	write("Assets/Materials/Valid.mat", "Material:\n  m_Shader: {fileID: 4800000, guid: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa, type: 3}\n")
	write("Assets/Materials/Broken.mat", "Material:\n  m_Shader: {fileID: 4800000, guid: deadbeefdeadbeefdeadbeefdeadbeef, type: 3}\n")
	write("Library/ShaderCache/cache.bin", "cache")
	write("Temp/ShaderCache/cache.bin", "cache")

	svc := GitService{}
	res, err := svc.ShaderSelfHealingWizard(repo, "STATUS")
	if err != nil {
		t.Fatalf("ShaderSelfHealingWizard failed: %v", err)
	}
	if res.EngineType != "Unity URP" || res.BrokenShaderCount != 1 || res.ShaderCacheCleared {
		t.Fatalf("unexpected shader audit: %+v", res)
	}
	if res.HealthGrade != "ATTENTION_REQUIRED" || !strings.Contains(res.Summary, "缺失 Shader 引用材质 1 个") {
		t.Fatalf("broken material should be reported: %+v", res)
	}
	if res.BlackRenderMeshesCount != 0 || !strings.Contains(res.Summary, "渲染期验证") {
		t.Fatalf("black meshes must be explicitly deferred to runtime: %+v", res)
	}

	if _, err := svc.ShaderSelfHealingWizard(repo, "NOT_AN_ACTION"); err == nil {
		t.Fatal("unknown shader healing actions must fail instead of silently scanning")
	}
	res, err = svc.ShaderSelfHealingWizard(repo, "HEAL_ALL")
	if err != nil {
		t.Fatalf("ShaderSelfHealingWizard HEAL_ALL failed: %v", err)
	}
	if !res.ShaderCacheCleared || !strings.Contains(res.Summary, "已清理 2 个缓存目录") {
		t.Fatalf("cache dirs should be cleared: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(repo, "Library", "ShaderCache")); !os.IsNotExist(err) {
		t.Fatalf("Library/ShaderCache should be removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "Temp", "ShaderCache")); !os.IsNotExist(err) {
		t.Fatalf("Temp/ShaderCache should be removed: %v", err)
	}

	if err := os.Remove(filepath.Join(repo, "Assets", "Shaders", "Broken.shader")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repo, "Assets", "Materials", "Broken.mat")); err != nil {
		t.Fatal(err)
	}
	res, err = svc.ShaderSelfHealingWizard(repo, "STATUS")
	if err != nil || res.BrokenShaderCount != 0 || res.HealthGrade != "OPTIMAL" {
		t.Fatalf("healthy shader graph should be OPTIMAL: %+v err=%v", res, err)
	}
}
