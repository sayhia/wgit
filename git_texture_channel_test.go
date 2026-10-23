package main

import (
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTextureTestPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

func fillNRGBA(width, height int, c color.NRGBA) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func fillGray(width, height int, c color.Gray) image.Image {
	img := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetGray(x, y, c)
		}
	}
	return img
}

func TestAuditTextureChannelPackingUsesRealImageHeaders(t *testing.T) {
	repo := t.TempDir()
	writeTextureTestPNG(t, filepath.Join(repo, "Assets/Textures/T_Rock_ORM.png"), fillNRGBA(4, 4, color.NRGBA{R: 1, G: 2, B: 3, A: 4}))
	writeTextureTestPNG(t, filepath.Join(repo, "Assets/Textures/T_Rock_Roughness.png"), fillNRGBA(4, 4, color.NRGBA{R: 8, G: 9, B: 10, A: 11}))
	writeTextureTestPNG(t, filepath.Join(repo, "Assets/Textures/T_Rock_AO.png"), fillGray(4, 4, color.Gray{Y: 12}))
	writeTextureTestPNG(t, filepath.Join(repo, "Assets/Textures/T_Rock_Normal.png"), fillGray(4, 4, color.Gray{Y: 13}))
	writeTextureTestPNG(t, filepath.Join(repo, "Assets/UI/Icon_Close.png"), fillNRGBA(2, 2, color.NRGBA{R: 1, G: 1, B: 1, A: 1}))
	writeTextureTestPNG(t, filepath.Join(repo, "Library/Ignored_ORM.png"), fillNRGBA(4, 4, color.NRGBA{R: 1, G: 1, B: 1, A: 1}))

	result, err := GitService{}.AuditTextureChannelPacking(repo, "")
	if err != nil {
		t.Fatalf("audit texture packing: %v", err)
	}

	if result.TotalTexturesScanned != 5 {
		t.Fatalf("expected 5 scanned textures (Library excluded), got %d", result.TotalTexturesScanned)
	}
	if result.OrmCompliantCount != 1 {
		t.Fatalf("expected 1 compliant ORM texture, got %d", result.OrmCompliantCount)
	}
	if result.UnpackedTexturesCount != 2 {
		t.Fatalf("expected 2 split-channel textures, got %d", result.UnpackedTexturesCount)
	}
	if result.InvalidNormalMapsCount != 1 {
		t.Fatalf("expected 1 invalid normal map, got %d", result.InvalidNormalMapsCount)
	}
	if result.VramWasteEstimateBytes != 64 {
		t.Fatalf("expected 64 bytes of channel waste (16 ORM alpha + 48 roughness RGB), got %d", result.VramWasteEstimateBytes)
	}
	if len(result.Textures) != 4 {
		t.Fatalf("expected 4 PBR-related entries, got %d", len(result.Textures))
	}
	if !strings.Contains(result.Summary, "扫描 5 张图片") {
		t.Fatalf("summary should report real scan count, got %q", result.Summary)
	}

	byPath := make(map[string]TextureChannelAuditItem, len(result.Textures))
	for _, item := range result.Textures {
		byPath[item.Path] = item
	}
	orm := byPath["Assets/Textures/T_Rock_ORM.png"]
	if !orm.IsCompliant || !strings.Contains(orm.PackingType, "ORM") {
		t.Fatalf("ORM texture should be compliant: %+v", orm)
	}
	if !strings.Contains(orm.Issue, "Alpha") {
		t.Fatalf("ORM RGBA should report unused alpha, got %q", orm.Issue)
	}
	normal := byPath["Assets/Textures/T_Rock_Normal.png"]
	if normal.IsCompliant {
		t.Fatalf("grayscale normal map should be non-compliant: %+v", normal)
	}
	roughness := byPath["Assets/Textures/T_Rock_Roughness.png"]
	if roughness.PackingType != "SplitChannels" || roughness.IsCompliant {
		t.Fatalf("split roughness should be flagged as unpacked: %+v", roughness)
	}
}

func TestAuditTextureChannelPackingScopesTargetDir(t *testing.T) {
	repo := t.TempDir()
	writeTextureTestPNG(t, filepath.Join(repo, "Assets/Textures/T_A_ORM.png"), fillNRGBA(2, 2, color.NRGBA{R: 1, G: 2, B: 3, A: 255}))
	writeTextureTestPNG(t, filepath.Join(repo, "Other/T_B_ORM.png"), fillNRGBA(2, 2, color.NRGBA{R: 1, G: 2, B: 3, A: 255}))

	result, err := GitService{}.AuditTextureChannelPacking(repo, "Assets")
	if err != nil {
		t.Fatalf("audit texture packing: %v", err)
	}
	if result.TotalTexturesScanned != 1 || len(result.Textures) != 1 {
		t.Fatalf("targetDir should scope the scan to Assets, got scanned=%d entries=%d", result.TotalTexturesScanned, len(result.Textures))
	}
	if result.Textures[0].Path != "Assets/Textures/T_A_ORM.png" {
		t.Fatalf("unexpected scoped path: %s", result.Textures[0].Path)
	}
}

func TestInspectTextureFileReadsJPEGHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "photo.jpg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create jpeg: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		f.Close()
		t.Fatalf("encode jpeg: %v", err)
	}
	f.Close()

	info, err := inspectTextureFile(path)
	if err != nil {
		t.Fatalf("inspect jpeg: %v", err)
	}
	if info.width != 3 || info.height != 2 || info.channels != 3 || info.colorType != "RGB" {
		t.Fatalf("unexpected JPEG header info: %+v", info)
	}
}

func TestInspectTextureFileReadsTGAHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "banner.tga")
	data := make([]byte, 18+18)
	data[2] = 2 // uncompressed true-color
	binary.LittleEndian.PutUint16(data[12:14], 3)
	binary.LittleEndian.PutUint16(data[14:16], 2)
	data[16] = 24
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write tga: %v", err)
	}

	info, err := inspectTextureFile(path)
	if err != nil {
		t.Fatalf("inspect tga: %v", err)
	}
	if info.width != 3 || info.height != 2 || info.channels != 3 || info.colorType != "TGA/24bit" {
		t.Fatalf("unexpected TGA header info: %+v", info)
	}
}

func TestInspectTextureFileDetectsPNGSRGBChunk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "T_Data_Normal.png")
	writeTextureTestPNG(t, path, fillNRGBA(2, 2, color.NRGBA{R: 128, G: 128, B: 255, A: 255}))

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png: %v", err)
	}
	if len(data) < 33 {
		t.Fatalf("unexpected png length %d", len(data))
	}
	chunk := make([]byte, 13)
	binary.BigEndian.PutUint32(chunk[0:4], 1)
	copy(chunk[4:8], "sRGB")
	chunk[8] = 0
	crc := crc32.NewIEEE()
	crc.Write(chunk[4:9])
	binary.BigEndian.PutUint32(chunk[9:13], crc.Sum32())

	patched := make([]byte, 0, len(data)+len(chunk))
	patched = append(patched, data[:33]...)
	patched = append(patched, chunk...)
	patched = append(patched, data[33:]...)
	if err := os.WriteFile(path, patched, 0o644); err != nil {
		t.Fatalf("rewrite png: %v", err)
	}

	info, err := inspectTextureFile(path)
	if err != nil {
		t.Fatalf("inspect png: %v", err)
	}
	if !info.hasSRGB {
		t.Fatalf("expected sRGB chunk to be detected: %+v", info)
	}
	item, _, _, _, invalid := classifyTextureChannelPacking("T_Data_Normal.png", info)
	if !invalid || item.IsCompliant || !strings.Contains(item.Issue, "sRGB") {
		t.Fatalf("sRGB normal map should be invalid, got %+v", item)
	}
}

func TestAuditTextureChannelPackingEmptyRepoIsHonest(t *testing.T) {
	result, err := GitService{}.AuditTextureChannelPacking(t.TempDir(), "")
	if err != nil {
		t.Fatalf("audit empty repo: %v", err)
	}
	if result.TotalTexturesScanned != 0 || len(result.Textures) != 0 || result.VramWasteEstimateBytes != 0 {
		t.Fatalf("empty repo should not fabricate texture results: %+v", result)
	}
	if !strings.Contains(result.Summary, "未发现") {
		t.Fatalf("empty repo summary should be explicit, got %q", result.Summary)
	}
}
