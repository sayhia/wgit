package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Release planning groups the pre-flight checks a studio needs before tagging a build.

// ReleaseCheck is one pre-flight result.
//
// Title/Detail carry the authoritative, already-rendered text so the payload is
// useful on its own (CLI, logs). TitleKey/DetailKey + Params let the desktop UI
// render the same sentence from its own catalog; the frontend prefers the key
// when the catalog has it and falls back to the plain text otherwise.
type ReleaseCheck struct {
	Key       string            `json:"key"`
	Title     string            `json:"title"`
	TitleKey  string            `json:"titleKey,omitempty"`
	Status    string            `json:"status"` // "pass" | "warn" | "fail"
	Detail    string            `json:"detail"`
	DetailKey string            `json:"detailKey,omitempty"`
	Params    map[string]string `json:"params,omitempty"`
	Blocking  bool              `json:"blocking"`
}

type ReleasePlan struct {
	Branch           string            `json:"branch"`
	LastTag          string            `json:"lastTag"`
	CommitCount      int               `json:"commitCount"`
	AheadOfRemote    int               `json:"aheadOfRemote"`
	BehindRemote     int               `json:"behindRemote"`
	SuggestedVersion string            `json:"suggestedVersion"`
	NextMinor        bool              `json:"nextMinor"`
	Checks           []ReleaseCheck    `json:"checks"`
	Changelog        string            `json:"changelog"`
	Ready            bool              `json:"ready"`
	Summary          string            `json:"summary"`
	SummaryKey       string            `json:"summaryKey,omitempty"`
	SummaryParams    map[string]string `json:"summaryParams,omitempty"`
}

type ReleaseCreateResult struct {
	Success    bool   `json:"success"`
	Tag        string `json:"tag"`
	Pushed     bool   `json:"pushed"`
	Message    string `json:"message"`
	PushOutput string `json:"pushOutput"`
}

var semverTagRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)
var releaseVersionRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// GetReleasePlan inspects the repo and proposes the next version.
func (s GitService) GetReleasePlan(repoPath string) (*ReleasePlan, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}

	plan := &ReleasePlan{Checks: []ReleaseCheck{}}

	branch, _ := ExecGit(repoPath, "branch", "--show-current")
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "HEAD (detached)"
	}
	plan.Branch = branch

	lastTag, _ := ExecGit(repoPath, "describe", "--tags", "--abbrev=0")
	lastTag = strings.TrimSpace(lastTag)
	plan.LastTag = lastTag

	rangeSpec := "HEAD"
	if lastTag != "" {
		rangeSpec = lastTag + "..HEAD"
	}
	countOut, countErr := ExecGit(repoPath, "rev-list", "--count", rangeSpec)
	count, _ := strconv.Atoi(strings.TrimSpace(countOut))
	plan.CommitCount = count

	var subjects []string
	if count > 0 {
		if logOut, err := ExecGit(repoPath, "log", "--max-count=200", "--format=%s", rangeSpec); err == nil {
			subjects = strings.Split(logOut, "\n")
		}
	}

	plan.SuggestedVersion, plan.NextMinor = suggestNextVersion(lastTag, subjects)

	// Check 1: is there anything to release?
	if countErr != nil || count == 0 {
		plan.Checks = append(plan.Checks, ReleaseCheck{
			Key: "commits", Title: "待发布提交", TitleKey: "release.check.commits.title",
			Status: "fail", Blocking: true,
			Detail:    "自上一版本以来没有新的提交，无法生成有意义的发布版本",
			DetailKey: "release.check.commits.detail.none",
		})
	} else {
		detail := fmt.Sprintf("自 %s 以来共 %d 个提交", orDefault(lastTag, "仓库初始提交"), count)
		plan.Checks = append(plan.Checks, ReleaseCheck{
			Key: "commits", Title: "待发布提交", TitleKey: "release.check.commits.title",
			Status: "pass", Detail: detail,
			DetailKey: "release.check.commits.detail.ok",
			Params:    map[string]string{"base": lastTag, "count": strconv.Itoa(count)},
		})
	}

	// Check 2: working tree.
	statusOut, _ := ExecGit(repoPath, "status", "--porcelain=v1")
	dirty := 0
	for _, line := range strings.Split(statusOut, "\n") {
		if strings.TrimSpace(line) != "" {
			dirty++
		}
	}
	if dirty > 0 {
		plan.Checks = append(plan.Checks, ReleaseCheck{
			Key: "workspace", Title: "工作区", TitleKey: "release.check.workspace.title",
			Status:    "warn",
			Detail:    fmt.Sprintf("有 %d 个未提交改动，打出的标签不会包含这些内容", dirty),
			DetailKey: "release.check.workspace.detail.dirty",
			Params:    map[string]string{"count": strconv.Itoa(dirty)},
		})
	} else {
		plan.Checks = append(plan.Checks, ReleaseCheck{
			Key: "workspace", Title: "工作区", TitleKey: "release.check.workspace.title",
			Status: "pass", Detail: "工作区干净，标签内容与工作区一致",
			DetailKey: "release.check.workspace.detail.clean",
		})
	}

	// Check 3: release branch.
	releaseBranch := branch == "main" || branch == "master" || strings.HasPrefix(branch, "release/") || strings.HasPrefix(branch, "hotfix/")
	if releaseBranch {
		plan.Checks = append(plan.Checks, ReleaseCheck{
			Key: "branch", Title: "发布分支", TitleKey: "release.check.branch.title",
			Status: "pass", Detail: "当前位于发布线路 " + branch,
			DetailKey: "release.check.branch.detail.ok",
			Params:    map[string]string{"branch": branch},
		})
	} else {
		plan.Checks = append(plan.Checks, ReleaseCheck{
			Key: "branch", Title: "发布分支", TitleKey: "release.check.branch.title",
			Status:    "warn",
			Detail:    fmt.Sprintf("当前在 %s 分支，请确认这是要对外发布的线路", branch),
			DetailKey: "release.check.branch.detail.other",
			Params:    map[string]string{"branch": branch},
		})
	}

	// Check 4: remote sync.
	upstream, upErr := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	upstream = strings.TrimSpace(upstream)
	if upErr != nil || upstream == "" {
		plan.Checks = append(plan.Checks, ReleaseCheck{
			Key: "remote", Title: "远端同步", TitleKey: "release.check.remote.title",
			Status:    "warn",
			Detail:    "当前分支未设置上游分支，创建标签后需要手动推送",
			DetailKey: "release.check.remote.detail.no.upstream",
		})
	} else {
		abOut, abErr := ExecGit(repoPath, "rev-list", "--left-right", "--count", upstream+"...HEAD")
		if abErr == nil {
			fields := strings.Fields(abOut)
			if len(fields) >= 2 {
				plan.BehindRemote, _ = strconv.Atoi(fields[0])
				plan.AheadOfRemote, _ = strconv.Atoi(fields[1])
			}
		}
		switch {
		case plan.BehindRemote > 0:
			plan.Checks = append(plan.Checks, ReleaseCheck{
				Key: "remote", Title: "远端同步", TitleKey: "release.check.remote.title",
				Status:    "warn",
				Detail:    fmt.Sprintf("远端 %s 还有 %d 个新提交，建议先拉取再发版", upstream, plan.BehindRemote),
				DetailKey: "release.check.remote.detail.behind",
				Params:    map[string]string{"upstream": upstream, "count": strconv.Itoa(plan.BehindRemote)},
			})
		case plan.AheadOfRemote > 0:
			plan.Checks = append(plan.Checks, ReleaseCheck{
				Key: "remote", Title: "远端同步", TitleKey: "release.check.remote.title",
				Status:    "pass",
				Detail:    fmt.Sprintf("本地领先 %s %d 个提交，发版后记得推送代码", upstream, plan.AheadOfRemote),
				DetailKey: "release.check.remote.detail.ahead",
				Params:    map[string]string{"upstream": upstream, "count": strconv.Itoa(plan.AheadOfRemote)},
			})
		default:
			plan.Checks = append(plan.Checks, ReleaseCheck{
				Key: "remote", Title: "远端同步", TitleKey: "release.check.remote.title",
				Status: "pass", Detail: "与 " + upstream + " 已同步",
				DetailKey: "release.check.remote.detail.synced",
				Params:    map[string]string{"upstream": upstream},
			})
		}
	}

	// Check 5: commit signing.
	if signing, err := s.GetCommitSigningStatus(repoPath); err == nil && signing.GpgSignEnabled {
		plan.Checks = append(plan.Checks, ReleaseCheck{
			Key: "signing", Title: "提交签名", TitleKey: "release.check.signing.title",
			Status: "pass", Detail: "已启用签名，发布来源可校验",
			DetailKey: "release.check.signing.detail.enabled",
		})
	} else {
		plan.Checks = append(plan.Checks, ReleaseCheck{
			Key: "signing", Title: "提交签名", TitleKey: "release.check.signing.title",
			Status: "warn", Detail: "未启用提交签名，发布来源不可校验",
			DetailKey: "release.check.signing.detail.disabled",
		})
	}

	// Changelog preview.
	if count > 0 {
		if changelog, err := s.GenerateStudioChangelog(repoPath, lastTag, "HEAD"); err == nil && changelog != nil {
			plan.Changelog = changelog.MarkdownDoc
		}
	}
	if plan.Changelog == "" {
		plan.Changelog = "（暂无可用于生成发版日志的提交）"
	}

	plan.Ready = true
	blocked := 0
	for _, check := range plan.Checks {
		if check.Blocking && check.Status == "fail" {
			plan.Ready = false
			blocked++
		}
	}
	if plan.Ready {
		plan.Summary = fmt.Sprintf("可以发布：自 %s 以来 %d 个提交，建议版本 %s。",
			orDefault(lastTag, "仓库初始提交"), plan.CommitCount, plan.SuggestedVersion)
		plan.SummaryKey = "release.summary.ready"
		plan.SummaryParams = map[string]string{
			"base":    lastTag,
			"count":   strconv.Itoa(plan.CommitCount),
			"version": plan.SuggestedVersion,
		}
	} else {
		plan.Summary = fmt.Sprintf("暂不可发布：有 %d 项阻塞检查未通过。", blocked)
		plan.SummaryKey = "release.summary.blocked"
		plan.SummaryParams = map[string]string{"count": strconv.Itoa(blocked)}
	}

	return plan, nil
}

// CreateRelease creates an annotated tag and optionally pushes it.
func (GitService) CreateRelease(repoPath string, version string, message string, push bool, remote string, sign bool) (*ReleaseCreateResult, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, errors.New("版本号不能为空")
	}
	if !releaseVersionRe.MatchString(version) {
		return nil, errors.New("版本号只能包含字母、数字、点、下划线、加号或连字符")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "-q", "--verify", "refs/tags/"+version); err == nil {
		return nil, fmt.Errorf("标签 %s 已存在，请换一个版本号", version)
	}

	message = strings.TrimSpace(message)
	if message == "" {
		message = "Release " + version
	}

	args := []string{"tag", "-a", version, "-m", message}
	if sign {
		args = []string{"tag", "-s", version, "-m", message}
	}
	if _, err := ExecGit(repoPath, args...); err != nil {
		if sign {
			return nil, fmt.Errorf("创建签名标签失败（请检查 GPG/SSH 签名配置）: %v", err)
		}
		return nil, err
	}

	res := &ReleaseCreateResult{
		Success: true,
		Tag:     version,
		Message: fmt.Sprintf("已创建标签 %s", version),
	}

	if push {
		remote = strings.TrimSpace(remote)
		if remote == "" {
			remote = defaultPushRemote(repoPath)
		}
		out, err := ExecGit(repoPath, "push", remote, "refs/tags/"+version)
		if err != nil {
			res.Message = fmt.Sprintf("标签 %s 已创建，但推送到 %s 失败：%v", version, remote, err)
			res.PushOutput = out
			return res, nil
		}
		res.Pushed = true
		res.PushOutput = out
		res.Message = fmt.Sprintf("已创建并推送标签 %s 到 %s", version, remote)
	}

	return res, nil
}

func suggestNextVersion(lastTag string, subjects []string) (string, bool) {
	minor := false
	for _, subject := range subjects {
		lower := strings.ToLower(strings.TrimSpace(subject))
		for _, prefix := range []string{"feat", "art", "level", "asset", "ui", "audio", "design"} {
			if strings.HasPrefix(lower, prefix+"(") || strings.HasPrefix(lower, prefix+":") || strings.HasPrefix(lower, prefix+"!") {
				minor = true
				break
			}
		}
		if minor {
			break
		}
	}

	if m := semverTagRe.FindStringSubmatch(strings.TrimSpace(lastTag)); m != nil {
		major, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		patch, _ := strconv.Atoi(m[3])
		prefix := ""
		if strings.HasPrefix(strings.TrimSpace(lastTag), "v") {
			prefix = "v"
		}
		if minor {
			return fmt.Sprintf("%s%d.%d.0", prefix, major, min+1), true
		}
		return fmt.Sprintf("%s%d.%d.%d", prefix, major, min, patch+1), false
	}

	return "v0.1.0", false
}

func orDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
