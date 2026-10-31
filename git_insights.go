package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ==========================================
// Studio Collaboration Insights (Round 101)
// ==========================================

type ActivityDay struct {
	Date      string `json:"date"` // YYYY-MM-DD
	Commits   int    `json:"commits"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Level     int    `json:"level"` // 0 - 4 heat level
	Weekday   int    `json:"weekday"`
}

type ContributorActivity struct {
	Name       string  `json:"name"`
	Email      string  `json:"email"`
	Commits    int     `json:"commits"`
	Additions  int     `json:"additions"`
	Deletions  int     `json:"deletions"`
	LastActive string  `json:"lastActive"`
	Share      float64 `json:"share"` // percent of total commits
	IsMe       bool    `json:"isMe"`
}

type FileChurnHotspot struct {
	Path      string   `json:"path"`
	Changes   int      `json:"changes"`
	Additions int      `json:"additions"`
	Deletions int      `json:"deletions"`
	Authors   []string `json:"authors"`
	Category  string   `json:"category"`
	RiskScore int      `json:"riskScore"` // 0 - 100
}

type TeamActivityResult struct {
	Days             []ActivityDay         `json:"days"`
	Contributors     []ContributorActivity `json:"contributors"`
	Hotspots         []FileChurnHotspot    `json:"hotspots"`
	TotalCommits     int                   `json:"totalCommits"`
	ActiveDays       int                   `json:"activeDays"`
	CurrentStreak    int                   `json:"currentStreak"`
	LongestStreak    int                   `json:"longestStreak"`
	PeakHour         string                `json:"peakHour"`
	PeakWeekday      string                `json:"peakWeekday"`
	AvgCommitsPerDay float64               `json:"avgCommitsPerDay"`
	MergeCommits     int                   `json:"mergeCommits"`
	HourBuckets      []int                 `json:"hourBuckets"`
	WeekdayCommits   []int                 `json:"weekdayCommits"`
	RangeStart       string                `json:"rangeStart"`
	RangeEnd         string                `json:"rangeEnd"`
	BusiestDay       string                `json:"busiestDay"`
	BusiestDayCount  int                   `json:"busiestDayCount"`
	Summary          string                `json:"summary"`
}

var weekdayLabels = []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

// GetTeamActivity aggregates commit heatmap, contributor ranking and churn hotspots.
func (GitService) GetTeamActivity(repoPath string, days int) (*TeamActivityResult, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}
	if days <= 0 {
		days = 182
	}
	if days < 7 {
		days = 7
	}
	if days > 730 {
		days = 730
	}

	now := time.Now()
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(days - 1))
	endDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	res := &TeamActivityResult{
		Days:           make([]ActivityDay, 0, days),
		Contributors:   []ContributorActivity{},
		Hotspots:       []FileChurnHotspot{},
		HourBuckets:    make([]int, 24),
		WeekdayCommits: make([]int, 7),
		RangeStart:     startDay.Format("2006-01-02"),
		RangeEnd:       endDay.Format("2006-01-02"),
	}

	format := "%x01%H%x1f%aI%x1f%an%x1f%ae%x1f%P%x1f%s"
	fetchLog := func(useSince bool) (string, error) {
		args := []string{"log", "--no-color", "--no-renames", "--pretty=format:" + format, "--numstat"}
		if useSince {
			args = append(args, "--since="+res.RangeStart, "--max-count=5000")
		} else {
			args = append(args, "--max-count=3000")
		}
		return ExecGit(repoPath, args...)
	}

	raw, err := fetchLog(true)
	emptyRepo := false
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits yet") || strings.Contains(raw, "does not have any commits yet") {
			emptyRepo = true
		} else {
			return nil, err
		}
	}

	byDay := make(map[string]*ActivityDay, days)
	byAuthor := make(map[string]*ContributorActivity)
	hotspots := make(map[string]*FileChurnHotspot)
	hotspotAuthors := make(map[string]map[string]bool)

	// ingest parses a raw `git log --numstat` stream and folds it into the aggregates.
	// Commits older than the window are ignored, so this is safe for the open-ended fallback.
	ingest := func(rawLog string) {
		var current *ContributorActivity
		currentDay := ""

		for _, line := range strings.Split(rawLog, "\n") {
			line = strings.TrimSuffix(line, "\r")
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "\x01") {
				fields := strings.Split(strings.TrimPrefix(line, "\x01"), "\x1f")
				if len(fields) < 6 {
					current = nil
					continue
				}
				ts, perr := time.Parse(time.RFC3339, strings.TrimSpace(fields[1]))
				if perr != nil {
					if d, derr := time.Parse("2006-01-02", strings.TrimSpace(fields[1])); derr == nil {
						ts = d
					} else {
						current = nil
						continue
					}
				}
				dayKey := ts.Format("2006-01-02")
				// A non-monotonic commit date can surface stale commits; keep only the window.
				if dayKey < res.RangeStart || dayKey > res.RangeEnd {
					current = nil
					continue
				}
				name := strings.TrimSpace(fields[2])
				email := strings.ToLower(strings.TrimSpace(fields[3]))
				if name == "" {
					name = "未知作者"
				}

				day := byDay[dayKey]
				if day == nil {
					day = &ActivityDay{Date: dayKey, Weekday: int(ts.Weekday())}
					byDay[dayKey] = day
				}
				day.Commits++

				key := email
				if key == "" {
					key = strings.ToLower(name)
				}
				contrib := byAuthor[key]
				if contrib == nil {
					contrib = &ContributorActivity{Name: name, Email: strings.TrimSpace(fields[3])}
					byAuthor[key] = contrib
				}
				contrib.Commits++
				if len(fields[4]) > 0 && strings.Contains(fields[4], " ") {
					res.MergeCommits++
				}
				if dayKey > contrib.LastActive {
					contrib.LastActive = dayKey
				}

				res.TotalCommits++
				res.HourBuckets[ts.Hour()]++
				res.WeekdayCommits[int(ts.Weekday())]++

				current = contrib
				currentDay = dayKey
				continue
			}

			if current == nil {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) < 3 {
				continue
			}
			add, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			del, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			path := strings.TrimSpace(parts[2])
			if path == "" {
				continue
			}
			current.Additions += add
			current.Deletions += del

			if day := byDay[currentDay]; day != nil {
				day.Additions += add
				day.Deletions += del
			}

			hot := hotspots[path]
			if hot == nil {
				hot = &FileChurnHotspot{
					Path:     path,
					Category: classifyAssetCategory(strings.ToLower(filepath.Ext(path))),
					Authors:  []string{},
				}
				hotspots[path] = hot
				hotspotAuthors[path] = map[string]bool{}
			}
			hot.Changes++
			hot.Additions += add
			hot.Deletions += del
			if !hotspotAuthors[path][current.Name] {
				hotspotAuthors[path][current.Name] = true
				hot.Authors = append(hot.Authors, current.Name)
			}
		}
	}

	if !emptyRepo {
		ingest(raw)
		// `git log --since` prunes the whole traversal when the branch tip carries an old
		// commit date (rebased/cherry-picked history). Retry open-ended and filter by day.
		if res.TotalCommits == 0 {
			if fallback, ferr := fetchLog(false); ferr == nil {
				ingest(fallback)
			}
		}
	}

	// Build a continuous day axis so the heatmap has no gaps.
	maxPerDay := 0
	for i := 0; i < days; i++ {
		date := startDay.AddDate(0, 0, i)
		key := date.Format("2006-01-02")
		day := ActivityDay{Date: key, Weekday: int(date.Weekday())}
		if existing := byDay[key]; existing != nil {
			day.Commits = existing.Commits
			day.Additions = existing.Additions
			day.Deletions = existing.Deletions
		}
		if day.Commits > 0 {
			res.ActiveDays++
		}
		if day.Commits > maxPerDay {
			maxPerDay = day.Commits
		}
		if day.Commits > res.BusiestDayCount {
			res.BusiestDayCount = day.Commits
			res.BusiestDay = key
		}
		res.Days = append(res.Days, day)
	}

	// Heat levels relative to the busiest day of the window.
	for i := range res.Days {
		d := &res.Days[i]
		if d.Commits == 0 {
			d.Level = 0
			continue
		}
		if maxPerDay <= 1 {
			d.Level = 4
			continue
		}
		ratio := float64(d.Commits) / float64(maxPerDay)
		switch {
		case ratio <= 0.25:
			d.Level = 1
		case ratio <= 0.5:
			d.Level = 2
		case ratio <= 0.75:
			d.Level = 3
		default:
			d.Level = 4
		}
	}

	// Streaks.
	longest := 0
	run := 0
	for _, d := range res.Days {
		if d.Commits > 0 {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	res.LongestStreak = longest
	idx := len(res.Days) - 1
	if idx >= 0 && res.Days[idx].Commits == 0 {
		idx--
	}
	for ; idx >= 0 && res.Days[idx].Commits > 0; idx-- {
		res.CurrentStreak++
	}

	// Contributors.
	// Kept swallowed (batch 6): `git config user.name/user.email` exits
	// non-zero when the identity is simply unconfigured — the normal exit-1
	// case, not a failure. The values only drive the IsMe badge, and an
	// unread identity degrades fail-safe: nobody gets flagged as "me",
	// never a false positive.
	meName, _ := ExecGit(repoPath, "config", "user.name")
	meEmail, _ := ExecGit(repoPath, "config", "user.email")
	meName = strings.ToLower(strings.TrimSpace(meName))
	meEmail = strings.ToLower(strings.TrimSpace(meEmail))

	for _, c := range byAuthor {
		if res.TotalCommits > 0 {
			c.Share = float64(c.Commits) / float64(res.TotalCommits) * 100
		}
		if meEmail != "" && strings.EqualFold(strings.TrimSpace(c.Email), meEmail) {
			c.IsMe = true
		} else if meEmail == "" && meName != "" && strings.ToLower(c.Name) == meName {
			c.IsMe = true
		}
		res.Contributors = append(res.Contributors, *c)
	}
	sort.Slice(res.Contributors, func(i, j int) bool {
		if res.Contributors[i].Commits == res.Contributors[j].Commits {
			return res.Contributors[i].Name < res.Contributors[j].Name
		}
		return res.Contributors[i].Commits > res.Contributors[j].Commits
	})
	if len(res.Contributors) > 20 {
		res.Contributors = res.Contributors[:20]
	}

	// Hotspots ranked by churn pressure.
	for _, h := range hotspots {
		freqScore := h.Changes * 6
		if freqScore > 60 {
			freqScore = 60
		}
		churnScore := (h.Additions + h.Deletions) / 100
		if churnScore > 40 {
			churnScore = 40
		}
		h.RiskScore = freqScore + churnScore
		if h.RiskScore > 100 {
			h.RiskScore = 100
		}
		sort.Strings(h.Authors)
		if len(h.Authors) > 5 {
			h.Authors = h.Authors[:5]
		}
		res.Hotspots = append(res.Hotspots, *h)
	}
	sort.Slice(res.Hotspots, func(i, j int) bool {
		if res.Hotspots[i].RiskScore == res.Hotspots[j].RiskScore {
			return res.Hotspots[i].Changes > res.Hotspots[j].Changes
		}
		return res.Hotspots[i].RiskScore > res.Hotspots[j].RiskScore
	})
	if len(res.Hotspots) > 12 {
		res.Hotspots = res.Hotspots[:12]
	}

	// Rhythm peaks.
	peakHour := 0
	for h, c := range res.HourBuckets {
		if c > res.HourBuckets[peakHour] {
			peakHour = h
		}
	}
	if res.TotalCommits == 0 {
		res.PeakHour = "--"
		res.PeakWeekday = "--"
	} else {
		res.PeakHour = fmt.Sprintf("%02d:00 - %02d:00", peakHour, (peakHour+1)%24)
		peakWeekday := 0
		for d, c := range res.WeekdayCommits {
			if c > res.WeekdayCommits[peakWeekday] {
				peakWeekday = d
			}
		}
		res.PeakWeekday = weekdayLabels[peakWeekday]
	}

	if res.TotalCommits > 0 {
		res.AvgCommitsPerDay = float64(res.TotalCommits) / float64(days)
		res.Summary = fmt.Sprintf("近 %d 天共 %d 次提交，%d 天有人提交，人均节奏 %.2f 次/天；最长连续 %d 天。",
			days, res.TotalCommits, res.ActiveDays, res.AvgCommitsPerDay, res.LongestStreak)
	} else {
		res.Summary = fmt.Sprintf("近 %d 天没有检测到任何提交记录。", days)
	}

	return res, nil
}

// ==========================================
// Historical Content Archaeology (Pickaxe)
// ==========================================

type PickaxeHit struct {
	Hash      string   `json:"hash"`
	ShortHash string   `json:"shortHash"`
	Author    string   `json:"author"`
	Date      string   `json:"date"`
	Subject   string   `json:"subject"`
	Added     int      `json:"added"`
	Removed   int      `json:"removed"`
	Files     []string `json:"files"`
	Snippet   string   `json:"snippet"`
	Change    string   `json:"change"` // 新增 / 移除 / 改写 / 触碰
}

type PickaxeResult struct {
	Query        string       `json:"query"`
	Mode         string       `json:"mode"`
	TotalHits    int          `json:"totalHits"`
	AnalyzedHits int          `json:"analyzedHits"`
	AddedLines   int          `json:"addedLines"`
	RemovedLines int          `json:"removedLines"`
	FileCount    int          `json:"fileCount"`
	FirstSeen    string       `json:"firstSeen"`
	LastSeen     string       `json:"lastSeen"`
	Truncated    bool         `json:"truncated"`
	Hits         []PickaxeHit `json:"hits"`
	Message      string       `json:"message"`
}

// pickaxeShowWorkers bounds how many `git show` processes run at once when
// expanding pickaxe hits (same bounded-pool pattern as commitDetailsWorkers).
const pickaxeShowWorkers = 8

// PickaxeSearch finds commits where a string appeared, disappeared or was rewritten.
// mode "content" maps to git log -S, mode "regex" maps to git log -G.
func (GitService) PickaxeSearch(repoPath string, query string, mode string, author string, pathFilter string, limit int, ignoreCase bool) (*PickaxeResult, error) {
	query = strings.TrimSpace(query)
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}
	if query == "" {
		return nil, errors.New("搜索关键字不能为空")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 60 {
		limit = 60
	}

	useRegex := strings.EqualFold(mode, "regex") || strings.EqualFold(mode, "G")
	resultMode := "content"
	if useRegex {
		resultMode = "regex"
	}

	res := &PickaxeResult{
		Query: query,
		Mode:  resultMode,
		Hits:  []PickaxeHit{},
	}

	var matcher *regexp.Regexp
	if useRegex {
		compiled, err := regexp.Compile(query)
		if err != nil {
			return nil, fmt.Errorf("正则表达式无效: %v", err)
		}
		matcher = compiled
	}

	matches := func(s string) bool {
		if matcher != nil {
			return matcher.MatchString(s)
		}
		if ignoreCase {
			return strings.Contains(strings.ToLower(s), strings.ToLower(query))
		}
		return strings.Contains(s, query)
	}

	format := "%x01%H%x1f%h%x1f%an%x1f%ad%x1f%s"
	args := []string{"log", "--no-color", "--max-count=200", "--date=short", "--pretty=format:" + format}
	if ignoreCase && !useRegex {
		args = append(args, "-i")
	}
	if useRegex {
		args = append(args, "-G"+query)
	} else {
		args = append(args, "-S"+query)
	}
	if strings.TrimSpace(author) != "" {
		args = append(args, "--author="+strings.TrimSpace(author))
	}
	if strings.TrimSpace(pathFilter) != "" {
		args = append(args, "--", strings.TrimSpace(pathFilter))
	}

	raw, err := ExecGit(repoPath, args...)
	if err != nil && strings.TrimSpace(raw) == "" {
		return nil, err
	}

	type hitMeta struct {
		hash, short, authorName, date, subject string
	}
	var metas []hitMeta
	for _, line := range strings.Split(raw, "\n") {
		if !strings.HasPrefix(line, "\x01") {
			continue
		}
		fields := strings.Split(strings.TrimPrefix(line, "\x01"), "\x1f")
		if len(fields) < 5 {
			continue
		}
		metas = append(metas, hitMeta{
			hash:       fields[0],
			short:      fields[1],
			authorName: fields[2],
			date:       fields[3],
			subject:    fields[4],
		})
	}

	res.TotalHits = len(metas)
	res.Truncated = len(metas) >= 200

	if len(metas) == 0 {
		res.Message = fmt.Sprintf("没有找到包含「%s」的历史提交，试试切换到正则模式或放宽过滤条件。", query)
		return res, nil
	}

	analyzed := len(metas)
	if analyzed > limit {
		analyzed = limit
	}

	// Expanding each hit spawns one `git show` per commit; run them on a
	// bounded pool (same pattern as commitDetailsWorkers) instead of serially.
	// Each worker owns a distinct slot, so input order is preserved, and the
	// shared fileSet is only mutated afterwards by the single parsing loop.
	diffs := make([]string, analyzed)
	sem := make(chan struct{}, pickaxeShowWorkers)
	var wg sync.WaitGroup
	for i := 0; i < analyzed; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, hash string) {
			defer wg.Done()
			defer func() { <-sem }()
			raw, err := ExecGit(repoPath, "show", "--no-color", "--no-renames", "--format=", "--unified=0", hash)
			if err == nil {
				diffs[idx] = raw
			}
			// 与原串行语义一致：单个命中的 show 失败按空差异处理，该命中
			// 退化为「触碰」，不影响其它命中，也不升级为整体错误。
		}(i, metas[i].hash)
	}
	wg.Wait()

	fileSet := make(map[string]bool)
	for i := 0; i < analyzed; i++ {
		meta := metas[i]
		hit := PickaxeHit{
			Hash:      meta.hash,
			ShortHash: meta.short,
			Author:    meta.authorName,
			Date:      meta.date,
			Subject:   meta.subject,
			Files:     []string{},
		}

		if diff := diffs[i]; diff != "" {
			currentFile := ""
			seenFiles := make(map[string]bool)
			for _, dl := range strings.Split(diff, "\n") {
				if strings.HasPrefix(dl, "+++ b/") {
					currentFile = strings.TrimPrefix(dl, "+++ b/")
					if !seenFiles[currentFile] {
						seenFiles[currentFile] = true
						hit.Files = append(hit.Files, currentFile)
						fileSet[currentFile] = true
					}
					continue
				}
				if strings.HasPrefix(dl, "--- ") || strings.HasPrefix(dl, "@@") || strings.HasPrefix(dl, "diff ") || strings.HasPrefix(dl, "index ") {
					continue
				}
				if strings.HasPrefix(dl, "+") && matches(dl[1:]) {
					hit.Added++
					if hit.Snippet == "" {
						hit.Snippet = truncateRunes(strings.TrimSpace(dl[1:]), 160)
					}
				} else if strings.HasPrefix(dl, "-") && matches(dl[1:]) {
					hit.Removed++
					if hit.Snippet == "" {
						hit.Snippet = truncateRunes(strings.TrimSpace(dl[1:]), 160)
					}
				}
			}
		}

		switch {
		case hit.Added > 0 && hit.Removed > 0:
			hit.Change = "改写"
		case hit.Added > 0:
			hit.Change = "新增"
		case hit.Removed > 0:
			hit.Change = "移除"
		default:
			hit.Change = "触碰"
		}

		res.AddedLines += hit.Added
		res.RemovedLines += hit.Removed
		res.Hits = append(res.Hits, hit)
	}

	res.AnalyzedHits = analyzed
	res.FileCount = len(fileSet)
	res.LastSeen = metas[0].date
	res.FirstSeen = metas[len(metas)-1].date
	res.Message = fmt.Sprintf("命中 %d 个历史提交，已展开最近 %d 个：新增 %d 行 / 移除 %d 行，涉及 %d 个文件。",
		res.TotalHits, analyzed, res.AddedLines, res.RemovedLines, res.FileCount)
	if res.Truncated {
		res.Message += " 结果已达 200 条上限，建议补充过滤条件。"
	}

	return res, nil
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// ==========================================
// Commit Message Convention Linter
// ==========================================

type CommitLintIssue struct {
	Severity string `json:"severity"` // error / warning / hint
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

type CommitLintResult struct {
	Header     string            `json:"header"`
	Type       string            `json:"type"`
	Scope      string            `json:"scope"`
	SubjectLen int               `json:"subjectLen"`
	Score      int               `json:"score"`
	Grade      string            `json:"grade"`
	Compliant  bool              `json:"compliant"`
	Issues     []CommitLintIssue `json:"issues"`
	AutoFixed  string            `json:"autoFixed"`
	HasFix     bool              `json:"hasFix"`
	Summary    string            `json:"summary"`
}

var conventionalTypes = map[string]bool{
	"feat": true, "fix": true, "docs": true, "style": true, "refactor": true,
	"perf": true, "test": true, "build": true, "ci": true, "chore": true,
	"revert": true, "art": true, "level": true, "asset": true, "ui": true,
	"audio": true, "design": true,
}

var conventionalHeaderRe = regexp.MustCompile(`^([A-Za-z]+)(\(([^()]+)\))?(!)?:[ \t]*(.*)$`)
var colonSpaceRe = regexp.MustCompile(`^[A-Za-z]+(\([^()]+\))?!?:[^\s]`)
var typePrefixFixRe = regexp.MustCompile(`^([A-Za-z]+(?:\([^()]+\))?!?):[ \t]*`)

// LintCommitMessage scores a commit message against studio-friendly conventions.
func (GitService) LintCommitMessage(message string) *CommitLintResult {
	normalized := strings.ReplaceAll(message, "\r\n", "\n")
	trimmed := strings.TrimSpace(normalized)

	res := &CommitLintResult{
		Issues:    []CommitLintIssue{},
		Score:     100,
		Grade:     "A",
		AutoFixed: trimmed,
	}

	add := func(severity, rule, msg string) {
		res.Issues = append(res.Issues, CommitLintIssue{Severity: severity, Rule: rule, Message: msg})
	}

	if trimmed == "" {
		res.Score = 0
		res.Grade = "D"
		res.Summary = "提交信息为空，无法通过规范检查"
		res.Issues = append(res.Issues, CommitLintIssue{Severity: "error", Rule: "empty", Message: "提交信息不能为空"})
		res.HasFix = false
		return res
	}

	lines := strings.Split(trimmed, "\n")
	header := strings.TrimSpace(lines[0])
	res.Header = header
	res.SubjectLen = len([]rune(header))

	desc := header
	if m := conventionalHeaderRe.FindStringSubmatch(header); m != nil {
		res.Type = strings.ToLower(m[1])
		res.Scope = strings.TrimSpace(m[3])
		desc = strings.TrimSpace(m[5])
		if !conventionalTypes[res.Type] {
			add("warning", "type", fmt.Sprintf("类型「%s」不在推荐集合内（feat/fix/docs/refactor/perf/test/chore/art/level…）", res.Type))
		}
		if desc == "" {
			add("error", "empty-desc", "类型前缀后缺少具体描述")
		}
	} else {
		add("warning", "conventional", "缺少 Conventional Commits 类型前缀（如 feat: / fix: / art:）")
		if strings.Contains(header, ":") && !strings.Contains(header, ": ") {
			add("warning", "colon-space", "冒号后缺少空格")
		}
	}

	// Space after the type colon.
	if colonSpaceRe.MatchString(header) {
		add("warning", "colon-space", "类型冒号后应保留一个空格，例如 feat: 新增副本加载")
	}

	if res.SubjectLen > 100 {
		add("error", "subject-length", fmt.Sprintf("标题 %d 字，超过 100 字上限", res.SubjectLen))
	} else if res.SubjectLen > 72 {
		add("warning", "subject-length", fmt.Sprintf("标题 %d 字，建议控制在 72 字以内", res.SubjectLen))
	}

	if strings.HasSuffix(header, "。") || strings.HasSuffix(header, ".") {
		add("warning", "trailing-period", "标题末尾不需要句号")
	}

	lowerHeader := strings.ToLower(header)
	for _, bad := range []string{"wip", "tmp", "temp commit", "fixup!", "squash!", "临时提交", "暂存一下"} {
		if strings.Contains(lowerHeader, bad) {
			add("warning", "wip", fmt.Sprintf("检测到临时标记「%s」，合并前请整理提交历史", bad))
			break
		}
	}

	vague := map[string]bool{
		"update": true, "updates": true, "change": true, "changes": true, "fix": true,
		"bugfix": true, "修改": true, "更新": true, "提交": true, "修复": true, "改动": true, "test": true,
	}
	if vague[strings.ToLower(strings.TrimSpace(desc))] {
		add("warning", "vague", "描述过于笼统，建议说明「改了什么、为什么改」")
	}

	if strings.Contains(header, "  ") {
		add("hint", "double-space", "标题中存在连续空格")
	}

	bodyStart := -1
	if len(lines) > 1 {
		if strings.TrimSpace(lines[1]) != "" {
			add("warning", "blank-line", "标题与正文之间需要空一行")
		} else {
			bodyStart = 2
		}
		for i, l := range lines {
			if strings.TrimRight(l, " \t") != l {
				add("hint", "trailing-space", fmt.Sprintf("第 %d 行存在行尾空格", i+1))
				break
			}
		}
		if bodyStart >= 0 {
			for i := bodyStart; i < len(lines); i++ {
				if len([]rune(lines[i])) > 100 {
					add("warning", "body-width", fmt.Sprintf("正文第 %d 行超过 100 字，建议手动换行", i+1))
					break
				}
			}
		}
	}

	// Score.
	for _, issue := range res.Issues {
		switch issue.Severity {
		case "error":
			res.Score -= 25
		case "warning":
			res.Score -= 10
		default:
			res.Score -= 3
		}
	}
	if res.Score < 0 {
		res.Score = 0
	}
	switch {
	case res.Score >= 90:
		res.Grade = "A"
	case res.Score >= 75:
		res.Grade = "B"
	case res.Score >= 60:
		res.Grade = "C"
	default:
		res.Grade = "D"
	}
	// Build an auto-fix candidate from the mechanical issues only.
	fixedHeader := strings.Join(strings.Fields(header), " ")
	fixedHeader = typePrefixFixRe.ReplaceAllString(fixedHeader, "$1: ")
	fixedHeader = strings.TrimRight(fixedHeader, "。.")
	if fixedHeader != header {
		fixedLines := append([]string{fixedHeader}, lines[1:]...)
		res.AutoFixed = strings.Join(fixedLines, "\n")
		res.HasFix = true
	}
	if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		res.AutoFixed = fixedHeader + "\n\n" + strings.TrimSpace(strings.Join(lines[1:], "\n"))
		res.HasFix = true
	}
	res.AutoFixed = strings.TrimSpace(res.AutoFixed)

	errors, warnings := 0, 0
	for _, issue := range res.Issues {
		if issue.Severity == "error" {
			errors++
		} else if issue.Severity == "warning" {
			warnings++
		}
	}
	// A message without a Conventional Commits type is never fully compliant,
	// even when the remaining wording is fine.
	res.Compliant = errors == 0 && res.Type != "" && res.Score >= 80
	switch {
	case len(res.Issues) == 0:
		res.Summary = "提交信息完全符合团队规范"
	case errors > 0:
		res.Summary = fmt.Sprintf("存在 %d 处错误、%d 处提醒，建议修正后再提交", errors, warnings)
	case warnings > 0:
		res.Summary = fmt.Sprintf("规范度 %d 分，有 %d 处可以优化的细节", res.Score, warnings)
	default:
		res.Summary = "格式良好，仅有少量细节提示"
	}

	return res
}
