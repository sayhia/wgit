package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// =========================================================================
// 二进制资产差异
//
// 游戏仓库里最常见的改动是「换了一张贴图 / 改了一个模型」，但 git 只会说
// "Binary files differ"。这里把两个版本都取出来，给出尺寸、体积、格式与可
// 渲染的预览数据，让美术在客户端里就能看出改了什么。
// =========================================================================

type AssetDiffSide struct {
	Exists      bool   `json:"exists"`
	Ref         string `json:"ref"`
	Size        int64  `json:"size"`
	SizeHuman   string `json:"sizeHuman"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Format      string `json:"format"`
	Mime        string `json:"mime"`
	DataURL     string `json:"dataUrl"`
	Previewable bool   `json:"previewable"`
}

type AssetDiffResult struct {
	Path        string        `json:"path"`
	Kind        string        `json:"kind"` // image | audio | model | binary
	Old         AssetDiffSide `json:"old"`
	New         AssetDiffSide `json:"new"`
	SizeDelta   int64         `json:"sizeDelta"`
	WidthDelta  int           `json:"widthDelta"`
	HeightDelta int           `json:"heightDelta"`
	Message     string        `json:"message"`
}

// execGitBytes captures raw stdout, which ExecGit cannot do (it returns a
// trimmed string, corrupting binary blobs).
func execGitBytes(repoPath string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		// Filters and hooks can echo credential-bearing remotes, so the same
		// policy used by the activity log applies before the text reaches the UI.
		msg = redactLogSecrets(msg)
		return nil, fmt.Errorf("%s (%w)", msg, err)
	}
	return stdout.Bytes(), nil
}

func assetKindOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".tga", ".psd", ".psb",
		".exr", ".tif", ".tiff", ".hdr":
		return "image"
	case ".wav", ".mp3", ".ogg", ".bank", ".fsb", ".wem", ".flac":
		return "audio"
	case ".fbx", ".obj", ".glb", ".gltf", ".ma", ".mb", ".blend", ".max",
		".uasset", ".umap", ".unity", ".prefab":
		return "model"
	default:
		return "binary"
	}
}

// Webviews render these natively; TGA/PSD/EXR do not, so they only get metadata.
func assetPreviewMime(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".gif":
		return "image/gif", true
	case ".bmp":
		return "image/bmp", true
	case ".webp":
		return "image/webp", true
	case ".svg":
		return "image/svg+xml", true
	case ".wav":
		return "audio/wav", true
	case ".mp3":
		return "audio/mpeg", true
	case ".ogg":
		return "audio/ogg", true
	case ".flac":
		return "audio/flac", true
	case ".glb":
		return "model/gltf-binary", true
	case ".gltf":
		return "model/gltf+json", true
	case ".obj":
		return "text/plain", true
	default:
		return "", false
	}
}

// readAssetSide loads one side of the comparison: a committed blob, or the
// working tree when ref is "WORKTREE".
func readAssetSide(repoPath, ref, path string, maxPreviewBytes int64) (AssetDiffSide, error) {
	side := AssetDiffSide{Ref: ref}

	var data []byte
	var err error
	if strings.EqualFold(ref, "WORKTREE") {
		data, err = os.ReadFile(filepath.Join(repoPath, path))
		if err != nil {
			if os.IsNotExist(err) {
				return side, nil // deleted in the working tree
			}
			return side, err
		}
	} else {
		listed, listErr := ExecGit(repoPath, "ls-tree", "-r", "--name-only", ref, "--", path)
		if listErr != nil {
			return side, fmt.Errorf("读取 %s 的文件列表失败: %w", ref, listErr)
		}
		if strings.TrimSpace(listed) == "" {
			// The file simply may not exist at that revision.
			return side, nil
		}
		data, err = execGitBytes(repoPath, "show", ref+":"+path)
		if err != nil {
			return side, fmt.Errorf("读取 %s:%s 失败: %w", ref, path, err)
		}
	}

	side.Exists = true
	side.Size = int64(len(data))
	side.SizeHuman = formatBytes(side.Size)

	if cfg, format, derr := image.DecodeConfig(bytes.NewReader(data)); derr == nil {
		side.Width = cfg.Width
		side.Height = cfg.Height
		side.Format = format
	} else {
		ext := filepath.Ext(path)
		w, h, fmtStr := parseExtendedAssetMetadata(data, ext)
		side.Width = w
		side.Height = h
		side.Format = fmtStr
	}

	if mime, ok := assetPreviewMime(path); ok && side.Size <= maxPreviewBytes {
		side.Mime = mime
		side.Previewable = true
		side.DataURL = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	return side, nil
}

// parseExtendedAssetMetadata extracts dimensions and format details from headers of PSD, TGA, GLB, and FBX files.
func parseExtendedAssetMetadata(data []byte, ext string) (width int, height int, format string) {
	ext = strings.ToLower(ext)
	switch ext {
	case ".psd", ".psb":
		// PSD header: 4 bytes "8BPS", 2 bytes version, 6 bytes reserved, 2 bytes channels, 4 bytes height, 4 bytes width, 2 bytes depth
		if len(data) >= 26 && string(data[0:4]) == "8BPS" {
			channels := binary.BigEndian.Uint16(data[12:14])
			h := binary.BigEndian.Uint32(data[14:18])
			w := binary.BigEndian.Uint32(data[18:22])
			depth := binary.BigEndian.Uint16(data[22:24])
			return int(w), int(h), fmt.Sprintf("psd (%d-ch, %d-bit)", channels, depth)
		}
	case ".tga":
		// TGA header: 18 bytes. Width at offset 12 (uint16 LE), Height at offset 14 (uint16 LE), Pixel depth at offset 16 (uint8)
		if len(data) >= 18 {
			w := binary.LittleEndian.Uint16(data[12:14])
			h := binary.LittleEndian.Uint16(data[14:16])
			depth := data[16]
			if w > 0 && h > 0 && (depth == 8 || depth == 16 || depth == 24 || depth == 32) {
				return int(w), int(h), fmt.Sprintf("tga (%d-bit)", depth)
			}
		}
	case ".glb":
		if len(data) >= 12 && string(data[0:4]) == "glTF" {
			v := binary.LittleEndian.Uint32(data[4:8])
			return 0, 0, fmt.Sprintf("glb (v%d)", v)
		}
	case ".fbx":
		if len(data) >= 27 && strings.HasPrefix(string(data[0:20]), "Kaydara FBX Binary") {
			v := binary.LittleEndian.Uint32(data[23:27])
			return 0, 0, fmt.Sprintf("fbx (v%d)", v)
		}
	}
	return 0, 0, strings.TrimPrefix(ext, ".")
}

// GetAssetDiff compares a binary asset between two revisions and returns
// metadata plus, when the webview can render it, a data URL for each side.
func (GitService) GetAssetDiff(repoPath string, path string, oldRef string, newRef string) (*AssetDiffResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("asset path is required")
	}
	if strings.TrimSpace(oldRef) == "" || strings.TrimSpace(newRef) == "" {
		return nil, errors.New("需要指定新旧两个版本")
	}
	path, err := repoRelativeAssetPath(path)
	if err != nil {
		return nil, err
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}
	for _, ref := range []string{oldRef, newRef} {
		if strings.EqualFold(strings.TrimSpace(ref), "WORKTREE") {
			continue
		}
		if _, err := ExecGit(repoPath, "rev-parse", "--verify", ref+"^{commit}"); err != nil {
			return nil, fmt.Errorf("引用 %s 不存在或不可解析: %w", ref, err)
		}
	}

	const maxPreviewBytes = 12 << 20 // 12 MB: keeps the RPC payload sane

	oldSide, err := readAssetSide(repoPath, oldRef, path, maxPreviewBytes)
	if err != nil {
		return nil, fmt.Errorf("读取旧版本失败: %w", err)
	}
	newSide, err := readAssetSide(repoPath, newRef, path, maxPreviewBytes)
	if err != nil {
		return nil, fmt.Errorf("读取新版本失败: %w", err)
	}

	res := &AssetDiffResult{
		Path: path,
		Kind: assetKindOf(path),
		Old:  oldSide,
		New:  newSide,
	}
	if oldSide.Exists && newSide.Exists {
		res.SizeDelta = newSide.Size - oldSide.Size
		res.WidthDelta = newSide.Width - oldSide.Width
		res.HeightDelta = newSide.Height - oldSide.Height
	}

	switch {
	case !oldSide.Exists && !newSide.Exists:
		res.Message = "两个版本都不存在该文件。"
	case !oldSide.Exists:
		res.Message = fmt.Sprintf("新增资产：%s，%s", newSide.SizeHuman, dimensionSummary(newSide))
	case !newSide.Exists:
		res.Message = fmt.Sprintf("删除资产：原为 %s，%s", oldSide.SizeHuman, dimensionSummary(oldSide))
	default:
		parts := []string{sizeDeltaSummary(res.SizeDelta)}
		if res.WidthDelta != 0 || res.HeightDelta != 0 {
			parts = append(parts, fmt.Sprintf("尺寸 %d×%d → %d×%d",
				oldSide.Width, oldSide.Height, newSide.Width, newSide.Height))
		}
		if oldSide.Format != "" && newSide.Format != "" && oldSide.Format != newSide.Format {
			parts = append(parts, fmt.Sprintf("格式 %s → %s", oldSide.Format, newSide.Format))
		}
		res.Message = strings.Join(parts, "；")
	}

	if res.Kind == "image" && !oldSide.Previewable && !newSide.Previewable && (oldSide.Exists || newSide.Exists) {
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		res.Message += fmt.Sprintf("（.%s 无法在客户端内预览，建议用 DCC 工具对比）", ext)
	}
	return res, nil
}

func dimensionSummary(side AssetDiffSide) string {
	if side.Width > 0 && side.Height > 0 {
		return fmt.Sprintf("%d×%d", side.Width, side.Height)
	}
	return "尺寸未知"
}

func sizeDeltaSummary(delta int64) string {
	switch {
	case delta > 0:
		return "+" + formatBytes(delta)
	case delta < 0:
		return "-" + formatBytes(-delta)
	default:
		return "体积未变"
	}
}
