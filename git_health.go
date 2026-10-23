package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Repo health check aggregates the individual diagnostics into one score.

type HealthDimension struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Icon   string `json:"icon"`
	Status string `json:"status"` // "pass" | "warn" | "fail"
	Score  int    `json:"score"`  // 0 - 100
	Detail string `json:"detail"`
	Action string `json:"action"` // UI hook: settings / fsck / ignore / lfs / signing / conflict
}

type RepoHealthReport struct {
	Score     int               `json:"score"`
	Grade     string            `json:"grade"`
	PassCount int               `json:"passCount"`
	WarnCount int               `json:"warnCount"`
	FailCount int               `json:"failCount"`
	Items     []HealthDimension `json:"items"`
	Summary   string            `json:"summary"`
	CheckedAt string            `json:"checkedAt"`
	RepoName  string            `json:"repoName"`
}

// RunRepoHealthCheck runs six lightweight diagnostics and folds them into one score.
func (s GitService) RunRepoHealthCheck(repoPath string) (*RepoHealthReport, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}

	report := &RepoHealthReport{
		Items:     []HealthDimension{},
		CheckedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	if abs, err := filepath.Abs(repoPath); err == nil {
		report.RepoName = filepath.Base(abs)
	} else {
		report.RepoName = filepath.Base(repoPath)
	}

	add := func(key, title, icon string, status string, score int, detail, action string) {
		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}
		report.Items = append(report.Items, HealthDimension{
			Key: key, Title: title, Icon: icon,
			Status: status, Score: score, Detail: detail, Action: action,
		})
		switch status {
		case "pass":
			report.PassCount++
		case "warn":
			report.WarnCount++
		default:
			report.FailCount++
		}
	}

	// 1. Author identity — without it every commit is attributed to nobody.
	userName, _ := ExecGit(repoPath, "config", "user.name")
	userEmail, _ := ExecGit(repoPath, "config", "user.email")
	userName = strings.TrimSpace(userName)
	userEmail = strings.TrimSpace(userEmail)
	if userName != "" && userEmail != "" {
		add("identity", "提交身份", "🪪", "pass", 100,
			fmt.Sprintf("%s <%s>，提交作者信息完整", userName, userEmail), "settings")
	} else {
		missing := []string{}
		if userName == "" {
			missing = append(missing, "user.name")
		}
		if userEmail == "" {
			missing = append(missing, "user.email")
		}
		add("identity", "提交身份", "🪪", "fail", 20,
			"缺少 "+strings.Join(missing, " / ")+"，提交将无法正确归属", "settings")
	}

	// 2. Object database connectivity — cheap fsck that still catches corruption.
	fsckOut, fsckErr := ExecGit(repoPath, "fsck", "--connectivity-only", "--no-progress", "--no-dangling")
	lowerOut := strings.ToLower(fsckOut)
	switch {
	case fsckErr != nil:
		msg := firstNonEmptyLine(fsckOut)
		if msg == "" && fsckErr != nil {
			msg = fsckErr.Error()
		}
		add("objects", "对象数据库", "🧬", "fail", 25, "git fsck 报告异常："+truncateRunes(msg, 120), "fsck")
	case strings.Contains(lowerOut, "error") || strings.Contains(lowerOut, "missing"):
		add("objects", "对象数据库", "🧬", "fail", 40, "检测到对象缺失或损坏，建议运行完整 fsck", "fsck")
	case strings.Contains(lowerOut, "warning"):
		add("objects", "对象数据库", "🧬", "warn", 75, "存在悬空对象或告警，可执行 gc 清理", "fsck")
	default:
		add("objects", "对象数据库", "🧬", "pass", 100, "对象完整性与连通性检查通过", "fsck")
	}

	// 3. Ignore rules & accidentally tracked build artifacts.
	if audit, err := s.AuditIgnoreRules(repoPath); err != nil {
		add("ignore", "忽略规则", "🚫", "warn", 60, "忽略规则体检失败："+err.Error(), "ignore")
	} else {
		status := "pass"
		switch {
		case audit.HealthScore < 60:
			status = "fail"
		case audit.HealthScore < 85:
			status = "warn"
		}
		detail := fmt.Sprintf("%s · 健康度 %d 分", audit.EngineType, audit.HealthScore)
		if audit.TrackedCount > 0 {
			detail += fmt.Sprintf("，%d 个构建产物被误跟踪", audit.TrackedCount)
		} else if len(audit.MissingRules) > 0 {
			detail += fmt.Sprintf("，建议补齐 %d 条规则", len(audit.MissingRules))
		} else {
			detail += "，关键忽略规则齐备"
		}
		add("ignore", "忽略规则", "🚫", status, audit.HealthScore, detail, "ignore")
	}

	// 4. Oversized files that bypass LFS.
	if files, err := s.ScanLargeFiles(repoPath, 10); err != nil {
		add("largefiles", "大文件与 LFS", "💾", "warn", 60, "大文件扫描失败："+err.Error(), "lfs")
	} else {
		var offenders []LargeFileInfo
		var totalBytes int64
		lfsCount := 0
		for _, f := range files {
			if f.IsLFS {
				lfsCount++
				continue
			}
			if f.SizeBytes >= 10*1024*1024 {
				offenders = append(offenders, f)
				totalBytes += f.SizeBytes
			}
		}
		switch {
		case len(offenders) == 0:
			add("largefiles", "大文件与 LFS", "💾", "pass", 100,
				fmt.Sprintf("未发现超过 10MB 的未托管大文件（LFS 已接管 %d 个资产）", lfsCount), "lfs")
		case len(offenders) <= 2:
			add("largefiles", "大文件与 LFS", "💾", "warn", 65,
				fmt.Sprintf("%d 个文件超过 10MB 且未走 LFS，共 %s：%s", len(offenders), formatBytes(totalBytes), topFileNames(offenders, 3)), "lfs")
		default:
			add("largefiles", "大文件与 LFS", "💾", "fail", 35,
				fmt.Sprintf("%d 个文件超过 10MB 且未走 LFS，共 %s：%s", len(offenders), formatBytes(totalBytes), topFileNames(offenders, 3)), "lfs")
		}
	}

	// 5. Commit signing.
	if signing, err := s.GetCommitSigningStatus(repoPath); err != nil {
		add("signing", "提交签名", "✍️", "warn", 60, "签名状态检查失败："+err.Error(), "signing")
	} else if signing.GpgSignEnabled {
		detail := fmt.Sprintf("已启用 %s 签名", strings.ToUpper(signing.GpgFormat))
		if signing.ComplianceRate != "" {
			detail += "，近期合规率 " + signing.ComplianceRate
		}
		add("signing", "提交签名", "✍️", "pass", 100, detail, "signing")
	} else {
		add("signing", "提交签名", "✍️", "warn", 65, "未启用提交签名，无法校验提交来源真实性", "signing")
	}

	// 6. Interrupted operations and conflicts.
	status, statusErr := s.GetRepoStatus(repoPath)
	gitDir, _ := ExecGit(repoPath, "rev-parse", "--git-dir")
	gitDir = strings.TrimSpace(gitDir)
	if gitDir != "" && !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoPath, gitDir)
	}
	inProgress := ""
	if gitDir != "" {
		for name, label := range map[string]string{
			"MERGE_HEAD":       "合并未完成",
			"rebase-merge":     "交互式变基未完成",
			"rebase-apply":     "变基未完成",
			"CHERRY_PICK_HEAD": "拣选未完成",
			"REVERT_HEAD":      "回滚未完成",
			"BISECT_LOG":       "二分排查进行中",
		} {
			if _, err := os.Stat(filepath.Join(gitDir, name)); err == nil {
				inProgress = label
				break
			}
		}
	}
	switch {
	case statusErr != nil:
		add("workspace", "工作区状态", "🧯", "warn", 60, "工作区状态读取失败："+statusErr.Error(), "conflict")
	case status != nil && len(status.ConflictedFiles) > 0:
		add("workspace", "工作区状态", "🧯", "fail", 25,
			fmt.Sprintf("%d 个文件存在未解决的冲突", len(status.ConflictedFiles)), "conflict")
	case inProgress != "":
		add("workspace", "工作区状态", "🧯", "warn", 55, inProgress+"，请完成或中止后再继续", "conflict")
	case status != nil && !status.IsClean:
		dirty := len(status.StagedFiles) + len(status.UnstagedFiles) + len(status.UntrackedFiles)
		add("workspace", "工作区状态", "🧯", "pass", 85,
			fmt.Sprintf("无冲突；工作区有 %d 个待处理改动", dirty), "conflict")
	default:
		add("workspace", "工作区状态", "🧯", "pass", 100, "工作区干净，无冲突或中断操作", "conflict")
	}

	// Weighted score: object integrity matters most, signing is advisory.
	weights := map[string]int{
		"identity":   10,
		"objects":    25,
		"ignore":     20,
		"largefiles": 20,
		"signing":    10,
		"workspace":  15,
	}
	total := 0
	for _, item := range report.Items {
		total += item.Score * weights[item.Key]
	}
	report.Score = total / 100
	switch {
	case report.Score >= 90:
		report.Grade = "A"
	case report.Score >= 75:
		report.Grade = "B"
	case report.Score >= 60:
		report.Grade = "C"
	default:
		report.Grade = "D"
	}

	issues := report.WarnCount + report.FailCount
	if issues == 0 {
		report.Summary = fmt.Sprintf("六项体检全部通过，仓库健康度 %d 分，可以放心开工。", report.Score)
	} else {
		report.Summary = fmt.Sprintf("六项体检完成：%d 项通过、%d 项待优化、%d 项异常，综合健康度 %d 分。",
			report.PassCount, report.WarnCount, report.FailCount, report.Score)
	}

	return report, nil
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func topFileNames(files []LargeFileInfo, limit int) string {
	names := make([]string, 0, limit)
	for i, f := range files {
		if i >= limit {
			break
		}
		names = append(names, f.Path)
	}
	out := strings.Join(names, "、")
	if len(files) > limit {
		out += fmt.Sprintf(" 等 %d 个", len(files))
	}
	return out
}
