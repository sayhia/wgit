package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditIgnoreRulesNormalizesCasePatterns(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "Assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitignore := `# Unity cache
Library/
Temp/
Obj/
Build/
Logs/
MemoryCaptures/
*.apk
*.unitypackage
`
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.AuditIgnoreRules(repo)
	if err != nil {
		t.Fatalf("audit ignore rules: %v", err)
	}
	if res.EngineType != "Unity 3D" || !res.HasGitIgnore {
		t.Fatalf("unexpected engine/ignore state: %+v", res)
	}
	if len(res.MissingRules) != 0 {
		t.Fatalf("common Library/ style rules should match [Ll]ibrary/ recommendations: %+v", res.MissingRules)
	}
	if res.HealthScore != 100 || res.TrackedCount != 0 {
		t.Fatalf("expected clean ignore audit: %+v", res)
	}
}

func TestApplyIgnoreSanitizerDoesNotDuplicateNormalizedRules(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "Assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitignore := `Library/
Temp/
Obj/
Build/
Builds/
Logs/
MemoryCaptures/
*.apk
*.unitypackage
`
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := GitService{}.ApplyIgnoreSanitizer(repo, "Unity 3D", false)
	if err != nil {
		t.Fatalf("apply ignore sanitizer: %v", err)
	}
	if len(res.RulesAdded) != 0 {
		t.Fatalf("normalized existing rules must not be duplicated: %+v", res.RulesAdded)
	}
}

func TestApplyIgnoreSanitizerRejectsUnreadableGitignore(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based fixture is meaningless as root")
	}
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "Assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, ".gitignore")
	if err := os.WriteFile(path, []byte("Library/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	if _, err := (GitService{}).ApplyIgnoreSanitizer(repo, "Unity 3D", false); err == nil {
		t.Fatal("an unreadable .gitignore must not be treated as empty")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "Library/\n" {
		t.Fatalf("original .gitignore must be preserved: %q err=%v", data, err)
	}
}

func TestApplyIgnoreSanitizerReturnsWriteError(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "Assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, ".gitignore"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (GitService{}).ApplyIgnoreSanitizer(repo, "Unity 3D", false); err == nil {
		t.Fatal("unwritable .gitignore must return an error")
	}
}
