package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditSymbolsAndDefinesScansRealSources(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := ExecGit(repo, args...)
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
		return out
	}
	write := func(name string, content string) {
		t.Helper()
		full := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")
	write("Assets/csc.rsp", "-define:BASE_ONLY;SHARED\n")
	write("ProjectSettings/ProjectSettings.asset", "PlayerSettings:\n  scriptingDefineSymbols:\n    Standalone: SHARED;BASE_PROJECT\n")
	run("add", "-A")
	run("commit", "-qm", "base defines")

	run("checkout", "-q", "-b", "feature")
	write("Assets/csc.rsp", "-define:TARGET_ONLY;SHARED;ENABLE_PROFILER\n")
	write("Assets/Hero.asmdef", `{"defineConstraints":["TARGET_ONLY","!UNITY_EDITOR"]}`)
	write("Source/MyGame.Build.cs", "PublicDefinitions.Add(\"UE_ONLY\");\n")
	write("ProjectSettings/ProjectSettings.asset", "PlayerSettings:\n  scriptingDefineSymbols:\n    Standalone: SHARED;TARGET_PROJECT\n")
	run("add", "-A")
	run("commit", "-qm", "feature defines")

	res, err := GitService{}.AuditSymbolsAndDefines(repo, "main", "feature")
	if err != nil {
		t.Fatalf("audit symbols: %v", err)
	}
	if !res.AnalysisKnown || res.Severity != "CRITICAL" {
		t.Fatalf("expected real critical defines audit: %+v", res)
	}
	if res.TotalDefines != len(res.TargetDefines) || res.TotalDefines != 6 {
		t.Fatalf("unexpected target define count: %+v", res)
	}
	contains := func(values []string, want string) bool {
		for _, value := range values {
			if value == want {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"TARGET_ONLY", "SHARED", "ENABLE_PROFILER", "UNITY_EDITOR", "UE_ONLY", "TARGET_PROJECT"} {
		if !contains(res.TargetDefines, want) {
			t.Fatalf("missing target define %s: %+v", want, res.TargetDefines)
		}
	}
	if !contains(res.AddedDefines, "TARGET_ONLY") || !contains(res.AddedDefines, "UE_ONLY") || !contains(res.RemovedDefines, "BASE_ONLY") {
		t.Fatalf("unexpected define diff: added=%+v removed=%+v", res.AddedDefines, res.RemovedDefines)
	}
	if len(res.DangerousDebugDefines) != 1 || !strings.Contains(res.DangerousDebugDefines[0], "ENABLE_PROFILER") {
		t.Fatalf("expected ENABLE_PROFILER to be dangerous: %+v", res.DangerousDebugDefines)
	}
	if !strings.Contains(res.Summary, "新增") || !strings.Contains(res.Summary, "扫描") {
		t.Fatalf("summary should describe real scan: %q", res.Summary)
	}

	if _, err := (GitService{}).AuditSymbolsAndDefines(repo, "missing-ref", "feature"); err == nil {
		t.Fatal("missing base ref must return an error")
	}

	emptyRepo := t.TempDir()
	if out, err := ExecGit(emptyRepo, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("init empty: %v (%s)", err, out)
	}
	if out, err := ExecGit(emptyRepo, "config", "user.name", "Tester"); err != nil {
		t.Fatalf("config: %v (%s)", err, out)
	}
	if out, err := ExecGit(emptyRepo, "config", "user.email", "tester@example.com"); err != nil {
		t.Fatalf("config: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(emptyRepo, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := ExecGit(emptyRepo, "add", "-A"); err != nil {
		t.Fatalf("add: %v (%s)", err, out)
	}
	if out, err := ExecGit(emptyRepo, "commit", "-qm", "base"); err != nil {
		t.Fatalf("commit: %v (%s)", err, out)
	}
	emptyRes, err := GitService{}.AuditSymbolsAndDefines(emptyRepo, "HEAD", "HEAD")
	if err != nil {
		t.Fatalf("audit empty defines: %v", err)
	}
	if emptyRes.AnalysisKnown || emptyRes.Severity != "UNKNOWN" || emptyRes.TotalDefines != 0 {
		t.Fatalf("empty define repo must not claim OK: %+v", emptyRes)
	}
	if !strings.Contains(emptyRes.Summary, "未发现可解析") {
		t.Fatalf("empty define summary should be explicit: %q", emptyRes.Summary)
	}
}
