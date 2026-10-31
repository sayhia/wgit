// git_blame_history.go — 自 git_service.go 抽离（blame/大对象族），
// 见 docs/big-files-assessment.md。函数级搬移，零逻辑改动。

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GetFileBlameWithOptions returns extended git blame line by line with commit, author, timestamp, previous commit, and boundary flag
func (GitService) GetFileBlameWithOptions(repoPath string, opts BlameOptions) ([]BlameLine, error) {
	if strings.TrimSpace(opts.FilePath) == "" {
		return nil, errors.New("file path is required")
	}

	cleanPath, err := repoRelativePathFromInput(repoPath, opts.FilePath)
	if err != nil {
		return nil, err
	}
	cleanPath = filepath.ToSlash(cleanPath)

	args := []string{"blame", "--line-porcelain"}

	start := opts.StartLine
	end := opts.EndLine
	if start > 0 && end > 0 && start > end {
		start, end = end, start
	}
	if start > 0 && end > 0 {
		args = append(args, "-L", fmt.Sprintf("%d,%d", start, end))
	} else if start > 0 {
		args = append(args, "-L", fmt.Sprintf("%d,", start))
	} else if end > 0 {
		args = append(args, "-L", fmt.Sprintf("1,%d", end))
	}

	if strings.TrimSpace(opts.Ref) != "" {
		args = append(args, strings.TrimSpace(opts.Ref))
	}

	args = append(args, "--", cleanPath)

	raw, err := ExecGit(repoPath, args...)
	if err != nil {
		return nil, err
	}

	var result []BlameLine
	if strings.TrimSpace(raw) == "" {
		return []BlameLine{}, nil
	}

	lines := strings.Split(raw, "\n")

	var currentHash, currentShort, currentAuthor, currentAuthorEmail, currentAuthorTz, currentDate, currentSummary string
	var currentAuthorTimestamp int64
	var currentPreviousCommit, currentPreviousFilename string
	var currentIsBoundary bool
	var currentLineNum int
	fallbackLineNum := 1

	for _, line := range lines {
		if len(line) >= 40 && !strings.Contains(line[:40], " ") && strings.Contains(line, " ") {
			// First line of header: <hash> <orig_line> <final_line> [group_line_count]
			pts := strings.Fields(line)
			if len(pts) >= 3 {
				currentHash = pts[0]
				if len(currentHash) >= 7 {
					currentShort = currentHash[:7]
				} else {
					currentShort = currentHash
				}
				if num, err := strconv.Atoi(pts[2]); err == nil {
					currentLineNum = num
				} else {
					currentLineNum = fallbackLineNum
				}
			}
			currentPreviousCommit = ""
			currentPreviousFilename = ""
			currentIsBoundary = false
			continue
		}

		if strings.HasPrefix(line, "author ") {
			currentAuthor = strings.TrimPrefix(line, "author ")
		} else if strings.HasPrefix(line, "author-mail ") {
			mail := strings.TrimSpace(strings.TrimPrefix(line, "author-mail "))
			currentAuthorEmail = strings.Trim(mail, "<>")
		} else if strings.HasPrefix(line, "author-time ") {
			tsStr := strings.TrimSpace(strings.TrimPrefix(line, "author-time "))
			ts, err := strconv.ParseInt(tsStr, 10, 64)
			if err == nil {
				currentAuthorTimestamp = ts
				currentDate = time.Unix(ts, 0).Format("2006-01-02")
			}
		} else if strings.HasPrefix(line, "author-tz ") {
			currentAuthorTz = strings.TrimSpace(strings.TrimPrefix(line, "author-tz "))
		} else if strings.HasPrefix(line, "summary ") {
			currentSummary = strings.TrimPrefix(line, "summary ")
		} else if strings.HasPrefix(line, "previous ") {
			prev := strings.TrimPrefix(line, "previous ")
			pts := strings.SplitN(prev, " ", 2)
			if len(pts) >= 1 {
				currentPreviousCommit = pts[0]
			}
			if len(pts) >= 2 {
				currentPreviousFilename = pts[1]
				if strings.HasPrefix(currentPreviousFilename, "\"") && strings.HasSuffix(currentPreviousFilename, "\"") {
					if unquoted, err := strconv.Unquote(currentPreviousFilename); err == nil {
						currentPreviousFilename = unquoted
					}
				}
			}
		} else if line == "boundary" || strings.HasPrefix(line, "boundary ") {
			currentIsBoundary = true
		} else if strings.HasPrefix(line, "\t") {
			// Content line
			content := strings.TrimPrefix(line, "\t")
			ln := currentLineNum
			if ln <= 0 {
				ln = fallbackLineNum
			}
			result = append(result, BlameLine{
				LineNumber:       ln,
				CommitHash:       currentHash,
				ShortHash:        currentShort,
				Author:           currentAuthor,
				AuthorEmail:      currentAuthorEmail,
				AuthorTimestamp:  currentAuthorTimestamp,
				AuthorTz:         currentAuthorTz,
				Date:             currentDate,
				Summary:          currentSummary,
				Content:          content,
				PreviousCommit:   currentPreviousCommit,
				PreviousFilename: currentPreviousFilename,
				IsBoundary:       currentIsBoundary,
			})
			fallbackLineNum++
			currentPreviousCommit = ""
			currentPreviousFilename = ""
			currentIsBoundary = false
			// Defensive protection: limit to 50,000 lines to prevent OOM
			if len(result) >= 50000 {
				break
			}
		}
	}

	if result == nil {
		result = []BlameLine{}
	}

	return result, nil
}

// largeBlobHistoryWorkers bounds how many `git log` processes run at once when
// resolving commit metadata for the largest historical blobs.
const largeBlobHistoryWorkers = 8

// ScanHistoryLargeBlobs scans Git object database for the largest historical blobs across all revisions
func (GitService) ScanHistoryLargeBlobs(repoPath string, limit int) ([]HistoryLargeBlob, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	cmd1 := exec.Command("git", "rev-list", "--objects", "--all")
	cmd1.Dir = repoPath

	cmd2 := exec.Command("git", "cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize) %(rest)")
	cmd2.Dir = repoPath

	pipe, err := cmd1.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd2.Stdin = pipe

	var outBuf bytes.Buffer
	cmd2.Stdout = &outBuf

	if err := cmd1.Start(); err != nil {
		return nil, err
	}
	if err := cmd2.Start(); err != nil {
		return nil, err
	}

	if err := cmd1.Wait(); err != nil {
		return nil, fmt.Errorf("枚举历史对象失败: %w", err)
	}
	if err := cmd2.Wait(); err != nil {
		return nil, fmt.Errorf("读取历史对象大小失败: %w", err)
	}

	type rawBlob struct {
		oid  string
		size int64
		path string
	}

	var blobs []rawBlob
	scanner := bufio.NewScanner(&outBuf)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 4)
		if len(parts) >= 3 && parts[1] == "blob" {
			size, err := strconv.ParseInt(parts[2], 10, 64)
			if err == nil {
				path := ""
				if len(parts) == 4 {
					path = strings.TrimSpace(parts[3])
				}
				blobs = append(blobs, rawBlob{
					oid:  parts[0],
					size: size,
					path: path,
				})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("解析历史对象列表失败: %w", err)
	}

	sort.Slice(blobs, func(i, j int) bool {
		return blobs[i].size > blobs[j].size
	})

	if len(blobs) > limit {
		blobs = blobs[:limit]
	}

	results := make([]HistoryLargeBlob, len(blobs))
	sem := make(chan struct{}, largeBlobHistoryWorkers)
	var wg sync.WaitGroup
	for i := range blobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			b := blobs[idx]
			commitHash := ""
			commitDate := ""
			author := ""
			subject := ""

			if b.path != "" {
				logOut, err := ExecGit(repoPath, "log", "-1", "--format=%h\x1f%ad\x1f%an\x1f%s", "--date=short", "--all", "--", b.path)
				if err == nil && logOut != "" {
					lParts := strings.Split(strings.TrimSpace(logOut), "\x1f")
					if len(lParts) >= 4 {
						commitHash = lParts[0]
						commitDate = lParts[1]
						author = lParts[2]
						subject = lParts[3]
					}
				}
			}

			results[idx] = HistoryLargeBlob{
				OID:        b.oid,
				Size:       b.size,
				SizeStr:    formatByteSize(b.size),
				Path:       b.path,
				CommitHash: commitHash,
				CommitDate: commitDate,
				Author:     author,
				Subject:    subject,
			}
		}(i)
	}
	wg.Wait()

	return results, nil
}

// GetFileRevisionTimeline retrieves the chronological commit history of a single file for historical time-travel
func (GitService) GetFileRevisionTimeline(repoPath string, filePath string) (*FileTimelineResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repoPath 不能为空")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}
	if filePath == "" {
		return nil, errors.New("filePath 不能为空")
	}

	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return nil, err
	}

	logOut, err := ExecGit(repoPath, "log", "--follow", "-n", "30", "--pretty=format:%H|%h|%an|%cd|%s", "--", cleanPath)
	if err != nil {
		// An empty repository has no commits yet, which is not a read failure.
		if _, headErr := ExecGit(repoPath, "rev-parse", "--verify", "HEAD"); headErr != nil {
			return &FileTimelineResult{
				FilePath:  cleanPath,
				Revisions: []FileRevisionItem{},
				Message:   "仓库尚无提交，暂无文件历史",
			}, nil
		}
		return nil, fmt.Errorf("读取文件历史失败: %w", err)
	}
	if strings.TrimSpace(logOut) == "" {
		return &FileTimelineResult{
			FilePath:  cleanPath,
			Revisions: []FileRevisionItem{},
			Message:   "未检索到该文件的提交历史记录",
		}, nil
	}

	revisions := []FileRevisionItem{}
	lines := strings.Split(logOut, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		parts := strings.Split(l, "|")
		if len(parts) >= 5 {
			h := parts[0]
			szHuman := "--"
			var szBytes int64 = 0
			szOut, szErr := ExecGit(repoPath, "cat-file", "-s", fmt.Sprintf("%s:%s", h, cleanPath))
			if szErr == nil && szOut != "" {
				if parsedSz, pErr := strconv.ParseInt(strings.TrimSpace(szOut), 10, 64); pErr == nil {
					szBytes = parsedSz
					szHuman = formatAssetSize(parsedSz)
				}
			}

			revisions = append(revisions, FileRevisionItem{
				CommitHash: h,
				ShortHash:  parts[1],
				AuthorName: parts[2],
				Date:       parts[3],
				Subject:    parts[4],
				SizeBytes:  szBytes,
				SizeStr:    szHuman,
			})
		}
	}

	return &FileTimelineResult{
		FilePath:  cleanPath,
		Revisions: revisions,
		Message:   fmt.Sprintf("成功提取 %d 条历史修订版本记录", len(revisions)),
	}, nil
}
