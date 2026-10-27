import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const timeLapseModalFile = fileURLToPath(
  new URL("../src/components/history/CommitTimeLapseModal.vue", import.meta.url)
);
const commitInspectorFile = fileURLToPath(
  new URL("../src/components/history/CommitInspector.vue", import.meta.url)
);
const historyViewFile = fileURLToPath(
  new URL("../src/components/HistoryView.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);
const typesFile = fileURLToPath(
  new URL("../src/types.ts", import.meta.url)
);

describe("Phase G3: Commit Time-Lapse Player & Ownership Bus Factor UX", () => {
  const modalSrc = readFileSync(timeLapseModalFile, "utf8");
  const inspectorSrc = readFileSync(commitInspectorFile, "utf8");
  const historySrc = readFileSync(historyViewFile, "utf8");
  const gitServiceSrc = readFileSync(gitServiceFile, "utf8");
  const typesSrc = readFileSync(typesFile, "utf8");

  test("CommitTimeLapseModal provides timeline playback controls and scrubber", () => {
    expect(modalSrc).toContain("timeline-slider");
    expect(modalSrc).toContain("play-btn");
    expect(modalSrc).toContain("togglePlay");
    expect(modalSrc).toContain("stepPrev");
    expect(modalSrc).toContain("stepNext");
    expect(modalSrc).toContain("cycleSpeed");
    expect(modalSrc).toContain("numberedLines");
  });

  test("CommitTimeLapseModal provides Code Ownership and Bus Factor analysis", () => {
    expect(modalSrc).toContain("ownership-panel");
    expect(modalSrc).toContain("ownershipReport");
    expect(modalSrc).toContain("busFactor");
    expect(modalSrc).toContain("busFactorAlert");
    expect(modalSrc).toContain("owners");
  });

  test("CommitInspector and HistoryView integrate time-lapse trigger", () => {
    expect(inspectorSrc).toContain("open-timelapse");
    expect(historySrc).toContain("handleOpenTimeLapse");
    expect(historySrc).toContain("CommitTimeLapseModal");
  });

  test("gitService exposes Phase G3 RPC endpoints", () => {
    expect(gitServiceSrc).toContain("getCodeOwnership:");
    expect(gitServiceSrc).toContain("getFileCommitTimeline:");
  });

  test("types.ts defines Phase G3 interfaces", () => {
    expect(typesSrc).toContain("export interface CodeOwnershipEntry");
    expect(typesSrc).toContain("export interface CodeOwnershipReport");
    expect(typesSrc).toContain("export interface FileTimelineEntry");
  });

  test("i18n parity for timelapse and ownership keys", () => {
    const requiredKeys = [
      "timelapse.title",
      "timelapse.subtitle",
      "timelapse.play",
      "timelapse.pause",
      "timelapse.step.prev",
      "timelapse.step.next",
      "timelapse.speed",
      "timelapse.empty",
      "timelapse.tab.player",
      "timelapse.tab.ownership",
      "ownership.title",
      "ownership.desc",
      "ownership.busfactor",
      "ownership.alert.critical",
      "ownership.alert.warning",
      "ownership.alert.healthy",
      "ownership.role.primary",
      "ownership.role.major",
      "ownership.role.minor",
      "ownership.total.commits",
      "ownership.total.lines",
      "ownership.last.active",
      "ownership.btn.inspect",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
    }
  });
});
