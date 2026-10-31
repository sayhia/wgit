// git_disk_analysis.go — 自 git_service.go 抽离（磁盘与体积分析族），
// 见 docs/big-files-assessment.md。函数级搬移，零逻辑改动。

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GetRepoStorageBreakdown analyzes working copy file sizes grouped by extension
func (GitService) GetRepoStorageBreakdown(repoPath string) ([]StorageBreakdownItem, error) {
	extMap := make(map[string]*StorageBreakdownItem)

	// StorageBreakdownItem has no field for measurement caveats, so the paths
	// Walk reports as unreadable are only counted and logged here; otherwise
	// the returned sizes would be silently understated.
	unreadable := 0
	_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			unreadable++
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" || base == ".venv" || base == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			ext = "无后缀"
		}
		item, exists := extMap[ext]
		if !exists {
			item = &StorageBreakdownItem{Extension: ext}
			extMap[ext] = item
		}
		item.Count++
		item.SizeBytes += info.Size()
		return nil
	})
	if unreadable > 0 {
		log.Printf("[StorageBreakdown] %d paths unreadable during walk; reported sizes may be understated", unreadable)
	}

	var result []StorageBreakdownItem
	for _, it := range extMap {
		if it.SizeBytes < 1024 {
			it.SizeHuman = fmt.Sprintf("%d B", it.SizeBytes)
		} else if it.SizeBytes < 1024*1024 {
			it.SizeHuman = fmt.Sprintf("%.1f KB", float64(it.SizeBytes)/1024)
		} else {
			it.SizeHuman = fmt.Sprintf("%.2f MB", float64(it.SizeBytes)/(1024*1024))
		}
		result = append(result, *it)
	}

	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].SizeBytes < result[j].SizeBytes {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	if len(result) > 15 {
		result = result[:15]
	}
	return result, nil
}

// GetRepositoryDiskBreakdown provides a comprehensive breakdown of repository storage, git objects, LFS, and working tree
func (GitService) GetRepositoryDiskBreakdown(repoPath string) (*DiskBreakdownResult, error) {
	result := &DiskBreakdownResult{
		TopFolders:      []DiskFolderItem{},
		AssetCategories: []StorageBreakdownItem{},
		Recommendations: []string{},
	}

	var gitInternalSize int64
	var lfsStoreSize int64
	var workingTreeSize int64
	// Shared counter for both walks below (.git and working tree): every path
	// reported unreadable is surfaced in Recommendations instead of being
	// skipped silently.
	unreadablePaths := 0

	// 1. Walk .git directory
	gitDir := gitCommonDir(repoPath)
	if fi, err := os.Stat(gitDir); err == nil && fi.IsDir() {
		_ = filepath.Walk(gitDir, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				unreadablePaths++
				return nil
			}
			if info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(gitDir, p)
			if strings.HasPrefix(rel, "lfs") {
				lfsStoreSize += info.Size()
			} else {
				gitInternalSize += info.Size()
			}
			return nil
		})
	}

	// 2. Measure top root folders & file categories in working tree
	folderSizes := make(map[string]int64)
	catSizes := make(map[string]int64)
	catCounts := make(map[string]int)

	entries, err := os.ReadDir(repoPath)
	if err == nil {
		for _, e := range entries {
			if e.Name() == ".git" {
				continue
			}
			subPath := filepath.Join(repoPath, e.Name())
			if e.IsDir() {
				var subSize int64
				_ = filepath.Walk(subPath, func(p string, info os.FileInfo, err error) error {
					if err != nil {
						unreadablePaths++
						return nil
					}
					if info.IsDir() {
						return nil
					}
					sz := info.Size()
					subSize += sz
					workingTreeSize += sz

					ext := strings.ToLower(filepath.Ext(p))
					cat := classifyAssetCategory(ext)
					catSizes[cat] += sz
					catCounts[cat]++
					return nil
				})
				folderSizes[e.Name()] = subSize
			} else {
				if fi, err := e.Info(); err == nil {
					sz := fi.Size()
					workingTreeSize += sz
					ext := strings.ToLower(filepath.Ext(e.Name()))
					cat := classifyAssetCategory(ext)
					catSizes[cat] += sz
					catCounts[cat]++
				}
			}
		}
	}

	// Sort top folders
	type fItem struct {
		name string
		size int64
	}
	var fList []fItem
	for k, v := range folderSizes {
		fList = append(fList, fItem{name: k, size: v})
	}
	sort.Slice(fList, func(i, j int) bool {
		return fList[i].size > fList[j].size
	})

	for i, f := range fList {
		if i >= 6 {
			break
		}
		result.TopFolders = append(result.TopFolders, DiskFolderItem{
			Path:    f.name,
			Size:    f.size,
			SizeStr: formatByteSize(f.size),
		})
	}

	// Asset categories breakdown
	var catList []StorageBreakdownItem
	for cat, sz := range catSizes {
		catList = append(catList, StorageBreakdownItem{
			Extension: cat,
			Count:     catCounts[cat],
			SizeBytes: sz,
			SizeHuman: formatByteSize(sz),
		})
	}
	sort.Slice(catList, func(i, j int) bool {
		return catList[i].SizeBytes > catList[j].SizeBytes
	})
	result.AssetCategories = catList

	total := gitInternalSize + lfsStoreSize + workingTreeSize
	result.TotalSize = total
	result.TotalSizeStr = formatByteSize(total)
	result.WorkingTreeSize = workingTreeSize
	result.WorkingTreeStr = formatByteSize(workingTreeSize)
	result.GitInternalSize = gitInternalSize
	result.GitInternalStr = formatByteSize(gitInternalSize)
	result.LfsStoreSize = lfsStoreSize
	result.LfsStoreStr = formatByteSize(lfsStoreSize)

	// Engine caches
	var engineCacheTotal int64
	workflows, _ := GitService{}.DetectGameEngineWorkflows(repoPath)
	for _, wf := range workflows {
		engineCacheTotal += wf.CacheSizeBytes
	}
	result.EngineCacheSize = engineCacheTotal
	result.EngineCacheStr = formatByteSize(engineCacheTotal)

	// Recommendations
	if lfsStoreSize > 500*1024*1024 {
		result.Recommendations = append(result.Recommendations, fmt.Sprintf("LFS 本地缓存占用较多 (%s)，建议运行 LFS Prune 修剪历史版本资产", formatByteSize(lfsStoreSize)))
	}
	if engineCacheTotal > 300*1024*1024 {
		result.Recommendations = append(result.Recommendations, fmt.Sprintf("游戏引擎中间生成文件占用 %s，可在引擎工作流中一键安全清除", formatByteSize(engineCacheTotal)))
	}
	if gitInternalSize > 200*1024*1024 {
		result.Recommendations = append(result.Recommendations, "Git 版本库内部对象较多，建议运行 Git GC 执行松散对象打包与存储整理")
	}
	if unreadablePaths > 0 {
		result.Recommendations = append(result.Recommendations, fmt.Sprintf("有 %d 处路径读取失败，统计体积可能少算", unreadablePaths))
	}
	if len(result.Recommendations) == 0 {
		result.Recommendations = append(result.Recommendations, "当前代码仓库存储健康，未发现异常膨胀或大量未压缩资产。")
	}

	return result, nil
}

func classifyAssetCategory(ext string) string {
	switch ext {
	case ".fbx", ".obj", ".blend", ".max", ".ma", ".mb", ".3ds", ".dae":
		return "3D 模型与工程资产"
	case ".png", ".jpg", ".jpeg", ".tga", ".psd", ".exr", ".hdr", ".dds", ".bmp", ".tif", ".tiff", ".svg":
		return "纹理贴图与图像美术"
	case ".wav", ".mp3", ".ogg", ".flac", ".aiff", ".m4a":
		return "音频音效与音乐"
	case ".mp4", ".mov", ".avi", ".webm":
		return "动画与视频剪辑"
	case ".uasset", ".umap", ".unity", ".prefab", ".mat", ".asset", ".anim", ".controller":
		return "游戏引擎预制体与资产"
	case ".cpp", ".c", ".h", ".hpp", ".cs", ".ts", ".js", ".go", ".rs", ".lua", ".py", ".sh":
		return "源代码与核心逻辑"
	case ".json", ".yaml", ".yml", ".xml", ".csv", ".ini", ".txt", ".md":
		return "游戏数值配置与数据表"
	case ".dll", ".dylib", ".so", ".a", ".lib", ".exe", ".bin":
		return "二进制插件与依赖库"
	default:
		if ext == "" {
			return "无后缀其他文件"
		}
		return ext + " 格式文件"
	}
}
