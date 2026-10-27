package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddSubtreePersistsMetadata(t *testing.T) {
	lib := t.TempDir()
	main := t.TempDir()

	run := func(dir string, args ...string) {
		t.Helper()
		if _, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v in %s failed: %v", args, dir, err)
		}
	}
	write := func(dir, name, content string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, dir := range []string{lib, main} {
		run(dir, "init", "-q", "-b", "main")
		run(dir, "config", "user.email", "t@example.com")
		run(dir, "config", "user.name", "tester")
	}
	run(lib, "config", "receive.denyCurrentBranch", "ignore")
	write(lib, "lib.txt", "v1\n")
	run(lib, "add", "lib.txt")
	run(lib, "commit", "-qm", "lib base")
	write(main, "README.md", "main\n")
	run(main, "add", "README.md")
	run(main, "commit", "-qm", "main base")

	svc := GitService{}
	if _, err := svc.AddSubtree(main, "libs/lib", lib, "main", true); err != nil {
		t.Fatalf("AddSubtree: %v", err)
	}

	res, err := svc.ListSubtrees(main)
	if err != nil {
		t.Fatalf("ListSubtrees: %v", err)
	}
	if len(res.Subtrees) != 1 {
		t.Fatalf("got %d subtrees, want 1: %+v", len(res.Subtrees), res.Subtrees)
	}
	got := res.Subtrees[0]
	if got.Prefix != "libs/lib" || got.RemoteOrURL != lib || got.Branch != "main" {
		t.Fatalf("subtree metadata = %+v", got)
	}

	// Make the metadata path unwritable (a directory) while leaving git itself
	// healthy: the subtree operation must still succeed and report the warning.
	metaPath := filepath.Join(gitCommonDir(main), "wgit-subtrees.json")
	if err := os.Remove(metaPath); err != nil {
		t.Fatalf("remove subtree metadata: %v", err)
	}
	if err := os.Mkdir(metaPath, 0o755); err != nil {
		t.Fatalf("replace subtree metadata with directory: %v", err)
	}

	write(lib, "lib.txt", "v2\n")
	run(lib, "add", "lib.txt")
	run(lib, "commit", "-qm", "lib v2")
	pulled, err := svc.PullSubtree(main, "libs/lib", lib, "main", true)
	if err != nil {
		t.Fatalf("PullSubtree: %v, result=%+v", err, pulled)
	}
	if !pulled.Success || !strings.Contains(pulled.Message, "本地子树信息保存失败") {
		t.Fatalf("PullSubtree must report metadata persistence failure: %+v", pulled)
	}
	if data, err := os.ReadFile(filepath.Join(main, "libs", "lib", "lib.txt")); err != nil || string(data) != "v2\n" {
		t.Fatalf("PullSubtree did not update working tree: %q err=%v", data, err)
	}

	write(main, filepath.Join("libs", "lib", "lib.txt"), "from-main\n")
	run(main, "add", "libs/lib/lib.txt")
	run(main, "commit", "-qm", "main updates subtree")
	pushed, err := svc.PushSubtree(main, "libs/lib", lib, "main")
	if err != nil {
		t.Fatalf("PushSubtree: %v, result=%+v", err, pushed)
	}
	if !pushed.Success || !strings.Contains(pushed.Message, "本地子树信息保存失败") {
		t.Fatalf("PushSubtree must report metadata persistence failure: %+v", pushed)
	}
	if err := os.Remove(metaPath); err != nil {
		t.Fatalf("remove metadata directory: %v", err)
	}
	pushedContent, err := ExecGit(lib, "show", "main:lib.txt")
	if err != nil || pushedContent != "from-main" {
		t.Fatalf("PushSubtree did not update upstream: %q err=%v", pushedContent, err)
	}
	res, _ = svc.ListSubtrees(main)
	if len(res.Subtrees) != 1 || res.Subtrees[0].Prefix != "libs/lib" {
		t.Fatalf("subtree metadata lost after pull/push: %+v", res.Subtrees)
	}
	if _, err := svc.PullSubtree(main, "", lib, "main", true); err == nil {
		t.Fatal("expected error for empty subtree prefix")
	}
	if _, err := svc.PushSubtree(main, "libs/lib", "", "main"); err == nil {
		t.Fatal("expected error for empty subtree URL")
	}
}

func TestSubtreeMetadataCorruptionIsNotOverwritten(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v (%s)", err, out)
	}
	if out, err := ExecGit(repo, "config", "user.name", "Tester"); err != nil {
		t.Fatalf("git config: %v (%s)", err, out)
	}
	if out, err := ExecGit(repo, "config", "user.email", "tester@example.com"); err != nil {
		t.Fatalf("git config: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "base"}} {
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}

	metaPath := subtreeMetadataPath(repo)
	corrupt := []byte(`{"libs/lib":`)
	if err := os.WriteFile(metaPath, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := (GitService{}).ListSubtrees(repo); err == nil || !strings.Contains(err.Error(), "损坏") {
		t.Fatalf("ListSubtrees must report corrupt metadata, got %v", err)
	}
	if err := saveSubtreeMetadata(repo, "libs/lib", subtreeMetadata{RemoteOrURL: "x", Branch: "main"}); err == nil || !strings.Contains(err.Error(), "损坏") {
		t.Fatalf("saveSubtreeMetadata must refuse to overwrite corrupt metadata, got %v", err)
	}
	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read corrupt metadata: %v", err)
	}
	if !bytes.Equal(after, corrupt) {
		t.Fatalf("corrupt metadata must stay untouched, got %q", after)
	}
}

func TestListSubtreesMultilineBodyParsing(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if _, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}
	writeFile := func(name, content string) {
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

	// Commit 1: base README
	writeFile("README.md", "# Monorepo\n")
	run("add", "README.md")
	run("commit", "-m", "chore: initial commit")

	// Commit 2: Subtree with multiline commit message body where git-subtree-split is separated by text
	splitShaA := "1111222233334444555566667777888899990000"
	writeFile("packages/shared-ui/Button.tsx", "export const Button = () => null;\n")
	run("add", "packages/shared-ui/Button.tsx")
	msgA := "Squashed 'packages/shared-ui' content from commit " + splitShaA + "\n\n" +
		"Additional commit explanation line 1\n" +
		"Additional commit explanation line 2\n\n" +
		"git-subtree-dir: packages/shared-ui\n" +
		"git-subtree-split: " + splitShaA + "\n"
	run("commit", "-m", msgA)

	// Commit 3: Another subtree with multiline message
	splitShaB := "aaaabbbbccccddddeeeeffff0000111122223333"
	writeFile("packages/core-utils/index.ts", "export const add = (a: number, b: number) => a + b;\n")
	run("add", "packages/core-utils/index.ts")
	msgB := "Squashed 'packages/core-utils' content from commit " + splitShaB + "\n\n" +
		"Summary of changes:\n" +
		"- Added math utilities\n\n" +
		"git-subtree-dir: packages/core-utils\n" +
		"git-subtree-split: " + splitShaB + "\n"
	run("commit", "-m", msgB)

	// Commit 4: A historical subtree commit whose directory has been physically removed from disk
	writeFile("packages/obsolete/tmp.txt", "obsolete\n")
	run("add", "packages/obsolete/tmp.txt")
	run("commit", "-m", "Squashed 'packages/obsolete'\n\ngit-subtree-dir: packages/obsolete\ngit-subtree-split: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef\n")
	run("rm", "-r", "packages/obsolete")
	run("commit", "-m", "chore: remove obsolete subtree physically")

	// Pre-populate metadata for shared-ui, and add metadata for a third directory that is only in wgit-subtrees.json
	svc := GitService{}
	if err := svc.UpdateSubtreeMetadata(repo, "packages/shared-ui", "https://github.com/example/ui.git", "develop", true); err != nil {
		t.Fatalf("UpdateSubtreeMetadata: %v", err)
	}

	// Create directory for metadata-only subtree
	writeFile("packages/plugins/plugin.json", `{"name": "plugin"}`)
	run("add", "packages/plugins/plugin.json")
	run("commit", "-m", "feat: add plugin directory")
	if err := svc.UpdateSubtreeMetadata(repo, "packages/plugins", "https://github.com/example/plugins.git", "main", false); err != nil {
		t.Fatalf("UpdateSubtreeMetadata plugins: %v", err)
	}

	// Now list subtrees
	res, err := svc.ListSubtrees(repo)
	if err != nil {
		t.Fatalf("ListSubtrees failed: %v", err)
	}

	// We expect exactly 3 subtrees: core-utils (from git log), shared-ui (from git log + metadata), plugins (from metadata + physical dir)
	// packages/obsolete MUST BE EXCLUDED because it does not physically exist!
	if len(res.Subtrees) != 3 {
		t.Fatalf("expected 3 subtrees, got %d: %+v", len(res.Subtrees), res.Subtrees)
	}

	// Verify core-utils
	var coreFound, uiFound, pluginFound bool
	for _, it := range res.Subtrees {
		switch it.Prefix {
		case "packages/core-utils":
			coreFound = true
			if it.LastSquash != splitShaB {
				t.Errorf("core-utils LastSquash = %q, want %q", it.LastSquash, splitShaB)
			}
			if it.LastCommit == "" {
				t.Errorf("core-utils LastCommit should not be empty")
			}
			if it.Branch != "main" {
				t.Errorf("core-utils Branch = %q, want 'main'", it.Branch)
			}
		case "packages/shared-ui":
			uiFound = true
			if it.LastSquash != splitShaA {
				t.Errorf("shared-ui LastSquash = %q, want %q", it.LastSquash, splitShaA)
			}
			if it.RemoteOrURL != "https://github.com/example/ui.git" {
				t.Errorf("shared-ui RemoteOrURL = %q, want 'https://github.com/example/ui.git'", it.RemoteOrURL)
			}
			if it.Branch != "develop" {
				t.Errorf("shared-ui Branch = %q, want 'develop'", it.Branch)
			}
		case "packages/plugins":
			pluginFound = true
			if it.RemoteOrURL != "https://github.com/example/plugins.git" {
				t.Errorf("plugins RemoteOrURL = %q, want 'https://github.com/example/plugins.git'", it.RemoteOrURL)
			}
		case "packages/obsolete":
			t.Errorf("packages/obsolete should have been excluded because physical dir is deleted")
		}
	}

	if !coreFound || !uiFound || !pluginFound {
		t.Fatalf("expected coreFound=true, uiFound=true, pluginFound=true, got %v, %v, %v", coreFound, uiFound, pluginFound)
	}
}

func TestSplitSubtree(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if _, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}
	writeFile := func(name, content string) {
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

	writeFile("README.md", "# Root\n")
	run("add", "README.md")
	run("commit", "-m", "chore: initial")

	writeFile("components/dialog/index.ts", "export const Dialog = true;\n")
	writeFile("components/dialog/style.css", ".dialog { color: blue; }\n")
	run("add", "components/dialog")
	run("commit", "-m", "feat(dialog): add dialog component")

	writeFile("components/dialog/index.ts", "export const Dialog = 'v2';\n")
	run("add", "components/dialog/index.ts")
	run("commit", "-m", "feat(dialog): upgrade dialog component")

	svc := GitService{}

	// Test 1: Split with branch name and leading/trailing slash cleaning
	splitRes, err := svc.SplitSubtree(repo, "/components/dialog/", "split-dialog-branch", false, false)
	if err != nil {
		t.Fatalf("SplitSubtree failed: %v, result: %+v", err, splitRes)
	}
	if !splitRes.Success {
		t.Fatalf("expected split success, got: %+v", splitRes)
	}
	if splitRes.BranchName != "split-dialog-branch" {
		t.Errorf("expected branch name split-dialog-branch, got: %q", splitRes.BranchName)
	}
	if len(splitRes.NewCommit) != 40 {
		t.Errorf("expected 40-char commit sha, got: %q", splitRes.NewCommit)
	}
	if !strings.Contains(splitRes.Message, "split-dialog-branch") {
		t.Errorf("expected message to mention branch, got: %s", splitRes.Message)
	}

	// Verify that the split branch actually exists in git and its root contains index.ts
	content, err := ExecGit(repo, "show", "split-dialog-branch:index.ts")
	if err != nil || !strings.Contains(content, "export const Dialog = 'v2'") {
		t.Fatalf("split branch does not contain expected file at root: err=%v, content=%s", err, content)
	}

	// Test 2: Split without branch name (generates split commit SHA)
	splitNoBranch, err := svc.SplitSubtree(repo, "components/dialog", "", false, false)
	if err != nil {
		t.Fatalf("SplitSubtree without branch failed: %v", err)
	}
	if !splitNoBranch.Success || len(splitNoBranch.NewCommit) != 40 {
		t.Fatalf("expected valid split result without branch, got: %+v", splitNoBranch)
	}

	// Verify commit object exists in git
	if _, err := ExecGit(repo, "cat-file", "-e", splitNoBranch.NewCommit); err != nil {
		t.Fatalf("split commit does not exist in repo object db: %v", err)
	}

	// Test 3: Split with rejoin and squash
	splitRejoin, err := svc.SplitSubtree(repo, "components/dialog", "dialog-rejoin", true, true)
	if err != nil {
		t.Fatalf("SplitSubtree with squash and rejoin failed: %v", err)
	}
	if !splitRejoin.Success || len(splitRejoin.NewCommit) != 40 {
		t.Fatalf("expected valid rejoin result, got: %+v", splitRejoin)
	}

	// Test 4: Error handling for non-existent prefix
	if _, err := svc.SplitSubtree(repo, "non-existent-dir", "invalid", false, false); err == nil {
		t.Errorf("expected error for non-existent prefix")
	}

	// Test 5: Error handling for empty prefix
	if _, err := svc.SplitSubtree(repo, "", "invalid", false, false); err == nil {
		t.Errorf("expected error for empty prefix")
	}
}

func TestSubtreeMetadataCRUD(t *testing.T) {
	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v (%s)", err, out)
	}
	if out, err := ExecGit(repo, "config", "user.name", "Tester"); err != nil {
		t.Fatalf("git config: %v (%s)", err, out)
	}
	if out, err := ExecGit(repo, "config", "user.email", "tester@example.com"); err != nil {
		t.Fatalf("git config: %v (%s)", err, out)
	}

	svc := GitService{}

	// Test Create/Update with slashes
	err := svc.UpdateSubtreeMetadata(repo, "/vendor/analytics/", "https://github.com/org/analytics.git", "release-1.0", true)
	if err != nil {
		t.Fatalf("UpdateSubtreeMetadata: %v", err)
	}

	metas, err := readSubtreeMetadata(repo)
	if err != nil {
		t.Fatalf("readSubtreeMetadata: %v", err)
	}
	item, ok := metas["vendor/analytics"]
	if !ok {
		t.Fatalf("expected 'vendor/analytics' in metadata, got: %+v", metas)
	}
	if item.RemoteOrURL != "https://github.com/org/analytics.git" || item.Branch != "release-1.0" || !item.Squash {
		t.Fatalf("metadata content mismatch: %+v", item)
	}

	// Update existing record
	err = svc.UpdateSubtreeMetadata(repo, "vendor/analytics", "git@github.com:org/analytics.git", "main", false)
	if err != nil {
		t.Fatalf("UpdateSubtreeMetadata update: %v", err)
	}
	metas, _ = readSubtreeMetadata(repo)
	item = metas["vendor/analytics"]
	if item.RemoteOrURL != "git@github.com:org/analytics.git" || item.Branch != "main" || item.Squash {
		t.Fatalf("updated metadata mismatch: %+v", item)
	}

	// Create physical directory to ensure UnlinkSubtree does NOT delete physical files
	physDir := filepath.Join(repo, "vendor", "analytics")
	if err := os.MkdirAll(physDir, 0o755); err != nil {
		t.Fatal(err)
	}
	physFile := filepath.Join(physDir, "analytics.js")
	if err := os.WriteFile(physFile, []byte("console.log('hi');"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Unlink subtree
	err = svc.UnlinkSubtree(repo, "/vendor/analytics/")
	if err != nil {
		t.Fatalf("UnlinkSubtree: %v", err)
	}

	metas, _ = readSubtreeMetadata(repo)
	if _, ok := metas["vendor/analytics"]; ok {
		t.Fatalf("expected 'vendor/analytics' to be removed from metadata, got: %+v", metas)
	}

	// Verify physical files remain intact
	if _, err := os.Stat(physFile); err != nil {
		t.Fatalf("UnlinkSubtree must not remove physical files: %v", err)
	}

	// Empty prefix validation
	if err := svc.UpdateSubtreeMetadata(repo, "  /  ", "url", "main", false); err == nil {
		t.Errorf("expected error for empty prefix in UpdateSubtreeMetadata")
	}
	if err := svc.UnlinkSubtree(repo, ""); err == nil {
		t.Errorf("expected error for empty prefix in UnlinkSubtree")
	}
}
