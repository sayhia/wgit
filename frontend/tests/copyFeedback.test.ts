import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

describe("clipboard feedback", () => {
  test("Reflog and submodule copy helpers await the write and report failures", () => {
    for (const path of [
      "../src/components/tools/ReflogTab.vue",
      "../src/components/tools/SubmodulesTab.vue",
    ]) {
      const src = read(path);
      expect(src).toContain("await navigator.clipboard.writeText(text)");
      expect(src).toContain('t("common.copy.failed"');
      expect(src).not.toContain("navigator.clipboard.writeText(text);\n  notify(");
    }
    expect((zhCN as Record<string, string>)["common.copy.failed"]).toBeTruthy();
    expect((enUS as Record<string, string>)["common.copy.failed"]).toBeTruthy();
  });

  test("every remaining copy surface reports failures instead of swallowing them", () => {
    const paths = [
      "../src/components/WorkspaceView.vue",
      "../src/components/HistoryView.vue",
      "../src/components/history/PickaxeModal.vue",
      "../src/components/tools/DiskTab.vue",
      "../src/components/tools/WorktreesTab.vue",
      "../src/components/tools/RemotesTab.vue",
      "../src/components/tools/LfsTab.vue",
      "../src/components/tools/TerminalTab.vue",
    ];
    for (const path of paths) {
      const src = read(path);
      expect(src).toContain('utils/clipboard"');
      expect(src).toContain("writeClipboardText");
      if (!path.endsWith("TerminalTab.vue")) {
        expect(src).toContain('t("common.copy.failed"');
      }
    }
    const util = read("../src/utils/clipboard.ts");
    expect(util).toContain("navigator.clipboard.writeText(text)");
    expect(util).toContain("catch");
    expect(util).toContain("return err instanceof Error ? err.message : String(err)");
  });
});
