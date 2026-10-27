import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { gitService } from "../src/services/gitService";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

describe("Phase F3: LAN Event Bus & P2P LFS Assist UX", () => {
  test("gitService exposes LAN review broadcast and P2P LFS methods", () => {
    expect(typeof gitService.broadcastLanReviewEvent).toBe("function");
    expect(typeof gitService.detectLanLfsPeers).toBe("function");
    expect(typeof gitService.syncLanLfsBlob).toBe("function");
    expect(typeof gitService.configureLanLfsSharing).toBe("function");
  });

  test("LanReviewModal integrates SSE event stream and peer count indicator", () => {
    const filePath = resolve(__dirname, "../src/components/review/LanReviewModal.vue");
    const content = readFileSync(filePath, "utf-8");

    // SSE connection logic
    expect(content).toContain("EventSource");
    expect(content).toContain("activePeerCount");
    expect(content).toContain("broadcastLanReviewEvent");
    expect(content).toContain("review.lan.bus.peers");
    expect(content).toContain("review.lan.bus.broadcast");
  });

  test("LfsTab integrates LAN P2P cache acceleration controls", () => {
    const filePath = resolve(__dirname, "../src/components/tools/LfsTab.vue");
    const content = readFileSync(filePath, "utf-8");

    expect(content).toContain("detectLanLfsPeers");
    expect(content).toContain("configureLanLfsSharing");
    expect(content).toContain("lanLfsStatus");
    expect(content).toContain("lfs.lan.title");
    expect(content).toContain("lfs.lan.serving");
    expect(content).toContain("lfs.lan.scan");
    expect(content).toContain("lfs.lan.peers.discovered");
  });

  test("i18n catalogs contain complete LAN event and P2P LFS keys in both languages", () => {
    const requiredKeys = [
      "review.lan.event.comment",
      "review.lan.bus.peers",
      "review.lan.bus.broadcast",
      "review.lan.bus.broadcast.title",
      "review.lan.toast.broadcast.sent",
      "lfs.lan.title",
      "lfs.lan.serving",
      "lfs.lan.idle",
      "lfs.lan.scan",
      "lfs.lan.scan.title",
      "lfs.lan.start",
      "lfs.lan.stop",
      "lfs.lan.desc",
      "lfs.lan.peers.discovered",
      "lfs.lan.peer.blobs",
      "lfs.lan.peers.empty",
      "lfs.toast.lan.scanned",
      "lfs.toast.lan.started",
      "lfs.toast.lan.stopped",
      "lfs.toast.lan.failed",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key].length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key].length).toBeGreaterThan(0);
    }
  });
});
