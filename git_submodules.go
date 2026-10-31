// git_submodules.go — 自 git_service.go 抽离（submodule 族），
// 见 docs/big-files-assessment.md。函数级搬移，零逻辑改动。

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetSubmodules returns all git submodules registered in the repository
func (GitService) GetSubmodules(repoPath string) ([]SubmoduleInfo, error) {
	gitmodulesPath := filepath.Join(repoPath, ".gitmodules")
	if _, err := os.Stat(gitmodulesPath); os.IsNotExist(err) {
		return []SubmoduleInfo{}, nil
	}

	raw, err := ExecGit(repoPath, "submodule", "status")
	if err != nil {
		return nil, fmt.Errorf("读取子模块状态失败: %w", err)
	}

	// Prefetch every submodule URL and path in at most two git invocations
	// instead of running two config lookups per submodule in the loop below.
	// Submodule names may contain dots, so split the key on the fixed
	// "submodule." prefix and the ".url"/".path" suffix only.
	nameToPath := map[string]string{}
	nameToURL := map[string]string{}
	localNameToURL := map[string]string{}

	parseSubmoduleConfig := func(out string) map[string]map[string]string {
		values := map[string]map[string]string{}
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimPrefix(parts[0], "submodule.")
			value := strings.TrimSpace(parts[1])
			switch {
			case strings.HasSuffix(key, ".url"):
				name := strings.TrimSuffix(key, ".url")
				if name == "" {
					continue
				}
				if values[name] == nil {
					values[name] = map[string]string{}
				}
				values[name]["url"] = value
			case strings.HasSuffix(key, ".path"):
				name := strings.TrimSuffix(key, ".path")
				if name == "" {
					continue
				}
				if values[name] == nil {
					values[name] = map[string]string{}
				}
				values[name]["path"] = value
			}
		}
		return values
	}

	// `git config --get-regexp` exits with status 1 when nothing matches;
	// treat that (and any other .gitmodules read failure) as "no entries".
	if out, cfgErr := ExecGit(repoPath, "config", "-f", ".gitmodules", "--get-regexp", `^submodule\..*\.(url|path)$`); cfgErr == nil {
		for name, kv := range parseSubmoduleConfig(out) {
			if p := kv["path"]; p != "" {
				nameToPath[name] = p
			}
			if u := kv["url"]; u != "" {
				nameToURL[name] = u
			}
		}
	}
	// Local config is a best-effort fallback, matching the previous behavior.
	if out, cfgErr := ExecGit(repoPath, "config", "--get-regexp", `^submodule\..*\.url$`); cfgErr == nil {
		for name, kv := range parseSubmoduleConfig(out) {
			if u := kv["url"]; u != "" {
				localNameToURL[name] = u
			}
		}
	}

	pathToURL := map[string]string{}
	for name, subPath := range nameToPath {
		url := nameToURL[name]
		if url == "" {
			url = localNameToURL[name]
		}
		pathToURL[subPath] = url
	}
	// A local-only entry (typically keyed by path when the name equals the
	// path) still resolves the way the per-submodule fallback used to.
	for name, url := range localNameToURL {
		if pathToURL[name] == "" {
			pathToURL[name] = url
		}
	}

	list := []SubmoduleInfo{}
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// status character: ' ' (synced), '+' (modified commit), '-' (not initialized), 'U' (conflict)
		statusChar := line[0]
		statusStr := "synced"
		switch statusChar {
		case '-':
			statusStr = "uninitialized"
		case '+':
			statusStr = "modified"
		case 'U':
			statusStr = "conflict"
		}

		fields := strings.Fields(line)
		commitHash := ""
		subPath := ""
		branch := ""

		if len(fields) >= 1 {
			commitHash = strings.TrimPrefix(fields[0], "-")
			commitHash = strings.TrimPrefix(commitHash, "+")
			commitHash = strings.TrimPrefix(commitHash, "U")
		}
		if len(fields) >= 2 {
			subPath = fields[1]
		}
		if len(fields) >= 3 {
			branch = strings.Trim(fields[2], "()")
		}

		// URL comes from the prefetched maps. When .gitmodules exists but no
		// URL could be resolved (config read failure or an entry without an
		// url key), surface "(unreadable)" instead of silently showing an
		// empty URL — the frontend renders this string verbatim.
		subURL := strings.TrimSpace(pathToURL[subPath])
		if subURL == "" {
			subURL = "(unreadable)"
		}

		name := filepath.Base(subPath)
		list = append(list, SubmoduleInfo{
			Name:   name,
			Path:   subPath,
			URL:    subURL,
			Branch: branch,
			Commit: commitHash,
			Status: statusStr,
		})
	}

	return list, nil
}

// UpdateSubmodules runs git submodule update
func (GitService) UpdateSubmodules(repoPath string, init bool, recursive bool) (string, error) {
	args := []string{"submodule", "update"}
	if init {
		args = append(args, "--init")
	}
	if recursive {
		args = append(args, "--recursive")
	}
	return ExecGit(repoPath, args...)
}

// AddSubmodule adds a new git submodule
func (GitService) AddSubmodule(repoPath string, url string, path string) error {
	if url == "" || path == "" {
		return errors.New("url and path cannot be empty")
	}
	_, err := ExecGit(repoPath, "submodule", "add", url, path)
	return err
}

// RemoveSubmodule unbinds and removes a submodule
func (GitService) RemoveSubmodule(repoPath string, path string) error {
	if path == "" {
		return errors.New("submodule path cannot be empty")
	}
	if _, err := ExecGit(repoPath, "submodule", "deinit", "-f", "--", path); err != nil {
		return fmt.Errorf("取消初始化子模块失败: %w", err)
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".gitmodules")); err == nil {
		// git rm refuses to touch a .gitmodules with unstaged edits (e.g. branch config).
		if _, err := ExecGit(repoPath, "add", "--", ".gitmodules"); err != nil {
			return err
		}
	}
	_, err := ExecGit(repoPath, "rm", "-f", "--", path)
	if err != nil {
		return err
	}
	if data, readErr := os.ReadFile(filepath.Join(repoPath, ".gitmodules")); readErr == nil && !strings.Contains(string(data), "[submodule") {
		if removeErr := os.Remove(filepath.Join(repoPath, ".gitmodules")); removeErr != nil {
			return fmt.Errorf("子模块已移除，但清理空 .gitmodules 失败: %w", removeErr)
		}
		if _, addErr := ExecGit(repoPath, "add", "-A", "--", ".gitmodules"); addErr != nil {
			return fmt.Errorf("子模块已移除，但暂存 .gitmodules 删除失败: %w", addErr)
		}
	}
	return nil
}

// SyncSubmodule synchronizes submodules and updates recursively
func (GitService) SyncSubmodule(repoPath string, submodulePath string) (string, error) {
	args := []string{"submodule", "sync"}
	if submodulePath != "" {
		args = append(args, "--", submodulePath)
	}
	syncOut, err := ExecGit(repoPath, args...)
	if err != nil {
		return syncOut, err
	}

	updateArgs := []string{"submodule", "update", "--init", "--recursive"}
	if submodulePath != "" {
		updateArgs = append(updateArgs, "--", submodulePath)
	}
	updateOut, err := ExecGit(repoPath, updateArgs...)
	if err != nil {
		return syncOut + "\n" + updateOut, err
	}
	return syncOut + "\n" + updateOut, nil
}

// SetSubmoduleBranch updates tracking branch for a submodule
func (GitService) SetSubmoduleBranch(repoPath string, submodulePath string, branch string) error {
	if submodulePath == "" || branch == "" {
		return errors.New("submodule path and branch are required")
	}
	_, err := ExecGit(repoPath, "config", "-f", ".gitmodules", fmt.Sprintf("submodule.%s.branch", submodulePath), branch)
	return err
}

// UpdateSubmodulesRecursive updates all submodules recursively with init
func (GitService) UpdateSubmodulesRecursive(repoPath string) (string, error) {
	return ExecGit(repoPath, "submodule", "update", "--init", "--recursive")
}

// UpdateSubmoduleRemote fetches and fast-forwards submodule to its tracked remote branch
func (GitService) UpdateSubmoduleRemote(repoPath string, submodulePath string) (string, error) {
	args := []string{"submodule", "update", "--remote"}
	if submodulePath != "" {
		args = append(args, "--", submodulePath)
	}
	return ExecGit(repoPath, args...)
}
