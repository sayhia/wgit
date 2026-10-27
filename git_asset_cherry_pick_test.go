package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeCherryPickFixture(t *testing.T, repo, relPath, content string) {
	t.Helper()
	full := filepath.Join(repo, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("create dir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func cherryPickedByPath(res *AssetCherryPickResult) map[string]CherryPickedAsset {
	byPath := make(map[string]CherryPickedAsset, len(res.Assets))
	for _, asset := range res.Assets {
		byPath[asset.Path] = asset
	}
	return byPath
}

func TestCherryPickAssetBundleScansMetaInParallel(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")

	const (
		matAGuid = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		texBGuid = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)

	writeCherryPickFixture(t, repo, "Assets/MatA.mat", fmt.Sprintf(`%%YAML 1.1
--- !u!21 &2100000
Material:
  m_Name: MatA
  m_Texture: {fileID: 2800000, guid: %s, type: 3}
`, texBGuid))
	writeCherryPickFixture(t, repo, "Assets/MatA.mat.meta", fmt.Sprintf(`fileFormatVersion: 2
guid: %s
NativeFormatImporter:
  externalObjects: {}
`, matAGuid))

	writeCherryPickFixture(t, repo, "Assets/TexB.png", "tex-b-bytes\n")
	writeCherryPickFixture(t, repo, "Assets/TexB.png.meta", fmt.Sprintf(`fileFormatVersion: 2
guid: %s
TextureImporter:
  externalObjects: {}
`, texBGuid))

	noisePaths := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		noisePng := fmt.Sprintf("Assets/Noise%d.png", i)
		noisePaths = append(noisePaths, noisePng, noisePng+".meta")
		writeCherryPickFixture(t, repo, noisePng, fmt.Sprintf("noise-%d\n", i))
		writeCherryPickFixture(t, repo, noisePng+".meta", fmt.Sprintf(`fileFormatVersion: 2
guid: %032x
TextureImporter:
  externalObjects: {}
`, i+1))
	}

	runGitTest(t, repo, nil, "add", "-A")
	runGitTest(t, repo, nil, "commit", "-m", "chore: unity fixture")

	svc := GitService{}

	res, err := svc.CherryPickAssetBundle(repo, "HEAD", []string{"Assets/MatA.mat"}, true, true)
	if err != nil {
		t.Fatalf("resolveDependencies=true dry run failed: %v", err)
	}
	if !res.Success || !res.IsDryRun || res.TargetRef != "HEAD" {
		t.Fatalf("unexpected result metadata: %+v", res)
	}

	byPath := cherryPickedByPath(res)
	for _, want := range []string{"Assets/MatA.mat", "Assets/MatA.mat.meta", "Assets/TexB.png", "Assets/TexB.png.meta"} {
		if _, ok := byPath[want]; !ok {
			t.Fatalf("expected %s in cherry-picked bundle, got %v", want, byPath)
		}
	}
	if len(res.Assets) != 4 {
		t.Fatalf("expected exactly 4 assets, got %d: %+v", len(res.Assets), res.Assets)
	}
	if meta := byPath["Assets/MatA.mat.meta"]; !meta.IsDependency || meta.ParentAsset != "Assets/MatA.mat" {
		t.Fatalf("MatA meta should be a dependency of MatA, got %+v", meta)
	}
	if tex := byPath["Assets/TexB.png"]; !tex.IsDependency || tex.ParentAsset != "Assets/MatA.mat" {
		t.Fatalf("TexB should be a dependency of MatA, got %+v", tex)
	}
	if texMeta := byPath["Assets/TexB.png.meta"]; !texMeta.IsDependency || texMeta.ParentAsset != "Assets/TexB.png" {
		t.Fatalf("TexB meta should be a dependency of TexB, got %+v", texMeta)
	}
	if root := byPath["Assets/MatA.mat"]; root.IsDependency {
		t.Fatalf("explicitly requested MatA must not be marked as dependency: %+v", root)
	}
	for _, noise := range noisePaths {
		if _, ok := byPath[noise]; ok {
			t.Fatalf("unrelated asset %s must be excluded from the bundle", noise)
		}
	}

	resNoDeps, err := svc.CherryPickAssetBundle(repo, "HEAD", []string{"Assets/MatA.mat"}, false, true)
	if err != nil {
		t.Fatalf("resolveDependencies=false dry run failed: %v", err)
	}
	noDepsByPath := cherryPickedByPath(resNoDeps)
	for _, want := range []string{"Assets/MatA.mat", "Assets/MatA.mat.meta"} {
		if _, ok := noDepsByPath[want]; !ok {
			t.Fatalf("expected %s without dependency resolution, got %v", want, noDepsByPath)
		}
	}
	for _, unwanted := range []string{"Assets/TexB.png", "Assets/TexB.png.meta"} {
		if _, ok := noDepsByPath[unwanted]; ok {
			t.Fatalf("%s must be excluded when resolveDependencies=false", unwanted)
		}
	}
	if len(resNoDeps.Assets) != 2 {
		t.Fatalf("expected exactly 2 assets without dependency resolution, got %d: %+v", len(resNoDeps.Assets), resNoDeps.Assets)
	}
}
