import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

// Source-assertion suite for "swallowed catch" triage batch 2:
// SourceExplorer.vue (7 bare catches), ReviewView.vue (7), CommandPalette.vue (7).
// Fix points escalate user-decision-affecting swallowed service failures to an
// existing notify channel / console.error; keep points document why a benign
// localStorage/clipboard catch stays silent.

const sourceExplorerFile = fileURLToPath(
  new URL("../src/components/workspace/SourceExplorer.vue", import.meta.url)
);
const reviewViewFile = fileURLToPath(new URL("../src/components/ReviewView.vue", import.meta.url));
const commandPaletteFile = fileURLToPath(
  new URL("../src/components/CommandPalette.vue", import.meta.url)
);
const zhFile = fileURLToPath(new URL("../src/i18n/messages/zh-CN.ts", import.meta.url));
const enFile = fileURLToPath(new URL("../src/i18n/messages/en-US.ts", import.meta.url));

function count(src: string, needle: string): number {
  return src.split(needle).length - 1;
}

describe("Swallowed catch triage batch 2: fix points (user-visible wiring)", () => {
  const explorerSrc = readFileSync(sourceExplorerFile, "utf8");
  const reviewSrc = readFileSync(reviewViewFile, "utf8");

  test("SourceExplorer loadGitignorePatterns surfaces failures on the existing notify channel", () => {
    // A swallowed failure would make an ignored path look un-ignored, so the
    // file menu would offer "ignore" for a rule that already exists.
    expect(explorerSrc).toContain('console.error("Failed to load gitignore patterns:", err);');
    // Reuses the pre-existing gitignore.error.load key instead of a new one.
    expect(explorerSrc).toContain('emit("notify", t("gitignore.error.load", { error: err?.message || err }))');
    // State marking kept: the pattern list still falls back to empty.
    expect(explorerSrc).toContain("gitignorePatterns.value = [];");
  });

  test("ReviewView checkLanStatus logs probe failures instead of a bare /* ignore */", () => {
    expect(reviewSrc).toContain('console.error("Failed to check LAN review server status:", err);');
    // State marking kept: the header dot stays hidden on failure (not-running).
    expect(reviewSrc).toContain("isLanServerRunning.value = !!status?.running;");
  });

  test("ReviewView detectDefaultBase logs detection failures while keeping the HEAD~20 fallback", () => {
    expect(reviewSrc).toContain('console.error("Failed to detect default base ref:", err);');
    // Graceful-degradation contract kept: the fallback base is unchanged.
    expect(reviewSrc).toContain('detected = "HEAD~20";');
  });

  test("no new i18n keys were introduced (reused gitignore.error.load exists in both locales)", () => {
    expect((zhCN as Record<string, string>)["gitignore.error.load"]).toBeDefined();
    expect((enUS as Record<string, string>)["gitignore.error.load"]).toBeDefined();
    const zhSrc = readFileSync(zhFile, "utf8");
    const enSrc = readFileSync(enFile, "utf8");
    expect(count(zhSrc, '"gitignore.error.load"')).toBe(1);
    expect(count(enSrc, '"gitignore.error.load"')).toBe(1);
  });
});

describe("Swallowed catch triage batch 2: keep points (documented benign catches)", () => {
  const explorerSrc = readFileSync(sourceExplorerFile, "utf8");
  const reviewSrc = readFileSync(reviewViewFile, "utf8");
  const paletteSrc = readFileSync(commandPaletteFile, "utf8");

  test("SourceExplorer localStorage catches carry rationale comments; clipboard catches stay toasting", () => {
    expect(explorerSrc).toContain("Benign: unreadable/corrupt expansion prefs just start everything collapsed.");
    expect(explorerSrc).toContain("Benign: unreadable/corrupt flat-group prefs just start everything expanded.");
    // The two persistence writers keep their pre-existing annotation.
    expect(count(explorerSrc, "/* ignore quota / privacy-mode errors */")).toBe(2);
    // The two clipboard catches already surface explorer.toast.copy.failed.
    expect(count(explorerSrc, 't("explorer.toast.copy.failed")')).toBe(2);
  });

  test("ReviewView draft-storage and clipboard catches keep their best-effort annotations", () => {
    expect(
      count(reviewSrc, "best-effort: a corrupt draft is not worth blocking the review surface.")
    ).toBe(3);
    expect(reviewSrc).toContain("回退：某些 WebView 不开放 clipboard API。");
    expect(reviewSrc).toContain("Benign: clipboard is best-effort here");
    // The fallback still marks state instead of throwing.
    expect(count(reviewSrc, "exportCopied.value = false;")).toBe(2);
  });

  test("CommandPalette persistence catches are all annotated localStorage keep points", () => {
    expect(paletteSrc).toContain("Benign: unreadable/corrupt recents just start the list empty.");
    expect(paletteSrc).toContain("Benign: unreadable/corrupt pins just start the list empty.");
    expect(paletteSrc).toContain("Benign: unreadable/corrupt collapse prefs just start everything expanded.");
    expect(count(paletteSrc, "ignore quota / privacy-mode errors")).toBe(3);
    expect(paletteSrc).toContain("ignore storage issues");
  });
});
