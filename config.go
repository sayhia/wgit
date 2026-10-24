package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type AppConfig struct {
	RecentRepos []string `json:"recentRepos"`
	LastRepo    string   `json:"lastRepo"`
	Theme       string   `json:"theme"` // "dark", "light", or "system"
}

func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "wgit-config.json"
	}
	dir := filepath.Join(home, ".config", "wgit")
	_ = os.MkdirAll(dir, 0755)
	target := filepath.Join(dir, "config.json")

	// Backward-compatibility: if ~/.config/wgit/config.json doesn't exist, check ~/.config/ugit/config.json
	if _, err := os.Stat(target); os.IsNotExist(err) {
		oldPath := filepath.Join(home, ".config", "ugit", "config.json")
		if oldData, err := os.ReadFile(oldPath); err == nil {
			_ = os.WriteFile(target, oldData, 0644)
		}
	}
	return target
}

func LoadConfig() AppConfig {
	cfg := AppConfig{
		RecentRepos: []string{},
		Theme:       "dark",
	}
	path := getConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Preserve the unreadable file so the next SaveConfig (e.g. from
		// AddRecentRepo) cannot silently destroy the user's recent-repo list.
		backup := fmt.Sprintf("%s.corrupt-%d", path, time.Now().UnixNano())
		_ = os.Rename(path, backup)
		return AppConfig{RecentRepos: []string{}, Theme: "dark"}
	}
	return cfg
}

func SaveConfig(cfg AppConfig) {
	path := getConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, data, 0644)
	}
}

func AddRecentRepo(repoPath string) {
	cfg := LoadConfig()
	cleaned := filepath.Clean(repoPath)

	var list []string
	list = append(list, cleaned)
	for _, p := range cfg.RecentRepos {
		if p != cleaned && IsGitRepo(p) {
			list = append(list, p)
		}
		if len(list) >= 15 {
			break
		}
	}
	cfg.RecentRepos = list
	cfg.LastRepo = cleaned
	SaveConfig(cfg)
}
