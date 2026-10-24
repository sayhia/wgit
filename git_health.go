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

// Repo health check aggregates the individual diagnostics into one score.

type HealthDimension struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	// Catalog key for `title` (derived from Key: health.dim.<key>.title).
	TitleKey string `json:"titleKey,omitempty"`
	Icon     string `json:"icon"`
	Status   string `json:"status"` // "pass" | "warn" | "fail"
	Score    int    `json:"score"`  // 0 - 100
	Detail   string `json:"detail"`
	// Catalog key + params for `detail`; `detailSuffixKeys` holds the keys of
	// conditional notes appended after the base sentence, in order.
	DetailKey        string            `json:"detailKey,omitempty"`
	DetailSuffixKeys []string          `json:"detailSuffixKeys,omitempty"`
	Params           map[string]string `json:"params,omitempty"`
	Action           string            `json:"action"` // UI hook: settings / fsck / ignore / lfs / signing / conflict
}

// healthLoc carries the catalog keys for one dimension's detail line.
//
// The rendered `detail` stays authoritative for logs and the CLI; the key +
// params let the desktop UI render the same sentence in its own language.
type healthLoc struct {
	detailKey  string
	params     map[string]string
	suffixKeys []string
}

type RepoHealthReport struct {
	Score     int               `json:"score"`
	Grade     string            `json:"grade"`
	PassCount int               `json:"passCount"`
	WarnCount int               `json:"warnCount"`
	FailCount int               `json:"failCount"`
	Items     []HealthDimension `json:"items"`
	Summary   string            `json:"summary"`
	// Catalog key + params for `summary`.
	SummaryKey    string            `json:"summaryKey,omitempty"`
	SummaryParams map[string]string `json:"summaryParams,omitempty"`
	CheckedAt     string            `json:"checkedAt"`
	RepoName      string            `json:"repoName"`
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

	add := func(key, title, icon string, status string, score int, detail, action string, loc healthLoc) {
		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}
		report.Items = append(report.Items, HealthDimension{
			Key: key, Title: title, TitleKey: "health.dim." + key + ".title", Icon: icon,
			Status: status, Score: score, Detail: detail, Action: action,
			DetailKey: loc.detailKey, Params: loc.params, DetailSuffixKeys: loc.suffixKeys,
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
			fmt.Sprintf("%s <%s>，提交作者信息完整", userName, userEmail), "settings",
			healthLoc{
				detailKey: "health.identity.detail.ok",
				params:    map[string]string{"name": userName, "email": userEmail},
			})
	} else {
		missing := []string{}
		if userName == "" {
			missing = append(missing, "user.name")
		}
		if userEmail == "" {
			missing = append(missing, "user.email")
		}
		add("identity", "提交身份", "🪪", "fail", 20,
			"缺少 "+strings.Join(missing, " / ")+"，提交将无法正确归属", "settings",
			healthLoc{
				detailKey: "health.identity.detail.missing",
				params:    map[string]string{"missing": strings.Join(missing, " / ")},
			})
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
		add("objects", "对象数据库", "🧬", "fail", 25, "git fsck 报告异常："+truncateRunes(msg, 120), "fsck",
			healthLoc{
				detailKey: "health.objects.detail.fsck.error",
				params:    map[string]string{"error": truncateRunes(msg, 120)},
			})
	case strings.Contains(lowerOut, "error") || strings.Contains(lowerOut, "missing"):
		add("objects", "对象数据库", "🧬", "fail", 40, "检测到对象缺失或损坏，建议运行完整 fsck", "fsck",
			healthLoc{detailKey: "health.objects.detail.corrupt"})
	case strings.Contains(lowerOut, "warning"):
		add("objects", "对象数据库", "🧬", "warn", 75, "存在悬空对象或告警，可执行 gc 清理", "fsck",
			healthLoc{detailKey: "health.objects.detail.dangling"})
	default:
		add("objects", "对象数据库", "🧬", "pass", 100, "对象完整性与连通性检查通过", "fsck",
			healthLoc{detailKey: "health.objects.detail.ok"})
	}

	// 3. Ignore rules & accidentally tracked build artifacts.
	if audit, err := s.AuditIgnoreRules(repoPath); err != nil {
		add("ignore", "忽略规则", "🚫", "warn", 60, "忽略规则体检失败："+err.Error(), "ignore",
			healthLoc{
				detailKey: "health.ignore.detail.audit.failed",
				params:    map[string]string{"error": err.Error()},
			})
	} else {
		status := "pass"
		switch {
		case audit.HealthScore < 60:
			status = "fail"
		case audit.HealthScore < 85:
			status = "warn"
		}
		detail := fmt.Sprintf("%s · 健康度 %d 分", audit.EngineType, audit.HealthScore)
		loc := healthLoc{
			detailKey: "health.ignore.detail.base",
			params: map[string]string{
				"engine": audit.EngineType,
				"score":  strconv.Itoa(audit.HealthScore),
			},
		}
		if audit.TrackedCount > 0 {
			detail += fmt.Sprintf("，%d 个构建产物被误跟踪", audit.TrackedCount)
			loc.suffixKeys = []string{"health.ignore.detail.suffix.tracked"}
			loc.params["count"] = strconv.Itoa(audit.TrackedCount)
		} else if len(audit.MissingRules) > 0 {
			detail += fmt.Sprintf("，建议补齐 %d 条规则", len(audit.MissingRules))
			loc.suffixKeys = []string{"health.ignore.detail.suffix.missing"}
			loc.params["count"] = strconv.Itoa(len(audit.MissingRules))
		} else {
			detail += "，关键忽略规则齐备"
			loc.suffixKeys = []string{"health.ignore.detail.suffix.ok"}
		}
		add("ignore", "忽略规则", "🚫", status, audit.HealthScore, detail, "ignore", loc)
	}

	// 4. Oversized files that bypass LFS.
	if files, err := s.ScanLargeFiles(repoPath, 10); err != nil {
		add("largefiles", "大文件与 LFS", "💾", "warn", 60, "大文件扫描失败："+err.Error(), "lfs",
			healthLoc{
				detailKey: "health.largefiles.detail.scan.failed",
				params:    map[string]string{"error": err.Error()},
			})
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
				fmt.Sprintf("未发现超过 10MB 的未托管大文件（LFS 已接管 %d 个资产）", lfsCount), "lfs",
				healthLoc{
					detailKey: "health.largefiles.detail.ok",
					params:    map[string]string{"count": strconv.Itoa(lfsCount)},
				})
		case len(offenders) <= 2:
			add("largefiles", "大文件与 LFS", "💾", "warn", 65,
				fmt.Sprintf("%d 个文件超过 10MB 且未走 LFS，共 %s：%s", len(offenders), formatBytes(totalBytes), topFileNames(offenders, 3)), "lfs",
				healthLoc{
					detailKey: "health.largefiles.detail.offenders",
					params: map[string]string{
						"count": strconv.Itoa(len(offenders)),
						"size":  formatBytes(totalBytes),
						"top":   topFileNames(offenders, 3),
					},
				})
		default:
			add("largefiles", "大文件与 LFS", "💾", "fail", 35,
				fmt.Sprintf("%d 个文件超过 10MB 且未走 LFS，共 %s：%s", len(offenders), formatBytes(totalBytes), topFileNames(offenders, 3)), "lfs",
				healthLoc{
					detailKey: "health.largefiles.detail.offenders",
					params: map[string]string{
						"count": strconv.Itoa(len(offenders)),
						"size":  formatBytes(totalBytes),
						"top":   topFileNames(offenders, 3),
					},
				})
		}
	}

	// 5. Commit signing.
	if signing, err := s.GetCommitSigningStatus(repoPath); err != nil {
		add("signing", "提交签名", "✍️", "warn", 60, "签名状态检查失败："+err.Error(), "signing",
			healthLoc{
				detailKey: "health.signing.detail.check.failed",
				params:    map[string]string{"error": err.Error()},
			})
	} else if signing.GpgSignEnabled {
		detail := fmt.Sprintf("已启用 %s 签名", strings.ToUpper(signing.GpgFormat))
		loc := healthLoc{
			detailKey: "health.signing.detail.ok",
			params:    map[string]string{"format": strings.ToUpper(signing.GpgFormat)},
		}
		if signing.ComplianceRate != "" {
			detail += "，近期合规率 " + signing.ComplianceRate
			loc.suffixKeys = []string{"health.signing.detail.suffix.compliance"}
			loc.params["rate"] = signing.ComplianceRate
		}
		add("signing", "提交签名", "✍️", "pass", 100, detail, "signing", loc)
	} else {
		add("signing", "提交签名", "✍️", "warn", 65, "未启用提交签名，无法校验提交来源真实性", "signing",
			healthLoc{detailKey: "health.signing.detail.disabled"})
	}

	// 6. Interrupted operations and conflicts.
	status, statusErr := s.GetRepoStatus(repoPath)
	gitDir, _ := ExecGit(repoPath, "rev-parse", "--git-dir")
	gitDir = strings.TrimSpace(gitDir)
	if gitDir != "" && !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoPath, gitDir)
	}
	inProgress := ""
	inProgressOp := ""
	if gitDir != "" {
		// The label is the Chinese fallback; the key lets the UI localize it.
		for name, op := range map[string][2]string{
			"MERGE_HEAD":       {"合并未完成", "merge"},
			"rebase-merge":     {"交互式变基未完成", "rebase.interactive"},
			"rebase-apply":     {"变基未完成", "rebase"},
			"CHERRY_PICK_HEAD": {"拣选未完成", "cherry.pick"},
			"REVERT_HEAD":      {"回滚未完成", "revert"},
			"BISECT_LOG":       {"二分排查进行中", "bisect"},
		} {
			if _, err := os.Stat(filepath.Join(gitDir, name)); err == nil {
				inProgress = op[0]
				inProgressOp = op[1]
				break
			}
		}
	}
	switch {
	case statusErr != nil:
		add("workspace", "工作区状态", "🧯", "warn", 60, "工作区状态读取失败："+statusErr.Error(), "conflict",
			healthLoc{
				detailKey: "health.workspace.detail.read.failed",
				params:    map[string]string{"error": statusErr.Error()},
			})
	case status != nil && len(status.ConflictedFiles) > 0:
		add("workspace", "工作区状态", "🧯", "fail", 25,
			fmt.Sprintf("%d 个文件存在未解决的冲突", len(status.ConflictedFiles)), "conflict",
			healthLoc{
				detailKey: "health.workspace.detail.conflicts",
				params:    map[string]string{"count": strconv.Itoa(len(status.ConflictedFiles))},
			})
	case inProgress != "":
		add("workspace", "工作区状态", "🧯", "warn", 55, inProgress+"，请完成或中止后再继续", "conflict",
			healthLoc{
				detailKey: "health.workspace.detail.in.progress",
				// The UI resolves health.workspace.op.<operation>.
				params: map[string]string{"operation": inProgressOp},
			})
	case status != nil && !status.IsClean:
		dirty := len(status.StagedFiles) + len(status.UnstagedFiles) + len(status.UntrackedFiles)
		add("workspace", "工作区状态", "🧯", "pass", 85,
			fmt.Sprintf("无冲突；工作区有 %d 个待处理改动", dirty), "conflict",
			healthLoc{
				detailKey: "health.workspace.detail.dirty",
				params:    map[string]string{"count": strconv.Itoa(dirty)},
			})
	default:
		add("workspace", "工作区状态", "🧯", "pass", 100, "工作区干净，无冲突或中断操作", "conflict",
			healthLoc{detailKey: "health.workspace.detail.ok"})
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
		report.SummaryKey = "health.summary.ok"
		report.SummaryParams = map[string]string{"score": strconv.Itoa(report.Score)}
	} else {
		report.Summary = fmt.Sprintf("六项体检完成：%d 项通过、%d 项待优化、%d 项异常，综合健康度 %d 分。",
			report.PassCount, report.WarnCount, report.FailCount, report.Score)
		report.SummaryKey = "health.summary.mixed"
		report.SummaryParams = map[string]string{
			"pass":  strconv.Itoa(report.PassCount),
			"warn":  strconv.Itoa(report.WarnCount),
			"fail":  strconv.Itoa(report.FailCount),
			"score": strconv.Itoa(report.Score),
		}
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
	out := strings.Join(names, ", ")
	if len(files) > limit {
		out += " …"
	}
	return out
}
