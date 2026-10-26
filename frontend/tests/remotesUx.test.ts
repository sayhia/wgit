import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const remotesTabFile = fileURLToPath(
  new URL("../src/components/tools/RemotesTab.vue", import.meta.url)
);

describe("Multi-Remote Sync & Network Credentials UX (Round 11)", () => {
  const src = readFileSync(remotesTabFile, "utf8");

  describe("Toolbar Actions & Batch Sync", () => {
    test("provides handlePushAllRemotes and push-all action", () => {
      expect(src).toContain("handlePushAllRemotes");
      expect(src).toContain("gitService.syncMultiRemotes(props.currentRepoPath, \"push-all\")");
      expect(src).toContain("remotes.push.all");
      expect(src).toContain("remotes.confirm.push.all");
    });

    test("provides handleFetchAllRemotes and fetch-all action", () => {
      expect(src).toContain("handleFetchAllRemotes");
      expect(src).toContain("gitService.syncMultiRemotes(props.currentRepoPath, \"fetch-all\")");
      expect(src).toContain("remotes.fetch.all");
    });

    test("provides credentials and SSH diagnosis button in toolbar", () => {
      expect(src).toContain("openAuthDiagModal()");
      expect(src).toContain("remotes.auth.diag");
    });
  });

  describe("Card Actions & Remote URL Protocol Switching", () => {
    test("contains copy URL and per-remote SSH diagnosis on card", () => {
      expect(src).toContain("copyToClipboard(r.url, r.name)");
      expect(src).toContain("remotes.action.copy.url");
      expect(src).toContain("openAuthDiagModal(extractHostFromUrl(r.url))");
      expect(src).toContain("remotes.action.diagnose");
    });

    test("contains convertRemoteUrlProtocol helper supporting SSH and HTTPS switching", () => {
      expect(src).toContain("convertRemoteUrlProtocol");
      expect(src).toContain("extractHostFromUrl");
      expect(src).toContain("convertRemoteUrlProtocol(newRemoteUrl, 'ssh')");
      expect(src).toContain("convertRemoteUrlProtocol(newRemoteUrl, 'https')");
      expect(src).toContain("convertRemoteUrlProtocol(editRemoteUrl, 'ssh')");
      expect(src).toContain("convertRemoteUrlProtocol(editRemoteUrl, 'https')");
    });
  });

  describe("Credentials & SSH Diagnostic Modal", () => {
    test("declares showAuthDiagModal with host input and result inspection", () => {
      expect(src).toContain("showAuthDiagModal");
      expect(src).toContain("handleRunAuthDiag");
      expect(src).toContain("gitService.diagnoseGitAuth(props.currentRepoPath, targetHost)");
      expect(src).toContain("authDiagResult.sshKeysFound");
      expect(src).toContain("authDiagResult.credentialHelper");
      expect(src).toContain("authDiagResult.sshAuthOutput");
    });
  });

  describe("i18n key completeness for Remotes & Credentials", () => {
    const requiredKeys = [
      "remotes.push.all",
      "remotes.push.all.title",
      "remotes.confirm.push.all",
      "remotes.toast.push.all.done",
      "remotes.toast.push.all.failed.short",
      "remotes.toast.push.all.failed",
      "remotes.auth.diag",
      "remotes.auth.diag.title",
      "remotes.auth.modal.title",
      "remotes.auth.test",
      "remotes.auth.testing",
      "remotes.auth.status.ok",
      "remotes.auth.status.warn",
      "remotes.auth.keys.found",
      "remotes.auth.not.configured",
      "remotes.auth.output",
      "remotes.action.copy.url",
      "remotes.action.diagnose",
      "remotes.modal.convert.protocol",
      "remotes.toast.auth.ok",
      "remotes.toast.auth.warn",
      "remotes.toast.auth.failed",
    ];

    test("zh-CN contains all required keys", () => {
      const catalog = zhCN as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].length).toBeGreaterThan(0);
      }
    });

    test("en-US contains all required keys", () => {
      const catalog = enUS as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].length).toBeGreaterThan(0);
      }
    });
  });
});
