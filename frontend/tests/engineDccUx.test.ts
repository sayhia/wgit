import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const branchesViewFile = fileURLToPath(new URL("../src/components/BranchesView.vue", import.meta.url));
const assetDiffModalFile = fileURLToPath(new URL("../src/components/assets/AssetDiffModal.vue", import.meta.url));
const gitServiceFile = fileURLToPath(new URL("../src/services/gitService.ts", import.meta.url));

describe("Phase E3: Game Engine & DCC External Toolchain Integration UX", () => {
  describe("gitService RPC Surface", () => {
    const src = readFileSync(gitServiceFile, "utf8");

    test("exposes detectRunningGameEngines RPC method", () => {
      expect(src).toContain("detectRunningGameEngines:");
      expect(src).toContain('"DetectRunningGameEngines"');
    });

    test("exposes launchExternalDiffTool RPC method", () => {
      expect(src).toContain("launchExternalDiffTool:");
      expect(src).toContain('"LaunchExternalDiffTool"');
    });
  });

  describe("BranchesView Engine Guard Integration", () => {
    const src = readFileSync(branchesViewFile, "utf8");

    test("detects running game engines before checkout", () => {
      expect(src).toContain("gitService.detectRunningGameEngines");
      expect(src).toContain("engineStatus.canSafelyCheckout");
      expect(src).toContain("engineStatus.warning");
      expect(src).toContain("engineStatus.recommendation");
    });
  });

  describe("AssetDiffModal External DCC Diff Launch", () => {
    const src = readFileSync(assetDiffModalFile, "utf8");

    test("renders external diff launch button in header", () => {
      expect(src).toContain("assets.diff.external");
      expect(src).toContain("assets.diff.external.hint");
      expect(src).toContain('@click="launchExternal"');
    });

    test("implements launchExternal calling launchExternalDiffTool", () => {
      expect(src).toContain("launchExternal");
      expect(src).toContain("gitService.launchExternalDiffTool");
    });
  });

  describe("i18n Parity for Phase E3", () => {
    const requiredKeys = [
      "branch.engine.guard.title",
      "assets.diff.external",
      "assets.diff.external.hint",
    ];

    test("zh-CN and en-US contain all required Phase E3 keys", () => {
      for (const k of requiredKeys) {
        expect(zhCN[k as keyof typeof zhCN]).toBeDefined();
        expect(enUS[k as keyof typeof enUS]).toBeDefined();
      }
    });
  });
});
