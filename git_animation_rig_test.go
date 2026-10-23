package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testRigGLTF = `{
  "nodes": [
    {"name": "Root", "children": [1]},
    {"name": "Hips", "children": [2]},
    {"name": "Spine"}
  ],
  "skins": [{"joints": [0, 1, 2]}],
  "animations": [{
    "name": "Run",
    "channels": [{"target": {"path": "rotation"}}, {"target": {"path": "translation"}}],
    "samplers": [{"input": 0, "interpolation": "LINEAR"}]
  }],
  "accessors": [{"count": 30, "min": [0.0], "max": [1.0]}]
}`

func TestLintAnimationRigUsesRealGLTFHierarchyAndClips(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "Hero.gltf"), []byte(testRigGLTF), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.LintAnimationRig(repo, "Hero.gltf", "MOBILE")
	if err != nil {
		t.Fatalf("lint gltf rig: %v", err)
	}
	if !res.AnalysisKnown || res.BoneCount != 3 || res.Severity != "OK" {
		t.Fatalf("unexpected glTF rig result: %+v", res)
	}
	depths := map[string]int{}
	for _, bone := range res.Bones {
		depths[bone.Name] = bone.Depth
	}
	if depths["Root"] != 0 || depths["Hips"] != 1 || depths["Spine"] != 2 {
		t.Fatalf("bone depths should come from the node hierarchy: %+v", depths)
	}
	if len(res.Clips) != 1 {
		t.Fatalf("expected one clip, got %+v", res.Clips)
	}
	clip := res.Clips[0]
	if clip.Name != "Run" || clip.DurationSeconds != 1 || clip.SampleRate != 30 || clip.FrameCount != 30 || clip.TrackCount != 2 {
		t.Fatalf("clip metadata should come from accessors/channels: %+v", clip)
	}
	if res.TotalDurationSeconds != 1 || res.OverallSampleRate != 30 || res.TotalTrackCount != 2 {
		t.Fatalf("unexpected aggregate rig metrics: %+v", res)
	}
	if res.MaxInfluencesPerVertex != 0 {
		t.Fatalf("skin weights are not parsed and must not default to 4: %+v", res)
	}
	if len(res.Issues) != 0 {
		t.Fatalf("no rig issue expected for this fixture: %+v", res.Issues)
	}
	if strings.Contains(res.Message, "Take 001") {
		t.Fatalf("messages must not contain fabricated clips: %q", res.Message)
	}

	writeTestGLB(t, filepath.Join(repo, "Hero.glb"), testRigGLTF)
	glbRes, err := GitService{}.LintAnimationRig(repo, "Hero.glb", "MOBILE")
	if err != nil {
		t.Fatalf("lint glb rig: %v", err)
	}
	if !glbRes.AnalysisKnown || glbRes.BoneCount != 3 || len(glbRes.Clips) != 1 || glbRes.Clips[0].Name != "Run" {
		t.Fatalf("GLB rig should be parsed from the JSON chunk: %+v", glbRes)
	}
}

func TestLintAnimationRigReadsWorktreeOverHEAD(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")

	path := filepath.Join(repo, "Hero.gltf")
	if err := os.WriteFile(path, []byte(testRigGLTF), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "Hero.gltf")
	run("commit", "-qm", "base rig")

	modified := `{
  "nodes": [{"name": "Root", "children": [1]}, {"name": "Hips"}],
  "skins": [{"joints": [0, 1]}],
  "animations": [],
  "accessors": []
}`
	if err := os.WriteFile(path, []byte(modified), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.LintAnimationRig(repo, "Hero.gltf", "MOBILE")
	if err != nil {
		t.Fatalf("lint modified rig: %v", err)
	}
	if !res.AnalysisKnown || res.BoneCount != 2 {
		t.Fatalf("audit should read the modified worktree, not HEAD: %+v", res)
	}
	if len(res.Clips) != 0 {
		t.Fatalf("modified worktree has no clips: %+v", res.Clips)
	}
}

func TestLintAnimationRigFBXUsesRealNodeNamesAndNoFakeFallback(t *testing.T) {
	repo := t.TempDir()
	fbx := "Kaydara FBX Binary  \x00Model::RootNode\x00Model::Pelvis\x00Model::Spine\x00"
	if err := os.WriteFile(filepath.Join(repo, "Hero.fbx"), []byte(fbx), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := GitService{}.LintAnimationRig(repo, "Hero.fbx", "MOBILE")
	if err != nil {
		t.Fatalf("lint fbx rig: %v", err)
	}
	if !res.AnalysisKnown || res.BoneCount != 3 {
		t.Fatalf("FBX Model:: names should populate the rig: %+v", res)
	}
	if len(res.Clips) != 0 || res.TotalDurationSeconds != 0 || res.OverallSampleRate != 0 {
		t.Fatalf("FBX clip timing is unknown and must not be fabricated: %+v", res)
	}

	unknown := filepath.Join(repo, "Unknown.fbx")
	if err := os.WriteFile(unknown, []byte("binary data without model names"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = GitService{}.LintAnimationRig(repo, "Unknown.fbx", "MOBILE")
	if err != nil {
		t.Fatalf("lint unknown fbx: %v", err)
	}
	if res.AnalysisKnown || res.BoneCount != 0 || len(res.Clips) != 0 {
		t.Fatalf("unknown FBX must not generate fallback bones or clips: %+v", res)
	}
	if res.Severity != "UNKNOWN" || res.PerformanceRating != "Unknown" || !strings.Contains(res.Message, "未解析") {
		t.Fatalf("unknown FBX should be reported honestly: %+v", res)
	}
	for _, bone := range res.Bones {
		if strings.HasPrefix(bone.Name, "Bone_") {
			t.Fatalf("fabricated fallback bone found: %+v", bone)
		}
	}
}
