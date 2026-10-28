package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// =========================================================================
// 提交内容安全规则（可配置）
//
// AuditCommitContentSafety 原来把 5 个违禁词写死在函数体里：用户看到违规
// 提示，却无法增加自己团队的敏感词，也无法去掉不适用的词。这里把规则
// 落到 .git/wgit-commit-safety.json，并保留一份内置默认值——规则文件缺失
// 或损坏时回退，而不是让审计整体失败。
// =========================================================================

const commitSafetyRulesFile = "wgit-commit-safety.json"

const (
	maxCommitSafetyTerms     = 200
	maxCommitSafetyTermBytes = 200
)

type CommitSafetyRules struct {
	Terms   []string `json:"terms"`
	Source  string   `json:"source"` // "default" | "custom"
	Path    string   `json:"path"`
	Message string   `json:"message"`
}

// defaultCommitSafetyTerms is the built-in list. It stays small on purpose:
// a default rule set that fires on innocent commits trains users to ignore
// the check entirely.
func defaultCommitSafetyTerms() []string {
	return []string{"fuck", "shit", "password=", "secret=", "project_titan_confidential"}
}

func commitSafetyRulesPath(repoPath string) string {
	return filepath.Join(gitPath(repoPath, ""), commitSafetyRulesFile)
}

// normalizeCommitSafetyTerms trims, drops blanks and de-duplicates while
// keeping the caller's order — a rule list is something users scan, so
// reordering it arbitrarily would be disorienting.
func normalizeCommitSafetyTerms(terms []string) []string {
	seen := make(map[string]bool, len(terms))
	out := make([]string, 0, len(terms))
	for _, raw := range terms {
		term := strings.TrimSpace(raw)
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	return out
}

func validateCommitSafetyTerms(terms []string) error {
	if len(terms) == 0 {
		return errors.New("规则至少需要一条")
	}
	if len(terms) > maxCommitSafetyTerms {
		return fmt.Errorf("规则最多 %d 条，当前 %d 条", maxCommitSafetyTerms, len(terms))
	}
	for _, term := range terms {
		if len(term) > maxCommitSafetyTermBytes {
			return fmt.Errorf("规则「%.20s…」过长（上限 %d 字节）", term, maxCommitSafetyTermBytes)
		}
	}
	return nil
}

// loadCommitSafetyRules reads the repo's rule file, falling back to the
// built-in list when it is missing or unreadable. A broken file must not break
// the audit: it reports Source="default" so the UI can tell the user why their
// custom words are not being applied.
//
// Unexported on purpose — the audit calls it directly, while the exported
// method below exists so the frontend can read the current rule set.
func loadCommitSafetyRules(repoPath string) (*CommitSafetyRules, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}

	path := commitSafetyRulesPath(repoPath)
	res := &CommitSafetyRules{Path: path, Source: "default", Terms: defaultCommitSafetyTerms()}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			res.Message = "使用内置默认规则；在设置中修改后会写入 .git/wgit-commit-safety.json。"
			return res, nil
		}
		res.Message = fmt.Sprintf("读取规则文件失败（%v），已回退到内置默认规则。", err)
		return res, nil
	}

	var stored struct {
		Terms []string `json:"terms"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		res.Message = "规则文件格式损坏，已回退到内置默认规则；重新保存可覆盖。"
		return res, nil
	}

	terms := normalizeCommitSafetyTerms(stored.Terms)
	if len(terms) == 0 {
		res.Message = "规则文件内容为空，已回退到内置默认规则。"
		return res, nil
	}

	res.Terms = terms
	res.Source = "custom"
	res.Message = fmt.Sprintf("使用自定义规则（%d 条）。", len(terms))
	return res, nil
}

func (GitService) SaveCommitSafetyRules(repoPath string, terms []string) (*CommitSafetyRules, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}

	normalized := normalizeCommitSafetyTerms(terms)
	if err := validateCommitSafetyTerms(normalized); err != nil {
		return nil, err
	}
	// Sorted only for the on-disk representation: it makes diffs in code review
	// readable without changing what the user sees in the editor.
	payload := struct {
		Terms []string `json:"terms"`
	}{Terms: normalized}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化规则失败: %w", err)
	}

	path := commitSafetyRulesPath(repoPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(path, data, 0644); err != nil {
		return nil, fmt.Errorf("写入规则文件失败: %w", err)
	}

	sorted := append([]string(nil), normalized...)
	sort.Strings(sorted)
	return &CommitSafetyRules{
		Terms:   normalized,
		Source:  "custom",
		Path:    path,
		Message: fmt.Sprintf("已保存 %d 条自定义规则。", len(normalized)),
	}, nil
}

// ResetCommitSafetyRules removes the custom file so the built-in list applies
// again — the only way back to defaults without hand-editing JSON.
func (GitService) ResetCommitSafetyRules(repoPath string) (*CommitSafetyRules, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	path := commitSafetyRulesPath(repoPath)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("删除规则文件失败: %w", err)
	}
	rules, err := loadCommitSafetyRules(repoPath)
	if err != nil {
		return nil, err
	}
	rules.Message = "已恢复内置默认规则。"
	return rules, nil
}

// writeFileAtomic writes through a temp file in the same directory, so a
// failure mid-write cannot leave a truncated rules file behind.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".wgit-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// LoadCommitSafetyRules exposes the current rule set to the frontend.
func (GitService) LoadCommitSafetyRules(repoPath string) (*CommitSafetyRules, error) {
	return loadCommitSafetyRules(repoPath)
}
