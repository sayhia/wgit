import { describe, expect, test, beforeEach } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

// Headless polyfill for localStorage in test environment
if (typeof (globalThis as any).localStorage === "undefined" || !(globalThis as any).localStorage?.getItem) {
  let store: Record<string, string> = {};
  (globalThis as any).localStorage = {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, val: string) => {
      store[key] = String(val);
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    },
  };
}

import { useSessionRestore } from "../src/composables/useSessionRestore";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const appVueFile = fileURLToPath(
  new URL("../src/App.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);
const typesFile = fileURLToPath(
  new URL("../src/types.ts", import.meta.url)
);

describe("Polish: Session Restore & Repo Cache Pre-warming UX", () => {
  const appSrc = readFileSync(appVueFile, "utf8");
  const gitServiceSrc = readFileSync(gitServiceFile, "utf8");
  const typesSrc = readFileSync(typesFile, "utf8");

  beforeEach(() => {
    localStorage.clear();
  });

  test("useSessionRestore manages workspace session state persistence", () => {
    const { loadSessionState, saveSessionState, clearSession } = useSessionRestore();

    expect(loadSessionState()).toBeNull();

    saveSessionState({
      lastRepoPath: "/work/test-repo",
      activeView: "history",
    });

    const loaded = loadSessionState();
    expect(loaded).not.toBeNull();
    expect(loaded?.lastRepoPath).toBe("/work/test-repo");
    expect(loaded?.activeView).toBe("history");
    expect(loaded?.lastUpdated).toBeGreaterThan(0);

    clearSession();
    expect(loadSessionState()).toBeNull();
  });

  test("App.vue integrates useSessionRestore for view state and cache prewarming", () => {
    expect(appSrc).toContain("useSessionRestore");
    expect(appSrc).toContain("prewarmRepo");
    expect(appSrc).toContain("saveSessionState");
  });

  test("gitService exposes prewarmRepoCache RPC wrapper", () => {
    expect(gitServiceSrc).toContain("prewarmRepoCache:");
  });

  test("types.ts defines RepoCachePrewarmResult", () => {
    expect(typesSrc).toContain("export interface RepoCachePrewarmResult");
  });

  test("i18n parity for prewarm and session restore keys", () => {
    const requiredKeys = [
      "prewarm.cached",
      "session.restored",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
    }
  });
});
