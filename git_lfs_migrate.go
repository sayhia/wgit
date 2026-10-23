package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// =========================================================================
// LFS 迁移向导
//
// ScanHistoryLargeBlobs 能扫出历史大对象，但只能看。这里补上真正把历史
// 迁进 LFS 的路径（git lfs migrate import），并且强制走「预览 → 备份 →
// 执行 → 校验」四步，因为重写历史是不可逆的高危操作。
// =========================================================================

type LfsMigrateRow struct {
	Pattern string `json:"pattern"`
	Files   int    `json:"files"`
	Size    string `json:"size"`
	/** git-lfs reports per-row completion here, e.g. "100%". */
	Progress string `json:"progress"`
}

type LfsMigratePreview struct {
	Rows    []LfsMigrateRow `json:"rows"`
	Raw     string          `json:"raw"`
	Command string          `json:"command"`
	Message string          `json:"message"`
}

type LfsMigrateOptions struct {
	Patterns     []string `json:"patterns"`
	Everything   bool     `json:"everything"`
	RevRange     string   `json:"revRange"`
	CreateBundle bool     `json:"createBundle"`
}

type LfsMigrateResult struct {
	Success       bool   `json:"success"`
	RewrittenFrom string `json:"rewrittenFrom"`
	RewrittenTo   string `json:"rewrittenTo"`
	BundlePath    string `json:"bundlePath"`
	Output        string `json:"output"`
	Message       string `json:"message"`
}

func normalizeLfsPatterns(patterns []string) (string, []string, error) {
	clean := []string{}
	seen := map[string]bool{}
	for _, raw := range patterns {
		p := strings.TrimSpace(raw)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		clean = append(clean, p)
	}
	if len(clean) == 0 {
		return "", nil, errors.New("至少需要一个文件模式（例如 *.psd 或 *.uasset）")
	}
	return strings.Join(clean, ","), clean, nil
}

// lfsMigrateScopeArgs translates the wizard choice into git-lfs flags.
// `--include-ref` accepts one ref, not a revision range, and an empty scope
// must never silently widen a destructive rewrite to the whole repository.
func lfsMigrateScopeArgs(repoPath string, everything bool, revRange string) ([]string, error) {
	if everything {
		return []string{"--everything"}, nil
	}
	ref := strings.TrimSpace(revRange)
	if ref == "" {
		return nil, errors.New("关闭「全部历史」时必须指定一个分支或引用")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", ref+"^{commit}"); err != nil {
		return nil, fmt.Errorf("引用 %s 不存在或不可解析（--include-ref 只接受单个分支/引用，不支持 main..feature 范围）: %w", ref, err)
	}
	return []string{"--include-ref=" + ref}, nil
}

// PreviewLfsMigration asks git-lfs itself what would move. `migrate info` is
// read-only, so this is safe to run on every keystroke in the wizard.
func (GitService) PreviewLfsMigration(repoPath string, patterns []string, everything bool, revRange string) (*LfsMigratePreview, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if _, err := ExecGit(repoPath, "lfs", "version"); err != nil {
		return nil, errors.New("未检测到 git-lfs，请先安装：brew install git-lfs && git lfs install")
	}
	include, _, err := normalizeLfsPatterns(patterns)
	if err != nil {
		return nil, err
	}
	scopeArgs, err := lfsMigrateScopeArgs(repoPath, everything, revRange)
	if err != nil {
		return nil, err
	}

	args := append([]string{"lfs", "migrate", "info", "--include=" + include}, scopeArgs...)

	out, cmdErr := ExecGitWithTimeout(repoPath, 10*time.Minute, args...)
	res := &LfsMigratePreview{
		Rows:    []LfsMigrateRow{},
		Raw:     out,
		Command: "git " + strings.Join(args, " "),
	}
	if cmdErr != nil {
		detail := firstLine(out)
		if detail == "" {
			detail = cmdErr.Error()
		} else {
			detail += " (" + cmdErr.Error() + ")"
		}
		return nil, fmt.Errorf("预览失败: %s", detail)
	}

	var totalFiles int
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "migrate:") {
			continue
		}
		// Real format: "*.psd\t66 KB\t1/1 file\t100%"
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		files := 0
		// "1/1 file" — the denominator is the total matched by this pattern.
		if prog := strings.TrimSpace(parts[2]); prog != "" {
			if slash := strings.Index(prog, "/"); slash >= 0 {
				if fields := strings.Fields(prog[slash+1:]); len(fields) > 0 {
					files, _ = strconv.Atoi(fields[0])
				}
			}
		}
		row := LfsMigrateRow{
			Pattern:  strings.TrimSpace(parts[0]),
			Files:    files,
			Size:     strings.TrimSpace(parts[1]),
			Progress: strings.TrimSpace(parts[3]),
		}
		totalFiles += files
		res.Rows = append(res.Rows, row)
	}

	switch {
	case len(res.Rows) == 0:
		res.Message = "没有匹配到需要迁移的历史文件，换个模式试试（例如 *.psd、Content/**/*.uasset）。"
	case totalFiles == 0:
		res.Message = "匹配到的文件已经全部在 LFS 里，无需迁移。"
	default:
		res.Message = fmt.Sprintf("将影响 %d 个历史文件，分散在下方 %d 类模式中。迁移会重写提交，所有协作者需要重新 clone。", totalFiles, len(res.Rows))
	}
	return res, nil
}

// ExecuteLfsMigration rewrites history so the matching blobs become LFS
// pointers. It optionally writes a bundle first so the old history stays
// recoverable even after a force-push.
func (s GitService) ExecuteLfsMigration(repoPath string, opts LfsMigrateOptions) (*LfsMigrateResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}
	if _, err := ExecGit(repoPath, "lfs", "version"); err != nil {
		return nil, errors.New("未检测到 git-lfs，无法迁移")
	}
	include, _, err := normalizeLfsPatterns(opts.Patterns)
	if err != nil {
		return nil, err
	}
	scopeArgs, err := lfsMigrateScopeArgs(repoPath, opts.Everything, opts.RevRange)
	if err != nil {
		return nil, err
	}

	res := &LfsMigrateResult{}
	before, _ := ExecGit(repoPath, "rev-parse", "HEAD")
	res.RewrittenFrom = strings.TrimSpace(before)

	// 1. Safety net first: a bundle is the only cheap way back after a rewrite.
	if opts.CreateBundle {
		bundlePath := filepath.Join(gitCommonDir(repoPath), "wgit-prefetch-"+time.Now().Format("20060102-150405")+".bundle")
		if _, err := s.CreateGitBundle(repoPath, bundlePath, "--all"); err != nil {
			return nil, fmt.Errorf("创建回滚备份失败，已中止迁移: %w", err)
		}
		res.BundlePath = bundlePath
	}

	// 2. Rewrite.
	args := append([]string{"lfs", "migrate", "import", "--include=" + include, "--yes"}, scopeArgs...)
	out, cmdErr := ExecGitWithTimeout(repoPath, 60*time.Minute, args...)
	res.Output = out
	if cmdErr != nil {
		detail := strings.TrimSpace(out)
		if detail == "" {
			detail = cmdErr.Error()
		}
		res.Message = "迁移失败：" + firstLine(detail)
		return res, nil
	}

	after, afterErr := ExecGit(repoPath, "rev-parse", "HEAD")
	res.RewrittenTo = strings.TrimSpace(after)
	res.Success = true

	msg := "历史迁移完成，所有匹配的 blob 已重写为 LFS 指针。"
	if res.RewrittenFrom != "" && res.RewrittenTo != "" && res.RewrittenFrom != res.RewrittenTo {
		msg += fmt.Sprintf(" HEAD 已由 %s 变为 %s。", shortHash(res.RewrittenFrom), shortHash(res.RewrittenTo))
	}
	if afterErr != nil || res.RewrittenTo == "" {
		msg += " 但读取迁移后的 HEAD 失败，无法确认新提交哈希，请运行 git log 核对后再推送。"
	}
	msg += " 接下来需要 git push --force --all 与 --tags，并通知所有协作者重新 clone。"
	if res.BundlePath != "" {
		msg += " 回滚点：" + res.BundlePath
	}
	res.Message = msg
	return res, nil
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}
