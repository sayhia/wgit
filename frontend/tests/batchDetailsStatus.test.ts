import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const historyViewFile = fileURLToPath(
  new URL("../src/components/HistoryView.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(new URL("../src/services/gitService.ts", import.meta.url));
const zhFile = fileURLToPath(new URL("../src/i18n/messages/zh-CN.ts", import.meta.url));
const enFile = fileURLToPath(new URL("../src/i18n/messages/en-US.ts", import.meta.url));

const PARTIAL_KEY = "history.toast.details.partial.failed";

describe("Batch copy details switches to GetCommitDetailsBatchWithStatus", () => {
  const src = readFileSync(historyViewFile, "utf8");

  test("handleCopyBatchDetails calls getCommitDetailsBatchWithStatus", () => {
    expect(src).toContain("gitService.getCommitDetailsBatchWithStatus(");
  });

  test("lossy getCommitDetailsBatch is no longer used in HistoryView", () => {
    // "BatchWithStatus(" does not contain "Batch(", so this is exact.
    expect(src).not.toContain("gitService.getCommitDetailsBatch(");
  });

  test("gitService keeps the RPC-parity WithStatus wrapper", () => {
    const svc = readFileSync(gitServiceFile, "utf8");
    expect(svc).toContain('callBackend<CommitDetailsBatchResult>("GetCommitDetailsBatchWithStatus"');
  });
});

describe("Partial-failure branch wiring in handleCopyBatchDetails", () => {
  const src = readFileSync(historyViewFile, "utf8");

  test("failed items branch on result.failed.length", () => {
    expect(src).toContain("if (result.failed.length > 0)");
    expect(src).toContain("result.failed[0]");
    expect(src).toContain("{ count: result.failed.length, reason }");
  });

  test("failure toast goes through the existing showToast channel with count + truncated reason", () => {
    expect(src).toMatch(
      /showToast\(\s*t\("history\.toast\.details\.partial\.failed",\s*\{\s*count:\s*result\.failed\.length,\s*reason\s*\}\s*\)/
    );
    // First failure reason is truncated before interpolation.
    expect(src).toContain("slice(0, 120)");
  });

  test("success path still reports the copied count unchanged", () => {
    expect(src).toContain('t("history.toast.details.copied", { count: details.length })');
  });
});

describe("items rendering path unchanged", () => {
  const src = readFileSync(historyViewFile, "utf8");

  test("clipboard text is still built from result.items exactly as before", () => {
    expect(src).toContain("const details = result.items;");
    expect(src).toContain("const text = details");
    expect(src).toContain("commit.authorName} <${commit.authorEmail}");
    expect(src).toContain('join("\\n\\n---\\n\\n")');
  });

  test("clipboard write + execCommand fallback intact", () => {
    expect(src).toContain("navigator.clipboard.writeText(text)");
    expect(src).toContain('document.execCommand("copy")');
  });
});

describe("i18n key parity for batch details partial failure", () => {
  test(`key "${PARTIAL_KEY}" is lowercase dot-literal`, () => {
    expect(PARTIAL_KEY).toMatch(/^[a-z0-9.]+$/);
  });

  test("exists in zh-CN and en-US with non-empty translation carrying {count}/{reason}", () => {
    const zh = (zhCN as Record<string, string>)[PARTIAL_KEY];
    const en = (enUS as Record<string, string>)[PARTIAL_KEY];
    for (const msg of [zh, en]) {
      expect(msg).toBeDefined();
      expect(msg.trim().length).toBeGreaterThan(0);
      expect(msg).toContain("{count}");
      expect(msg).toContain("{reason}");
    }
  });

  test("declared at the aligned position right after history.toast.details.failed in both locales", () => {
    for (const file of [zhFile, enFile]) {
      const text = readFileSync(file, "utf8");
      const lines = text.split("\n");
      const failedIdx = lines.findIndex((line) => line.includes('"history.toast.details.failed"'));
      const partialIdx = lines.findIndex((line) => line.includes(`"${PARTIAL_KEY}"`));
      expect(failedIdx).toBeGreaterThan(-1);
      expect(partialIdx).toBe(failedIdx + 1);
    }
  });
});
