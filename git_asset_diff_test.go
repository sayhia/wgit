package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeAssetPNG(t *testing.T, dir, rel string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 20), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGetAssetDiffReportsImageDimensions(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAssetPNG(t, dir, "Assets/Tex/Hero.png", 4, 3)
	runGitTest(t, dir, nil, "add", "-A")
	runGitTest(t, dir, nil, "commit", "-m", "art: add hero texture")

	// New revision: bigger canvas, different size on disk.
	writeAssetPNG(t, dir, "Assets/Tex/Hero.png", 8, 5)

	svc := GitService{}
	res, err := svc.GetAssetDiff(dir, "Assets/Tex/Hero.png", "HEAD", "WORKTREE")
	if err != nil {
		t.Fatalf("GetAssetDiff failed: %v", err)
	}
	if res.Kind != "image" {
		t.Fatalf("expected image kind, got %q", res.Kind)
	}
	if !res.Old.Exists || !res.New.Exists {
		t.Fatalf("both sides should exist: old=%+v new=%+v", res.Old, res.New)
	}
	if res.Old.Width != 4 || res.Old.Height != 3 {
		t.Fatalf("old dimensions wrong: %d×%d", res.Old.Width, res.Old.Height)
	}
	if res.New.Width != 8 || res.New.Height != 5 {
		t.Fatalf("new dimensions wrong: %d×%d", res.New.Width, res.New.Height)
	}
	if res.WidthDelta != 4 || res.HeightDelta != 2 {
		t.Fatalf("delta wrong: %d×%d", res.WidthDelta, res.HeightDelta)
	}
	if !res.Old.Previewable || !res.New.Previewable {
		t.Fatal("png should be previewable")
	}
	if res.Old.DataURL == "" || res.New.DataURL == "" {
		t.Fatal("expected data URLs for both sides")
	}
	if res.Old.Mime != "image/png" {
		t.Fatalf("unexpected mime: %q", res.Old.Mime)
	}
	if res.Message == "" {
		t.Fatal("expected a human summary")
	}
}

func TestGetAssetDiffHandlesAddedAsset(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "README.md", "hi\n", "chore: init", time.Now(), "Alice")

	writeAssetPNG(t, dir, "Assets/New.png", 2, 2)

	svc := GitService{}
	res, err := svc.GetAssetDiff(dir, "Assets/New.png", "HEAD", "WORKTREE")
	if err != nil {
		t.Fatalf("GetAssetDiff failed: %v", err)
	}
	if res.Old.Exists {
		t.Fatal("old side should not exist")
	}
	if !res.New.Exists {
		t.Fatal("new side should exist")
	}
	if res.SizeDelta != 0 {
		t.Fatalf("delta should stay 0 when one side is missing, got %d", res.SizeDelta)
	}
}

func TestGetAssetDiffRejectsMissingRefs(t *testing.T) {
	dir := initClosedLoopRepo(t)
	svc := GitService{}
	if _, err := svc.GetAssetDiff(dir, "a.png", "", "HEAD"); err == nil {
		t.Fatal("expected an error for an empty old ref")
	}
}

func TestAssetKindAndMime(t *testing.T) {
	kinds := map[string]string{
		"a.PSD":    "image",
		"a.uasset": "model",
		"a.wav":    "audio",
		"a.bin":    "binary",
	}
	for path, want := range kinds {
		if got := assetKindOf(path); got != want {
			t.Fatalf("assetKindOf(%q) = %q, want %q", path, got, want)
		}
	}
	if _, ok := assetPreviewMime("a.psd"); ok {
		t.Fatal("psd must not claim webview preview support")
	}
	if mime, ok := assetPreviewMime("a.JPG"); !ok || mime != "image/jpeg" {
		t.Fatalf("jpeg mime wrong: %q ok=%v", mime, ok)
	}
}

// --- LFS migration --------------------------------------------------------

func TestNormalizeLfsPatterns(t *testing.T) {
	joined, clean, err := normalizeLfsPatterns([]string{" *.psd ", "*.psd", "", "*.fbx"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if joined != "*.psd,*.fbx" {
		t.Fatalf("unexpected join: %q", joined)
	}
	if len(clean) != 2 {
		t.Fatalf("expected de-duplication, got %v", clean)
	}
	if _, _, err := normalizeLfsPatterns([]string{"  "}); err == nil {
		t.Fatal("expected an error for empty patterns")
	}
}

func TestPreviewLfsMigrationListsCandidates(t *testing.T) {
	dir := initClosedLoopRepo(t)
	requireGitLFS(t, dir)

	if err := os.MkdirAll(filepath.Join(dir, "Assets"), 0755); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 64*1024)
	if err := os.WriteFile(filepath.Join(dir, "Assets", "hero.psd"), payload, 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "-A")
	runGitTest(t, dir, nil, "commit", "-m", "art: hero")

	svc := GitService{}
	res, err := svc.PreviewLfsMigration(dir, []string{"*.psd"}, true, "")
	if err != nil {
		t.Fatalf("PreviewLfsMigration failed: %v", err)
	}
	if len(res.Rows) == 0 {
		t.Fatalf("expected at least one candidate row, raw=%q", res.Raw)
	}
	if res.Rows[0].Files < 1 {
		t.Fatalf("expected file count, got %+v", res.Rows[0])
	}
	if res.Command == "" {
		t.Fatal("expected the underlying command to be reported")
	}
}

func TestExecuteLfsMigrationRejectsEmptyPatterns(t *testing.T) {
	dir := initClosedLoopRepo(t)
	svc := GitService{}
	if _, err := svc.ExecuteLfsMigration(dir, LfsMigrateOptions{Everything: true}); err == nil {
		t.Fatal("expected an error when no pattern is given")
	}
}

func TestExecuteLfsMigrationRewritesHistoryWithBackup(t *testing.T) {
	dir := initClosedLoopRepo(t)
	requireGitLFS(t, dir)

	if err := os.MkdirAll(filepath.Join(dir, "Assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Assets", "hero.psd"), make([]byte, 64*1024), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "-A")
	runGitTest(t, dir, nil, "commit", "-m", "art: hero")
	before := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))

	svc := GitService{}
	res, err := svc.ExecuteLfsMigration(dir, LfsMigrateOptions{
		Patterns:     []string{"*.psd"},
		Everything:   true,
		CreateBundle: true,
	})
	if err != nil {
		t.Fatalf("ExecuteLfsMigration failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected a successful migration, got %+v", res)
	}
	if res.RewrittenFrom != before {
		t.Fatalf("expected the original HEAD %s, got %q", before, res.RewrittenFrom)
	}
	if res.RewrittenTo == "" || res.RewrittenTo == before {
		t.Fatalf("expected HEAD to be rewritten, got %+v", res)
	}
	if res.BundlePath == "" {
		t.Fatal("expected a rollback bundle")
	}
	if fi, err := os.Stat(res.BundlePath); err != nil || fi.Size() == 0 {
		t.Fatalf("rollback bundle is missing or empty: %v", err)
	}
	if files := runGitTest(t, dir, nil, "lfs", "ls-files"); !strings.Contains(files, "Assets/hero.psd") {
		t.Fatalf("expected the PSD to be tracked by LFS, got %q", files)
	}
	if head := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD")); head != res.RewrittenTo {
		t.Fatalf("reported HEAD %q does not match git HEAD %q", res.RewrittenTo, head)
	}
}

func TestLfsMigrationRequiresRefWhenEverythingDisabled(t *testing.T) {
	dir := initClosedLoopRepo(t)
	requireGitLFS(t, dir)
	svc := GitService{}

	if _, err := svc.PreviewLfsMigration(dir, []string{"*.psd"}, false, ""); err == nil || !strings.Contains(err.Error(), "必须指定") {
		t.Fatalf("an empty scope must be rejected instead of rewriting all history: %v", err)
	}
	if _, err := svc.ExecuteLfsMigration(dir, LfsMigrateOptions{Patterns: []string{"*.psd"}, Everything: false}); err == nil || !strings.Contains(err.Error(), "必须指定") {
		t.Fatalf("an empty scope must abort the rewrite: %v", err)
	}
}

func TestLfsMigrationRejectsRevisionRange(t *testing.T) {
	dir := initClosedLoopRepo(t)
	requireGitLFS(t, dir)

	if _, err := (GitService{}).PreviewLfsMigration(dir, []string{"*.psd"}, false, "main..feature"); err == nil || !strings.Contains(err.Error(), "--include-ref") {
		t.Fatalf("a revision range must be rejected with an explanation: %v", err)
	}
}

func TestLfsMigrationScopesToNamedRef(t *testing.T) {
	dir := initClosedLoopRepo(t)
	requireGitLFS(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "Assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Assets", "hero.psd"), make([]byte, 64*1024), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "-A")
	runGitTest(t, dir, nil, "commit", "-m", "art: hero")

	res, err := (GitService{}).PreviewLfsMigration(dir, []string{"*.psd"}, false, "main")
	if err != nil {
		t.Fatalf("PreviewLfsMigration failed for a valid ref: %v", err)
	}
	if len(res.Rows) == 0 {
		t.Fatalf("expected the named ref to report candidates, raw=%q", res.Raw)
	}
	if !strings.Contains(res.Command, "--include-ref=main") {
		t.Fatalf("expected --include-ref in the command, got %q", res.Command)
	}
}

func TestGetAssetDiffRejectsPathOutsideRepo(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAssetPNG(t, dir, "Assets/Tex/Hero.png", 4, 3)
	runGitTest(t, dir, nil, "add", "-A")
	runGitTest(t, dir, nil, "commit", "-m", "art: add hero texture")

	svc := GitService{}
	if _, err := svc.GetAssetDiff(dir, "../outside.png", "HEAD", "WORKTREE"); err == nil {
		t.Fatal("a parent-directory path must not be read from the working tree")
	}
	if _, err := svc.GetAssetDiff(dir, "/etc/hosts", "HEAD", "WORKTREE"); err == nil {
		t.Fatal("an absolute path must not be read from the working tree")
	}
}

func TestGetAssetDiffRejectsInvalidRevision(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAssetPNG(t, dir, "Assets/Tex/Hero.png", 4, 3)
	runGitTest(t, dir, nil, "add", "-A")
	runGitTest(t, dir, nil, "commit", "-m", "art: add hero texture")

	svc := GitService{}
	if _, err := svc.GetAssetDiff(dir, "Assets/Tex/Hero.png", "missing-ref", "WORKTREE"); err == nil {
		t.Fatal("invalid old revision must not be reported as an added asset")
	}
	if _, err := svc.GetAssetDiff(dir, "Assets/Tex/Hero.png", "HEAD", "missing-ref"); err == nil {
		t.Fatal("invalid new revision must not be reported as a deleted asset")
	}
}

func TestParseExtendedAssetMetadata(t *testing.T) {
	// 1. PSD
	psd := make([]byte, 26)
	copy(psd[0:4], "8BPS")
	psd[5] = 1 // version 1
	psd[12], psd[13] = 0, 4 // 4 channels
	psd[14], psd[15], psd[16], psd[17] = 0, 0, 4, 0x38 // height 1080
	psd[18], psd[19], psd[20], psd[21] = 0, 0, 7, 0x80 // width 1920
	psd[22], psd[23] = 0, 8 // 8 bit
	w, h, fmtStr := parseExtendedAssetMetadata(psd, ".psd")
	if w != 1920 || h != 1080 || !strings.Contains(fmtStr, "4-ch") || !strings.Contains(fmtStr, "8-bit") {
		t.Errorf("PSD parse mismatch: w=%d, h=%d, fmt=%s", w, h, fmtStr)
	}

	// 2. TGA
	tga := make([]byte, 18)
	tga[12] = 0x00
	tga[13] = 0x02 // width 512
	tga[14] = 0x00
	tga[15] = 0x01 // height 256
	tga[16] = 32   // 32-bit
	w, h, fmtStr = parseExtendedAssetMetadata(tga, ".tga")
	if w != 512 || h != 256 || fmtStr != "tga (32-bit)" {
		t.Errorf("TGA parse mismatch: w=%d, h=%d, fmt=%s", w, h, fmtStr)
	}

	// 3. GLB
	glb := make([]byte, 12)
	copy(glb[0:4], "glTF")
	glb[4] = 2 // version 2
	w, h, fmtStr = parseExtendedAssetMetadata(glb, ".glb")
	if w != 0 || h != 0 || fmtStr != "glb (v2)" {
		t.Errorf("GLB parse mismatch: fmt=%s", fmtStr)
	}

	// 4. FBX
	fbx := make([]byte, 27)
	copy(fbx[0:20], "Kaydara FBX Binary  ")
	fbx[23] = 0xe8
	fbx[24] = 0x1c // version 7400
	w, h, fmtStr = parseExtendedAssetMetadata(fbx, ".fbx")
	if w != 0 || h != 0 || fmtStr != "fbx (v7400)" {
		t.Errorf("FBX parse mismatch: fmt=%s", fmtStr)
	}

	// 5. Fallback
	w, h, fmtStr = parseExtendedAssetMetadata([]byte("data"), ".xyz")
	if w != 0 || h != 0 || fmtStr != "xyz" {
		t.Errorf("Fallback mismatch: fmt=%s", fmtStr)
	}
}
