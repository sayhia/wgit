package main

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testOBJBase = `v 0 0 0
v 1 0 0
v 0 1 0
v 1 1 0
f 1 2 3
f 2 4 3
`

const testOBJLod1 = `v 0 0 0
v 1 0 0
v 0 1 0
f 1 2 3
`

func TestAuditLodStreamingUsesRealMeshAndTextureHeaders(t *testing.T) {
	repo := t.TempDir()
	meshDir := filepath.Join(repo, "Meshes")
	texDir := filepath.Join(repo, "Textures")
	if err := os.MkdirAll(meshDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(texDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(meshDir, "Tri.obj"), []byte(testOBJBase), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(meshDir, "Tri_LOD1.obj"), []byte(testOBJLod1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(meshDir, "Unknown.fbx"), []byte("not a parseable fbx"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTextureTestPNG(t, filepath.Join(texDir, "Valid.png"), fillNRGBA(4, 4, color.NRGBA{R: 1, G: 2, B: 3, A: 4}))
	if err := os.WriteFile(filepath.Join(texDir, "Broken.png"), []byte("not a png"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.AuditLodStreaming(repo, "")
	if err != nil {
		t.Fatalf("audit lod streaming: %v", err)
	}
	if res.TotalMeshesAudited != 2 || res.TotalTexturesAudited != 2 {
		t.Fatalf("unexpected audit counts: %+v", res)
	}
	if res.LODUnknownCount != 1 || res.StreamingUnknownCount != 1 {
		t.Fatalf("expected unknown mesh/texture counts, got lod=%d stream=%d", res.LODUnknownCount, res.StreamingUnknownCount)
	}
	if !strings.Contains(res.Summary, "1 项面数未知") || !strings.Contains(res.Summary, "1 项尺寸未知") {
		t.Fatalf("summary should expose unknown counts: %q", res.Summary)
	}

	meshes := make(map[string]MeshLodAuditItem, len(res.Meshes))
	for _, mesh := range res.Meshes {
		meshes[filepath.ToSlash(mesh.Path)] = mesh
	}
	obj := meshes["Meshes/Tri.obj"]
	if !obj.TriangleCountKnown || obj.BaseTriangles != 2 {
		t.Fatalf("OBJ base triangles should be parsed from faces: %+v", obj)
	}
	if len(obj.LodTiers) != 2 || !obj.LodTiers[1].TriangleCountKnown || obj.LodTiers[1].TriangleCount != 1 {
		t.Fatalf("LOD1 triangles should be parsed from the real OBJ: %+v", obj.LodTiers)
	}
	if obj.LodTiers[1].ReductionPercent != 50 {
		t.Fatalf("expected 50%% of base triangles at LOD1, got %v", obj.LodTiers[1].ReductionPercent)
	}
	unknown := meshes["Meshes/Unknown.fbx"]
	if unknown.TriangleCountKnown || unknown.BaseTriangles != 0 || unknown.Severity != "WARNING" {
		t.Fatalf("unsupported mesh format must be marked unknown, not estimated: %+v", unknown)
	}
	if !strings.Contains(unknown.Recommendation, "无法直接读取三角形数") {
		t.Fatalf("unknown mesh advice should explain the limitation: %q", unknown.Recommendation)
	}

	textures := make(map[string]TextureStreamingAuditItem, len(res.Textures))
	for _, texture := range res.Textures {
		textures[filepath.ToSlash(texture.Path)] = texture
	}
	valid := textures["Textures/Valid.png"]
	if !valid.DimensionsKnown || valid.Width != 4 || valid.Height != 4 || !valid.IsPOT || valid.MipCount != 3 {
		t.Fatalf("valid PNG should use real dimensions and mip chain: %+v", valid)
	}
	broken := textures["Textures/Broken.png"]
	if broken.DimensionsKnown || broken.RawSizeBytes != 0 || !strings.Contains(broken.Issue, "无法解析纹理头") {
		t.Fatalf("unreadable texture must not fall back to 1024x1024: %+v", broken)
	}
}

func TestCountMeshTrianglesReadsGLTFAndGLB(t *testing.T) {
	dir := t.TempDir()
	gltf := `{"meshes":[{"primitives":[{"indices":0,"mode":4}]}],"accessors":[{"count":9}]}`
	gltfPath := filepath.Join(dir, "mesh.gltf")
	if err := os.WriteFile(gltfPath, []byte(gltf), 0o644); err != nil {
		t.Fatal(err)
	}
	if triangles, ok := countMeshFileTriangles(gltfPath, ".gltf"); !ok || triangles != 3 {
		t.Fatalf("glTF triangle count: got %d known=%v", triangles, ok)
	}

	jsonChunk := []byte(gltf)
	if pad := (4 - len(jsonChunk)%4) % 4; pad > 0 {
		jsonChunk = append(jsonChunk, bytes.Repeat([]byte(" "), pad)...)
	}
	glb := make([]byte, 20+len(jsonChunk))
	copy(glb[0:4], "glTF")
	binary.LittleEndian.PutUint32(glb[4:8], 2)
	binary.LittleEndian.PutUint32(glb[8:12], uint32(len(glb)))
	binary.LittleEndian.PutUint32(glb[12:16], uint32(len(jsonChunk)))
	binary.LittleEndian.PutUint32(glb[16:20], 0x4E4F534A)
	copy(glb[20:], jsonChunk)
	glbPath := filepath.Join(dir, "mesh.glb")
	if err := os.WriteFile(glbPath, glb, 0o644); err != nil {
		t.Fatal(err)
	}
	if triangles, ok := countMeshFileTriangles(glbPath, ".glb"); !ok || triangles != 3 {
		t.Fatalf("GLB triangle count: got %d known=%v", triangles, ok)
	}

	if _, ok := countMeshFileTriangles(filepath.Join(dir, "mesh.fbx"), ".fbx"); ok {
		t.Fatal("FBX triangle count must be reported as unknown")
	}
}

func TestAuditNativePluginsAbiEmptyRepoIsHonest(t *testing.T) {
	res, err := (GitService{}).AuditNativePluginsAbi(t.TempDir())
	if err != nil {
		t.Fatalf("audit native plugins: %v", err)
	}
	if res.TotalPluginsCount != 0 {
		t.Fatalf("expected no plugins, got %d", res.TotalPluginsCount)
	}
	if !strings.Contains(res.Message, "未作 ABI 兼容结论") {
		t.Fatalf("empty audit message must not claim compatibility: %q", res.Message)
	}
	if len(res.SummaryRecommendations) == 0 || !strings.Contains(strings.Join(res.SummaryRecommendations, " "), "未发现原生插件") {
		t.Fatalf("empty audit recommendation must be explicit: %+v", res.SummaryRecommendations)
	}
}

func TestAuditLodStreamingEmptyRepoIsHonest(t *testing.T) {
	res, err := GitService{}.AuditLodStreaming(t.TempDir(), "")
	if err != nil {
		t.Fatalf("audit empty lod repo: %v", err)
	}
	if res.AnalysisKnown || res.OverallHealthScore != 0 || res.TotalMeshesAudited != 0 || res.TotalTexturesAudited != 0 {
		t.Fatalf("empty LOD audit must not score 100: %+v", res)
	}
	if res.Meshes == nil || res.Textures == nil || res.Recommendations == nil {
		t.Fatalf("empty LOD lists must serialize as arrays: %+v", res)
	}
	if !strings.Contains(res.Summary, "未发现可审计") {
		t.Fatalf("empty LOD summary should be explicit: %q", res.Summary)
	}

	if _, err := (GitService{}).AuditLodStreaming(t.TempDir(), "Missing"); err == nil {
		t.Fatal("missing scan directory must return an error")
	}
}
