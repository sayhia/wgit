package main

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// =========================================================================
// 资产变更影响分析
//
// 「我改这张贴图会波及什么」是美术改图前最该问的问题，但 Git 本身答不了。
// InspectAssetReferences 已经能正向扫引用，这里补上：二级传播 + 责任人 +
// 按影响面排序，输出一份可以直接贴给主美的结论。
// =========================================================================

type AssetImpactRef struct {
	Path       string `json:"path"`
	FileType   string `json:"fileType"`
	MatchCount int    `json:"matchCount"`
	/** 1 = references the asset directly, 2 = references a referencer. */
	Depth      int    `json:"depth"`
	LastAuthor string `json:"lastAuthor"`
	LastTime   string `json:"lastTime"`
}

type AssetImpactResult struct {
	TargetPath    string           `json:"targetPath"`
	DirectCount   int              `json:"directCount"`
	IndirectCount int              `json:"indirectCount"`
	TotalCount    int              `json:"totalCount"`
	References    []AssetImpactRef `json:"references"`
	Owners        []string         `json:"owners"`
	RiskLevel     string           `json:"riskLevel"` // none | low | medium | high | unknown
	SafeToDelete  bool             `json:"safeToDelete"`
	Truncated     bool             `json:"truncated"`
	Message       string           `json:"message"`
	// Catalog key + params for `Message`. Conditional suffixes (truncation,
	// incomplete scan) are appended in order from `MessageSuffixKeys`, so the
	// backend stays the single source of truth for *which* notes apply.
	MessageKey        string            `json:"messageKey,omitempty"`
	MessageSuffixKeys []string          `json:"messageSuffixKeys,omitempty"`
	MessageParams     map[string]string `json:"messageParams,omitempty"`
	// False when any scan hop failed or skipped files; counts are then a floor.
	AnalysisComplete bool     `json:"analysisComplete"`
	Warnings         []string `json:"warnings"`
}

// Second-level expansion is O(直接引用数) full-repo greps, so cap it hard.
const assetImpactSecondLevelLimit = 20

// AnalyzeAssetImpact walks references one level deep and attributes each hit
// to the person who last touched it.
func (s GitService) AnalyzeAssetImpact(repoPath string, targetPath string, depth int) (*AssetImpactResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if strings.TrimSpace(targetPath) == "" {
		return nil, errors.New("资产路径不能为空")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}
	if depth <= 0 {
		depth = 2
	}
	if depth > 2 {
		depth = 2
	}

	direct, err := s.InspectAssetReferences(repoPath, targetPath)
	if err != nil {
		return nil, err
	}

	res := &AssetImpactResult{
		TargetPath:       targetPath,
		References:       []AssetImpactRef{},
		Owners:           []string{},
		SafeToDelete:     direct.SafeToDelete,
		AnalysisComplete: direct.AnalysisComplete,
		Warnings:         []string{},
	}

	const maxImpactWarnings = 8
	warningSeen := map[string]bool{}
	warningTotal := 0
	addWarning := func(msg string) {
		msg = strings.TrimSpace(msg)
		if msg == "" || warningSeen[msg] {
			return
		}
		warningSeen[msg] = true
		warningTotal++
		if len(res.Warnings) < maxImpactWarnings {
			res.Warnings = append(res.Warnings, msg)
		}
	}
	for _, warning := range direct.Warnings {
		addWarning(warning)
	}

	seen := map[string]bool{}
	for _, ref := range direct.References {
		if seen[ref.FilePath] {
			continue
		}
		seen[ref.FilePath] = true
		res.References = append(res.References, AssetImpactRef{
			Path:       ref.FilePath,
			FileType:   ref.FileType,
			MatchCount: ref.MatchCount,
			Depth:      1,
		})
	}
	res.DirectCount = len(res.References)

	// Level 2: files that reference the files above. Capped, because each hop
	// is another full-repo scan.
	if depth >= 2 && res.DirectCount > 0 {
		firstLevel := make([]string, 0, len(res.References))
		for _, ref := range res.References {
			firstLevel = append(firstLevel, ref.Path)
		}
		if len(firstLevel) > assetImpactSecondLevelLimit {
			firstLevel = firstLevel[:assetImpactSecondLevelLimit]
			res.Truncated = true
		}

		for _, parent := range firstLevel {
			next, err := s.InspectAssetReferences(repoPath, parent)
			if err != nil {
				res.AnalysisComplete = false
				addWarning(fmt.Sprintf("二级引用扫描失败：%s（%v），其下游影响未知", parent, err))
				continue
			}
			if !next.AnalysisComplete {
				res.AnalysisComplete = false
				for _, warning := range next.Warnings {
					addWarning(warning)
				}
			}
			for _, ref := range next.References {
				if seen[ref.FilePath] || ref.FilePath == targetPath {
					continue
				}
				seen[ref.FilePath] = true
				res.References = append(res.References, AssetImpactRef{
					Path:       ref.FilePath,
					FileType:   ref.FileType,
					MatchCount: ref.MatchCount,
					Depth:      2,
				})
			}
		}
	}
	if warningTotal > len(res.Warnings) {
		res.Warnings = append(res.Warnings, fmt.Sprintf("另有 %d 条扫描警告已省略", warningTotal-len(res.Warnings)))
	}
	if !res.AnalysisComplete {
		// Counts are a lower bound, so deletion must not be advertised as safe.
		res.SafeToDelete = false
	}
	res.IndirectCount = len(res.References) - res.DirectCount
	res.TotalCount = len(res.References)

	// Who owns the blast radius.
	ownerSet := map[string]bool{}
	for i := range res.References {
		meta, metaErr := ExecGit(repoPath, "log", "-1", "--format=%an%x09%ar", "--", res.References[i].Path)
		if metaErr != nil {
			addWarning(fmt.Sprintf("无法读取 %s 的最后修改者：%s", res.References[i].Path, firstLine(metaErr.Error())))
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(meta), "\t", 2)
		if len(parts) == 2 {
			res.References[i].LastAuthor = strings.TrimSpace(parts[0])
			res.References[i].LastTime = strings.TrimSpace(parts[1])
			if res.References[i].LastAuthor != "" {
				ownerSet[res.References[i].LastAuthor] = true
			}
		}
	}
	for name := range ownerSet {
		res.Owners = append(res.Owners, name)
	}
	sort.Strings(res.Owners)

	// Direct references first, then by how many times the asset is used.
	sort.SliceStable(res.References, func(i, j int) bool {
		if res.References[i].Depth != res.References[j].Depth {
			return res.References[i].Depth < res.References[j].Depth
		}
		return res.References[i].MatchCount > res.References[j].MatchCount
	})

	res.MessageParams = map[string]string{
		"total":    strconv.Itoa(res.TotalCount),
		"direct":   strconv.Itoa(res.DirectCount),
		"indirect": strconv.Itoa(res.IndirectCount),
		"owners":   strconv.Itoa(len(res.Owners)),
		"limit":    strconv.Itoa(assetImpactSecondLevelLimit),
	}
	switch {
	case res.TotalCount == 0 && !res.AnalysisComplete:
		res.RiskLevel = "unknown"
		res.Message = "引用扫描不完整，无法判断影响面；请先处理下方警告，再决定是否改名或删除。"
		res.MessageKey = "assets.impact.message.incomplete"
	case res.TotalCount == 0:
		res.RiskLevel = "none"
		res.Message = "没有任何文件引用该资产，删除或改名不会影响其他内容。"
		res.MessageKey = "assets.impact.message.none"
	case res.TotalCount <= 3:
		res.RiskLevel = "low"
		res.Message = fmt.Sprintf("共 %d 处引用（%d 直接 / %d 间接），影响面很小。",
			res.TotalCount, res.DirectCount, res.IndirectCount)
		res.MessageKey = "assets.impact.message.low"
	case res.TotalCount <= 10:
		res.RiskLevel = "medium"
		res.Message = fmt.Sprintf("%d 处引用（%d 直接 / %d 间接），涉及 %d 位负责人，改名前建议先同步。",
			res.TotalCount, res.DirectCount, res.IndirectCount, len(res.Owners))
		res.MessageKey = "assets.impact.message.medium"
	default:
		res.RiskLevel = "high"
		res.Message = fmt.Sprintf("%d 处引用（%d 直接 / %d 间接），涉及 %d 位负责人。这是高影响资产，建议只改内容不改路径。",
			res.TotalCount, res.DirectCount, res.IndirectCount, len(res.Owners))
		res.MessageKey = "assets.impact.message.high"
	}
	if res.Truncated {
		res.Message += fmt.Sprintf(" 二级引用已按前 %d 个直接引用截断。", assetImpactSecondLevelLimit)
		res.MessageSuffixKeys = append(res.MessageSuffixKeys, "assets.impact.message.truncated")
	}
	if !res.AnalysisComplete && res.TotalCount > 0 {
		res.Message += " 扫描不完整，实际影响可能更多。"
		res.MessageSuffixKeys = append(res.MessageSuffixKeys, "assets.impact.message.incomplete.suffix")
	}
	return res, nil
}
