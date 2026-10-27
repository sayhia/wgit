import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const submodulesTabFile = fileURLToPath(
  new URL("../src/components/tools/SubmodulesTab.vue", import.meta.url)
);

const requiredKeys = [
  "sub.subtree.split",
  "sub.subtree.split.title",
  "sub.subtree.edit",
  "sub.subtree.edit.title",
  "sub.subtree.unlink",
  "sub.subtree.unlink.title",
  "sub.confirm.subtree.unlink",
  "sub.modal.subtree.split.title",
  "sub.modal.subtree.split.label",
  "sub.modal.subtree.split.branch",
  "sub.modal.subtree.split.branch.aria",
  "sub.modal.subtree.split.branch.placeholder",
  "sub.modal.subtree.split.squash",
  "sub.modal.subtree.split.rejoin",
  "sub.modal.subtree.split.btn",
  "sub.modal.subtree.edit.title",
  "sub.modal.subtree.edit.label",
  "sub.toast.subtree.split.success",
  "sub.toast.subtree.split.failed",
  "sub.toast.subtree.edit.success",
  "sub.toast.subtree.edit.failed",
  "sub.toast.subtree.unlink.success",
  "sub.toast.subtree.unlink.failed",
];

describe("Subtree advanced operations wiring", () => {
  const src = readFileSync(submodulesTabFile, "utf8");

  test("offers Split, Edit, and Unlink actions backed by gitService", () => {
    expect(src).toContain("openSplitSubtreeModal");
    expect(src).toContain("handleSplitSubtree");
    expect(src).toContain("gitService.splitSubtree(");
    expect(src).toContain("openEditSubtreeModal");
    expect(src).toContain("handleEditSubtree");
    expect(src).toContain("gitService.updateSubtreeMetadata(");
    expect(src).toContain("handleUnlinkSubtree");
    expect(src).toContain("gitService.unlinkSubtree(");
  });

  test("zh-CN and en-US contain all subtree operation keys", () => {
    const zh = zhCN as Record<string, string>;
    const en = enUS as Record<string, string>;
    for (const key of requiredKeys) {
      expect(zh[key]?.length ?? 0).toBeGreaterThan(0);
      expect(en[key]?.length ?? 0).toBeGreaterThan(0);
    }
  });
});
