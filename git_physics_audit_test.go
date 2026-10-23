package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditPhysicsMeshCollisionParsesUnityDocuments(t *testing.T) {
	repo := t.TempDir()
	prefab := `%YAML 1.1
--- !u!64 &1
MeshCollider:
  m_Convex: 1
  m_Material: {fileID: 0}
--- !u!64 &2
MeshCollider:
  m_Convex: 0
  m_Material: {fileID: 11400000}
--- !u!54 &3
Rigidbody:
  m_IsKinematic: 0
`
	if err := os.WriteFile(filepath.Join(repo, "Assets_Level.prefab"), []byte(prefab), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Map.umap"), []byte("MeshCollider binary"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.AuditPhysicsMeshCollision(repo, "")
	if err != nil {
		t.Fatalf("audit physics collision: %v", err)
	}
	if !res.AnalysisKnown || res.TotalColliders != 2 || res.ConvexHullsCount != 1 || res.ConcaveMeshCount != 1 {
		t.Fatalf("unexpected collider counts: %+v", res)
	}
	if len(res.NonConvexDynamicColliders) != 1 || len(res.MissingPhysicMaterials) != 1 {
		t.Fatalf("expected dynamic concave and missing material issues: %+v", res)
	}
	if res.HealthScore != 65 {
		t.Fatalf("expected score 65 for one dynamic concave and one missing material, got %d", res.HealthScore)
	}
	if !strings.Contains(res.Summary, "跳过 1 个 UE .umap") {
		t.Fatalf("summary should mention skipped UE assets: %q", res.Summary)
	}
}

func TestAuditPhysicsMeshCollisionEmptyRepoIsHonest(t *testing.T) {
	res, err := GitService{}.AuditPhysicsMeshCollision(t.TempDir(), "")
	if err != nil {
		t.Fatalf("audit empty physics repo: %v", err)
	}
	if res.AnalysisKnown || res.TotalColliders != 0 || res.ConvexHullsCount != 0 || res.ConcaveMeshCount != 0 {
		t.Fatalf("empty repo must not fabricate 4/3/1 colliders: %+v", res)
	}
	if res.HealthScore != 0 || !strings.Contains(res.Summary, "未发现") {
		t.Fatalf("empty repo should not be scored as healthy: %+v", res)
	}
}
