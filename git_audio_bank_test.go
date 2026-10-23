package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testWwiseObject struct {
	id      uint32
	objType byte
	extra   []byte
}

func buildWwiseBank(version, bankID uint32, objects []testWwiseObject) []byte {
	bkhdPayload := make([]byte, 8)
	binary.LittleEndian.PutUint32(bkhdPayload[0:4], version)
	binary.LittleEndian.PutUint32(bkhdPayload[4:8], bankID)

	hircBody := make([]byte, 4)
	binary.LittleEndian.PutUint32(hircBody[0:4], uint32(len(objects)))
	for _, obj := range objects {
		payload := make([]byte, 4+len(obj.extra))
		binary.LittleEndian.PutUint32(payload[0:4], obj.id)
		copy(payload[4:], obj.extra)
		entry := make([]byte, 5+len(payload))
		entry[0] = obj.objType
		binary.LittleEndian.PutUint32(entry[1:5], uint32(len(payload)))
		copy(entry[5:], payload)
		hircBody = append(hircBody, entry...)
	}

	bank := wwiseTestChunk("BKHD", bkhdPayload)
	return append(bank, wwiseTestChunk("HIRC", hircBody)...)
}

func wwiseTestChunk(id string, payload []byte) []byte {
	chunk := make([]byte, 8+len(payload))
	copy(chunk[0:4], id)
	binary.LittleEndian.PutUint32(chunk[4:8], uint32(len(payload)))
	copy(chunk[8:], payload)
	return chunk
}

func TestAuditAudioBankDiffParsesWwiseHirc(t *testing.T) {
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
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")

	bankPath := filepath.Join(repo, "Audio/Main.bnk")
	if err := os.MkdirAll(filepath.Dir(bankPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	baseBank := buildWwiseBank(145, 9001, []testWwiseObject{
		{id: 1, objType: 2},
		{id: 2, objType: 2},
	})
	if err := os.WriteFile(bankPath, baseBank, 0o644); err != nil {
		t.Fatalf("write base bank: %v", err)
	}
	run("add", "Audio/Main.bnk")
	run("commit", "-qm", "base bank")

	targetBank := buildWwiseBank(145, 9001, []testWwiseObject{
		{id: 2, objType: 2, extra: []byte{7, 7, 7, 7}},
		{id: 3, objType: 2},
	})
	if err := os.WriteFile(bankPath, targetBank, 0o644); err != nil {
		t.Fatalf("write target bank: %v", err)
	}
	run("add", "Audio/Main.bnk")
	run("commit", "-qm", "target bank")

	res, err := GitService{}.AuditAudioBankDiff(repo, "Audio/Main.bnk", "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("audit audio bank: %v", err)
	}
	if res.EngineType != "Wwise" || res.BankName != "Main.bnk" {
		t.Fatalf("unexpected bank identity: %+v", res)
	}
	if res.BaseSizeBytes != int64(len(baseBank)) || res.TargetSizeBytes != int64(len(targetBank)) {
		t.Fatalf("expected real bank sizes %d/%d, got %d/%d", len(baseBank), len(targetBank), res.BaseSizeBytes, res.TargetSizeBytes)
	}
	if res.DeltaSizeBytes != res.TargetSizeBytes-res.BaseSizeBytes {
		t.Fatalf("delta must match byte sizes: %+v", res)
	}
	if res.TotalEventsBase != 2 || res.TotalEventsTarget != 2 {
		t.Fatalf("expected 2 HIRC objects on both sides, got %+v", res)
	}
	if len(res.AddedEvents) != 1 || res.AddedEvents[0] != "HIRC 对象 3" {
		t.Fatalf("unexpected added objects: %+v", res.AddedEvents)
	}
	if len(res.RemovedEvents) != 1 || res.RemovedEvents[0] != "HIRC 对象 1" {
		t.Fatalf("unexpected removed objects: %+v", res.RemovedEvents)
	}
	if len(res.ModifiedEvents) != 1 || res.ModifiedEvents[0].EventName != "HIRC 对象 2" {
		t.Fatalf("unexpected modified objects: %+v", res.ModifiedEvents)
	}
	if res.ModifiedEvents[0].OldSizeBytes != 4 || res.ModifiedEvents[0].NewSizeBytes != 8 {
		t.Fatalf("expected real object payload sizes 4→8, got %+v", res.ModifiedEvents[0])
	}
	if len(res.OrphanEvents) != 0 || res.OrphanEvents == nil {
		t.Fatalf("orphan events must be an empty non-nil list, got %+v", res.OrphanEvents)
	}
	if !strings.Contains(res.Summary, "BKHD v145") || !strings.Contains(res.Summary, "HIRC 对象 2 → 2") {
		t.Fatalf("summary should describe real bank header and counts: %q", res.Summary)
	}
	if !strings.Contains(res.Summary, "不伪造") || strings.Contains(res.Summary, "Play_BGM") {
		t.Fatalf("summary must not contain fabricated event names: %q", res.Summary)
	}
}

func TestAuditAudioBankDiffReportsMissingTarget(t *testing.T) {
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
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")

	bankPath := filepath.Join(repo, "Audio/Main.bnk")
	if err := os.MkdirAll(filepath.Dir(bankPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(bankPath, buildWwiseBank(145, 1, []testWwiseObject{{id: 1, objType: 2}}), 0o644); err != nil {
		t.Fatalf("write bank: %v", err)
	}
	run("add", "Audio/Main.bnk")
	run("commit", "-qm", "add bank")
	if err := os.Remove(bankPath); err != nil {
		t.Fatalf("remove bank: %v", err)
	}
	run("add", "-A")
	run("commit", "-qm", "delete bank")

	res, err := GitService{}.AuditAudioBankDiff(repo, "Audio/Main.bnk", "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("deleted bank should produce a report, not an error: %v", err)
	}
	if res.TargetSizeBytes != 0 || !strings.Contains(res.Summary, "目标 HEAD 不存在该 Bank") {
		t.Fatalf("missing target bank should be reported honestly: %+v", res)
	}

	if _, err := (GitService{}).AuditAudioBankDiff(repo, "Audio/Missing.bnk", "HEAD", "HEAD"); err == nil {
		t.Fatal("a bank missing from both refs must return an error")
	}
}

func TestAudioBankParsesFMODHeaderAndNegativeByteSizes(t *testing.T) {
	header := make([]byte, 12)
	copy(header[0:4], "FSB5")
	binary.LittleEndian.PutUint32(header[4:8], 1)
	binary.LittleEndian.PutUint32(header[8:12], 7)
	info := parseAudioBankHeader("Main.bank", header)
	if info.engine != "FMOD" || info.objectCount != 7 || !strings.Contains(info.detail, "FSB5 v1") {
		t.Fatalf("unexpected FMOD header: %+v", info)
	}
	if got := formatByteSize(-1536); got != "-1.5 KB" {
		t.Fatalf("negative sizes must keep their unit, got %q", got)
	}
}
