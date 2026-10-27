package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type LanLfsPeer struct {
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	URL         string `json:"url"`
	Active      bool   `json:"active"`
	RepoName    string `json:"repoName"`
	CachedBlobs int    `json:"cachedBlobs"`
	LatencyMs   int64  `json:"latencyMs"`
}

type LanLfsPeerStatus struct {
	Peers           []LanLfsPeer `json:"peers"`
	LocalLfsServing bool         `json:"localLfsServing"`
	LocalPort       int          `json:"localPort"`
	LocalBlobCount  int          `json:"localBlobCount"`
	TotalLocalBytes int64        `json:"totalLocalBytes"`
	TotalLocalStr   string       `json:"totalLocalStr"`
}

type LanLfsTransferResult struct {
	Success      bool    `json:"success"`
	Oid          string  `json:"oid"`
	BytesCopied  int64   `json:"bytesCopied"`
	DurationMs   int64   `json:"durationMs"`
	SpeedMBps    float64 `json:"speedMBps"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
}

type LanLfsShareConfig struct {
	Enabled   bool   `json:"enabled"`
	Port      int    `json:"port"`
	SharePath string `json:"sharePath"`
	UpdatedAt string `json:"updatedAt"`
}

var validOidRegex = regexp.MustCompile(`^[0-9a-fA-F]{4,128}$`)

func registerLanLfsEndpoints(mux *http.ServeMux, repoPath string) {
	mux.HandleFunc("/api/lfs/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		blobCount, totalBytes := getLfsStats(repoPath)
		repoName := filepath.Base(repoPath)

		_ = json.NewEncoder(w).Encode(map[string]any{
			"lfs":         true,
			"repoName":    repoName,
			"cachedBlobs": blobCount,
			"totalBytes":  totalBytes,
		})
	})

	mux.HandleFunc("/api/lfs/blob", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		oid := strings.TrimSpace(r.URL.Query().Get("oid"))
		if oid == "" || !validOidRegex.MatchString(oid) {
			http.Error(w, `{"error":"invalid oid"}`, http.StatusBadRequest)
			return
		}

		blobPath := filepath.Join(repoPath, ".git", "lfs", "objects", oid[:2], oid[2:4], oid)
		info, err := os.Stat(blobPath)
		if err != nil || info.IsDir() {
			http.Error(w, `{"error":"blob not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", oid))
		http.ServeFile(w, r, blobPath)
	})
}

func getLfsStats(repoPath string) (int, int64) {
	lfsDir := filepath.Join(repoPath, ".git", "lfs", "objects")
	var count int
	var totalBytes int64

	_ = filepath.Walk(lfsDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			count++
			totalBytes += info.Size()
		}
		return nil
	})
	return count, totalBytes
}

func (GitService) DetectLanLfsPeers(repoPath string) (*LanLfsPeerStatus, error) {
	blobCount, totalBytes := getLfsStats(repoPath)

	status := &LanLfsPeerStatus{
		Peers:           []LanLfsPeer{},
		LocalLfsServing: false,
		LocalPort:       8080,
		LocalBlobCount:  blobCount,
		TotalLocalBytes: totalBytes,
		TotalLocalStr:   formatBytes(totalBytes),
	}

	lanMu.Lock()
	if lanInfo != nil {
		status.LocalLfsServing = lanInfo.Running
		status.LocalPort = lanInfo.Port
	}
	lanMu.Unlock()

	// Probes localhost and standard peer port
	client := http.Client{Timeout: 300 * time.Millisecond}
	candidates := []string{
		fmt.Sprintf("http://127.0.0.1:%d", status.LocalPort),
	}

	ips := getLocalIPv4Addresses()
	for _, ip := range ips {
		if ip != "127.0.0.1" {
			candidates = append(candidates, fmt.Sprintf("http://%s:%d", ip, status.LocalPort))
		}
	}

	var peerWg sync.WaitGroup
	var peerMu sync.Mutex

	for _, cand := range candidates {
		candURL := cand
		peerWg.Add(1)
		go func() {
			defer peerWg.Done()
			start := time.Now()
			resp, err := client.Get(candURL + "/api/lfs/status")
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var res struct {
						Lfs         bool   `json:"lfs"`
						RepoName    string `json:"repoName"`
						CachedBlobs int    `json:"cachedBlobs"`
					}
					if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && res.Lfs {
						latency := time.Since(start).Milliseconds()
						peerMu.Lock()
						status.Peers = append(status.Peers, LanLfsPeer{
							IP:          strings.TrimPrefix(strings.Split(candURL, ":")[1], "//"),
							Port:        status.LocalPort,
							URL:         candURL,
							Active:      true,
							RepoName:    res.RepoName,
							CachedBlobs: res.CachedBlobs,
							LatencyMs:   latency,
						})
						peerMu.Unlock()
					}
				}
			}
		}()
	}
	peerWg.Wait()

	return status, nil
}

func (GitService) SyncLanLfsBlob(repoPath string, peerUrl string, oid string) (*LanLfsTransferResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	oid = strings.TrimSpace(oid)
	if !validOidRegex.MatchString(oid) {
		return nil, errors.New("invalid LFS OID")
	}
	if peerUrl == "" {
		return nil, errors.New("peerUrl cannot be empty")
	}

	peerUrl = strings.TrimRight(peerUrl, "/")
	downloadUrl := fmt.Sprintf("%s/api/lfs/blob?oid=%s", peerUrl, oid)

	startTime := time.Now()
	client := http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(downloadUrl)
	if err != nil {
		return &LanLfsTransferResult{
			Success:      false,
			Oid:          oid,
			ErrorMessage: err.Error(),
		}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("peer returned HTTP %d", resp.StatusCode)
		return &LanLfsTransferResult{
			Success:      false,
			Oid:          oid,
			ErrorMessage: err.Error(),
		}, err
	}

	targetDir := filepath.Join(repoPath, ".git", "lfs", "objects", oid[:2], oid[2:4])
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lfs directory: %w", err)
	}

	targetPath := filepath.Join(targetDir, oid)
	tmpPath := targetPath + ".tmp"

	outFile, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(outFile, hasher)
	written, err := io.Copy(writer, resp.Body)
	_ = outFile.Close()

	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("transfer stream interrupted: %w", err)
	}

	computedOid := hex.EncodeToString(hasher.Sum(nil))
	if len(oid) == 64 && !strings.EqualFold(computedOid, oid) {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", oid, computedOid)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to finalize lfs blob: %w", err)
	}

	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()
	if durationMs == 0 {
		durationMs = 1
	}

	speedMBps := (float64(written) / (1024 * 1024)) / (float64(durationMs) / 1000.0)

	return &LanLfsTransferResult{
		Success:     true,
		Oid:         oid,
		BytesCopied: written,
		DurationMs:  durationMs,
		SpeedMBps:   speedMBps,
	}, nil
}

func (s GitService) ConfigureLanLfsSharing(repoPath string, enable bool, port int) (*LanLfsShareConfig, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if port <= 0 || port > 65535 {
		port = 8080
	}

	if enable {
		_, err := s.StartLanReviewServer(repoPath, port)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := s.StopLanReviewServer()
		if err != nil {
			return nil, err
		}
	}

	cfg := &LanLfsShareConfig{
		Enabled:   enable,
		Port:      port,
		SharePath: filepath.Join(repoPath, ".git", "lfs", "objects"),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	cfgPath := filepath.Join(repoPath, ".git", "wgit_lan_lfs.json")
	if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		_ = os.WriteFile(cfgPath, data, 0644)
	}

	return cfg, nil
}
