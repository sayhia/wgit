package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffUnrealAssetNodesMarksBinaryAsUnverified(t *testing.T) {
	repo := t.TempDir()
	data := make([]byte, 20)
	binary.LittleEndian.PutUint32(data[0:4], 0x9E2A83C1)
	data = append(data, []byte("K2Node_CallFunction\x00FunctionName\x00K2Node_Event\x00")...)
	if err := os.WriteFile(filepath.Join(repo, "BP.bin.uasset"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.DiffUnrealAssetNodes(repo, "BP.bin.uasset", "4b825dc642cb6eb9a060e54bf8d69288fbee4904", "WORKTREE")
	if err != nil {
		t.Fatalf("diff binary uasset: %v", err)
	}
	if res.AnalysisKnown || !strings.Contains(res.AnalysisNote, "未验证") {
		t.Fatalf("binary UE scan must be marked unverified: %+v", res)
	}
	if !strings.Contains(res.VisualSummary, "未验证 AST") {
		t.Fatalf("summary should carry the AST caveat: %q", res.VisualSummary)
	}
	if res.TargetHeader == nil || !strings.Contains(res.TargetHeader.Magic, "UE Asset") {
		t.Fatalf("binary header should still be reported: %+v", res.TargetHeader)
	}
}

func TestDiffUnrealAssetNodesKeepsTextBlueprintVerified(t *testing.T) {
	repo := t.TempDir()
	text := `Begin Object Class=/Script/BlueprintGraph.K2Node_CallFunction Name="CallFoo"
   FunctionReference=(MemberName="PrintString")
End Object
`
	if err := os.WriteFile(filepath.Join(repo, "BP_text.uasset"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := GitService{}.DiffUnrealAssetNodes(repo, "BP_text.uasset", "4b825dc642cb6eb9a060e54bf8d69288fbee4904", "WORKTREE")
	if err != nil {
		t.Fatalf("diff text blueprint: %v", err)
	}
	if !res.AnalysisKnown || res.AnalysisNote != "" {
		t.Fatalf("text blueprint should be treated as verified: %+v", res)
	}
	if res.TargetHeader == nil || !strings.Contains(res.TargetHeader.Magic, "Textual Blueprint") {
		t.Fatalf("unexpected text blueprint header: %+v", res.TargetHeader)
	}
}

func TestDiffUnrealAssetNodesRejectsInvalidRefs(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "BP.uasset"), []byte("K2Node_CallFunction"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (GitService{}).DiffUnrealAssetNodes(repo, "BP.uasset", "missing-base", "WORKTREE"); err == nil {
		t.Fatal("invalid base ref must return an error instead of reporting all nodes as added")
	}
	if _, err := (GitService{}).DiffUnrealAssetNodes(repo, "BP.uasset", "4b825dc642cb6eb9a060e54bf8d69288fbee4904", "missing-target"); err == nil {
		t.Fatal("invalid target ref must return an error")
	}
}
