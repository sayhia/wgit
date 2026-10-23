package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectAudioFileDoesNotFabricateCompressedMetadata(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "clip.mp3"), []byte("ID3 not parsed"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GitService{}.InspectAudioFile(repo, "clip.mp3")
	if err != nil {
		t.Fatalf("inspect mp3: %v", err)
	}
	if res.MetadataKnown || res.SampleRate != 0 || res.Channels != 0 {
		t.Fatalf("compressed audio must not inherit 44.1kHz/stereo defaults: %+v", res)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("unknown metadata must not produce a 44.1kHz warning: %+v", res.Warnings)
	}
	if !strings.Contains(res.Message, "未作 48kHz 规范判定") {
		t.Fatalf("message should be explicit about missing metadata: %q", res.Message)
	}

	if err := os.WriteFile(filepath.Join(repo, "broken.wav"), []byte("not a wave file"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = GitService{}.InspectAudioFile(repo, "broken.wav")
	if err != nil {
		t.Fatalf("inspect broken wav: %v", err)
	}
	if res.MetadataKnown || res.SampleRate != 0 || len(res.Warnings) != 0 {
		t.Fatalf("unparseable WAV must not be reported as 44.1kHz: %+v", res)
	}
	if !strings.Contains(res.Message, "未能从 WAV 直接解析") {
		t.Fatalf("broken WAV message should explain the limitation: %q", res.Message)
	}
}
