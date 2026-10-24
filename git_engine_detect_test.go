package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 0x40, G: 0x80, B: 0xc0, A: 0xff})
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestGameEngineDetectionPresetsAndCleanup(t *testing.T) {
	svc := GitService{}

	unity := t.TempDir()
	if err := os.MkdirAll(filepath.Join(unity, "Assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(unity, "ProjectSettings"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unity, "ProjectSettings", "ProjectVersion.txt"), []byte("m_EditorVersion: 2022.3.10f1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unity, ".gitattributes"), []byte("*.psd filter=lfs diff=lfs merge=lfs -text\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	unityInfo, err := svc.DetectGameEngineProject(unity)
	if err != nil || unityInfo.EngineType != "Unity" || unityInfo.EngineVersion != "2022.3.10f1" || !unityInfo.HasLfsRules {
		t.Fatalf("DetectGameEngineProject(unity) failed: %v, info=%+v", err, unityInfo)
	}
	if preset := svc.GetGameEngineGitAttributesPreset("unity"); !strings.Contains(preset, "unityyamlmerge") && !strings.Contains(preset, "filter=lfs") {
		t.Fatalf("Unity preset missing expected rules: %q", preset)
	}

	for _, dir := range []string{"Library", "Temp", "Logs", "obj"} {
		if err := os.MkdirAll(filepath.Join(unity, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(unity, dir, "cache.bin"), []byte("cache"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	workflows, err := svc.DetectGameEngineWorkflows(unity)
	if err != nil || len(workflows) == 0 || workflows[0].EngineType != "unity" || len(workflows[0].CacheFolders) == 0 {
		t.Fatalf("DetectGameEngineWorkflows(unity) failed: %v, workflows=%+v", err, workflows)
	}
	cleanUnity, err := svc.CleanGameEngineCaches(unity, "unity")
	if err != nil || cleanUnity == nil || len(cleanUnity.CleanedFolders) != 4 {
		t.Fatalf("CleanGameEngineCaches(unity) failed: %v, res=%+v", err, cleanUnity)
	}
	for _, dir := range []string{"Library", "Temp", "Logs", "obj"} {
		if _, err := os.Stat(filepath.Join(unity, dir)); !os.IsNotExist(err) {
			t.Fatalf("Unity cache dir %s was not removed (err=%v)", dir, err)
		}
	}

	unreal := t.TempDir()
	if err := os.WriteFile(filepath.Join(unreal, "MyGame.uproject"), []byte(`{"EngineAssociation": "5.3"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"Content", "Config", "Intermediate", "Saved", "DerivedDataCache"} {
		if err := os.MkdirAll(filepath.Join(unreal, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if dir == "Intermediate" || dir == "Saved" || dir == "DerivedDataCache" {
			if err := os.WriteFile(filepath.Join(unreal, dir, "cache.bin"), []byte("cache"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	unrealInfo, err := svc.DetectUnrealProject(unreal)
	if err != nil || !unrealInfo.IsUnrealProject || unrealInfo.ProjectName != "MyGame" || unrealInfo.EngineVersion != "5.3" {
		t.Fatalf("DetectUnrealProject failed: %v, info=%+v", err, unrealInfo)
	}
	if preset := svc.GetGameEngineGitAttributesPreset("unreal"); !strings.Contains(preset, "*.uasset") {
		t.Fatalf("Unreal preset missing uasset rule")
	}
	unrealWorkflows, err := svc.DetectGameEngineWorkflows(unreal)
	if err != nil || len(unrealWorkflows) == 0 || unrealWorkflows[0].EngineType != "unreal" {
		t.Fatalf("DetectGameEngineWorkflows(unreal) failed: %v, workflows=%+v", err, unrealWorkflows)
	}
	cacheFolders := unrealWorkflows[0].CacheFolders
	cleanUnreal, err := svc.CleanUnrealEngineCache(unreal)
	if err != nil || cleanUnreal == nil || len(cleanUnreal.CleanedFolders) != len(cacheFolders) || len(cacheFolders) == 0 {
		t.Fatalf("CleanUnrealEngineCache failed: %v, res=%+v, expected=%v", err, cleanUnreal, cacheFolders)
	}
	for _, dir := range cacheFolders {
		if _, err := os.Stat(filepath.Join(unreal, dir)); !os.IsNotExist(err) {
			t.Fatalf("Unreal cache dir %s was not removed (err=%v)", dir, err)
		}
	}

	// A folder that cannot be removed must not count towards the freed size.
	blockedRepo := t.TempDir()
	if err := os.WriteFile(filepath.Join(blockedRepo, "Blocked.uproject"), []byte(`{"EngineAssociation": "5.3"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"Intermediate", "Saved", "DerivedDataCache"} {
		if err := os.MkdirAll(filepath.Join(blockedRepo, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(blockedRepo, dir, "cache.bin"), []byte("cache"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	blockedDir := filepath.Join(blockedRepo, "Saved")
	if err := os.Chmod(blockedDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blockedDir, 0o755) })
	blockedClean, err := svc.CleanUnrealEngineCache(blockedRepo)
	if err != nil {
		t.Fatalf("CleanUnrealEngineCache on blocked folder failed: %v", err)
	}
	if len(blockedClean.CleanedFolders) != 2 || blockedClean.FreedSizeBytes != 10 {
		t.Fatalf("freed size must reflect successful removals only: %+v", blockedClean)
	}
	if _, err := os.Stat(filepath.Join(blockedDir, "cache.bin")); err != nil {
		t.Fatalf("blocked cache folder should remain: %v", err)
	}

	godot := t.TempDir()
	if err := os.WriteFile(filepath.Join(godot, "project.godot"), []byte("config/name=\"Demo\"\nconfig/features=PackedStringArray(\"4.2\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if info, err := svc.DetectGameEngineProject(godot); err != nil || info.EngineType != "Godot" {
		t.Fatalf("DetectGameEngineProject(godot) failed: %v, info=%+v", err, info)
	}

	cocos := t.TempDir()
	if err := os.WriteFile(filepath.Join(cocos, "project.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if info, err := svc.DetectGameEngineProject(cocos); err != nil || info.EngineType != "Cocos" {
		t.Fatalf("DetectGameEngineProject(cocos) failed: %v, info=%+v", err, info)
	}

	textures := t.TempDir()
	writeTestPNG(t, filepath.Join(textures, "Assets", "Textures", "Pot.png"), 4, 4)
	writeTestPNG(t, filepath.Join(textures, "Assets", "Textures", "NonPot.png"), 3, 3)
	lint, err := svc.LintGameTextures(textures, true)
	if err != nil || lint.TotalChecked != 2 || lint.TotalIssues == 0 {
		t.Fatalf("LintGameTextures failed: %v, res=%+v", err, lint)
	}
	var potFound, nonPotFound bool
	for _, item := range lint.Items {
		if strings.HasSuffix(item.Path, "Pot.png") && item.IsPOT {
			potFound = true
		}
		if strings.HasSuffix(item.Path, "NonPot.png") && !item.IsPOT {
			nonPotFound = true
		}
	}
	if !potFound || !nonPotFound {
		t.Fatalf("Texture lint did not classify POT correctly: %+v", lint.Items)
	}
}
