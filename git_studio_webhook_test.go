package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManageStudioWebhooksStoresRealConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q"); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, out)
	}
	svc := GitService{}
	empty, err := svc.ManageStudioWebhooks(repo, "list", "", "", "")
	if err != nil {
		t.Fatalf("list webhooks failed: %v", err)
	}
	if empty.TotalWebhooks != 0 || len(empty.Webhooks) != 0 || empty.LastDispatchedMessage != "" {
		t.Fatalf("empty repo must not fabricate webhooks: %+v", empty)
	}
	if !strings.Contains(empty.Message, "不会发送网络请求") {
		t.Fatalf("message should state local-only behavior: %q", empty.Message)
	}

	added, err := svc.ManageStudioWebhooks(repo, "ADD", "FEISHU", "https://open.feishu.cn/open-apis/bot/v2/hook/secret-token", "COMMIT")
	if err != nil {
		t.Fatalf("add webhook failed: %v", err)
	}
	if added.TotalWebhooks != 1 || len(added.Webhooks) != 1 || added.Webhooks[0].LastPingSuccess {
		t.Fatalf("unexpected added webhook: %+v", added)
	}
	if strings.Contains(added.Webhooks[0].UrlMasked, "secret-token") {
		t.Fatalf("webhook URL should be masked: %+v", added.Webhooks[0])
	}
	if len(added.Webhooks[0].EventRules) != 1 || added.Webhooks[0].EventRules[0] != "COMMIT" {
		t.Fatalf("event rules should be persisted: %+v", added.Webhooks[0])
	}
	if got := maskWebhookURL("https://x"); got != "***" {
		t.Fatalf("short webhook URL must be fully masked, got %q", got)
	}
	if _, err := svc.ManageStudioWebhooks(repo, "ADD", "FEISHU", "ftp://invalid", "COMMIT"); err == nil {
		t.Fatal("non-http(s) webhook URL must be rejected")
	}
	data, err := os.ReadFile(filepath.Join(repo, ".git", "studio_webhooks.json"))
	if err != nil || strings.Contains(string(data), "secret-token") {
		t.Fatalf("persisted config should be masked: %q err=%v", data, err)
	}

	listed, err := svc.ManageStudioWebhooks(repo, "list", "", "", "")
	if err != nil || listed.TotalWebhooks != 1 || listed.Webhooks[0].WebhookType != "FEISHU" {
		t.Fatalf("persisted webhook not loaded: %+v err=%v", listed, err)
	}

	if _, err := svc.ManageStudioWebhooks(repo, "REMOVE", "wh-missing", "", ""); err == nil {
		t.Fatal("removing an unknown webhook must fail")
	}
	removed, err := svc.ManageStudioWebhooks(repo, "REMOVE", added.Webhooks[0].ID, "", "")
	if err != nil || removed.TotalWebhooks != 0 || len(removed.Webhooks) != 0 {
		t.Fatalf("webhook removal failed: %+v err=%v", removed, err)
	}
	if _, err := svc.ManageStudioWebhooks(repo, "REMOVE", "", "", ""); err == nil {
		t.Fatal("removing without an id must fail")
	}
	if _, err := svc.ManageStudioWebhooks(repo, "NOT_AN_ACTION", "", "", ""); err == nil {
		t.Fatal("unknown webhook actions must fail instead of silently listing")
	}

	corruptPath := filepath.Join(repo, ".git", "studio_webhooks.json")
	corrupt := []byte(`[{"id":`)
	if err := os.WriteFile(corruptPath, corrupt, 0o644); err != nil {
		t.Fatalf("write corrupt webhook config: %v", err)
	}
	if _, err := svc.ManageStudioWebhooks(repo, "list", "", "", ""); err == nil || !strings.Contains(err.Error(), "损坏") {
		t.Fatalf("list must report a corrupt webhook config instead of an empty list, got %v", err)
	}
	if _, err := svc.ManageStudioWebhooks(repo, "ADD", "FEISHU", "https://example.com/hook", "PUSH"); err == nil || !strings.Contains(err.Error(), "损坏") {
		t.Fatalf("ADD must refuse to overwrite a corrupt webhook config, got %v", err)
	}
	after, err := os.ReadFile(corruptPath)
	if err != nil || string(after) != string(corrupt) {
		t.Fatalf("corrupt webhook config must stay untouched, got %q err=%v", after, err)
	}
}
