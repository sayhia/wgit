package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// =========================================================================
// Project Browser Module (项目浏览与多工程管理)
// =========================================================================

// PickFolderDialog pops up the native folder picker to select a directory for scanning
func (s GitService) PickFolderDialog() (string, error) {
	path, err := ChooseFolderDialog("选择项目根目录")
	if err != nil {
		return "", err
	}
	return path, nil
}

// maxProjectDetailWorkers caps how many projects are inspected concurrently.
// Every project costs a handful of git subprocesses, so a small pool already
// removes most of the serial latency without flooding the process table.
const maxProjectDetailWorkers = 8

// Source file detail payload caps. Blobs larger than these limits are reported
// by size only: loading a multi-GB revision blob into memory (or base64
// encoding it for the image preview) can exhaust the process heap.
const (
	maxSourceTextBytes  = 5 * 1024 * 1024
	maxSourceImageBytes = 30 * 1024 * 1024
)

// gitBlobSize returns the byte size of filePath at rev without materializing
// the blob, so callers can enforce a memory cap before running `git show`.
func gitBlobSize(repoPath string, rev string, filePath string) (int64, error) {
	out, err := ExecGit(repoPath, "cat-file", "-s", fmt.Sprintf("%s:%s", rev, filePath))
	if err != nil {
		return 0, fmt.Errorf("读取 %s@%s 的文件大小失败: %w", filePath, rev, err)
	}
	size, convErr := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if convErr != nil {
		return 0, fmt.Errorf("读取 %s@%s 的文件大小失败: %w", filePath, rev, convErr)
	}
	return size, nil
}

// countBlobLines counts lines in verbatim blob content without treating a
// single trailing newline as an extra line, so revision reads report the same
// line count as the trimmed reads they replace.
func countBlobLines(content string) int {
	return strings.Count(strings.TrimSuffix(content, "\n"), "\n") + 1
}

// GetProjectsDetails returns rich metadata for a list of project paths:
// branch, dirty status, ahead/behind, engine type, last commit.
//
// Every path is inspected independently, so the work is spread over a bounded
// worker pool. Each worker writes only its own result slot, which keeps the
// returned slice in exactly the input order despite the concurrency.
func (s GitService) GetProjectsDetails(paths []string, activePath string) ([]ProjectDetailItem, error) {
	activeClean := filepath.Clean(activePath)

	cleaned := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		cleaned = append(cleaned, filepath.Clean(p))
	}

	results := make([]ProjectDetailItem, len(cleaned))
	if len(cleaned) == 0 {
		return results, nil
	}

	workers := len(cleaned)
	if workers > maxProjectDetailWorkers {
		workers = maxProjectDetailWorkers
	}

	jobs := make(chan int, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for idx := range jobs {
				results[idx] = inspectProjectDetail(cleaned[idx], activeClean)
			}
		}()
	}
	for i := range cleaned {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results, nil
}

// inspectProjectDetail collects metadata for a single project path. It reads
// only its arguments and stateless helpers, so concurrent calls for different
// repositories are safe.
func inspectProjectDetail(cleanPath, activeClean string) ProjectDetailItem {
	name := filepath.Base(cleanPath)
	item := ProjectDetailItem{
		Name:       name,
		Path:       cleanPath,
		IsCurrent:  cleanPath == activeClean,
		EngineType: detectProjectEngine(cleanPath),
	}

	if !IsGitRepo(cleanPath) {
		item.IsValid = false
		return item
	}
	item.IsValid = true

	// 1. Current Branch
	branchOut, err := ExecGit(cleanPath, "symbolic-ref", "--short", "HEAD")
	if err == nil && strings.TrimSpace(branchOut) != "" {
		item.CurrentBranch = strings.TrimSpace(branchOut)
	} else {
		// Detached HEAD or commit hash
		headOut, _ := ExecGit(cleanPath, "rev-parse", "--short", "HEAD")
		item.CurrentBranch = strings.TrimSpace(headOut)
	}

	// 2. Dirty count
	statusOut, statusErr := ExecGit(cleanPath, "status", "--porcelain=v1")
	if statusErr != nil {
		item.StatusWarning = "读取工作区状态失败: " + statusErr.Error()
	} else {
		lines := strings.Split(strings.TrimSpace(statusOut), "\n")
		count := 0
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				count++
			}
		}
		item.DirtyCount = count
		item.IsDirty = count > 0
	}

	// 3. Ahead / Behind
	if item.CurrentBranch != "" && !strings.Contains(item.CurrentBranch, " ") {
		upstreamOut, err := ExecGit(cleanPath, "rev-parse", "--abbrev-ref", item.CurrentBranch+"@{upstream}")
		if err == nil && strings.TrimSpace(upstreamOut) != "" {
			countOut, err := ExecGit(cleanPath, "rev-list", "--left-right", "--count", item.CurrentBranch+"..."+strings.TrimSpace(upstreamOut))
			if err == nil {
				parts := strings.Fields(countOut)
				if len(parts) >= 2 {
					item.AheadCount, _ = strconv.Atoi(parts[0])
					item.BehindCount, _ = strconv.Atoi(parts[1])
				}
			}
		}
	}

	// 4. Last Commit Info
	logOut, err := ExecGit(cleanPath, "log", "-1", "--format=%h%x09%s%x09%cr")
	if err == nil && strings.TrimSpace(logOut) != "" {
		fields := strings.Split(strings.TrimSpace(logOut), "\t")
		if len(fields) >= 1 {
			item.LastCommitHash = fields[0]
		}
		if len(fields) >= 2 {
			item.LastCommitSubject = fields[1]
		}
		if len(fields) >= 3 {
			item.LastCommitDate = fields[2]
		}
	}

	return item
}

func detectProjectEngine(repoPath string) string {
	if fi, err := os.Stat(filepath.Join(repoPath, "Assets")); err == nil && fi.IsDir() {
		if fi2, err2 := os.Stat(filepath.Join(repoPath, "ProjectSettings")); err2 == nil && fi2.IsDir() {
			return "Unity"
		}
	}
	matches, _ := filepath.Glob(filepath.Join(repoPath, "*.uproject"))
	if len(matches) > 0 {
		return "Unreal"
	}
	if _, err := os.Stat(filepath.Join(repoPath, "project.godot")); err == nil {
		return "Godot"
	}
	if _, err := os.Stat(filepath.Join(repoPath, "project.json")); err == nil {
		return "Cocos"
	}
	if _, err := os.Stat(filepath.Join(repoPath, "go.mod")); err == nil {
		return "Go"
	}
	if _, err := os.Stat(filepath.Join(repoPath, "package.json")); err == nil {
		return "Node"
	}
	if _, err := os.Stat(filepath.Join(repoPath, "Cargo.toml")); err == nil {
		return "Rust"
	}
	return "Git"
}

// ScanLocalProjects discovers all Git repositories under searchRoot up to 3 levels deep
func (s GitService) ScanLocalProjects(searchRoot string) ([]string, error) {
	if strings.TrimSpace(searchRoot) == "" {
		searchRoot = "/Users/sunmking/workplace"
		if _, err := os.Stat(searchRoot); err != nil {
			home, _ := os.UserHomeDir()
			searchRoot = home
		}
	}
	searchRoot = filepath.Clean(searchRoot)

	found := make([]string, 0)
	maxDepth := strings.Count(searchRoot, string(os.PathSeparator)) + 3

	_ = filepath.Walk(searchRoot, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if !fi.IsDir() {
			return nil
		}
		base := fi.Name()
		if base == "node_modules" || base == ".cache" || base == "Library" || base == ".Trash" || base == "Pods" || base == "DerivedData" || base == "build" || base == "dist" {
			return filepath.SkipDir
		}

		if base == ".git" {
			repoDir := filepath.Dir(p)
			found = append(found, repoDir)
			return filepath.SkipDir
		}

		if strings.Count(p, string(os.PathSeparator)) >= maxDepth {
			return filepath.SkipDir
		}
		return nil
	})

	return found, nil
}

// GetProjectAllFiles returns all tracked files in the repo for full file tree exploration
func (s GitService) GetProjectAllFiles(repoPath string) ([]string, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	out, err := ExecGit(repoPath, "ls-files")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	files := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

func isBinaryExtension(ext string) bool {
	binaryExts := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".ico": true, ".webp": true, ".tga": true, ".psd": true,
		".exe": true, ".dll": true, ".dylib": true, ".so": true, ".a": true, ".lib": true, ".bin": true,
		".zip": true, ".tar": true, ".gz": true, ".7z": true, ".rar": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
		".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".aac": true, ".bnk": true, ".wem": true,
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
		".fbx": true, ".obj": true, ".blend": true, ".max": true, ".mb": true, ".ma": true,
		".uasset": true, ".umap": true, ".prefab": true, ".unity": true, ".asset": true,
	}
	return binaryExts[ext]
}

func isImageExtension(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".bmp", ".avif", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

func getImageMimeType(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".bmp":
		return "image/bmp"
	case ".avif":
		return "image/avif"
	case ".tif", ".tiff":
		return "image/tiff"
	default:
		return "application/octet-stream"
	}
}

func detectSourceLanguage(filePath string) string {
	base := strings.ToLower(filepath.Base(filePath))
	switch base {
	case "dockerfile", "dockerfile.dev", "dockerfile.prod":
		return "Dockerfile"
	case "makefile", "gnumakefile":
		return "Makefile"
	case "taskfile.yml", "taskfile.yaml":
		return "YAML"
	case "gemfile", "rakefile":
		return "Ruby"
	case ".gitignore", ".gitattributes", ".gitmodules":
		return "Git"
	case ".editorconfig":
		return "Config"
	}
	if strings.HasPrefix(base, ".env") {
		return "Config"
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go", ".mod", ".sum":
		return "Go"
	case ".ts", ".mts", ".cts":
		return "TypeScript"
	case ".tsx":
		return "TypeScript React"
	case ".js", ".mjs", ".cjs":
		return "JavaScript"
	case ".jsx":
		return "JavaScript React"
	case ".vue":
		return "Vue"
	case ".svelte":
		return "Svelte"
	case ".py", ".pyw":
		return "Python"
	case ".rs":
		return "Rust"
	case ".c", ".h":
		return "C"
	case ".cpp", ".hpp", ".cc", ".cxx", ".hxx", ".inl", ".ipp":
		return "C++"
	case ".cs":
		return "C#"
	case ".java":
		return "Java"
	case ".kt", ".kts":
		return "Kotlin"
	case ".swift":
		return "Swift"
	case ".php", ".phtml":
		return "PHP"
	case ".rb", ".rake":
		return "Ruby"
	case ".dart":
		return "Dart"
	case ".zig":
		return "Zig"
	case ".shader", ".hlsl", ".ush", ".usf", ".glsl", ".frag", ".vert", ".comp", ".geom", ".wgsl":
		return "Shader"
	case ".html", ".htm", ".xhtml":
		return "HTML"
	case ".css", ".scss", ".sass", ".less":
		return "CSS"
	case ".json", ".json5", ".jsonc":
		return "JSON"
	case ".md", ".markdown", ".mdx":
		return "Markdown"
	case ".yaml", ".yml":
		return "YAML"
	case ".toml":
		return "TOML"
	case ".xml", ".xaml", ".plist", ".resx":
		return "XML"
	case ".svg":
		return "SVG"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".bmp", ".avif":
		return "Image"
	case ".sh", ".bash", ".zsh", ".fish":
		return "Shell"
	case ".bat", ".cmd", ".ps1", ".psm1":
		return "PowerShell"
	case ".lua":
		return "Lua"
	case ".sql":
		return "SQL"
	case ".proto":
		return "Protobuf"
	case ".graphql", ".gql":
		return "GraphQL"
	case ".diff", ".patch":
		return "Diff"
	case ".ini", ".conf", ".cfg", ".properties":
		return "Config"
	default:
		return "Text"
	}
}

// GetProjectSourceFiles returns all workspace source files with status, size and binary flag.
// Large-repo notes: `git ls-files -z` keeps raw UTF-8 paths (spaces / Chinese / quotes),
// porcelain status is parsed from -z output, and file sizes are collected with a bounded
// worker pool because serial os.Stat calls dominate load time on monorepos.
func (s GitService) GetProjectSourceFiles(repoPath string) ([]ProjectSourceFileItem, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	paths, err := listRepoFiles(repoPath)
	if err != nil {
		return nil, err
	}
	statusMap := porcelainStatusMap(repoPath)

	items := make([]ProjectSourceFileItem, 0, len(paths))
	for _, p := range paths {
		status := "clean"
		if st, ok := statusMap[p]; ok {
			status = st
		}
		ext := strings.ToLower(filepath.Ext(p))
		items = append(items, ProjectSourceFileItem{
			Path:     p,
			Name:     filepath.Base(p),
			Size:     0,
			Status:   status,
			IsBinary: isBinaryExtension(ext),
		})
	}
	collectFileSizes(repoPath, items)
	return items, nil
}

// GetSourceFileDetail reads full content, syntax language, line count and latest commit info
func (s GitService) GetSourceFileDetail(repoPath string, filePath string, revision string) (*SourceFileDetail, error) {
	if repoPath == "" || filePath == "" {
		return nil, errors.New("repoPath and filePath cannot be empty")
	}

	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return nil, err
	}
	filePath = cleanPath
	name := filepath.Base(filePath)
	ext := strings.ToLower(filepath.Ext(filePath))
	isBin := isBinaryExtension(ext)
	language := detectSourceLanguage(filePath)

	var content string
	var size int64
	var lineCount int

	fullPath := filepath.Join(repoPath, filePath)

	var isImage bool
	var imageData string
	var imgWidth, imgHeight int

	if isImageExtension(ext) {
		isImage = true
		mimeType := getImageMimeType(ext)
		var imgBytes []byte

		if revision == "" || revision == "working" {
			if fi, err := os.Stat(fullPath); err == nil {
				size = fi.Size()
				if size <= maxSourceImageBytes {
					var readErr error
					imgBytes, readErr = os.ReadFile(fullPath)
					if readErr != nil && (revision == "" || revision == "working") {
						return nil, fmt.Errorf("读取图片内容失败: %w", readErr)
					}
				}
			}
		}
		if len(imgBytes) == 0 {
			targetRev := revision
			if targetRev == "" || targetRev == "working" {
				targetRev = "HEAD"
			}
			revSize, sizeErr := gitBlobSize(repoPath, targetRev, filePath)
			if sizeErr != nil {
				return nil, sizeErr
			}
			size = revSize
			if revSize <= maxSourceImageBytes {
				raw, showErr := ExecGitRaw(repoPath, "show", fmt.Sprintf("%s:%s", targetRev, filePath))
				if showErr != nil {
					return nil, fmt.Errorf("读取 %s@%s 的图片内容失败: %w", filePath, targetRev, showErr)
				}
				imgBytes = []byte(raw)
				size = int64(len(imgBytes))
			}
		}

		if len(imgBytes) > 0 {
			imageData = fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgBytes))
			if cfg, _, err := image.DecodeConfig(bytes.NewReader(imgBytes)); err == nil {
				imgWidth = cfg.Width
				imgHeight = cfg.Height
			}
			if ext == ".svg" {
				content = string(imgBytes)
				lineCount = strings.Count(content, "\n") + 1
				isBin = false
			} else {
				isBin = true
			}
		}
	} else if revision == "" || revision == "working" {
		if fi, err := os.Stat(fullPath); err == nil {
			size = fi.Size()
			if isBin || size > maxSourceTextBytes {
				isBin = true
				content = ""
			} else {
				data, readErr := os.ReadFile(fullPath)
				if readErr != nil {
					return nil, fmt.Errorf("读取文件内容失败: %w", readErr)
				}
				sample := data
				if len(sample) > 8000 {
					sample = sample[:8000]
				}
				for _, b := range sample {
					if b == 0 {
						isBin = true
						break
					}
				}
				if !isBin {
					content = string(data)
					lineCount = strings.Count(content, "\n") + 1
				}
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("读取文件状态失败: %w", err)
		} else {
			headSize, sizeErr := gitBlobSize(repoPath, "HEAD", filePath)
			if sizeErr != nil {
				return nil, sizeErr
			}
			size = headSize
			if headSize > maxSourceTextBytes {
				// The worktree copy is gone and the committed blob is too large to
				// load: report its real size and flag it as binary instead of OOM.
				isBin = true
				content = ""
			} else {
				raw, showErr := ExecGitRaw(repoPath, "show", fmt.Sprintf("HEAD:%s", filePath))
				if showErr != nil {
					return nil, fmt.Errorf("读取文件内容失败: %w", showErr)
				}
				size = int64(len(raw))
				content = raw
				lineCount = countBlobLines(content)
			}
		}
	} else {
		revSize, sizeErr := gitBlobSize(repoPath, revision, filePath)
		if sizeErr != nil {
			return nil, sizeErr
		}
		size = revSize
		if revSize > maxSourceTextBytes {
			// Oversized revision blob: expose the size only, never the content.
			isBin = true
			content = ""
		} else {
			raw, showErr := ExecGitRaw(repoPath, "show", fmt.Sprintf("%s:%s", revision, filePath))
			if showErr != nil {
				return nil, fmt.Errorf("读取 %s@%s 的文件内容失败: %w", filePath, revision, showErr)
			}
			size = int64(len(raw))
			content = raw
			lineCount = countBlobLines(content)
		}
	}

	var lastHash, lastAuthor, lastDate, lastSubject string
	revParam := revision
	if revParam == "" || revParam == "working" {
		revParam = "HEAD"
	}
	logOut, _ := ExecGit(repoPath, "log", "-1", "--format=%H%x1f%an%x1f%cr%x1f%s", revParam, "--", filePath)
	if logOut != "" {
		parts := strings.Split(strings.TrimSpace(logOut), "\x1f")
		if len(parts) >= 4 {
			lastHash = parts[0]
			lastAuthor = parts[1]
			lastDate = parts[2]
			lastSubject = parts[3]
		}
	}

	sizeFmt := formatByteSize(size)

	return &SourceFileDetail{
		Path:           filePath,
		Name:           name,
		Size:           size,
		SizeFormatted:  sizeFmt,
		LineCount:      lineCount,
		Language:       language,
		Content:        content,
		IsBinary:       isBin,
		IsImage:        isImage,
		ImageData:      imageData,
		ImageWidth:     imgWidth,
		ImageHeight:    imgHeight,
		LastCommitHash: lastHash,
		LastAuthor:     lastAuthor,
		LastDate:       lastDate,
		LastSubject:    lastSubject,
	}, nil
}

// GetSourceFileDiff returns unified diff of a specific file against HEAD or a revision
func (s GitService) GetSourceFileDiff(repoPath string, filePath string, revision string) (string, error) {
	if repoPath == "" || filePath == "" {
		return "", errors.New("repoPath and filePath cannot be empty")
	}
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return "", err
	}
	// `git diff` and `git show --patch` buffer their whole output before it can
	// be capped, so an oversized blob must be rejected up front: a multi-GB
	// file at HEAD or at a revision would exhaust the heap long before
	// truncateDiffPayload ever runs.
	tooLargeErr := func() error {
		return fmt.Errorf("文件超过 %d MB，已跳过差异加载以避免内存占用", maxSourceTextBytes/(1024*1024))
	}
	exceedsCap := func(size int64, known bool) bool {
		return known && size > maxSourceTextBytes
	}
	var out string
	if revision == "" || revision == "working" {
		fullPath := filepath.Join(repoPath, cleanPath)
		var workSize int64
		if fi, statErr := os.Stat(fullPath); statErr == nil {
			workSize = fi.Size()
		}
		// A missing HEAD blob (unborn HEAD, brand-new or untracked file) is an
		// unknown size rather than a failure: those cases still rely on the
		// untracked diff fallback below.
		headSize, headErr := gitBlobSize(repoPath, "HEAD", cleanPath)
		if exceedsCap(workSize, true) || exceedsCap(headSize, headErr == nil) {
			return "", tooLargeErr()
		}
		out, err = ExecGit(repoPath, "diff", "HEAD", "--", cleanPath)
		if err != nil || strings.TrimSpace(out) == "" {
			var fallbackErr error
			out, fallbackErr = ExecGit(repoPath, "diff", "--", cleanPath)
			if fallbackErr != nil {
				return "", fmt.Errorf("读取文件差异失败: %w", fallbackErr)
			}
		}
	} else {
		// Both sides of the patch matter, so check the revision blob and its
		// parent. Parent lookup is best-effort: root commits have none, and a
		// size query failure is treated as unknown instead of fatal.
		revSize, revErr := gitBlobSize(repoPath, revision, cleanPath)
		parentSize, parentErr := gitBlobSize(repoPath, revision+"^", cleanPath)
		if exceedsCap(revSize, revErr == nil) || exceedsCap(parentSize, parentErr == nil) {
			return "", tooLargeErr()
		}
		// `git diff <root>^!` exits 0 with empty output, so use `git show` for
		// every revision; it produces the commit's patch for this file.
		out, err = ExecGit(repoPath, "show", "--format=", "--patch", revision, "--", cleanPath)
		if err != nil {
			var fallbackErr error
			out, fallbackErr = ExecGit(repoPath, "show", fmt.Sprintf("%s:%s", revision, cleanPath))
			if fallbackErr != nil {
				return "", fmt.Errorf("读取 %s@%s 的差异失败: %w", filePath, revision, err)
			}
		}
	}
	capped, _ := truncateDiffPayload(out)
	return capped, nil
}
