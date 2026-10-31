import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const lfsFile = fileURLToPath(new URL("../src/components/tools/LfsTab.vue", import.meta.url));
const lfsSrc = readFileSync(lfsFile, "utf8");
const studioFile = fileURLToPath(new URL("../src/components/tools/StudioIntegrationSection.vue", import.meta.url));
const studioSrc = readFileSync(studioFile, "utf8");
const subFile = fileURLToPath(new URL("../src/components/tools/SubmodulesTab.vue", import.meta.url));
const subSrc = readFileSync(subFile, "utf8");

describe("frontend swallow triage batch 4", () => {
  test("LAN LFS peer detection failure is diagnosable, not fully silent", () => {
    expect(lfsSrc).toContain("console.error(\"LAN LFS peer detection failed\"");
    // 完全静默的空 catch 不得回归
    expect(lfsSrc).not.toMatch(/detectLanLfsPeers[\s\S]{0,160}\.catch\(\(\) => \{\}\)/);
  });

  test("LFS panel load errors keep surfacing through the aggregated toast", () => {
    expect(lfsSrc).toContain('notify(t("lfs.toast.load.failed"');
    // 7 个分区加载各自把错误记入 loadErrors 后统一提示——该结构仍在
    expect((lfsSrc.match(/loadErrors\.push/g) || []).length).toBeGreaterThanOrEqual(5);
  });

  test("TAPD head-commit prefill keeps the honest empty fallback with rationale", () => {
    expect(studioSrc).toContain("getCommits(props.currentRepoPath, 1).catch(() => [])");
    expect(studioSrc).toContain("诚实降级");
  });

  test("submodule matrix failure keeps its reportError channel", () => {
    expect(subSrc).toContain('reportError("Failed to load submodule matrix", err)');
  });
});
