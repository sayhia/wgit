package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func initClosedLoopRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")
	return dir
}

// --- range-diff -----------------------------------------------------------

func TestGetRangeDiffDetectsDroppedCommit(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "base.txt", "base\n", "chore: base", time.Now(), "Alice")
	base := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))

	writeAndCommit(t, dir, "b.txt", "b\n", "feat: b", time.Now(), "Alice")
	writeAndCommit(t, dir, "c.txt", "c\n", "feat: c", time.Now(), "Alice")
	runGitTest(t, dir, nil, "branch", "old-tip")
	runGitTest(t, dir, nil, "reset", "--hard", "HEAD~1")

	svc := GitService{}
	res, err := svc.GetRangeDiff(dir, base, "old-tip", base, "main")
	if err != nil {
		t.Fatalf("GetRangeDiff failed: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d (%+v)", len(res.Entries), res.Entries)
	}
	if res.Equal != 1 {
		t.Fatalf("expected 1 equal entry, got %d (%+v)", res.Equal, res.Entries)
	}
	if res.Removed != 1 {
		t.Fatalf("expected 1 removed entry, got %d (%+v)", res.Removed, res.Entries)
	}
	if res.Message == "" {
		t.Fatal("expected a human summary")
	}
}

func TestGetRangeDiffRejectsMissingRange(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")

	svc := GitService{}
	if _, err := svc.GetRangeDiff(dir, "", "HEAD", "HEAD", "HEAD"); err == nil {
		t.Fatal("expected an error for an empty base")
	}
}

// --- fixup / squash -------------------------------------------------------

func TestCreateFixupCommitPointsAtTarget(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: 战斗结算", time.Now(), "Alice")

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a fixed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "a.txt")

	svc := GitService{}
	res, err := svc.CreateFixupCommit(dir, "HEAD", "fixup")
	if err != nil {
		t.Fatalf("CreateFixupCommit failed: %v", err)
	}
	if res.Mode != "fixup" || res.Hash == "" {
		t.Fatalf("unexpected result: %+v", res)
	}
	subject := strings.TrimSpace(runGitTest(t, dir, nil, "log", "-1", "--pretty=%s"))
	if !strings.HasPrefix(subject, "fixup! feat: 战斗结算") {
		t.Fatalf("expected fixup! subject, got %q", subject)
	}
}

func TestCreateFixupCommitReportsUnreadableHash(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("b\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "a.txt")

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git not found: %v", err)
	}
	binDir := t.TempDir()
	wrapper := "#!/bin/sh\n" +
		"if [ \"$1\" = \"rev-parse\" ] && [ \"$2\" = \"--short\" ]; then\n" +
		"  echo 'fatal: simulated rev-parse failure' >&2\n" +
		"  exit 128\n" +
		"fi\n" +
		"exec \"$WGIT_REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WGIT_REAL_GIT", gitPath)
	t.Setenv("PATH", binDir)

	res, err := (GitService{}).CreateFixupCommit(dir, "", "amend")
	if err != nil {
		t.Fatalf("the amend itself succeeded, so it must not be reported as an error: %v", err)
	}
	if res.Hash != "" {
		t.Fatalf("expected an unknown hash, got %+v", res)
	}
	if !strings.Contains(res.Message, "读取新 HEAD 失败") {
		t.Fatalf("the message must not pretend the new hash is known: %q", res.Message)
	}
}

func TestCreateFixupCommitRejectsEmptyIndex(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")

	svc := GitService{}
	if _, err := svc.CreateFixupCommit(dir, "HEAD", "fixup"); err == nil {
		t.Fatal("expected an error when the index is clean")
	}
}

// --- merge overlap --------------------------------------------------------

func TestGetMergeOverlapWarningFindsSharedFiles(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "shared.txt", "base\n", "chore: base", time.Now(), "Alice")
	writeAndCommit(t, dir, "stable.txt", "stable\n", "chore: stable", time.Now(), "Alice")

	runGitTest(t, dir, nil, "checkout", "-b", "feature")
	writeAndCommit(t, dir, "shared.txt", "mine\n", "feat: mine", time.Now(), "Alice")
	writeAndCommit(t, dir, "only-mine.txt", "mine\n", "feat: mine only", time.Now(), "Alice")

	runGitTest(t, dir, nil, "checkout", "main")
	writeAndCommit(t, dir, "shared.txt", "theirs\n", "feat: teammate", time.Now(), "Bob")
	writeAndCommit(t, dir, "only-theirs.txt", "theirs\n", "feat: teammate only", time.Now(), "Bob")
	runGitTest(t, dir, nil, "checkout", "feature")

	svc := GitService{}
	res, err := svc.GetMergeOverlapWarning(dir, "main", 30)
	if err != nil {
		t.Fatalf("GetMergeOverlapWarning failed: %v", err)
	}
	if len(res.OverlapFiles) != 1 {
		t.Fatalf("expected exactly 1 overlapping file, got %+v", res.OverlapFiles)
	}
	if res.OverlapFiles[0].Path != "shared.txt" {
		t.Fatalf("expected shared.txt, got %q", res.OverlapFiles[0].Path)
	}
	if res.RiskLevel != "low" {
		t.Fatalf("expected low risk, got %q", res.RiskLevel)
	}
	if res.MyFileCount != 2 {
		t.Fatalf("expected 2 files on my side, got %d", res.MyFileCount)
	}
	// The UI renders from the key, so the key and its params must be present.
	if res.MessageKey != "merge.overlap.message.low" {
		t.Fatalf("unexpected overlap message key: %q", res.MessageKey)
	}
	if res.MessageParams["count"] != "1" || res.MessageParams["files"] == "" {
		t.Fatalf("overlap message is missing params: %+v", res.MessageParams)
	}
}

func TestGetMergeOverlapWarningNoOverlap(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "shared.txt", "base\n", "chore: base", time.Now(), "Alice")

	runGitTest(t, dir, nil, "checkout", "-b", "feature")
	writeAndCommit(t, dir, "mine.txt", "mine\n", "feat: mine", time.Now(), "Alice")
	runGitTest(t, dir, nil, "checkout", "main")
	writeAndCommit(t, dir, "theirs.txt", "theirs\n", "feat: theirs", time.Now(), "Bob")
	runGitTest(t, dir, nil, "checkout", "feature")

	svc := GitService{}
	res, err := svc.GetMergeOverlapWarning(dir, "main", 30)
	if err != nil {
		t.Fatalf("GetMergeOverlapWarning failed: %v", err)
	}
	if len(res.OverlapFiles) != 0 || res.RiskLevel != "none" {
		t.Fatalf("expected no overlap, got %+v", res)
	}
}

// --- object store ---------------------------------------------------------

func TestOptimizeObjectStoreWritesCommitGraph(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")
	writeAndCommit(t, dir, "b.txt", "b\n", "feat: b", time.Now(), "Alice")

	svc := GitService{}
	res, err := svc.OptimizeObjectStore(dir, []string{"commit-graph", "pack-refs"})
	if err != nil {
		t.Fatalf("OptimizeObjectStore failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %+v", res.Results)
	}
	if len(res.Results) != 2 {
		t.Fatalf("expected 2 task results, got %d", len(res.Results))
	}
	if res.GitDirBefore == "" || res.GitDirAfter == "" {
		t.Fatal("expected git dir sizes to be reported")
	}
	graph := filepath.Join(dir, ".git", "objects", "info", "commit-graph")
	if _, err := os.Stat(graph); err != nil {
		t.Fatalf("commit-graph was not written: %v", err)
	}
}

func TestOptimizeObjectStoreSkipsUnknownTask(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")

	svc := GitService{}
	res, err := svc.OptimizeObjectStore(dir, []string{"nope"})
	if err != nil {
		t.Fatalf("OptimizeObjectStore failed: %v", err)
	}
	if len(res.Results) != 1 || !res.Results[0].Skipped {
		t.Fatalf("expected a skipped entry, got %+v", res.Results)
	}
}

// --- health fix -----------------------------------------------------------

func TestRunHealthFixIdentityNeedsInput(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")

	svc := GitService{}
	res, err := svc.RunHealthFix(dir, "identity")
	if err != nil {
		t.Fatalf("RunHealthFix failed: %v", err)
	}
	if !res.NeedsInput || res.Executed {
		t.Fatalf("identity should require user input, got %+v", res)
	}
	if res.Target != "settings" {
		t.Fatalf("expected settings target, got %q", res.Target)
	}
}

func TestRunHealthFixObjectsRunsGC(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")

	svc := GitService{}
	res, err := svc.RunHealthFix(dir, "objects")
	if err != nil {
		t.Fatalf("RunHealthFix failed: %v", err)
	}
	if !res.Executed || !res.Success {
		t.Fatalf("expected objects fix to execute, got %+v", res)
	}
	if res.Hint == "" {
		t.Fatal("expected an explanatory hint")
	}
}

func TestRunHealthFixIgnoreBacksUpBeforeRewriting(t *testing.T) {
	dir := initClosedLoopRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "Assets"), 0755); err != nil {
		t.Fatal(err)
	}
	ignorePath := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(ignorePath, []byte("*.log\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := GitService{}
	res, err := svc.RunHealthFix(dir, "ignore")
	if err != nil {
		t.Fatalf("RunHealthFix failed: %v", err)
	}
	if !res.Executed || !res.Success {
		t.Fatalf("expected the ignore fix to run, got %+v", res)
	}
	if !strings.Contains(res.Output, "wgit-fix-backup") {
		t.Fatalf("expected the backup path in the output, got %q", res.Output)
	}

	matches, err := filepath.Glob(filepath.Join(gitCommonDir(dir), "wgit-fix-backup", "*", ".gitignore"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly one .gitignore backup, matches=%v err=%v", matches, err)
	}
	backup, err := os.ReadFile(matches[0])
	if err != nil || string(backup) != "*.log\n" {
		t.Fatalf("backup must preserve the original content: %q err=%v", backup, err)
	}
	updated, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "[Ll]ibrary/") {
		t.Fatalf("expected the fix to append engine rules, got %q", updated)
	}
}

func TestRunHealthFixIgnoreAbortsWhenBackupFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based fixture is meaningless as root")
	}
	dir := initClosedLoopRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "Assets"), 0755); err != nil {
		t.Fatal(err)
	}
	ignorePath := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(ignorePath, []byte("*.log\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ignorePath, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ignorePath, 0644) })

	svc := GitService{}
	res, err := svc.RunHealthFix(dir, "ignore")
	if err != nil {
		t.Fatalf("RunHealthFix should report the backup failure in its result: %v", err)
	}
	if res.Executed || res.Success {
		t.Fatalf("a failed backup must abort the fix: %+v", res)
	}
	if !strings.Contains(res.Output, "无法备份") {
		t.Fatalf("expected an explicit backup failure, got %q", res.Output)
	}
	if err := os.Chmod(ignorePath, 0644); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(ignorePath); err != nil || string(data) != "*.log\n" {
		t.Fatalf("original .gitignore must be preserved: %q err=%v", data, err)
	}
}

func TestRunHealthFixRejectsInvalidRepo(t *testing.T) {
	svc := GitService{}
	if _, err := svc.RunHealthFix(t.TempDir(), "identity"); err == nil {
		t.Fatal("a non-repository directory must not return navigation advice")
	}
}

func TestRunHealthFixRejectsUnknownKey(t *testing.T) {
	dir := initClosedLoopRepo(t)
	svc := GitService{}
	if _, err := svc.RunHealthFix(dir, "not-a-dimension"); err == nil {
		t.Fatal("expected an error for an unknown dimension")
	}
}

// --- staged large files -> LFS -------------------------------------------

func requireGitLFS(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("git-lfs not installed")
	}
	runGitTest(t, dir, nil, "lfs", "install", "--local")
}

func TestTrackStagedLargeFilesAsLfsConvertsBlob(t *testing.T) {
	dir := initClosedLoopRepo(t)
	requireGitLFS(t, dir)

	// 1 MiB of incompressible-ish payload so git cannot shrink it below the
	// pointer size we assert on.
	payload := make([]byte, 1<<20)
	for i := range payload {
		payload[i] = byte(i*37 + i/251)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Assets", "hero.psd"), payload, 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "Assets/hero.psd")

	svc := GitService{}
	res, err := svc.TrackStagedLargeFilesAsLfs(dir, []string{"Assets/hero.psd"}, true)
	if err != nil {
		t.Fatalf("TrackStagedLargeFilesAsLfs failed: %v", err)
	}
	if res.Converted != 1 || len(res.Items) != 1 {
		t.Fatalf("expected 1 conversion, got %+v", res)
	}
	if res.Items[0].NewBytes == 0 || res.Items[0].NewBytes >= res.Items[0].OldBytes {
		t.Fatalf("expected a small LFS pointer, got %+v", res.Items[0])
	}
	// The pointer must actually be what git will commit.
	out := strings.TrimSpace(runGitTest(t, dir, nil, "cat-file", "-p", ":Assets/hero.psd"))
	if !strings.HasPrefix(out, "version https://git-lfs.github.com/spec/v1") {
		t.Fatalf("staged blob is not an LFS pointer: %q", out)
	}
	// .gitattributes must record the pattern so the next artist is covered.
	attrs, err := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if err != nil {
		t.Fatalf("read .gitattributes: %v", err)
	}
	if !strings.Contains(string(attrs), "*.psd") {
		t.Fatalf(".gitattributes missing *.psd: %s", attrs)
	}
}

func TestTrackStagedLargeFilesAsLfsSkipsTrackedFile(t *testing.T) {
	dir := initClosedLoopRepo(t)
	requireGitLFS(t, dir)

	runGitTest(t, dir, nil, "lfs", "track", "*.bin")
	if err := os.WriteFile(filepath.Join(dir, "already.bin"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "already.bin")

	svc := GitService{}
	res, err := svc.TrackStagedLargeFilesAsLfs(dir, []string{"already.bin"}, false)
	if err != nil {
		t.Fatalf("TrackStagedLargeFilesAsLfs failed: %v", err)
	}
	if res.Skipped != 1 || res.Converted != 0 {
		t.Fatalf("expected the file to be skipped, got %+v", res)
	}
}

// --- pattern helpers ------------------------------------------------------

func TestLfsPatternForPath(t *testing.T) {
	cases := map[string]string{
		"Assets/Hero.PSD":    "*.PSD",
		"Content/Big.uasset": "*.uasset",
		"noextension":        "noextension",
		"dir.with.dots/file": "dir.with.dots/file",
	}
	for in, want := range cases {
		if got := lfsPatternForPath(in); got != want {
			t.Fatalf("lfsPatternForPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLfsPatternIsLockable(t *testing.T) {
	if !lfsPatternIsLockable("*.PSD") {
		t.Fatal("PSD should be lockable regardless of case")
	}
	if lfsPatternIsLockable("*.png") {
		t.Fatal("png should not be lockable")
	}
}

// --- tag signature -------------------------------------------------------

func TestVerifyTagSignatureRejectsLightweightTag(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")
	runGitTest(t, dir, nil, "tag", "v1.0.0")

	svc := GitService{}
	res, err := svc.VerifyTagSignature(dir, "v1.0.0")
	if err != nil {
		t.Fatalf("VerifyTagSignature failed: %v", err)
	}
	if res.Verified {
		t.Fatal("a lightweight tag can never be verified")
	}
	if !strings.Contains(res.Message, "轻量标签") {
		t.Fatalf("expected an explanatory message, got %q", res.Message)
	}
}

func TestVerifyTagSignatureRejectsMissingTag(t *testing.T) {
	dir := initClosedLoopRepo(t)
	svc := GitService{}
	if _, err := svc.VerifyTagSignature(dir, "v9.9.9"); err == nil {
		t.Fatal("expected an error for a missing tag")
	}
}

func TestVerifyTagSignatureHandlesUnsignedAnnotatedTag(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")
	runGitTest(t, dir, nil, "tag", "-a", "v1.1.0", "-m", "release 1.1.0")

	svc := GitService{}
	res, err := svc.VerifyTagSignature(dir, "v1.1.0")
	if err != nil {
		t.Fatalf("VerifyTagSignature failed: %v", err)
	}
	if res.Verified {
		t.Fatal("an unsigned annotated tag must not verify")
	}
	if res.Message == "" {
		t.Fatal("expected an explanatory message")
	}
}

func TestGetMergeOverlapWarningRejectsInvalidBranch(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")
	if _, err := (GitService{}).GetMergeOverlapWarning(dir, "missing-branch", 7); err == nil {
		t.Fatal("invalid target branch must not be reported as no overlap")
	}
}

func TestVerifyTagSignaturePropagatesMissingTagObject(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")
	runGitTest(t, dir, nil, "tag", "-a", "v1.2.0", "-m", "release 1.2.0")
	out, err := ExecGit(dir, "rev-parse", "refs/tags/v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	tagHash := strings.TrimSpace(out)
	if len(tagHash) < 3 {
		t.Fatalf("unexpected tag object hash: %q", tagHash)
	}
	objectPath := filepath.Join(dir, ".git", "objects", tagHash[:2], tagHash[2:])
	if err := os.Remove(objectPath); err != nil {
		t.Fatalf("remove tag object: %v", err)
	}

	svc := GitService{}
	if _, err := svc.VerifyTagSignature(dir, "v1.2.0"); err == nil || !strings.Contains(err.Error(), "读取标签") {
		t.Fatalf("missing tag object must be surfaced, not treated as lightweight: %v", err)
	}
}
