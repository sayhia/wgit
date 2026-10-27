import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const hudFile = fileURLToPath(
  new URL("../src/components/workspace/FlightRecorderHud.vue", import.meta.url)
);
const workspaceViewFile = fileURLToPath(
  new URL("../src/components/WorkspaceView.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);
const typesFile = fileURLToPath(
  new URL("../src/types.ts", import.meta.url)
);

describe("Phase H3: Workspace Flight Recorder & Instant Snapshot Backup UX", () => {
  const hudSrc = readFileSync(hudFile, "utf8");
  const wsSrc = readFileSync(workspaceViewFile, "utf8");
  const gitServiceSrc = readFileSync(gitServiceFile, "utf8");
  const typesSrc = readFileSync(typesFile, "utf8");

  test("FlightRecorderHud renders snapshot timeline and instant restore controls", () => {
    expect(hudSrc).toContain("flight-recorder-trigger-btn");
    expect(hudSrc).toContain("take-flight-snapshot-btn");
    expect(hudSrc).toContain("restore-flight-snapshot-btn");
    expect(hudSrc).toContain("createFlightRecorderSnapshot");
    expect(hudSrc).toContain("listFlightSnapshots");
    expect(hudSrc).toContain("restoreFlightSnapshot");
    expect(hudSrc).toContain("purgeFlightSnapshots");
    expect(hudSrc).toContain("flight_recorder.title");
    expect(hudSrc).toContain("flight_recorder.btn.snapshot");
    expect(hudSrc).toContain("flight_recorder.btn.restore");
  });

  test("WorkspaceView integrates FlightRecorderHud in workspace status bar", () => {
    expect(wsSrc).toContain("FlightRecorderHud");
    expect(wsSrc).toContain(':repo-path="currentRepoPath"');
  });

  test("gitService exposes Phase H3 RPC endpoints", () => {
    expect(gitServiceSrc).toContain("createFlightRecorderSnapshot:");
    expect(gitServiceSrc).toContain("listFlightSnapshots:");
    expect(gitServiceSrc).toContain("restoreFlightSnapshot:");
    expect(gitServiceSrc).toContain("purgeFlightSnapshots:");
  });

  test("types.ts defines Phase H3 interfaces", () => {
    expect(typesSrc).toContain("export interface FlightSnapshotInfo");
    expect(typesSrc).toContain("export interface SnapshotRestoreResult");
  });

  test("i18n parity for flight recorder keys", () => {
    const requiredKeys = [
      "flight_recorder.title",
      "flight_recorder.subtitle",
      "flight_recorder.badge",
      "flight_recorder.btn.snapshot",
      "flight_recorder.btn.purge",
      "flight_recorder.list.title",
      "flight_recorder.list.empty",
      "flight_recorder.action.manual",
      "flight_recorder.action.pre_reset",
      "flight_recorder.action.pre_checkout",
      "flight_recorder.action.pre_merge",
      "flight_recorder.action.pre_rebase",
      "flight_recorder.file_count",
      "flight_recorder.btn.restore",
      "flight_recorder.restoring",
      "flight_recorder.restore.success",
      "flight_recorder.restore.failed",
      "flight_recorder.snapshot.success",
      "flight_recorder.purge.success",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
    }
  });
});
