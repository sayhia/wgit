package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// =========================================================================
// 引擎工程一键 Bootstrap
//
// 新建或接手一个 Unity / Unreal 仓库时，该配的东西其实是固定的：
// .gitattributes 预设、.gitignore 规则、UnityYAMLMerge、LFS 跟踪范围。
// 这些零件早就都有了，缺的只是「一个向导串起来 + 复检打分」。
// =========================================================================

type EngineBootstrapStep struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"` // applied | skipped | failed
	Detail string `json:"detail"`
}

type EngineBootstrapOptions struct {
	/** Empty = auto-detect from the working tree. */
	Engine              string   `json:"engine"`
	WriteGitattributes  bool     `json:"writeGitattributes"`
	WriteGitignore      bool     `json:"writeGitignore"`
	UntrackArtifacts    bool     `json:"untrackArtifacts"`
	ConfigureUnityMerge bool     `json:"configureUnityMerge"`
	LfsExtensions       []string `json:"lfsExtensions"`
}

type EngineBootstrapResult struct {
	Engine    string                `json:"engine"`
	Success   bool                  `json:"success"`
	Applied   int                   `json:"applied"`
	Skipped   int                   `json:"skipped"`
	Failed    int                   `json:"failed"`
	Steps     []EngineBootstrapStep `json:"steps"`
	Readiness int                   `json:"readiness"`
	Message   string                `json:"message"`
}

// presetKeyForEngineName bridges the display names used by AuditIgnoreRules
// ("Unity 3D") and the lowercase keys used by the .gitattributes presets.
func presetKeyForEngineName(name string) string {
	switch name {
	case "Unity 3D":
		return "unity"
	case "Unreal Engine":
		return "unreal"
	case "Godot Engine":
		return "godot"
	case "Cocos Creator":
		return "cocos"
	}
	return ""
}

// mergeGitattributesFile appends only the missing non-comment lines, so a
// hand-maintained .gitattributes never gets clobbered.
func mergeGitattributesFile(repoPath string, preset string) (int, error) {
	path := filepath.Join(repoPath, ".gitattributes")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("读取 .gitattributes 失败: %w", err)
	}
	existingText := string(existing)

	if strings.TrimSpace(existingText) == "" {
		if err := os.WriteFile(path, []byte(strings.TrimSpace(preset)+"\n"), 0644); err != nil {
			return 0, err
		}
		// Count the meaningful lines we just wrote.
		added := 0
		for _, line := range strings.Split(preset, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				added++
			}
		}
		return added, nil
	}

	present := map[string]bool{}
	for _, line := range strings.Split(existingText, "\n") {
		present[strings.TrimSpace(line)] = true
	}

	var toAppend []string
	for _, line := range strings.Split(preset, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if present[trimmed] {
			continue
		}
		toAppend = append(toAppend, trimmed)
	}
	if len(toAppend) == 0 {
		return 0, nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}

	prefix := ""
	if !strings.HasSuffix(existingText, "\n") {
		prefix = "\n"
	}
	payload := prefix + "\n# Added by wgit engine bootstrap\n" + strings.Join(toAppend, "\n") + "\n"
	if _, err := f.WriteString(payload); err != nil {
		_ = f.Close()
		return 0, err
	}
	if err := f.Close(); err != nil {
		return 0, err
	}
	return len(toAppend), nil
}

// RunEngineBootstrap wires the existing presets into one idempotent pass.
func (s GitService) RunEngineBootstrap(repoPath string, opts EngineBootstrapOptions) (*EngineBootstrapResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	res := &EngineBootstrapResult{Steps: []EngineBootstrapStep{}}
	add := func(id, label, status, detail string) {
		res.Steps = append(res.Steps, EngineBootstrapStep{ID: id, Label: label, Status: status, Detail: detail})
		switch status {
		case "applied":
			res.Applied++
		case "skipped":
			res.Skipped++
		default:
			res.Failed++
		}
	}

	// Resolve the engine: explicit hint > detection > ignore-audit fallback.
	engine := strings.ToLower(strings.TrimSpace(opts.Engine))
	if engine == "" {
		if info, err := s.DetectGameEngineProject(repoPath); err == nil && info != nil {
			engine = strings.ToLower(info.RecommendedPreset)
		}
	}
	if engine == "" {
		if audit, err := s.AuditIgnoreRules(repoPath); err == nil && audit != nil {
			engine = presetKeyForEngineName(audit.EngineType)
		}
	}
	if engine == "" {
		engine = "generic"
	}
	res.Engine = engine

	// 1. .gitattributes preset
	if opts.WriteGitattributes {
		preset := s.GetGameEngineGitAttributesPreset(engine)
		if strings.TrimSpace(preset) == "" {
			add("gitattributes", "写入 .gitattributes 引擎预设", "skipped", "该引擎没有内置预设，可手动编辑")
		} else if added, err := mergeGitattributesFile(repoPath, preset); err != nil {
			add("gitattributes", "写入 .gitattributes 引擎预设", "failed", err.Error())
		} else if added == 0 {
			add("gitattributes", "写入 .gitattributes 引擎预设", "skipped", "预设条目已全部存在")
		} else {
			add("gitattributes", "写入 .gitattributes 引擎预设", "applied", fmt.Sprintf("补充 %d 条规则（LFS / 合并驱动 / 换行）", added))
		}
	} else {
		add("gitattributes", "写入 .gitattributes 引擎预设", "skipped", "未勾选")
	}

	// 2. .gitignore rules (+ optional untrack of already-committed artifacts)
	if opts.WriteGitignore {
		audit, err := s.AuditIgnoreRules(repoPath)
		if err != nil || audit == nil {
			add("gitignore", "补全 .gitignore 规则", "failed", "体检失败，无法确定引擎类型")
		} else if len(audit.MissingRules) == 0 && len(audit.TrackedArtifacts) == 0 {
			add("gitignore", "补全 .gitignore 规则", "skipped", "规则已齐备")
		} else {
			action, err := s.ApplyIgnoreSanitizer(repoPath, audit.EngineType, opts.UntrackArtifacts)
			switch {
			case err != nil:
				add("gitignore", "补全 .gitignore 规则", "failed", err.Error())
			case action == nil:
				add("gitignore", "补全 .gitignore 规则", "failed", "修复未返回结果，无法确认 .gitignore 与跟踪状态")
			case action.Success:
				detail := fmt.Sprintf("新增 %d 条规则", len(action.RulesAdded))
				if len(action.UntrackedFiles) > 0 {
					detail += fmt.Sprintf("，取消跟踪 %d 个构建产物", len(action.UntrackedFiles))
				}
				add("gitignore", "补全 .gitignore 规则", "applied", detail)
			default:
				detail := fmt.Sprintf("新增 %d 条规则，取消跟踪 %d 个构建产物，%d 个移出失败",
					len(action.RulesAdded), len(action.UntrackedFiles), len(action.FailedFiles))
				if len(action.FailedFiles) > 0 {
					detail += "：" + firstLine(action.FailedFiles[0])
				}
				add("gitignore", "补全 .gitignore 规则", "failed", detail)
			}
		}
	} else {
		add("gitignore", "补全 .gitignore 规则", "skipped", "未勾选")
	}

	// 3. UnityYAMLMerge (Unity only — merging .unity/.prefab as text is a
	// guaranteed disaster without the semantic driver).
	if engine == "unity" {
		if opts.ConfigureUnityMerge {
			status, err := s.DetectUnityYamlMerge(repoPath)
			if err != nil {
				add("unity-merge", "配置 UnityYAMLMerge", "failed", err.Error())
			} else if status.IsConfigured {
				add("unity-merge", "配置 UnityYAMLMerge", "skipped", "已配置："+status.ConfiguredToolPath)
			} else if err := s.ConfigureUnityYamlMerge(repoPath, "", true); err != nil {
				add("unity-merge", "配置 UnityYAMLMerge", "failed", err.Error())
			} else {
				add("unity-merge", "配置 UnityYAMLMerge", "applied", "已写入 merge.unityyamlmerge 与 .gitattributes")
			}
		} else {
			add("unity-merge", "配置 UnityYAMLMerge", "skipped", "未勾选")
		}
	} else {
		add("unity-merge", "配置 UnityYAMLMerge", "skipped", "非 Unity 工程")
	}

	// 4. LFS tracking patterns
	if len(opts.LfsExtensions) > 0 {
		if err := s.TrackExtensionsAsLfs(repoPath, opts.LfsExtensions); err != nil {
			add("lfs", "跟踪 LFS 资产扩展名", "failed", err.Error())
		} else {
			add("lfs", "跟踪 LFS 资产扩展名", "applied",
				fmt.Sprintf("已跟踪 %d 类扩展名：%s", len(opts.LfsExtensions), strings.Join(opts.LfsExtensions, "、")))
		}
	} else {
		add("lfs", "跟踪 LFS 资产扩展名", "skipped", "未指定扩展名")
	}

	// Re-check instead of trusting the steps: the score must reflect disk state.
	res.Readiness = s.engineReadiness(repoPath, engine)
	res.Success = res.Failed == 0
	switch {
	case res.Failed > 0:
		res.Message = fmt.Sprintf("%d 步成功，%d 步失败。工程就绪度 %d%%。", res.Applied, res.Failed, res.Readiness)
	case res.Applied == 0:
		res.Message = fmt.Sprintf("没有需要改动的项，工程就绪度 %d%%。", res.Readiness)
	default:
		res.Message = fmt.Sprintf("完成 %d 项配置，工程就绪度 %d%%。", res.Applied, res.Readiness)
	}
	return res, nil
}

// engineReadiness scores four independent checks at 25 points each.
func (s GitService) engineReadiness(repoPath string, engine string) int {
	score := 0

	info, err := s.DetectGameEngineProject(repoPath)
	if err == nil && info != nil {
		if info.HasGitAttributes {
			score += 25
		}
		if info.HasLfsRules {
			score += 25
		}
	}

	if audit, err := s.AuditIgnoreRules(repoPath); err == nil && audit != nil {
		if audit.HealthScore >= 80 {
			score += 25
		} else if audit.HealthScore >= 50 {
			score += 12
		}
	}

	if engine == "unity" {
		if status, err := s.DetectUnityYamlMerge(repoPath); err == nil && status.IsConfigured {
			score += 25
		}
	} else {
		// No engine-specific driver to configure: award the slot so the score
		// stays comparable across engines.
		score += 25
	}
	return score
}
