package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCheckFileLockStatusUsesJSONAndReturnsErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX git wrapper")
	}
	binDir := t.TempDir()
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"lfs\" ] && [ \"$2\" = \"locks\" ]; then\n" +
		"  case \"$*\" in\n" +
		"    *--json*)\n" +
		"      if [ \"$FAKE_FAIL\" = \"1\" ]; then echo 'remote unavailable' >&2; exit 1; fi\n" +
		"      if [ \"$FAKE_NO_JSON\" = \"1\" ]; then exit 1; fi\n" +
		"      printf '%s\\n' '[{\"id\":\"42\",\"path\":\"Assets/hero.psd\",\"owner\":{\"name\":\"Alice Smith\"},\"locked_at\":\"2026-10-18T09:30:00Z\"}]'\n" +
		"      exit 0;;\n" +
		"    *)\n" +
		"      if [ \"$FAKE_FAIL\" = \"1\" ]; then echo 'remote unavailable' >&2; exit 1; fi\n" +
		"      printf 'Assets/hero.psd\\tAlice Smith\\tID:42\\n'\n" +
		"      exit 0;;\n" +
		"  esac\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("FAKE_FAIL", "")
	t.Setenv("FAKE_NO_JSON", "")

	repo := t.TempDir()
	svc := GitService{}
	owner, err := svc.CheckFileLockStatus(repo, "Assets/hero.psd")
	if err != nil || owner != "Alice Smith" {
		t.Fatalf("JSON lock lookup = %q err=%v", owner, err)
	}

	t.Setenv("FAKE_NO_JSON", "1")
	owner, err = svc.CheckFileLockStatus(repo, "Assets/hero.psd")
	if err != nil || owner != "Alice Smith" {
		t.Fatalf("plain-text fallback = %q err=%v", owner, err)
	}

	t.Setenv("FAKE_NO_JSON", "")
	t.Setenv("FAKE_FAIL", "1")
	if _, err := svc.CheckFileLockStatus(repo, "Assets/hero.psd"); err == nil || !strings.Contains(err.Error(), "remote unavailable") {
		t.Fatalf("offline lookup should surface the git error, got %v", err)
	}
}

func TestParseLfsLocksJSON(t *testing.T) {
	raw := `[{"id":"42","path":"Art/hero.psd","owner":{"name":"Alice"},"locked_at":"2026-10-18T09:30:00Z"},{"id":"43","path":"Art/boss.fbx","owner":{"name":"Bob"}}]`
	locks, ok := parseLfsLocksJSON(raw)
	if !ok {
		t.Fatal("expected valid JSON lock output")
	}
	if len(locks) != 2 {
		t.Fatalf("got %d locks: %+v", len(locks), locks)
	}
	if l := locks[0]; l.ID != "42" || l.Path != "Art/hero.psd" || l.Owner != "Alice" || l.LockedAt != "2026-10-18T09:30:00Z" {
		t.Fatalf("unexpected first lock: %+v", l)
	}
	if l := locks[1]; l.ID != "43" || l.Owner != "Bob" || l.LockedAt != "" {
		t.Fatalf("missing locked_at should stay empty: %+v", l)
	}

	if _, ok := parseLfsLocksJSON("not json"); ok {
		t.Fatal("invalid JSON should fall back to plain parsing")
	}
	if locks, ok := parseLfsLocksJSON("[]"); !ok || len(locks) != 0 {
		t.Fatalf("empty JSON array should parse as no locks: %+v ok=%v", locks, ok)
	}
}

func TestParseLfsLocksPlain(t *testing.T) {
	raw := "Art/hero.psd\tAlice\tID:42\nArt/boss.fbx  \tBob  \tID:43\n"
	locks := parseLfsLocksPlain(raw)
	if len(locks) != 2 {
		t.Fatalf("got %d locks: %+v", len(locks), locks)
	}
	if l := locks[0]; l.ID != "42" || l.Path != "Art/hero.psd" || l.Owner != "Alice" || l.LockedAt != "" {
		t.Fatalf("unexpected first lock: %+v", l)
	}
	if l := locks[1]; l.ID != "43" || l.Path != "Art/boss.fbx" || l.Owner != "Bob" {
		t.Fatalf("unexpected second lock: %+v", l)
	}
}

func TestLfsLockDuration(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		lockedAt string
		duration string
		zombie   bool
	}{
		{lockedAt: now.Add(-3 * time.Hour).Format(time.RFC3339), duration: "持有 3 小时"},
		{lockedAt: now.Add(-50 * time.Hour).Format(time.RFC3339), duration: "持有 2 天"},
		{lockedAt: now.Add(-72 * time.Hour).Format(time.RFC3339), duration: "持有 3 天"},
		{lockedAt: now.Add(-73 * time.Hour).Format(time.RFC3339), duration: "持有 3 天", zombie: true},
		{lockedAt: now.Add(-5 * time.Minute).Format(time.RFC3339), duration: "刚刚锁定"},
		{lockedAt: now.Add(time.Hour).Format(time.RFC3339), duration: "刚刚锁定"},
		{lockedAt: "", duration: "持有时间未知"},
		{lockedAt: "not-a-time", duration: "持有时间未知"},
	}
	for _, tc := range cases {
		duration, zombie := lfsLockDuration(tc.lockedAt, now)
		if duration != tc.duration || zombie != tc.zombie {
			t.Fatalf("lockedAt=%q got %q zombie=%v, want %q zombie=%v", tc.lockedAt, duration, zombie, tc.duration, tc.zombie)
		}
	}
}

func TestLfsLockActivityDetectsZombieAndCollision(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX git wrapper")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.name", "Alice")
	run("config", "user.email", "alice@example.com")
	if err := os.MkdirAll(filepath.Join(repo, "Art"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Art", "hero.psd"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git not found: %v", err)
	}
	binDir := t.TempDir()
	wrapper := "#!/bin/sh\n" +
		"if [ \"$1\" = \"lfs\" ] && [ \"$2\" = \"locks\" ] && [ \"$3\" = \"--json\" ]; then\n" +
		"  printf '%s\\n' \"$FAKE_LOCKS_JSON\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"exec \"$WGIT_REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	lockedAt := time.Now().Add(-100 * time.Hour).UTC().Format(time.RFC3339)
	t.Setenv("FAKE_LOCKS_JSON", fmt.Sprintf(`[{"id":"7","path":"Art/hero.psd","owner":{"name":"Bob"},"locked_at":%q}]`, lockedAt))
	t.Setenv("WGIT_REAL_GIT", gitPath)
	t.Setenv("PATH", binDir)

	activity, err := (GitService{}).GetLfsLockActivity(repo)
	if err != nil {
		t.Fatalf("GetLfsLockActivity failed: %v", err)
	}
	if activity.TotalLocks != 1 || activity.TeamLockCount != 1 || activity.ZombieCount != 1 || activity.CollisionCount != 1 {
		t.Fatalf("unexpected lock activity: %+v", activity)
	}
	lock := activity.Locks[0]
	if lock.LockedAt != lockedAt || !lock.IsZombie || !lock.HasConflict || !strings.Contains(lock.DurationStr, "4 天") {
		t.Fatalf("unexpected lock item: %+v", lock)
	}
	if !strings.Contains(activity.Message, "僵尸锁") || !strings.Contains(activity.Message, "冲突") {
		t.Fatalf("message should mention zombies and collisions: %q", activity.Message)
	}
}

func TestRenderLfsLocksReportEscaping(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	path := "Art/Hero, \"Final\"|v2.psd"
	owner := "QA, \"Lead\"\nTeam"
	locks := []LfsLockInfo{{
		ID:       "1",
		Path:     path,
		Owner:    owner,
		LockedAt: now.Add(-100 * time.Hour).Format(time.RFC3339),
	}}

	csvText, err := renderLfsLocksCSV(locks, now)
	if err != nil {
		t.Fatalf("renderLfsLocksCSV failed: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(csvText)).ReadAll()
	if err != nil {
		t.Fatalf("CSV should be parseable: %v\n%s", err, csvText)
	}
	if len(records) != 2 || records[1][0] != path || records[1][1] != owner || records[1][4] != "4" || records[1][5] != "true" {
		t.Fatalf("CSV round-trip mismatch: %+v", records)
	}

	md := renderLfsLocksMarkdown("repo|path", locks, now)
	if !strings.Contains(md, `Art/Hero, "Final"\|v2.psd`) {
		t.Fatalf("markdown should escape table pipes: %q", md)
	}
	if !strings.Contains(md, `QA, "Lead" Team`) {
		t.Fatalf("markdown should flatten newlines: %q", md)
	}
	if !strings.Contains(md, "僵尸锁") {
		t.Fatalf("100h-old lock should be marked zombie: %q", md)
	}
}
