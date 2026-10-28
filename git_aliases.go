package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// GitAliasItem represents a git command alias.
type GitAliasItem struct {
	Key     string `json:"key"`
	Command string `json:"command"`
	Scope   string `json:"scope"` // "local" or "global"
}

var aliasKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

// GetGitAliases returns all defined git aliases across local (if repo given) and global scopes.
func (GitService) GetGitAliases(repoPath string) ([]GitAliasItem, error) {
	aliasMap := make(map[string]GitAliasItem)

	workingDir := strings.TrimSpace(repoPath)
	if workingDir == "" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			workingDir = home
		} else {
			workingDir = "."
		}
	}

	// 1. Fetch global aliases (ignore error if no aliases set)
	globalOut, _ := ExecGit(workingDir, "config", "--global", "--get-regexp", `^alias\.`)
	parseAliasOutput(globalOut, "global", aliasMap)

	// 2. Fetch local aliases (if valid repo)
	if strings.TrimSpace(repoPath) != "" {
		localOut, _ := ExecGit(repoPath, "config", "--local", "--get-regexp", `^alias\.`)
		parseAliasOutput(localOut, "local", aliasMap)
	}

	var list []GitAliasItem
	for _, item := range aliasMap {
		list = append(list, item)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Scope != list[j].Scope {
			return list[i].Scope < list[j].Scope
		}
		return list[i].Key < list[j].Key
	})

	return list, nil
}

func parseAliasOutput(output string, scope string, dest map[string]GitAliasItem) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimPrefix(parts[0], "alias.")
		cmd := strings.TrimSpace(parts[1])
		dest[scope+":"+key] = GitAliasItem{
			Key:     key,
			Command: cmd,
			Scope:   scope,
		}
	}
}

// SetGitAlias sets or updates an alias in the specified scope ("local" or "global").
func (GitService) SetGitAlias(repoPath string, scope string, key string, command string) error {
	trimmedKey := strings.TrimSpace(key)
	if !aliasKeyRegex.MatchString(trimmedKey) {
		return errors.New("别名只能包含字母、数字、下划线或连字符")
	}
	trimmedCmd := strings.TrimSpace(command)
	if trimmedCmd == "" {
		return errors.New("别名命令内容不能为空")
	}

	normScope := strings.ToLower(strings.TrimSpace(scope))
	if normScope != "local" && normScope != "global" {
		normScope = "global"
	}

	workingDir := strings.TrimSpace(repoPath)
	if normScope == "local" {
		if workingDir == "" {
			return errors.New("本地别名需要指定仓库路径")
		}
		_, err := ExecGit(workingDir, "config", "--local", "alias."+trimmedKey, trimmedCmd)
		if err != nil {
			return fmt.Errorf("保存本地别名失败: %w", err)
		}
		return nil
	}

	if workingDir == "" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			workingDir = home
		} else {
			workingDir = "."
		}
	}

	_, err := ExecGit(workingDir, "config", "--global", "alias."+trimmedKey, trimmedCmd)
	if err != nil {
		return fmt.Errorf("保存全局别名失败: %w", err)
	}
	return nil
}

// DeleteGitAlias removes an alias from the specified scope ("local" or "global").
func (GitService) DeleteGitAlias(repoPath string, scope string, key string) error {
	trimmedKey := strings.TrimSpace(key)
	if !aliasKeyRegex.MatchString(trimmedKey) {
		return errors.New("无效的别名名称")
	}

	normScope := strings.ToLower(strings.TrimSpace(scope))
	if normScope != "local" && normScope != "global" {
		normScope = "global"
	}

	workingDir := strings.TrimSpace(repoPath)
	if normScope == "local" {
		if workingDir == "" {
			return errors.New("本地别名需要指定仓库路径")
		}
		_, err := ExecGit(workingDir, "config", "--local", "--unset", "alias."+trimmedKey)
		if err != nil {
			return fmt.Errorf("删除本地别名失败: %w", err)
		}
		return nil
	}

	if workingDir == "" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			workingDir = home
		} else {
			workingDir = "."
		}
	}

	_, err := ExecGit(workingDir, "config", "--global", "--unset", "alias."+trimmedKey)
	if err != nil {
		return fmt.Errorf("删除全局别名失败: %w", err)
	}
	return nil
}
