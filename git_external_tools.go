package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Supported external diff & merge tool catalog
type toolCandidate struct {
	ID            string
	Name          string
	Candidates    []string
	DiffArgs      func(exe, left, right string) []string
	MergeArgs     func(exe, theirs, ours, base, result string) []string
	SupportsDiff  bool
	SupportsMerge bool
	Description   string
}

func getToolCandidates() []toolCandidate {
	home, _ := os.UserHomeDir()
	return []toolCandidate{
		{
			ID:   "cursor",
			Name: "Cursor",
			Candidates: []string{
				"cursor",
				"/Applications/Cursor.app/Contents/Resources/app/bin/cursor",
				filepath.Join(home, "AppData/Local/Programs/cursor/resources/app/bin/cursor.cmd"),
			},
			DiffArgs: func(exe, left, right string) []string {
				return []string{exe, "--diff", left, right}
			},
			MergeArgs: func(exe, theirs, ours, base, result string) []string {
				return []string{exe, "--wait", "--merge", theirs, ours, base, result}
			},
			SupportsDiff:  true,
			SupportsMerge: true,
			Description:   "智能 AI 代码编辑器 (Cursor)，支持左右分栏与三方合并",
		},
		{
			ID:   "vscode",
			Name: "Visual Studio Code",
			Candidates: []string{
				"code",
				"code-insiders",
				"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
				"/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code",
				filepath.Join(home, "AppData/Local/Programs/Microsoft VS Code/bin/code.cmd"),
			},
			DiffArgs: func(exe, left, right string) []string {
				return []string{exe, "--diff", left, right}
			},
			MergeArgs: func(exe, theirs, ours, base, result string) []string {
				return []string{exe, "--wait", "--merge", theirs, ours, base, result}
			},
			SupportsDiff:  true,
			SupportsMerge: true,
			Description:   "主流轻量编辑器，支持原生分屏 Diff 与合并裁决",
		},
		{
			ID:   "beyondcompare",
			Name: "Beyond Compare",
			Candidates: []string{
				"bcomp",
				"/Applications/Beyond Compare.app/Contents/MacOS/bcomp",
				"C:\\Program Files\\Beyond Compare 4\\BComp.exe",
				"C:\\Program Files\\Beyond Compare 5\\BComp.exe",
			},
			DiffArgs: func(exe, left, right string) []string {
				return []string{exe, left, right}
			},
			MergeArgs: func(exe, theirs, ours, base, result string) []string {
				return []string{exe, ours, theirs, base, result}
			},
			SupportsDiff:  true,
			SupportsMerge: true,
			Description:   "工业级专业文件与目录对比合并工具，对游戏重度资产极其友好",
		},
		{
			ID:   "idea",
			Name: "IntelliJ IDEA / JetBrains",
			Candidates: []string{
				"idea",
				"/Applications/IntelliJ IDEA.app/Contents/MacOS/idea",
				filepath.Join(home, "Library/Application Support/JetBrains/Toolbox/scripts/idea"),
			},
			DiffArgs: func(exe, left, right string) []string {
				return []string{exe, "diff", left, right}
			},
			MergeArgs: func(exe, theirs, ours, base, result string) []string {
				return []string{exe, "merge", ours, theirs, base, result}
			},
			SupportsDiff:  true,
			SupportsMerge: true,
			Description:   "JetBrains 全功能 IDE 差异对比与冲突合并器",
		},
		{
			ID:   "sublimemerge",
			Name: "Sublime Merge",
			Candidates: []string{
				"smerge",
				"/Applications/Sublime Merge.app/Contents/SharedSupport/bin/smerge",
				"C:\\Program Files\\Sublime Merge\\smerge.exe",
			},
			DiffArgs: func(exe, left, right string) []string {
				return []string{exe, "diff", left, right}
			},
			MergeArgs: func(exe, theirs, ours, base, result string) []string {
				return []string{exe, "mergetool", ours, theirs, base, result}
			},
			SupportsDiff:  true,
			SupportsMerge: true,
			Description:   "极致快速响应的专用 Git 客户端与对比合并工具",
		},
		{
			ID:   "opendiff",
			Name: "FileMerge (opendiff)",
			Candidates: []string{
				"/usr/bin/opendiff",
				"opendiff",
			},
			DiffArgs: func(exe, left, right string) []string {
				return []string{exe, left, right}
			},
			MergeArgs: func(exe, theirs, ours, base, result string) []string {
				return []string{exe, ours, theirs, "-ancestor", base, "-merge", result}
			},
			SupportsDiff:  true,
			SupportsMerge: true,
			Description:   "macOS / Xcode 原生内置双栏/三方对比工具",
		},
		{
			ID:   "meld",
			Name: "Meld",
			Candidates: []string{
				"meld",
				"/Applications/Meld.app/Contents/MacOS/Meld",
			},
			DiffArgs: func(exe, left, right string) []string {
				return []string{exe, left, right}
			},
			MergeArgs: func(exe, theirs, ours, base, result string) []string {
				return []string{exe, "--auto-merge", ours, base, theirs, "-o", result}
			},
			SupportsDiff:  true,
			SupportsMerge: true,
			Description:   "可视化 Diff 与 Merge 经典跨平台开源工具",
		},
	}
}

// resolveExecutable locates the first existing candidate binary
func resolveExecutable(candidates []string) (string, bool) {
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if filepath.IsAbs(c) {
			if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
				return c, true
			}
		} else {
			if p, err := exec.LookPath(c); err == nil {
				return p, true
			}
		}
	}
	return "", false
}

// DetectExternalTools discovers installed diff/merge tools on the local machine
func (GitService) DetectExternalTools() ([]ExternalToolInfo, error) {
	candidates := getToolCandidates()
	results := make([]ExternalToolInfo, 0, len(candidates))

	for _, tc := range candidates {
		exe, ok := resolveExecutable(tc.Candidates)
		results = append(results, ExternalToolInfo{
			ID:            tc.ID,
			Name:          tc.Name,
			Executable:    exe,
			IsAvailable:   ok,
			SupportsDiff:  tc.SupportsDiff,
			SupportsMerge: tc.SupportsMerge,
			Description:   tc.Description,
		})
	}

	return results, nil
}

// GetExternalToolConfig reads configured diff/merge tools or returns smart defaults
func (s GitService) GetExternalToolConfig(repoPath string) (*ExternalToolConfig, error) {
	diffTool, _ := ExecGit(repoPath, "config", "--get", "diff.tool")
	mergeTool, _ := ExecGit(repoPath, "config", "--get", "merge.tool")
	customDiffCmd, _ := ExecGit(repoPath, "config", "--get", "wgit.customDiffCmd")
	customMergeCmd, _ := ExecGit(repoPath, "config", "--get", "wgit.customMergeCmd")

	cfg := &ExternalToolConfig{
		DiffTool:       strings.TrimSpace(diffTool),
		MergeTool:      strings.TrimSpace(mergeTool),
		CustomDiffCmd:  strings.TrimSpace(customDiffCmd),
		CustomMergeCmd: strings.TrimSpace(customMergeCmd),
		IsGlobal:       externalToolConfigIsGlobal(repoPath),
	}

	// Smart fallback: if not set, pick the first available detected tool
	if cfg.DiffTool == "" || cfg.MergeTool == "" {
		tools, _ := s.DetectExternalTools()
		for _, t := range tools {
			if t.IsAvailable {
				if cfg.DiffTool == "" && t.SupportsDiff {
					cfg.DiffTool = t.ID
				}
				if cfg.MergeTool == "" && t.SupportsMerge {
					cfg.MergeTool = t.ID
				}
			}
		}
	}

	return cfg, nil
}

func externalToolConfigIsGlobal(repoPath string) bool {
	hasGlobal := false
	for _, key := range []string{"diff.tool", "merge.tool", "wgit.customDiffCmd", "wgit.customMergeCmd"} {
		out, err := ExecGit(repoPath, "config", "--show-scope", "--get", key)
		if err != nil {
			continue
		}
		fields := strings.Fields(out)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "local":
			return false
		case "global":
			hasGlobal = true
		}
	}
	return hasGlobal
}

// SetExternalToolConfig saves diff.tool, merge.tool, and custom command templates
func (GitService) SetExternalToolConfig(repoPath string, diffTool, mergeTool, customDiffCmd, customMergeCmd string, isGlobal bool) error {
	scope := "--local"
	if isGlobal {
		scope = "--global"
	}

	setOrUnset := func(key, value string) error {
		if value == "" {
			// Clearing the field must remove the previous value, not leave it stale.
			return clearGitConfigKey(repoPath, key, isGlobal)
		}
		_, err := ExecGit(repoPath, "config", scope, key, value)
		return err
	}
	for _, item := range [][2]string{
		{"diff.tool", diffTool},
		{"merge.tool", mergeTool},
		{"wgit.customDiffCmd", customDiffCmd},
		{"wgit.customMergeCmd", customMergeCmd},
	} {
		if err := setOrUnset(item[0], item[1]); err != nil {
			return err
		}
	}
	return nil
}

// OpenExternalDiff launches the selected or default external diff tool
func (s GitService) OpenExternalDiff(repoPath string, filePath string, isStaged bool, toolID string) error {
	if repoPath == "" || filePath == "" {
		return errors.New("repoPath and filePath are required")
	}
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return err
	}
	filePath = cleanPath
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	cfg, err := s.GetExternalToolConfig(repoPath)
	if err != nil {
		return fmt.Errorf("读取外部工具配置失败: %w", err)
	}
	targetToolID := toolID
	if targetToolID == "" {
		targetToolID = cfg.DiffTool
	}
	if targetToolID == "" {
		targetToolID = "vscode"
	}

	candidates := getToolCandidates()
	var selectedCandidate *toolCandidate
	for i := range candidates {
		if candidates[i].ID == targetToolID {
			selectedCandidate = &candidates[i]
			break
		}
	}

	var exe string
	var hasExe bool
	if selectedCandidate != nil {
		exe, hasExe = resolveExecutable(selectedCandidate.Candidates)
	}

	// Fallback to any available tool if target not found. An explicit custom
	// command must not be overridden by an auto-detected tool.
	if !hasExe && targetToolID != "custom" {
		tools, _ := s.DetectExternalTools()
		for _, t := range tools {
			if t.IsAvailable && t.SupportsDiff {
				for i := range candidates {
					if candidates[i].ID == t.ID {
						selectedCandidate = &candidates[i]
						exe = t.Executable
						hasExe = true
						break
					}
				}
				if hasExe {
					break
				}
			}
		}
	}

	if targetToolID == "custom" && strings.TrimSpace(cfg.CustomDiffCmd) == "" {
		return errors.New("自定义比对命令为空，请在设置中配置")
	}
	if !hasExe && cfg.CustomDiffCmd == "" {
		return fmt.Errorf("未找到可用的外部对比工具 (%s)，请在设置中配置", targetToolID)
	}

	// Prepare left (base / old) and right (target / new) files
	tmpDir := os.TempDir()
	baseName := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	plainName := strings.TrimSuffix(baseName, ext)

	leftFile := filepath.Join(tmpDir, fmt.Sprintf("wgit_old_%s%s", plainName, ext))
	workingFile := filepath.Join(repoPath, filePath)
	var rightFile string

	// Extract old version content from git
	var oldContent string
	if isStaged {
		// comparing HEAD vs Staged (index)
		out, err := ExecGitRaw(repoPath, "show", fmt.Sprintf("HEAD:%s", filePath))
		if err == nil {
			oldContent = out
		}
		// right side is index staged content
		rightFile = filepath.Join(tmpDir, fmt.Sprintf("wgit_staged_%s%s", plainName, ext))
		stagedContent, err := ExecGitRaw(repoPath, "show", fmt.Sprintf(":%s", filePath))
		if err != nil {
			return fmt.Errorf("读取暂存区文件内容失败: %w", err)
		}
		if err := os.WriteFile(rightFile, []byte(stagedContent), 0o644); err != nil {
			return fmt.Errorf("无法创建暂存区比对临时文件: %w", err)
		}
	} else {
		// Comparing index vs worktree. An untracked file legitimately has no
		// index/HEAD version, so an empty left side is valid for a new file.
		out, err := ExecGitRaw(repoPath, "show", fmt.Sprintf(":%s", filePath))
		if err != nil {
			out, _ = ExecGitRaw(repoPath, "show", fmt.Sprintf("HEAD:%s", filePath))
		}
		oldContent = out
		rightFile = workingFile
	}

	if err := os.WriteFile(leftFile, []byte(oldContent), 0644); err != nil {
		return fmt.Errorf("无法创建比对临时文件: %w", err)
	}

	// Build execution command
	var cmd *exec.Cmd
	if selectedCandidate != nil && hasExe {
		args := selectedCandidate.DiffArgs(exe, leftFile, rightFile)
		cmd = exec.Command(args[0], args[1:]...)
	} else if cfg.CustomDiffCmd != "" {
		cmdStr := strings.ReplaceAll(cfg.CustomDiffCmd, "{left}", leftFile)
		cmdStr = strings.ReplaceAll(cmdStr, "{right}", rightFile)
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", cmdStr)
		} else {
			cmd = exec.Command("sh", "-c", cmdStr)
		}
	} else {
		return errors.New("无可用外部 Diff 命令")
	}

	cmd.Dir = repoPath
	return cmd.Start()
}

// OpenExternalMerge launches the merge tool for conflict resolution
func (s GitService) OpenExternalMerge(repoPath string, filePath string, toolID string) error {
	if repoPath == "" || filePath == "" {
		return errors.New("repoPath and filePath are required")
	}
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return err
	}
	filePath = cleanPath
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	cfg, err := s.GetExternalToolConfig(repoPath)
	if err != nil {
		return fmt.Errorf("读取外部工具配置失败: %w", err)
	}
	targetToolID := toolID
	if targetToolID == "" {
		targetToolID = cfg.MergeTool
	}

	// Try running `git mergetool` directly if tool is supported
	if targetToolID != "" && targetToolID != "custom" {
		cmd := exec.Command("git", "mergetool", "--no-prompt", fmt.Sprintf("--tool=%s", targetToolID), filePath)
		cmd.Dir = repoPath
		if err := cmd.Start(); err == nil {
			return nil
		}
	}

	// Fallback to manual 3-way launch with temp files
	tmpDir := os.TempDir()
	baseName := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	plain := strings.TrimSuffix(baseName, ext)

	baseFile := filepath.Join(tmpDir, fmt.Sprintf("wgit_base_%s%s", plain, ext))
	oursFile := filepath.Join(tmpDir, fmt.Sprintf("wgit_ours_%s%s", plain, ext))
	theirsFile := filepath.Join(tmpDir, fmt.Sprintf("wgit_theirs_%s%s", plain, ext))
	resultFile := filepath.Join(repoPath, filePath)

	// Stage 1: base, Stage 2: ours, Stage 3: theirs. A missing stage is
	// legitimate (add/add has no base), but all three failing means the file
	// is not conflicted or the index cannot be read.
	baseContent, baseErr := ExecGitRaw(repoPath, "show", fmt.Sprintf(":1:%s", filePath))
	oursContent, oursErr := ExecGitRaw(repoPath, "show", fmt.Sprintf(":2:%s", filePath))
	theirsContent, theirsErr := ExecGitRaw(repoPath, "show", fmt.Sprintf(":3:%s", filePath))
	if baseErr != nil && oursErr != nil && theirsErr != nil {
		return fmt.Errorf("无法读取冲突阶段内容（文件可能已无冲突或索引损坏）: %v", oursErr)
	}
	for _, item := range []struct {
		path    string
		content string
	}{
		{baseFile, baseContent},
		{oursFile, oursContent},
		{theirsFile, theirsContent},
	} {
		if err := os.WriteFile(item.path, []byte(item.content), 0o644); err != nil {
			return fmt.Errorf("无法创建合并临时文件 %s: %w", filepath.Base(item.path), err)
		}
	}

	if targetToolID == "custom" {
		if strings.TrimSpace(cfg.CustomMergeCmd) == "" {
			return errors.New("自定义合并命令为空，请在设置中配置")
		}
		cmdStr := cfg.CustomMergeCmd
		for placeholder, path := range map[string]string{
			"{theirs}": theirsFile,
			"{ours}":   oursFile,
			"{base}":   baseFile,
			"{result}": resultFile,
		} {
			cmdStr = strings.ReplaceAll(cmdStr, placeholder, path)
		}
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", cmdStr)
		} else {
			cmd = exec.Command("sh", "-c", cmdStr)
		}
		cmd.Dir = repoPath
		return cmd.Start()
	}

	candidates := getToolCandidates()
	var selectedCandidate *toolCandidate
	for i := range candidates {
		if candidates[i].ID == targetToolID {
			selectedCandidate = &candidates[i]
			break
		}
	}

	var exe string
	var hasExe bool
	if selectedCandidate != nil {
		exe, hasExe = resolveExecutable(selectedCandidate.Candidates)
	}

	if !hasExe {
		tools, _ := s.DetectExternalTools()
		for _, t := range tools {
			if t.IsAvailable && t.SupportsMerge {
				for i := range candidates {
					if candidates[i].ID == t.ID {
						selectedCandidate = &candidates[i]
						exe = t.Executable
						hasExe = true
						break
					}
				}
				if hasExe {
					break
				}
			}
		}
	}

	if !hasExe {
		return fmt.Errorf("未找到可用合并工具: %s", targetToolID)
	}

	args := selectedCandidate.MergeArgs(exe, theirsFile, oursFile, baseFile, resultFile)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = repoPath
	return cmd.Start()
}

// ============================================================================
// Remote Web Forge (GitHub / GitLab / Gitea / Gitee / Bitbucket)
// ============================================================================

// ParseRemoteWebURL analyzes a remote git clone URL and generates web interface URLs
func ParseRemoteWebURL(remoteURL string) *RemoteWebInfo {
	info := &RemoteWebInfo{
		RemoteURL: remoteURL,
		Provider:  "generic",
	}

	if remoteURL == "" {
		return info
	}

	raw := strings.TrimSpace(remoteURL)
	var host, path string

	// 1. SSH format: git@github.com:owner/repo.git or ssh://git@host:port/owner/repo.git
	if strings.HasPrefix(raw, "git@") {
		// git@host:owner/repo.git
		parts := strings.SplitN(strings.TrimPrefix(raw, "git@"), ":", 2)
		if len(parts) == 2 {
			host = parts[0]
			path = parts[1]
		}
	} else if strings.HasPrefix(raw, "ssh://") {
		u, err := url.Parse(raw)
		if err == nil {
			host = u.Hostname()
			path = strings.TrimPrefix(u.Path, "/")
		}
	} else if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err == nil {
			host = u.Host
			path = strings.TrimPrefix(u.Path, "/")
		}
	}

	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimPrefix(path, "/")

	if host == "" || path == "" {
		return info
	}

	info.HasRemote = true
	info.WebBaseURL = fmt.Sprintf("https://%s/%s", host, path)

	// Provider detection
	lowerHost := strings.ToLower(host)
	switch {
	case strings.Contains(lowerHost, "github.com"):
		info.Provider = "github"
		info.CommitURLTpl = fmt.Sprintf("%s/commit/{hash}", info.WebBaseURL)
		info.BranchURLTpl = fmt.Sprintf("%s/tree/{branch}", info.WebBaseURL)
		info.FileURLTpl = fmt.Sprintf("%s/blob/{branch}/{path}", info.WebBaseURL)
		info.CompareURLTpl = fmt.Sprintf("%s/compare/{base}...{head}", info.WebBaseURL)
	case strings.Contains(lowerHost, "gitlab"):
		info.Provider = "gitlab"
		info.CommitURLTpl = fmt.Sprintf("%s/-/commit/{hash}", info.WebBaseURL)
		info.BranchURLTpl = fmt.Sprintf("%s/-/tree/{branch}", info.WebBaseURL)
		info.FileURLTpl = fmt.Sprintf("%s/-/blob/{branch}/{path}", info.WebBaseURL)
		info.CompareURLTpl = fmt.Sprintf("%s/-/compare/{base}...{head}", info.WebBaseURL)
	case strings.Contains(lowerHost, "gitee.com"):
		info.Provider = "gitee"
		info.CommitURLTpl = fmt.Sprintf("%s/commit/{hash}", info.WebBaseURL)
		info.BranchURLTpl = fmt.Sprintf("%s/tree/{branch}", info.WebBaseURL)
		info.FileURLTpl = fmt.Sprintf("%s/blob/{branch}/{path}", info.WebBaseURL)
		info.CompareURLTpl = fmt.Sprintf("%s/compare/{base}...{head}", info.WebBaseURL)
	case strings.Contains(lowerHost, "bitbucket.org"):
		info.Provider = "bitbucket"
		info.CommitURLTpl = fmt.Sprintf("%s/commits/{hash}", info.WebBaseURL)
		info.BranchURLTpl = fmt.Sprintf("%s/branch/{branch}", info.WebBaseURL)
		info.FileURLTpl = fmt.Sprintf("%s/src/{branch}/{path}", info.WebBaseURL)
		info.CompareURLTpl = fmt.Sprintf("%s/branches/compare/{head}%%0D{base}#diff", info.WebBaseURL)
	case strings.Contains(lowerHost, "gitea") || strings.Contains(lowerHost, "forgejo"):
		info.Provider = "gitea"
		info.CommitURLTpl = fmt.Sprintf("%s/commit/{hash}", info.WebBaseURL)
		info.BranchURLTpl = fmt.Sprintf("%s/src/branch/{branch}", info.WebBaseURL)
		info.FileURLTpl = fmt.Sprintf("%s/src/branch/{branch}/{path}", info.WebBaseURL)
		info.CompareURLTpl = fmt.Sprintf("%s/compare/{base}...{head}", info.WebBaseURL)
	default:
		// Generic git web UI standard
		info.Provider = "generic"
		info.CommitURLTpl = fmt.Sprintf("%s/commit/{hash}", info.WebBaseURL)
		info.BranchURLTpl = fmt.Sprintf("%s/tree/{branch}", info.WebBaseURL)
		info.FileURLTpl = fmt.Sprintf("%s/blob/{branch}/{path}", info.WebBaseURL)
		info.CompareURLTpl = fmt.Sprintf("%s/compare/{base}...{head}", info.WebBaseURL)
	}

	return info
}

// GetRemoteWebInfo inspects origin remote and returns web links
func (GitService) GetRemoteWebInfo(repoPath string) (*RemoteWebInfo, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repoPath 不能为空")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}
	out, err := ExecGit(repoPath, "remote", "get-url", "origin")
	remoteURL := strings.TrimSpace(out)
	remoteName := "origin"

	if err != nil || remoteURL == "" {
		// Check any other remote if origin is not present
		remotes, rErr := ExecGit(repoPath, "remote")
		if rErr == nil && strings.TrimSpace(remotes) != "" {
			first := strings.Split(strings.TrimSpace(remotes), "\n")[0]
			remoteName = strings.TrimSpace(first)
			urlOut, _ := ExecGit(repoPath, "remote", "get-url", remoteName)
			remoteURL = strings.TrimSpace(urlOut)
		}
	}

	info := ParseRemoteWebURL(remoteURL)
	info.RemoteName = remoteName
	return info, nil
}

// OpenWebURL opens an arbitrary HTTP/HTTPS URL in the default browser
func (GitService) OpenWebURL(targetURL string) error {
	if targetURL == "" {
		return errors.New("URL cannot be empty")
	}
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		return errors.New("invalid web URL scheme")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	return cmd.Start()
}

// OpenCommitInWeb opens a commit hash in GitHub / GitLab web UI
func (s GitService) OpenCommitInWeb(repoPath string, commitHash string) error {
	info, err := s.GetRemoteWebInfo(repoPath)
	if err != nil || !info.HasRemote {
		return errors.New("当前仓库未配置远端托管地址")
	}
	u := strings.ReplaceAll(info.CommitURLTpl, "{hash}", commitHash)
	return s.OpenWebURL(u)
}

// OpenFileInWeb opens a source file at the specified branch/revision in web UI
func (s GitService) OpenFileInWeb(repoPath string, filePath string, branch string) error {
	info, err := s.GetRemoteWebInfo(repoPath)
	if err != nil || !info.HasRemote {
		return errors.New("当前仓库未配置远端托管地址")
	}
	b := branch
	if b == "" {
		cur, _ := ExecGit(repoPath, "branch", "--show-current")
		b = strings.TrimSpace(cur)
		if b == "" {
			b = "main"
		}
	}
	u := strings.ReplaceAll(info.FileURLTpl, "{branch}", b)
	u = strings.ReplaceAll(u, "{path}", filePath)
	return s.OpenWebURL(u)
}

// OpenBranchInWeb opens the branch tree view in web UI
func (s GitService) OpenBranchInWeb(repoPath string, branch string) error {
	info, err := s.GetRemoteWebInfo(repoPath)
	if err != nil || !info.HasRemote {
		return errors.New("当前仓库未配置远端托管地址")
	}
	b := branch
	if b == "" {
		cur, _ := ExecGit(repoPath, "branch", "--show-current")
		b = strings.TrimSpace(cur)
	}
	u := strings.ReplaceAll(info.BranchURLTpl, "{branch}", b)
	return s.OpenWebURL(u)
}

// OpenCompareInWeb opens PR / MR creation page between base and head
func (s GitService) OpenCompareInWeb(repoPath string, baseBranch string, headBranch string) error {
	info, err := s.GetRemoteWebInfo(repoPath)
	if err != nil || !info.HasRemote {
		return errors.New("当前仓库未配置远端托管地址")
	}
	base := baseBranch
	if base == "" {
		base = "main"
	}
	head := headBranch
	if head == "" {
		cur, _ := ExecGit(repoPath, "branch", "--show-current")
		head = strings.TrimSpace(cur)
	}
	u := strings.ReplaceAll(info.CompareURLTpl, "{base}", base)
	u = strings.ReplaceAll(u, "{head}", head)
	return s.OpenWebURL(u)
}

// OpenRepoInWeb opens the repository homepage in browser
func (s GitService) OpenRepoInWeb(repoPath string) error {
	info, err := s.GetRemoteWebInfo(repoPath)
	if err != nil || !info.HasRemote {
		return errors.New("当前仓库未配置远端托管地址")
	}
	return s.OpenWebURL(info.WebBaseURL)
}

// FindIssueReferences extracts #123, GH-123 or JIRA-style ABC-123 from commit messages
func FindIssueReferences(msg string) []string {
	re := regexp.MustCompile(`(?i)(?:#\d+|[A-Z]{2,10}-\d+)`)
	matches := re.FindAllString(msg, -1)
	seen := make(map[string]bool)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}
