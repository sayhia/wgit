package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// AssetService 收拢引擎资产看板（方案 §3.7 阶段 3）：把资产扫描结果与 DCC 锁
// 状态合并为统一的资产条目，并在仓库内落一份最小状态机（.wgit/assets.json）。
//
// 复用选型（不复制既有实现，同 package main 直接调用）：
//   - 资产清单：git_engine_asset.go 没有整仓"已跟踪引擎资产"清单函数——其仓库级
//     扫描器要么引擎专属（CheckUnityMetaSync 只看 Unity .meta 配对、LintGameTextures
//     只看贴图且遍历整个工作区），要么是逐文件巡检（InspectAssetMetadata 每个文件
//     各起 1-2 个 git 进程，不可批量复用）。因此这里复用 GetProjectAllFiles（单次
//     `git ls-files` 的已跟踪文件扫描）+ assetKindOf（既有资产分类，非
//     image/audio/model 的 "binary" 视为非引擎资产）组合出清单。
//   - 锁状态：GetLfsLocks 一次性取整张锁表后内存匹配，绝不逐资产起进程。
//   - 路径安全：外部传入路径先过 git_engine_asset.go 的 repoRelativeAssetPath。
//   - 预算比：现有预算能力（AnalyzePackageSizeBudget 的平台包体预算是仓库级、
//     LintModelMesh 的预算是模型三角面级）都不提供"单资产体积预算"，故
//     BudgetRatio 恒为 0 并在看板 Warnings 中说明，待预算配置落地后接入。
type AssetService struct{}

// 资产状态机的四个合法状态。tracked 是默认态：git 内已跟踪且未被锁定，
// 也没有落盘的 review/approved 覆盖。
const (
	assetStateTracked  = "tracked"
	assetStateLocked   = "locked"
	assetStateReview   = "review"
	assetStateApproved = "approved"

	assetBoardDefaultLimit = 100
	assetBoardMaxLimit     = 500

	assetStatesDirName = ".wgit"
	assetStatesFile    = "assets.json"
)

// AssetItem 是看板上的一条资产。
type AssetItem struct {
	Path        string  `json:"path"`
	State       string  `json:"state"`
	Owner       string  `json:"owner"`
	SizeBytes   int64   `json:"sizeBytes"`
	BudgetRatio float64 `json:"budgetRatio"`
	UpdatedAt   string  `json:"updatedAt"`
}

// AssetBoard 是一次看板生成结果。Summary 按最终状态统计全量资产（不受 limit
// 截断影响），Items 受 limit 截断。
type AssetBoard struct {
	Items       []AssetItem    `json:"items"`
	Summary     map[string]int `json:"summary"`
	GeneratedAt string         `json:"generatedAt"`
	Warnings    []string       `json:"warnings"`
}

// assetStateRecord 是 .wgit/assets.json 中单条落盘状态。
type assetStateRecord struct {
	State     string `json:"state"`
	UpdatedAt string `json:"updatedAt"`
}

// assetStatesPath 仿 subtreeMetadataPath：仓库内固定相对路径的落盘文件。
func assetStatesPath(repoPath string) string {
	return filepath.Join(repoPath, assetStatesDirName, assetStatesFile)
}

// readAssetStates 读取落盘状态机。文件缺失视为空表；JSON 损坏时按"容错重建"
// 处理——返回空表与 corrupted=true，调用方据此提示，绝不因此失败。
func readAssetStates(repoPath string) (map[string]assetStateRecord, bool, error) {
	data, err := os.ReadFile(assetStatesPath(repoPath))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]assetStateRecord{}, false, nil
		}
		return nil, false, fmt.Errorf("读取资产状态文件失败: %w", err)
	}
	states := map[string]assetStateRecord{}
	if err := json.Unmarshal(data, &states); err != nil {
		return map[string]assetStateRecord{}, true, nil
	}
	return states, false, nil
}

// saveAssetStates 原子写盘：复用 git_commit_safety_rules.go 的
// writeFileAtomic（同目录临时文件 + rename）。rename 在同一文件系统内是
// 原子的，读方（GetAssetBoard / GetAssetStates）要么看到完整的旧文件、
// 要么看到完整的新文件，绝不会读到半截 JSON；写失败时旧状态原样保留。
func saveAssetStates(repoPath string, states map[string]assetStateRecord) error {
	if err := os.MkdirAll(filepath.Join(repoPath, assetStatesDirName), 0o755); err != nil {
		return fmt.Errorf("创建 %s 目录失败: %w", assetStatesDirName, err)
	}
	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(assetStatesPath(repoPath), data, 0o644)
}

// assetStatesMu 序列化对 .wgit/assets.json 的「读-改-写」周期。Wails 多窗口
// 会并发调用 SetAssetState：无锁时两个窗口各自读到旧表、各自整文件重写，
// 后写者会覆盖先写者的更新（丢更新）。读路径（GetAssetBoard/GetAssetStates）
// 无需持锁——saveAssetStates 的 tmp+rename 原子替换保证读方只会看到完整
// 的新旧两态之一。桌面单用户场景下包级互斥（而非按仓库分锁）已足够，且
// SetAssetState 内的 git ls-files 扫描本身通常远快于用户连续点击。
var assetStatesMu sync.Mutex

func isValidAssetState(state string) bool {
	switch state {
	case assetStateTracked, assetStateLocked, assetStateReview, assetStateApproved:
		return true
	}
	return false
}

// collectTrackedAssets 枚举仓库中已跟踪的引擎资产：一次 `git ls-files`
// （复用 GetProjectAllFiles）+ 既有 assetKindOf 分类过滤（"binary" 表示非
// 引擎资产）。返回 slash 相对路径 → 工作区文件大小；已跟踪但工作区缺失的
// 文件大小记 0，条目保留。
func collectTrackedAssets(repoPath string) (map[string]int64, error) {
	tracked, err := (GitService{}).GetProjectAllFiles(repoPath)
	if err != nil {
		return nil, fmt.Errorf("读取已跟踪文件失败: %w", err)
	}
	sizes := make(map[string]int64, len(tracked))
	for _, p := range tracked {
		if assetKindOf(p) == "binary" {
			continue
		}
		rel := filepath.ToSlash(filepath.Clean(p))
		if _, seen := sizes[rel]; seen {
			continue
		}
		var size int64
		if fi, statErr := os.Stat(filepath.Join(repoPath, filepath.FromSlash(rel))); statErr == nil && !fi.IsDir() {
			size = fi.Size()
		}
		sizes[rel] = size
	}
	return sizes, nil
}

// GetAssetBoard 生成资产看板：扫描已跟踪资产 → 套用落盘状态 → 被 DCC 锁
// 锁住的资产强制 State=locked（锁释放后自然回落到落盘状态）。limit 非正数
// 取默认 100，超过 500 截到 500。
func (AssetService) GetAssetBoard(repoPath string, limit int) (*AssetBoard, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repoPath 不能为空")
	}
	if limit <= 0 {
		limit = assetBoardDefaultLimit
	}
	if limit > assetBoardMaxLimit {
		limit = assetBoardMaxLimit
	}

	sizes, err := collectTrackedAssets(repoPath)
	if err != nil {
		return nil, err
	}

	// DCC 锁表：一次性读取后建立 path → lock 索引，内存匹配。
	lockByPath := make(map[string]LfsLockInfo)
	var warnings []string
	locks, lockErr := (GitService{}).GetLfsLocks(repoPath)
	if lockErr != nil {
		warnings = append(warnings, "读取 DCC 锁表失败，locked 状态可能缺失: "+lockErr.Error())
	} else {
		for _, l := range locks {
			lockByPath[filepath.ToSlash(filepath.Clean(l.Path))] = l
		}
	}

	states, corrupted, err := readAssetStates(repoPath)
	if err != nil {
		return nil, err
	}
	if corrupted {
		warnings = append(warnings, "资产状态文件 .wgit/assets.json 已损坏，已忽略并按默认状态重建")
	}
	warnings = append(warnings, "未配置资产体积预算，budgetRatio 暂为 0")

	paths := make([]string, 0, len(sizes))
	for p := range sizes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	summary := map[string]int{
		assetStateTracked:  0,
		assetStateLocked:   0,
		assetStateReview:   0,
		assetStateApproved: 0,
	}
	items := make([]AssetItem, 0, len(paths))
	for _, p := range paths {
		item := AssetItem{
			Path:      p,
			State:     assetStateTracked,
			SizeBytes: sizes[p],
		}
		if rec, ok := states[p]; ok && isValidAssetState(rec.State) {
			item.State = rec.State
			item.UpdatedAt = rec.UpdatedAt
		}
		// 锁定强制：真实 DCC 锁优先于落盘状态与 tracked 默认值。
		if lock, ok := lockByPath[p]; ok {
			item.State = assetStateLocked
			item.Owner = lock.Owner
			item.UpdatedAt = lock.LockedAt
		}
		summary[item.State]++
		items = append(items, item)
	}

	board := &AssetBoard{
		Items:       items,
		Summary:     summary,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Warnings:    warnings,
	}
	if len(board.Items) > limit {
		board.Items = board.Items[:limit]
	}
	return board, nil
}

// SetAssetState 把单个资产的审阅状态写入 .wgit/assets.json。state 必须是四个
// 合法状态之一；path 必须出现在当前扫描结果中（防随意写）。显式写 tracked 会
// 作为一条覆盖记录保留（与默认态等效），便于前端回显"最近一次操作"。
func (AssetService) SetAssetState(repoPath, path, state string) error {
	if strings.TrimSpace(repoPath) == "" {
		return errors.New("repoPath 不能为空")
	}
	if !isValidAssetState(state) {
		return fmt.Errorf("非法资产状态 %q，允许值: tracked/locked/review/approved", state)
	}
	cleanPath, err := repoRelativeAssetPath(path)
	if err != nil {
		return err
	}
	relPath := filepath.ToSlash(cleanPath)

	// 读-改-写整个临界区持锁：两个窗口并发 SetAssetState 时后写者不得覆盖
	// 先写者的状态（丢更新）。校验在锁外完成，锁内只做状态文件操作。
	assetStatesMu.Lock()
	defer assetStatesMu.Unlock()

	sizes, err := collectTrackedAssets(repoPath)
	if err != nil {
		return err
	}
	if _, ok := sizes[relPath]; !ok {
		return fmt.Errorf("路径 %s 不在资产扫描结果中，拒绝写入状态", relPath)
	}

	states, _, err := readAssetStates(repoPath)
	if err != nil {
		return err
	}
	// 损坏文件已在 readAssetStates 中容错为空表，这里整文件重写即完成重建。
	states[relPath] = assetStateRecord{State: state, UpdatedAt: time.Now().Format(time.RFC3339)}
	if err := saveAssetStates(repoPath, states); err != nil {
		recordStateChange("app.asset.state", fmt.Sprintf("%s → %s（落盘失败）", relPath, state), false)
		return err
	}
	recordStateChange("app.asset.state", fmt.Sprintf("%s → %s", relPath, state), true)
	return nil
}

// GetAssetStates 读取落盘的资产状态机（path → state）。文件缺失或损坏时返回
// 空表；损坏不报错，下一次 SetAssetState 会重写文件完成重建。repoPath 为空
// 会落到进程工作目录下的相对路径 .wgit/assets.json，必须显式拒绝。
func (AssetService) GetAssetStates(repoPath string) (map[string]string, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repoPath 不能为空")
	}
	states, _, err := readAssetStates(repoPath)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(states))
	for p, rec := range states {
		out[p] = rec.State
	}
	return out, nil
}
