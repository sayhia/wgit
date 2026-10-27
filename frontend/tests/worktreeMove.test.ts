import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const worktreesTabFile = fileURLToPath(
  new URL("../src/components/tools/WorktreesTab.vue", import.meta.url)
);

const requiredKeys = [
  "worktrees.move.aria",
  "worktrees.move.title",
  "worktrees.move.source",
  "worktrees.move.target",
  "worktrees.move.target.placeholder",
  "worktrees.move.confirm",
  "worktrees.move.moving",
  "worktrees.confirm.move",
  "worktrees.toast.moved",
  "worktrees.toast.move.path.required",
  "worktrees.toast.move.same",
  "worktrees.toast.move.failed",
];

describe("Worktree move wiring", () => {
  const src = readFileSync(worktreesTabFile, "utf8");

  test("offers a move action backed by MoveWorktree", () => {
    expect(src).toContain("openMoveWorktreeModal");
    expect(src).toContain("handleMoveWorktree");
    expect(src).toContain("gitService.moveWorktree(props.currentRepoPath, source, target)");
    expect(src).toContain('t("worktrees.confirm.move"');
    expect(src).toContain("isMovingWorktree");
  });

  test("zh-CN and en-US contain all move keys", () => {
    const zh = zhCN as Record<string, string>;
    const en = enUS as Record<string, string>;
    for (const key of requiredKeys) {
      expect(zh[key]?.length ?? 0).toBeGreaterThan(0);
      expect(en[key]?.length ?? 0).toBeGreaterThan(0);
    }
  });
});
