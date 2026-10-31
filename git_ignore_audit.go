// git_ignore_audit.go — 自 git_service.go 抽离（ignore/audit 族），
// 见 docs/big-files-assessment.md。函数级搬移，零逻辑改动。

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AuditIgnoreRules inspects .gitignore completeness and discovers tracked engine build artifacts
func (GitService) AuditIgnoreRules(repoPath string) (*IgnoreSanitizerResult, error) {
	res := &IgnoreSanitizerResult{
		TrackedArtifacts: []IgnoreArtifactItem{},
		MissingRules:     []string{},
		HealthScore:      100,
	}

	// 1. Detect engine
	if _, err := os.Stat(filepath.Join(repoPath, "Assets")); err == nil {
		res.EngineType = "Unity 3D"
	} else if matches, _ := filepath.Glob(filepath.Join(repoPath, "*.uproject")); len(matches) > 0 {
		res.EngineType = "Unreal Engine"
	} else if _, err := os.Stat(filepath.Join(repoPath, "project.godot")); err == nil {
		res.EngineType = "Godot Engine"
	} else if _, err := os.Stat(filepath.Join(repoPath, "project.json")); err == nil {
		res.EngineType = "Cocos Creator"
	} else {
		res.EngineType = "通用游戏工程"
	}

	// 2. Read .gitignore
	ignorePath := filepath.Join(repoPath, ".gitignore")
	ignoreBytes, err := os.ReadFile(ignorePath)
	res.HasGitIgnore = (err == nil)
	ignoreContent := string(ignoreBytes)

	// Check missing recommended rules
	type ruleCheck struct {
		pattern  string
		category string
	}
	recommended := []ruleCheck{}
	switch res.EngineType {
	case "Unity 3D":
		recommended = []ruleCheck{
			{"[Ll]ibrary/", "Unity Library 临时缓存"},
			{"[Tt]emp/", "Unity Temp 编辑器构建目录"},
			{"[Oo]bj/", "编译中间产物 Obj"},
			{"[Bb]uild/", "构建输出目录"},
			{"[Ll]ogs/", "Unity 编辑器日志"},
			{"MemoryCaptures/", "内存诊断抓包"},
			{"*.apk", "Android 包文件"},
			{"*.unitypackage", "导出的包文件"},
		}
	case "Unreal Engine":
		recommended = []ruleCheck{
			{"Binaries/", "虚幻可执行文件 Binaries"},
			{"DerivedDataCache/", "DDC 派生数据缓存"},
			{"Intermediate/", "虚幻中间生成文件"},
			{"Saved/", "本地保存与 Crash 报告"},
			{".vs/", "Visual Studio 配置"},
			{"*.pdb", "调试符号文件 PDB"},
		}
	case "Godot Engine":
		recommended = []ruleCheck{
			{".godot/", "Godot 4 导入缓存"},
			{".import/", "Godot 3 导入缓存"},
			{"export_presets.cfg", "导出预设密码凭证"},
		}
	default:
		recommended = []ruleCheck{
			{".DS_Store", "macOS 系统元数据"},
			{"Thumbs.db", "Windows 缩略图缓存"},
			{".idea/", "JetBrains IDE 配置"},
			{".vscode/", "VSCode 本地配置"},
			{"*.log", "运行时日志"},
			{"*.tmp", "临时文件"},
		}
	}

	for _, rc := range recommended {
		if !hasIgnoreRule(ignoreContent, rc.pattern) {
			res.MissingRules = append(res.MissingRules, fmt.Sprintf("%s (%s)", rc.pattern, rc.category))
		}
	}

	// 3. Scan git ls-files for dangerously tracked build artifacts
	lsOut, err := ExecGit(repoPath, "ls-files")
	if err == nil && lsOut != "" {
		lines := strings.Split(lsOut, "\n")
		for _, file := range lines {
			file = strings.TrimSpace(file)
			if file == "" {
				continue
			}

			category := ""
			lower := strings.ToLower(file)
			if strings.HasPrefix(file, "Library/") || strings.HasPrefix(lower, "library/") {
				category = "Unity Library 缓存 (极度臃肿)"
			} else if strings.HasPrefix(file, "Temp/") || strings.HasPrefix(lower, "temp/") {
				category = "Unity Temp 临时目录"
			} else if strings.HasPrefix(file, "Binaries/") || strings.HasPrefix(file, "Intermediate/") || strings.HasPrefix(file, "DerivedDataCache/") {
				category = "Unreal Engine 构建产物"
			} else if strings.HasPrefix(file, ".vs/") || strings.HasPrefix(file, ".idea/") {
				category = "IDE 配置文件"
			} else if strings.HasSuffix(lower, ".pdb") || strings.HasSuffix(lower, ".obj") || strings.HasSuffix(lower, ".apk") || strings.HasSuffix(lower, ".exe") {
				category = "二进制构建产物"
			} else if strings.HasSuffix(lower, ".ds_store") || strings.HasSuffix(lower, "thumbs.db") {
				category = "操作系统无用元数据"
			}

			if category != "" {
				full := filepath.Join(repoPath, file)
				var sz int64 = 0
				szHuman := "0 B"
				if fi, sErr := os.Stat(full); sErr == nil {
					sz = fi.Size()
					if sz < 1024 {
						szHuman = strconv.FormatInt(sz, 10) + " B"
					} else if sz < 1024*1024 {
						szHuman = fmt.Sprintf("%.1f KB", float64(sz)/1024)
					} else {
						szHuman = fmt.Sprintf("%.1f MB", float64(sz)/(1024*1024))
					}
				}

				res.TrackedArtifacts = append(res.TrackedArtifacts, IgnoreArtifactItem{
					Path:      file,
					Category:  category,
					IsTracked: true,
					SizeBytes: sz,
					SizeHuman: szHuman,
					Ignored:   false,
					Remedy:    "执行 git rm --cached 移出跟踪",
				})
			}
		}
	}

	res.TrackedCount = len(res.TrackedArtifacts)
	res.TotalArtifacts = res.TrackedCount

	// Compute score
	score := 100 - (res.TrackedCount * 15) - (len(res.MissingRules) * 5)
	if !res.HasGitIgnore {
		score -= 30
	}
	if score < 0 {
		score = 0
	}
	res.HealthScore = score

	if res.TrackedCount > 0 {
		res.Summary = fmt.Sprintf("⚠️ 风险发现：检测到 %d 个编译产物/临时目录被错误推入 Git 跟踪！缺失 %d 条关键忽略规则，健康度评分: %d/100。", res.TrackedCount, len(res.MissingRules), res.HealthScore)
	} else if len(res.MissingRules) > 0 {
		res.Summary = fmt.Sprintf("ℹ️ 提示：未发现错误跟踪的产物，但缺少 %d 条推荐的 %s 规范规则，健康度评分: %d/100。", len(res.MissingRules), res.EngineType, res.HealthScore)
	} else {
		res.Summary = fmt.Sprintf("✅ 规范体检完美通过！.gitignore 完备，无临时构建产物跟踪，健康度评分: %d/100。", res.HealthScore)
	}

	return res, nil
}

func normalizeIgnoreRule(rule string) string {
	lower := strings.ToLower(strings.TrimSpace(rule))
	lower = strings.TrimPrefix(lower, "/")
	var normalized strings.Builder
	for i := 0; i < len(lower); {
		if i+3 < len(lower) && lower[i] == '[' && lower[i+3] == ']' && lower[i+1] == lower[i+2] {
			normalized.WriteByte(lower[i+1])
			i += 4
			continue
		}
		normalized.WriteByte(lower[i])
		i++
	}
	return normalized.String()
}

func hasIgnoreRule(content string, pattern string) bool {
	normalizedPattern := normalizeIgnoreRule(pattern)
	if normalizedPattern == "" {
		return false
	}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if normalizedLine := normalizeIgnoreRule(line); normalizedLine != "" && strings.Contains(normalizedLine, normalizedPattern) {
			return true
		}
	}
	return false
}

// ApplyIgnoreSanitizer fixes .gitignore rules and untracks dangerous build artifacts
func (s GitService) ApplyIgnoreSanitizer(repoPath string, engineType string, untrackExisting bool) (*IgnoreSanitizeActionResult, error) {
	res := &IgnoreSanitizeActionResult{
		RulesAdded:     []string{},
		UntrackedFiles: []string{},
		FailedFiles:    []string{},
	}

	// 1. Audit first
	audit, err := s.AuditIgnoreRules(repoPath)
	if err != nil {
		return nil, err
	}

	// 2. Append rules to .gitignore
	ignorePath := filepath.Join(repoPath, ".gitignore")
	var existingContent string
	if b, readErr := os.ReadFile(ignorePath); readErr == nil {
		existingContent = string(b)
	} else if !os.IsNotExist(readErr) {
		// Treating an unreadable file as empty would append duplicate rules.
		return nil, fmt.Errorf("读取 .gitignore 失败: %w", readErr)
	}

	rulesToAdd := []string{}
	ruleTemplates := map[string][]string{
		"Unity 3D": {
			"[Ll]ibrary/", "[Tt]emp/", "[Oo]bj/", "[Bb]uild/", "[Bb]uilds/",
			"[Ll]ogs/", "MemoryCaptures/", "*.apk", "*.unitypackage",
		},
		"Unreal Engine": {
			"Binaries/", "DerivedDataCache/", "Intermediate/", "Saved/",
			".vs/", "*.pdb", "*.sln",
		},
		"Godot Engine": {
			".godot/", ".import/", "export_presets.cfg",
		},
		"通用游戏工程": {
			".DS_Store", "Thumbs.db", ".idea/", ".vscode/", "*.log", "*.tmp",
		},
	}

	tpl, ok := ruleTemplates[engineType]
	if !ok {
		tpl = ruleTemplates["通用游戏工程"]
	}

	for _, r := range tpl {
		if !hasIgnoreRule(existingContent, r) {
			rulesToAdd = append(rulesToAdd, r)
		}
	}

	if len(rulesToAdd) > 0 {
		appendStr := "\n# --- Studio Game Engine Ignore Rules ---\n" + strings.Join(rulesToAdd, "\n") + "\n"
		f, fErr := os.OpenFile(ignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if fErr != nil {
			return nil, fmt.Errorf("写入 .gitignore 失败: %w", fErr)
		}
		if _, writeErr := f.WriteString(appendStr); writeErr != nil {
			_ = f.Close()
			return nil, fmt.Errorf("写入 .gitignore 失败: %w", writeErr)
		}
		if closeErr := f.Close(); closeErr != nil {
			return nil, fmt.Errorf("关闭 .gitignore 失败: %w", closeErr)
		}
		res.RulesAdded = rulesToAdd
	}

	// 3. Untrack tracked artifacts if requested. A single `git rm --cached`
	// call covers every artifact: the index is written once instead of once per
	// file, and one failing batch does not silently look like full success.
	if untrackExisting && len(audit.TrackedArtifacts) > 0 {
		paths := make([]string, 0, len(audit.TrackedArtifacts))
		seen := make(map[string]bool, len(audit.TrackedArtifacts))
		for _, item := range audit.TrackedArtifacts {
			p := strings.TrimSpace(item.Path)
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			paths = append(paths, p)
		}

		if len(paths) > 0 {
			batchErr := error(nil)
			if _, rmErr := ExecGit(repoPath, append([]string{"rm", "--cached", "--quiet", "--"}, paths...)...); rmErr != nil {
				batchErr = rmErr
			}

			if batchErr == nil {
				res.UntrackedFiles = append(res.UntrackedFiles, paths...)
			} else {
				// The batch is atomic from git's point of view only for the
				// paths it touched; verify what really remains tracked so partial
				// success is reported truthfully. If the verification itself
				// fails we fail closed and report nothing as untracked.
				verifyOut, verifyErr := ExecGitRaw(repoPath, append([]string{"ls-files", "-z", "--"}, paths...)...)
				if verifyErr != nil {
					for _, p := range paths {
						res.FailedFiles = append(res.FailedFiles,
							fmt.Sprintf("%s: %v；校验跟踪状态失败: %v", p, batchErr, verifyErr))
					}
				} else {
					stillTracked := make(map[string]bool, len(paths))
					for _, entry := range strings.Split(verifyOut, "\x00") {
						if entry != "" {
							stillTracked[entry] = true
						}
					}
					for _, p := range paths {
						if stillTracked[p] {
							res.FailedFiles = append(res.FailedFiles,
								fmt.Sprintf("%s: %v", p, batchErr))
						} else {
							res.UntrackedFiles = append(res.UntrackedFiles, p)
						}
					}
				}
			}
		}
	}

	res.Success = len(res.FailedFiles) == 0
	if len(res.FailedFiles) > 0 {
		res.Message = fmt.Sprintf("已优化 .gitignore（新增 %d 条规则），移出 %d 个误跟踪产物，%d 个失败", len(res.RulesAdded), len(res.UntrackedFiles), len(res.FailedFiles))
	} else {
		res.Message = fmt.Sprintf("已成功优化 .gitignore（新增 %d 条规则），并安全移出 %d 个误跟踪产物！", len(res.RulesAdded), len(res.UntrackedFiles))
	}
	return res, nil
}
