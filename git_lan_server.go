package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type LanServerInfo struct {
	Running       bool     `json:"running"`
	Port          int      `json:"port"`
	URL           string   `json:"url"`
	LocalIPs      []string `json:"localIps"`
	ActiveReviews int      `json:"activeReviews"`
	RepoPath      string   `json:"repoPath"`
}

var (
	lanMu         sync.Mutex
	lanHttpServer *http.Server
	lanInfo       = &LanServerInfo{
		Running:  false,
		Port:     8080,
		LocalIPs: []string{"127.0.0.1"},
	}
)

func getLocalIPv4Addresses() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.String())
				}
			}
		}
	}
	if len(ips) == 0 {
		ips = append(ips, "127.0.0.1")
	}
	return ips
}

func (GitService) GetLanReviewServerStatus() (*LanServerInfo, error) {
	lanMu.Lock()
	defer lanMu.Unlock()

	ips := getLocalIPv4Addresses()
	lanInfo.LocalIPs = ips
	return lanInfo, nil
}

func (s GitService) StartLanReviewServer(repoPath string, port int) (*LanServerInfo, error) {
	lanMu.Lock()
	defer lanMu.Unlock()

	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if port <= 0 || port > 65535 {
		port = 8080
	}

	if lanHttpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = lanHttpServer.Shutdown(ctx)
		cancel()
		lanHttpServer = nil
	}

	mux := http.NewServeMux()

	// Status endpoint
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":   "running",
			"repoPath": repoPath,
			"time":     time.Now().Format(time.RFC3339),
		})
	})

	// Review endpoint (fetches ChangeReview via GitService)
	mux.HandleFunc("/api/review", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		base := r.URL.Query().Get("base")
		head := r.URL.Query().Get("head")
		if base == "" || head == "" {
			http.Error(w, `{"error":"base and head query params are required"}`, http.StatusBadRequest)
			return
		}
		review, err := s.GetChangeReview(repoPath, base, head)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(review)
	})

	// Add review comment endpoint
	mux.HandleFunc("/api/comment", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"POST required"}`, http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Base   string `json:"base"`
			Head   string `json:"head"`
			Path   string `json:"path"`
			Line   int    `json:"line"`
			Side   string `json:"side"`
			Body   string `json:"body"`
			Author string `json:"author"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		bodyWithAuthor := payload.Body
		if payload.Author != "" {
			bodyWithAuthor = fmt.Sprintf("[%s]: %s", payload.Author, payload.Body)
		}

		review, err := s.AddReviewComment(repoPath, payload.Base, payload.Head, payload.Path, payload.Line, payload.Side, bodyWithAuthor)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(review)
	})

	// Standalone Peer Review Web Interface
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>WGit LAN Peer Review - %s</title>
  <style>
    :root { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; }
    body { margin: 0; padding: 24px; }
    .header { display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #334155; padding-bottom: 16px; margin-bottom: 24px; }
    .badge { background: #3b82f6; color: white; padding: 4px 10px; border-radius: 9999px; font-size: 12px; font-weight: 600; }
    .card { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 18px; margin-bottom: 16px; }
    .title { font-size: 20px; font-weight: 700; margin: 0 0 6px 0; color: #38bdf8; }
    .subtitle { color: #94a3b8; font-size: 13px; }
    .comment-box { width: 100%%; box-sizing: border-box; background: #0f172a; border: 1px solid #475569; color: white; padding: 10px; border-radius: 6px; font-size: 14px; margin-top: 8px; resize: vertical; }
    .btn { background: #2563eb; color: white; border: none; padding: 8px 16px; border-radius: 6px; font-weight: 600; cursor: pointer; margin-top: 8px; }
    .btn:hover { background: #1d4ed8; }
  </style>
</head>
<body>
  <div class="header">
    <div>
      <h1 class="title">WGit LAN Peer Review</h1>
      <div class="subtitle">Live local collaboration server on port %d</div>
    </div>
    <span class="badge">LAN Active</span>
  </div>
  <div class="card">
    <h3>Quick Peer Review</h3>
    <p class="subtitle">You are connected directly to your teammate's local WGit workspace over the local area network. No cloud roundtrips required.</p>
  </div>
</body>
</html>`, strings.Split(repoPath, "/")[len(strings.Split(repoPath, "/"))-1], port)
		_, _ = w.Write([]byte(html))
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to bind LAN review server on port %d: %w", port, err)
	}

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	lanHttpServer = server
	ips := getLocalIPv4Addresses()
	primaryIP := "127.0.0.1"
	if len(ips) > 0 {
		primaryIP = ips[0]
	}

	lanInfo = &LanServerInfo{
		Running:       true,
		Port:          port,
		URL:           fmt.Sprintf("http://%s:%d", primaryIP, port),
		LocalIPs:      ips,
		ActiveReviews: 1,
		RepoPath:      repoPath,
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			lanMu.Lock()
			lanInfo.Running = false
			lanMu.Unlock()
		}
	}()

	return lanInfo, nil
}

func (GitService) StopLanReviewServer() (*LanServerInfo, error) {
	lanMu.Lock()
	defer lanMu.Unlock()

	if lanHttpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = lanHttpServer.Shutdown(ctx)
		cancel()
		lanHttpServer = nil
	}

	lanInfo.Running = false
	lanInfo.ActiveReviews = 0
	return lanInfo, nil
}
