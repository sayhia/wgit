import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { LruCache } from "../src/utils/lruCache";
import { createAdaptivePoller } from "../src/utils/adaptivePolling";
import { commitDetailCache, gitService } from "../src/services/gitService";

const graphListFile = fileURLToPath(
  new URL("../src/components/history/CommitGraphList.vue", import.meta.url)
);

describe("S3: LRU Cache, Adaptive Poller & Graph Performance Suite", () => {
  describe("LruCache Implementation", () => {
    test("stores and retrieves key-value pairs", () => {
      const cache = new LruCache<string, number>(3);
      cache.set("a", 1);
      cache.set("b", 2);
      expect(cache.get("a")).toBe(1);
      expect(cache.get("b")).toBe(2);
      expect(cache.size()).toBe(2);
    });

    test("evicts least recently used item when capacity is exceeded", () => {
      const cache = new LruCache<string, string>(2);
      cache.set("k1", "v1");
      cache.set("k2", "v2");
      // Access k1 to make it most recently used
      expect(cache.get("k1")).toBe("v1");

      // Insert k3 -> k2 should be evicted (least recently used)
      cache.set("k3", "v3");
      expect(cache.has("k1")).toBe(true);
      expect(cache.has("k2")).toBe(false);
      expect(cache.has("k3")).toBe(true);
      expect(cache.size()).toBe(2);
    });

    test("deletes, checks existence, and clears all items", () => {
      const cache = new LruCache<string, number>(5);
      cache.set("x", 100);
      expect(cache.has("x")).toBe(true);
      cache.delete("x");
      expect(cache.has("x")).toBe(false);

      cache.set("y", 200);
      cache.set("z", 300);
      expect(cache.size()).toBe(2);
      cache.clear();
      expect(cache.size()).toBe(0);
      expect(cache.get("y")).toBeUndefined();
    });

    test("rejects invalid non-positive capacity", () => {
      expect(() => new LruCache(0)).toThrow();
      expect(() => new LruCache(-5)).toThrow();
    });
  });

  describe("AdaptivePoller Mechanics", () => {
    test("starts, reports running status, and stops cleanly", () => {
      let runCount = 0;
      const poller = createAdaptivePoller(
        () => {
          runCount++;
        },
        { activeIntervalMs: 50, backgroundIntervalMs: 200 }
      );

      expect(poller.isRunning()).toBe(false);
      poller.start();
      expect(poller.isRunning()).toBe(true);

      poller.stop();
      expect(poller.isRunning()).toBe(false);
    });

    test("triggers action immediately upon triggerNow()", async () => {
      let triggered = false;
      const poller = createAdaptivePoller(
        () => {
          triggered = true;
        },
        { activeIntervalMs: 5000 }
      );

      poller.start();
      await poller.triggerNow();
      expect(triggered).toBe(true);
      poller.stop();
    });
  });

  describe("gitService Commit Detail Caching", () => {
    test("commitDetailCache is exported and supports cache operations", () => {
      expect(commitDetailCache).toBeDefined();
      expect(commitDetailCache.getCapacity()).toBe(150);

      const mockHash = "feedbeef1234567890";
      const dummyDetail: any = { hash: mockHash, subject: "test commit", diff: "" };
      commitDetailCache.set(`repo:${mockHash}`, dummyDetail);

      expect(commitDetailCache.get(`repo:${mockHash}`)).toEqual(dummyDetail);
      gitService.clearCommitDetailCache();
      expect(commitDetailCache.get(`repo:${mockHash}`)).toBeUndefined();
    });
  });

  describe("CommitGraphList SVG Path Memoization", () => {
    const src = readFileSync(graphListFile, "utf8");

    test("imports LruCache and instantiates graphPathsCache", () => {
      expect(src).toContain('import { LruCache } from "../../utils/lruCache"');
      expect(src).toContain("const graphPathsCache = new LruCache<string, GraphPath[]>(600)");
    });

    test("memoizes graphPaths using commit hash and lane count", () => {
      expect(src).toContain("const cacheKey = `${row.hash}:${totalLaneCount.value}`");
      expect(src).toContain("const cached = graphPathsCache.get(cacheKey)");
      expect(src).toContain("graphPathsCache.set(cacheKey, paths)");
    });

    test("clears graph cache on commit list and graph mode changes", () => {
      expect(src).toContain("graphPathsCache.clear()");
    });
  });
});
