import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const inspectorFile = fileURLToPath(new URL("../src/components/history/CommitInspector.vue", import.meta.url));
const inspectorSrc = readFileSync(inspectorFile, "utf8");
const aiServiceFile = fileURLToPath(new URL("../src/services/aiService.ts", import.meta.url));
const aiServiceSrc = readFileSync(aiServiceFile, "utf8");
const gitServiceFile = fileURLToPath(new URL("../src/services/gitService.ts", import.meta.url));
const gitServiceSrc = readFileSync(gitServiceFile, "utf8");
const bindingsFile = fileURLToPath(new URL("../bindings/wgit/aiservice.ts", import.meta.url));
const bindingsSrc = readFileSync(bindingsFile, "utf8");

const dict = (m: object) => m as Record<string, string>;

describe("AI commit summary (F2, flag aiSummary)", () => {
  test("backend RPC is exposed on AiService bindings and routed by full name", () => {
    expect(bindingsSrc).toContain("SummarizeCommit");
    expect(gitServiceSrc).toContain('Call.ByName("main.AiService.SummarizeCommit", repoPath, commitHash)');
    expect(aiServiceSrc).toContain("export function summarizeCommit(repoPath: string, commitHash: string)");
  });

  test("inspector button and card are gated by FeatureGate flag=aiSummary", () => {
    expect(inspectorSrc).toContain('<FeatureGate flag="aiSummary">');
    expect(inspectorSrc).toContain('data-testid="ai-summary-btn"');
    expect(inspectorSrc).toContain('data-testid="ai-summary-card"');
    expect(inspectorSrc).toContain('@click="generateAiSummary"');
  });

  test("summary responses are scoped to the selected commit", () => {
    expect(inspectorSrc).toContain("() => [props.selectedCommit?.hash, props.repoPath] as const");
    expect(inspectorSrc).toContain("let aiSummarySeq = 0");
    expect(inspectorSrc).toContain('kind: "summary"');
    expect(inspectorSrc).toContain("if (seq !== aiSummarySeq || props.selectedCommit?.hash !== hash) return;");
    expect(inspectorSrc).toContain("aiSummaryError.value = \"\"");
    expect(inspectorSrc).toContain('role="alert"');
    expect(inspectorSrc).toContain('aria-controls="ai-summary-body"');
    expect(inspectorSrc).toContain("aiSummary.value += delta");
    expect(inspectorSrc).toContain('data-testid="ai-summary-cancel"');
    expect(inspectorSrc).toContain("session.cancel()");
  });

  test("not-configured path reuses the existing ai.notconfigured key via toast", () => {
    expect(inspectorSrc).toContain('pushToast(t("ai.notconfigured")');
    expect(typeof dict(zhCN)["ai.notconfigured"]).toBe("string");
  });

  test("new i18n keys exist in both locales and keep the {message} placeholder", () => {
    for (const key of ["ai.summarize", "ai.summarizing", "ai.summary.failed", "ai.summary.retry", "ai.summary.card.title"]) {
      expect(typeof dict(zhCN)[key]).toBe("string");
      expect(dict(zhCN)[key].length).toBeGreaterThan(0);
      expect(typeof dict(enUS)[key]).toBe("string");
      expect(dict(enUS)[key].length).toBeGreaterThan(0);
    }
    expect(dict(zhCN)["ai.summary.failed"]).toContain("{message}");
    expect(dict(enUS)["ai.summary.failed"]).toContain("{message}");
  });
});
