package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// 游戏引擎与资产管线：Unity/Unreal 工程识别、资产/音频/网格巡检、LFS 友好配置。
// 拆自 git_service.go（Phase 2），与核心 Git 操作解耦，便于按引擎能力独立演进。

// InspectAssetMetadata extracts technical dimensions, LFS pointers, size, and mime information
// repoRelativeAssetPath keeps asset RPCs from reading files outside the
// repository via "../" or absolute paths.
func repoRelativeAssetPath(p string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(p))
	if clean == "" || clean == "." || filepath.IsAbs(clean) || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("资产路径必须位于仓库内")
	}
	return clean, nil
}

func (GitService) InspectAssetMetadata(repoPath string, filePath string) (*AssetMetadata, error) {
	cleanPath, err := repoRelativeAssetPath(filePath)
	if err != nil {
		return nil, err
	}
	filePath = cleanPath
	fullPath := filepath.Join(repoPath, filePath)
	fi, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	meta := &AssetMetadata{
		Path:        filePath,
		Extension:   ext,
		SizeBytes:   fi.Size(),
		SizeHuman:   formatByteSize(fi.Size()),
		MimeType:    detectAssetMime(ext),
		LastModTime: fi.ModTime().Format("2006-01-02 15:04:05"),
	}

	// Check image dimensions
	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" {
		f, err := os.Open(fullPath)
		if err == nil {
			defer f.Close()
			cfg, _, err := image.DecodeConfig(f)
			if err == nil {
				meta.Dimensions = fmt.Sprintf("%d × %d px", cfg.Width, cfg.Height)
			}
		}
	}

	// Check LFS status from the actual attribute rules, not a hard-coded
	// extension heuristic (which mislabels any .png as LFS even without rules).
	if attrOut, attrErr := ExecGit(repoPath, "check-attr", "filter", "--", filePath); attrErr == nil {
		meta.IsLFS = strings.Contains(attrOut, "filter: lfs")
	}
	if meta.IsLFS {
		ptr, ptrErr := ExecGit(repoPath, "lfs", "pointer", fmt.Sprintf("--file=%s", filePath))
		if ptrErr != nil {
			meta.LFSPointerError = "读取 LFS 指针失败: " + ptrErr.Error()
		} else if strings.Contains(ptr, "version https://git-lfs.github.com/spec/v1") {
			meta.LFSPointer = strings.TrimSpace(ptr)
		}
	}

	return meta, nil
}

// CheckUnityMetaSync scans repository changed files to detect missing or un-staged Unity .meta files
func (s GitService) CheckUnityMetaSync(repoPath string) ([]UnityMetaMismatch, error) {
	status, err := s.GetRepoStatus(repoPath)
	if err != nil {
		return nil, err
	}

	stagedSet := make(map[string]bool)
	for _, f := range status.StagedFiles {
		stagedSet[f.Path] = true
	}

	unstagedSet := make(map[string]bool)
	for _, f := range status.UnstagedFiles {
		unstagedSet[f.Path] = true
	}

	trackedSet := make(map[string]bool)
	if trackedOut, trackedErr := ExecGit(repoPath, "ls-files", "-z"); trackedErr == nil {
		for _, p := range strings.Split(trackedOut, "\x00") {
			if p != "" {
				trackedSet[p] = true
			}
		}
	}

	seenKeys := make(map[string]bool)
	mismatches := []UnityMetaMismatch{}

	// `git status` collapses untracked directories (Assets/), so enumerate the
	// actual files under Assets/ and Packages/ instead of relying on status paths.
	raw, err := ExecGit(repoPath, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "Assets", "Packages")
	if err != nil {
		return []UnityMetaMismatch{}, nil
	}

	for _, p := range strings.Split(raw, "\x00") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, ".meta") {
			assetPath := strings.TrimSuffix(p, ".meta")
			key := assetPath + "::meta"
			if seenKeys[key] {
				continue
			}
			seenKeys[key] = true

			fullAssetPath := filepath.Join(repoPath, assetPath)
			_, statErr := os.Stat(fullAssetPath)
			if statErr != nil && os.IsNotExist(statErr) {
				mismatches = append(mismatches, UnityMetaMismatch{
					AssetPath: assetPath,
					MetaPath:  p,
					IssueType: "orphan_meta",
					Message:   fmt.Sprintf("孤立的 .meta 文件: 对应资产 %s 已被删除或不存在", assetPath),
				})
			} else if stagedSet[p] && !stagedSet[assetPath] && (unstagedSet[assetPath] || !trackedSet[assetPath]) {
				mismatches = append(mismatches, UnityMetaMismatch{
					AssetPath: assetPath,
					MetaPath:  p,
					IssueType: "unpaired_staged",
					Message:   fmt.Sprintf(".meta 文件已暂存，但资产文件 %s 未暂存", assetPath),
				})
			}
			continue
		}

		metaPath := p + ".meta"
		key := p + "::asset"
		if seenKeys[key] {
			continue
		}
		seenKeys[key] = true

		fullAssetPath := filepath.Join(repoPath, p)
		fi, statErr := os.Stat(fullAssetPath)
		if statErr != nil || fi.IsDir() {
			continue
		}

		if _, metaStatErr := os.Stat(filepath.Join(repoPath, metaPath)); os.IsNotExist(metaStatErr) {
			mismatches = append(mismatches, UnityMetaMismatch{
				AssetPath: p,
				MetaPath:  metaPath,
				IssueType: "missing_meta",
				Message:   fmt.Sprintf("缺少配套 .meta 文件: %s 缺少对应的 %s", p, metaPath),
			})
		} else if stagedSet[p] && !stagedSet[metaPath] && (unstagedSet[metaPath] || !trackedSet[metaPath]) {
			mismatches = append(mismatches, UnityMetaMismatch{
				AssetPath: p,
				MetaPath:  metaPath,
				IssueType: "unpaired_staged",
				Message:   fmt.Sprintf("资产文件已暂存，但配套 %s 未暂存", metaPath),
			})
		}
	}

	return mismatches, nil
}

// AutoFixUnityMeta automatically stages unpaired .meta files or their assets
func (s GitService) AutoFixUnityMeta(repoPath string) ([]string, error) {
	mismatches, err := s.CheckUnityMetaSync(repoPath)
	if err != nil {
		return nil, err
	}

	fixed := []string{}
	failures := []string{}
	stage := func(path string) {
		if _, err := ExecGit(repoPath, "add", "--", path); err != nil {
			failures = append(failures, path+": "+err.Error())
			return
		}
		fixed = append(fixed, path)
	}
	for _, m := range mismatches {
		if m.IssueType == "unpaired_staged" {
			// If meta is missing from index, stage it
			fullMeta := filepath.Join(repoPath, m.MetaPath)
			if _, err := os.Stat(fullMeta); err == nil {
				stage(m.MetaPath)
			}
			fullAsset := filepath.Join(repoPath, m.AssetPath)
			if _, err := os.Stat(fullAsset); err == nil {
				stage(m.AssetPath)
			}
		}
	}
	if len(failures) > 0 {
		return fixed, fmt.Errorf("部分配对未能暂存: %s", strings.Join(failures, "; "))
	}
	return fixed, nil
}

// DetectGameEngineProject detects Unity, Unreal Engine, Godot, or Cocos project structure
func (GitService) DetectGameEngineProject(repoPath string) (*GameEngineInfo, error) {
	info := &GameEngineInfo{
		EngineType:        "None",
		HasGitAttributes:  false,
		HasLfsRules:       false,
		ProjectName:       filepath.Base(repoPath),
		EngineVersion:     "",
		RecommendedPreset: "",
	}

	// Check .gitattributes
	attrData, err := os.ReadFile(filepath.Join(repoPath, ".gitattributes"))
	if err == nil {
		info.HasGitAttributes = true
		content := string(attrData)
		if strings.Contains(content, "filter=lfs") {
			info.HasLfsRules = true
		}
	}

	// 1. Check Unreal Engine (*.uproject)
	entries, _ := os.ReadDir(repoPath)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".uproject") {
			info.EngineType = "Unreal"
			info.ProjectName = strings.TrimSuffix(entry.Name(), ".uproject")
			info.RecommendedPreset = "unreal"
			uprojectData, _ := os.ReadFile(filepath.Join(repoPath, entry.Name()))
			if idx := strings.Index(string(uprojectData), `"EngineAssociation":`); idx != -1 {
				sub := string(uprojectData)[idx:]
				parts := strings.Split(sub, `"`)
				if len(parts) >= 4 {
					info.EngineVersion = parts[3]
				}
			}
			return info, nil
		}
	}

	// 2. Check Unity (Assets/ and ProjectSettings/)
	if _, err := os.Stat(filepath.Join(repoPath, "Assets")); err == nil {
		if _, err := os.Stat(filepath.Join(repoPath, "ProjectSettings")); err == nil {
			info.EngineType = "Unity"
			info.RecommendedPreset = "unity"
			pvPath := filepath.Join(repoPath, "ProjectSettings", "ProjectVersion.txt")
			pvData, err := os.ReadFile(pvPath)
			if err == nil {
				for _, line := range strings.Split(string(pvData), "\n") {
					if strings.HasPrefix(line, "m_EditorVersion:") {
						info.EngineVersion = strings.TrimSpace(strings.TrimPrefix(line, "m_EditorVersion:"))
						break
					}
				}
			}
			return info, nil
		}
	}

	// 3. Check Godot (project.godot)
	if _, err := os.Stat(filepath.Join(repoPath, "project.godot")); err == nil {
		info.EngineType = "Godot"
		info.RecommendedPreset = "godot"
		return info, nil
	}

	// 4. Check Cocos Creator (project.json or assets/)
	if _, err := os.Stat(filepath.Join(repoPath, "project.json")); err == nil {
		info.EngineType = "Cocos"
		info.RecommendedPreset = "cocos"
		return info, nil
	}

	return info, nil
}

// GetGameEngineGitAttributesPreset returns tailored .gitattributes content for the specified engine
func (GitService) GetGameEngineGitAttributesPreset(engine string) string {
	engine = strings.ToLower(engine)
	switch engine {
	case "unity":
		return `# Tencent UGit Unity 3D 最佳实践 .gitattributes
# Auto-detect text files and perform LF normalization
* text=auto eol=lf

# Unity YAML Merge Tool Configuration
*.prefab text merge=unityyamlmerge eol=lf
*.unity text merge=unityyamlmerge eol=lf
*.asset text merge=unityyamlmerge eol=lf
*.mat text merge=unityyamlmerge eol=lf

# 3D Model & Animation Assets (Git LFS)
*.fbx filter=lfs diff=lfs merge=lfs -text
*.obj filter=lfs diff=lfs merge=lfs -text
*.max filter=lfs diff=lfs merge=lfs -text
*.blend filter=lfs diff=lfs merge=lfs -text
*.dae filter=lfs diff=lfs merge=lfs -text
*.3ds filter=lfs diff=lfs merge=lfs -text
*.anim filter=lfs diff=lfs merge=lfs -text
*.controller filter=lfs diff=lfs merge=lfs -text

# Textures & Images (Git LFS)
*.psd filter=lfs diff=lfs merge=lfs -text
*.png filter=lfs diff=lfs merge=lfs -text
*.jpg filter=lfs diff=lfs merge=lfs -text
*.jpeg filter=lfs diff=lfs merge=lfs -text
*.tga filter=lfs diff=lfs merge=lfs -text
*.tiff filter=lfs diff=lfs merge=lfs -text
*.exr filter=lfs diff=lfs merge=lfs -text
*.hdr filter=lfs diff=lfs merge=lfs -text
*.cubemap filter=lfs diff=lfs merge=lfs -text

# Audio & Video (Git LFS)
*.mp3 filter=lfs diff=lfs merge=lfs -text
*.wav filter=lfs diff=lfs merge=lfs -text
*.ogg filter=lfs diff=lfs merge=lfs -text
*.mp4 filter=lfs diff=lfs merge=lfs -text
*.mov filter=lfs diff=lfs merge=lfs -text

# Compiled Libraries & Binary Archives
*.dll filter=lfs diff=lfs merge=lfs -text
*.so filter=lfs diff=lfs merge=lfs -text
*.bundle filter=lfs diff=lfs merge=lfs -text
*.dylib filter=lfs diff=lfs merge=lfs -text
*.aar filter=lfs diff=lfs merge=lfs -text
*.jar filter=lfs diff=lfs merge=lfs -text
*.zip filter=lfs diff=lfs merge=lfs -text
*.tar.gz filter=lfs diff=lfs merge=lfs -text
`
	case "unreal":
		return `# Tencent UGit Unreal Engine 5 最佳实践 .gitattributes
# Auto-detect text files and perform LF normalization
* text=auto eol=lf

# Unreal Engine Binary Assets (Git LFS with Lock Support)
*.uasset filter=lfs diff=lfs merge=lfs -text lockable
*.umap filter=lfs diff=lfs merge=lfs -text lockable

# 3D Meshes & Skeletal Rigs (Git LFS)
*.fbx filter=lfs diff=lfs merge=lfs -text lockable
*.obj filter=lfs diff=lfs merge=lfs -text
*.blend filter=lfs diff=lfs merge=lfs -text

# Textures, HDRIs & VFX (Git LFS)
*.png filter=lfs diff=lfs merge=lfs -text
*.psd filter=lfs diff=lfs merge=lfs -text lockable
*.tga filter=lfs diff=lfs merge=lfs -text
*.exr filter=lfs diff=lfs merge=lfs -text
*.hdr filter=lfs diff=lfs merge=lfs -text

# Audio & Media (Git LFS)
*.wav filter=lfs diff=lfs merge=lfs -text
*.mp3 filter=lfs diff=lfs merge=lfs -text
*.mp4 filter=lfs diff=lfs merge=lfs -text

# Compiled Binaries & Plugins
*.dll filter=lfs diff=lfs merge=lfs -text
*.dylib filter=lfs diff=lfs merge=lfs -text
*.so filter=lfs diff=lfs merge=lfs -text
*.lib filter=lfs diff=lfs merge=lfs -text
*.a filter=lfs diff=lfs merge=lfs -text
`
	case "godot":
		return `# Tencent UGit Godot Engine 最佳实践 .gitattributes
* text=auto eol=lf
*.tscn text eol=lf
*.tres text eol=lf
*.gd text eol=lf

# Godot Binary Imports & Media (Git LFS)
*.png filter=lfs diff=lfs merge=lfs -text
*.psd filter=lfs diff=lfs merge=lfs -text
*.wav filter=lfs diff=lfs merge=lfs -text
*.ogg filter=lfs diff=lfs merge=lfs -text
*.mp3 filter=lfs diff=lfs merge=lfs -text
*.blend filter=lfs diff=lfs merge=lfs -text
*.fbx filter=lfs diff=lfs merge=lfs -text
*.glb filter=lfs diff=lfs merge=lfs -text
*.gltf filter=lfs diff=lfs merge=lfs -text
`
	case "cocos":
		return `# Tencent UGit Cocos Creator 最佳实践 .gitattributes
* text=auto eol=lf
*.fire text eol=lf
*.prefab text eol=lf
*.anim text eol=lf

# Cocos Binary Media (Git LFS)
*.png filter=lfs diff=lfs merge=lfs -text
*.jpg filter=lfs diff=lfs merge=lfs -text
*.psd filter=lfs diff=lfs merge=lfs -text
*.mp3 filter=lfs diff=lfs merge=lfs -text
*.wav filter=lfs diff=lfs merge=lfs -text
*.zip filter=lfs diff=lfs merge=lfs -text
`
	default:
		return `# Tencent UGit 通用游戏多媒体资产 .gitattributes
* text=auto eol=lf

# Common Game Art & Binary Formats (Git LFS)
*.psd filter=lfs diff=lfs merge=lfs -text lockable
*.png filter=lfs diff=lfs merge=lfs -text
*.fbx filter=lfs diff=lfs merge=lfs -text lockable
*.blend filter=lfs diff=lfs merge=lfs -text
*.wav filter=lfs diff=lfs merge=lfs -text
*.mp3 filter=lfs diff=lfs merge=lfs -text
*.mp4 filter=lfs diff=lfs merge=lfs -text
*.zip filter=lfs diff=lfs merge=lfs -text
`
	}
}

// DetectUnityYamlMerge detects Unity installations and UnityYAMLMerge driver configuration
func (GitService) DetectUnityYamlMerge(repoPath string) (*UnityMergeStatus, error) {
	status := &UnityMergeStatus{
		InstalledVersions: []string{},
	}

	// 1. Check existing git config
	driver, err := ExecGit(repoPath, "config", "--get", "merge.unityyamlmerge.driver")
	if err == nil && strings.TrimSpace(driver) != "" {
		status.IsConfigured = true
		status.ConfiguredToolPath = strings.TrimSpace(driver)
	}

	// 2. Check .gitattributes for unityyamlmerge
	attrPath := filepath.Join(repoPath, ".gitattributes")
	if data, err := os.ReadFile(attrPath); err == nil {
		if strings.Contains(string(data), "merge=unityyamlmerge") {
			status.HasGitattributes = true
		}
	}

	// 3. Detect standard Unity installations
	var candidates []string
	if runtime.GOOS == "darwin" {
		hubDir := "/Applications/Unity/Hub/Editor"
		if entries, err := os.ReadDir(hubDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					tool := filepath.Join(hubDir, e.Name(), "Unity.app", "Contents", "Tools", "UnityYAMLMerge")
					if fi, err := os.Stat(tool); err == nil && !fi.IsDir() {
						candidates = append(candidates, tool)
						status.InstalledVersions = append(status.InstalledVersions, e.Name())
					}
				}
			}
		}
		legacyTool := "/Applications/Unity/Unity.app/Contents/Tools/UnityYAMLMerge"
		if fi, err := os.Stat(legacyTool); err == nil && !fi.IsDir() {
			candidates = append(candidates, legacyTool)
			status.InstalledVersions = append(status.InstalledVersions, "Standalone")
		}
	} else if runtime.GOOS == "windows" {
		hubDir := `C:\Program Files\Unity\Hub\Editor`
		if entries, err := os.ReadDir(hubDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					tool := filepath.Join(hubDir, e.Name(), "Editor", "Data", "Tools", "UnityYAMLMerge.exe")
					if fi, err := os.Stat(tool); err == nil && !fi.IsDir() {
						candidates = append(candidates, tool)
						status.InstalledVersions = append(status.InstalledVersions, e.Name())
					}
				}
			}
		}
	}

	if len(candidates) > 0 {
		status.DetectedToolPath = candidates[len(candidates)-1]
	}

	return status, nil
}

// ConfigureUnityYamlMerge configures git merge driver for Unity scenes and prefabs
func (GitService) ConfigureUnityYamlMerge(repoPath string, customPath string, applyGitattributes bool) error {
	toolPath := customPath
	if toolPath == "" {
		st, _ := GitService{}.DetectUnityYamlMerge(repoPath)
		if st != nil && st.DetectedToolPath != "" {
			toolPath = st.DetectedToolPath
		}
	}
	if toolPath == "" {
		return errors.New("未指定或未检测到 UnityYAMLMerge 可执行文件路径")
	}

	// Configure git config
	if _, err := ExecGit(repoPath, "config", "merge.unityyamlmerge.name", "Unity Smart Merge"); err != nil {
		return err
	}
	driverCmd := fmt.Sprintf("'%s' merge -h -p --mode 1 -a %%O %%B %%A %%A", toolPath)
	if _, err := ExecGit(repoPath, "config", "merge.unityyamlmerge.driver", driverCmd); err != nil {
		return err
	}
	if _, err := ExecGit(repoPath, "config", "merge.unityyamlmerge.trustExitCode", "false"); err != nil {
		return err
	}

	// Append to .gitattributes
	if applyGitattributes {
		attrPath := filepath.Join(repoPath, ".gitattributes")
		var existingContent string
		if data, err := os.ReadFile(attrPath); err == nil {
			existingContent = string(data)
		}
		if !strings.Contains(existingContent, "merge=unityyamlmerge") {
			var sb strings.Builder
			if existingContent != "" && !strings.HasSuffix(existingContent, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("# Unity Smart Merge (UnityYAMLMerge)\n")
			sb.WriteString("*.unity merge=unityyamlmerge\n")
			sb.WriteString("*.prefab merge=unityyamlmerge\n")
			sb.WriteString("*.asset merge=unityyamlmerge\n")
			sb.WriteString("*.mat merge=unityyamlmerge\n")
			if err := os.WriteFile(attrPath, []byte(existingContent+sb.String()), 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

// DetectUnrealProject detects Unreal Engine .uproject and project cache usage
func (GitService) DetectUnrealProject(repoPath string) (*UnrealProjectInfo, error) {
	info := &UnrealProjectInfo{
		DetectedEnginePaths: []string{},
		CacheFoldersFound:   []string{},
	}

	// 1. Locate .uproject file
	var uProjectPath string
	entries, err := os.ReadDir(repoPath)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".uproject") {
				uProjectPath = filepath.Join(repoPath, e.Name())
				info.ProjectName = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
				break
			}
		}
	}

	// Also check one-level subdirectories if not found in root
	if uProjectPath == "" && err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "node_modules" {
				subDir := filepath.Join(repoPath, e.Name())
				subEntries, _ := os.ReadDir(subDir)
				for _, subE := range subEntries {
					if !subE.IsDir() && strings.HasSuffix(strings.ToLower(subE.Name()), ".uproject") {
						uProjectPath = filepath.Join(subDir, subE.Name())
						info.ProjectName = strings.TrimSuffix(subE.Name(), filepath.Ext(subE.Name()))
						break
					}
				}
				if uProjectPath != "" {
					break
				}
			}
		}
	}

	if uProjectPath == "" {
		info.IsUnrealProject = false
		return info, nil
	}

	info.IsUnrealProject = true
	info.UProjectPath = uProjectPath

	// Read engine version from .uproject JSON
	if data, err := os.ReadFile(uProjectPath); err == nil {
		content := string(data)
		idx := strings.Index(content, "\"EngineAssociation\"")
		if idx != -1 {
			sub := content[idx:]
			colonIdx := strings.Index(sub, ":")
			if colonIdx != -1 {
				valPart := sub[colonIdx+1:]
				valPart = strings.TrimSpace(valPart)
				valPart = strings.Trim(valPart, ",\r\n}")
				valPart = strings.Trim(valPart, "\"")
				info.EngineVersion = valPart
			}
		}
	}

	// Detect installed Unreal Engine versions on system
	if runtime.GOOS == "darwin" {
		epicGamesDir := "/Users/Shared/Epic Games"
		if ueEntries, err := os.ReadDir(epicGamesDir); err == nil {
			for _, ueE := range ueEntries {
				if strings.HasPrefix(ueE.Name(), "UE_") {
					editorApp := filepath.Join(epicGamesDir, ueE.Name(), "Engine", "Binaries", "Mac", "UnrealEditor.app")
					if _, err := os.Stat(editorApp); err == nil {
						info.DetectedEnginePaths = append(info.DetectedEnginePaths, editorApp)
					}
				}
			}
		}
	} else if runtime.GOOS == "windows" {
		epicGamesDir := `C:\Program Files\Epic Games`
		if ueEntries, err := os.ReadDir(epicGamesDir); err == nil {
			for _, ueE := range ueEntries {
				if strings.HasPrefix(ueE.Name(), "UE_") {
					editorExe := filepath.Join(epicGamesDir, ueE.Name(), "Engine", "Binaries", "Win64", "UnrealEditor.exe")
					if _, err := os.Stat(editorExe); err == nil {
						info.DetectedEnginePaths = append(info.DetectedEnginePaths, editorExe)
					}
				}
			}
		}
	}

	// Calculate cache size (Intermediate, Saved, DerivedDataCache, .vs)
	projectDir := filepath.Dir(uProjectPath)
	cacheFolderNames := []string{"Intermediate", "Saved", "DerivedDataCache", ".vs"}
	var totalCacheBytes int64

	for _, folderName := range cacheFolderNames {
		targetDir := filepath.Join(projectDir, folderName)
		if fi, err := os.Stat(targetDir); err == nil && fi.IsDir() {
			info.CacheFoldersFound = append(info.CacheFoldersFound, folderName)
			_ = filepath.Walk(targetDir, func(p string, fi os.FileInfo, err error) error {
				if err == nil && !fi.IsDir() {
					totalCacheBytes += fi.Size()
				}
				return nil
			})
		}
	}

	info.CacheSizeBytes = totalCacheBytes
	info.CacheSizeStr = formatByteSize(totalCacheBytes)

	return info, nil
}

// CleanUnrealEngineCache removes Intermediate, Saved, DerivedDataCache, and .vs
func (GitService) CleanUnrealEngineCache(repoPath string) (*UnrealCacheCleanResult, error) {
	st, err := GitService{}.DetectUnrealProject(repoPath)
	if err != nil {
		return nil, err
	}
	if !st.IsUnrealProject {
		return nil, errors.New("当前仓库未检测到 Unreal Engine 项目 (.uproject)")
	}

	projectDir := filepath.Dir(st.UProjectPath)
	result := &UnrealCacheCleanResult{CleanedFolders: []string{}}

	var totalFreed int64
	for _, folder := range st.CacheFoldersFound {
		target := filepath.Join(projectDir, folder)
		var folderBytes int64
		_ = filepath.Walk(target, func(p string, fi os.FileInfo, err error) error {
			if err == nil && fi != nil && !fi.IsDir() {
				folderBytes += fi.Size()
			}
			return nil
		})
		if err := os.RemoveAll(target); err == nil {
			result.CleanedFolders = append(result.CleanedFolders, folder)
			totalFreed += folderBytes
		}
	}
	// Report what was actually removed: a failed RemoveAll must not contribute
	// to the freed size just because the pre-scan measured it.
	result.FreedSizeBytes = totalFreed
	result.FreedSizeStr = formatByteSize(totalFreed)

	return result, nil
}

// DetectGameEngineWorkflows detects Unreal Engine, Unity 3D, Godot, and Cocos Creator workflows
func (GitService) DetectGameEngineWorkflows(repoPath string) ([]EngineWorkflowInfo, error) {
	var workflows []EngineWorkflowInfo

	// 1. Check Unreal Engine
	ueInfo, err := GitService{}.DetectUnrealProject(repoPath)
	if err == nil && ueInfo.IsUnrealProject {
		workflows = append(workflows, EngineWorkflowInfo{
			EngineType:     "unreal",
			ProjectName:    ueInfo.ProjectName,
			ProjectPath:    ueInfo.UProjectPath,
			EngineVersion:  ueInfo.EngineVersion,
			CacheFolders:   ueInfo.CacheFoldersFound,
			CacheSizeBytes: ueInfo.CacheSizeBytes,
			CacheSizeStr:   ueInfo.CacheSizeStr,
			RecommendedActions: []string{
				"清理中间生成与 DDC 缓存 (Intermediate/Saved/DerivedDataCache)",
				"执行 LFS 资产只读锁保护与权限同步",
				"按岗位定制 LFS 选择性拉取 (策划/美术/音频)",
			},
		})
	}

	// 2. Check Unity 3D
	unityProjectVersion := filepath.Join(repoPath, "ProjectSettings", "ProjectVersion.txt")
	if fi, err := os.Stat(unityProjectVersion); err == nil && !fi.IsDir() {
		projectName := filepath.Base(repoPath)
		version := "Unity (未知版本)"
		if data, err := os.ReadFile(unityProjectVersion); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "m_EditorVersion:") {
					version = strings.TrimSpace(strings.TrimPrefix(line, "m_EditorVersion:"))
					break
				}
			}
		}
		var cacheFolders []string
		var cacheBytes int64
		for _, f := range []string{"Library", "Temp", "Logs", "obj"} {
			tp := filepath.Join(repoPath, f)
			if s, err := os.Stat(tp); err == nil && s.IsDir() {
				cacheFolders = append(cacheFolders, f)
				_ = filepath.Walk(tp, func(p string, info os.FileInfo, err error) error {
					if err == nil && !info.IsDir() {
						cacheBytes += info.Size()
					}
					return nil
				})
			}
		}
		workflows = append(workflows, EngineWorkflowInfo{
			EngineType:     "unity",
			ProjectName:    projectName,
			ProjectPath:    repoPath,
			EngineVersion:  version,
			CacheFolders:   cacheFolders,
			CacheSizeBytes: cacheBytes,
			CacheSizeStr:   formatByteSize(cacheBytes),
			RecommendedActions: []string{
				"配置 UnityYAMLMerge 智能场景预制体合并",
				"巡检 Unity .meta 文件孤立与丢失状态",
				"清理 Library/Temp 临时缓存",
			},
		})
	}

	// 3. Check Godot Engine
	godotProjectFile := filepath.Join(repoPath, "project.godot")
	if fi, err := os.Stat(godotProjectFile); err == nil && !fi.IsDir() {
		projectName := filepath.Base(repoPath)
		godotVersion := "Godot 4.x"
		if data, err := os.ReadFile(godotProjectFile); err == nil {
			content := string(data)
			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "config/name=") {
					name := strings.Trim(strings.TrimPrefix(line, "config/name="), "\"")
					if name != "" {
						projectName = name
					}
				}
				if strings.HasPrefix(line, "config/features=") && strings.Contains(line, "3.") {
					godotVersion = "Godot 3.x"
				}
			}
		}
		var cacheFolders []string
		var cacheBytes int64
		for _, f := range []string{".godot/imported", ".godot/editor", ".godot/shader_cache", ".import"} {
			tp := filepath.Join(repoPath, f)
			if s, err := os.Stat(tp); err == nil && s.IsDir() {
				cacheFolders = append(cacheFolders, f)
				_ = filepath.Walk(tp, func(p string, info os.FileInfo, err error) error {
					if err == nil && !info.IsDir() {
						cacheBytes += info.Size()
					}
					return nil
				})
			}
		}
		workflows = append(workflows, EngineWorkflowInfo{
			EngineType:     "godot",
			ProjectName:    projectName,
			ProjectPath:    godotProjectFile,
			EngineVersion:  godotVersion,
			CacheFolders:   cacheFolders,
			CacheSizeBytes: cacheBytes,
			CacheSizeStr:   formatByteSize(cacheBytes),
			RecommendedActions: []string{
				"清理 .godot 导入缓存与着色器编译副本",
				"检查 .uid 与 res:// 资源引用一致性",
				"配置 .gitattributes 保护 .tscn / .tres 文本文件",
			},
		})
	}

	// 4. Check Cocos Creator
	cocosProjectFile := filepath.Join(repoPath, "project.json")
	if _, err := os.Stat(cocosProjectFile); os.IsNotExist(err) {
		cocosProjectFile = filepath.Join(repoPath, "settings", "project.json")
	}
	if fi, err := os.Stat(cocosProjectFile); err == nil && !fi.IsDir() {
		projectName := filepath.Base(repoPath)
		cocosVersion := "Cocos Creator"
		if data, err := os.ReadFile(cocosProjectFile); err == nil {
			var parsed map[string]interface{}
			if err := json.Unmarshal(data, &parsed); err == nil {
				if n, ok := parsed["name"].(string); ok && n != "" {
					projectName = n
				}
				if v, ok := parsed["version"].(string); ok && v != "" {
					cocosVersion = "Cocos Creator " + v
				} else if c, ok := parsed["creator"].(map[string]interface{}); ok {
					if cv, ok := c["version"].(string); ok {
						cocosVersion = "Cocos Creator " + cv
					}
				}
			}
		}
		var cacheFolders []string
		var cacheBytes int64
		for _, f := range []string{"temp", "library", "local", "profiles"} {
			tp := filepath.Join(repoPath, f)
			if s, err := os.Stat(tp); err == nil && s.IsDir() {
				cacheFolders = append(cacheFolders, f)
				_ = filepath.Walk(tp, func(p string, info os.FileInfo, err error) error {
					if err == nil && !info.IsDir() {
						cacheBytes += info.Size()
					}
					return nil
				})
			}
		}
		workflows = append(workflows, EngineWorkflowInfo{
			EngineType:     "cocos",
			ProjectName:    projectName,
			ProjectPath:    cocosProjectFile,
			EngineVersion:  cocosVersion,
			CacheFolders:   cacheFolders,
			CacheSizeBytes: cacheBytes,
			CacheSizeStr:   formatByteSize(cacheBytes),
			RecommendedActions: []string{
				"清理 temp 与 library 引擎构建与缓存数据",
				"检查 Cocos assets/*.meta 资产配对完整性",
				"将 Cocos 构建输出 build/ 加入 .gitignore",
			},
		})
	}

	return workflows, nil
}

// CleanGameEngineCaches cleans temporary cache folders for specific game engines
func (GitService) CleanGameEngineCaches(repoPath string, engineType string) (*UnrealCacheCleanResult, error) {
	var targetFolders []string
	switch engineType {
	case "unreal":
		return GitService{}.CleanUnrealEngineCache(repoPath)
	case "unity":
		targetFolders = []string{"Library", "Temp", "Logs", "obj"}
	case "godot":
		targetFolders = []string{".godot/imported", ".godot/editor", ".godot/shader_cache"}
	case "cocos":
		targetFolders = []string{"temp", "library", "local", "profiles"}
	default:
		return nil, fmt.Errorf("未知或不支持的游戏引擎类型: %s", engineType)
	}

	result := &UnrealCacheCleanResult{
		CleanedFolders: []string{},
	}
	var totalFreed int64

	for _, folder := range targetFolders {
		p := filepath.Join(repoPath, folder)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			var fBytes int64
			_ = filepath.Walk(p, func(wp string, wi os.FileInfo, werr error) error {
				if werr == nil && !wi.IsDir() {
					fBytes += wi.Size()
				}
				return nil
			})
			if errRem := os.RemoveAll(p); errRem == nil {
				result.CleanedFolders = append(result.CleanedFolders, folder)
				totalFreed += fBytes
			}
		}
	}

	result.FreedSizeBytes = totalFreed
	result.FreedSizeStr = formatByteSize(totalFreed)
	return result, nil
}

// LaunchGameEngine launches the appropriate game engine editor for the repository
func (GitService) LaunchGameEngine(repoPath string, engineType string) (string, error) {
	switch engineType {
	case "unreal":
		ueInfo, err := GitService{}.DetectUnrealProject(repoPath)
		if err != nil || !ueInfo.IsUnrealProject {
			return "", errors.New("未检测到 Unreal Engine .uproject 工程文件")
		}
		if runtime.GOOS == "darwin" {
			_ = exec.Command("open", ueInfo.UProjectPath).Start()
		} else if runtime.GOOS == "windows" {
			_ = exec.Command("cmd", "/c", "start", "", ueInfo.UProjectPath).Start()
		}
		return fmt.Sprintf("已启动 Unreal Engine 打开工程 %s", ueInfo.ProjectName), nil

	case "unity":
		if runtime.GOOS == "darwin" {
			_ = exec.Command("open", "-a", "Unity", "--args", "-projectPath", repoPath).Start()
		} else if runtime.GOOS == "windows" {
			_ = exec.Command("cmd", "/c", "start", "", repoPath).Start()
		}
		return "已向 Unity Editor 发送工程打开指令", nil

	case "godot":
		godotFile := filepath.Join(repoPath, "project.godot")
		if runtime.GOOS == "darwin" {
			_ = exec.Command("open", godotFile).Start()
		} else if runtime.GOOS == "windows" {
			_ = exec.Command("cmd", "/c", "start", "", godotFile).Start()
		}
		return "已向 Godot Engine 发送工程打开指令", nil

	case "cocos":
		cocosFile := filepath.Join(repoPath, "project.json")
		if runtime.GOOS == "darwin" {
			_ = exec.Command("open", cocosFile).Start()
		} else if runtime.GOOS == "windows" {
			_ = exec.Command("cmd", "/c", "start", "", cocosFile).Start()
		}
		return "已向 Cocos Creator 发送工程打开指令", nil

	default:
		return "", fmt.Errorf("未知游戏引擎: %s", engineType)
	}
}

// LintGameTextures inspects game texture files for Power-of-Two (POT) dimensions and resolution limits
func (GitService) LintGameTextures(repoPath string, checkAll bool) (*TextureLintResult, error) {
	result := &TextureLintResult{
		Items: []TextureLintItem{},
	}

	imgExts := map[string]bool{
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".webp": true,
		".bmp":  true,
	}

	var candidates []string

	if !checkAll {
		// First check changed files
		out, err := ExecGit(repoPath, "status", "--porcelain")
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				if len(line) > 3 {
					fp := strings.TrimSpace(line[3:])
					ext := strings.ToLower(filepath.Ext(fp))
					if imgExts[ext] {
						candidates = append(candidates, fp)
					}
				}
			}
		}
	}

	// If no changed images or checkAll requested, scan working tree
	if len(candidates) == 0 {
		count := 0
		_ = filepath.Walk(repoPath, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				if fi != nil && fi.IsDir() {
					name := fi.Name()
					if name == ".git" || name == "node_modules" || name == "build" ||
						name == "Intermediate" || name == "Saved" || name == "Library" || name == "temp" {
						return filepath.SkipDir
					}
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if imgExts[ext] {
				rel, _ := filepath.Rel(repoPath, p)
				candidates = append(candidates, rel)
				count++
				if !checkAll && count >= 50 {
					return filepath.SkipAll
				}
			}
			return nil
		})
	}

	for _, relPath := range candidates {
		fullPath := filepath.Join(repoPath, relPath)
		fi, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		f, err := os.Open(fullPath)
		if err != nil {
			continue
		}

		cfg, format, err := image.DecodeConfig(f)
		_ = f.Close()
		if err != nil {
			continue
		}

		w, h := cfg.Width, cfg.Height
		isPot := isPowerOfTwo(w) && isPowerOfTwo(h)
		isOver := w > 4096 || h > 4096

		item := TextureLintItem{
			Path:        relPath,
			Width:       w,
			Height:      h,
			Dimensions:  fmt.Sprintf("%dx%d", w, h),
			Format:      strings.ToUpper(format),
			SizeBytes:   fi.Size(),
			SizeStr:     formatByteSize(fi.Size()),
			IsPOT:       isPot,
			IsOverSized: isOver,
		}

		if isOver {
			item.Severity = "ERROR"
			item.Issue = "超清超大贴图 (>4096px)"
			item.Advice = "容易导致移动端/主机端显存 (VRAM) 爆仓或闪退，建议在 DCC 工具中分块或下采样"
			result.TotalIssues++
		} else if !isPot {
			item.Severity = "WARNING"
			item.Issue = "非 2 的幂次方 (NPOT)"
			item.Advice = "无法生成硬件 Mipmap 并阻止 ASTC/ETC2 显卡硬件贴图压缩，建议调整宽高至 512, 1024, 2048 等尺寸"
			result.TotalIssues++
		} else {
			item.Severity = "OK"
			item.Issue = "符合规范"
			item.Advice = "标准 GPU 友好尺寸，支持硬件无损/有损纹理压缩与 Mipmap"
		}

		result.Items = append(result.Items, item)
		result.TotalChecked++
	}

	return result, nil
}

// InspectAssetReferences scans the game project to find all scenes, prefabs, materials, and scripts referencing the specified asset
func (GitService) InspectAssetReferences(repoPath string, targetPath string) (*AssetReferenceResult, error) {
	if targetPath == "" {
		return nil, errors.New("targetPath 不能为空")
	}
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	cleanTarget, err := repoRelativeAssetPath(targetPath)
	if err != nil {
		return nil, err
	}

	res := &AssetReferenceResult{
		TargetPath:   cleanTarget,
		References:   []AssetReferenceItem{},
		SafeToDelete: true,
		Warnings:     []string{},
	}

	const maxAssetScanWarnings = 8
	warningDetails := []string{}
	warningTotal := 0
	addWarning := func(msg string) {
		warningTotal++
		if len(warningDetails) < maxAssetScanWarnings {
			warningDetails = append(warningDetails, msg)
		}
	}

	// 1. Check for Unity .meta file
	fullTarget := filepath.Join(repoPath, cleanTarget)
	metaPath := fullTarget + ".meta"
	guid := ""
	if metaBytes, err := os.ReadFile(metaPath); err == nil {
		reGuid := regexp.MustCompile(`guid:\s*([a-f0-9]{32})`)
		matches := reGuid.FindStringSubmatch(string(metaBytes))
		if len(matches) > 1 {
			guid = matches[1]
			res.AssetGuid = guid
			res.EngineHint = "Unity 引擎资产 (检测到 GUID: " + guid[:8] + "...)"
		}
	} else if !os.IsNotExist(err) {
		addWarning(fmt.Sprintf("无法读取 %s.meta：%v；GUID 引用可能漏扫", cleanTarget, err))
	}

	targetBase := filepath.Base(cleanTarget)
	targetNameNoExt := strings.TrimSuffix(targetBase, filepath.Ext(targetBase))

	if res.EngineHint == "" {
		res.EngineHint = "通用游戏资产引用模式 (匹配路径及文件名: " + targetBase + ")"
	}

	// 2. Scan repository files for references
	searchExts := map[string]string{
		".prefab":             "Prefab (预制体)",
		".unity":              "Scene (场景)",
		".mat":                "Material (材质)",
		".asset":              "ScriptableAsset (数据资产)",
		".anim":               "Animation (动画)",
		".controller":         "Animator (动画控制器)",
		".overrideController": "Animator (覆盖控制器)",
		".playable":           "Timeline (时间轴)",
		".guiskin":            "GUI Skin (界面皮肤)",
		".shader":             "Shader (着色器)",
		".compute":            "Compute Shader",
		".umap":               "Unreal Level (虚幻关卡)",
		".uasset":             "Unreal Asset (虚幻资产)",
		".cs":                 "C# Script (脚本)",
		".ts":                 "TypeScript (脚本)",
		".js":                 "JavaScript (脚本)",
		".json":               "JSON (配置文件)",
		".yaml":               "YAML (配置文件)",
		".yml":                "YAML (配置文件)",
		".xml":                "XML (配置文件)",
		".ini":                "INI (配置文件)",
		".tscn":               "Godot Scene (场景)",
		".tres":               "Godot Resource (资源)",
		".fire":               "Cocos Scene (场景)",
	}

	skipDirs := map[string]bool{
		".git": true, "node_modules": true, ".vs": true, ".idea": true,
		"Library": true, "Temp": true, "obj": true, "bin": true, "build": true,
		"DerivedDataCache": true, "Intermediate": true, "Saved": true,
	}

	skippedLarge := 0
	err = filepath.Walk(repoPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if path == repoPath {
				return walkErr
			}
			rel, relErr := filepath.Rel(repoPath, path)
			if relErr != nil {
				rel = path
			}
			addWarning(fmt.Sprintf("无法访问 %s：%v", rel, walkErr))
			return nil
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		rel, relErr := filepath.Rel(repoPath, path)
		if relErr != nil {
			rel = path
		}
		if rel == cleanTarget || rel == cleanTarget+".meta" {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		category, ok := searchExts[ext]
		if !ok {
			return nil
		}

		// Read file up to 2MB to prevent memory exhaustion
		if info.Size() > 2*1024*1024 {
			skippedLarge++
			return nil
		}

		contentBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			addWarning(fmt.Sprintf("无法读取 %s：%v", rel, readErr))
			return nil
		}

		contentStr := string(contentBytes)
		matches := 0
		snippet := ""

		if guid != "" && strings.Contains(contentStr, guid) {
			matches = strings.Count(contentStr, guid)
			lines := strings.Split(contentStr, "\n")
			for _, l := range lines {
				if strings.Contains(l, guid) {
					snippet = strings.TrimSpace(l)
					if len(snippet) > 100 {
						snippet = snippet[:100] + "..."
					}
					break
				}
			}
		} else if strings.Contains(contentStr, cleanTarget) || strings.Contains(contentStr, targetBase) {
			matches = strings.Count(contentStr, cleanTarget)
			if matches == 0 {
				matches = strings.Count(contentStr, targetBase)
			}
			lines := strings.Split(contentStr, "\n")
			for _, l := range lines {
				if strings.Contains(l, targetBase) {
					snippet = strings.TrimSpace(l)
					if len(snippet) > 100 {
						snippet = snippet[:100] + "..."
					}
					break
				}
			}
		}

		if matches > 0 {
			res.References = append(res.References, AssetReferenceItem{
				FilePath:       rel,
				FileType:       category,
				MatchCount:     matches,
				PreviewSnippet: snippet,
			})
			res.ReferenceCount += matches
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if skippedLarge > 0 {
		addWarning(fmt.Sprintf("%d 个候选文件超过 2MB，未扫描，可能漏掉引用", skippedLarge))
	}
	if warningTotal > len(warningDetails) {
		warningDetails = append(warningDetails, fmt.Sprintf("另有 %d 条扫描警告已省略", warningTotal-len(warningDetails)))
	}
	res.Warnings = warningDetails
	res.AnalysisComplete = warningTotal == 0

	switch {
	case res.ReferenceCount > 0:
		res.SafeToDelete = false
		res.Message = fmt.Sprintf("⚠️ 发现 %d 处资产直接引用（涉及 %d 个工程文件）！修改或删除可能破坏预制体/场景。", res.ReferenceCount, len(res.References))
		if !res.AnalysisComplete {
			res.Message += " 扫描不完整，实际引用可能更多。"
		}
	case res.AnalysisComplete:
		res.SafeToDelete = true
		res.Message = fmt.Sprintf("✅ 巡检完成：未发现对 %s (%s) 的直接引用，可安全重命名、移动或删除。", targetBase, targetNameNoExt)
	default:
		res.SafeToDelete = false
		res.Message = fmt.Sprintf("⚠️ 引用扫描不完整：有文件无法读取或超出扫描上限，不能判定 %s 可安全删除。", targetBase)
	}

	return res, nil
}

// InspectAudioFile reads game audio/sfx metadata (sample rate, channels, bit depth, duration) and encodes playable preview
func (GitService) InspectAudioFile(repoPath string, targetPath string) (*AudioInspectResult, error) {
	if targetPath == "" {
		return nil, errors.New("targetPath 不能为空")
	}

	cleanTarget, err := repoRelativeAssetPath(targetPath)
	if err != nil {
		return nil, err
	}
	fullPath := filepath.Join(repoPath, cleanTarget)

	fi, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("音频资产不存在: %s", cleanTarget)
	}

	ext := strings.ToLower(filepath.Ext(cleanTarget))
	sz := fi.Size()
	szStr := formatAssetSize(sz)

	res := &AudioInspectResult{
		Path:          cleanTarget,
		Format:        strings.ToUpper(strings.TrimPrefix(ext, ".")),
		SizeBytes:     sz,
		SizeStr:       szStr,
		Warnings:      []string{},
		IsStandard48k: false,
		MetadataKnown: false,
		ChannelStr:    "未知",
		DurationStr:   "--",
	}

	// 1. WAV specific header decoding
	if ext == ".wav" {
		header := make([]byte, 128)
		f, openErr := os.Open(fullPath)
		if openErr == nil {
			n, _ := f.Read(header)
			_ = f.Close()
			if n >= 44 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WAVE" {
				// Parse fmt chunk
				fmtIdx := bytes.Index(header[:n], []byte("fmt "))
				if fmtIdx != -1 && fmtIdx+24 <= n {
					channels := int(uint16(header[fmtIdx+10]) | uint16(header[fmtIdx+11])<<8)
					sampleRate := int(uint32(header[fmtIdx+12]) | uint32(header[fmtIdx+13])<<8 | uint32(header[fmtIdx+14])<<16 | uint32(header[fmtIdx+15])<<24)
					byteRate := int(uint32(header[fmtIdx+16]) | uint32(header[fmtIdx+17])<<8 | uint32(header[fmtIdx+18])<<16 | uint32(header[fmtIdx+19])<<24)
					bitsPerSample := int(uint16(header[fmtIdx+22]) | uint16(header[fmtIdx+23])<<8)

					if channels > 0 && sampleRate > 0 {
						res.MetadataKnown = true
						res.Channels = channels
						res.SampleRate = sampleRate
						res.BitsPerSample = bitsPerSample
						if channels == 1 {
							res.ChannelStr = "单声道 (Mono)"
						} else if channels == 2 {
							res.ChannelStr = "双声道 (Stereo)"
						} else {
							res.ChannelStr = fmt.Sprintf("%d 声道环绕声", channels)
						}
					}

					// Find data chunk
					dataIdx := bytes.Index(header[:n], []byte("data"))
					if dataIdx != -1 && dataIdx+8 <= n {
						dataLen := int64(uint32(header[dataIdx+4]) | uint32(header[dataIdx+5])<<8 | uint32(header[dataIdx+6])<<16 | uint32(header[dataIdx+7])<<24)
						if byteRate > 0 {
							durSec := float64(dataLen) / float64(byteRate)
							res.DurationSec = durSec
							mins := int(durSec) / 60
							secs := int(durSec) % 60
							res.DurationStr = fmt.Sprintf("%02d:%02d (%.1fs)", mins, secs, durSec)
						}
					}
				}
			}
		}
	} else if ext == ".mp3" || ext == ".ogg" || ext == ".flac" {
		res.DurationStr = "压缩音频流"
	} else if ext == ".bank" {
		res.Format = "FMOD/Wwise 音频包"
		res.DurationStr = "复合 SoundBank 资产"
	}

	// 2. Studio Standards & Quality Verification
	if res.MetadataKnown {
		if res.SampleRate == 48000 {
			res.IsStandard48k = true
		} else if res.SampleRate == 44100 {
			res.Warnings = append(res.Warnings, "采样率为 44.1kHz (CD 标准)，建议统一重采样为 48.0kHz (游戏引擎工业标准)，避免运行时实时重采样损耗 CPU")
		} else {
			res.Warnings = append(res.Warnings, fmt.Sprintf("非常规采样率 (%d Hz)，建议转换为 48kHz 标准", res.SampleRate))
		}
	}

	if ext == ".wav" && sz > 15*1024*1024 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("未压缩巨型 WAV 文件 (%.1f MB)，建议转为 Ogg Vorbis/Opus 压缩格式或交由 SoundBank 打包", float64(sz)/(1024*1024)))
	}

	// 3. Audio Preview Data URI for playback (if <= 5MB)
	if sz <= 5*1024*1024 && (ext == ".wav" || ext == ".mp3" || ext == ".ogg") {
		audioData, rErr := os.ReadFile(fullPath)
		if rErr == nil {
			mime := "audio/wav"
			if ext == ".mp3" {
				mime = "audio/mpeg"
			} else if ext == ".ogg" {
				mime = "audio/ogg"
			}
			res.DataUri = fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(audioData))
		}
	}

	switch {
	case len(res.Warnings) > 0:
		res.Message = fmt.Sprintf("发现 %d 项音频规范优化建议（采样率/体积）", len(res.Warnings))
	case !res.MetadataKnown && ext == ".bank":
		res.Message = "SoundBank 复合资产，未解析内部采样率/声道，未作 48kHz 规范判定"
	case !res.MetadataKnown:
		res.Message = fmt.Sprintf("未能从 %s 直接解析采样率/声道，未作 48kHz 规范判定", strings.ToUpper(strings.TrimPrefix(ext, ".")))
	default:
		res.Message = "音频元数据符合游戏工作室研发规范（48kHz/标准格式）"
	}

	return res, nil
}

type meshGeometrySummary struct {
	triangles int
	vertices  int
	submeshes int
	materials int
	known     bool
}

func inspectMeshGeometry(path string, ext string) meshGeometrySummary {
	switch ext {
	case ".obj":
		return inspectOBJGeometry(path)
	case ".gltf":
		data, err := os.ReadFile(path)
		if err != nil {
			return meshGeometrySummary{}
		}
		return inspectGLTFGeometry(data)
	case ".glb":
		data, err := os.ReadFile(path)
		if err != nil {
			return meshGeometrySummary{}
		}
		chunk := glbJSONChunk(data)
		if chunk == nil {
			return meshGeometrySummary{}
		}
		return inspectGLTFGeometry(chunk)
	}
	return meshGeometrySummary{}
}

func inspectOBJGeometry(path string) meshGeometrySummary {
	f, err := os.Open(path)
	if err != nil {
		return meshGeometrySummary{}
	}
	defer f.Close()

	res := meshGeometrySummary{known: true}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	groups := map[string]bool{}
	materials := map[string]bool{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "v "):
			res.vertices++
		case strings.HasPrefix(line, "f "):
			parts := strings.Fields(line[2:])
			if len(parts) >= 3 {
				res.triangles += len(parts) - 2
			}
		case strings.HasPrefix(line, "o "), strings.HasPrefix(line, "g "):
			if name := strings.TrimSpace(line[2:]); name != "" {
				groups[name] = true
			}
		case strings.HasPrefix(line, "usemtl "):
			if material := strings.TrimSpace(line[7:]); material != "" {
				materials[material] = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return meshGeometrySummary{}
	}
	res.submeshes = 1
	if len(groups) > 0 {
		res.submeshes = len(groups)
	}
	res.materials = len(materials)
	return res
}

func inspectGLTFGeometry(data []byte) meshGeometrySummary {
	var doc gltfDocumentSummary
	if err := json.Unmarshal(bytes.TrimSpace(data), &doc); err != nil {
		return meshGeometrySummary{}
	}
	res := meshGeometrySummary{known: true}
	materials := map[int]bool{}
	for _, mesh := range doc.Meshes {
		for _, primitive := range mesh.Primitives {
			positionIndex, hasPosition := primitive.Attributes["POSITION"]
			if !hasPosition || positionIndex < 0 || positionIndex >= len(doc.Accessors) {
				return meshGeometrySummary{}
			}
			accessorIndex := positionIndex
			if primitive.Indices != nil {
				accessorIndex = *primitive.Indices
			}
			if accessorIndex < 0 || accessorIndex >= len(doc.Accessors) {
				return meshGeometrySummary{}
			}
			res.submeshes++
			if primitive.Material != nil {
				materials[*primitive.Material] = true
			}
			mode := 4
			if primitive.Mode != nil {
				mode = *primitive.Mode
			}
			res.vertices += doc.Accessors[positionIndex].Count
			if mode == 4 {
				if count := doc.Accessors[accessorIndex].Count; count >= 3 {
					res.triangles += count / 3
				}
			}
		}
	}
	res.materials = len(materials)
	return res
}

// LintModelMesh inspects 3D models (OBJ/glTF/GLB) for polygon count and checks against studio performance budgets
func (GitService) LintModelMesh(repoPath string, targetPath string, budgetPreset string) (*MeshLintResult, error) {
	if targetPath == "" {
		return nil, errors.New("targetPath 不能为空")
	}

	cleanTarget, err := repoRelativePathFromInput(repoPath, targetPath)
	if err != nil {
		return nil, err
	}
	fullPath := filepath.Join(repoPath, cleanTarget)

	fi, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("3D 模型资产不存在: %s", cleanTarget)
	}

	ext := strings.ToLower(filepath.Ext(cleanTarget))
	sz := fi.Size()
	szStr := formatAssetSize(sz)

	budgetLimit := 30000
	presetName := "移动端主角 (Mobile Character, 30,000 面)"
	switch budgetPreset {
	case "mobile_prop":
		budgetLimit = 5000
		presetName = "移动端道具/物件 (Mobile Prop, 5,000 面)"
	case "mobile_character":
		budgetLimit = 30000
		presetName = "移动端角色 (Mobile Character, 30,000 面)"
	case "pc_character":
		budgetLimit = 100000
		presetName = "PC/主机端角色 (PC Character, 100,000 面)"
	case "pc_environment":
		budgetLimit = 250000
		presetName = "PC/主机大场景 (PC Environment, 250,000 面)"
	}

	geometry := inspectMeshGeometry(fullPath, ext)
	res := &MeshLintResult{
		Path:               cleanTarget,
		Format:             strings.ToUpper(strings.TrimPrefix(ext, ".")),
		TriangleCount:      geometry.triangles,
		TriangleCountKnown: geometry.known,
		VertexCount:        geometry.vertices,
		SubmeshCount:       geometry.submeshes,
		MaterialCount:      geometry.materials,
		BudgetPreset:       presetName,
		BudgetLimit:        budgetLimit,
		SizeBytes:          sz,
		SizeStr:            szStr,
		Severity:           "OK",
		Warnings:           []string{},
	}

	if !geometry.known {
		res.Severity = "UNKNOWN"
		res.Advice = "该格式无法直接解析三角形/顶点数量（当前支持 OBJ / glTF / GLB）；请在 DCC 或引擎导入器中查看统计，本次未作预算判定。"
		return res, nil
	}

	// 2. Budget Evaluation
	if res.TriangleCount > budgetLimit {
		res.IsOverBudget = true
		res.Severity = "ERROR"
		excess := res.TriangleCount - budgetLimit
		pct := float64(excess) / float64(budgetLimit) * 100
		res.Advice = fmt.Sprintf("⚠️ 严重超标！当前多边形为 %d 三角面，超出 %s 预算上限（%d 面）达到 %.1f%%。极易造成渲染管线 Overdraw 与帧率骤降，请在 Maya/Blender/3dsMax 中进行减面优化或烘焙贴图。", res.TriangleCount, presetName, budgetLimit, pct)
		res.Warnings = append(res.Warnings, fmt.Sprintf("面数超标 %d 面", excess))
	} else if float64(res.TriangleCount) > float64(budgetLimit)*0.85 {
		res.Severity = "WARNING"
		res.Advice = fmt.Sprintf("⚡ 贴近预算警戒线！当前面数 %d 面，已达到预算上限（%d 面）的 85%% 以上，新增细分或附件可能引发超标。", res.TriangleCount, budgetLimit)
	} else {
		res.Severity = "OK"
		res.Advice = fmt.Sprintf("✅ 规范健康：当前多边形面数 %d 面，完全在 %s 预算范围（≤ %d 面）之内。", res.TriangleCount, presetName, budgetLimit)
	}

	return res, nil
}
