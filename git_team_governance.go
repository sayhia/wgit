package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type GovernanceConfigItem struct {
	Key            string `json:"key"`
	Value          string `json:"value"`
	Scope          string `json:"scope"`  // "local", "global", "unset"
	Status         string `json:"status"` // "compliant", "warning", "missing"
	Description    string `json:"description"`
	Recommendation string `json:"recommendation"`
}

type TeamGovernanceReport struct {
	ComplianceScore        int                    `json:"complianceScore"`
	Configs                []GovernanceConfigItem `json:"configs"`
	MissingRecommendations []string               `json:"missingRecommendations"`
	Summary                string                 `json:"summary"`
}

type ApplyGovernancePresetResult struct {
	PresetType      string   `json:"presetType"`
	AppliedSettings []string `json:"appliedSettings"`
	Success         bool     `json:"success"`
	Message         string   `json:"message"`
}

func execGitGov(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func readConfigValue(repoPath string, key string) (val string, scope string) {
	// Try local first
	outL, errL := execGitGov(repoPath, "config", "--local", "--get", key)
	if errL == nil && strings.TrimSpace(outL) != "" {
		return strings.TrimSpace(outL), "local"
	}
	// Try global
	outG, errG := execGitGov(repoPath, "config", "--global", "--get", key)
	if errG == nil && strings.TrimSpace(outG) != "" {
		return strings.TrimSpace(outG), "global"
	}
	return "", "unset"
}

// InspectRepoGitConfigs diagnoses critical Git configuration items against team best practices.
func (GitService) InspectRepoGitConfigs(repoPath string) (*TeamGovernanceReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	trackedKeys := []struct {
		key      string
		desc     string
		recVal   string
		recLabel string
	}{
		{"core.autocrlf", "跨平台换行符自动转换策略", "input/true", "推荐 input (mac/Linux) 或 true (Windows)"},
		{"pull.rebase", "默认拉取策略 (Pull Rebase)", "true", "推荐开启 true 保持线性无分支分叉历史"},
		{"push.default", "推送行为匹配策略", "current/simple", "推荐 current 或 simple 防止跨分支误推送"},
		{"commit.gpgsign", "提交自动数字签名", "true", "高安全敏感团队推荐开启 GPG 提交签名校验"},
		{"user.name", "提交者姓名标识", "非空字符串", "必须配置合法的开发者姓名"},
		{"user.email", "提交者工作邮箱", "公司/企业邮箱", "必须配置统一的企业工作邮箱后缀"},
		{"core.hooksPath", "团队共享 Git Hooks 目录", ".githooks", "建议配置团队托管的一致性 Hooks 路径"},
	}

	configs := make([]GovernanceConfigItem, 0, len(trackedKeys))
	missingRecs := make([]string, 0)
	score := 100

	for _, item := range trackedKeys {
		val, scope := readConfigValue(repoPath, item.key)
		status := "compliant"

		switch item.key {
		case "core.autocrlf":
			if val == "" {
				status = "warning"
				score -= 15
				missingRecs = append(missingRecs, "未显式配置 core.autocrlf，多人协作易产生全量 CRLF/LF 换行噪音")
			}
		case "pull.rebase":
			if val == "" || val == "false" {
				status = "warning"
				score -= 10
				missingRecs = append(missingRecs, "建议开启 pull.rebase 保证团队分支历史线性整洁")
			}
		case "user.name":
			if val == "" {
				status = "missing"
				score -= 25
				missingRecs = append(missingRecs, "未配置 user.name，提交将使用系统匿名默认值")
			}
		case "user.email":
			if val == "" {
				status = "missing"
				score -= 25
				missingRecs = append(missingRecs, "未配置 user.email，无法正确追踪提交作者权属")
			}
		case "commit.gpgsign":
			if val != "true" {
				status = "warning"
				score -= 10
			}
		}

		configs = append(configs, GovernanceConfigItem{
			Key:            item.key,
			Value:          val,
			Scope:          scope,
			Status:         status,
			Description:    item.desc,
			Recommendation: item.recLabel,
		})
	}

	if score < 0 {
		score = 0
	}

	summary := fmt.Sprintf("Team Governance Compliance Score: %d/100 (%d items inspected)", score, len(configs))

	return &TeamGovernanceReport{
		ComplianceScore:        score,
		Configs:                configs,
		MissingRecommendations: missingRecs,
		Summary:                summary,
	}, nil
}

// ApplyRecommendedTeamPreset applies local configuration presets.
func (GitService) ApplyRecommendedTeamPreset(repoPath string, presetType string) (*ApplyGovernancePresetResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	presetType = strings.TrimSpace(strings.ToLower(presetType))
	var applied []string

	applyConfig := func(k, v string) error {
		_, err := execGitGov(repoPath, "config", "--local", k, v)
		if err == nil {
			applied = append(applied, fmt.Sprintf("%s=%s", k, v))
		}
		return err
	}

	switch presetType {
	case "cross-platform":
		crlf := "input"
		if runtime.GOOS == "windows" {
			crlf = "true"
		}
		_ = applyConfig("core.autocrlf", crlf)
		_ = applyConfig("core.eol", "lf")
		_ = applyConfig("pull.rebase", "true")
	case "security-strict":
		_ = applyConfig("commit.gpgsign", "true")
		_ = applyConfig("push.default", "simple")
	case "standard-dev":
		fallthrough
	default:
		crlf := "input"
		if runtime.GOOS == "windows" {
			crlf = "true"
		}
		_ = applyConfig("core.autocrlf", crlf)
		_ = applyConfig("pull.rebase", "true")
		_ = applyConfig("push.default", "current")
	}

	return &ApplyGovernancePresetResult{
		PresetType:      presetType,
		AppliedSettings: applied,
		Success:         true,
		Message:         fmt.Sprintf("Successfully applied %d settings for preset '%s'", len(applied), presetType),
	}, nil
}
