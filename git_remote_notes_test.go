package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteManagementAndNotesSyncInTempRepo(t *testing.T) {
	svc := GitService{}
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "base.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	head, _ := ExecGit(repo, "rev-parse", "HEAD")
	head = strings.TrimSpace(head)
	baseBranch, _ := ExecGit(repo, "symbolic-ref", "--short", "HEAD")
	baseBranch = strings.TrimSpace(baseBranch)

	remote1 := filepath.Join(parent, "remote1.git")
	remote2 := filepath.Join(parent, "remote2.git")
	for _, remote := range []string{remote1, remote2} {
		if _, err := ExecGit(parent, "init", "--bare", remote); err != nil {
			t.Fatalf("init bare remote failed: %v", err)
		}
	}

	if err := svc.AddRemote(repo, "origin", remote1); err != nil {
		t.Fatalf("AddRemote failed: %v", err)
	}
	if _, err := svc.Push(repo, "origin", baseBranch, false); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	multi, err := svc.GetMultiRemoteStatus(repo)
	if err != nil || multi == nil || multi.CurrentBranch != baseBranch || len(multi.Remotes) != 1 {
		t.Fatalf("GetMultiRemoteStatus failed: %+v err=%v", multi, err)
	}
	if multi.Remotes[0].Name != "origin" || multi.Remotes[0].FetchURL != remote1 || multi.Remotes[0].Ahead != 0 || multi.Remotes[0].Behind != 0 {
		t.Fatalf("unexpected multi-remote status: %+v", multi.Remotes[0])
	}
	syncResult, err := svc.SyncMultiRemotes(repo, "fetch-all")
	if err != nil || syncResult == nil || !syncResult.Success || len(syncResult.Messages) == 0 {
		t.Fatalf("SyncMultiRemotes fetch-all failed: %+v err=%v", syncResult, err)
	}
	if _, err := svc.SyncMultiRemotes(repo, "unsupported"); err == nil {
		t.Fatal("expected error for unsupported multi-remote action")
	}
	conn, err := svc.TestRemoteConnection(repo, "origin")
	if err != nil || conn == nil || !conn.Success || conn.Latency == "" {
		t.Fatalf("TestRemoteConnection failed: %+v err=%v", conn, err)
	}
	var sawBranch bool
	for _, headName := range conn.Heads {
		if headName == baseBranch {
			sawBranch = true
		}
	}
	if !sawBranch {
		t.Fatalf("expected remote head %s, got %+v", baseBranch, conn.Heads)
	}
	badConn, err := svc.TestRemoteConnection(repo, "missing-remote")
	if err != nil || badConn == nil || badConn.Success || badConn.Error == "" {
		t.Fatalf("expected failed connection result, got %+v err=%v", badConn, err)
	}

	noteMessage, err := svc.SaveCommitNote(repo, head, "review pass")
	if err != nil || noteMessage == "" {
		t.Fatalf("SaveCommitNote failed: %q err=%v", noteMessage, err)
	}
	note, err := svc.GetCommitNote(repo, head)
	if err != nil || !strings.Contains(note, "review pass") {
		t.Fatalf("GetCommitNote failed: %q err=%v", note, err)
	}
	noted, err := svc.GetNotedCommitHashes(repo)
	if err != nil {
		t.Fatalf("GetNotedCommitHashes failed: %v", err)
	}
	var notedHead bool
	for _, hash := range noted {
		if strings.HasPrefix(head, hash) || strings.HasPrefix(hash, head) {
			notedHead = true
		}
	}
	if !notedHead {
		t.Fatalf("expected HEAD in noted hashes, got %+v", noted)
	}
	pushOut, err := svc.PushCommitNotes(repo, "origin")
	if err != nil {
		t.Fatalf("PushCommitNotes failed: %v (%s)", err, pushOut)
	}
	lsNotes, err := ExecGit(repo, "ls-remote", "origin", "refs/notes/commits")
	if err != nil || !strings.Contains(lsNotes, "refs/notes/commits") {
		t.Fatalf("expected notes ref on remote, got %q err=%v", lsNotes, err)
	}
	if _, err := svc.PushCommitNotes(repo, ""); err != nil {
		t.Fatalf("PushCommitNotes with default remote failed: %v", err)
	}

	if err := svc.RenameRemote(repo, "origin", "upstream"); err != nil {
		t.Fatalf("RenameRemote failed: %v", err)
	}
	remotes, _ := svc.GetRemoteDetails(repo)
	for _, remote := range remotes {
		if remote.Name != "upstream" {
			t.Fatalf("expected only upstream after rename, got %+v", remotes)
		}
	}
	if err := svc.SetRemoteURL(repo, "upstream", remote2); err != nil {
		t.Fatalf("SetRemoteURL failed: %v", err)
	}
	remotes, _ = svc.GetRemoteDetails(repo)
	if len(remotes) < 2 {
		t.Fatalf("expected fetch/push remote entries, got %+v", remotes)
	}
	for _, remote := range remotes {
		if remote.Name != "upstream" || remote.URL != remote2 {
			t.Fatalf("unexpected remote after set-url: %+v", remote)
		}
	}
	conn, err = svc.TestRemoteConnection(repo, "upstream")
	if err != nil || conn == nil || !conn.Success {
		t.Fatalf("TestRemoteConnection(upstream) failed: %+v err=%v", conn, err)
	}
	if len(conn.Heads) != 0 {
		t.Fatalf("expected empty remote2 heads, got %+v", conn.Heads)
	}
	if _, err := svc.PushCommitNotes(repo, "upstream"); err != nil {
		t.Fatalf("PushCommitNotes(upstream) failed: %v", err)
	}
	if _, err := svc.PushCommitNotes(repo, ""); err != nil {
		t.Fatalf("PushCommitNotes with non-origin default failed: %v", err)
	}
	lsNotes, err = ExecGit(repo, "ls-remote", "upstream", "refs/notes/commits")
	if err != nil || !strings.Contains(lsNotes, "refs/notes/commits") {
		t.Fatalf("expected notes ref on remote2, got %q err=%v", lsNotes, err)
	}
	if err := svc.RemoveRemote(repo, "upstream"); err != nil {
		t.Fatalf("RemoveRemote failed: %v", err)
	}
	multi, err = svc.GetMultiRemoteStatus(repo)
	if err != nil || multi == nil || multi.Remotes == nil || len(multi.Remotes) != 0 {
		t.Fatalf("expected an empty non-nil remote list, got %+v err=%v", multi, err)
	}
	if _, err := svc.SyncMultiRemotes(repo, "push-all"); err == nil {
		t.Fatal("expected push-all to fail without remotes")
	}

	if err := svc.RenameRemote(repo, "", "x"); err == nil {
		t.Fatal("expected error renaming empty remote")
	}
	if err := svc.SetRemoteURL(repo, "", remote2); err == nil {
		t.Fatal("expected error setting empty remote name")
	}
	if err := svc.SetRemoteURL(repo, "upstream", ""); err == nil {
		t.Fatal("expected error setting empty remote URL")
	}
}
