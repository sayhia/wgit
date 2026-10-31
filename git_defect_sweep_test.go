package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// This file enforces the "unfinished traces" audit (defect/boundary sweep) as a
// permanent regression guard:
//
//  1. panic( is a forbidden crash surface in library/service code; only the
//     main.go startup path may terminate the process.
//  2. TODO/FIXME/XXX/HACK markers must not accumulate in non-test Go sources;
//     unfinished work belongs in the task tracker, not in code comments.
//  3. The same TODO/FIXME rule applies to frontend/src (.ts/.vue).
//
// A violation is reported as "file:line: match" so it can be triaged directly.

var (
	defectSweepPanicRe  = regexp.MustCompile(`\bpanic\(`)
	defectSweepMarkerRe = regexp.MustCompile(`\b(?:TODO|FIXME|XXX|HACK)\b`)
	defectSweepFEMarker = regexp.MustCompile(`\b(?:TODO|FIXME)\b`)
)

// defectSweepPrunedDir reports whether a directory is excluded from the sweep.
// "build" holds generated Wails assets, "bindings" holds generated frontend
// bridges, and the rest are VCS/toolchain directories with no hand-written Go.
func defectSweepPrunedDir(name string) bool {
	switch name {
	case ".git", "build", "node_modules", "bindings", "dist":
		return true
	}
	return false
}

// defectSweepGoFiles returns the repo-relative paths of all hand-written,
// non-test Go files under the repository root.
func defectSweepGoFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." && defectSweepPrunedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo root: %v", err)
	}
	return files
}

// defectSweepScanLines yields the 1-based line numbers and contents whose text
// matches re.
func defectSweepScanLines(t *testing.T, path string, re *regexp.Regexp) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var hits []string
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if re.MatchString(scanner.Text()) {
			hits = append(hits, filepath.ToSlash(path)+":"+strconv.Itoa(lineNo)+": "+
				strings.TrimSpace(scanner.Text()))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return hits
}

// TestDefectSweepNoPanicInLibraryCode fails if any non-test Go file outside the
// main.go startup path calls panic(. Service methods must return errors instead
// of crashing the whole Wails process.
func TestDefectSweepNoPanicInLibraryCode(t *testing.T) {
	var violations []string
	for _, path := range defectSweepGoFiles(t) {
		if filepath.ToSlash(path) == "main.go" {
			continue // startup path is allowed to fail fast
		}
		violations = append(violations, defectSweepScanLines(t, path, defectSweepPanicRe)...)
	}
	for _, v := range violations {
		t.Errorf("panic in library/service code (return an error instead): %s", v)
	}
}

// TestDefectSweepNoTodoMarkersInGo fails if any non-test Go file carries
// TODO/FIXME/XXX/HACK markers.
func TestDefectSweepNoTodoMarkersInGo(t *testing.T) {
	var violations []string
	for _, path := range defectSweepGoFiles(t) {
		violations = append(violations, defectSweepScanLines(t, path, defectSweepMarkerRe)...)
	}
	for _, v := range violations {
		t.Errorf("unfinished-work marker in Go source: %s", v)
	}
}

// TestDefectSweepNoTodoMarkersInFrontend fails if any hand-written frontend
// source under frontend/src carries TODO/FIXME markers.
func TestDefectSweepNoTodoMarkersInFrontend(t *testing.T) {
	const feRoot = "frontend/src"
	if _, err := os.Stat(feRoot); err != nil {
		t.Fatalf("frontend source root missing: %v", err)
	}
	var violations []string
	err := filepath.WalkDir(feRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.Contains(strings.ToLower(d.Name()), "test") {
				return filepath.SkipDir
			}
			return nil
		}
		base := d.Name()
		if !strings.HasSuffix(base, ".ts") && !strings.HasSuffix(base, ".vue") {
			return nil
		}
		if strings.Contains(strings.ToLower(base), "test") || strings.Contains(base, ".spec.") {
			return nil
		}
		violations = append(violations, defectSweepScanLines(t, path, defectSweepFEMarker)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", feRoot, err)
	}
	for _, v := range violations {
		t.Errorf("unfinished-work marker in frontend source: %s", v)
	}
}
