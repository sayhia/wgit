package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// =========================================================================
// 修复中心：把散落在各页的「诊断」收敛成「可执行的修复」
//
// 设计约定：
//   - ScanFixCenter 只做快速、只读的聚合，绝不修改仓库；
//   - ApplyAuditFix 通过 fixId 分发到既有写操作，不自造新的 Git 语义；
//   - 写文件之前统一备份到 .git/wgit-fix-backup/<ts>/，保证可回滚。
// =========================================================================

// FixCenterItem is one aggregated finding.
//
// Title/Summary/FixLabel are the rendered Chinese text (useful for logs and for
// any consumer without a catalog). The *Key + Params fields let the desktop UI
// render the same sentence in the active language; the frontend prefers the key
// and falls back to the plain text.
type FixCenterItem struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	TitleKey    string            `json:"titleKey,omitempty"`
	Severity    string            `json:"severity"` // "pass" | "warn" | "fail"
	Count       int               `json:"count"`
	Summary     string            `json:"summary"`
	SummaryKey  string            `json:"summaryKey,omitempty"`
	Params      map[string]string `json:"params,omitempty"`
	Fixable     bool              `json:"fixable"`
	FixLabel    string            `json:"fixLabel"`
	FixLabelKey string            `json:"fixLabelKey,omitempty"`
	/** Targets the UI must pass back to ApplyAuditFix (files / lock paths). */
	Targets []string `json:"targets"`
	/** True when the user should review the target list before applying. */
	NeedsReview bool `json:"needsReview"`
}

type FixCenterResult struct {
	Items         []FixCenterItem   `json:"items"`
	TotalFixes    int               `json:"totalFixes"`
	ScannedAt     string            `json:"scannedAt"`
	Message       string            `json:"message"`
	MessageKey    string            `json:"messageKey,omitempty"`
	MessageParams map[string]string `json:"messageParams,omitempty"`
	// False when any diagnostic failed; "nothing to fix" is then unproven.
	AnalysisComplete bool     `json:"analysisComplete"`
	Warnings         []string `json:"warnings"`
}

type AuditFixResult struct {
	FixID     string   `json:"fixId"`
	Success   bool     `json:"success"`
	Applied   int      `json:"applied"`
	Skipped   int      `json:"skipped"`
	Failed    int      `json:"failed"`
	Details   []string `json:"details"`
	BackupDir string   `json:"backupDir"`
	Message   string   `json:"message"`
}

// ScanFixCenter aggregates the fast, read-only diagnostics that have a known
// remediation. Slow or repo-type-specific audits (Unity meta, engine quality)
// stay in their own pages.
func (s GitService) ScanFixCenter(repoPath string) (*FixCenterResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	res := &FixCenterResult{
		Items:            []FixCenterItem{},
		ScannedAt:        time.Now().Format("2006-01-02 15:04:05"),
		AnalysisComplete: true,
		Warnings:         []string{},
	}
	addWarning := func(msg string) {
		res.AnalysisComplete = false
		res.Warnings = append(res.Warnings, msg)
	}

	// 1. Staged secrets — highest severity, always worth surfacing first.
	findings, secretsErr := s.ScanStagedSecrets(repoPath)
	if secretsErr != nil {
		addWarning("暂存区敏感信息扫描失败：" + secretsErr.Error())
	} else if len(findings) > 0 {
		seen := map[string]bool{}
		targets := []string{}
		critical := 0
		for _, f := range findings {
			if strings.EqualFold(f.Severity, "CRITICAL") {
				critical++
			}
			if !seen[f.FilePath] {
				seen[f.FilePath] = true
				targets = append(targets, f.FilePath)
			}
		}
		severity := "warn"
		if critical > 0 {
			severity = "fail"
		}
		res.Items = append(res.Items, FixCenterItem{
			ID:         "secret-unstage",
			Title:      "暂存区敏感信息",
			TitleKey:   "fix.item.secret.title",
			Severity:   severity,
			Count:      len(findings),
			Summary:    fmt.Sprintf("%d 处命中分布在 %d 个文件（其中 %d 处为 CRITICAL），提交即泄露。", len(findings), len(targets), critical),
			SummaryKey: "fix.item.secret.summary",
			Params: map[string]string{
				"hits":     strconv.Itoa(len(findings)),
				"files":    strconv.Itoa(len(targets)),
				"critical": strconv.Itoa(critical),
			},
			Fixable:     true,
			FixLabel:    "取消暂存并加入忽略",
			FixLabelKey: "fix.item.secret.fix",
			Targets:     targets,
			NeedsReview: true,
		})
	}

	// 2. Staged large files bypassing LFS.
	files, largeErr := s.CheckStagedNonLfsLargeFiles(repoPath, 10)
	if largeErr != nil {
		addWarning("暂存区大文件扫描失败：" + largeErr.Error())
	} else if len(files) > 0 {
		targets := make([]string, 0, len(files))
		var total int64
		for _, f := range files {
			targets = append(targets, f.Path)
			total += f.SizeBytes
		}
		res.Items = append(res.Items, FixCenterItem{
			ID:          "staged-large-lfs",
			Title:       "暂存区未走 LFS 的大文件",
			TitleKey:    "fix.item.large.title",
			Severity:    "warn",
			Count:       len(files),
			Summary:     fmt.Sprintf("%d 个文件超过 10MB 且未由 LFS 跟踪，合计 %s。", len(files), formatBytes(total)),
			SummaryKey:  "fix.item.large.summary",
			Params:      map[string]string{"count": strconv.Itoa(len(files)), "size": formatBytes(total)},
			Fixable:     true,
			FixLabel:    "全部转为 LFS",
			FixLabelKey: "fix.item.large.fix",
			Targets:     targets,
		})
	}

	// 3. Zombie LFS locks blocking teammates.
	activity, lockErr := s.GetLfsLockActivity(repoPath)
	if lockErr != nil {
		addWarning("LFS 锁活动查询失败：" + lockErr.Error())
	} else if activity != nil && activity.ZombieCount > 0 {
		targets := []string{}
		owners := map[string]bool{}
		for _, l := range activity.Locks {
			if l.IsZombie {
				targets = append(targets, l.Path)
				if l.Owner != "" {
					owners[l.Owner] = true
				}
			}
		}
		res.Items = append(res.Items, FixCenterItem{
			ID:         "stale-locks",
			Title:      "僵尸 LFS 锁",
			TitleKey:   "fix.item.locks.title",
			Severity:   "warn",
			Count:      len(targets),
			Summary:    fmt.Sprintf("%d 把锁长期无活动，持有人：%s。", len(targets), strings.Join(keysOf(owners), "、")),
			SummaryKey: "fix.item.locks.summary",
			// Joined with a neutral separator so the same string reads fine in
			// both languages.
			Params:      map[string]string{"count": strconv.Itoa(len(targets)), "owners": strings.Join(keysOf(owners), ", ")},
			Fixable:     true,
			FixLabel:    "批量破除",
			FixLabelKey: "fix.item.locks.fix",
			Targets:     targets,
			NeedsReview: true,
		})
	}

	// 4. Ignore rules missing or tracking build artifacts.
	audit, ignoreErr := s.AuditIgnoreRules(repoPath)
	if ignoreErr != nil {
		addWarning("忽略规则体检失败：" + ignoreErr.Error())
	} else if audit != nil {
		problems := len(audit.MissingRules) + len(audit.TrackedArtifacts)
		if problems > 0 {
			severity := "warn"
			if audit.HealthScore < 60 {
				severity = "fail"
			}
			res.Items = append(res.Items, FixCenterItem{
				ID:       "ignore-rules",
				Title:    "忽略规则缺口",
				TitleKey: "fix.item.ignore.title",
				Severity: severity,
				Count:    problems,
				Summary: fmt.Sprintf("%s · 健康度 %d：缺 %d 条规则，%d 个构建产物被误跟踪。",
					audit.EngineType, audit.HealthScore, len(audit.MissingRules), len(audit.TrackedArtifacts)),
				SummaryKey: "fix.item.ignore.summary",
				Params: map[string]string{
					"engine":  audit.EngineType,
					"score":   strconv.Itoa(audit.HealthScore),
					"missing": strconv.Itoa(len(audit.MissingRules)),
					"tracked": strconv.Itoa(len(audit.TrackedArtifacts)),
				},
				Fixable:     true,
				FixLabel:    "补全规则并取消跟踪",
				FixLabelKey: "fix.item.ignore.fix",
			})
		}
	}

	for i := range res.Items {
		if res.Items[i].Targets == nil {
			res.Items[i].Targets = []string{}
		}
	}
	for _, item := range res.Items {
		if item.Fixable {
			res.TotalFixes++
		}
	}
	if res.TotalFixes == 0 {
		if res.AnalysisComplete {
			res.Message = "没有发现可自动修复的问题，仓库状态良好。"
			res.MessageKey = "fix.message.clean"
		} else {
			res.Message = "扫描不完整，暂未发现可修复项；请先处理下方告警后重试。"
			res.MessageKey = "fix.message.incomplete"
		}
	} else {
		res.Message = fmt.Sprintf("发现 %d 类可修复问题。改写 .gitignore/.gitattributes 的修复会先备份到 .git/wgit-fix-backup/。", res.TotalFixes)
		res.MessageKey = "fix.message.found"
		res.MessageParams = map[string]string{"count": strconv.Itoa(res.TotalFixes)}
	}
	return res, nil
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Deterministic output keeps the summary stable for tests and diffs.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// backupRepoFiles snapshots the given repo-relative paths into
// .git/wgit-fix-backup/<timestamp>/ before a fix rewrites them.
func backupRepoFiles(repoPath string, relPaths []string) (string, []string) {
	if len(relPaths) == 0 {
		return "", nil
	}
	// Nanosecond precision keeps back-to-back fixes from sharing (and
	// overwriting) the same snapshot directory.
	root := filepath.Join(gitCommonDir(repoPath), "wgit-fix-backup", time.Now().Format("20060102-150405.000000000"))
	backed := []string{}
	for _, rel := range relPaths {
		src := filepath.Join(repoPath, rel)
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			continue
		}
		backed = append(backed, rel)
	}
	if len(backed) == 0 {
		return "", nil
	}
	return root, backed
}

// ApplyAuditFix runs the remediation registered for fixID. It never invents
// new Git semantics: each branch delegates to the write API the corresponding
// page already uses, so behaviour stays identical whichever entry point the
// user picks.
func (s GitService) ApplyAuditFix(repoPath string, fixID string, targets []string) (*AuditFixResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	res := &AuditFixResult{FixID: fixID, Details: []string{}}

	// secret-unstage and staged-large-lfs also rewrite .gitignore /
	// .gitattributes, so all file-writing fixes share the same backup gate.
	switch fixID {
	case "secret-unstage", "staged-large-lfs", "ignore-rules":
		backupTargets := []string{".gitignore", ".gitattributes"}
		backupDir, backed := backupRepoFiles(repoPath, backupTargets)
		res.BackupDir = backupDir
		backedSet := make(map[string]bool, len(backed))
		for _, path := range backed {
			backedSet[path] = true
		}
		for _, rel := range backupTargets {
			_, statErr := os.Stat(filepath.Join(repoPath, rel))
			if statErr == nil && !backedSet[rel] {
				res.Failed = 1
				res.Message = "无法备份 " + rel + "，已中止修复"
				return res, nil
			}
			if statErr != nil && !os.IsNotExist(statErr) {
				res.Failed = 1
				res.Message = "无法检查 " + rel + "，已中止修复：" + statErr.Error()
				return res, nil
			}
		}
		if len(backed) > 0 {
			res.Details = append(res.Details, "已备份："+strings.Join(backed, "、"))
		}
	}

	switch fixID {
	case "secret-unstage":
		remediation, err := s.RemediateStagedSecrets(repoPath, targets, true, false)
		if err != nil {
			return nil, err
		}
		res.Applied = remediation.Unstaged
		res.Failed = remediation.Failed
		res.Details = remediation.Details
		res.Message = remediation.Message

	case "staged-large-lfs":
		converted, err := s.TrackStagedLargeFilesAsLfs(repoPath, targets, true)
		if err != nil {
			return nil, err
		}
		res.Applied = converted.Converted
		res.Skipped = converted.Skipped
		res.Failed = converted.Failed
		for _, it := range converted.Items {
			if it.Detail != "" {
				res.Details = append(res.Details, fmt.Sprintf("%s：%s", it.Path, it.Detail))
			}
		}
		res.Message = converted.Message

	case "stale-locks":
		unlock, err := s.UnlockStaleLfsLocks(repoPath, targets, true)
		if err != nil {
			return nil, err
		}
		res.Applied = unlock.Unlocked
		res.Failed = unlock.Failed
		for _, it := range unlock.Items {
			if it.Status != "unlocked" && it.Detail != "" {
				res.Details = append(res.Details, fmt.Sprintf("%s：%s", it.Path, it.Detail))
			}
		}
		res.Message = unlock.Message

	case "ignore-rules":
		audit, err := s.AuditIgnoreRules(repoPath)
		if err != nil {
			return nil, fmt.Errorf("忽略规则体检失败: %w", err)
		}
		if audit == nil {
			return nil, errors.New("忽略规则体检没有返回结果")
		}

		action, err := s.ApplyIgnoreSanitizer(repoPath, audit.EngineType, true)
		if err != nil {
			res.Failed = 1
			res.Message = "补全忽略规则失败：" + err.Error()
			return res, nil
		}
		if action != nil {
			res.Applied = len(action.RulesAdded)
			res.Skipped = len(audit.MissingRules) - len(action.RulesAdded)
			if len(action.UntrackedFiles) > 0 {
				res.Details = append(res.Details,
					fmt.Sprintf("取消跟踪 %d 个构建产物", len(action.UntrackedFiles)))
			}
		}
		res.Message = fmt.Sprintf("已补全 %d 条忽略规则。", res.Applied)

	default:
		return nil, fmt.Errorf("未知的修复项: %s", fixID)
	}

	res.Success = res.Failed == 0 && res.Applied > 0
	if res.Success && res.Message == "" {
		res.Message = fmt.Sprintf("已修复 %d 项。", res.Applied)
	}
	return res, nil
}

// -------------------------------------------------------------------------
// B2 · 暂存区密钥处置
// -------------------------------------------------------------------------

type SecretRemediationResult struct {
	Success       bool     `json:"success"`
	Unstaged      int      `json:"unstaged"`
	Failed        int      `json:"failed"`
	PatternsAdded []string `json:"patternsAdded"`
	AllowlistPath string   `json:"allowlistPath"`
	Details       []string `json:"details"`
	Message       string   `json:"message"`
}

// RemediateStagedSecrets pulls the offending files back out of the index and,
// optionally, adds them to .gitignore so the same mistake cannot be re-staged
// by a careless `git add -A`.
func (GitService) RemediateStagedSecrets(repoPath string, paths []string, writeGitignore bool, allowlist bool) (*SecretRemediationResult, error) {
	if err := ensureRole(RoleAdmin); err != nil {
		return nil, err
	}
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if len(paths) == 0 {
		return nil, errors.New("没有选择要处置的文件")
	}

	res := &SecretRemediationResult{Details: []string{}}
	seen := map[string]bool{}
	failedPatterns := 0

	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true

		if _, err := ExecGit(repoPath, "restore", "--staged", "--", path); err != nil {
			// A file that was never staged is not a failure worth blocking on.
			if strings.Contains(err.Error(), "did not match") || strings.Contains(err.Error(), "pathspec") {
				res.Details = append(res.Details, path+"：未在暂存区，已跳过")
				continue
			}
			res.Failed++
			res.Details = append(res.Details, path+"：取消暂存失败（"+firstLine(err.Error())+"）")
			continue
		}
		res.Unstaged++
	}

	if writeGitignore && res.Unstaged > 0 {
		for path := range seen {
			pattern := path
			// Directory-level secrets (.env, credentials/) are better ignored
			// by basename when the file lives at the repo root.
			if !strings.Contains(path, "/") {
				pattern = path
			}
			if err := appendGitignorePattern(repoPath, pattern); err == nil {
				res.PatternsAdded = append(res.PatternsAdded, pattern)
			} else {
				failedPatterns++
				res.Details = append(res.Details, path+"：写入 .gitignore 失败（"+firstLine(err.Error())+"）")
			}
		}
	}

	if allowlist && res.Unstaged > 0 {
		allowPath := filepath.Join(gitCommonDir(repoPath), "wgit-secret-allowlist")
		var lines []string
		for path := range seen {
			lines = append(lines, path)
		}
		sortStrings(lines)
		if err := os.WriteFile(allowPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); err == nil {
			res.AllowlistPath = allowPath
		}
	}

	res.Success = res.Failed == 0 && failedPatterns == 0 && res.Unstaged > 0
	switch {
	case res.Unstaged == 0:
		res.Message = "没有文件被移出暂存区，请确认这些文件确实已暂存。"
	case res.Failed > 0 || failedPatterns > 0:
		res.Message = fmt.Sprintf("已将 %d 个文件移出暂存区；%d 个取消暂存失败，%d 条忽略规则写入失败。",
			res.Unstaged, res.Failed, failedPatterns)
	case len(res.PatternsAdded) > 0:
		res.Message = fmt.Sprintf("已将 %d 个文件移出暂存区并加入 .gitignore，请轮换已泄露的密钥。", res.Unstaged)
	default:
		res.Message = fmt.Sprintf("已将 %d 个文件移出暂存区，请轮换已泄露的密钥。", res.Unstaged)
	}
	return res, nil
}

// appendGitignorePattern adds a pattern to .gitignore unless it is already
// present, keeping the file free of duplicates.
func appendGitignorePattern(repoPath, pattern string) error {
	gitignorePath := filepath.Join(repoPath, ".gitignore")
	existing, err := os.ReadFile(gitignorePath)
	if err == nil {
		for _, line := range strings.Split(string(existing), "\n") {
			if strings.TrimSpace(line) == pattern {
				return nil
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("读取 .gitignore 失败: %w", err)
	}
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	prefix := ""
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		prefix = "\n"
	}
	if len(existing) == 0 {
		if _, err = f.WriteString("# Added by wgit secret remediation\n"); err != nil {
			_ = f.Close()
			return err
		}
	}
	if _, err = f.WriteString(prefix + pattern + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
