import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const riverViewFile = fileURLToPath(
  new URL("../src/components/branches/BranchRiverView.vue", import.meta.url)
);
const branchesViewFile = fileURLToPath(
  new URL("../src/components/BranchesView.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);
const typesFile = fileURLToPath(
  new URL("../src/types.ts", import.meta.url)
);

describe("Phase G2: Branch River Topology & Stale Lifecycle Archiver UX", () => {
  const riverSrc = readFileSync(riverViewFile, "utf8");
  const branchesSrc = readFileSync(branchesViewFile, "utf8");
  const gitServiceSrc = readFileSync(gitServiceFile, "utf8");
  const typesSrc = readFileSync(typesFile, "utf8");

  test("BranchRiverView renders river topology and lifecycle archiver", () => {
    expect(riverSrc).toContain("loadRiverData");
    expect(riverSrc).toContain("openArchiver");
    expect(riverSrc).toContain("handleArchiveAndDelete");
    expect(riverSrc).toContain("selectedStaleBranches");
    expect(riverSrc).toContain("riverData");
    expect(riverSrc).toContain("staleReport");
    expect(riverSrc).toContain("river.title");
    expect(riverSrc).toContain("river.archiver.title");
  });

  test("BranchesView embeds BranchRiverView in tab navigation", () => {
    expect(branchesSrc).toContain("BranchRiverView");
    expect(branchesSrc).toContain("activeTab === 'river'");
    expect(branchesSrc).toContain("t(\"branch.tab.river\")");
  });

  test("gitService exposes Phase G2 RPC wrappers", () => {
    expect(gitServiceSrc).toContain("getBranchRiverData:");
    expect(gitServiceSrc).toContain("detectStaleBranches:");
    expect(gitServiceSrc).toContain("archiveAndDeleteBranches:");
  });

  test("types.ts defines Phase G2 interfaces", () => {
    expect(typesSrc).toContain("export interface BranchRiverNode");
    expect(typesSrc).toContain("export interface BranchRiverData");
    expect(typesSrc).toContain("export interface StaleBranchInfo");
    expect(typesSrc).toContain("export interface StaleBranchesReport");
    expect(typesSrc).toContain("export interface BranchArchiveResult");
  });

  test("i18n parity for branch river and archiver keys", () => {
    const requiredKeys = [
      "branch.tab.river",
      "river.title",
      "river.subtitle",
      "river.base",
      "river.refresh",
      "river.total",
      "river.active",
      "river.stale",
      "river.merged",
      "river.ahead",
      "river.behind",
      "river.divergence",
      "river.risk.score",
      "river.risk.low",
      "river.risk.medium",
      "river.risk.high",
      "river.risk.critical",
      "river.action.checkout",
      "river.action.compare",
      "river.archiver.btn",
      "river.archiver.title",
      "river.archiver.desc",
      "river.archiver.empty",
      "river.archiver.selected",
      "river.archiver.confirm",
      "river.archiver.success",
      "river.archiver.scanning",
      "river.archiver.days",
      "river.archiver.gone",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
    }
  });
});
