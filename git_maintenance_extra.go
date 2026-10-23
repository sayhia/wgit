package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// =========================================================================
// C7 补齐：worktree repair/prune、bisect skip、hook 脚本读写
//
// 这三个是「看得见问题但没有对应命令」的典型：工作树路径被移动后失效、
// 二分遇到不可构建的提交只能手动绕过、钩子只能启停不能改内容。
// =========================================================================

type WorktreeOpResult struct {
	Success bool     `json:"success"`
	Action  string   `json:"action"`
	Output  string   `json:"output"`
	Details []string `json:"details"`
	Message string   `json:"message"`
}

// RepairWorktrees re-links worktrees whose .git file points at a moved
// repository. Without arguments git repairs everything it can find.
func (GitService) RepairWorktrees(repoPath string, paths []string) (*WorktreeOpResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	args := []string{"worktree", "repair"}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			args = append(args, p)
		}
	}

	out, err := ExecGitWithTimeout(repoPath, 2*time.Minute, args...)
	res := &WorktreeOpResult{Action: "repair", Output: out, Details: []string{}}
	if err != nil {
		detail := strings.TrimSpace(out)
		if detail == "" {
			detail = err.Error()
		}
		res.Message = "修复失败：" + firstLine(detail)
		return res, nil
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			res.Details = append(res.Details, line)
		}
	}
	res.Success = true
	if len(res.Details) == 0 {
		res.Message = "没有发现需要修复的工作树链接。"
	} else {
		res.Message = fmt.Sprintf("已修复 %d 处工作树链接。", len(res.Details))
	}
	return res, nil
}

// PruneWorktrees drops worktree administrative entries whose directory is gone.
func (GitService) PruneWorktrees(repoPath string) (*WorktreeOpResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	// git defaults to gc.worktreePruneExpire (3 months), so a directory deleted
	// a minute ago would survive the prune. Users clicking "clean up" expect the
	// stale entries gone now, not next quarter.
	out, err := ExecGit(repoPath, "worktree", "prune", "-v", "--expire=now")
	res := &WorktreeOpResult{Action: "prune", Output: out, Details: []string{}}
	if err != nil {
		detail := strings.TrimSpace(out)
		if detail == "" {
			detail = err.Error()
		}
		res.Message = "清理失败：" + firstLine(detail)
		return res, nil
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			res.Details = append(res.Details, line)
		}
	}
	res.Success = true
	if len(res.Details) == 0 {
		res.Message = "没有失效的工作树记录需要清理（已按 --expire=now 检查）。"
	} else {
		res.Message = fmt.Sprintf("已清理 %d 条失效记录（目录已不存在）。", len(res.Details))
	}
	return res, nil
}

// BisectSkip marks the current commit as untestable. Essential when a commit
// in the middle of the range does not build — without it the session stalls.
func (s GitService) BisectSkip(repoPath string) (*BisectStatus, error) {
	out, err := ExecGit(repoPath, "bisect", "skip")
	if err != nil {
		return nil, fmt.Errorf("跳过当前提交失败：%s %s", strings.TrimSpace(out), err.Error())
	}
	return s.BisectStatus(repoPath)
}

// -------------------------------------------------------------------------
// Hook 脚本读写
// -------------------------------------------------------------------------

type GitHookContent struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Enabled bool   `json:"enabled"`
	Size    int64  `json:"size"`
	Content string `json:"content"`
	Message string `json:"message"`
}

// The eight hooks surfaced by GetGitHooks; anything else is rejected so a
// typo cannot create an arbitrary executable file inside .git/hooks.
var editableGitHooks = map[string]bool{
	"pre-commit":         true,
	"prepare-commit-msg": true,
	"commit-msg":         true,
	"post-commit":        true,
	"pre-rebase":         true,
	"post-checkout":      true,
	"post-merge":         true,
	"pre-push":           true,
}

const hookContentLimit = 256 * 1024

func resolveHookPaths(repoPath, name string) (active string, disabled string, err error) {
	if !editableGitHooks[name] {
		return "", "", fmt.Errorf("不支持的钩子名称: %s", name)
	}
	hooksDir := gitPath(repoPath, "hooks")
	return filepath.Join(hooksDir, name), filepath.Join(hooksDir, name+".disabled"), nil
}

// ReadGitHook returns the script behind a hook, preferring the active file and
// falling back to the disabled copy.
func (GitService) ReadGitHook(repoPath string, name string) (*GitHookContent, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	active, disabled, err := resolveHookPaths(repoPath, name)
	if err != nil {
		return nil, err
	}

	res := &GitHookContent{Name: name, Path: active}

	if info, statErr := os.Stat(active); statErr == nil {
		res.Exists = true
		res.Enabled = true
		res.Size = info.Size()
	} else if info, statErr := os.Stat(disabled); statErr == nil {
		res.Exists = true
		res.Enabled = false
		res.Size = info.Size()
		res.Path = disabled
	} else {
		res.Message = "该钩子尚未创建，保存后将新建脚本。"
		return res, nil
	}

	if res.Size > hookContentLimit {
		res.Message = fmt.Sprintf("脚本超过 %d KB，已跳过内容读取。", hookContentLimit/1024)
		return res, nil
	}
	data, readErr := os.ReadFile(res.Path)
	if readErr != nil {
		return nil, fmt.Errorf("读取钩子脚本失败: %w", readErr)
	}
	res.Content = string(data)
	if !res.Enabled {
		res.Message = "该钩子当前处于禁用状态（.disabled），保存时可直接启用。"
	}
	return res, nil
}

// SaveGitHook writes the script and optionally (de)activates it. The file is
// always made executable when enabled — git refuses to run non-executable hooks.
func (GitService) SaveGitHook(repoPath string, name string, content string, enable bool) error {
	if strings.TrimSpace(repoPath) == "" {
		return errors.New("repository path is required")
	}
	active, disabled, err := resolveHookPaths(repoPath, name)
	if err != nil {
		return err
	}
	if len(content) > hookContentLimit {
		return fmt.Errorf("脚本超过 %d KB，请拆分或改用外部脚本", hookContentLimit/1024)
	}
	if strings.TrimSpace(content) == "" {
		return errors.New("脚本内容不能为空")
	}

	if err := os.MkdirAll(filepath.Dir(active), 0755); err != nil {
		return err
	}

	target := active
	if !enable {
		target = disabled
	}
	perm := os.FileMode(0644)
	if enable {
		perm = 0755
	}
	if err := os.WriteFile(target, []byte(content), perm); err != nil {
		return fmt.Errorf("写入钩子脚本失败: %w", err)
	}
	// Writing the active file requires removing the disabled twin, otherwise
	// ToggleGitHook would later see two copies.
	if enable {
		_ = os.Remove(disabled)
	} else {
		_ = os.Remove(active)
	}
	return nil
}
