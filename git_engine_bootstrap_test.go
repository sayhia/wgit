package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func initUnityRepo(t *testing.T) string {
	t.Helper()
	dir := initClosedLoopRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "Assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "ProjectSettings"), 0755); err != nil {
		t.Fatal(err)
	}
	writeAndCommit(t, dir, "Assets/Player.cs", "// player\n", "feat: player", time.Now(), "Alice")
	return dir
}

func TestRunEngineBootstrapUnityWritesPresets(t *testing.T) {
	dir := initUnityRepo(t)

	svc := GitService{}
	res, err := svc.RunEngineBootstrap(dir, EngineBootstrapOptions{
		WriteGitattributes: true,
		WriteGitignore:     true,
		UntrackArtifacts:   false,
	})
	if err != nil {
		t.Fatalf("RunEngineBootstrap failed: %v", err)
	}
	if res.Engine != "unity" {
		t.Fatalf("expected unity engine, got %q", res.Engine)
	}
	if res.Applied == 0 {
		t.Fatalf("expected some steps to apply, got %+v", res.Steps)
	}

	attrs, err := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if err != nil {
		t.Fatalf("read .gitattributes: %v", err)
	}
	if !strings.Contains(string(attrs), "merge=unityyamlmerge") {
		t.Fatalf(".gitattributes missing the Unity merge driver: %s", attrs)
	}
	if !strings.Contains(string(attrs), "filter=lfs") {
		t.Fatalf(".gitattributes missing LFS rules: %s", attrs)
	}

	ignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(ignore), "[Ll]ibrary/") {
		t.Fatalf(".gitignore missing the Library rule: %s", ignore)
	}

	if res.Readiness < 75 {
		t.Fatalf("expected readiness >= 75, got %d", res.Readiness)
	}
	if res.Message == "" {
		t.Fatal("expected a summary message")
	}
}

func TestRunEngineBootstrapIsIdempotent(t *testing.T) {
	dir := initUnityRepo(t)
	svc := GitService{}

	if _, err := svc.RunEngineBootstrap(dir, EngineBootstrapOptions{
		WriteGitattributes: true,
		WriteGitignore:     true,
	}); err != nil {
		t.Fatalf("first bootstrap failed: %v", err)
	}

	second, err := svc.RunEngineBootstrap(dir, EngineBootstrapOptions{
		WriteGitattributes: true,
		WriteGitignore:     true,
	})
	if err != nil {
		t.Fatalf("second bootstrap failed: %v", err)
	}
	if second.Applied != 0 {
		t.Fatalf("a second run should be a no-op, applied=%d steps=%+v", second.Applied, second.Steps)
	}
	if second.Skipped == 0 {
		t.Fatal("expected skipped entries on the second run")
	}
}

func TestMergeGitattributesFileKeepsCustomLines(t *testing.T) {
	dir := initClosedLoopRepo(t)
	custom := "*.custom text eol=lf\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte(custom), 0644); err != nil {
		t.Fatal(err)
	}

	added, err := mergeGitattributesFile(dir, "*.psd filter=lfs diff=lfs merge=lfs -text\n")
	if err != nil {
		t.Fatalf("mergeGitattributesFile failed: %v", err)
	}
	if added != 1 {
		t.Fatalf("expected 1 appended line, got %d", added)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if !strings.Contains(string(data), "*.custom text eol=lf") {
		t.Fatalf("custom line was lost: %s", data)
	}
	if !strings.Contains(string(data), "*.psd filter=lfs") {
		t.Fatalf("preset line missing: %s", data)
	}

	// A second merge must add nothing.
	again, err := mergeGitattributesFile(dir, "*.psd filter=lfs diff=lfs merge=lfs -text\n")
	if err != nil {
		t.Fatalf("second merge failed: %v", err)
	}
	if again != 0 {
		t.Fatalf("expected no new lines, got %d", again)
	}
}

func TestPresetKeyForEngineName(t *testing.T) {
	cases := map[string]string{
		"Unity 3D":      "unity",
		"Unreal Engine": "unreal",
		"Godot Engine":  "godot",
		"Cocos Creator": "cocos",
		"通用游戏工程":        "",
	}
	for in, want := range cases {
		if got := presetKeyForEngineName(in); got != want {
			t.Fatalf("presetKeyForEngineName(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- asset impact ---------------------------------------------------------

func writeUnityAsset(t *testing.T, dir, relPath, guid, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	meta := "fileFormatVersion: 2\nguid: " + guid + "\n"
	if err := os.WriteFile(full+".meta", []byte(meta), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeAssetImpactFindsDirectAndIndirect(t *testing.T) {
	dir := initClosedLoopRepo(t)

	texGuid := "1111111111111111111111111111aaaa"
	matGuid := "2222222222222222222222222222bbbb"

	writeUnityAsset(t, dir, "Assets/Tex/Hero.png", texGuid, "PNGDATA")
	// The material references the texture's GUID.
	writeUnityAsset(t, dir, "Assets/Mat/Hero.mat", matGuid,
		"Material:\n  m_Texture: {fileID: 2800000, guid: "+texGuid+", type: 3}\n")
	// The prefab references the material's GUID (level 2 for the texture).
	if err := os.MkdirAll(filepath.Join(dir, "Assets/Prefab"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Assets/Prefab/Hero.prefab"),
		[]byte("Prefab:\n  m_Material: {fileID: 2100000, guid: "+matGuid+", type: 2}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "-A")
	runGitTest(t, dir, nil, "commit", "-m", "art: hero assets")

	svc := GitService{}
	res, err := svc.AnalyzeAssetImpact(dir, "Assets/Tex/Hero.png", 2)
	if err != nil {
		t.Fatalf("AnalyzeAssetImpact failed: %v", err)
	}
	if res.DirectCount != 1 {
		t.Fatalf("expected 1 direct reference, got %d (%+v)", res.DirectCount, res.References)
	}
	if res.References[0].Path != "Assets/Mat/Hero.mat" {
		t.Fatalf("unexpected direct reference: %+v", res.References[0])
	}
	if res.IndirectCount != 1 {
		t.Fatalf("expected 1 indirect reference, got %d (%+v)", res.IndirectCount, res.References)
	}
	if res.TotalCount != 2 {
		t.Fatalf("expected 2 total references, got %d", res.TotalCount)
	}
	if res.References[0].Depth != 1 {
		t.Fatalf("references must be sorted direct-first: %+v", res.References)
	}
	if res.RiskLevel != "low" {
		t.Fatalf("expected low risk, got %q", res.RiskLevel)
	}
	if res.SafeToDelete {
		t.Fatal("an asset with references is not safe to delete")
	}
}

func TestAnalyzeAssetImpactWithoutReferences(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeUnityAsset(t, dir, "Assets/Tex/Orphan.png", "3333333333333333333333333333cccc", "PNG")
	runGitTest(t, dir, nil, "add", "-A")
	runGitTest(t, dir, nil, "commit", "-m", "art: orphan")

	svc := GitService{}
	res, err := svc.AnalyzeAssetImpact(dir, "Assets/Tex/Orphan.png", 1)
	if err != nil {
		t.Fatalf("AnalyzeAssetImpact failed: %v", err)
	}
	if res.TotalCount != 0 {
		t.Fatalf("expected no references, got %+v", res.References)
	}
	if res.RiskLevel != "none" {
		t.Fatalf("expected none risk, got %q", res.RiskLevel)
	}
	if !res.SafeToDelete {
		t.Fatal("an unreferenced asset should be safe to delete")
	}
}

func TestAnalyzeAssetImpactRejectsEmptyPath(t *testing.T) {
	dir := initClosedLoopRepo(t)
	svc := GitService{}
	if _, err := svc.AnalyzeAssetImpact(dir, "", 2); err == nil {
		t.Fatal("expected an error for an empty path")
	}
}

func TestAnalyzeAssetImpactWarnsWhenOwnerLookupFails(t *testing.T) {
	dir := initClosedLoopRepo(t)
	texGuid := "4444444444444444444444444444dddd"
	writeUnityAsset(t, dir, "Assets/Tex/Hero.png", texGuid, "PNGDATA")
	if err := os.MkdirAll(filepath.Join(dir, "Assets/Mat"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Assets/Mat/Hero.mat"),
		[]byte("Material:\n  m_Texture: {fileID: 2800000, guid: "+texGuid+", type: 3}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "-A")
	runGitTest(t, dir, nil, "commit", "-m", "art: hero")

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git not found: %v", err)
	}
	binDir := t.TempDir()
	wrapper := "#!/bin/sh\n" +
		"if [ \"$1\" = \"log\" ] && [ \"$2\" = \"-1\" ]; then\n" +
		"  echo 'fatal: simulated log failure' >&2\n" +
		"  exit 128\n" +
		"fi\n" +
		"exec \"$WGIT_REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WGIT_REAL_GIT", gitPath)
	t.Setenv("PATH", binDir)

	res, err := (GitService{}).AnalyzeAssetImpact(dir, "Assets/Tex/Hero.png", 1)
	if err != nil {
		t.Fatalf("AnalyzeAssetImpact failed: %v", err)
	}
	if len(res.References) != 1 {
		t.Fatalf("a failed attribution lookup must not drop references: %+v", res.References)
	}
	if len(res.Owners) != 0 {
		t.Fatalf("no owners can be known when the lookup failed: %v", res.Owners)
	}
	if !strings.Contains(strings.Join(res.Warnings, "\n"), "最后修改者") {
		t.Fatalf("expected an attribution warning, got %v", res.Warnings)
	}
}

func TestMergeGitattributesAbortsOnReadFailure(t *testing.T) {
	dir := initClosedLoopRepo(t)
	path := filepath.Join(dir, ".gitattributes")
	if err := os.WriteFile(path, []byte("*.keep text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o200); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	preset := "*.psd filter=lfs diff=lfs merge=lfs -text\n"
	if _, err := mergeGitattributesFile(dir, preset); err == nil {
		t.Fatal("unreadable .gitattributes must abort instead of being overwritten")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "*.keep text\n" {
		t.Fatalf("original .gitattributes must be preserved: %q err=%v", data, err)
	}
}
