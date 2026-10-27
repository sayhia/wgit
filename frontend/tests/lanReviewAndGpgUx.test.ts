import { describe, expect, test } from "bun:test";
import { gitService } from "../src/services/gitService";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import { readFileSync, existsSync } from "fs";
import { resolve } from "path";

describe("Phase E5: LAN Peer Review & Enterprise Security UX", () => {
  test("gitService exposes all 6 LAN review and enterprise compliance RPCs", () => {
    expect(typeof gitService.getLanReviewServerStatus).toBe("function");
    expect(typeof gitService.startLanReviewServer).toBe("function");
    expect(typeof gitService.stopLanReviewServer).toBe("function");
    expect(typeof gitService.saveRepoComplianceRules).toBe("function");
    expect(typeof gitService.inspectRepoCompliance).toBe("function");
    expect(typeof gitService.validateCommitPolicy).toBe("function");
  });

  test("LanReviewModal component exists, has clean template and zero hardcoded Chinese copy", () => {
    const modalPath = resolve(__dirname, "../src/components/review/LanReviewModal.vue");
    expect(existsSync(modalPath)).toBe(true);

    const src = readFileSync(modalPath, "utf-8");
    expect(src).toContain("review.lan.modal.title");
    expect(src).toContain("review.lan.port.label");
    expect(src).toContain("review.lan.status.title");
    expect(src).toContain("gitService.getLanReviewServerStatus");
    expect(src).toContain("gitService.startLanReviewServer");
    expect(src).toContain("gitService.stopLanReviewServer");

    // Template zero hardcoded Chinese check
    const templateMatch = src.match(/<template>([\s\S]*?)<\/template>/);
    const template = templateMatch ? templateMatch[1] : "";
    const stripped = template
      .replace(/t\(['"][^'"]+['"]\)/g, "")
      .replace(/<!--[\s\S]*?-->/g, "");

    const chineseRegex = /[\u4e00-\u9fa5]/;
    expect(chineseRegex.test(stripped)).toBe(false);
  });

  test("ReviewView wires LanReviewModal and header button", () => {
    const reviewPath = resolve(__dirname, "../src/components/ReviewView.vue");
    const src = readFileSync(reviewPath, "utf-8");

    expect(src).toContain("LanReviewModal");
    expect(src).toContain("review.lan.btn.title");
    expect(src).toContain("showLanModal");
    expect(src).toContain("isLanServerRunning");
    expect(src).toContain("checkLanStatus");
  });

  test("SigningSection integrates Enterprise Compliance rules (.wgit-rules.json)", () => {
    const signingPath = resolve(__dirname, "../src/components/settings/SigningSection.vue");
    const src = readFileSync(signingPath, "utf-8");

    expect(src).toContain("settings.compliance.title");
    expect(src).toContain("complianceRules");
    expect(src).toContain("saveComplianceRules");
    expect(src).toContain("gitService.inspectRepoCompliance");
    expect(src).toContain("gitService.saveRepoComplianceRules");
    expect(src).toContain("directCommitBranchesInput");
  });

  test("WorkspaceView checks commit policy before committing", () => {
    const wsPath = resolve(__dirname, "../src/components/WorkspaceView.vue");
    const src = readFileSync(wsPath, "utf-8");

    expect(src).toContain("gitService.validateCommitPolicy");
    expect(src).toContain("ws.compliance.policy.blocked");
  });

  test("i18n catalogs contain complete LAN review and compliance keys in both languages", () => {
    const requiredKeys = [
      "review.lan.btn.title",
      "review.lan.modal.title",
      "review.lan.status.title",
      "review.lan.status.running",
      "review.lan.status.stopped",
      "review.lan.status.desc",
      "review.lan.btn.start",
      "review.lan.btn.stop",
      "review.lan.port.label",
      "review.lan.port.hint",
      "review.lan.link.label",
      "review.lan.ips.title",
      "review.lan.notice",
      "review.lan.toast.started",
      "review.lan.toast.stopped",
      "review.lan.toast.failed",
      "review.lan.toast.copied",
      "settings.compliance.title",
      "settings.compliance.desc",
      "settings.compliance.file.active",
      "settings.compliance.file.default",
      "settings.compliance.enforce.sign",
      "settings.compliance.enforce.sign.desc",
      "settings.compliance.msg.pattern",
      "settings.compliance.protected.branches",
      "settings.compliance.email.domain",
      "settings.compliance.save.btn",
      "settings.compliance.saved",
      "settings.compliance.save.failed",
      "ws.compliance.policy.blocked",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]).toBeDefined();
    }
  });
});
