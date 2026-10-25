package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGitService(t *testing.T) {
	if !IsGitRepo(".") {
		t.Skip("Skipping TestGitService: '.' is not a git repository")
	}
	if head, err := ExecGit(".", "rev-parse", "--verify", "HEAD"); err != nil || strings.TrimSpace(head) == "" {
		t.Skip("Skipping TestGitService: project root has no commits")
	}
	svc := GitService{}
	repo, err := svc.OpenRepo(".")
	if err != nil {
		t.Fatalf("OpenRepo failed: %v", err)
	}

	if repo.Name != "wgit" {
		t.Errorf("Expected repo name 'wgit', got '%s'", repo.Name)
	}

	status, err := svc.GetRepoStatus(".")
	if err != nil {
		t.Fatalf("GetRepoStatus failed: %v", err)
	}
	t.Logf("Current branch: %s, Clean: %v", status.CurrentBranch, status.IsClean)

	commits, err := svc.GetCommits(".", 10, "")
	if err != nil {
		t.Fatalf("GetCommits failed: %v", err)
	}
	if len(commits) == 0 {
		t.Fatalf("Expected commits, got 0")
	}
	t.Logf("Found %d commits", len(commits))
	for i, c := range commits {
		t.Logf("Commit #%d: [%s] lane=%d, lines=%d, refs=%v, subject=%s",
			i, c.ShortHash, c.Lane, len(c.Lines), c.Refs, c.Subject)
	}

	branches, err := svc.GetBranches(".")
	if err != nil {
		t.Fatalf("GetBranches failed: %v", err)
	}
	t.Logf("Found %d branches", len(branches))

	tags, err := svc.GetTags(".")
	if err != nil {
		t.Fatalf("GetTags failed: %v", err)
	}
	t.Logf("Found %d tags", len(tags))

	detail, err := svc.GetCommitDetail(".", commits[0].Hash)
	if err != nil {
		t.Fatalf("GetCommitDetail failed: %v", err)
	}
	t.Logf("Commit detail files: %d, +%d -%d", len(detail.Files), detail.Additions, detail.Deletions)

	// Test Git Config
	cfg, err := svc.GetGitConfig(".")
	if err != nil {
		t.Fatalf("GetGitConfig failed: %v", err)
	}
	t.Logf("Git user config: name=%s, email=%s", cfg.UserName, cfg.UserEmail)

	// Test Last Commit Message
	lastMsg, err := svc.GetLastCommitMessage(".")
	if err != nil {
		t.Fatalf("GetLastCommitMessage failed: %v", err)
	}
	t.Logf("Last commit message: %s", lastMsg)

	// Test Large Files Scan
	largeFiles, err := svc.ScanLargeFiles(".", 1)
	if err != nil {
		t.Fatalf("ScanLargeFiles failed: %v", err)
	}
	t.Logf("Found %d files > 1MB", len(largeFiles))

	// Test Git Blame
	blameLines, err := svc.GetFileBlame(".", "git_service.go")
	if err != nil {
		blameLines, err = svc.GetFileBlame("..", "git_service.go")
	}
	if err != nil {
		t.Logf("GetFileBlame skipped (uncommitted subfolder): %v", err)
	} else if len(blameLines) > 0 {
		t.Logf("Git blame returned %d lines; line 1 author: %s", len(blameLines), blameLines[0].Author)
	}

	// Test Repo Overview
	overview, err := svc.GetRepoOverview(".")
	if err != nil {
		t.Fatalf("GetRepoOverview failed: %v", err)
	}
	t.Logf("Repo Overview: commits=%d, branches=%d, tags=%d, contributors=%d",
		overview.TotalCommits, overview.TotalBranches, overview.TotalTags, len(overview.Contributors))
	if overview.TotalCommits == 0 {
		t.Errorf("Expected TotalCommits > 0")
	}

	// Test Diff Between Commits
	if len(commits) >= 2 {
		diffRes, err := svc.GetDiffBetween(".", commits[1].Hash, commits[0].Hash)
		if err != nil {
			t.Fatalf("GetDiffBetween failed: %v", err)
		}
		t.Logf("Diff between %s..%s: files=%d, +%d -%d", commits[1].ShortHash, commits[0].ShortHash, len(diffRes.Files), diffRes.Additions, diffRes.Deletions)
	}

	// Test Activity Logs
	logs := svc.GetActivityLogs()
	if len(logs) == 0 {
		t.Fatalf("Expected activity logs, got 0")
	}
	t.Logf("Activity logs recorded: %d entries (latest: %s in %dms)", len(logs), logs[len(logs)-1].Command, logs[len(logs)-1].DurationMs)

	// Test Git Hooks
	hooks, err := svc.GetGitHooks(".")
	if err != nil {
		t.Fatalf("GetGitHooks failed: %v", err)
	}
	t.Logf("Git hooks inspected: %d standard hooks", len(hooks))

	// Test Clean Working Tree (Dry Run)
	cleanFiles, err := svc.CleanWorkingTree(".", true)
	if err != nil {
		t.Fatalf("CleanWorkingTree dry-run failed: %v", err)
	}
	t.Logf("Clean dry-run candidates: %d files", len(cleanFiles))

	// Test Reflog
	reflogItems, err := svc.GetReflog(".", 10)
	if err != nil {
		t.Fatalf("GetReflog failed: %v", err)
	}
	t.Logf("Reflog: found %d items (latest: %s %s)", len(reflogItems), reflogItems[0].Selector, reflogItems[0].Action)

	// Test Bisect Status
	bisectState, err := svc.BisectStatus(".")
	if err != nil {
		t.Fatalf("BisectStatus failed: %v", err)
	}
	t.Logf("Bisect active: %v", bisectState.IsActive)

	// Test Worktrees
	wts, err := svc.GetWorktrees(".")
	if err != nil {
		t.Fatalf("GetWorktrees failed: %v", err)
	}
	t.Logf("Worktrees found: %d", len(wts))

	// Test LFS Tracked Patterns
	lfsPats, err := svc.GetLfsTrackedPatterns(".")
	if err != nil {
		t.Fatalf("GetLfsTrackedPatterns failed: %v", err)
	}
	t.Logf("LFS tracked patterns: %d", len(lfsPats))

	// Test Maintenance Stats
	mStats, err := svc.GetMaintenanceStats(".")
	if err != nil {
		t.Fatalf("GetMaintenanceStats failed: %v", err)
	}
	t.Logf("Maintenance stats: git dir size=%s, packs=%d, loose=%d", mStats.GitDirSize, mStats.PackFilesCount, mStats.LooseObjects)

	// Test Filtered Commits
	filteredCommits, err := svc.GetCommitsFiltered(".", "HEAD", "", "", "", "optimizations", 0, 10)
	if err != nil {
		t.Fatalf("GetCommitsFiltered failed: %v", err)
	}
	t.Logf("Filtered commits: found %d", len(filteredCommits.Commits))

	// Test Git Version Info
	gitVer, err := svc.GetGitVersion()
	if err != nil {
		t.Fatalf("GetGitVersion failed: %v", err)
	}
	t.Logf("Git version: %s, OS: %s", gitVer.Version, gitVer.System)

	// Test Commit Signature Details
	sigDetails, err := svc.GetCommitSignatureDetails(".", "HEAD")
	if err != nil {
		t.Fatalf("GetCommitSignatureDetails failed: %v", err)
	}
	t.Logf("Signature status: %s, Signer: %s", sigDetails.Status, sigDetails.Signer)

	// Test Storage Breakdown
	storageBreakdown, err := svc.GetRepoStorageBreakdown(".")
	if err != nil {
		t.Fatalf("GetRepoStorageBreakdown failed: %v", err)
	}
	t.Logf("Storage breakdown: %d file groups found (top: %s %s)", len(storageBreakdown), storageBreakdown[0].Extension, storageBreakdown[0].SizeHuman)

	// Test Historical File Content. The repo layout differs between checkouts
	// (some have a root package.json, this one only has frontend/package.json),
	// so probe for a file that actually exists at HEAD instead of hardcoding.
	probeFile := "go.mod"
	for _, candidate := range []string{"package.json", "frontend/package.json", "go.mod", "README.md"} {
		if _, statErr := os.Stat(candidate); statErr == nil {
			probeFile = candidate
			break
		}
	}
	fileContent, err := svc.GetFileContentAtCommit(".", "HEAD", probeFile)
	if err != nil {
		t.Fatalf("GetFileContentAtCommit failed: %v", err)
	}
	if len(fileContent) == 0 {
		t.Fatalf("Expected %s content at HEAD", probeFile)
	}
	t.Logf("File content at HEAD (%s): %d bytes", probeFile, len(fileContent))

	// Test Asset Metadata Inspector
	testAssetFile := probeFile
	assetMeta, err := svc.InspectAssetMetadata(".", testAssetFile)
	if err != nil {
		t.Fatalf("InspectAssetMetadata failed: %v", err)
	}
	t.Logf("Asset metadata for %s: ext=%s, size=%s, mime=%s", testAssetFile, assetMeta.Extension, assetMeta.SizeHuman, assetMeta.MimeType)

	// Test Branch Comparison. Use whichever branches actually exist: the old
	// assertion hardcoded "feature/fast-commit", which only exists in the
	// author's long-lived checkout and made this smoke test fail on any fresh
	// clone of the same project.
	compareBranches, _ := svc.GetBranches(".")
	if len(compareBranches) >= 2 {
		branchCompare, err := svc.CompareBranches(".", compareBranches[0].Name, compareBranches[1].Name)
		if err != nil {
			t.Fatalf("CompareBranches failed: %v", err)
		}
		if branchCompare.AheadCommits == nil || branchCompare.BehindCommits == nil {
			t.Fatalf("CompareBranches must return empty slices instead of nil arrays")
		}
		t.Logf("Branch compare (%s vs %s): ahead=%d, behind=%d", compareBranches[0].Name, compareBranches[1].Name, branchCompare.AheadCount, branchCompare.BehindCount)
	} else {
		t.Logf("CompareBranches skipped: need two branches, found %d", len(compareBranches))
	}

	// Test Tag Detail (only when the repo actually has tags)
	repoTags, _ := svc.GetTags(".")
	if len(repoTags) > 0 {
		tagDetail, err := svc.GetTagDetail(".", repoTags[0].Name)
		if err != nil {
			t.Fatalf("GetTagDetail failed: %v", err)
		}
		t.Logf("Tag %s: hash=%s, isAnnotated=%v, message=%s", repoTags[0].Name, tagDetail.CommitHash, tagDetail.IsAnnotated, tagDetail.Message)
	} else {
		t.Logf("GetTagDetail skipped: repository has no tags")
	}

	// Test Export Commit Report
	mdReport, err := svc.ExportCommitReport(".", "", "", "markdown")
	if err != nil || len(mdReport) == 0 {
		t.Fatalf("ExportCommitReport markdown failed: %v", err)
	}
	t.Logf("Exported markdown report: %d characters", len(mdReport))

	// Test Auth Diagnostics
	authDiag, err := svc.DiagnoseGitAuth(".", "github.com")
	if err != nil {
		t.Fatalf("DiagnoseGitAuth failed: %v", err)
	}
	t.Logf("Auth diagnostic: host=%s, SSH keys found=%v, credential helper=%s", authDiag.Host, authDiag.SSHKeysFound, authDiag.CredentialHelper)

	// Test Sparse Checkout Folders
	sparseFolders, err := svc.GetSparseCheckoutFolders(".")
	if err != nil {
		t.Fatalf("GetSparseCheckoutFolders failed: %v", err)
	}
	t.Logf("Sparse checkout folders: %d active", len(sparseFolders))

	// Test Rounds 81-100: InspectLfsFileHistory
	lfsHist, err := svc.InspectLfsFileHistory(".", probeFile)
	if err != nil {
		t.Fatalf("InspectLfsFileHistory failed: %v", err)
	}
	t.Logf("InspectLfsFileHistory on %s: %d revisions", probeFile, len(lfsHist))

	// Test Rounds 81-100: SearchRepoCode
	matches, err := svc.SearchRepoCode(".", "GitService", false, 10)
	if err != nil {
		t.Fatalf("SearchRepoCode failed: %v", err)
	}
	if len(matches) == 0 {
		matches, err = svc.SearchRepoCode("..", "GitService", false, 10)
	}
	if len(matches) > 0 {
		t.Logf("SearchRepoCode found %d matches (top: %s:%d)", len(matches), matches[0].Path, matches[0].LineNumber)
	} else {
		t.Logf("SearchRepoCode: zero matches in uncommitted workspace")
	}

	// Test Rounds 81-100: GetDetailedGitConfig
	configs, err := svc.GetDetailedGitConfig(".")
	if err != nil {
		t.Fatalf("GetDetailedGitConfig failed: %v", err)
	}
	t.Logf("GetDetailedGitConfig: %d config entries found", len(configs))

	// Test Rounds 81-100: VerifyRepoHealth
	fsckRes, err := svc.VerifyRepoHealth(".")
	if err != nil {
		t.Fatalf("VerifyRepoHealth failed: %v", err)
	}
	t.Logf("VerifyRepoHealth: isClean=%v, danglingBlobs=%d", fsckRes.IsClean, fsckRes.DanglingBlob)

	// Test Rounds 81-100: CheckIgnoreRule
	ignoreRes, err := svc.CheckIgnoreRule(".", "node_modules")
	if err != nil {
		t.Fatalf("CheckIgnoreRule failed: %v", err)
	}
	t.Logf("CheckIgnoreRule (node_modules): isIgnored=%v, source=%s:%d", ignoreRes.IsIgnored, ignoreRes.SourceFile, ignoreRes.LineNumber)

	// Test Rounds 81-100: GetCommitTopologyStats
	topoStats, err := svc.GetCommitTopologyStats(".")
	if err != nil {
		t.Fatalf("GetCommitTopologyStats failed: %v", err)
	}
	t.Logf("Commit topology: total=%d, merges=%d, direct=%d, ratio=%.2f", topoStats.TotalCommits, topoStats.MergeCommits, topoStats.DirectCommits, topoStats.MergeRatio)

	// Test Rounds 81-100: GetImageBase64. Probe for a PNG that is actually in
	// HEAD instead of assuming resources/icon.png exists.
	iconPath := ""
	if out, lsErr := ExecGit(".", "ls-tree", "-r", "--name-only", "HEAD"); lsErr == nil {
		for _, line := range strings.Split(out, "\n") {
			if strings.HasSuffix(strings.ToLower(strings.TrimSpace(line)), ".png") {
				iconPath = strings.TrimSpace(line)
				break
			}
		}
	}
	if iconPath != "" {
		imgBase64, err := svc.GetImageBase64(".", "HEAD", iconPath)
		if err != nil {
			t.Fatalf("GetImageBase64 failed: %v", err)
		}
		if len(imgBase64) < 50 || !strings.HasPrefix(imgBase64, "data:image/png;base64,") {
			t.Fatalf("Invalid image base64 data URI format")
		}
		t.Logf("GetImageBase64 on %s: generated %d chars data URI", iconPath, len(imgBase64))
	} else {
		t.Logf("GetImageBase64 skipped: no PNG tracked at HEAD")
	}

	// Test Unity Meta Sync Check
	metaMismatches, err := svc.CheckUnityMetaSync(".")
	if err != nil {
		t.Fatalf("CheckUnityMetaSync failed: %v", err)
	}
	t.Logf("CheckUnityMetaSync returned %d mismatches", len(metaMismatches))

	// Test Bulk Lock Modified LFS
	bulkLock, err := svc.LockAllModifiedLfs(".")
	if err != nil {
		t.Fatalf("LockAllModifiedLfs failed: %v", err)
	}
	t.Logf("LockAllModifiedLfs result: %s", bulkLock.Message)

	// Test LFS Disk Usage
	lfsUsage, err := svc.GetLfsDiskUsage(".")
	if err != nil {
		t.Fatalf("GetLfsDiskUsage failed: %v", err)
	}
	t.Logf("LFS Disk Usage: %s", lfsUsage)

	// Test LFS Prune (dry run)
	pruneRes, err := svc.LfsPrune(".", true, false)
	if err == nil {
		t.Logf("LFS Prune dry-run: prunedFiles=%d, reclaimed=%s", pruneRes.PrunedFiles, pruneRes.ReclaimedStr)
	} else {
		t.Logf("LFS Prune skipped or expected notice: %v", err)
	}

	// Test Game Engine Detection
	engineInfo, err := svc.DetectGameEngineProject(".")
	if err != nil {
		t.Fatalf("DetectGameEngineProject failed: %v", err)
	}
	t.Logf("Game Engine Project detected: type=%s, hasAttrs=%v", engineInfo.EngineType, engineInfo.HasGitAttributes)

	// Test Game Engine GitAttributes Presets
	unityPreset := svc.GetGameEngineGitAttributesPreset("unity")
	if !strings.Contains(unityPreset, "unityyamlmerge") {
		t.Fatalf("Unity preset missing unityyamlmerge")
	}
	unrealPreset := svc.GetGameEngineGitAttributesPreset("unreal")
	if !strings.Contains(unrealPreset, "*.uasset") {
		t.Fatalf("Unreal preset missing uasset")
	}
	// Test Staged Non-LFS Large Files Check
	nonLfsFiles, err := svc.CheckStagedNonLfsLargeFiles(".", 10)
	if err != nil {
		t.Fatalf("CheckStagedNonLfsLargeFiles failed: %v", err)
	}
	t.Logf("Staged Non-LFS files detected: %d", len(nonLfsFiles))

	// Test Git Proxy Config
	proxyCfg, err := svc.GetGitProxy(".")
	if err != nil {
		t.Fatalf("GetGitProxy failed: %v", err)
	}
	t.Logf("Git Proxy Config: http=%q, https=%q, isGlobal=%v", proxyCfg.HttpProxy, proxyCfg.HttpsProxy, proxyCfg.IsGlobal)

	// Test Merged Branches Scanner
	mergedBranches, err := svc.GetMergedBranches(".", "HEAD")
	if err != nil {
		t.Fatalf("GetMergedBranches failed: %v", err)
	}
	t.Logf("Merged branches eligible for pruning: %d (%v)", len(mergedBranches), mergedBranches)

	// Test GetCommitDetailsBatch
	headHash, err := ExecGit(".", "rev-parse", "HEAD")
	if err == nil && headHash != "" {
		details, err := svc.GetCommitDetailsBatch(".", []string{strings.TrimSpace(headHash)})
		if err != nil {
			t.Fatalf("GetCommitDetailsBatch failed: %v", err)
		}
		if len(details) != 1 {
			t.Fatalf("Expected 1 commit detail, got %d", len(details))
		}
		if details[0].Hash != strings.TrimSpace(headHash) {
			t.Fatalf("Expected hash %s, got %s", headHash, details[0].Hash)
		}
	}

	// Test ScanStagedSecrets in a temporary repo
	tempDir, err := os.MkdirTemp("", "wgit-secrets-test-*")
	if err == nil {
		defer os.RemoveAll(tempDir)
		_, _ = ExecGit(tempDir, "init")
		_, _ = ExecGit(tempDir, "config", "user.name", "TestUser")
		_, _ = ExecGit(tempDir, "config", "user.email", "test@wgit.com")

		// Create a file with simulated secrets
		dummySecretFile := filepath.Join(tempDir, "credentials.py")
		dummyContent := "AWS_KEY = \"AKIAIOSFODNN7EXAMPLE\"\nPRIVATE_KEY = \"-----BEGIN RSA PRIVATE KEY-----\"\nOPENAI_KEY = \"sk-1234567890abcdef1234567890abcdef\"\n"
		_ = os.WriteFile(dummySecretFile, []byte(dummyContent), 0644)
		_ = os.WriteFile(filepath.Join(tempDir, ".env"), []byte("DB_PASSWORD=secret123456\n"), 0644)

		_, _ = ExecGit(tempDir, "add", "credentials.py", ".env")

		findings, err := svc.ScanStagedSecrets(tempDir)
		if err != nil {
			t.Fatalf("ScanStagedSecrets failed: %v", err)
		}
		if len(findings) < 3 {
			t.Fatalf("Expected at least 3 secret findings, got %d", len(findings))
		}

		hasAws := false
		hasEnv := false
		hasPrivKey := false
		for _, f := range findings {
			if strings.Contains(f.RuleName, "AWS") {
				hasAws = true
			}
			if strings.Contains(f.RuleName, "环境") {
				hasEnv = true
			}
			if strings.Contains(f.RuleName, "私有密钥") {
				hasPrivKey = true
			}
		}

		if !hasAws || !hasEnv || !hasPrivKey {
			t.Fatalf("Missing expected finding: aws=%v, env=%v, privKey=%v", hasAws, hasEnv, hasPrivKey)
		}
	}

	// Test ScanHistoryLargeBlobs
	largeBlobs, err := svc.ScanHistoryLargeBlobs(".", 5)
	if err != nil {
		t.Fatalf("ScanHistoryLargeBlobs failed: %v", err)
	}
	t.Logf("History large blobs count: %d", len(largeBlobs))
	if len(largeBlobs) > 0 {
		t.Logf("Top blob: %s (%s, %s)", largeBlobs[0].Path, largeBlobs[0].SizeStr, largeBlobs[0].OID[:7])
	}

	// Test ScanEmptyDirectories and GenerateGitkeeps in temp dir
	tempEmptyRepo, err := os.MkdirTemp("", "wgit-empty-dirs-*")
	if err == nil {
		defer os.RemoveAll(tempEmptyRepo)
		_, _ = ExecGit(tempEmptyRepo, "init")
		emptySub := filepath.Join(tempEmptyRepo, "Assets", "StreamingAssets")
		_ = os.MkdirAll(emptySub, 0755)

		emptyDirs, err := svc.ScanEmptyDirectories(tempEmptyRepo)
		if err != nil {
			t.Fatalf("ScanEmptyDirectories failed: %v", err)
		}
		if len(emptyDirs) != 1 {
			t.Fatalf("Expected 1 empty dir, got %d (%v)", len(emptyDirs), emptyDirs)
		}

		genCount, err := svc.GenerateGitkeeps(tempEmptyRepo, emptyDirs)
		if err != nil {
			t.Fatalf("GenerateGitkeeps failed: %v", err)
		}
		if genCount != 1 {
			t.Fatalf("Expected 1 .gitkeep generated, got %d", genCount)
		}
	}

	// Test ExportLfsLocksReport
	csvReport, err := svc.ExportLfsLocksReport(".", "csv")
	if err != nil {
		t.Fatalf("ExportLfsLocksReport csv failed: %v", err)
	}
	if !strings.Contains(csvReport, "Path,Owner,ID") {
		t.Fatalf("Invalid CSV report header")
	}
	locksMdReport, err := svc.ExportLfsLocksReport(".", "markdown")
	if err != nil {
		t.Fatalf("ExportLfsLocksReport markdown failed: %v", err)
	}
	if !strings.Contains(locksMdReport, "Git LFS 排他锁审计报表") {
		t.Fatalf("Invalid Markdown report header")
	}

	// Test CommitTemplate
	tempTplRepo, err := os.MkdirTemp("", "wgit-commit-tpl-*")
	if err == nil {
		defer os.RemoveAll(tempTplRepo)
		_, _ = ExecGit(tempTplRepo, "init")

		tplInfo, err := svc.GetCommitTemplate(tempTplRepo)
		if err != nil {
			t.Fatalf("GetCommitTemplate failed: %v", err)
		}
		if tplInfo.HasTemplate {
			t.Fatalf("Expected no template initially")
		}

		testTplContent := "[feat]: 游戏玩法更新\n\nTAPD: #12345"
		err = svc.SaveCommitTemplate(tempTplRepo, testTplContent, true)
		if err != nil {
			t.Fatalf("SaveCommitTemplate failed: %v", err)
		}

		savedInfo, err := svc.GetCommitTemplate(tempTplRepo)
		if err != nil || !savedInfo.HasTemplate || savedInfo.Content != testTplContent {
			t.Fatalf("Failed to retrieve saved commit template: %v, info: %+v", err, savedInfo)
		}
	}

	// Test UnityYamlMerge detection and configuration
	tempUnityRepo, err := os.MkdirTemp("", "wgit-unity-merge-*")
	if err == nil {
		defer os.RemoveAll(tempUnityRepo)
		_, _ = ExecGit(tempUnityRepo, "init")

		uStatus, err := svc.DetectUnityYamlMerge(tempUnityRepo)
		if err != nil {
			t.Fatalf("DetectUnityYamlMerge failed: %v", err)
		}
		t.Logf("UnityYamlMerge status: configured=%v, detectedPath=%s", uStatus.IsConfigured, uStatus.DetectedToolPath)

		dummyToolPath := "/Applications/Unity/Hub/Editor/2022.3.20f1/Unity.app/Contents/Tools/UnityYAMLMerge"
		err = svc.ConfigureUnityYamlMerge(tempUnityRepo, dummyToolPath, true)
		if err != nil {
			t.Fatalf("ConfigureUnityYamlMerge failed: %v", err)
		}

		cfgStatus, err := svc.DetectUnityYamlMerge(tempUnityRepo)
		if err != nil || !cfgStatus.IsConfigured || !cfgStatus.HasGitattributes {
			t.Fatalf("ConfigureUnityYamlMerge verification failed: %+v", cfgStatus)
		}
	}

	// Test VerifyPrePushLfs
	prePushRes, err := svc.VerifyPrePushLfs(".", "", "")
	if err != nil {
		t.Fatalf("VerifyPrePushLfs failed: %v", err)
	}
	t.Logf("PrePush verification: isSafe=%v, summary=%s", prePushRes.IsSafe, prePushRes.StatusSummary)

	// Test InspectLfsCacheStats
	lfsCacheStats, err := svc.InspectLfsCacheStats(".")
	if err != nil {
		t.Fatalf("InspectLfsCacheStats failed: %v", err)
	}
	t.Logf("LFS Cache stats: files=%d, totalSize=%s", lfsCacheStats.FileCount, lfsCacheStats.TotalSizeStr)

	// Test LfsFilterConfig (Selective Download)
	tempLfsRepo, err := os.MkdirTemp("", "wgit-lfs-filters-*")
	if err == nil {
		defer os.RemoveAll(tempLfsRepo)
		_, _ = ExecGit(tempLfsRepo, "init")

		fCfg, err := svc.GetLfsFilterConfig(tempLfsRepo)
		if err != nil {
			t.Fatalf("GetLfsFilterConfig failed: %v", err)
		}
		if fCfg.IsConfigured {
			t.Fatalf("Expected empty filter config initially")
		}

		err = svc.SetLfsFilterConfig(tempLfsRepo, []string{"Assets/Characters/**", "Assets/Models/**"}, []string{"Assets/Audio/**", "Assets/Videos/**"})
		if err != nil {
			t.Fatalf("SetLfsFilterConfig failed: %v", err)
		}

		savedCfg, err := svc.GetLfsFilterConfig(tempLfsRepo)
		if err != nil || !savedCfg.IsConfigured || len(savedCfg.FetchInclude) != 2 || len(savedCfg.FetchExclude) != 2 {
			t.Fatalf("Failed to verify saved LFS filters: %+v", savedCfg)
		}
	}

	// Test DetectUnrealProject
	tempUeRepo, err := os.MkdirTemp("", "wgit-ue-test-*")
	if err == nil {
		defer os.RemoveAll(tempUeRepo)
		_, _ = ExecGit(tempUeRepo, "init")
		uprojectFile := filepath.Join(tempUeRepo, "MyGameProject.uproject")
		_ = os.WriteFile(uprojectFile, []byte(`{"FileVersion": 3, "EngineAssociation": "5.3"}`), 0644)
		_ = os.MkdirAll(filepath.Join(tempUeRepo, "Intermediate"), 0755)
		_ = os.WriteFile(filepath.Join(tempUeRepo, "Intermediate", "test.txt"), []byte("cachedata"), 0644)

		ueInfo, err := svc.DetectUnrealProject(tempUeRepo)
		if err != nil {
			t.Fatalf("DetectUnrealProject failed: %v", err)
		}
		if !ueInfo.IsUnrealProject || ueInfo.ProjectName != "MyGameProject" || ueInfo.EngineVersion != "5.3" {
			t.Fatalf("DetectUnrealProject returned incorrect info: %+v", ueInfo)
		}

		cleanRes, err := svc.CleanUnrealEngineCache(tempUeRepo)
		if err != nil || len(cleanRes.CleanedFolders) != 1 {
			t.Fatalf("CleanUnrealEngineCache failed: %v, res: %+v", err, cleanRes)
		}
	}

	// Test SyncLockableFilePermissions & ToggleLockableReadOnlyProtection
	tempLockableRepo, err := os.MkdirTemp("", "wgit-lockable-*")
	if err == nil {
		defer os.RemoveAll(tempLockableRepo)
		_, _ = ExecGit(tempLockableRepo, "init")
		_ = os.WriteFile(filepath.Join(tempLockableRepo, ".gitattributes"), []byte("*.uasset lockable\n*.fbx lockable\n"), 0644)
		modelFile := filepath.Join(tempLockableRepo, "Hero.fbx")
		_ = os.WriteFile(modelFile, []byte("fbxbinarydata"), 0644)

		syncRes, err := svc.ToggleLockableReadOnlyProtection(tempLockableRepo, true)
		if err != nil {
			t.Fatalf("ToggleLockableReadOnlyProtection failed: %v", err)
		}
		if syncRes.TotalLockableFiles != 1 || syncRes.ReadOnlyCount != 1 {
			t.Fatalf("Expected 1 read-only protected lockable file, got: %+v", syncRes)
		}

		fi, _ := os.Stat(modelFile)
		if fi.Mode().Perm()&0200 != 0 {
			t.Fatalf("Expected file to be read-only, got mode: %v", fi.Mode())
		}

		// Disable protection
		syncOffRes, err := svc.ToggleLockableReadOnlyProtection(tempLockableRepo, false)
		if err != nil || syncOffRes.WritableCount != 1 {
			t.Fatalf("Failed to disable read-only protection: %+v", syncOffRes)
		}
	}

	// Test GenerateStudioChangelog. "HEAD~5" only resolves on a repo with at
	// least six commits; clamp the range so a fresh clone still exercises it.
	changelogBase := "HEAD~1"
	if out, countErr := ExecGit(".", "rev-list", "--count", "HEAD"); countErr == nil {
		if n, parseErr := strconv.Atoi(strings.TrimSpace(out)); parseErr == nil && n >= 6 {
			changelogBase = "HEAD~5"
		}
	}
	changelog, err := svc.GenerateStudioChangelog(".", changelogBase, "HEAD")
	if err != nil {
		t.Fatalf("GenerateStudioChangelog failed: %v", err)
	}
	if changelog.TotalCount == 0 || !strings.Contains(changelog.MarkdownDoc, "发版变更与里程碑日志") {
		t.Fatalf("GenerateStudioChangelog invalid result: %+v", changelog)
	}
	t.Logf("Changelog generated successfully: %d commits across %d categories", changelog.TotalCount, len(changelog.Categories))

	// Test GetCustomActions & ExecuteCustomAction
	actions, err := svc.GetCustomActions(".")
	if err != nil || len(actions) < 4 {
		t.Fatalf("GetCustomActions failed: %v, len=%d", err, len(actions))
	}
	t.Logf("Default custom actions count: %d (first: %s)", len(actions), actions[0].Name)

	// Run in a clean isolated repo so concurrent uncommitted edits in the
	// shared workspace cannot make diff --check fail this execution test.
	actionRepo := t.TempDir()
	runGitTest(t, actionRepo, nil, "init", "-b", "main")
	runGitTest(t, actionRepo, nil, "config", "user.name", "Alice")
	runGitTest(t, actionRepo, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, actionRepo, "clean.txt", "clean\n", "chore: init", time.Now(), "Alice")
	actRes, err := svc.ExecuteCustomAction(actionRepo, "act_diff_check")
	if err != nil || !actRes.Success {
		t.Fatalf("ExecuteCustomAction act_diff_check failed: %v, res=%+v", err, actRes)
	}
	t.Logf("Executed action act_diff_check in %dms (exit code %d)", actRes.DurationMs, actRes.ExitCode)

	// Test CustomActions persistence
	tempActionsRepo := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempActionsRepo, ".git"), 0755)
	customList := []CustomActionConfig{
		{
			ID:            "act_custom_build",
			Name:          "Studio Custom Build",
			Command:       "echo",
			Args:          "'Build Complete'",
			ShowInToolbar: true,
		},
	}
	if err := svc.SaveCustomActions(tempActionsRepo, customList); err != nil {
		t.Fatalf("SaveCustomActions failed: %v", err)
	}
	savedActions, err := svc.GetCustomActions(tempActionsRepo)
	if err != nil || len(savedActions) != 1 || savedActions[0].ID != "act_custom_build" {
		t.Fatalf("GetCustomActions after save mismatch: %+v", savedActions)
	}
	runEcho, err := svc.ExecuteCustomAction(tempActionsRepo, "act_custom_build")
	if err != nil || !runEcho.Success || !strings.Contains(runEcho.Output, "Build Complete") {
		t.Fatalf("ExecuteCustomAction on custom action failed: %v, %+v", err, runEcho)
	}

	// Test DetectGameEngineWorkflows & CleanGameEngineCaches (Godot & Cocos)
	tempEngineRepo := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempEngineRepo, ".git"), 0755)
	// Mock Godot
	_ = os.WriteFile(filepath.Join(tempEngineRepo, "project.godot"), []byte("[application]\nconfig/name=\"GodotSpaceGame\"\nconfig/features=PackedStringArray(\"4.2\", \"Forward+\")\n"), 0644)
	godotImportDir := filepath.Join(tempEngineRepo, ".godot", "imported")
	_ = os.MkdirAll(godotImportDir, 0755)
	_ = os.WriteFile(filepath.Join(godotImportDir, "texture.ctex"), []byte("dummytexturedata"), 0644)

	workflows, err := svc.DetectGameEngineWorkflows(tempEngineRepo)
	if err != nil || len(workflows) == 0 {
		t.Fatalf("DetectGameEngineWorkflows failed: %v, count=%d", err, len(workflows))
	}
	if workflows[0].EngineType != "godot" || workflows[0].ProjectName != "GodotSpaceGame" {
		t.Fatalf("Unexpected workflow: %+v", workflows[0])
	}
	t.Logf("Detected Godot project: %s, cache size: %s", workflows[0].ProjectName, workflows[0].CacheSizeStr)

	cleanRes, err := svc.CleanGameEngineCaches(tempEngineRepo, "godot")
	if err != nil || cleanRes.FreedSizeBytes == 0 {
		t.Fatalf("CleanGameEngineCaches godot failed: %v, res=%+v", err, cleanRes)
	}
	t.Logf("Cleaned Godot caches: %s freed", cleanRes.FreedSizeStr)

	// Mock Cocos Creator
	tempCocosRepo := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempCocosRepo, ".git"), 0755)
	cocosJson := `{"name": "CocosMiniGame", "creator": {"version": "3.8.2"}}`
	_ = os.WriteFile(filepath.Join(tempCocosRepo, "project.json"), []byte(cocosJson), 0644)
	cocosTempDir := filepath.Join(tempCocosRepo, "temp")
	_ = os.MkdirAll(cocosTempDir, 0755)
	_ = os.WriteFile(filepath.Join(cocosTempDir, "cache.bin"), []byte("cocostempcachebytes"), 0644)

	cocosWorkflows, err := svc.DetectGameEngineWorkflows(tempCocosRepo)
	if err != nil || len(cocosWorkflows) == 0 || cocosWorkflows[0].EngineType != "cocos" {
		t.Fatalf("DetectGameEngineWorkflows cocos failed: %v, workflows=%+v", err, cocosWorkflows)
	}
	cocosClean, err := svc.CleanGameEngineCaches(tempCocosRepo, "cocos")
	if err != nil || cocosClean.FreedSizeBytes == 0 {
		t.Fatalf("CleanGameEngineCaches cocos failed: %v, res=%+v", err, cocosClean)
	}

	// Test GetRepositoryDiskBreakdown
	diskBreakdown, err := svc.GetRepositoryDiskBreakdown(".")
	if err != nil {
		t.Fatalf("GetRepositoryDiskBreakdown failed: %v", err)
	}
	if diskBreakdown.TotalSize == 0 || diskBreakdown.WorkingTreeSize == 0 || len(diskBreakdown.TopFolders) == 0 {
		t.Fatalf("GetRepositoryDiskBreakdown invalid: %+v", diskBreakdown)
	}
	t.Logf("Disk breakdown: Total=%s, WorkingTree=%s, GitInternal=%s, TopFolders=%d",
		diskBreakdown.TotalSizeStr, diskBreakdown.WorkingTreeStr, diskBreakdown.GitInternalStr, len(diskBreakdown.TopFolders))

	// Test GetMultiRemoteStatus
	multiRemote, err := svc.GetMultiRemoteStatus(".")
	if err != nil {
		t.Fatalf("GetMultiRemoteStatus failed: %v", err)
	}
	t.Logf("Multi-remote status on branch %s: remotes count=%d", multiRemote.CurrentBranch, len(multiRemote.Remotes))

	// Test LintGameTextures
	lintRes, err := svc.LintGameTextures(".", true)
	if err != nil {
		t.Fatalf("LintGameTextures failed: %v", err)
	}
	if lintRes.TotalChecked == 0 {
		t.Fatalf("Expected at least 1 image checked, got 0")
	}
	t.Logf("Texture Linter: checked %d images, found %d issues (first: %s %s POT=%v)",
		lintRes.TotalChecked, lintRes.TotalIssues, lintRes.Items[0].Path, lintRes.Items[0].Dimensions, lintRes.Items[0].IsPOT)

	// Test ListSubtrees
	subtrees, err := svc.ListSubtrees(".")
	if err != nil {
		t.Fatalf("ListSubtrees failed: %v", err)
	}
	t.Logf("Found %d subtrees configured", len(subtrees.Subtrees))

	// Test GetLfsTuningConfig & ApplyLfsTuningOptimization
	tempTuningRepo := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempTuningRepo, ".git"), 0755)
	_, _ = ExecGit(tempTuningRepo, "init")

	tuneRes, err := svc.ApplyLfsTuningOptimization(tempTuningRepo, "studio_high_speed")
	if err != nil || !tuneRes.Success || !tuneRes.Config.IsOptimized || tuneRes.Config.ConcurrentTransfers != 16 {
		t.Fatalf("ApplyLfsTuningOptimization studio_high_speed failed: %v, res=%+v", err, tuneRes)
	}
	t.Logf("LFS tuning applied: transfers=%d, buffer=%dMB, isOptimized=%v",
		tuneRes.Config.ConcurrentTransfers, tuneRes.Config.PostBufferMB, tuneRes.Config.IsOptimized)

	resetTune, err := svc.ApplyLfsTuningOptimization(tempTuningRepo, "reset_default")
	if err != nil || resetTune.Config.ConcurrentTransfers != 3 {
		t.Fatalf("ApplyLfsTuningOptimization reset_default failed: %v, res=%+v", err, resetTune)
	}

	// Modal 72: Test InspectAssetReferences
	tempAssetRepo := t.TempDir()
	_, _ = ExecGit(tempAssetRepo, "init")
	_ = os.MkdirAll(filepath.Join(tempAssetRepo, "Assets", "Textures"), 0755)
	_ = os.MkdirAll(filepath.Join(tempAssetRepo, "Assets", "Prefabs"), 0755)
	heroPng := filepath.Join(tempAssetRepo, "Assets", "Textures", "Hero.png")
	heroMeta := heroPng + ".meta"
	_ = os.WriteFile(heroPng, []byte("pngdata"), 0644)
	_ = os.WriteFile(heroMeta, []byte("fileFormatVersion: 2\nguid: aabbccddeeff11223344556677889900\n"), 0644)

	prefabFile := filepath.Join(tempAssetRepo, "Assets", "Prefabs", "Warrior.prefab")
	_ = os.WriteFile(prefabFile, []byte("--- !u!1 &123\nSpriteRenderer:\n  m_Sprite: {fileID: 21300000, guid: aabbccddeeff11223344556677889900, type: 3}\n"), 0644)

	refRes, err := svc.InspectAssetReferences(tempAssetRepo, "Assets/Textures/Hero.png")
	if err != nil || refRes.ReferenceCount != 1 || refRes.SafeToDelete {
		t.Fatalf("InspectAssetReferences failed: %v, res=%+v", err, refRes)
	}
	t.Logf("Asset reference inspector: found %d refs in %s (SafeToDelete: %v, GUID: %s)",
		refRes.ReferenceCount, refRes.References[0].FilePath, refRes.SafeToDelete, refRes.AssetGuid)

	// Modal 73: Test GetCommitSigningStatus & ConfigureCommitSigning
	signStatus, err := svc.GetCommitSigningStatus(tempAssetRepo)
	if err != nil {
		t.Fatalf("GetCommitSigningStatus failed: %v", err)
	}
	t.Logf("Commit signing status: enabled=%v, format=%s, compliance=%s",
		signStatus.GpgSignEnabled, signStatus.GpgFormat, signStatus.ComplianceRate)

	signCfgRes, err := svc.ConfigureCommitSigning(tempAssetRepo, true, "~/.ssh/id_rsa.pub", "ssh")
	if err != nil || !signCfgRes.Success {
		t.Fatalf("ConfigureCommitSigning failed: %v", err)
	}

	// Modal 74: Test GetLfsLockActivity
	lockAct, err := svc.GetLfsLockActivity(tempAssetRepo)
	if err != nil {
		t.Fatalf("GetLfsLockActivity failed: %v", err)
	}
	t.Logf("LFS lock activity: total=%d, my=%d, team=%d", lockAct.TotalLocks, lockAct.MyLockCount, lockAct.TeamLockCount)

	// Modal 75: Test AuditIgnoreRules & ApplyIgnoreSanitizer
	auditRes, err := svc.AuditIgnoreRules(tempAssetRepo)
	if err != nil || !auditRes.HasGitIgnore {
		// No .gitignore initially
	}
	t.Logf("Ignore rule audit: engine=%s, missing rules=%d, health=%d/100",
		auditRes.EngineType, len(auditRes.MissingRules), auditRes.HealthScore)

	// Add dangerous tracked artifact
	vsDir := filepath.Join(tempAssetRepo, ".vs")
	_ = os.MkdirAll(vsDir, 0755)
	vsFile := filepath.Join(vsDir, "config.json")
	_ = os.WriteFile(vsFile, []byte("{}"), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", ".vs/config.json")

	auditRes2, _ := svc.AuditIgnoreRules(tempAssetRepo)
	if auditRes2.TrackedCount != 1 {
		t.Fatalf("Expected 1 tracked artifact, got %d", auditRes2.TrackedCount)
	}

	sanRes, err := svc.ApplyIgnoreSanitizer(tempAssetRepo, "Unity 3D", true)
	if err != nil || !sanRes.Success || len(sanRes.RulesAdded) == 0 || len(sanRes.UntrackedFiles) != 1 {
		t.Fatalf("ApplyIgnoreSanitizer failed: %v, res=%+v", err, sanRes)
	}
	t.Logf("Applied ignore sanitizer: added %d rules, untracked %d files",
		len(sanRes.RulesAdded), len(sanRes.UntrackedFiles))

	// Modal 76: Test InspectAudioFile
	audioDir := filepath.Join(tempAssetRepo, "Assets", "Audio")
	_ = os.MkdirAll(audioDir, 0755)
	wavPath := filepath.Join(audioDir, "Laser.wav")
	// Construct minimal 44-byte RIFF/WAVE header: 48000Hz, 2ch, 16bit
	wavHeader := []byte{
		'R', 'I', 'F', 'F',
		36, 0, 0, 0, // 36 + data size (0)
		'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ',
		16, 0, 0, 0, // Subchunk1Size (16 for PCM)
		1, 0, // AudioFormat (1=PCM)
		2, 0, // NumChannels (2)
		0x80, 0xbb, 0, 0, // SampleRate = 48000 (0x0000bb80)
		0x00, 0xee, 0x02, 0, // ByteRate = 48000 * 2 * 2 = 192000 (0x0002ee00)
		4, 0, // BlockAlign (4)
		16, 0, // BitsPerSample (16)
		'd', 'a', 't', 'a',
		0, 0, 0, 0, // DataSize = 0
	}
	_ = os.WriteFile(wavPath, wavHeader, 0644)
	audioRes, err := svc.InspectAudioFile(tempAssetRepo, "Assets/Audio/Laser.wav")
	if err != nil || audioRes.SampleRate != 48000 || !audioRes.IsStandard48k || audioRes.Channels != 2 {
		t.Fatalf("InspectAudioFile failed: %v, res=%+v", err, audioRes)
	}
	t.Logf("Audio inspector: format=%s, sampleRate=%d, channels=%s, std48k=%v",
		audioRes.Format, audioRes.SampleRate, audioRes.ChannelStr, audioRes.IsStandard48k)

	// Modal 77: Test LintModelMesh
	modelsDir := filepath.Join(tempAssetRepo, "Assets", "Models")
	_ = os.MkdirAll(modelsDir, 0755)
	objPath := filepath.Join(modelsDir, "Sword.obj")
	objContent := "# Sword Model\nv 0.0 0.0 0.0\nv 1.0 0.0 0.0\nv 0.0 1.0 0.0\nv 1.0 1.0 0.0\nf 1 2 3\nf 2 4 3\n"
	_ = os.WriteFile(objPath, []byte(objContent), 0644)
	meshRes, err := svc.LintModelMesh(tempAssetRepo, "Assets/Models/Sword.obj", "mobile_prop")
	if err != nil || meshRes.TriangleCount != 2 || meshRes.VertexCount != 4 || meshRes.IsOverBudget {
		t.Fatalf("LintModelMesh failed: %v, res=%+v", err, meshRes)
	}
	t.Logf("Mesh linter: triangles=%d, vertices=%d, budget=%s, severity=%s",
		meshRes.TriangleCount, meshRes.VertexCount, meshRes.BudgetPreset, meshRes.Severity)

	// Modal 78: CI monitor must not fabricate runs for a repo without CI config
	// (the old assertion expected five fake stages, which locked in the
	// placeholder behaviour the audit later removed).
	ciRes, err := svc.GetStudioCIPipelines(tempAssetRepo)
	if err != nil || ciRes.LatestRun != nil || ciRes.Provider != "未检测到 CI 配置" {
		t.Fatalf("GetStudioCIPipelines should report no CI config: %v, res=%+v", err, ciRes)
	}
	if _, err := svc.TriggerStudioCIPipeline(tempAssetRepo, "Nightly-Cook"); err == nil {
		t.Fatal("TriggerStudioCIPipeline must fail without a GitHub Actions remote")
	}
	t.Logf("Studio CI monitor: provider=%s, message=%s", ciRes.Provider, ciRes.Message)

	// Modal 79: Test GetFileRevisionTimeline & RevertFileToRevision
	testDoc := filepath.Join(tempAssetRepo, "README.md")
	_ = os.WriteFile(testDoc, []byte("# Version 1"), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "README.md")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "docs: version 1")
	firstHash, _ := ExecGit(tempAssetRepo, "rev-parse", "HEAD")
	firstHash = strings.TrimSpace(firstHash)

	_ = os.WriteFile(testDoc, []byte("# Version 2 with changes"), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "README.md")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "docs: version 2")

	timelineRes, err := svc.GetFileRevisionTimeline(tempAssetRepo, "README.md")
	if err != nil || len(timelineRes.Revisions) < 2 {
		t.Fatalf("GetFileRevisionTimeline failed: %v, res=%+v", err, timelineRes)
	}
	t.Logf("File revision timeline: %d revisions found for %s", len(timelineRes.Revisions), timelineRes.FilePath)

	revertRes, err := svc.RevertFileToRevision(tempAssetRepo, "README.md", firstHash)
	if err != nil || !revertRes.Success {
		t.Fatalf("RevertFileToRevision failed: %v, res=%+v", err, revertRes)
	}
	revertedBytes, _ := os.ReadFile(testDoc)
	if string(revertedBytes) != "# Version 1" {
		t.Fatalf("Expected file to be reverted to '# Version 1', got '%s'", string(revertedBytes))
	}
	t.Logf("Single file reverted successfully to %s", revertRes.CommitHash)

	// ==========================================
	// Round 101: Test InspectShaderVariants
	// ==========================================
	shadersDir := filepath.Join(tempAssetRepo, "Assets", "Shaders")
	_ = os.MkdirAll(shadersDir, 0755)

	pbrShaderPath := filepath.Join(shadersDir, "CustomPBR.shader")
	pbrContent := `Shader "Custom/PBRStandard" {
		Properties {
			_Color ("Color", Color) = (1,1,1,1)
		}
		SubShader {
			Pass {
				Name "ForwardBase"
				#pragma multi_compile _ DIRECTIONAL POINT SPOT
				#pragma multi_compile _ SHADOWS_SOFT
				#pragma shader_feature _ NORMALMAP
			}
		}
	}`
	_ = os.WriteFile(pbrShaderPath, []byte(pbrContent), 0644)

	uberShaderPath := filepath.Join(shadersDir, "UberExplosion.shader")
	var uberLines []string
	uberLines = append(uberLines, "Shader \"Custom/UberExplosion\" { SubShader { Pass { Name \"UberPass\"")
	for i := 1; i <= 11; i++ {
		uberLines = append(uberLines, fmt.Sprintf("#pragma multi_compile _ FEATURE_SWITCH_%d", i))
	}
	uberLines = append(uberLines, "} } }")
	_ = os.WriteFile(uberShaderPath, []byte(strings.Join(uberLines, "\n")), 0644)

	pbrRes, err := svc.InspectShaderVariants(tempAssetRepo, "Assets/Shaders/CustomPBR.shader")
	if err != nil || pbrRes.TotalVariants != 16 || pbrRes.IsExplosion || pbrRes.Severity != "OK" {
		t.Fatalf("InspectShaderVariants (PBR) failed: %v, res=%+v", err, pbrRes)
	}
	t.Logf("Shader inspector (PBR): variants=%d, pragmas=%d, severity=%s",
		pbrRes.TotalVariants, len(pbrRes.Pragmas), pbrRes.Severity)

	uberRes, err := svc.InspectShaderVariants(tempAssetRepo, "Assets/Shaders/UberExplosion.shader")
	if err != nil || uberRes.TotalVariants != 2048 || !uberRes.IsExplosion || uberRes.Severity != "CRITICAL" {
		t.Fatalf("InspectShaderVariants (UberExplosion) failed: %v, res=%+v", err, uberRes)
	}
	t.Logf("Shader inspector (Explosion): variants=%d, isExplosion=%v, severity=%s, adviceCount=%d",
		uberRes.TotalVariants, uberRes.IsExplosion, uberRes.Severity, len(uberRes.StrippingAdvice))

	// ==========================================
	// Round 102: Test DetectDCCLockActivity & AutoLockDCCAsset
	// ==========================================
	dccCharDir := filepath.Join(tempAssetRepo, "Assets", "Characters")
	dccPropDir := filepath.Join(tempAssetRepo, "Assets", "Props")
	_ = os.MkdirAll(dccCharDir, 0755)
	_ = os.MkdirAll(dccPropDir, 0755)

	mayaModel := filepath.Join(dccCharDir, "Warrior.mb")
	mayaLock := filepath.Join(dccCharDir, "Warrior.mb.swp")
	_ = os.WriteFile(mayaModel, []byte("MAYA BINARY DUMMY DATA"), 0644)
	_ = os.WriteFile(mayaLock, []byte("MAYA SWAP LOCK"), 0644)

	blenderModel := filepath.Join(dccPropDir, "Chest.blend")
	blenderLock := filepath.Join(dccPropDir, "Chest.blend@")
	_ = os.WriteFile(blenderModel, []byte("BLENDER DUMMY DATA"), 0644)
	_ = os.WriteFile(blenderLock, []byte("BLENDER AUTO LOCK"), 0644)

	dccRes, err := svc.DetectDCCLockActivity(tempAssetRepo)
	if err != nil || dccRes.TotalDCCFiles < 2 {
		t.Fatalf("DetectDCCLockActivity failed: %v, res=%+v", err, dccRes)
	}
	t.Logf("DCC Lock Watcher: totalDCCFiles=%d, activeLocks=%d, summary=%s",
		dccRes.TotalDCCFiles, dccRes.ActiveLocksCount, dccRes.Summary)

	lockRes, err := svc.AutoLockDCCAsset(tempAssetRepo, "Assets/Characters/Warrior.mb")
	if err != nil {
		t.Fatalf("AutoLockDCCAsset encountered fatal err: %v", err)
	}
	t.Logf("Auto-lock result: success=%v, msg=%s", lockRes.Success, lockRes.Message)

	// ==========================================
	// Round 103: Test DiffUnrealAssetNodes
	// ==========================================
	bpDir := filepath.Join(tempAssetRepo, "Content", "Blueprints")
	_ = os.MkdirAll(bpDir, 0755)
	bpFile := filepath.Join(bpDir, "BP_Player.uasset")

	bpV1Content := `Begin Object Class=/Script/BlueprintGraph.K2Node_Event Name="ReceiveBeginPlay"
   NodePosX=100
   NodePosY=200
   Pin (PinName="Output",Direction="Output",PinType.PinCategory="Exec",LinkedTo=(CallFunc_1:Input))
End Object
Begin Object Class=/Script/BlueprintGraph.K2Node_CallFunction Name="CallFunc_1"
   NodePosX=300
   NodePosY=200
   FunctionReference=(MemberName="PrintString")
   Pin (PinName="Input",Direction="Input",PinType.PinCategory="Exec")
   Pin (PinName="InString",Direction="Input",PinType.PinCategory="String",DefaultValue="Hello World")
End Object`
	_ = os.WriteFile(bpFile, []byte(bpV1Content), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "Content/Blueprints/BP_Player.uasset")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: initial player blueprint")

	bpV2Content := `Begin Object Class=/Script/BlueprintGraph.K2Node_Event Name="ReceiveBeginPlay"
   NodePosX=100
   NodePosY=200
   Pin (PinName="Output",Direction="Output",PinType.PinCategory="Exec",LinkedTo=(CallFunc_1:Input))
End Object
Begin Object Class=/Script/BlueprintGraph.K2Node_CallFunction Name="CallFunc_1"
   NodePosX=300
   NodePosY=200
   FunctionReference=(MemberName="PrintString")
   Pin (PinName="Input",Direction="Input",PinType.PinCategory="Exec")
   Pin (PinName="InString",Direction="Input",PinType.PinCategory="String",DefaultValue="Updated Greeting")
End Object
Begin Object Class=/Script/BlueprintGraph.K2Node_CallFunction Name="CallFunc_ApplyDamage"
   NodePosX=500
   NodePosY=200
   FunctionReference=(MemberName="ApplyDamage")
   Pin (PinName="Input",Direction="Input",PinType.PinCategory="Exec")
End Object`
	_ = os.WriteFile(bpFile, []byte(bpV2Content), 0644)

	diffRes, err := svc.DiffUnrealAssetNodes(tempAssetRepo, "Content/Blueprints/BP_Player.uasset", "HEAD", "WORKTREE")
	if err != nil || len(diffRes.AddedNodes) != 1 || len(diffRes.ModifiedNodes) != 1 {
		t.Fatalf("DiffUnrealAssetNodes failed: %v, res=%+v", err, diffRes)
	}
	t.Logf("Blueprint AST Diff: added=%d, modified=%d, summary=%s",
		len(diffRes.AddedNodes), len(diffRes.ModifiedNodes), diffRes.VisualSummary)

	// ==========================================
	// Round 104: Test AuditSceneStreamMerge & ResolveSceneStreamConflict
	// ==========================================
	scenesDir := filepath.Join(tempAssetRepo, "Content", "Maps")
	_ = os.MkdirAll(scenesDir, 0755)
	levelFile := filepath.Join(scenesDir, "SubLevel_Arena.umap")

	levelContent := `Begin Actor Class=StaticMeshActor Name="Statue_Boss" ActorGuid="guid-boss-1001" Location=(X=0.0, Y=0.0, Z=0.0)
End Actor
Begin Actor Class=PointLight Name="Torch_1" ActorGuid="guid-torch-2002" Location=(X=10.0, Y=10.0, Z=5.0)
End Actor`
	_ = os.WriteFile(levelFile, []byte(levelContent), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "Content/Maps/SubLevel_Arena.umap")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: arena sublevel")

	sceneAuditRes, err := svc.AuditSceneStreamMerge(tempAssetRepo, "HEAD", "Content/Maps/SubLevel_Arena.umap")
	if err != nil || len(sceneAuditRes.AuditedSubLevels) == 0 {
		t.Fatalf("AuditSceneStreamMerge failed: %v, res=%+v", err, sceneAuditRes)
	}
	t.Logf("Scene stream audit: engine=%s, actors=%d, safetyScore=%d, verdict=%s",
		sceneAuditRes.EngineType, sceneAuditRes.TotalActorsCurrent, sceneAuditRes.SafetyScore, sceneAuditRes.MergeVerdict)

	resolveRes, err := svc.ResolveSceneStreamConflict(tempAssetRepo, "Content/Maps/SubLevel_Arena.umap", "AUTO_REGENERATE_GUID", "guid-boss-1001")
	if err != nil || !resolveRes.Success || resolveRes.NewActorGuid == "" {
		t.Fatalf("ResolveSceneStreamConflict failed: %v, res=%+v", err, resolveRes)
	}
	t.Logf("Scene conflict resolved: newGuid=%s, msg=%s", resolveRes.NewActorGuid, resolveRes.Message)

	// ==========================================
	// Round 105: Test LintAnimationRig
	// ==========================================
	fbxDir := filepath.Join(tempAssetRepo, "Assets", "Animations")
	_ = os.MkdirAll(fbxDir, 0755)
	heroFbx := filepath.Join(fbxDir, "Hero_Run.fbx")
	fbxHeader := []byte("Kaydara FBX Binary  \x00\x1a\x00\x18\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00Model::RootNode\x00Model::Pelvis\x00Model::Spine\x00Model::Thigh_L\x00Model::Thigh_R\x00AnimationCurveNode\x00AnimCurve\x00AnimCurve\x00AnimCurve\x00")
	_ = os.WriteFile(heroFbx, fbxHeader, 0644)

	rigRes, err := svc.LintAnimationRig(tempAssetRepo, "Assets/Animations/Hero_Run.fbx", "MOBILE")
	if err != nil || rigRes.BoneCount == 0 {
		t.Fatalf("LintAnimationRig failed: %v, res=%+v", err, rigRes)
	}
	t.Logf("Animation Rig Linter: bones=%d, tracks=%d, duration=%.1fs, rating=%s, budgetPass=%v",
		rigRes.BoneCount, rigRes.TotalTrackCount, rigRes.TotalDurationSeconds, rigRes.PerformanceRating, !rigRes.IsOverBudget)

	// ==========================================
	// Round 106: Test AuditLocalizationParity
	// ==========================================
	locDir := filepath.Join(tempAssetRepo, "Localization")
	_ = os.MkdirAll(locDir, 0755)
	zhFile := filepath.Join(locDir, "zh-CN.json")
	enFile := filepath.Join(locDir, "en-US.json")
	_ = os.WriteFile(zhFile, []byte(`{"ui.start": "开始游戏", "ui.quit": "退出游戏", "ui.hero": "英雄"}`), 0644)
	_ = os.WriteFile(enFile, []byte(`{"ui.start": "Start Game", "ui.quit": "Quit Game"}`), 0644)

	locRes, err := svc.AuditLocalizationParity(tempAssetRepo, "")
	if err != nil || locRes.TotalUniqueKeys == 0 {
		t.Fatalf("AuditLocalizationParity failed: %v, res=%+v", err, locRes)
	}
	t.Logf("Localization audit: totalKeys=%d, languages=%d, score=%.1f%%, missing=%d",
		locRes.TotalUniqueKeys, len(locRes.LanguagesFound), locRes.OverallParityScore, len(locRes.UntranslatedList))

	// ==========================================
	// Round 107: Test AuditNativePluginsAbi
	// ==========================================
	pluginDir := filepath.Join(tempAssetRepo, "Plugins", "Android", "libs", "arm64-v8a")
	_ = os.MkdirAll(pluginDir, 0755)
	dummySo := filepath.Join(pluginDir, "libgamecore.so")
	elfHeader := []byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x03\x00\xb7\x00\x01\x00\x00\x00")
	_ = os.WriteFile(dummySo, elfHeader, 0755)

	abiRes, err := svc.AuditNativePluginsAbi(tempAssetRepo)
	if err != nil || abiRes.TotalPluginsCount == 0 {
		t.Fatalf("AuditNativePluginsAbi failed: %v, res=%+v", err, abiRes)
	}
	t.Logf("Native Plugins ABI: plugins=%d, compliantCount=%d, criticalMismatch=%v",
		abiRes.TotalPluginsCount, abiRes.CompliantCount, abiRes.HasCriticalMismatch)

	// ==========================================
	// Round 108: Test FetchTAPDWorkItems & BindCommitToTAPD
	// ==========================================
	tapdRes, err := svc.FetchTAPDWorkItems(tempAssetRepo)
	if err != nil || tapdRes.TotalCount != 0 || tapdRes.IsConnected || len(tapdRes.Items) != 0 {
		t.Fatalf("TAPD should not fabricate work items: %v, res=%+v", err, tapdRes)
	}
	t.Logf("TAPD work items: total=%d, message=%s", tapdRes.TotalCount, tapdRes.Message)

	tapdRefRes := svc.ParseTAPDCommitReferences("feat: combat system overhaul [TAPD-1002341] bug:1002342")
	if len(tapdRefRes.FoundTickets) != 2 {
		t.Fatalf("ParseTAPDCommitReferences failed: %+v", tapdRefRes)
	}

	bindRes, err := svc.BindCommitToTAPD(tempAssetRepo, firstHash, "1002341", "resolved")
	if err != nil || !bindRes.Success || !strings.Contains(bindRes.Message, "本地 Git Note") {
		t.Fatalf("BindCommitToTAPD failed: %v, res=%+v", err, bindRes)
	}
	note, err := ExecGit(tempAssetRepo, "notes", "show", firstHash)
	if err != nil || !strings.Contains(note, "TAPD #1002341") {
		t.Fatalf("expected local Git Note binding, note=%q err=%v", note, err)
	}
	t.Logf("TAPD commit bound: commit=%s, tapd=%s, status=%s", bindRes.CommitHash, bindRes.TapdID, bindRes.CurrentStatus)

	// ==========================================
	// Round 109: Test GenerateHotfixManifest
	// ==========================================
	hotfixRes, err := svc.GenerateHotfixManifest(tempAssetRepo, "HEAD~1", "HEAD", "v1.2.0")
	if err != nil || hotfixRes.AppVersion != "v1.2.0" {
		t.Fatalf("GenerateHotfixManifest failed: %v, res=%+v", err, hotfixRes)
	}
	t.Logf("Hotfix manifest: version=%s, totalFiles=%d, totalBytes=%d, fullPackRequired=%v",
		hotfixRes.ManifestVersion, hotfixRes.TotalFilesCount, hotfixRes.TotalPatchBytes, hotfixRes.FullPackRequired)

	// ==========================================
	// Round 110: Test AnalyzePackageSizeBudget
	// ==========================================
	budgetRes, err := svc.AnalyzePackageSizeBudget(tempAssetRepo, "IOS")
	if err != nil || budgetRes.TotalSizeBytes == 0 {
		t.Fatalf("AnalyzePackageSizeBudget failed: %v, res=%+v", err, budgetRes)
	}
	t.Logf("Package size budget: total=%s, overrun=%v, worstPlatform=%s",
		budgetRes.TotalSizeStr, budgetRes.HasOverrun, budgetRes.WorstPlatform)

	// ==========================================
	// Round 111: Test ManageStudioSwitchboard
	// ==========================================
	switchboardRes, err := svc.ManageStudioSwitchboard(tempAssetRepo, "list", "", "", "")
	if err != nil || switchboardRes.TotalProjects == 0 {
		t.Fatalf("ManageStudioSwitchboard failed: %v, res=%+v", err, switchboardRes)
	}
	t.Logf("Studio switchboard: totalProjects=%d, active=%s",
		switchboardRes.TotalProjects, switchboardRes.ActiveProjectPath)

	// ==========================================
	// Round 112: Test AuditDataTableDiff
	// ==========================================
	tableDir := filepath.Join(tempAssetRepo, "Config")
	_ = os.MkdirAll(tableDir, 0755)
	heroCsv := filepath.Join(tableDir, "Hero.csv")
	_ = os.WriteFile(heroCsv, []byte("ID,Name,Atk,Def\n101,Warrior,120,50\n102,Mage,150,30\n"), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "Config/Hero.csv")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: base hero table")

	_ = os.WriteFile(heroCsv, []byte("ID,Name,Atk,Def,Speed\n101,Warrior,140,50,10\n103,Archer,130,40,15\n"), 0644)
	tableRes, err := svc.AuditDataTableDiff(tempAssetRepo, "", "HEAD", "")
	if err != nil || tableRes.TotalTables == 0 {
		t.Fatalf("AuditDataTableDiff failed: %v, res=%+v", err, tableRes)
	}
	t.Logf("Data table audit: tables=%d, criticals=%d, warnings=%d, valid=%v, summary=%s",
		tableRes.TotalTables, tableRes.CriticalCount, tableRes.WarningCount, tableRes.IsValid, tableRes.OverallSummary)

	// ==========================================
	// Round 113: Test InspectCredentialVault & StoreCredentialSecret
	// ==========================================
	storeSecRes, err := svc.StoreCredentialSecret(tempAssetRepo, "TENCENT_CLOUD_AKSK", "AKID_STUDIO_TEST_1", "SecretKeyVal9876543210ABCDEF", "LOCAL_REPO")
	if err != nil || !storeSecRes.Success || storeSecRes.MaskedValue == "" || storeSecRes.EncryptionMethod != "AES-256-GCM-KMS" {
		t.Fatalf("StoreCredentialSecret failed: %v, res=%+v", err, storeSecRes)
	}
	t.Logf("KMS Store Secret: ID=%s, Masked=%s, Algo=%s", storeSecRes.SecretID, storeSecRes.MaskedValue, storeSecRes.EncryptionMethod)

	vaultRes, err := svc.InspectCredentialVault(tempAssetRepo)
	if err != nil || vaultRes.TotalCredentials == 0 || !vaultRes.KmsEncryptionEnabled {
		t.Fatalf("InspectCredentialVault failed: %v, res=%+v", err, vaultRes)
	}
	t.Logf("Credential Vault: Total=%d, Secure=%d, KMS=%v, Summary=%s",
		vaultRes.TotalCredentials, vaultRes.SecureCount, vaultRes.KmsEncryptionEnabled, vaultRes.Summary)

	// ==========================================
	// Round 114: Test DemangleCrashStack
	// ==========================================
	crashSrcDir := filepath.Join(tempAssetRepo, "Source")
	_ = os.MkdirAll(crashSrcDir, 0755)
	heroSrcFile := filepath.Join(crashSrcDir, "Hero.cpp")
	_ = os.WriteFile(heroSrcFile, []byte("// Hero implementation\n#include \"Hero.h\"\nint Hero::CalculateDamage() {\n    return 100;\n}\n"), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "Source/Hero.cpp")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: hero combat logic")

	rawCrashDump := `Crash Reason: EXCEPTION_ACCESS_VIOLATION_READ (0x0000000000000008)
Faulting Module: GameCore.dll
Thread 0 (Crashed):
#0  0x0000000100003fa0 in ?CalculateDamage@Hero@@QEAAHXZ at Source/Hero.cpp:4
#1  0x00007ff6a1b2c340 in _ZN4Hero15CalculateDamageEv at Source/Hero.cpp:4
`
	demangleRes, err := svc.DemangleCrashStack(tempAssetRepo, rawCrashDump, "HEAD")
	if err != nil || demangleRes.DemangledCount < 2 || demangleRes.CulpritFile == "" {
		t.Fatalf("DemangleCrashStack failed: %v, res=%+v", err, demangleRes)
	}
	if demangleRes.Frames[0].DemangledSymbol != "Hero::CalculateDamage" || demangleRes.Frames[1].DemangledSymbol != "Hero::CalculateDamage" {
		t.Fatalf("Demangle symbol mismatch: frame0=%s, frame1=%s", demangleRes.Frames[0].DemangledSymbol, demangleRes.Frames[1].DemangledSymbol)
	}
	t.Logf("Crash Demangler: TotalFrames=%d, DemangledCount=%d, Culprit=%s (Author: %s, Commit: %s)",
		demangleRes.TotalFrames, demangleRes.DemangledCount, demangleRes.CulpritFile, demangleRes.CulpritAuthor, demangleRes.CulpritCommit)

	// ==========================================
	// Round 115: Test CherryPickAssetBundle
	// ==========================================
	origBranch, _ := ExecGit(tempAssetRepo, "branch", "--show-current")
	origBranch = strings.TrimSpace(origBranch)
	if origBranch == "" {
		origBranch = "master"
	}
	_, _ = ExecGit(tempAssetRepo, "checkout", "-b", "feature/art-assets")
	artMatDir := filepath.Join(tempAssetRepo, "Assets", "Materials")
	artTexDir := filepath.Join(tempAssetRepo, "Assets", "Textures")
	_ = os.MkdirAll(artMatDir, 0755)
	_ = os.MkdirAll(artTexDir, 0755)

	artTexFile := filepath.Join(artTexDir, "Hero_Albedo.png")
	artTexMeta := filepath.Join(artTexDir, "Hero_Albedo.png.meta")
	_ = os.WriteFile(artTexFile, []byte("DUMMY PNG DATA"), 0644)
	_ = os.WriteFile(artTexMeta, []byte("fileFormatVersion: 2\nguid: a1b2c3d4e5f60718293a4b5c6d7e8f90\nTextureImporter:\n"), 0644)

	artMatFile := filepath.Join(artMatDir, "Hero.mat")
	artMatMeta := filepath.Join(artMatDir, "Hero.mat.meta")
	artMatContent := `YAML
m_Shader: {fileID: 4800000, guid: 0000000000000000f000000000000000, type: 3}
m_SavedProperties:
  m_TexEnvs:
    - _MainTex:
        m_Texture: {fileID: 2800000, guid: a1b2c3d4e5f60718293a4b5c6d7e8f90, type: 3}
`
	_ = os.WriteFile(artMatFile, []byte(artMatContent), 0644)
	_ = os.WriteFile(artMatMeta, []byte("fileFormatVersion: 2\nguid: f0f1f2f3f4f5f6f7f8f9fa0b1c2d3e4f\n"), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "Assets")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: art hero material and albedo texture")

	// Switch back to original branch
	_, _ = ExecGit(tempAssetRepo, "checkout", origBranch)
	if _, err := os.Stat(artMatFile); err == nil {
		t.Fatalf("Hero.mat should not exist in %s branch before cherry-pick", origBranch)
	}

	cherryRes, err := svc.CherryPickAssetBundle(tempAssetRepo, "feature/art-assets", []string{"Assets/Materials/Hero.mat"}, true, false)
	if err != nil || !cherryRes.Success || cherryRes.TotalAssetsCount < 4 {
		t.Fatalf("CherryPickAssetBundle failed: %v, res=%+v", err, cherryRes)
	}
	t.Logf("Selective Asset Cherry-Pick: Total=%d (Requested: %d, Dep: %d), Summary=%s",
		cherryRes.TotalAssetsCount, cherryRes.TotalRequested, cherryRes.TotalDependencies, cherryRes.Summary)

	// Verify both Material, Texture, and their .meta files exist on disk
	if _, err := os.Stat(artMatFile); err != nil {
		t.Fatalf("Cherry-picked material not found on disk: %v", err)
	}
	if _, err := os.Stat(artTexFile); err != nil {
		t.Fatalf("Cherry-picked dependent texture not found on disk: %v", err)
	}

	// ==========================================
	// Round 116: Test ScanMissingMaterials
	// ==========================================
	pinkMatFile := filepath.Join(artMatDir, "BrokenPink.mat")
	pinkMatMeta := filepath.Join(artMatDir, "BrokenPink.mat.meta")
	pinkContent := `YAML
m_Shader: {fileID: 4800000, guid: e1e2e3e4e5e6e7e8e9e0e1e2e3e4e5e6, type: 3}
m_ShaderName: "Hidden/InternalErrorShader"
m_SavedProperties:
  m_TexEnvs:
    - _BumpMap:
        m_Texture: {fileID: 2800000, guid: 99999999999999999999999999999999, type: 3}
`
	_ = os.WriteFile(pinkMatFile, []byte(pinkContent), 0644)
	_ = os.WriteFile(pinkMatMeta, []byte("fileFormatVersion: 2\nguid: d1d2d3d4d5d6d7d8d9d0d1d2d3d4d5d6\n"), 0644)

	scanRes, err := svc.ScanMissingMaterials(tempAssetRepo, "Assets")
	if err != nil || scanRes.PinkShaderCount == 0 || scanRes.BrokenMaterialsCount == 0 {
		t.Fatalf("ScanMissingMaterials failed: %v, res=%+v", err, scanRes)
	}
	t.Logf("Pink Shader Guard: Scanned=%d, Broken=%d, PinkCount=%d, MissingTex=%d, Score=%d",
		scanRes.TotalMaterialsScanned, scanRes.BrokenMaterialsCount, scanRes.PinkShaderCount, scanRes.MissingTexturesCount, scanRes.HealthScore)

	// ==========================================
	// Round 117: Test ManageOfflineSubmitQueue
	// ==========================================
	qRes1, err := svc.ManageOfflineSubmitQueue(tempAssetRepo, "enqueue", "", `{"operation":"push","target":"main"}`)
	if err != nil || qRes1.TotalCount != 1 || qRes1.QueuedCount != 1 {
		t.Fatalf("ManageOfflineSubmitQueue enqueue failed: %v, res=%+v", err, qRes1)
	}

	qRes2, err := svc.ManageOfflineSubmitQueue(tempAssetRepo, "pause", "all", "")
	if err != nil || qRes2.PausedCount != 1 {
		t.Fatalf("ManageOfflineSubmitQueue pause failed: %v, res=%+v", err, qRes2)
	}

	qRes3, err := svc.ManageOfflineSubmitQueue(tempAssetRepo, "resume", "all", "")
	if err != nil || qRes3.QueuedCount != 1 {
		t.Fatalf("ManageOfflineSubmitQueue resume failed: %v, res=%+v", err, qRes3)
	}

	qRes4, err := svc.ManageOfflineSubmitQueue(tempAssetRepo, "replay", "all", "")
	if err != nil {
		t.Fatalf("ManageOfflineSubmitQueue replay failed: %v", err)
	}
	t.Logf("Offline queue: total=%d, queued=%d, failed=%d, msg=%s",
		qRes4.TotalCount, qRes4.QueuedCount, qRes4.FailedCount, qRes4.Message)

	qRes5, err := svc.ManageOfflineSubmitQueue(tempAssetRepo, "discard", "all", "")
	if err != nil || qRes5.TotalCount != 0 {
		t.Fatalf("ManageOfflineSubmitQueue discard failed: %v, res=%+v", err, qRes5)
	}

	// ==========================================
	// Round 118: Test AuditPrePushChecklist
	// ==========================================
	checklistRes, err := svc.AuditPrePushChecklist(tempAssetRepo, origBranch)
	if err != nil || checklistRes.PassReadinessScore == 0 {
		t.Fatalf("AuditPrePushChecklist failed: %v, res=%+v", err, checklistRes)
	}
	t.Logf("Pre-Push Checklist: score=%d, grade=%s, blocked=%v, rules=%d, summary=%s",
		checklistRes.PassReadinessScore, checklistRes.ReadinessGrade, checklistRes.IsBlocked, len(checklistRes.RulesChecked), checklistRes.Summary)

	// ==========================================
	// Round 119: Test AuditLodStreaming
	// ==========================================
	lodDir := filepath.Join(tempAssetRepo, "Assets", "LODTest")
	_ = os.MkdirAll(lodDir, 0755)

	objMeshPath := filepath.Join(lodDir, "BossMonster.obj")
	var objLines []string
	objLines = append(objLines, "# Wavefront OBJ test")
	for i := 1; i <= 200; i++ {
		objLines = append(objLines, fmt.Sprintf("v %.2f %.2f %.2f", float64(i), float64(i*2), 0.0))
	}
	for i := 1; i <= 25000; i++ {
		objLines = append(objLines, fmt.Sprintf("f %d %d %d", (i%190)+1, (i%190)+2, (i%190)+3))
	}
	_ = os.WriteFile(objMeshPath, []byte(strings.Join(objLines, "\n")), 0644)

	// 128-byte DDS test header (POT 1024x1024, 11 mipmaps)
	ddsPath := filepath.Join(lodDir, "Boss_Diffuse.dds")
	ddsHeader := make([]byte, 128)
	copy(ddsHeader[0:4], []byte("DDS "))
	binary.LittleEndian.PutUint32(ddsHeader[12:16], 1024) // height
	binary.LittleEndian.PutUint32(ddsHeader[16:20], 1024) // width
	binary.LittleEndian.PutUint32(ddsHeader[28:32], 11)   // mipmaps
	_ = os.WriteFile(ddsPath, ddsHeader, 0644)

	lodAuditRes, err := svc.AuditLodStreaming(tempAssetRepo, "Assets/LODTest")
	if err != nil || lodAuditRes.TotalMeshesAudited == 0 || lodAuditRes.TotalTexturesAudited == 0 {
		t.Fatalf("AuditLodStreaming failed: %v, res=%+v", err, lodAuditRes)
	}
	t.Logf("LOD & Streaming Audit: meshes=%d, textures=%d, baselineVram=%.2fMB, streamingVram=%.2fMB, savings=%.1f%%, score=%d",
		lodAuditRes.TotalMeshesAudited, lodAuditRes.TotalTexturesAudited,
		lodAuditRes.BaselineVramMB, lodAuditRes.StreamingVramMB,
		lodAuditRes.SavingsPercentage, lodAuditRes.OverallHealthScore)

	// ==========================================
	// Round 120: Test GetFullParityStatusSummary
	// ==========================================
	parityHubRes, err := svc.GetFullParityStatusSummary(tempAssetRepo)
	if err != nil || parityHubRes.CompletedRounds != 120 || parityHubRes.ParityPercentage != 100.0 {
		t.Fatalf("GetFullParityStatusSummary failed: %v, res=%+v", err, parityHubRes)
	}
	t.Logf("Grand Parity Hub: rounds=%d/%d (%.1f%%), badge=%s, readiness=%s, domains=%d, tests=%d",
		parityHubRes.CompletedRounds, parityHubRes.TotalRounds, parityHubRes.ParityPercentage,
		parityHubRes.CertificationBadge, parityHubRes.SystemReadiness,
		len(parityHubRes.Domains), parityHubRes.TotalTestCoverageCount)

	// ==========================================
	// ==========================================
	// ==========================================
	// Round 121: Test AuditAudioBankDiff
	// ==========================================
	{
		r121AudioDir := filepath.Join(tempAssetRepo, "Audio", "Wwise")
		_ = os.MkdirAll(r121AudioDir, 0755)
		sampleBnk := filepath.Join(r121AudioDir, "Weapons.bnk")
		_ = os.WriteFile(sampleBnk, []byte("BKHD\x00\x00\x00\x10AKBK\x00\x00\x00\x00Event_Shoot_Ak47\x00Event_Reload\x00"), 0644)
		// The diff reads both sides from git objects, so the fixture has to be
		// committed; leaving it untracked makes HEAD:<path> unresolvable.
		runGitTest(t, tempAssetRepo, nil, "add", "Audio/Wwise/Weapons.bnk")
		runGitTest(t, tempAssetRepo, nil, "commit", "-m", "test: add wwise bank fixture")
		res, err := svc.AuditAudioBankDiff(tempAssetRepo, "Audio/Wwise/Weapons.bnk", "HEAD", "HEAD")
		if err != nil || res == nil {
			t.Fatalf("AuditAudioBankDiff failed: %v", err)
		}
		t.Logf("AudioBank Diff: engine=%s, bank=%s, added=%d, mod=%d, summary=%s",
			res.EngineType, res.BankName, len(res.AddedEvents), len(res.ModifiedEvents), res.Summary)
	}

	// ==========================================
	// Round 122: Test AuditPhysicsMeshCollision
	// ==========================================
	{
		r122PhysDir := filepath.Join(tempAssetRepo, "Assets", "PhysicsTest")
		_ = os.MkdirAll(r122PhysDir, 0755)
		physMatPath := filepath.Join(r122PhysDir, "IceFriction.physicMaterial")
		_ = os.WriteFile(physMatPath, []byte("%YAML 1.1\nPhysicMaterial:\nm_DynamicFriction: 0.05\nm_StaticFriction: 0.05\n"), 0644)
		res, err := svc.AuditPhysicsMeshCollision(tempAssetRepo, "Assets/PhysicsTest")
		if err != nil || res == nil {
			t.Fatalf("AuditPhysicsMeshCollision failed: %v", err)
		}
		t.Logf("Physics Mesh Collision: totalColliders=%d, convexCount=%d, score=%d",
			res.TotalColliders, res.ConvexHullsCount, res.HealthScore)
	}

	// ==========================================
	// Round 123: Test DiffWorldDataLayers
	// ==========================================
	{
		dataLayerPath := filepath.Join(tempAssetRepo, "WorldPartition", "DataLayer_Cinematics.asset")
		_ = os.MkdirAll(filepath.Dir(dataLayerPath), 0755)
		_ = os.WriteFile(dataLayerPath, []byte("DataLayerInstance: CinematicLayer\nm_InitialState: Activated\n"), 0644)
		res, err := svc.DiffWorldDataLayers(tempAssetRepo, "HEAD", "HEAD")
		if err != nil || res == nil {
			t.Fatalf("DiffWorldDataLayers failed: %v", err)
		}
		t.Logf("World Data Layers Diff: totalLayers=%d, conflicts=%d, summary=%s",
			res.TotalDataLayers, len(res.StreamingConflicts), res.Summary)
	}

	// ==========================================
	// Round 124: Test AuditMaterialInstanceOverrides
	// ==========================================
	{
		r124MatDir := filepath.Join(tempAssetRepo, "Assets", "Materials")
		_ = os.MkdirAll(r124MatDir, 0755)
		matInstPath := filepath.Join(r124MatDir, "Hero_Armor_Inst.mat")
		_ = os.WriteFile(matInstPath, []byte("%YAML 1.1\nMaterial:\nm_Shader: {guid: 1234}\nm_SavedProperties:\n  m_TexEnvs:\n    - _MainTex: {guid: hero_alb}\n"), 0644)
		res, err := svc.AuditMaterialInstanceOverrides(tempAssetRepo, "Assets/Materials")
		if err != nil || res == nil {
			t.Fatalf("AuditMaterialInstanceOverrides failed: %v", err)
		}
		t.Logf("Material Instance Overrides: instances=%d, overridden=%d, dead=%d",
			res.TotalInstances, res.OverriddenParametersCount, res.DeadParametersCount)
	}

	// ==========================================
	// Round 125: Test AuditSymbolsAndDefines
	// ==========================================
	{
		rspPath := filepath.Join(tempAssetRepo, "csc.rsp")
		_ = os.WriteFile(rspPath, []byte("-define:DEBUG_LOG;ENABLE_PROFILER;UNIT_TEST\n"), 0644)
		res, err := svc.AuditSymbolsAndDefines(tempAssetRepo, "HEAD", "HEAD")
		if err != nil || res == nil {
			t.Fatalf("AuditSymbolsAndDefines failed: %v", err)
		}
		t.Logf("Symbols & Defines: total=%d, active=%d, dangerous=%d, severity=%s",
			res.TotalDefines, len(res.ActiveDefines), len(res.DangerousDebugDefines), res.Severity)
	}

	// ==========================================
	// Round 126: Test DetectAssetCyclicDependencies
	// ==========================================
	{
		r126CycleDir := filepath.Join(tempAssetRepo, "Assets", "Cycles")
		_ = os.MkdirAll(r126CycleDir, 0755)
		prefabA := filepath.Join(r126CycleDir, "PrefabA.prefab")
		prefabB := filepath.Join(r126CycleDir, "PrefabB.prefab")
		_ = os.WriteFile(prefabA, []byte("PrefabA guid: 11111111111111111111111111111111 ref: 22222222222222222222222222222222\n"), 0644)
		_ = os.WriteFile(prefabB, []byte("PrefabB guid: 22222222222222222222222222222222 ref: 11111111111111111111111111111111\n"), 0644)
		res, err := svc.DetectAssetCyclicDependencies(tempAssetRepo, "Assets/Cycles")
		if err != nil || res == nil {
			t.Fatalf("DetectAssetCyclicDependencies failed: %v", err)
		}
		t.Logf("Asset Cyclic Dependencies: scanned=%d, cyclesCount=%d, hasCycles=%v",
			res.TotalAssetsScanned, res.CyclesCount, res.HasCycles)
	}

	// ==========================================
	// Round 127: Test AuditTextureChannelPacking
	// ==========================================
	{
		r127TexDir := filepath.Join(tempAssetRepo, "Assets", "Textures")
		_ = os.MkdirAll(r127TexDir, 0755)
		ormPath := filepath.Join(r127TexDir, "Character_Hero_ORM.png")
		_ = os.WriteFile(ormPath, []byte("fake_orm_png_bytes"), 0644)
		ormMeta := filepath.Join(r127TexDir, "Character_Hero_ORM.png.meta")
		_ = os.WriteFile(ormMeta, []byte("sRGBTexture: 0\ntextureType: 0\n"), 0644)
		res, err := svc.AuditTextureChannelPacking(tempAssetRepo, "Assets/Textures")
		if err != nil || res == nil {
			t.Fatalf("AuditTextureChannelPacking failed: %v", err)
		}
		t.Logf("Texture Channel Packing: scanned=%d, ormCompliant=%d, unpacked=%d",
			res.TotalTexturesScanned, res.OrmCompliantCount, res.UnpackedTexturesCount)
	}

	// ==========================================
	// Round 128: Test ManageStudioWebhooks
	// ==========================================
	{
		resList, err := svc.ManageStudioWebhooks(tempAssetRepo, "list", "", "", "")
		if err != nil || resList == nil {
			t.Fatalf("ManageStudioWebhooks list failed: %v", err)
		}
		resAdd, err := svc.ManageStudioWebhooks(tempAssetRepo, "add", "WECOM", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=mock", "COMMIT")
		if err != nil || resAdd == nil {
			t.Fatalf("ManageStudioWebhooks add failed: %v", err)
		}
		t.Logf("Studio Webhooks: total=%d, success=%v, msg=%s",
			resAdd.TotalWebhooks, resAdd.Success, resAdd.Message)
	}

	// ==========================================
	// Round 129: Test ManageStudioDDC
	// ==========================================
	{
		res, err := svc.ManageStudioDDC(tempAssetRepo, "status")
		if err != nil || res == nil {
			t.Fatalf("ManageStudioDDC failed: %v", err)
		}
		t.Logf("Studio DDC: localCache=%s, sharedPath=%s, isConnected=%v, hitRatio=%.2f, latency=%dms",
			res.LocalCachePath, res.SharedCachePath, res.IsSharedConnected, res.CacheHitRatio, res.LatencyMs)
	}

	// ==========================================
	// Round 130: Test AnalyzeBundleDiff
	// ==========================================
	{
		catV1 := filepath.Join(tempAssetRepo, "catalog_v1.json")
		catV2 := filepath.Join(tempAssetRepo, "catalog_v2.json")
		_ = os.WriteFile(catV1, []byte(`{"version":"1.0","bundles":[{"name":"ui_core","size":1048576,"hash":"aaa111"}]}`), 0644)
		_ = os.WriteFile(catV2, []byte(`{"version":"1.1","bundles":[{"name":"ui_core","size":1200000,"hash":"bbb222"},{"name":"hero_bundle","size":500000,"hash":"ccc333"}]}`), 0644)
		res, err := svc.AnalyzeBundleDiff(tempAssetRepo, "catalog_v1.json", "catalog_v2.json")
		if err != nil || res == nil {
			t.Fatalf("AnalyzeBundleDiff failed: %v", err)
		}
		t.Logf("Bundle Diff: base=%d, target=%d, changed=%d, initialDelta=%s, patchBytes=%s",
			res.TotalBundlesBase, res.TotalBundlesTarget, len(res.ChangedBundles), res.InitialInstallDeltaStr, res.PatchDownloadStr)
	}

	// ==========================================
	// Round 131: Test GenerateOfflinePatchBundle
	// ==========================================
	{
		res, err := svc.GenerateOfflinePatchBundle(tempAssetRepo, "HEAD~1", "HEAD", "")
		if err != nil || res == nil {
			t.Fatalf("GenerateOfflinePatchBundle failed: %v", err)
		}
		t.Logf("Offline Patch Bundle: path=%s, commits=%d, lfs=%d, totalSize=%s, success=%v",
			res.BundlePath, res.CommitsCount, res.LfsBlobsCount, res.TotalSizeStr, res.Success)
	}

	// ==========================================
	// Round 132: Test AuditLightmapBakeSettings
	// ==========================================
	{
		r132LightingDir := filepath.Join(tempAssetRepo, "Assets", "Lighting")
		_ = os.MkdirAll(r132LightingDir, 0755)
		lightSettingPath := filepath.Join(r132LightingDir, "MainLevelLighting.lighting")
		_ = os.WriteFile(lightSettingPath, []byte("LightingSettings:\nm_LightmapResolution: 32\nm_Bounces: 3\nm_LightmapMaxSize: 1024\n"), 0644)
		res, err := svc.AuditLightmapBakeSettings(tempAssetRepo, "Assets/Lighting")
		if err != nil || res == nil {
			t.Fatalf("AuditLightmapBakeSettings failed: %v", err)
		}
		t.Logf("Lightmap Bake Settings: totalMeshes=%d, overlapUv2=%d, score=%d",
			res.TotalMeshes, res.OverlappingUv2Count, res.HealthScore)
	}

	// ==========================================
	// Round 133: Test AuditPrivacyAndCompliance
	// ==========================================
	{
		r133PrivDir := filepath.Join(tempAssetRepo, "App")
		_ = os.MkdirAll(r133PrivDir, 0755)
		iosPriv := filepath.Join(r133PrivDir, "PrivacyInfo.xcprivacy")
		_ = os.WriteFile(iosPriv, []byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>NSPrivacyTracking</key><false/><key>NSPrivacyAccessedAPITypes</key><array><dict><key>NSPrivacyAccessedAPIType</key><string>NSPrivacyAccessedAPICategoryUserDefaults</string><key>NSPrivacyAccessedAPITypeReasons</key><array><string>CA92.1</string></array></dict></array></dict></plist>`), 0644)
		res, err := svc.AuditPrivacyAndCompliance(tempAssetRepo)
		if err != nil || res == nil {
			t.Fatalf("AuditPrivacyAndCompliance failed: %v", err)
		}
		t.Logf("Privacy Compliance: platform=%s, permissions=%d, sensitive=%d, grade=%s, ready=%v",
			res.Platform, res.TotalPermissionsDeclared, res.SensitivePermissionsCount, res.StoreReadinessGrade, res.IsStoreReady)
	}

	// ==========================================
	// Round 134: Test AuditFontLicenses
	// ==========================================
	{
		r134FontDir := filepath.Join(tempAssetRepo, "Assets", "Fonts")
		_ = os.MkdirAll(r134FontDir, 0755)
		fontPath := filepath.Join(r134FontDir, "GameFont_OFL.ttf")
		_ = os.WriteFile(fontPath, []byte("\x00\x01\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00name\x00\x00\x00\x00\x00\x00\x00\x20SIL Open Font License Source Han Sans\n"), 0644)
		res, err := svc.AuditFontLicenses(tempAssetRepo)
		if err != nil || res == nil {
			t.Fatalf("AuditFontLicenses failed: %v", err)
		}
		t.Logf("Font License Audit: total=%d, safe=%d, restricted=%d, riskScore=%s",
			res.TotalFonts, res.CommercialSafeCount, res.RestrictedFontsCount, res.LegalRiskScore)
	}

	// ==========================================
	// Round 135: Test TraceHotfixBackportStatus
	// ==========================================
	{
		// The tracker resolves both refs, so the fixture repo needs them to
		// exist regardless of the machine's init.defaultBranch setting.
		runGitTest(t, tempAssetRepo, nil, "branch", "-f", "main", "HEAD")
		runGitTest(t, tempAssetRepo, nil, "branch", "-f", "hotfix/v1.0.1", "HEAD")
		res, err := svc.TraceHotfixBackportStatus(tempAssetRepo, "hotfix/v1.0.1", "main")
		if err != nil || res == nil {
			t.Fatalf("TraceHotfixBackportStatus failed: %v", err)
		}
		t.Logf("Hotfix Backport Tracker: hotfix=%s, target=%s, total=%d, backported=%d, isFully=%v",
			res.HotfixBranch, res.TargetBranch, res.TotalHotfixCommits, res.BackportedCommits, res.IsFullyBackported)
	}

	// ==========================================
	// Round 136: Test AuditCommitContentSafety
	// ==========================================
	{
		res, err := svc.AuditCommitContentSafety(tempAssetRepo, 5)
		if err != nil || res == nil {
			t.Fatalf("AuditCommitContentSafety failed: %v", err)
		}
		t.Logf("Commit Content Safety: audited=%d, violations=%d, passed=%v, summary=%s",
			res.CommitsAudited, res.ViolationsCount, res.Passed, res.Summary)
	}

	// ==========================================
	// Round 138: Test ArtistOneClickPipeline
	// ==========================================
	{
		res, err := svc.ArtistOneClickPipeline(tempAssetRepo, "feat: artist delivery", "main")
		if err != nil || res == nil {
			t.Fatalf("ArtistOneClickPipeline failed: %v", err)
		}
		t.Logf("Artist One-Click Pipeline: staged=%d, locked=%d, success=%v, msg=%s",
			res.StagedAssetsCount, res.LockedAssetsCount, res.Success, res.Message)
	}

	// ==========================================
	// Round 139: Test ShaderSelfHealingWizard
	// ==========================================
	{
		res, err := svc.ShaderSelfHealingWizard(tempAssetRepo, "diagnose")
		if err != nil || res == nil {
			t.Fatalf("ShaderSelfHealingWizard failed: %v", err)
		}
		t.Logf("Shader Self-Healing Wizard: engine=%s, broken=%d, blackMeshes=%d, grade=%s, success=%v",
			res.EngineType, res.BrokenShaderCount, res.BlackRenderMeshesCount, res.HealthGrade, res.Success)
	}

	// ==========================================
	// Round 140: Test GetPinnacleParityStatusSummary
	// ==========================================
	{
		res, err := svc.GetPinnacleParityStatusSummary(tempAssetRepo)
		if err != nil || res.CompletedRounds != 140 || res.ParityPercentage != 100.0 {
			t.Fatalf("GetPinnacleParityStatusSummary failed: %v, res=%+v", err, res)
		}
		t.Logf("Grand Pinnacle Master Parity Hub: rounds=%d/%d (%.1f%%), badge=%s, readiness=%s, domains=%d, tests=%d, tools=%d",
			res.CompletedRounds, res.TotalRounds, res.ParityPercentage,
			res.CertificationBadge, res.SystemReadiness,
			len(res.Domains), res.TotalTestCoverageCount, res.TotalToolsCount)
	}

	// ==========================================
	// Test Project Source Code Browser
	// ==========================================
	{
		testFile := filepath.Join(tempAssetRepo, "main.go")
		_ = os.WriteFile(testFile, []byte("package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n"), 0644)
		_, _ = ExecGit(tempAssetRepo, "add", "main.go")
		_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: add main.go for source browser test")

		files, err := svc.GetProjectSourceFiles(tempAssetRepo)
		if err != nil || len(files) == 0 {
			t.Fatalf("GetProjectSourceFiles failed: %v", err)
		}
		t.Logf("Project source files count: %d", len(files))

		detail, err := svc.GetSourceFileDetail(tempAssetRepo, "main.go", "HEAD")
		if err != nil || detail.Language != "Go" || detail.LineCount < 4 {
			t.Fatalf("GetSourceFileDetail failed: %v, detail=%+v", err, detail)
		}
		t.Logf("Source file detail: path=%s, lang=%s, lines=%d, size=%s",
			detail.Path, detail.Language, detail.LineCount, detail.SizeFormatted)

		// Test GetSourceFileDiff
		diffStr, err := svc.GetSourceFileDiff(tempAssetRepo, "main.go", "HEAD")
		if err != nil {
			t.Fatalf("GetSourceFileDiff failed: %v", err)
		}
		t.Logf("GetSourceFileDiff: len=%d", len(diffStr))
	}
}

func TestGitService_MergeAndRebase(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()

	// 1. Setup repository
	if _, err := ExecGit(repo, "init", "-b", "main"); err != nil {
		if _, errInit := ExecGit(repo, "init"); errInit != nil {
			t.Fatalf("git init failed: %v", errInit)
		}
		_, _ = ExecGit(repo, "checkout", "-b", "main")
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@wgit.com")

	baseFile := filepath.Join(repo, "base.txt")
	_ = os.WriteFile(baseFile, []byte("line 1\nline 2\nline 3\n"), 0644)
	_, _ = ExecGit(repo, "add", "base.txt")
	_, _ = ExecGit(repo, "commit", "-m", "Initial commit on main")

	// 2. Test MergeAdvanced with --no-ff
	_, _ = ExecGit(repo, "checkout", "-b", "feature-noff")
	_ = os.WriteFile(filepath.Join(repo, "feature_noff.txt"), []byte("noff feature content"), 0644)
	_, _ = ExecGit(repo, "add", "feature_noff.txt")
	_, _ = ExecGit(repo, "commit", "-m", "feat: add noff feature")

	_, _ = ExecGit(repo, "checkout", "main")
	mergeRes, err := svc.MergeAdvanced(repo, MergeAdvancedOptions{
		TargetBranch:  "feature-noff",
		Strategy:      "no-ff",
		CommitMessage: "Merge feature-noff with no-ff",
	})
	if err != nil || mergeRes == nil || !mergeRes.Success {
		t.Fatalf("MergeAdvanced (--no-ff) failed: %v, res=%+v", err, mergeRes)
	}
	t.Logf("Merge --no-ff result: %s", mergeRes.Message)

	// Verify merge commit exists
	lastMsg, _ := svc.GetLastCommitMessage(repo)
	if !strings.Contains(lastMsg, "Merge feature-noff with no-ff") {
		t.Errorf("Expected merge commit message, got: %s", lastMsg)
	}

	// 3. Test MergeAdvanced with --squash
	_, _ = ExecGit(repo, "checkout", "-b", "feature-squash")
	_ = os.WriteFile(filepath.Join(repo, "sq1.txt"), []byte("squash 1"), 0644)
	_, _ = ExecGit(repo, "add", "sq1.txt")
	_, _ = ExecGit(repo, "commit", "-m", "sq commit 1")
	_ = os.WriteFile(filepath.Join(repo, "sq2.txt"), []byte("squash 2"), 0644)
	_, _ = ExecGit(repo, "add", "sq2.txt")
	_, _ = ExecGit(repo, "commit", "-m", "sq commit 2")

	_, _ = ExecGit(repo, "checkout", "main")
	squashRes, err := svc.MergeAdvanced(repo, MergeAdvancedOptions{
		TargetBranch: "feature-squash",
		Strategy:     "squash",
	})
	if err != nil || squashRes == nil || !squashRes.Success {
		t.Fatalf("MergeAdvanced (--squash) failed: %v, res=%+v", err, squashRes)
	}

	// Status should have staged files
	st, err := svc.GetRepoStatus(repo)
	if err != nil || len(st.StagedFiles) < 2 {
		t.Fatalf("Expected staged files after --squash, got %+v", st)
	}
	// Commit squashed changes
	_, _ = ExecGit(repo, "commit", "-m", "feat: squashed feature")

	// 4. Test Merge Conflict detection & Abort
	_, _ = ExecGit(repo, "checkout", "-b", "branch-conflict")
	_ = os.WriteFile(baseFile, []byte("conflict from branch\n"), 0644)
	_, _ = ExecGit(repo, "add", "base.txt")
	_, _ = ExecGit(repo, "commit", "-m", "conflict from branch")

	_, _ = ExecGit(repo, "checkout", "main")
	_ = os.WriteFile(baseFile, []byte("conflict from main\n"), 0644)
	_, _ = ExecGit(repo, "add", "base.txt")
	_, _ = ExecGit(repo, "commit", "-m", "conflict from main")

	// Try merging branch-conflict -> conflict is returned as an actionable result
	conflictRes, err := svc.MergeAdvanced(repo, MergeAdvancedOptions{TargetBranch: "branch-conflict"})
	if err != nil || conflictRes == nil || !conflictRes.HasConflicts || conflictRes.Success {
		t.Fatalf("Expected MergeAdvanced conflict result, err=%v res=%+v", err, conflictRes)
	}
	stConflict, err := svc.GetRepoStatus(repo)
	if err != nil {
		t.Fatalf("GetRepoStatus failed during merge conflict: %v", err)
	}
	if !stConflict.IsMerging {
		t.Errorf("Expected IsMerging == true, got %v", stConflict.IsMerging)
	}
	if len(stConflict.ConflictedFiles) == 0 {
		t.Errorf("Expected ConflictedFiles > 0, got %d", len(stConflict.ConflictedFiles))
	}
	t.Logf("Detected merge conflict: IsMerging=%v, HeadBranch=%s, Conflicts=%v",
		stConflict.IsMerging, stConflict.MergeHeadBranch, stConflict.ConflictedFiles)

	// Test AbortConflictOperation on merge
	if _, err := svc.AbortConflictOperation(repo); err != nil {
		t.Fatalf("AbortConflictOperation (merge) failed: %v", err)
	}
	stAfterAbort, _ := svc.GetRepoStatus(repo)
	if stAfterAbort.IsMerging || len(stAfterAbort.ConflictedFiles) > 0 {
		t.Errorf("Expected IsMerging == false after AbortConflictOperation, got %v", stAfterAbort.IsMerging)
	}

	// 5. Test RebaseAdvanced with --autostash
	_, _ = ExecGit(repo, "checkout", "-b", "feature-rebase")
	_ = os.WriteFile(filepath.Join(repo, "rb_feature.txt"), []byte("rebase feature"), 0644)
	_, _ = ExecGit(repo, "add", "rb_feature.txt")
	_, _ = ExecGit(repo, "commit", "-m", "feat: rebase feature commit")

	// Switch to main, add commit
	_, _ = ExecGit(repo, "checkout", "main")
	_ = os.WriteFile(filepath.Join(repo, "main_advance.txt"), []byte("main advanced"), 0644)
	_, _ = ExecGit(repo, "add", "main_advance.txt")
	_, _ = ExecGit(repo, "commit", "-m", "chore: advance main")

	// Switch to feature-rebase, create dirty unstaged file
	_, _ = ExecGit(repo, "checkout", "feature-rebase")
	dirtyFile := filepath.Join(repo, "dirty_work.txt")
	_ = os.WriteFile(dirtyFile, []byte("uncommitted work"), 0644)

	// Rebase onto main with --autostash
	rebaseRes, err := svc.RebaseAdvanced(repo, RebaseAdvancedOptions{Upstream: "main", Autostash: true})
	if err != nil || rebaseRes == nil || !rebaseRes.Success {
		t.Fatalf("RebaseAdvanced (--autostash) failed: %v, res=%+v", err, rebaseRes)
	}
	t.Logf("Rebase with autostash result: %s", rebaseRes.Message)

	// Verify dirty work is preserved
	dirtyContent, err := os.ReadFile(dirtyFile)
	if err != nil || string(dirtyContent) != "uncommitted work" {
		t.Errorf("Expected dirty_work.txt to be preserved by autostash, got %s (err: %v)", string(dirtyContent), err)
	}
	_ = os.Remove(dirtyFile)

	// 6. Test Rebase Conflict, State Detection, and AbortConflictOperation
	_, _ = ExecGit(repo, "checkout", "-b", "feature-rebase-conflict")
	_ = os.WriteFile(baseFile, []byte("rebase conflict line on branch\n"), 0644)
	_, _ = ExecGit(repo, "add", "base.txt")
	_, _ = ExecGit(repo, "commit", "-m", "rebase conflict commit")

	// Switch to main, modify same base.txt
	_, _ = ExecGit(repo, "checkout", "main")
	_ = os.WriteFile(baseFile, []byte("rebase conflict line on main\n"), 0644)
	_, _ = ExecGit(repo, "add", "base.txt")
	_, _ = ExecGit(repo, "commit", "-m", "main rebase conflict commit")

	// Switch to feature-rebase-conflict and rebase onto main -> triggers conflict
	_, _ = ExecGit(repo, "checkout", "feature-rebase-conflict")
	rebaseConflictRes, err := svc.RebaseAdvanced(repo, RebaseAdvancedOptions{Upstream: "main"})
	if err != nil || rebaseConflictRes == nil || !rebaseConflictRes.HasConflicts || rebaseConflictRes.Success {
		t.Fatalf("Expected RebaseAdvanced conflict result, err=%v res=%+v", err, rebaseConflictRes)
	}

	stRebaseConflict, err := svc.GetRepoStatus(repo)
	if err != nil {
		t.Fatalf("GetRepoStatus failed during rebase conflict: %v", err)
	}
	if !stRebaseConflict.IsRebasing {
		t.Errorf("Expected IsRebasing == true, got %v", stRebaseConflict.IsRebasing)
	}
	t.Logf("Detected rebase in-progress: IsRebasing=%v, Head=%s, Step=%d/%d, Conflicts=%v",
		stRebaseConflict.IsRebasing, stRebaseConflict.RebaseHeadName,
		stRebaseConflict.RebaseStep, stRebaseConflict.RebaseTotal,
		stRebaseConflict.ConflictedFiles)

	// Abort rebase
	if _, err := svc.AbortConflictOperation(repo); err != nil {
		t.Fatalf("AbortConflictOperation (rebase) failed: %v", err)
	}
	stAfterRebaseAbort, _ := svc.GetRepoStatus(repo)
	if stAfterRebaseAbort.IsRebasing {
		t.Errorf("Expected IsRebasing == false after AbortConflictOperation, got %v", stAfterRebaseAbort.IsRebasing)
	}

	// 7. Test Cherry-Pick In-Progress Detection and AbortConflictOperation
	commits, _ := svc.GetCommits(repo, 5, "feature-rebase-conflict")
	if len(commits) > 0 {
		conflictSha := commits[0].Hash
		_, _ = ExecGit(repo, "checkout", "main")
		_ = svc.CherryPick(repo, conflictSha)

		stCherry, _ := svc.GetRepoStatus(repo)
		t.Logf("Cherry-pick conflict state: IsCherryPicking=%v, Conflicts=%v",
			stCherry.IsCherryPicking, stCherry.ConflictedFiles)
		if !stCherry.IsCherryPicking {
			t.Errorf("Expected IsCherryPicking == true during cherry-pick conflict")
		}

		// Abort cherry-pick
		if _, err := svc.AbortConflictOperation(repo); err != nil {
			t.Fatalf("AbortConflictOperation (cherry-pick) failed: %v", err)
		}
		stAfterCherryAbort, _ := svc.GetRepoStatus(repo)
		if stAfterCherryAbort.IsCherryPicking {
			t.Errorf("Expected IsCherryPicking == false after AbortConflictOperation")
		}
	}
}

func TestCompareBranchesEmptySlices(t *testing.T) {
	repo := t.TempDir()
	if _, err := ExecGit(repo, "init", "-b", "main"); err != nil {
		if _, errInit := ExecGit(repo, "init"); errInit != nil {
			t.Fatalf("git init failed: %v", errInit)
		}
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@wgit.com")
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("line\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repo, "add", "base.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repo, "commit", "-m", "init"); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.CompareBranches(repo, "main", "main")
	if err != nil {
		t.Fatalf("CompareBranches failed: %v", err)
	}
	if res.AheadCommits == nil || res.BehindCommits == nil {
		t.Fatalf("empty compare must return empty slices, got ahead=%v behind=%v", res.AheadCommits, res.BehindCommits)
	}
}

func TestGetSourceFileDiffRootCommit(t *testing.T) {
	repo := t.TempDir()
	if _, err := ExecGit(repo, "init", "-b", "main"); err != nil {
		if _, errInit := ExecGit(repo, "init"); errInit != nil {
			t.Fatalf("git init failed: %v", errInit)
		}
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@wgit.com")
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("line\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, _ = ExecGit(repo, "add", "base.txt")
	if _, err := ExecGit(repo, "commit", "-m", "root"); err != nil {
		t.Fatal(err)
	}

	diff, err := GitService{}.GetSourceFileDiff(repo, "base.txt", "HEAD")
	if err != nil {
		t.Fatalf("GetSourceFileDiff failed: %v", err)
	}
	if !strings.Contains(diff, "diff --git") || !strings.Contains(diff, "+line") {
		t.Fatalf("expected root commit patch, got %q", diff)
	}
}

func TestArtistOneClickPipelineCommitsAndPushes(t *testing.T) {
	repo := t.TempDir()
	remote := t.TempDir()
	if _, err := ExecGit(remote, "init", "--bare"); err != nil {
		t.Fatalf("init bare remote: %v", err)
	}
	if _, err := ExecGit(repo, "init", "-b", "main"); err != nil {
		t.Fatalf("init repo: %v", err)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@wgit.com")
	if err := os.MkdirAll(filepath.Join(repo, "Assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Assets", "hero.png"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = ExecGit(repo, "add", "-A")
	if _, err := ExecGit(repo, "commit", "-m", "base"); err != nil {
		t.Fatalf("base commit: %v", err)
	}
	if _, err := ExecGit(repo, "remote", "add", "origin", remote); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repo, "push", "-u", "origin", "main"); err != nil {
		t.Fatalf("initial push: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Assets", "hero.png"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.ArtistOneClickPipeline(repo, "artist delivery", "main")
	if err != nil {
		t.Fatalf("ArtistOneClickPipeline: %v", err)
	}
	if !res.Success || res.StagedAssetsCount == 0 || res.TexturesChecked == 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if !res.Pushed {
		t.Fatalf("expected push success: %+v", res)
	}
	out, err := ExecGit(remote, "log", "--oneline", "main")
	if err != nil || !strings.Contains(out, "[Art] artist delivery") {
		t.Fatalf("remote missing artist commit: %q err=%v", out, err)
	}
}

func TestGitServiceWorkingDiffAdvanced(t *testing.T) {
	repo := t.TempDir()
	if _, err := ExecGit(repo, "init", "-b", "main"); err != nil {
		if _, errInit := ExecGit(repo, "init"); errInit != nil {
			t.Fatalf("git init failed: %v", errInit)
		}
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@wgit.com")

	svc := GitService{}

	// 1. Initial file commit
	initialContent := "func calculate() int {\n\treturn 42;\n}\n"
	filePath := "calc.go"
	fullPath := filepath.Join(repo, filePath)
	if err := os.WriteFile(fullPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repo, "add", filePath); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repo, "commit", "-m", "init calc"); err != nil {
		t.Fatal(err)
	}

	// 2. Introduce whitespace-only changes (tab changed to spaces)
	whitespaceContent := "func calculate() int {\n        return 42;\n}\n"
	if err := os.WriteFile(fullPath, []byte(whitespaceContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Unstaged diff without ignoreWhitespace should show diff
	normalDiff, err := svc.GetWorkingDiffAdvanced(repo, filePath, false, false)
	if err != nil {
		t.Fatalf("GetWorkingDiffAdvanced normal failed: %v", err)
	}
	if strings.TrimSpace(normalDiff) == "" || !strings.Contains(normalDiff, "return 42;") {
		t.Fatalf("expected non-empty diff showing whitespace changes, got: %q", normalDiff)
	}

	// Unstaged diff WITH ignoreWhitespace should omit whitespace changes (empty)
	ignoredDiff, err := svc.GetWorkingDiffAdvanced(repo, filePath, false, true)
	if err != nil {
		t.Fatalf("GetWorkingDiffAdvanced ignoreWhitespace failed: %v", err)
	}
	if strings.TrimSpace(ignoredDiff) != "" {
		t.Fatalf("expected empty diff when ignoreWhitespace=true, got: %q", ignoredDiff)
	}

	// 3. Stage the whitespace changes and test staged diff
	if _, err := ExecGit(repo, "add", filePath); err != nil {
		t.Fatal(err)
	}

	stagedNormalDiff, err := svc.GetWorkingDiffAdvanced(repo, filePath, true, false)
	if err != nil {
		t.Fatalf("staged GetWorkingDiffAdvanced normal failed: %v", err)
	}
	if strings.TrimSpace(stagedNormalDiff) == "" {
		t.Fatalf("expected non-empty staged diff, got empty")
	}

	stagedIgnoredDiff, err := svc.GetWorkingDiffAdvanced(repo, filePath, true, true)
	if err != nil {
		t.Fatalf("staged GetWorkingDiffAdvanced ignoreWhitespace failed: %v", err)
	}
	if strings.TrimSpace(stagedIgnoredDiff) != "" {
		t.Fatalf("expected empty staged diff when ignoreWhitespace=true, got: %q", stagedIgnoredDiff)
	}

	// 4. Test GetWorkingDiff backward compatibility
	legacyDiff, err := svc.GetWorkingDiff(repo, filePath, true)
	if err != nil {
		t.Fatalf("GetWorkingDiff failed: %v", err)
	}
	if legacyDiff != stagedNormalDiff {
		t.Fatalf("expected GetWorkingDiff to match stagedNormalDiff, got %q vs %q", legacyDiff, stagedNormalDiff)
	}

	// 5. Test untracked file handling
	untrackedPath := "untracked.txt"
	if err := os.WriteFile(filepath.Join(repo, untrackedPath), []byte("hello untracked\n"), 0644); err != nil {
		t.Fatal(err)
	}
	untrackedDiff, err := svc.GetWorkingDiffAdvanced(repo, untrackedPath, false, false)
	if err != nil {
		t.Fatalf("untracked diff failed: %v", err)
	}
	if !strings.Contains(untrackedDiff, "hello untracked") {
		t.Fatalf("expected untracked content in diff, got: %q", untrackedDiff)
	}

	untrackedDiffIgnored, err := svc.GetWorkingDiffAdvanced(repo, untrackedPath, false, true)
	if err != nil {
		t.Fatalf("untracked diff with ignoreWhitespace failed: %v", err)
	}
	if !strings.Contains(untrackedDiffIgnored, "hello untracked") {
		t.Fatalf("expected untracked content in ignored diff, got: %q", untrackedDiffIgnored)
	}
}
