import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const qualitySectionFile = fileURLToPath(
  new URL("../src/components/tools/EngineQualitySection.vue", import.meta.url)
);
const engineTabFile = fileURLToPath(
  new URL("../src/components/tools/EngineTab.vue", import.meta.url)
);

describe("Game Engine & Digital Assets UX (Round 15)", () => {
  const qualitySrc = readFileSync(qualitySectionFile, "utf8");
  const engineSrc = readFileSync(engineTabFile, "utf8");

  describe("DCC Lock Activity Actions & Auto-lock Guard", () => {
    test("contains quick lock handler setting dccAssetPath and auto-locking", () => {
      expect(qualitySrc).toContain("handleQuickLockDcc");
      expect(qualitySrc).toContain("dccAssetPath.value = path");
      expect(qualitySrc).toContain("handleAutoLockDcc()");
    });

    test("contains copyDccPath clipboard helper", () => {
      expect(qualitySrc).toContain("copyDccPath");
      expect(qualitySrc).toContain("navigator.clipboard.writeText(path)");
      expect(qualitySrc).toContain('t("eq.toast.dcc.path.copied")');
    });

    test("renders 1-click lock and copy buttons in DCC lock activity list", () => {
      expect(qualitySrc).toContain("handleQuickLockDcc(item.sourceAssetPath)");
      expect(qualitySrc).toContain("copyDccPath(item.sourceAssetPath || item.lockFilePath)");
      expect(qualitySrc).toContain("eq.dcc.item.lock");
      expect(qualitySrc).toContain("eq.dcc.copy.path");
    });
  });

  describe("Unity Meta Pairing & Inspection", () => {
    test("renders copy asset path and copy meta path buttons on mismatch items", () => {
      expect(engineSrc).toContain("copyPath(m.assetPath)");
      expect(engineSrc).toContain("copyPath(m.metaPath)");
      expect(engineSrc).toContain("engine.copy.asset.path");
      expect(engineSrc).toContain("engine.copy.meta.path");
    });

    test("contains handleAutoFixMeta and copyPath functions", () => {
      expect(engineSrc).toContain("handleAutoFixMeta");
      expect(engineSrc).toContain("copyPath");
      expect(engineSrc).toContain('t("engine.toast.copied"');
    });
  });

  describe("Unified audit action briefs", () => {
    test("exports every completed read-only audit as one Markdown bundle", () => {
      expect(qualitySrc).toContain("loadedAudits");
      expect(qualitySrc).toContain("exportAllAuditPlans");
      expect(qualitySrc).toContain("buildAuditBundle");
      expect(qualitySrc).toContain("auditSectionFromResult");
      expect(qualitySrc).toContain('data-testid="export-all-audit-plans"');
      expect(qualitySrc).toContain('t("eq.audit.plan.exportall.done"');
      expect(qualitySrc).not.toContain("writeFile"); // export only, never auto-fix
    });

    test("the action-brief labels exist in both catalogs", () => {
      for (const key of [
        "eq.audit.plan.exportall",
        "eq.audit.plan.exportall.done",
        "eq.audit.plan.exportall.none",
        "eq.audit.plan.exportall.hint",
      ]) {
        expect((zhCN as Record<string, string>)[key]).toBeTruthy();
        expect((enUS as Record<string, string>)[key]).toBeTruthy();
      }
    });
  });

  describe("i18n completeness for Game Engine & Assets", () => {
    const requiredKeys = [
      "eq.dcc.item.lock",
      "eq.dcc.item.lock.title",
      "eq.dcc.copy.path",
      "eq.toast.dcc.path.copied",
      "eq.toast.copy.failed",
      "engine.copy.asset.path",
      "engine.copy.meta.path",
      "engine.toast.copied",
      "engine.toast.copy.failed",
    ];

    test("zh-CN contains all required Round 15 keys", () => {
      for (const k of requiredKeys) {
        expect((zhCN as Record<string, string>)[k]).toBeDefined();
        expect((zhCN as Record<string, string>)[k].length).toBeGreaterThan(0);
      }
    });

    test("en-US contains all required Round 15 keys", () => {
      for (const k of requiredKeys) {
        expect((enUS as Record<string, string>)[k]).toBeDefined();
        expect((enUS as Record<string, string>)[k].length).toBeGreaterThan(0);
      }
    });
  });
});
