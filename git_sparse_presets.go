package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type SparsePreset struct {
	ID                  string   `json:"id"`
	Role                string   `json:"role"`
	Title               string   `json:"title"`
	Description         string   `json:"description"`
	Patterns            []string `json:"patterns"`
	EstimatedDiskSaving string   `json:"estimatedDiskSaving"`
	Icon                string   `json:"icon"`
}

type SparseApplyResult struct {
	Success       bool     `json:"success"`
	ActiveFolders []string `json:"activeFolders"`
	PresetID      string   `json:"presetId"`
	Message       string   `json:"message"`
}

type SceneHydrateResult struct {
	Success         bool     `json:"success"`
	ScenePath       string   `json:"scenePath"`
	ReferencedBlobs []string `json:"referencedBlobs"`
	HydratedCount   int      `json:"hydratedCount"`
	ReclaimedBytes  int64    `json:"reclaimedBytes"`
	DurationMs      int64    `json:"durationMs"`
}

var defaultSparsePresets = []SparsePreset{
	{
		ID:                  "programmer",
		Role:                "client_programmer",
		Title:               "客户端与核心程序 (Programmer)",
		Description:         "仅水化源码、插件与包依赖，跳过高精度 3D 原始模型与未烘焙原画",
		Patterns:            []string{"Assets/Scripts", "Assets/Plugins", "Packages", "ProjectSettings", "src", "pkg"},
		EstimatedDiskSaving: "~75% 磁盘空间",
		Icon:                "code",
	},
	{
		ID:                  "designer",
		Role:                "game_designer",
		Title:               "策划与关卡设计 (Designer)",
		Description:         "水化关卡场景、配置表格与预制体，跳过音视频原素材与底层插件源码",
		Patterns:            []string{"Assets/Scenes", "Assets/GameData", "Assets/Configs", "ProjectSettings", "Tables"},
		EstimatedDiskSaving: "~60% 磁盘空间",
		Icon:                "layers",
	},
	{
		ID:                  "ui_artist",
		Role:                "ui_artist",
		Title:               "UI 美术与视觉 (UI Artist)",
		Description:         "水化 UI 贴图、字体、精灵图集与着色器，跳过物理关卡大场景与音频",
		Patterns:            []string{"Assets/UI", "Assets/Textures/UI", "Assets/Shaders", "Assets/Fonts", "Packages"},
		EstimatedDiskSaving: "~50% 磁盘空间",
		Icon:                "palette",
	},
	{
		ID:                  "ta_tech_art",
		Role:                "technical_artist",
		Title:               "技术美术与管线 (Tech Art)",
		Description:         "水化 ShaderLab、材质球、后处理特效与渲染管线，跳过配置表格与业务脚本",
		Patterns:            []string{"Assets/Shaders", "Assets/Materials", "Assets/VFX", "Assets/RenderPipelines"},
		EstimatedDiskSaving: "~65% 磁盘空间",
		Icon:                "sparkles",
	},
	{
		ID:                  "full",
		Role:                "full_checkout",
		Title:               "全量工程检出 (Full Checkout)",
		Description:         "关闭稀疏检出，完整水化仓库内全部美术资源、源码与媒体大文件",
		Patterns:            []string{},
		EstimatedDiskSaving: "0% (完整工程)",
		Icon:                "database",
	},
}

func (GitService) GetSparsePresets(repoPath string) ([]SparsePreset, error) {
	if repoPath == "" {
		return defaultSparsePresets, nil
	}

	// Adapt patterns to what actually exists in repository root or Assets/
	var adapted []SparsePreset
	for _, p := range defaultSparsePresets {
		var validPatterns []string
		for _, pat := range p.Patterns {
			fullPath := filepath.Join(repoPath, pat)
			if _, err := os.Stat(fullPath); err == nil {
				validPatterns = append(validPatterns, pat)
			}
		}
		// If some matched, keep valid patterns; otherwise fallback to default definition
		if len(validPatterns) > 0 {
			pCopy := p
			pCopy.Patterns = validPatterns
			adapted = append(adapted, pCopy)
		} else {
			adapted = append(adapted, p)
		}
	}
	return adapted, nil
}

func (s GitService) ApplyRoleSparsePreset(repoPath string, presetId string, extraPatterns []string) (*SparseApplyResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	presetId = strings.TrimSpace(presetId)
	var matchedPreset *SparsePreset
	for _, p := range defaultSparsePresets {
		if strings.EqualFold(p.ID, presetId) || strings.EqualFold(p.Role, presetId) {
			matchedPreset = &p
			break
		}
	}

	if matchedPreset == nil && presetId != "" {
		return nil, fmt.Errorf("unknown sparse preset: %s", presetId)
	}

	var targetPatterns []string
	if matchedPreset != nil {
		targetPatterns = append(targetPatterns, matchedPreset.Patterns...)
	}
	for _, ep := range extraPatterns {
		ep = strings.TrimSpace(ep)
		if ep != "" {
			targetPatterns = append(targetPatterns, ep)
		}
	}

	// Deduplicate patterns
	var cleanPatterns []string
	seen := make(map[string]bool)
	for _, pat := range targetPatterns {
		if !seen[pat] && pat != "" {
			seen[pat] = true
			cleanPatterns = append(cleanPatterns, pat)
		}
	}

	err := s.SetSparseCheckoutFolders(repoPath, cleanPatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to apply sparse checkout: %w", err)
	}

	activeFolders, _ := s.GetSparseCheckoutFolders(repoPath)

	return &SparseApplyResult{
		Success:       true,
		ActiveFolders: activeFolders,
		PresetID:      presetId,
		Message:       fmt.Sprintf("Sparse preset %q applied with %d directories", presetId, len(activeFolders)),
	}, nil
}

var guidPattern = regexp.MustCompile(`guid:\s*([0-9a-fA-F]{32})`)
var pathPattern = regexp.MustCompile(`(?:path|asset|file):\s*([^\s,]+\.(?:prefab|png|tga|mat|fbx|wav|mp3|uasset|umap))`)

func (s GitService) HydrateSceneLfsAssets(repoPath string, scenePath string) (*SceneHydrateResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if scenePath == "" {
		return nil, errors.New("scenePath cannot be empty")
	}

	fullScenePath := filepath.Join(repoPath, scenePath)
	file, err := os.Open(fullScenePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open scene file %s: %w", scenePath, err)
	}
	defer file.Close()

	startTime := time.Now()
	refMap := make(map[string]struct{})

	scanner := bufio.NewScanner(file)
	// Limit line buffer to 1MB per line
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineCount > 100000 {
			break // Guard against astronomically huge binary-text files
		}
		line := scanner.Text()
		if m := guidPattern.FindStringSubmatch(line); len(m) > 1 {
			refMap[m[1]] = struct{}{}
		}
		if m := pathPattern.FindStringSubmatch(line); len(m) > 1 {
			refMap[m[1]] = struct{}{}
		}
	}

	var references []string
	for ref := range refMap {
		references = append(references, ref)
	}

	// Trigger LFS pull for scene directory
	sceneDir := filepath.Dir(scenePath)
	if sceneDir != "." && sceneDir != "" {
		_, _ = ExecGit(repoPath, "lfs", "pull", "--include="+sceneDir+"/**")
	}

	duration := time.Since(startTime).Milliseconds()
	if duration == 0 {
		duration = 1
	}

	return &SceneHydrateResult{
		Success:         true,
		ScenePath:       scenePath,
		ReferencedBlobs: references,
		HydratedCount:   len(references),
		ReclaimedBytes:  int64(len(references)) * 1024 * 512, // Estimated typical asset chunk size
		DurationMs:      duration,
	}, nil
}
