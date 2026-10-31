package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// git_asset_service_test.go 用真实临时仓库验证 AssetService 的看板收拢与
// 最小状态机。DCC 锁通过一个只劫持 `git lfs locks` 的 stub git 注入，其余
// 命令全部转发给真实 git，使锁表联动不依赖本机是否安装 git-lfs。

const assetLockStubScript = `#!/bin/sh
if [ "$1" = "lfs" ] && [ "$2" = "locks" ]; then
  printf '%s\n' "$WGIT_ASSET_LOCKS_JSON"
  exit 0
fi
exec "$WGIT_ASSET_REAL_GIT" "$@"
`

// assetBoardLookupGit 在任何 t.Setenv 改动 PATH 之前解析真实 git 路径。
func assetBoardLookupGit(t *testing.T) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	return realGit
}

// installAssetLockStub 把 stub git 排到 PATH 最前。realGit 必须在安装前用
// assetBoardLookupGit 解析好。locksJSON 为 stub 对 `git lfs locks --json` 的
// 应答，传 "[]" 表示空锁表。
func installAssetLockStub(t *testing.T, realGit, locksJSON string) {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(assetLockStubScript), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("WGIT_ASSET_REAL_GIT", realGit)
	t.Setenv("WGIT_ASSET_LOCKS_JSON", locksJSON)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// assetBoardGit 用真实 git 二进制做夹具操作。
func assetBoardGit(t *testing.T, realGit, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command(realGit, args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// assetBoardIsolateGitConfig 隔离开发者全局/系统 git 配置（签名、lfs 过滤等）。
func assetBoardIsolateGitConfig(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// newAssetBoardRepo 提交 4 个模拟资产（文本替身）与 1 个非资产文件。
func newAssetBoardRepo(t *testing.T, realGit string) string {
	t.Helper()
	assetBoardIsolateGitConfig(t)
	repo := t.TempDir()
	files := map[string]string{
		"Assets/Textures/hero.png":  "fake png bytes\n",
		"Assets/Textures/icon.png":  "fake png bytes\n",
		"Assets/Models/hero.uasset": "fake uasset bytes\n",
		"Assets/Audio/bgm.wav":      "fake wav bytes\n",
		"docs/readme.md":            "# not an asset\n",
	}
	for rel, content := range files {
		full := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	assetBoardGit(t, realGit, repo, "init", "-q", "-b", "main")
	assetBoardGit(t, realGit, repo, "config", "user.name", "me")
	assetBoardGit(t, realGit, repo, "config", "user.email", "me@studio.dev")
	assetBoardGit(t, realGit, repo, "add", "-A")
	assetBoardGit(t, realGit, repo, "commit", "-q", "-m", "chore: seed assets")
	return repo
}

func assetBoardItem(t *testing.T, board *AssetBoard, path string) AssetItem {
	t.Helper()
	for _, item := range board.Items {
		if item.Path == path {
			return item
		}
	}
	t.Fatalf("asset %q not found in board items", path)
	return AssetItem{}
}

func TestAssetServiceGetAssetBoardStructure(t *testing.T) {
	realGit := assetBoardLookupGit(t)
	repo := newAssetBoardRepo(t, realGit)
	installAssetLockStub(t, realGit, "[]")

	svc := AssetService{}
	if _, err := svc.GetAssetBoard("", 0); err == nil {
		t.Fatal("empty repoPath should fail")
	}

	board, err := svc.GetAssetBoard(repo, 0)
	if err != nil {
		t.Fatalf("GetAssetBoard failed: %v", err)
	}
	if len(board.Items) != 4 {
		t.Fatalf("expected 4 asset items, got %d: %+v", len(board.Items), board.Items)
	}
	// 非资产文件不得出现在看板中。
	for _, item := range board.Items {
		if item.Path == "docs/readme.md" {
			t.Fatalf("non-asset file leaked into board: %+v", item)
		}
	}
	if assetBoardItem(t, board, "Assets/Textures/hero.png").SizeBytes == 0 {
		t.Fatal("expected non-zero SizeBytes for tracked asset")
	}
	if got := assetBoardItem(t, board, "Assets/Textures/hero.png"); got.State != assetStateTracked || got.Owner != "" {
		t.Fatalf("default state should be tracked without owner, got %+v", got)
	}
	if board.Summary[assetStateTracked] != 4 || board.Summary[assetStateLocked] != 0 ||
		board.Summary[assetStateReview] != 0 || board.Summary[assetStateApproved] != 0 {
		t.Fatalf("unexpected summary: %+v", board.Summary)
	}
	if board.GeneratedAt == "" {
		t.Fatal("GeneratedAt should be set")
	}
	foundBudgetWarning := false
	for _, w := range board.Warnings {
		if strings.Contains(w, "未配置资产体积预算") {
			foundBudgetWarning = true
		}
	}
	if !foundBudgetWarning {
		t.Fatalf("expected budget warning, got %+v", board.Warnings)
	}
}

func TestAssetServiceGetAssetBoardLimitDefaultAndCap(t *testing.T) {
	realGit := assetBoardLookupGit(t)
	assetBoardIsolateGitConfig(t)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "Assets", "Bulk"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const totalAssets = 505
	for i := 1; i <= totalAssets; i++ {
		rel := filepath.Join(repo, "Assets", "Bulk", fmt.Sprintf("a%03d.png", i))
		if err := os.WriteFile(rel, []byte("x"), 0o644); err != nil {
			t.Fatalf("write asset %d: %v", i, err)
		}
	}
	assetBoardGit(t, realGit, repo, "init", "-q", "-b", "main")
	assetBoardGit(t, realGit, repo, "config", "user.name", "me")
	assetBoardGit(t, realGit, repo, "config", "user.email", "me@studio.dev")
	assetBoardGit(t, realGit, repo, "add", "-A")
	assetBoardGit(t, realGit, repo, "commit", "-q", "-m", "chore: bulk assets")

	installAssetLockStub(t, realGit, "[]")
	svc := AssetService{}

	defaultBoard, err := svc.GetAssetBoard(repo, 0)
	if err != nil {
		t.Fatalf("GetAssetBoard default failed: %v", err)
	}
	if len(defaultBoard.Items) != assetBoardDefaultLimit {
		t.Fatalf("default limit should be %d, got %d", assetBoardDefaultLimit, len(defaultBoard.Items))
	}

	cappedBoard, err := svc.GetAssetBoard(repo, 100000)
	if err != nil {
		t.Fatalf("GetAssetBoard capped failed: %v", err)
	}
	if len(cappedBoard.Items) != assetBoardMaxLimit {
		t.Fatalf("limit should cap at %d, got %d", assetBoardMaxLimit, len(cappedBoard.Items))
	}
	// Summary 统计全量资产，不随 limit 截断。
	if got := cappedBoard.Summary[assetStateTracked]; got != totalAssets {
		t.Fatalf("summary should count all %d assets, got %d", totalAssets, got)
	}

	tinyBoard, err := svc.GetAssetBoard(repo, 7)
	if err != nil {
		t.Fatalf("GetAssetBoard tiny failed: %v", err)
	}
	if len(tinyBoard.Items) != 7 {
		t.Fatalf("explicit limit 7 should yield 7 items, got %d", len(tinyBoard.Items))
	}
}

func TestAssetServiceLockForcesLockedStateAndFallsBack(t *testing.T) {
	realGit := assetBoardLookupGit(t)
	repo := newAssetBoardRepo(t, realGit)
	svc := AssetService{}

	// 先落盘一个 review 状态，验证真实锁存在时会被强制覆盖为 locked。
	if err := svc.SetAssetState(repo, "Assets/Models/hero.uasset", assetStateReview); err != nil {
		t.Fatalf("SetAssetState failed: %v", err)
	}

	const lockedAt = "2026-01-02T03:04:05Z"
	locksJSON := fmt.Sprintf(`[{"id":"1","path":"Assets/Models/hero.uasset","owner":{"name":"maya01"},"locked_at":%q}]`, lockedAt)
	installAssetLockStub(t, realGit, locksJSON)

	board, err := svc.GetAssetBoard(repo, 0)
	if err != nil {
		t.Fatalf("GetAssetBoard failed: %v", err)
	}
	locked := assetBoardItem(t, board, "Assets/Models/hero.uasset")
	if locked.State != assetStateLocked {
		t.Fatalf("lock should force state locked, got %+v", locked)
	}
	if locked.Owner != "maya01" {
		t.Fatalf("lock owner should be maya01, got %+v", locked)
	}
	if locked.UpdatedAt != lockedAt {
		t.Fatalf("locked item UpdatedAt should come from lock, got %+v", locked)
	}
	if board.Summary[assetStateLocked] != 1 || board.Summary[assetStateReview] != 0 {
		t.Fatalf("forced lock should be reflected in summary, got %+v", board.Summary)
	}
	if got := assetBoardItem(t, board, "Assets/Textures/icon.png"); got.State != assetStateTracked {
		t.Fatalf("unlocked asset should stay tracked, got %+v", got)
	}

	// 锁释放：回落到落盘的 review 状态。
	t.Setenv("WGIT_ASSET_LOCKS_JSON", "[]")
	boardAfterRelease, err := svc.GetAssetBoard(repo, 0)
	if err != nil {
		t.Fatalf("GetAssetBoard after release failed: %v", err)
	}
	freed := assetBoardItem(t, boardAfterRelease, "Assets/Models/hero.uasset")
	if freed.State != assetStateReview {
		t.Fatalf("state should fall back to persisted review after lock release, got %+v", freed)
	}
	if freed.Owner != "" {
		t.Fatalf("released asset should have no owner, got %+v", freed)
	}
	if boardAfterRelease.Summary[assetStateLocked] != 0 || boardAfterRelease.Summary[assetStateReview] != 1 {
		t.Fatalf("unexpected summary after release: %+v", boardAfterRelease.Summary)
	}
}

func TestAssetServiceSetAssetStateValidation(t *testing.T) {
	realGit := assetBoardLookupGit(t)
	repo := newAssetBoardRepo(t, realGit)
	svc := AssetService{}

	for _, bad := range []string{"archived", "LOCKED", ""} {
		if err := svc.SetAssetState(repo, "Assets/Textures/hero.png", bad); err == nil {
			t.Fatalf("invalid state %q should be rejected", bad)
		}
	}
	// 非资产已跟踪文件不在扫描结果中。
	if err := svc.SetAssetState(repo, "docs/readme.md", assetStateReview); err == nil {
		t.Fatal("non-asset tracked path should be rejected")
	}
	// 不存在的路径。
	if err := svc.SetAssetState(repo, "Assets/Textures/missing.png", assetStateReview); err == nil {
		t.Fatal("unknown path should be rejected")
	}
	// 仓库外路径。
	if err := svc.SetAssetState(repo, "../escape.png", assetStateReview); err == nil {
		t.Fatal("path outside repo should be rejected")
	}

	if err := svc.SetAssetState(repo, "Assets/Textures/hero.png", assetStateApproved); err != nil {
		t.Fatalf("valid SetAssetState failed: %v", err)
	}
	states, err := svc.GetAssetStates(repo)
	if err != nil {
		t.Fatalf("GetAssetStates failed: %v", err)
	}
	if states["Assets/Textures/hero.png"] != assetStateApproved {
		t.Fatalf("persisted state mismatch: %+v", states)
	}
}

func TestAssetServiceCorruptStatesFileTolerated(t *testing.T) {
	realGit := assetBoardLookupGit(t)
	repo := newAssetBoardRepo(t, realGit)
	installAssetLockStub(t, realGit, "[]")
	svc := AssetService{}

	if err := os.MkdirAll(filepath.Join(repo, assetStatesDirName), 0o755); err != nil {
		t.Fatalf("mkdir .wgit: %v", err)
	}
	if err := os.WriteFile(assetStatesPath(repo), []byte("{ this is not json"), 0o644); err != nil {
		t.Fatalf("write corrupt states file: %v", err)
	}

	states, err := svc.GetAssetStates(repo)
	if err != nil {
		t.Fatalf("GetAssetStates should tolerate corrupt file, got error: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("corrupt file should yield empty states, got %+v", states)
	}

	board, err := svc.GetAssetBoard(repo, 0)
	if err != nil {
		t.Fatalf("GetAssetBoard should tolerate corrupt file, got error: %v", err)
	}
	corruptWarned := false
	for _, w := range board.Warnings {
		if strings.Contains(w, "已损坏") {
			corruptWarned = true
		}
	}
	if !corruptWarned {
		t.Fatalf("expected corruption warning, got %+v", board.Warnings)
	}

	// 损坏后可重建：写入新状态成功，且旧垃圾内容被覆盖。
	if err := svc.SetAssetState(repo, "Assets/Audio/bgm.wav", assetStateReview); err != nil {
		t.Fatalf("SetAssetState after corruption failed: %v", err)
	}
	rebuilt, err := svc.GetAssetStates(repo)
	if err != nil {
		t.Fatalf("GetAssetStates after rebuild failed: %v", err)
	}
	if len(rebuilt) != 1 || rebuilt["Assets/Audio/bgm.wav"] != assetStateReview {
		t.Fatalf("rebuilt states mismatch: %+v", rebuilt)
	}

	boardAfter, err := svc.GetAssetBoard(repo, 0)
	if err != nil {
		t.Fatalf("GetAssetBoard after rebuild failed: %v", err)
	}
	for _, w := range boardAfter.Warnings {
		if strings.Contains(w, "已损坏") {
			t.Fatalf("corruption warning should disappear after rebuild: %+v", boardAfter.Warnings)
		}
	}
}

func TestAssetServiceGetAssetStatesRoundTrip(t *testing.T) {
	repo := newAssetBoardRepo(t, assetBoardLookupGit(t))
	svc := AssetService{}

	states, err := svc.GetAssetStates(repo)
	if err != nil {
		t.Fatalf("GetAssetStates on fresh repo failed: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("fresh repo should have no persisted states, got %+v", states)
	}

	if err := svc.SetAssetState(repo, "Assets/Textures/hero.png", assetStateApproved); err != nil {
		t.Fatalf("SetAssetState approved failed: %v", err)
	}
	if err := svc.SetAssetState(repo, "Assets/Audio/bgm.wav", assetStateReview); err != nil {
		t.Fatalf("SetAssetState review failed: %v", err)
	}
	if err := svc.SetAssetState(repo, "Assets/Models/hero.uasset", assetStateTracked); err != nil {
		t.Fatalf("SetAssetState tracked failed: %v", err)
	}

	states, err = svc.GetAssetStates(repo)
	if err != nil {
		t.Fatalf("GetAssetStates failed: %v", err)
	}
	want := map[string]string{
		"Assets/Textures/hero.png":  assetStateApproved,
		"Assets/Audio/bgm.wav":      assetStateReview,
		"Assets/Models/hero.uasset": assetStateTracked,
	}
	for path, expect := range want {
		if got := states[path]; got != expect {
			t.Fatalf("state for %s: want %q got %q (all: %+v)", path, expect, got, states)
		}
	}
	if len(states) != len(want) {
		t.Fatalf("unexpected extra states: %+v", states)
	}
}
