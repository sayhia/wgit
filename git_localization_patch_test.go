package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// localizationRepo writes two language files where the Chinese one is missing
// two of the three keys. The audit infers the language from the path, so the
// directory must contain "loc"/"i18n"/"string"/"lang".
func localizationRepo(t *testing.T) string {
	t.Helper()
	dir := initClosedLoopRepo(t)

	locales := filepath.Join(dir, "locales")
	if err := os.MkdirAll(locales, 0755); err != nil {
		t.Fatal(err)
	}
	en := `{"menu_file":"File","menu_edit":"Edit","menu_help":"Help"}`
	zh := `{"menu_file":"文件"}`
	if err := os.WriteFile(filepath.Join(locales, "en-US.json"), []byte(en), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locales, "zh-CN.json"), []byte(zh), 0644); err != nil {
		t.Fatal(err)
	}
	writeAndCommit(t, dir, "README.md", "x\n", "chore: seed locales", time.Now(), "Alice")
	return dir
}

func TestGenerateLocalizationKeyPatchJSON(t *testing.T) {
	dir := localizationRepo(t)
	svc := GitService{}

	patch, err := svc.GenerateLocalizationKeyPatch(dir, "zh-CN", "json")
	if err != nil {
		t.Fatalf("GenerateLocalizationKeyPatch failed: %v", err)
	}
	if patch.MissingCount != 2 {
		t.Fatalf("expected 2 missing keys, got %d (%v)", patch.MissingCount, patch.Keys)
	}
	if !containsString(patch.Keys, "menu_edit") || !containsString(patch.Keys, "menu_help") {
		t.Fatalf("unexpected keys: %v", patch.Keys)
	}
	if containsString(patch.Keys, "menu_file") {
		t.Fatalf("menu_file is translated and must not appear: %v", patch.Keys)
	}
	if !strings.Contains(patch.Content, `"menu_edit": ""`) {
		t.Fatalf("json content missing the empty-value entry:\n%s", patch.Content)
	}
	// Values stay empty on purpose: copying the source text would look translated.
	if strings.Contains(patch.Content, "Edit") {
		t.Fatalf("source text must not be written as the value:\n%s", patch.Content)
	}
	if !strings.Contains(patch.SuggestedPath, "zh-CN") {
		t.Fatalf("suggested path should point at the zh-CN file, got %q", patch.SuggestedPath)
	}
}

func TestGenerateLocalizationKeyPatchCSVHasHeader(t *testing.T) {
	dir := localizationRepo(t)
	svc := GitService{}

	patch, err := svc.GenerateLocalizationKeyPatch(dir, "zh-CN", "csv")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(patch.Content), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected header + 2 rows, got %d:\n%s", len(lines), patch.Content)
	}
	if !strings.HasPrefix(lines[0], "key,sourceText,translation") {
		t.Fatalf("unexpected header: %q", lines[0])
	}
	if !strings.Contains(patch.Content, "menu_edit,Edit") {
		t.Fatalf("csv should carry the source text in its own column:\n%s", patch.Content)
	}
}

func TestGenerateLocalizationKeyPatchTSCommentsSourceText(t *testing.T) {
	dir := localizationRepo(t)
	svc := GitService{}

	patch, err := svc.GenerateLocalizationKeyPatch(dir, "zh-CN", "ts")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if !strings.Contains(patch.Content, `"menu_edit": "",`) {
		t.Fatalf("ts entry missing:\n%s", patch.Content)
	}
	if !strings.Contains(patch.Content, "// Edit") {
		t.Fatalf("ts should keep the source text as a comment:\n%s", patch.Content)
	}
}

func TestGenerateLocalizationKeyPatchRejectsUnknownFormat(t *testing.T) {
	dir := localizationRepo(t)
	svc := GitService{}
	if _, err := svc.GenerateLocalizationKeyPatch(dir, "zh-CN", "yaml"); err == nil {
		t.Fatal("expected an error for an unsupported format")
	}
}

func TestGenerateLocalizationKeyPatchPicksLanguageWhenOmitted(t *testing.T) {
	dir := localizationRepo(t)
	svc := GitService{}

	patch, err := svc.GenerateLocalizationKeyPatch(dir, "", "json")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if patch.TargetLanguage != "zh-CN" {
		t.Fatalf("expected zh-CN to be picked automatically, got %q", patch.TargetLanguage)
	}
	if patch.MissingCount != 2 {
		t.Fatalf("expected 2 missing keys, got %d", patch.MissingCount)
	}
}

func TestGenerateLocalizationKeyPatchReportsNothingMissing(t *testing.T) {
	dir := localizationRepo(t)
	svc := GitService{}

	// English is the base language, so it has nothing missing.
	patch, err := svc.GenerateLocalizationKeyPatch(dir, "en-US", "json")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if patch.MissingCount != 0 {
		t.Fatalf("expected no gaps, got %d", patch.MissingCount)
	}
	if patch.Message == "" {
		t.Fatal("expected an explanatory message when there is nothing to do")
	}
	if patch.Content != "" {
		t.Fatalf("no content should be rendered, got %q", patch.Content)
	}
}
