package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const unitySceneFixture = `%YAML 1.1
--- !u!1 &100
GameObject:
  m_Name: Player
--- !u!4 &200
Transform:
  m_GameObject: {fileID: 100}
  m_LocalPosition: {x: 3, y: 4, z: 5}
--- !u!1 &101
GameObject:
  m_Name: NoTransform
`

func TestParseSceneStreamActorsUsesRealTransforms(t *testing.T) {
	unityActors := parseUnitySceneActors("Main.unity", unitySceneFixture, "CURRENT")
	if len(unityActors) != 1 {
		t.Fatalf("expected only the GameObject with a Transform, got %+v", unityActors)
	}
	actor := unityActors[0]
	if actor.Guid != "100" || actor.Name != "Player" || actor.Position.X != 3 || actor.Position.Y != 4 || actor.Position.Z != 5 {
		t.Fatalf("unexpected Unity actor: %+v", actor)
	}
	if actor.Bounds.Extents.X != 0 || actor.Bounds.Extents.Y != 0 || actor.Bounds.Extents.Z != 0 {
		t.Fatalf("unknown bounds must not be fabricated: %+v", actor.Bounds)
	}

	godot := `[gd_scene load_steps=2 format=3]
[node name="Root" type="Node3D"]
[node name="Cube" type="MeshInstance3D" parent="."]
transform = Transform3D(1, 0, 0, 0, 1, 0, 0, 0, 1, 7, 8, 9)
`
	godotActors := parseGodotSceneActors("Main.tscn", godot, "CURRENT")
	if len(godotActors) != 2 {
		t.Fatalf("expected two Godot nodes, got %+v", godotActors)
	}
	if godotActors[0].Guid != "name:Root" || godotActors[1].Guid != "name:Cube" {
		t.Fatalf("Godot node keys must be real paths, got %+v", godotActors)
	}
	if godotActors[1].Position.X != 7 || godotActors[1].Position.Y != 8 || godotActors[1].Position.Z != 9 {
		t.Fatalf("Godot transform should be parsed from Transform3D: %+v", godotActors[1])
	}

	unreal := `Begin Actor
Class=StaticMeshActor
Name=Rock
ActorGuid=abc123
Location=X=1.0, Y=2.0, Z=3.0
End Actor
Begin Actor
End Actor
`
	unrealActors := parseUnrealLevelActors("Main.umap", unreal, "CURRENT")
	if len(unrealActors) != 1 {
		t.Fatalf("actors without a name or GUID must be skipped: %+v", unrealActors)
	}
	if unrealActors[0].Guid != "abc123" || unrealActors[0].Position.X != 1 || unrealActors[0].Position.Z != 3 {
		t.Fatalf("unexpected Unreal actor: %+v", unrealActors[0])
	}
}

func TestAuditSceneStreamEmptyRepoIsUnknown(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(repo, "readme.txt"), []byte("no scenes"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "base")

	res, err := GitService{}.AuditSceneStreamMerge(repo, "main", "")
	if err != nil {
		t.Fatalf("audit empty scenes: %v", err)
	}
	if res.AnalysisKnown || res.SafetyScore != 0 || res.SafetyLevel != "UNKNOWN" || res.MergeVerdict != "NO_DATA" {
		t.Fatalf("empty scene audit must not be SAFE: %+v", res)
	}
	if !strings.Contains(res.SummaryMessage, "未作合流安全结论") {
		t.Fatalf("empty scene summary should be explicit: %q", res.SummaryMessage)
	}
	if res.AuditedSubLevels == nil {
		t.Fatal("audited sub-levels must serialize as an array, not null")
	}
}

func TestAuditSceneStreamUnreadableSceneIsNotSafe(t *testing.T) {
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
	scene := filepath.Join(repo, "Main.unity")
	if err := os.WriteFile(scene, []byte(unitySceneFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "base scene")

	run("checkout", "-q", "-b", "feature")
	updated := strings.Replace(unitySceneFixture, "{x: 3, y: 4, z: 5}", "{x: 8, y: 4, z: 5}", 1)
	if err := os.WriteFile(scene, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "move player")

	// The tracked scene exists but is unreadable (a directory at its path);
	// git trees are intact, so the target branch still parses fine.
	if err := os.Remove(scene); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, "scene-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repo, "scene-dir"), scene); err != nil {
		t.Fatal(err)
	}

	res, err := (GitService{}).AuditSceneStreamMerge(repo, "main", "")
	if err != nil {
		t.Fatalf("audit should return a partial result rather than fail: %v", err)
	}
	if res.SafetyLevel == "SAFE" || res.MergeVerdict == "SAFE_AUTO_MERGE" {
		t.Fatalf("unreadable scene must not produce a safe verdict: %+v", res)
	}
	if res.SafetyLevel != "WARNING" || res.MergeVerdict != "MANUAL_REVIEW_REQUIRED" {
		t.Fatalf("unreadable scene should require manual review: %+v", res)
	}
	if res.SafetyScore > 84 {
		t.Fatalf("partial audit must not keep a perfect score: %+v", res)
	}
	if !strings.Contains(res.SummaryMessage, "不可读") || !strings.Contains(res.SummaryMessage, "人工复核") {
		t.Fatalf("summary must explain the incomplete audit: %q", res.SummaryMessage)
	}
}

func TestResolveSceneStreamConflictOffsetsOnlyMatchingUnityTransform(t *testing.T) {
	repo := t.TempDir()
	scenePath := filepath.Join(repo, "Main.unity")
	content := `%YAML 1.1
--- !u!1 &100
GameObject:
  m_Name: Player
--- !u!4 &200
Transform:
  m_GameObject: {fileID: 100}
  m_LocalPosition: {x: 3, y: 4, z: 5}
--- !u!4 &201
Transform:
  m_GameObject: {fileID: 101}
  m_LocalPosition: {x: 1, y: 1, z: 1}
`
	if err := os.WriteFile(scenePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.ResolveSceneStreamConflict(repo, "Main.unity", "SPATIAL_OFFSET", "100")
	if err != nil {
		t.Fatalf("spatial offset: %v", err)
	}
	if !res.Success || res.OffsetApplied == nil || res.OffsetApplied.X != 150 || res.OffsetApplied.Z != 50 {
		t.Fatalf("unexpected offset result: %+v", res)
	}
	updated, err := os.ReadFile(scenePath)
	if err != nil {
		t.Fatal(err)
	}
	updatedText := string(updated)
	if !strings.Contains(updatedText, "m_LocalPosition: {x: 153, y: 4, z: 55}") {
		t.Fatalf("matching transform was not moved: %s", updatedText)
	}
	if !strings.Contains(updatedText, "m_LocalPosition: {x: 1, y: 1, z: 1}") {
		t.Fatalf("other transforms must stay untouched: %s", updatedText)
	}

	before, _ := os.ReadFile(scenePath)
	if _, err := (GitService{}).ResolveSceneStreamConflict(repo, "Main.unity", "SPATIAL_OFFSET", "999"); err == nil {
		t.Fatal("missing actor must fail without writing")
	}
	after, _ := os.ReadFile(scenePath)
	if string(before) != string(after) {
		t.Fatal("failed resolution must not modify the scene")
	}

	if _, err := (GitService{}).ResolveSceneStreamConflict(repo, "Main.unity", "AUTO_REGENERATE_GUID", "not-present"); err == nil {
		t.Fatal("missing GUID token must fail instead of claiming success")
	}
	regen, err := GitService{}.ResolveSceneStreamConflict(repo, "Main.unity", "AUTO_REGENERATE_GUID", "100")
	if err != nil {
		t.Fatalf("regenerate GUID: %v", err)
	}
	if regen.NewActorGuid == "" || len(regen.NewActorGuid) != 32 {
		t.Fatalf("expected a real 32-hex GUID, got %+v", regen)
	}
}

func TestAuditSceneStreamRejectsUnrelatedHistories(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "main")

	run("checkout", "-q", "--orphan", "other")
	run("rm", "-rf", ".")
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "other")
	run("checkout", "-q", "main")

	if _, err := (GitService{}).AuditSceneStreamMerge(repo, "other", ""); err == nil || !strings.Contains(err.Error(), "读取关卡差异失败") {
		t.Fatalf("unrelated histories must surface a diff error, got %v", err)
	}
}

func TestAuditSceneStreamRejectsInvalidBranchAndMissingScene(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(repo, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "base")

	if _, err := (GitService{}).AuditSceneStreamMerge(repo, "missing-branch", ""); err == nil {
		t.Fatal("invalid target branch must return an error")
	}
	if _, err := (GitService{}).AuditSceneStreamMerge(repo, "main", "Assets/Missing.unity"); err == nil {
		t.Fatal("missing scene path must return an error")
	}
}
