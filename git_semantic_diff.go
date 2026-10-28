package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type SemanticSymbolChange struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`       // "function", "class", "struct", "interface", "type", "method"
	ChangeType string `json:"changeType"` // "added", "modified", "deleted"
	OldLine    int    `json:"oldLine"`
	NewLine    int    `json:"newLine"`
	Signature  string `json:"signature"`
}

type SemanticDiffOutline struct {
	FilePath      string                 `json:"filePath"`
	Language      string                 `json:"language"`
	Symbols       []SemanticSymbolChange `json:"symbols"`
	TotalAdded    int                    `json:"totalAdded"`
	TotalModified int                    `json:"totalModified"`
	TotalDeleted  int                    `json:"totalDeleted"`
}

type MovedBlockPair struct {
	OldStartLine int    `json:"oldStartLine"`
	OldEndLine   int    `json:"oldEndLine"`
	NewStartLine int    `json:"newStartLine"`
	NewEndLine   int    `json:"newEndLine"`
	LineCount    int    `json:"lineCount"`
	Sample       string `json:"sample"`
}

type MovedBlocksReport struct {
	FilePath        string           `json:"filePath"`
	MovedPairs      []MovedBlockPair `json:"movedPairs"`
	TotalMovedLines int              `json:"totalMovedLines"`
}

func execGitSemantic(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func detectFileLang(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".vue":
		return "vue"
	case ".cpp", ".cc", ".c", ".h", ".hpp":
		return "cpp"
	default:
		return "generic"
	}
}

// GetSemanticDiffOutline extracts symbol changes (functions, types, classes) from a file diff.
func (GitService) GetSemanticDiffOutline(repoPath string, commitA string, commitB string, filePath string) (*SemanticDiffOutline, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, errors.New("filePath cannot be empty")
	}

	commitA = strings.TrimSpace(commitA)
	commitB = strings.TrimSpace(commitB)

	var diffOut string
	if commitA == "" && commitB == "" {
		// Diff unstaged or staged vs HEAD
		diffOut, _ = execGitSemantic(repoPath, "diff", "HEAD", "--", filePath)
		if strings.TrimSpace(diffOut) == "" {
			diffOut, _ = execGitSemantic(repoPath, "diff", "--", filePath)
		}
	} else if commitB == "" {
		diffOut, _ = execGitSemantic(repoPath, "show", commitA, "--", filePath)
	} else {
		diffOut, _ = execGitSemantic(repoPath, "diff", commitA, commitB, "--", filePath)
	}

	lang := detectFileLang(filePath)

	var goFuncRe = regexp.MustCompile(`^(?:\+|\-)?\s*func\s+(?:\([^\)]+\)\s+)?([A-Za-z0-9_]+)\s*\(`)
	var goTypeRe = regexp.MustCompile(`^(?:\+|\-)?\s*type\s+([A-Za-z0-9_]+)\s+(struct|interface)`)
	var tsFuncRe = regexp.MustCompile(`^(?:\+|\-)?\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z0-9_]+)`)
	var tsClassRe = regexp.MustCompile(`^(?:\+|\-)?\s*(?:export\s+)?class\s+([A-Za-z0-9_]+)`)
	var pyDefRe = regexp.MustCompile(`^(?:\+|\-)?\s*def\s+([A-Za-z0-9_]+)\s*\(`)
	var pyClassRe = regexp.MustCompile(`^(?:\+|\-)?\s*class\s+([A-Za-z0-9_]+)`)

	addedSymbols := make(map[string]SemanticSymbolChange)
	deletedSymbols := make(map[string]SemanticSymbolChange)

	lines := strings.Split(diffOut, "\n")
	oldLineNum := 0
	newLineNum := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "@@ ") {
			// Parse chunk header @@ -old,count +new,count @@
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				oldPart := strings.TrimPrefix(parts[1], "-")
				newPart := strings.TrimPrefix(parts[2], "+")
				oldLineNum = parseChunkLine(oldPart)
				newLineNum = parseChunkLine(newPart)
			}
			continue
		}

		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			continue
		}

		isAdd := strings.HasPrefix(line, "+")
		isDel := strings.HasPrefix(line, "-")

		if !isAdd && !isDel {
			oldLineNum++
			newLineNum++
			continue
		}

		var symName, symKind string

		switch lang {
		case "go":
			if m := goFuncRe.FindStringSubmatch(line); len(m) > 1 {
				symName = m[1]
				symKind = "function"
				if strings.Contains(line, ") ") {
					symKind = "method"
				}
			} else if m := goTypeRe.FindStringSubmatch(line); len(m) > 2 {
				symName = m[1]
				symKind = m[2]
			}
		case "typescript", "javascript", "vue":
			if m := tsFuncRe.FindStringSubmatch(line); len(m) > 1 {
				symName = m[1]
				symKind = "function"
			} else if m := tsClassRe.FindStringSubmatch(line); len(m) > 1 {
				symName = m[1]
				symKind = "class"
			}
		case "python":
			if m := pyDefRe.FindStringSubmatch(line); len(m) > 1 {
				symName = m[1]
				symKind = "function"
			} else if m := pyClassRe.FindStringSubmatch(line); len(m) > 1 {
				symName = m[1]
				symKind = "class"
			}
		}

		if symName != "" {
			item := SemanticSymbolChange{
				Name:      symName,
				Kind:      symKind,
				Signature: strings.TrimSpace(strings.TrimPrefix(line, "+")),
			}
			if isAdd {
				item.NewLine = newLineNum
				item.ChangeType = "added"
				addedSymbols[symName] = item
			} else if isDel {
				item.OldLine = oldLineNum
				item.ChangeType = "deleted"
				deletedSymbols[symName] = item
			}
		}

		if isAdd {
			newLineNum++
		} else if isDel {
			oldLineNum++
		}
	}

	var symbols []SemanticSymbolChange
	totalAdded := 0
	totalModified := 0
	totalDeleted := 0

	for name, addSym := range addedSymbols {
		if delSym, exists := deletedSymbols[name]; exists {
			addSym.ChangeType = "modified"
			addSym.OldLine = delSym.OldLine
			symbols = append(symbols, addSym)
			delete(deletedSymbols, name)
			totalModified++
		} else {
			symbols = append(symbols, addSym)
			totalAdded++
		}
	}

	for _, delSym := range deletedSymbols {
		symbols = append(symbols, delSym)
		totalDeleted++
	}

	return &SemanticDiffOutline{
		FilePath:      filePath,
		Language:      lang,
		Symbols:       symbols,
		TotalAdded:    totalAdded,
		TotalModified: totalModified,
		TotalDeleted:  totalDeleted,
	}, nil
}

func parseChunkLine(part string) int {
	sub := strings.Split(part, ",")[0]
	num := 0
	for _, c := range sub {
		if c >= '0' && c <= '9' {
			num = num*10 + int(c-'0')
		}
	}
	return num
}

// DetectMovedBlocks detects sequences of lines that were deleted in one section and inserted in another.
func (GitService) DetectMovedBlocks(repoPath string, commitA string, commitB string, filePath string) (*MovedBlocksReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, errors.New("filePath cannot be empty")
	}

	commitA = strings.TrimSpace(commitA)
	commitB = strings.TrimSpace(commitB)

	var diffOut string
	if commitA == "" && commitB == "" {
		diffOut, _ = execGitSemantic(repoPath, "diff", "HEAD", "--", filePath)
		if strings.TrimSpace(diffOut) == "" {
			diffOut, _ = execGitSemantic(repoPath, "diff", "--", filePath)
		}
	} else if commitB == "" {
		diffOut, _ = execGitSemantic(repoPath, "show", commitA, "--", filePath)
	} else {
		diffOut, _ = execGitSemantic(repoPath, "diff", commitA, commitB, "--", filePath)
	}

	type DiffLineInfo struct {
		LineNum int
		Text    string
	}

	var deletedLines []DiffLineInfo
	var addedLines []DiffLineInfo

	lines := strings.Split(diffOut, "\n")
	oldLineNum := 0
	newLineNum := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "@@ ") {
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				oldLineNum = parseChunkLine(strings.TrimPrefix(parts[1], "-"))
				newLineNum = parseChunkLine(strings.TrimPrefix(parts[2], "+"))
			}
			continue
		}
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			continue
		}

		if strings.HasPrefix(line, "-") {
			clean := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if len(clean) > 3 && clean != "}" && clean != "{" {
				deletedLines = append(deletedLines, DiffLineInfo{LineNum: oldLineNum, Text: clean})
			}
			oldLineNum++
		} else if strings.HasPrefix(line, "+") {
			clean := strings.TrimSpace(strings.TrimPrefix(line, "+"))
			if len(clean) > 3 && clean != "}" && clean != "{" {
				addedLines = append(addedLines, DiffLineInfo{LineNum: newLineNum, Text: clean})
			}
			newLineNum++
		} else {
			oldLineNum++
			newLineNum++
		}
	}

	var movedPairs []MovedBlockPair
	totalMoved := 0

	// Sliding window match of 2 or more lines
	minMatch := 2
	for i := 0; i <= len(deletedLines)-minMatch; i++ {
		for j := 0; j <= len(addedLines)-minMatch; j++ {
			matchLen := 0
			for i+matchLen < len(deletedLines) && j+matchLen < len(addedLines) {
				if deletedLines[i+matchLen].Text == addedLines[j+matchLen].Text {
					matchLen++
				} else {
					break
				}
			}
			if matchLen >= minMatch {
				sample := deletedLines[i].Text
				if len(sample) > 40 {
					sample = sample[:40] + "..."
				}
				pair := MovedBlockPair{
					OldStartLine: deletedLines[i].LineNum,
					OldEndLine:   deletedLines[i+matchLen-1].LineNum,
					NewStartLine: addedLines[j].LineNum,
					NewEndLine:   addedLines[j+matchLen-1].LineNum,
					LineCount:    matchLen,
					Sample:       sample,
				}
				movedPairs = append(movedPairs, pair)
				totalMoved += matchLen
				i += matchLen - 1
				break
			}
		}
	}

	return &MovedBlocksReport{
		FilePath:        filePath,
		MovedPairs:      movedPairs,
		TotalMovedLines: totalMoved,
	}, nil
}
