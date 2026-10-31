package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// subtreeMetadata persists connection details for a subtree prefix
type subtreeMetadata struct {
	RemoteOrURL string `json:"remoteOrUrl"`
	Branch      string `json:"branch"`
	Squash      bool   `json:"squash"`
}

// SubtreeSplitResult represents the outcome of splitting a directory into a subtree branch/commit
type SubtreeSplitResult struct {
	Success    bool   `json:"success"`
	Prefix     string `json:"prefix"`
	NewCommit  string `json:"newCommit"`
	BranchName string `json:"branchName"`
	Message    string `json:"message"`
	Output     string `json:"output"`
}

func subtreeMetadataPath(repoPath string) string {
	return filepath.Join(gitCommonDir(repoPath), "wgit-subtrees.json")
}

func cleanSubtreePrefix(prefix string) string {
	p := strings.TrimSpace(prefix)
	p = strings.Trim(p, "/\\")
	if p == "" || p == "." {
		return ""
	}
	p = filepath.ToSlash(filepath.Clean(p))
	p = strings.Trim(p, "/")
	if p == "." {
		return ""
	}
	return p
}

func isHexSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func readSubtreeMetadata(repoPath string) (map[string]subtreeMetadata, error) {
	metas := make(map[string]subtreeMetadata)
	data, err := os.ReadFile(subtreeMetadataPath(repoPath))
	if err != nil {
		if os.IsNotExist(err) {
			return metas, nil
		}
		return metas, fmt.Errorf("读取子树元数据失败: %w", err)
	}
	if err := json.Unmarshal(data, &metas); err != nil {
		return nil, fmt.Errorf("子树元数据文件 %s 已损坏，已拒绝覆盖；请备份或删除后重试: %w",
			subtreeMetadataPath(repoPath), err)
	}
	return metas, nil
}

func saveSubtreeMetadata(repoPath string, prefix string, meta subtreeMetadata) error {
	cleanPrefix := cleanSubtreePrefix(prefix)
	if cleanPrefix == "" {
		return errors.New("子树前缀路径不能为空")
	}
	metas, err := readSubtreeMetadata(repoPath)
	if err != nil {
		return err
	}
	if prefix != cleanPrefix {
		delete(metas, prefix)
	}
	metas[cleanPrefix] = meta
	data, err := json.MarshalIndent(metas, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(subtreeMetadataPath(repoPath), data, 0644)
}

func findSubtreeMetadata(metas map[string]subtreeMetadata, prefix string) (subtreeMetadata, bool) {
	clean := cleanSubtreePrefix(prefix)
	if m, ok := metas[clean]; ok {
		return m, true
	}
	if m, ok := metas[prefix]; ok {
		return m, true
	}
	for k, m := range metas {
		if cleanSubtreePrefix(k) == clean {
			return m, true
		}
	}
	return subtreeMetadata{}, false
}

// ListSubtrees discovers git-subtree components configured in repository history and metadata
func (GitService) ListSubtrees(repoPath string) (*SubtreeListResult, error) {
	metadata, err := readSubtreeMetadata(repoPath)
	if err != nil {
		return nil, err
	}

	subtreeMap := make(map[string]*SubtreeItem)

	// Fetch up to 200 commits with git-subtree-dir markers, using record separator \x1e to isolate multiline bodies.
	// Empty output with nil error means "no subtree commits" and stays a valid empty list;
	// a failing git log (broken repo, invalid path) must surface to the caller instead of
	// silently degrading into an empty list.
	out, err := ExecGit(repoPath, "log", "--grep=git-subtree-dir:", "-n", "200", "--pretty=format:%H%x00%s%x00%b%x1e")
	if err != nil {
		return nil, fmt.Errorf("list subtrees: %w", err)
	}
	if len(strings.TrimSpace(out)) > 0 {
		records := strings.Split(out, "\x1e")
		for _, record := range records {
			record = strings.TrimSpace(record)
			if record == "" {
				continue
			}
			parts := strings.Split(record, "\x00")
			if len(parts) >= 3 {
				commitHash := strings.TrimSpace(parts[0])
				body := parts[2]

				var dir string
				var split string
				for _, line := range strings.Split(body, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "git-subtree-dir:") {
						dir = strings.TrimSpace(strings.TrimPrefix(line, "git-subtree-dir:"))
					}
					if strings.HasPrefix(line, "git-subtree-split:") {
						split = strings.TrimSpace(strings.TrimPrefix(line, "git-subtree-split:"))
					}
				}

				cleanDir := cleanSubtreePrefix(dir)
				if cleanDir == "" {
					continue
				}

				// Check physical directory existence in workspace
				targetDir := filepath.Join(repoPath, filepath.FromSlash(cleanDir))
				fi, statErr := os.Stat(targetDir)
				if statErr != nil || !fi.IsDir() {
					continue
				}

				shortCommit := commitHash
				if len(shortCommit) > 8 {
					shortCommit = shortCommit[:8]
				}

				if existing, exists := subtreeMap[cleanDir]; exists {
					if existing.LastSquash == "" && split != "" {
						existing.LastSquash = split
					}
				} else {
					meta, _ := findSubtreeMetadata(metadata, cleanDir)
					branch := meta.Branch
					if branch == "" {
						branch = "main"
					}
					subtreeMap[cleanDir] = &SubtreeItem{
						Prefix:      cleanDir,
						RemoteOrURL: meta.RemoteOrURL,
						Branch:      branch,
						LastCommit:  shortCommit,
						LastSquash:  split,
					}
				}
			}
		}
	}

	// Bidirectional merge: check metadata entries that may not appear in recent git log
	for prefix, meta := range metadata {
		cleanPrefix := cleanSubtreePrefix(prefix)
		if cleanPrefix == "" {
			continue
		}
		targetDir := filepath.Join(repoPath, filepath.FromSlash(cleanPrefix))
		fi, statErr := os.Stat(targetDir)
		if statErr != nil || !fi.IsDir() {
			continue
		}

		if existing, exists := subtreeMap[cleanPrefix]; exists {
			if existing.RemoteOrURL == "" && meta.RemoteOrURL != "" {
				existing.RemoteOrURL = meta.RemoteOrURL
			}
			if meta.Branch != "" {
				existing.Branch = meta.Branch
			}
		} else {
			branch := meta.Branch
			if branch == "" {
				branch = "main"
			}
			subtreeMap[cleanPrefix] = &SubtreeItem{
				Prefix:      cleanPrefix,
				RemoteOrURL: meta.RemoteOrURL,
				Branch:      branch,
				LastCommit:  "",
				LastSquash:  "",
			}
		}
	}

	list := make([]SubtreeItem, 0, len(subtreeMap))
	for _, it := range subtreeMap {
		list = append(list, *it)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Prefix < list[j].Prefix
	})

	return &SubtreeListResult{Subtrees: list}, nil
}

// AddSubtree adds an external git repository as a subtree component
func (GitService) AddSubtree(repoPath string, prefix string, remoteOrUrl string, branch string, squash bool) (*SubtreeActionResult, error) {
	prefix = cleanSubtreePrefix(prefix)
	remoteOrUrl = strings.TrimSpace(remoteOrUrl)
	branch = strings.TrimSpace(branch)
	if prefix == "" || remoteOrUrl == "" || branch == "" {
		return nil, errors.New("子树前缀路径、仓库地址与分支不能为空")
	}

	args := []string{"subtree", "add", "--prefix=" + prefix, remoteOrUrl, branch}
	if squash {
		args = append(args, "--squash")
	}

	out, err := ExecGitWithTimeout(repoPath, 10*time.Minute, args...)
	if err != nil {
		return &SubtreeActionResult{
			Success: false,
			Message: fmt.Sprintf("添加 Subtree 失败: %v", err),
			Output:  out,
		}, err
	}

	msg := fmt.Sprintf("成功将子树 %s 集成至目录 %s", remoteOrUrl, prefix)
	if err := saveSubtreeMetadata(repoPath, prefix, subtreeMetadata{
		RemoteOrURL: remoteOrUrl,
		Branch:      branch,
		Squash:      squash,
	}); err != nil {
		msg += fmt.Sprintf("；但本地子树信息保存失败（%v），下次列表可能缺少远端/分支信息", err)
	}
	return &SubtreeActionResult{
		Success: true,
		Message: msg,
		Output:  out,
	}, nil
}

// PullSubtree pulls updates for an integrated subtree component
func (GitService) PullSubtree(repoPath string, prefix string, remoteOrUrl string, branch string, squash bool) (*SubtreeActionResult, error) {
	prefix = cleanSubtreePrefix(prefix)
	remoteOrUrl = strings.TrimSpace(remoteOrUrl)
	branch = strings.TrimSpace(branch)
	if prefix == "" || remoteOrUrl == "" || branch == "" {
		return nil, errors.New("子树前缀路径、仓库地址与分支不能为空")
	}

	args := []string{"subtree", "pull", "--prefix=" + prefix, remoteOrUrl, branch}
	if squash {
		args = append(args, "--squash")
	}

	out, err := ExecGitWithTimeout(repoPath, 10*time.Minute, args...)
	if err != nil {
		return &SubtreeActionResult{
			Success: false,
			Message: fmt.Sprintf("拉取 Subtree 失败: %v", err),
			Output:  out,
		}, err
	}

	msg := fmt.Sprintf("成功从远端同步子树 %s 最新更新", prefix)
	if err := saveSubtreeMetadata(repoPath, prefix, subtreeMetadata{
		RemoteOrURL: remoteOrUrl,
		Branch:      branch,
		Squash:      squash,
	}); err != nil {
		msg += fmt.Sprintf("；但本地子树信息保存失败（%v），下次列表可能缺少远端/分支信息", err)
	}
	return &SubtreeActionResult{
		Success: true,
		Message: msg,
		Output:  out,
	}, nil
}

// PushSubtree pushes local changes back to the upstream subtree repository
func (GitService) PushSubtree(repoPath string, prefix string, remoteOrUrl string, branch string) (*SubtreeActionResult, error) {
	prefix = cleanSubtreePrefix(prefix)
	remoteOrUrl = strings.TrimSpace(remoteOrUrl)
	branch = strings.TrimSpace(branch)
	if prefix == "" || remoteOrUrl == "" || branch == "" {
		return nil, errors.New("子树前缀路径、仓库地址与分支不能为空")
	}

	args := []string{"subtree", "push", "--prefix=" + prefix, remoteOrUrl, branch}
	out, err := ExecGitWithTimeout(repoPath, 10*time.Minute, args...)
	if err != nil {
		return &SubtreeActionResult{
			Success: false,
			Message: fmt.Sprintf("推送 Subtree 失败: %v", err),
			Output:  out,
		}, err
	}

	existing := subtreeMetadata{}
	if metas, metaErr := readSubtreeMetadata(repoPath); metaErr == nil {
		existing, _ = findSubtreeMetadata(metas, prefix)
	}
	msg := fmt.Sprintf("成功将子树 %s 变更推送回上游", prefix)
	if err := saveSubtreeMetadata(repoPath, prefix, subtreeMetadata{
		RemoteOrURL: remoteOrUrl,
		Branch:      branch,
		Squash:      existing.Squash,
	}); err != nil {
		msg += fmt.Sprintf("；但本地子树信息保存失败（%v），下次列表可能缺少远端/分支信息", err)
	}
	return &SubtreeActionResult{
		Success: true,
		Message: msg,
		Output:  out,
	}, nil
}

// SplitSubtree extracts history of a subtree prefix into a separate commit or branch
func (GitService) SplitSubtree(repoPath string, prefix string, branchName string, squash bool, rejoin bool) (*SubtreeSplitResult, error) {
	prefix = cleanSubtreePrefix(prefix)
	if prefix == "" {
		return nil, errors.New("子树前缀路径不能为空")
	}
	branchName = strings.TrimSpace(branchName)

	args := []string{"subtree", "split", "--prefix=" + prefix}
	if branchName != "" {
		args = append(args, "-b", branchName)
	}
	if squash {
		args = append(args, "--squash")
	}
	if rejoin {
		args = append(args, "--rejoin")
	}

	out, err := ExecGitWithTimeout(repoPath, 10*time.Minute, args...)
	if err != nil {
		return &SubtreeSplitResult{
			Success:    false,
			Prefix:     prefix,
			BranchName: branchName,
			Message:    fmt.Sprintf("拆分子树失败: %v", err),
			Output:     out,
		}, err
	}

	var newCommit string
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 40 && isHexSHA(trimmed) {
			newCommit = trimmed
		}
	}
	if newCommit == "" && branchName != "" {
		if rev, revErr := ExecGit(repoPath, "rev-parse", branchName); revErr == nil {
			trimmedRev := strings.TrimSpace(rev)
			if len(trimmedRev) == 40 && isHexSHA(trimmedRev) {
				newCommit = trimmedRev
			}
		}
	}

	shortSha := newCommit
	if len(shortSha) > 8 {
		shortSha = shortSha[:8]
	}

	var msg string
	if branchName != "" {
		if shortSha != "" {
			msg = fmt.Sprintf("成功将子树 %s 拆分为新分支 %s (%s)", prefix, branchName, shortSha)
		} else {
			msg = fmt.Sprintf("成功将子树 %s 拆分为新分支 %s", prefix, branchName)
		}
	} else {
		if shortSha != "" {
			msg = fmt.Sprintf("成功将子树 %s 拆分为独立提交 (%s)", prefix, shortSha)
		} else {
			msg = fmt.Sprintf("成功将子树 %s 拆分完成", prefix)
		}
	}

	return &SubtreeSplitResult{
		Success:    true,
		Prefix:     prefix,
		NewCommit:  newCommit,
		BranchName: branchName,
		Message:    msg,
		Output:     out,
	}, nil
}

// UpdateSubtreeMetadata updates or stores remote and branch metadata for a subtree prefix
func (GitService) UpdateSubtreeMetadata(repoPath string, prefix string, remoteOrUrl string, branch string, squash bool) error {
	prefix = cleanSubtreePrefix(prefix)
	if prefix == "" {
		return errors.New("子树前缀路径不能为空")
	}
	remoteOrUrl = strings.TrimSpace(remoteOrUrl)
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	return saveSubtreeMetadata(repoPath, prefix, subtreeMetadata{
		RemoteOrURL: remoteOrUrl,
		Branch:      branch,
		Squash:      squash,
	})
}

// UnlinkSubtree removes subtree metadata from configuration without modifying working tree files
func (GitService) UnlinkSubtree(repoPath string, prefix string) error {
	prefix = cleanSubtreePrefix(prefix)
	if prefix == "" {
		return errors.New("子树前缀路径不能为空")
	}
	metas, err := readSubtreeMetadata(repoPath)
	if err != nil {
		return err
	}
	delete(metas, prefix)
	for k := range metas {
		if cleanSubtreePrefix(k) == prefix {
			delete(metas, k)
		}
	}
	data, err := json.MarshalIndent(metas, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(subtreeMetadataPath(repoPath), data, 0644)
}
