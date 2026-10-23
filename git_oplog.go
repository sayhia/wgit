package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// =========================================================================
// 操作日志与撤销
//
// git 自己就把每次 HEAD 移动记在 reflog 里——不需要再维护一份元数据。这里
// 做的是「把 reflog 翻译成人话」+「一个安全的撤销按钮」：
//
//   · 只撤销改变了提交内容的位置移动（commit / amend / merge / cherry-pick /
//     revert / reset），checkout 与 rebase 明确拒绝并给出替代路径；
//   · 撤销用 `reset --soft`，改动全部退回暂存区，绝不 hard reset 抹掉工作区；
//   · 有未解决冲突时直接拒绝。
// =========================================================================

type OperationLogEntry struct {
	Index       int    `json:"index"` // 0 = 最新
	Hash        string `json:"hash"`
	ShortHash   string `json:"shortHash"`
	Selector    string `json:"selector"`
	Action      string `json:"action"`
	ActionLabel string `json:"actionLabel"`
	Subject     string `json:"subject"`
	Time        string `json:"time"`
	Undoable    bool   `json:"undoable"`
	UndoHint    string `json:"undoHint"`
}

type OperationLogResult struct {
	Entries []OperationLogEntry `json:"entries"`
	Total   int                 `json:"total"`
	CanUndo bool                `json:"canUndo"`
	Message string              `json:"message"`
}

type UndoResult struct {
	Success   bool   `json:"success"`
	Action    string `json:"action"`
	FromHash  string `json:"fromHash"`
	ToHash    string `json:"toHash"`
	KeptDirty bool   `json:"keptDirty"`
	Message   string `json:"message"`
}

// classifyReflog turns a raw `%gs` subject into something a UI can render and
// decide on.
func classifyReflog(action string) (key, label string, undoable bool, hint string) {
	lower := strings.ToLower(strings.TrimSpace(action))
	switch {
	case strings.HasPrefix(lower, "commit (amend)"):
		return "amend", "修正提交", true, "撤销后改动退回暂存区，可重新编辑提交信息"
	case strings.HasPrefix(lower, "commit (initial)"):
		return "commit-initial", "首次提交", true, "这是第一个提交，撤销需要保留文件到暂存区；若已无更早位置则不可撤销"
	case strings.HasPrefix(lower, "commit (merge)"):
		return "merge-commit", "合并提交", true, "撤销后合入的文件保留为暂存改动"
	case strings.HasPrefix(lower, "commit"):
		return "commit", "提交", true, "撤销后该提交的改动退回暂存区"
	case strings.HasPrefix(lower, "merge"):
		return "merge", "合并", true, "撤销回到合并前，合入的文件保留为暂存改动"
	case strings.HasPrefix(lower, "cherry-pick"):
		return "cherry-pick", "Cherry-Pick", true, "撤销会移除拣选结果，改动保留"
	case strings.HasPrefix(lower, "revert"):
		return "revert", "回滚提交", true, "撤销会移除这条回滚提交"
	case strings.HasPrefix(lower, "reset"):
		return "reset", "Reset", true, "撤销会回到 reset 之前的位置；若当时是 hard reset，丢弃的未提交改动无法恢复"
	case strings.HasPrefix(lower, "rebase"):
		return "rebase", "变基", false, "变基涉及多步重放，请在下方 reflog 里挑目标提交手动恢复"
	case strings.HasPrefix(lower, "checkout"):
		return "checkout", "切换分支", false, "切换分支不是内容改动；如需回去，用 reflog 的「检出」"
	case strings.HasPrefix(lower, "pull"):
		return "pull", "Pull", false, "Pull 可能含合并或变基，请在 reflog 中手动恢复"
	case strings.HasPrefix(lower, "clone"):
		return "clone", "克隆", false, ""
	case strings.HasPrefix(lower, "branch"):
		return "branch", "分支操作", false, ""
	default:
		return "other", strings.TrimSpace(action), false, ""
	}
}

// GetOperationLog reads the reflog and annotates each entry with an undo
// verdict, so the UI never has to guess what is safe.
func (GitService) GetOperationLog(repoPath string, limit int) (*OperationLogResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if limit <= 0 || limit > 200 {
		limit = 40
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	out, err := ExecGit(repoPath, "reflog", "--date=iso", "-n", strconv.Itoa(limit),
		"--format=%H%x00%gd%x00%gs%x00%ad")
	if err != nil {
		// A repository with no commits yet has no reflog; that is not an error.
		// Any other failure must surface instead of pretending history is empty.
		if _, headErr := ExecGit(repoPath, "rev-parse", "--verify", "HEAD"); headErr != nil {
			return &OperationLogResult{Entries: []OperationLogEntry{}, Message: "还没有任何操作记录。"}, nil
		}
		return nil, fmt.Errorf("读取操作日志失败: %w", err)
	}

	res := &OperationLogResult{Entries: []OperationLogEntry{}}
	index := 0
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Subjects may contain tabs; NUL is the only separator git never emits.
		parts := strings.SplitN(line, "\x00", 4)
		if len(parts) < 4 {
			continue
		}
		hash := strings.TrimSpace(parts[0])
		selector := strings.TrimSpace(parts[1])
		subject := strings.TrimSpace(parts[2])
		when := strings.TrimSpace(parts[3])

		action := subject
		if idx := strings.Index(subject, ":"); idx > 0 {
			action = strings.TrimSpace(subject[:idx])
		}
		key, label, undoable, hint := classifyReflog(action)

		res.Entries = append(res.Entries, OperationLogEntry{
			Index:       index,
			Hash:        hash,
			ShortHash:   shortHashOf(hash),
			Selector:    selector,
			Action:      key,
			ActionLabel: label,
			Subject:     subject,
			Time:        when,
			Undoable:    undoable,
			UndoHint:    hint,
		})
		index++
	}
	res.Total = len(res.Entries)

	// Undo needs a target: the position before the newest entry.
	if len(res.Entries) >= 2 && res.Entries[0].Undoable {
		res.CanUndo = true
		res.Message = fmt.Sprintf("最近一次操作是「%s」，可撤销回 %s。",
			res.Entries[0].ActionLabel, res.Entries[1].ShortHash)
	} else if len(res.Entries) == 0 {
		res.Message = "还没有任何操作记录。"
	} else if len(res.Entries) == 1 {
		res.Message = "只有一条记录（首次提交），没有可回退的位置。"
	} else if !res.Entries[0].Undoable {
		res.Message = fmt.Sprintf("最近一次操作是「%s」，不适合一键撤销：%s",
			res.Entries[0].ActionLabel, res.Entries[0].UndoHint)
	}
	return res, nil
}

// UndoLastOperation performs a soft reset to the previous reflog position.
// Nothing is ever destroyed: every change lands in the index.
func (GitService) UndoLastOperation(repoPath string) (*UndoResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}

	// Never undo on top of an unresolved conflict — the user would lose track
	// of which state the files belong to.
	conflicts, err := ExecGit(repoPath, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, fmt.Errorf("检查工作区冲突失败: %w", err)
	}
	if strings.TrimSpace(conflicts) != "" {
		return nil, errors.New("工作区还有未解决的冲突，请先处理完再撤销")
	}

	log, err := (GitService{}).GetOperationLog(repoPath, 5)
	if err != nil {
		return nil, err
	}
	if len(log.Entries) < 2 {
		return nil, errors.New("没有可回退的上一个位置")
	}
	newest := log.Entries[0]
	previous := log.Entries[1]
	if !newest.Undoable {
		hint := newest.UndoHint
		if hint == "" {
			hint = "该操作不支持一键撤销"
		}
		return nil, fmt.Errorf("最近一次操作是「%s」，%s", newest.ActionLabel, hint)
	}

	if _, err := ExecGit(repoPath, "reset", "--soft", previous.Hash); err != nil {
		return nil, fmt.Errorf("撤销失败: %w", err)
	}

	msg := fmt.Sprintf("已撤销「%s」，HEAD 回到 %s；本次撤销不会触碰工作区，当前改动保留在暂存区。",
		newest.ActionLabel, previous.ShortHash)
	if newest.Action == "reset" {
		msg += " 若原 Reset 为 --hard，当时丢弃的未提交改动无法恢复。"
	}
	return &UndoResult{
		Success:   true,
		Action:    newest.ActionLabel,
		FromHash:  newest.ShortHash,
		ToHash:    previous.ShortHash,
		KeptDirty: true,
		Message:   msg,
	}, nil
}
