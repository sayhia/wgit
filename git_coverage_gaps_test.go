package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

// This file pins the pure parsing / classification helpers surfaced by the
// round-12 coverage sweep (functions below 50% that needed no git binary,
// no GUI and no network). Each test is table-driven so the next sweep can
// extend expectations without new scaffolding.

// TestCoverageGapDetectSourceLanguage covers the project browser language
// badge mapper: special filenames take priority over extensions, the lookup
// is case-insensitive, and unknown extensions degrade to "Text" rather than
// an empty badge.
func TestCoverageGapDetectSourceLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"Dockerfile", "Dockerfile"},
		{"docker/Dockerfile.dev", "Dockerfile"},
		{"Makefile", "Makefile"},
		{"Taskfile.yml", "YAML"},
		{"Gemfile", "Ruby"},
		{".gitignore", "Git"},
		{".gitmodules", "Git"},
		{".editorconfig", "Config"},
		{".env.local", "Config"},
		{"main.go", "Go"},
		{"go.mod", "Go"},
		{"app/index.tsx", "TypeScript React"},
		{"app/utils.mjs", "JavaScript"},
		{"Comp.vue", "Vue"},
		{"main.py", "Python"},
		{"lib.rs", "Rust"},
		{"engine.cpp", "C++"},
		{"math.INL", "C++"}, // case-insensitive extension
		{"Scene.cpp.H", "C"},
		{"Game.cs", "C#"},
		{"boot.swift", "Swift"},
		{"page.phtml", "PHP"},
		{"model.rb", "Ruby"},
		{"lib.zig", "Zig"},
		{"light.usf", "Shader"},
		{"compute.wgsl", "Shader"},
		{"index.html", "HTML"},
		{"theme.scss", "CSS"},
		{"data.json5", "JSON"},
		{"README.MD", "Markdown"},
		{"ci.yml", "YAML"},
		{"Cargo.toml", "TOML"},
		{"scene.xaml", "XML"},
		{"logo.svg", "SVG"},
		{"tex.PNG", "Image"},
		{"deploy.sh", "Shell"},
		{"setup.ps1", "PowerShell"},
		{"init.lua", "Lua"},
		{"query.sql", "SQL"},
		{"api.graphql", "GraphQL"},
		{"fix.patch", "Diff"},
		{"app.properties", "Config"},
		{"notes.txt", "Text"},
		{"blob.unknownext", "Text"},
	}

	for _, tc := range tests {
		if got := detectSourceLanguage(tc.path); got != tc.want {
			t.Errorf("detectSourceLanguage(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestCoverageGapDetectAssetMime covers the asset metadata MIME mapper used by
// the asset panels: the lookup is case-insensitive and unknown extensions fall
// back to application/octet-stream.
func TestCoverageGapDetectAssetMime(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".png", "image/png"},
		{".JPG", "image/jpeg"},
		{".jpeg", "image/jpeg"},
		{".gif", "image/gif"},
		{".webp", "image/webp"},
		{".svg", "image/svg+xml"},
		{".mp4", "video/mp4"},
		{".MOV", "video/quicktime"},
		{".mp3", "audio/mpeg"},
		{".wav", "audio/wav"},
		{".ogg", "audio/ogg"},
		{".fbx", "model/fbx"},
		{".obj", "model/obj"},
		{".blend", "application/x-blender"},
		{".uasset", "application/x-unreal-asset"},
		{".psd", "image/vnd.adobe.photoshop"},
		{".docx", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tc := range tests {
		if got := detectAssetMime(tc.ext); got != tc.want {
			t.Errorf("detectAssetMime(%q) = %q, want %q", tc.ext, got, tc.want)
		}
	}
}

// TestCoverageGapDetermineNodeCategory covers the blueprint node classifier
// behind the asset graph diff: the keyword checks are ordered, so an Event
// class wins over Function even when both substrings appear.
func TestCoverageGapDetermineNodeCategory(t *testing.T) {
	tests := []struct {
		cls  string
		want string
	}{
		{"K2Node_Event", "Logic Event"},
		{"CustomEventFunctionEntry", "Logic Event"}, // Event outranks Function
		{"K2Node_FunctionEntry", "Function Call"},
		{"K2Node_CallFunction", "Function Call"},
		{"K2Node_VariableGet", "Variable"},
		{"K2Node_IfThenElse", "Flow Control"},
		{"K2Node_Sequence", "Flow Control"},
		{"K2Node_SomethingElse", "Graph Node"},
		{"", "Graph Node"},
	}

	for _, tc := range tests {
		if got := determineNodeCategory(tc.cls); got != tc.want {
			t.Errorf("determineNodeCategory(%q) = %q, want %q", tc.cls, got, tc.want)
		}
	}
}

// TestCoverageGapParseBundleManifestEntryJSON covers the release bundle
// manifest entry parser: explicit name aliases, size/version/asset alias
// families, scalar-only shapes, the implicit assetCount fallback, and the
// rejection of entries without a name or with an unsupported shape.
func TestCoverageGapParseBundleManifestEntryJSON(t *testing.T) {
	// argName is the map-key name passed to the parser; the rejection cases
	// below pass "" because a non-empty fallback name would be accepted.
	tests := []struct {
		name    string
		argName string
		raw     interface{}
		wantOK  bool
		want    bundleManifestEntry
	}{
		{
			name:    "canonical map",
			argName: "tex.bundle",
			raw: map[string]interface{}{
				"name": "textures",
				"size": json.Number("2048"),
			},
			wantOK: true,
			want:   bundleManifestEntry{name: "textures", size: 2048},
		},
		{
			name:    "non-string explicit name keeps the map-key name",
			argName: "tex.bundle",
			raw: map[string]interface{}{
				"name": json.Number("1234"),
				"size": json.Number("5"),
			},
			wantOK: true,
			want:   bundleManifestEntry{name: "tex.bundle", size: 5},
		},
		{
			name: "aliased map with version and assets",
			raw: map[string]interface{}{
				"bundleName": "shaders",
				"SizeBytes":  json.Number("4096"),
				"Hash":       json.Number("7"),
				"AssetPaths": []interface{}{"a.uasset", " b.umap ", ""},
			},
			wantOK: true,
			want: bundleManifestEntry{
				name:       "shaders",
				size:       4096,
				version:    "7",
				assets:     []string{"a.uasset", " b.umap "},
				assetCount: 2,
			},
		},
		{
			name: "assetCount fallback to len(assets)",
			raw: map[string]interface{}{
				"name":   "audio",
				"assets": []interface{}{"s1.wav", "s2.wav", "s3.wav"},
			},
			wantOK: true,
			want: bundleManifestEntry{
				name:       "audio",
				assets:     []string{"s1.wav", "s2.wav", "s3.wav"},
				assetCount: 3,
			},
		},
		{
			name: "explicit assetCount wins",
			raw: map[string]interface{}{
				"name":   "audio",
				"count":  json.Number("9"),
				"assets": []interface{}{"s1.wav"},
			},
			wantOK: true,
			want: bundleManifestEntry{
				name:       "audio",
				assets:     []string{"s1.wav"},
				assetCount: 9,
			},
		},
		{
			name:    "bare json.Number",
			argName: "tex.bundle",
			raw:     json.Number("8192"),
			wantOK:  true,
			want:    bundleManifestEntry{name: "tex.bundle", size: 8192},
		},
		{
			name:    "bare float64",
			argName: "tex.bundle",
			raw:     float64(100.9),
			wantOK:  true,
			want:    bundleManifestEntry{name: "tex.bundle", size: 100},
		},
		{
			name:    "bare string is a version",
			argName: "tex.bundle",
			raw:     "v1.2.3",
			wantOK:  true,
			want:    bundleManifestEntry{name: "tex.bundle", version: "v1.2.3"},
		},
		{
			name:    "bare array is an asset list",
			argName: "tex.bundle",
			raw:     []interface{}{"m.fbx", "t.png"},
			wantOK:  true,
			want: bundleManifestEntry{
				name:       "tex.bundle",
				assets:     []string{"m.fbx", "t.png"},
				assetCount: 2,
			},
		},
		{
			name:    "map without a name is rejected",
			argName: "",
			raw:     map[string]interface{}{"size": json.Number("1")},
			wantOK:  false,
		},
		{
			name:    "map with blank explicit name is rejected",
			argName: "",
			raw:     map[string]interface{}{"Name": "   "},
			wantOK:  false,
		},
		{
			name:   "scalar without a name is rejected",
			raw:    true,
			wantOK: false,
		},
		{
			name:   "nil is rejected",
			raw:    nil,
			wantOK: false,
		},
	}

	for _, tc := range tests {
		got, ok := parseBundleManifestEntryJSON(tc.argName, tc.raw)
		if ok != tc.wantOK {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: entry = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// TestCoverageGapJSONValueCoercion covers the manifest scalar coercers:
// json.Number / float64 / int / int64 / numeric strings all count, while
// non-numeric input degrades to the zero value instead of erroring.
func TestCoverageGapJSONValueCoercion(t *testing.T) {
	numTests := []struct {
		raw  interface{}
		want int64
	}{
		{json.Number("42"), 42},
		{json.Number("3.9"), 3},
		{float64(2.7), 2},
		{int64(9), 9},
		{int(7), 7},
		{" 12 ", 12},
		{"abc", 0},
		{true, 0},
		{nil, 0},
	}
	for _, tc := range numTests {
		if got := jsonNumericValue(tc.raw); got != tc.want {
			t.Errorf("jsonNumericValue(%#v) = %d, want %d", tc.raw, got, tc.want)
		}
	}

	strTests := []struct {
		raw  interface{}
		want string
	}{
		{"v1.2", "v1.2"},
		{json.Number("123"), "123"},
		{float64(1.5), "1.5"},
		{42, ""},
		{nil, ""},
		{[]interface{}{"x"}, ""},
	}
	for _, tc := range strTests {
		if got := jsonScalarString(tc.raw); got != tc.want {
			t.Errorf("jsonScalarString(%#v) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

// TestCoverageGapGetToolCandidates covers the external diff/merge tool
// registry used by the tool picker: every entry stays well-formed (unique ID,
// display name, description, at least one launch candidate, both capabilities)
// and each tool keeps its own diff/merge argument shape — the shapes are what
// the merge runner executes, so a regression here breaks every user tool.
func TestCoverageGapGetToolCandidates(t *testing.T) {
	tools := getToolCandidates()
	if len(tools) == 0 {
		t.Fatalf("getToolCandidates returned no tools")
	}

	seen := make(map[string]bool)
	for _, tc := range tools {
		if tc.ID == "" || tc.Name == "" || tc.Description == "" {
			t.Errorf("tool %q has empty ID/Name/Description: %+v", tc.ID, tc)
		}
		if seen[tc.ID] {
			t.Errorf("duplicate tool ID %q", tc.ID)
		}
		seen[tc.ID] = true
		if len(tc.Candidates) == 0 {
			t.Errorf("tool %q has no launch candidates", tc.ID)
		}
		if !tc.SupportsDiff || !tc.SupportsMerge {
			t.Errorf("tool %q should support both diff and merge", tc.ID)
		}
	}

	const (
		exe    = "/tool/bin/exe"
		left   = "a.go"
		right  = "b.go"
		ours   = "ours.go"
		theirs = "theirs.go"
		base   = "base.go"
		res    = "merged.go"
	)
	argTests := []struct {
		id        string
		wantDiff  []string
		wantMerge []string
	}{
		{"cursor", []string{exe, "--diff", left, right}, []string{exe, "--wait", "--merge", theirs, ours, base, res}},
		{"vscode", []string{exe, "--diff", left, right}, []string{exe, "--wait", "--merge", theirs, ours, base, res}},
		{"beyondcompare", []string{exe, left, right}, []string{exe, ours, theirs, base, res}},
		{"idea", []string{exe, "diff", left, right}, []string{exe, "merge", ours, theirs, base, res}},
		{"sublimemerge", []string{exe, "diff", left, right}, []string{exe, "mergetool", ours, theirs, base, res}},
		{"opendiff", []string{exe, left, right}, []string{exe, ours, theirs, "-ancestor", base, "-merge", res}},
		{"meld", []string{exe, left, right}, []string{exe, "--auto-merge", ours, base, theirs, "-o", res}},
	}
	byID := make(map[string]toolCandidate, len(tools))
	for _, tc := range tools {
		byID[tc.ID] = tc
	}
	for _, at := range argTests {
		tc, ok := byID[at.id]
		if !ok {
			t.Errorf("tool %q missing from registry", at.id)
			continue
		}
		if got := tc.DiffArgs(exe, left, right); !reflect.DeepEqual(got, at.wantDiff) {
			t.Errorf("%s.DiffArgs = %v, want %v", at.id, got, at.wantDiff)
		}
		if got := tc.MergeArgs(exe, theirs, ours, base, res); !reflect.DeepEqual(got, at.wantMerge) {
			t.Errorf("%s.MergeArgs = %v, want %v", at.id, got, at.wantMerge)
		}
	}

	// Windows-style fallback candidates stay in the list even on macOS, so the
	// picker keeps working cross-platform.
	cursor := byID["cursor"]
	foundCmdFallback := false
	for _, c := range cursor.Candidates {
		if filepath.Base(c) == "cursor.cmd" {
			foundCmdFallback = true
		}
	}
	if !foundCmdFallback {
		t.Errorf("cursor candidates lost the Windows .cmd fallback: %v", cursor.Candidates)
	}
}

// TestCoverageGapFormatBundleSize covers the bundle listing size formatter:
// sub-kilobyte sizes stay exact, larger sizes scale through the KB/MB/GB
// units with one decimal.
func TestCoverageGapFormatBundleSize(t *testing.T) {
	const k = 1024
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{k, "1.0 KB"},
		{1536, "1.5 KB"},
		{5120 * k, "5.0 MB"},
		{3 * k * k * k, "3.0 GB"},
		{2 * k * k * k * k, "2.0 TB"},
	}
	for _, tc := range tests {
		if got := formatBundleSize(tc.in); got != tc.want {
			t.Errorf("formatBundleSize(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
