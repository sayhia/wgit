package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestGLB(t *testing.T, path string, jsonText string) {
	t.Helper()
	chunk := []byte(jsonText)
	if pad := (4 - len(chunk)%4) % 4; pad > 0 {
		chunk = append(chunk, bytes.Repeat([]byte(" "), pad)...)
	}
	data := make([]byte, 20+len(chunk))
	copy(data[0:4], "glTF")
	binary.LittleEndian.PutUint32(data[4:8], 2)
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(data)))
	binary.LittleEndian.PutUint32(data[12:16], uint32(len(chunk)))
	binary.LittleEndian.PutUint32(data[16:20], 0x4E4F534A)
	copy(data[20:], chunk)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write glb: %v", err)
	}
}

func TestLintModelMeshUsesRealGeometry(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "Sword.obj"), []byte(testOBJBase), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.LintModelMesh(repo, "Sword.obj", "mobile_prop")
	if err != nil {
		t.Fatalf("lint obj: %v", err)
	}
	if !res.TriangleCountKnown || res.TriangleCount != 2 || res.VertexCount != 4 {
		t.Fatalf("unexpected OBJ geometry: %+v", res)
	}
	if res.IsOverBudget || res.Severity != "OK" {
		t.Fatalf("small OBJ should be within budget: %+v", res)
	}

	gltf := `{"meshes":[{"primitives":[{"indices":1,"material":0,"attributes":{"POSITION":0}}]}],"accessors":[{"count":9},{"count":9}]}`
	if err := os.WriteFile(filepath.Join(repo, "Hero.gltf"), []byte(gltf), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = GitService{}.LintModelMesh(repo, "Hero.gltf", "mobile_prop")
	if err != nil {
		t.Fatalf("lint gltf: %v", err)
	}
	if !res.TriangleCountKnown || res.TriangleCount != 3 || res.VertexCount != 9 || res.SubmeshCount != 1 || res.MaterialCount != 1 {
		t.Fatalf("unexpected glTF geometry: %+v", res)
	}

	writeTestGLB(t, filepath.Join(repo, "Hero.glb"), gltf)
	res, err = GitService{}.LintModelMesh(repo, "Hero.glb", "mobile_prop")
	if err != nil {
		t.Fatalf("lint glb: %v", err)
	}
	if !res.TriangleCountKnown || res.TriangleCount != 3 || res.VertexCount != 9 {
		t.Fatalf("unexpected GLB geometry: %+v", res)
	}

	overBudget := `{"meshes":[{"primitives":[{"indices":1,"attributes":{"POSITION":0}}]}],"accessors":[{"count":9000},{"count":18000}]}`
	if err := os.WriteFile(filepath.Join(repo, "Heavy.gltf"), []byte(overBudget), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = GitService{}.LintModelMesh(repo, "Heavy.gltf", "mobile_prop")
	if err != nil {
		t.Fatalf("lint heavy gltf: %v", err)
	}
	if res.TriangleCount != 6000 || !res.IsOverBudget || res.Severity != "ERROR" {
		t.Fatalf("real triangle count should exceed mobile prop budget: %+v", res)
	}

	malformed := `{"meshes":[{"primitives":[{"indices":5,"attributes":{"POSITION":6}}]}],"accessors":[]}`
	if err := os.WriteFile(filepath.Join(repo, "Broken.gltf"), []byte(malformed), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = GitService{}.LintModelMesh(repo, "Broken.gltf", "mobile_prop")
	if err != nil {
		t.Fatalf("lint malformed gltf: %v", err)
	}
	if res.TriangleCountKnown || res.Severity != "UNKNOWN" || res.IsOverBudget {
		t.Fatalf("invalid accessor references must not be reported as zero triangles/OK: %+v", res)
	}

	if err := os.WriteFile(filepath.Join(repo, "Rig.fbx"), []byte("not a parseable fbx"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = GitService{}.LintModelMesh(repo, "Rig.fbx", "mobile_prop")
	if err != nil {
		t.Fatalf("lint fbx: %v", err)
	}
	if res.TriangleCountKnown || res.TriangleCount != 0 || res.VertexCount != 0 {
		t.Fatalf("unsupported FBX must not be estimated by file size: %+v", res)
	}
	if res.Severity != "UNKNOWN" || !strings.Contains(res.Advice, "无法直接解析") {
		t.Fatalf("unsupported format should be reported as unknown: %+v", res)
	}
}
