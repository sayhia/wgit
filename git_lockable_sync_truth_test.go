package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// lockableSyncFakeGitScript stubs only the LFS/config surface consumed by
// SyncLockableFilePermissions: the current user identity, lfs.locksverify and
// the remote lock inventory. Everything else is forwarded to the real binary,
// which is resolved before the stub is placed on PATH. The identity defaults to
// "me"/"me@studio.dev" and can be overridden per test via
// setLockableSyncIdentity.
const lockableSyncFakeGitScript = `#!/bin/sh
real="$WGIT_LOCKABLE_REAL_GIT"

if [ "$1" = "config" ] && [ "$2" = "--get" ]; then
  case "$3" in
    user.name)
      printf '%s\n' "${WGIT_LOCKABLE_USER_NAME-me}"
      exit 0
      ;;
    user.email)
      printf '%s\n' "${WGIT_LOCKABLE_USER_EMAIL-me@studio.dev}"
      exit 0
      ;;
    lfs.locksverify)
      printf 'true\n'
      exit 0
      ;;
  esac
fi

if [ "$1" = "lfs" ] && [ "$2" = "locks" ] && [ "$3" = "--json" ]; then
  printf '%s\n' "$WGIT_LOCKABLE_LOCKS_JSON"
  exit 0
fi

exec "$real" "$@"
`

// installLockableSyncFakeGit puts the stub git ahead of the real one.
// realGit must have been resolved with exec.LookPath before this call.
func installLockableSyncFakeGit(t *testing.T, realGit, locksJSON string) {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(lockableSyncFakeGitScript), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("WGIT_LOCKABLE_REAL_GIT", realGit)
	t.Setenv("WGIT_LOCKABLE_LOCKS_JSON", locksJSON)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// setLockableSyncIdentity makes the stubbed git report the given identity to
// SyncLockableFilePermissions instead of the default me/me@studio.dev.
func setLockableSyncIdentity(t *testing.T, name, email string) {
	t.Helper()
	t.Setenv("WGIT_LOCKABLE_USER_NAME", name)
	t.Setenv("WGIT_LOCKABLE_USER_EMAIL", email)
}

// lockableSyncIsolateGitConfig keeps the fixture independent of the developer's
// global/system git configuration.
func lockableSyncIsolateGitConfig(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// lockableSyncGit runs the real git binary directly so fixture setup never goes
// through the stub installed later.
func lockableSyncGit(t *testing.T, realGit, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command(realGit, args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// newLockableSyncRepo creates a committed repo whose .gitattributes marks
// *.uasset as lockable and seeds the given relative files.
func newLockableSyncRepo(t *testing.T, realGit string, files ...string) string {
	t.Helper()
	repo := t.TempDir()
	lockableSyncGit(t, realGit, repo, "init", "-q", "-b", "main")
	lockableSyncGit(t, realGit, repo, "config", "user.name", "me")
	lockableSyncGit(t, realGit, repo, "config", "user.email", "me@studio.dev")

	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"),
		[]byte("*.uasset lockable\n"), 0o644); err != nil {
		t.Fatalf("write .gitattributes: %v", err)
	}
	for _, rel := range files {
		full := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte("asset\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return repo
}

func lockableSyncPerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

func TestSyncLockableFilePermissionsRejectsUnscannablePaths(t *testing.T) {
	svc := GitService{}

	missing := filepath.Join(t.TempDir(), "missing-repo")
	if res, err := svc.SyncLockableFilePermissions(missing); err == nil || res != nil {
		t.Fatalf("nonexistent path must fail, got res=%+v err=%v", res, err)
	} else if !strings.Contains(err.Error(), "扫描仓库目录失败") {
		t.Fatalf("nonexistent path error = %v, want it to mention 扫描仓库目录失败", err)
	}

	regular := filepath.Join(t.TempDir(), "Assets.uasset")
	if err := os.WriteFile(regular, []byte("asset\n"), 0o644); err != nil {
		t.Fatalf("write regular file: %v", err)
	}
	if res, err := svc.SyncLockableFilePermissions(regular); err == nil || res != nil {
		t.Fatalf("regular file must fail, got res=%+v err=%v", res, err)
	} else if !strings.Contains(err.Error(), "扫描仓库目录失败") {
		t.Fatalf("regular file error = %v, want it to mention 扫描仓库目录失败", err)
	}

	// The regular file must not have been touched or made read-only.
	if perm := lockableSyncPerm(t, regular); perm != 0o644 {
		t.Fatalf("regular file permissions changed to %o", perm)
	}
}

func TestSyncLockableFilePermissionsMatchesLockOwnership(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	lockableSyncIsolateGitConfig(t)

	repo := newLockableSyncRepo(t, realGit, "Content/Hero.uasset", "Content/Prop.uasset")
	installLockableSyncFakeGit(t, realGit,
		`[{"id":"lock-1","path":"Content/Hero.uasset","owner":{"name":"me"},"locked_at":"2026-01-01T00:00:00Z"}]`)

	res, err := (GitService{}).SyncLockableFilePermissions(repo)
	if err != nil {
		t.Fatalf("SyncLockableFilePermissions failed: %v", err)
	}
	if res == nil {
		t.Fatal("SyncLockableFilePermissions returned nil result")
	}
	if !res.IsProtectionActive {
		t.Errorf("IsProtectionActive = false, want true (lfs.locksverify=true)")
	}
	if res.TotalLockableFiles != 2 {
		t.Errorf("TotalLockableFiles = %d, want 2", res.TotalLockableFiles)
	}
	if res.WritableCount != 1 {
		t.Errorf("WritableCount = %d, want 1 (file locked by me)", res.WritableCount)
	}
	if res.ReadOnlyCount != 1 {
		t.Errorf("ReadOnlyCount = %d, want 1 (file locked by someone else)", res.ReadOnlyCount)
	}
	if res.FailedCount != 0 || len(res.FailedFiles) != 0 {
		t.Errorf("unexpected failures: count=%d files=%v", res.FailedCount, res.FailedFiles)
	}
	if res.FailedFiles == nil {
		t.Error("FailedFiles must be initialized, not nil")
	}
	if len(res.ProtectedFiles) != 1 || res.ProtectedFiles[0] != "Content/Prop.uasset" {
		t.Errorf("ProtectedFiles = %v, want [Content/Prop.uasset]", res.ProtectedFiles)
	}

	if perm := lockableSyncPerm(t, filepath.Join(repo, "Content", "Hero.uasset")); perm != 0o644 {
		t.Errorf("user-locked Hero.uasset mode = %o, want 0644", perm)
	}
	if perm := lockableSyncPerm(t, filepath.Join(repo, "Content", "Prop.uasset")); perm != 0o444 {
		t.Errorf("unlocked Prop.uasset mode = %o, want 0444", perm)
	}
}

func TestSyncLockableFilePermissionsReportsChmodFailure(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("chflags only exists on darwin")
	}

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	lockableSyncIsolateGitConfig(t)

	repo := newLockableSyncRepo(t, realGit, "Content/Hero.uasset", "Content/Prop.uasset")
	hero := filepath.Join(repo, "Content", "Hero.uasset")

	if out, err := exec.Command("chflags", "uchg", hero).CombinedOutput(); err != nil {
		t.Skipf("chflags uchg unavailable: %v (%s)", err, out)
	}
	t.Cleanup(func() {
		// Must run before t.TempDir removal: an immutable file cannot be deleted.
		_ = exec.Command("chflags", "nouchg", hero).Run()
		_ = os.Chmod(hero, 0o644)
	})
	// Probe once: some filesystems silently ignore uchg, in which case a chmod
	// failure cannot be simulated and this environment-dependent case is skipped.
	if probeErr := os.Chmod(hero, 0o644); probeErr == nil {
		t.Skip("filesystem ignores uchg, chmod failure cannot be simulated")
	}

	installLockableSyncFakeGit(t, realGit,
		`[{"id":"lock-1","path":"Content/Hero.uasset","owner":{"name":"me"},"locked_at":"2026-01-01T00:00:00Z"}]`)

	res, err := (GitService{}).SyncLockableFilePermissions(repo)
	if err != nil {
		t.Fatalf("SyncLockableFilePermissions failed: %v", err)
	}
	if res == nil {
		t.Fatal("SyncLockableFilePermissions returned nil result")
	}
	if res.FailedCount != 1 {
		t.Fatalf("FailedCount = %d, want 1 (immutable file): %+v", res.FailedCount, res)
	}
	if len(res.FailedFiles) != 1 || !strings.HasPrefix(res.FailedFiles[0], "Content/Hero.uasset: ") {
		t.Fatalf("FailedFiles = %v, want one entry for Content/Hero.uasset", res.FailedFiles)
	}
	if res.WritableCount != 0 {
		t.Errorf("WritableCount = %d, want 0 (the failed chmod must not count as writable)", res.WritableCount)
	}
	if res.ReadOnlyCount != 1 {
		t.Errorf("ReadOnlyCount = %d, want 1 (Prop.uasset)", res.ReadOnlyCount)
	}
	if len(res.ProtectedFiles) != 1 || res.ProtectedFiles[0] != "Content/Prop.uasset" {
		t.Errorf("ProtectedFiles = %v, want [Content/Prop.uasset]", res.ProtectedFiles)
	}
	if perm := lockableSyncPerm(t, hero); perm != 0o644 {
		t.Errorf("immutable Hero.uasset mode = %o, want unchanged 0644", perm)
	}
}

// TestSyncLockableFilePermissionsRejectsPartialOwnerMatch guards against the
// substring ownership check that let "Al" treat a lock owned by "Alice" as his
// own and write to a file somebody else had locked.
func TestSyncLockableFilePermissionsRejectsPartialOwnerMatch(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	lockableSyncIsolateGitConfig(t)

	repo := newLockableSyncRepo(t, realGit, "Content/Hero.uasset")
	setLockableSyncIdentity(t, "Al", "al@example.com")
	installLockableSyncFakeGit(t, realGit,
		`[{"id":"lock-1","path":"Content/Hero.uasset","owner":{"name":"Alice"},"locked_at":"2026-01-01T00:00:00Z"}]`)

	res, err := (GitService{}).SyncLockableFilePermissions(repo)
	if err != nil {
		t.Fatalf("SyncLockableFilePermissions failed: %v", err)
	}
	if res == nil {
		t.Fatal("SyncLockableFilePermissions returned nil result")
	}
	if !res.IsProtectionActive {
		t.Errorf("IsProtectionActive = false, want true (lfs.locksverify=true)")
	}
	if res.WritableCount != 0 {
		t.Errorf("WritableCount = %d, want 0 (lock owner Alice is not user Al)", res.WritableCount)
	}
	if res.ReadOnlyCount != 1 {
		t.Errorf("ReadOnlyCount = %d, want 1", res.ReadOnlyCount)
	}
	if len(res.ProtectedFiles) != 1 || res.ProtectedFiles[0] != "Content/Hero.uasset" {
		t.Errorf("ProtectedFiles = %v, want [Content/Hero.uasset]", res.ProtectedFiles)
	}
	if perm := lockableSyncPerm(t, filepath.Join(repo, "Content", "Hero.uasset")); perm != 0o444 {
		t.Errorf("Hero.uasset mode = %o, want 0444 (locked by Alice)", perm)
	}
}

// TestSyncLockableFilePermissionsMatchesOwnerExactly checks that ownership is
// matched exactly and case-insensitively on user.name and user.email.
func TestSyncLockableFilePermissionsMatchesOwnerExactly(t *testing.T) {
	cases := []struct {
		name     string
		userName string
		userMail string
		owner    string
	}{
		{name: "name differs only by case", userName: "al", userMail: "al.alice@example.com", owner: "AL"},
		{name: "email differs only by case", userName: "bob", userMail: "al@example.com", owner: "AL@EXAMPLE.COM"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatalf("locate git: %v", err)
			}
			lockableSyncIsolateGitConfig(t)

			repo := newLockableSyncRepo(t, realGit, "Content/Hero.uasset")
			setLockableSyncIdentity(t, tc.userName, tc.userMail)
			installLockableSyncFakeGit(t, realGit,
				`[{"id":"lock-1","path":"Content/Hero.uasset","owner":{"name":"`+tc.owner+`"},"locked_at":"2026-01-01T00:00:00Z"}]`)

			res, err := (GitService{}).SyncLockableFilePermissions(repo)
			if err != nil {
				t.Fatalf("SyncLockableFilePermissions failed: %v", err)
			}
			if res == nil {
				t.Fatal("SyncLockableFilePermissions returned nil result")
			}
			if res.WritableCount != 1 {
				t.Errorf("WritableCount = %d, want 1 (owner %q matches identity)", res.WritableCount, tc.owner)
			}
			if res.ReadOnlyCount != 0 || len(res.ProtectedFiles) != 0 {
				t.Errorf("ReadOnlyCount = %d, ProtectedFiles = %v, want 0 and none", res.ReadOnlyCount, res.ProtectedFiles)
			}
			if perm := lockableSyncPerm(t, filepath.Join(repo, "Content", "Hero.uasset")); perm != 0o644 {
				t.Errorf("Hero.uasset mode = %o, want 0644 (locked by current user)", perm)
			}
		})
	}
}
