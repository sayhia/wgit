package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLanEventBusBroadcast(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	testPort := 18992
	_, err := svc.StartLanReviewServer(repo, testPort)
	if err != nil {
		t.Fatalf("StartLanReviewServer failed: %v", err)
	}
	defer func() {
		_, _ = svc.StopLanReviewServer()
	}()

	time.Sleep(50 * time.Millisecond)

	// Connect SSE stream
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/events", testPort), nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect SSE failed: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	// Read initial connected line
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading sse line failed: %v", err)
	}
	if !strings.HasPrefix(line, "data: ") {
		t.Fatalf("expected data: prefix, got: %s", line)
	}

	// Now broadcast a custom event
	if err := svc.BroadcastLanReviewEvent("live_ping", "peer_ack"); err != nil {
		t.Fatalf("BroadcastLanReviewEvent failed: %v", err)
	}

	// Check if event arrives
	receivedPing := false
	for i := 0; i < 5; i++ {
		l, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(l, "live_ping") {
			receivedPing = true
			break
		}
	}
	if !receivedPing {
		t.Errorf("expected to receive live_ping event over SSE stream")
	}
}

func TestLanLfsSharingAndBlobSync(t *testing.T) {
	svc := GitService{}
	sourceRepo := t.TempDir()
	destRepo := t.TempDir()

	if _, err := svc.InitRepo(sourceRepo); err != nil {
		t.Fatalf("InitRepo source failed: %v", err)
	}
	if _, err := svc.InitRepo(destRepo); err != nil {
		t.Fatalf("InitRepo dest failed: %v", err)
	}

	// Create dummy LFS blob in source repo
	dummyData := []byte("WGit P2P LFS Accelerated Binary Chunk Data 123456789")
	hash := sha256.Sum256(dummyData)
	oid := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(sourceRepo, ".git", "lfs", "objects", oid[:2], oid[2:4])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		t.Fatalf("failed to create blob dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(blobDir, oid), dummyData, 0644); err != nil {
		t.Fatalf("failed to write dummy blob: %v", err)
	}

	testPort := 18993
	info, err := svc.StartLanReviewServer(sourceRepo, testPort)
	if err != nil {
		t.Fatalf("StartLanReviewServer failed: %v", err)
	}
	defer func() {
		_, _ = svc.StopLanReviewServer()
	}()

	time.Sleep(50 * time.Millisecond)

	// 1. Detect LAN peers
	peerStatus, err := svc.DetectLanLfsPeers(sourceRepo)
	if err != nil {
		t.Fatalf("DetectLanLfsPeers failed: %v", err)
	}
	if peerStatus.LocalBlobCount < 1 {
		t.Errorf("expected LocalBlobCount >= 1, got %d", peerStatus.LocalBlobCount)
	}
	if !peerStatus.LocalLfsServing {
		t.Errorf("expected LocalLfsServing to be true")
	}

	// 2. Sync blob from source to dest repo
	peerUrl := fmt.Sprintf("http://127.0.0.1:%d", info.Port)
	res, err := svc.SyncLanLfsBlob(destRepo, peerUrl, oid)
	if err != nil {
		t.Fatalf("SyncLanLfsBlob failed: %v", err)
	}
	if !res.Success || res.BytesCopied != int64(len(dummyData)) {
		t.Errorf("unexpected sync result: %+v", res)
	}

	// 3. Verify copied blob in destination repo
	destBlobPath := filepath.Join(destRepo, ".git", "lfs", "objects", oid[:2], oid[2:4], oid)
	copiedData, err := os.ReadFile(destBlobPath)
	if err != nil {
		t.Fatalf("failed to read copied blob: %v", err)
	}
	if string(copiedData) != string(dummyData) {
		t.Fatalf("blob content mismatch: expected %s, got %s", string(dummyData), string(copiedData))
	}
}

func TestConfigureLanLfsSharing(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	cfg, err := svc.ConfigureLanLfsSharing(repo, true, 18994)
	if err != nil {
		t.Fatalf("ConfigureLanLfsSharing(true) failed: %v", err)
	}
	if !cfg.Enabled || cfg.Port != 18994 {
		t.Errorf("unexpected config: %+v", cfg)
	}

	// Disable
	cfgDisabled, err := svc.ConfigureLanLfsSharing(repo, false, 18994)
	if err != nil {
		t.Fatalf("ConfigureLanLfsSharing(false) failed: %v", err)
	}
	if cfgDisabled.Enabled {
		t.Errorf("expected Enabled to be false")
	}
}
