package main

import (
	"testing"
)

func TestUnityAndGodotSceneParsers(t *testing.T) {
	unityScene := `%YAML 1.1
--- !u!1 &100
GameObject:
  m_Name: Player
  m_LocalPosition: {x: -1.5, y: 2.25, z: 0}
--- !u!4 &101
Transform:
  m_GameObject: {fileID: 100}
  m_LocalPosition: {x: -1.5, y: 2.25, z: 0}
--- !u!1 &200
GameObject:
  m_Name: Enemy
  m_LocalPosition: {x: 10, y: 0, z: -5}
--- !u!4 &201
Transform:
  m_GameObject: {fileID: 200}
  m_LocalPosition: {x: 10, y: 0, z: -5}
`
	prefabNodes := parseUnityPrefabAST(unityScene)
	if len(prefabNodes) != 2 {
		t.Fatalf("expected 2 prefab nodes, got %+v", prefabNodes)
	}
	if prefabNodes[0].Name != "Player" || prefabNodes[0].ID != "go_100" || prefabNodes[0].Class != "GameObject" {
		t.Fatalf("unexpected first prefab node: %+v", prefabNodes[0])
	}
	if prefabNodes[1].Name != "Enemy" || prefabNodes[1].PosX != 240 || prefabNodes[1].PosY != 100 {
		t.Fatalf("unexpected second prefab node: %+v", prefabNodes[1])
	}
	if len(prefabNodes[0].Pins) != 1 || prefabNodes[0].Pins[0].Name != "Transform" {
		t.Fatalf("expected transform pin: %+v", prefabNodes[0].Pins)
	}

	actors := parseUnitySceneActors("Assets/Scenes/Main.unity", unityScene, "ours")
	if len(actors) != 2 {
		t.Fatalf("expected 2 Unity actors, got %+v", actors)
	}
	if actors[0].Name != "Player" || actors[0].Position.X != -1.5 || actors[0].Position.Y != 2.25 || actors[0].Position.Z != 0 {
		t.Fatalf("unexpected Unity actor position: %+v", actors[0])
	}
	if actors[0].Bounds.Min.X != -1.5 || actors[0].Bounds.Max.Y != 2.25 {
		t.Fatalf("Unity actor bounds should not be fabricated: %+v", actors[0].Bounds)
	}
	if actors[0].SourceSide != "ours" || actors[0].SubLevel != "Assets/Scenes/Main.unity" || actors[0].Guid != "100" {
		t.Fatalf("unexpected Unity actor metadata: %+v", actors[0])
	}

	godotScene := `[gd_scene load_steps=2 format=3]

[node name="Main" type="Node3D"]

[node name="Player" type="CharacterBody3D" parent="."]
transform = Transform3D(1, 0, 0, 0, 1, 0, 0, 0, 1, 4, 5, 6)
`
	godotNodes := parseGodotSceneAST(godotScene)
	if len(godotNodes) != 2 || godotNodes[0].Name != "Main" || godotNodes[0].Class != "Node3D" || godotNodes[1].Name != "Player" {
		t.Fatalf("unexpected Godot AST nodes: %+v", godotNodes)
	}
	godotActors := parseGodotSceneActors("scenes/main.tscn", godotScene, "theirs")
	if len(godotActors) != 2 {
		t.Fatalf("expected 2 Godot actors, got %+v", godotActors)
	}
	if godotActors[1].Name != "Player" || godotActors[1].Position.X != 4 || godotActors[1].Position.Y != 5 || godotActors[1].Guid != "name:Player" || godotActors[1].SourceSide != "theirs" {
		t.Fatalf("unexpected Godot actor: %+v", godotActors[1])
	}

	vec := parseVectorLine("position = Vector3(X=1.5, Y=-2, Z=3.25)")
	if vec.X != 1.5 || vec.Y != -2 || vec.Z != 3.25 {
		t.Fatalf("parseVectorLine failed: %+v", vec)
	}
	unityVec := parseUnityVector("m_LocalPosition: {x: -1, y: 2.5, z: .5}")
	if unityVec.X != -1 || unityVec.Y != 2.5 || unityVec.Z != 0.5 {
		t.Fatalf("parseUnityVector failed: %+v", unityVec)
	}
	if zero := parseUnityVector("no vector here"); zero != (Vector3D{}) {
		t.Fatalf("invalid vector should be zero: %+v", zero)
	}

	box := computeAABB(Vector3D{X: 10, Y: 20, Z: 30}, Vector3D{X: 1, Y: 2, Z: 3})
	if box.Min != (Vector3D{X: 9, Y: 18, Z: 27}) || box.Max != (Vector3D{X: 11, Y: 22, Z: 33}) {
		t.Fatalf("computeAABB failed: %+v", box)
	}
	if volume := computeBoxVolume(box); volume != 48 {
		t.Fatalf("computeBoxVolume = %v, want 48", volume)
	}
	overlapBox := computeAABB(Vector3D{X: 12, Y: 20, Z: 30}, Vector3D{X: 2, Y: 2, Z: 2})
	if ok, volume := testAABBOverlap(box, overlapBox); !ok || volume <= 0 {
		t.Fatalf("expected overlapping boxes, ok=%v volume=%v", ok, volume)
	}
	farBox := computeAABB(Vector3D{X: 100, Y: 100, Z: 100}, Vector3D{X: 1, Y: 1, Z: 1})
	if ok, volume := testAABBOverlap(box, farBox); ok || volume != 0 {
		t.Fatalf("expected non-overlapping boxes, ok=%v volume=%v", ok, volume)
	}
}
