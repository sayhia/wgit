import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import {
  featureFlags,
  isFeatureEnabled,
  resetFeatureFlagsForTest,
} from "../src/composables/useFeatureFlags";
import { createReviewSource, createTAPDReviewSource, TAPD_REVIEW_CAPABILITIES } from "../src/services/reviewSource";
import type { TAPDReviewBackend } from "../src/services/reviewSource";
import type { TAPDWorkItem } from "../src/types";

const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");
const appSource = read("../src/App.vue");
const reviewSourceCode = read("../src/components/ReviewView.vue");
const tapdViewSource = read("../src/components/review/TapdReviewView.vue");

// ------------------------------------------------------- union type + guard --

describe("ReviewMode tapd kind (App.vue)", () => {
  test("the mode union admits tapd as the third kind", () => {
    expect(appSource).toContain('type ReviewMode = "local" | "remote" | "tapd"');
  });

  test("the restore guard only accepts a saved tapd mode while reviewFlow is enabled", () => {
    // The gated conjunction must be present...
    expect(appSource).toContain('savedMode === "tapd" && isFeatureEnabled("reviewFlow")');
    // ...and the old unconditional acceptance must be gone.
    expect(appSource).not.toContain('savedMode === "tapd") reviewMode.value = savedMode');
    // "local"/"remote" keep restoring without any flag condition.
    expect(appSource).toContain('savedMode === "local" ||');
    expect(appSource).toContain('savedMode === "remote" ||');
  });

  test("tapd restore falls back conservatively: no auto re-switch once flags land", () => {
    // The guard comment documents the tradeoff and no watcher re-applies tapd.
    expect(appSource).toContain("deliberately do NOT");
    expect(appSource).not.toContain('watch(featureFlags');
    expect(appSource).not.toContain('reviewMode.value = "tapd"');
  });

  test("the template dispatches the tapd mode to a dedicated review view", () => {
    expect(appSource).toContain("LazyTapdReview");
    expect(appSource).toContain("reviewMode === 'remote'");
    // The tapd branch still routes switch-mode back through setReviewMode.
    const branch = appSource.slice(appSource.indexOf("LazyTapdReview"));
    expect(branch).toContain('@switch-mode="setReviewMode"');
    expect(branch).toContain(':current-repo-path="currentRepoPath"');
  });
});

// ------------------------------------------------------ switch group button --

describe("TAPD switch button (ReviewView.vue)", () => {
  test("the third button exists, emits switch-mode tapd and is flag-gated dark", () => {
    expect(reviewSourceCode).toContain('import FeatureGate from "./ui/FeatureGate.vue"');
    expect(reviewSourceCode).toContain('<FeatureGate flag="reviewFlow">');
    // Emit contract matches the other switch buttons.
    expect(reviewSourceCode).toContain(`@click="emit('switch-mode', 'tapd')"`);
    expect(reviewSourceCode).toContain('t("review.mode.tapd")');
    expect(reviewSourceCode).toContain('(e: "switch-mode", mode: "local" | "remote" | "tapd")');
  });

  test("local and remote buttons stay outside the gate", () => {
    const group = reviewSourceCode.slice(
      reviewSourceCode.indexOf('role="group"'),
      reviewSourceCode.indexOf("</FeatureGate>")
    );
    expect(group).toContain(`emit('switch-mode', 'remote')`);
    expect(group).toContain('t("review.mode.local")');
  });
});

// ----------------------------------------------- tapd list binding + honest --

describe("tapd list rendering", () => {
  test("the tapd view binds the registered tapd source and passes the message through", () => {
    expect(tapdViewSource).toContain("createTAPDReviewSource(props.currentRepoPath)");
    expect(tapdViewSource).toContain("source.value.list()");
    expect(tapdViewSource).toContain("message.value = result.message");
    // Empty state shows the backend notice verbatim; webUrl uses the shared
    // openWebURL channel.
    expect(tapdViewSource).toContain("{{ message }}");
    expect(tapdViewSource).toContain("gitService.openWebURL(url)");
  });

  test("createReviewSource resolves the registered tapd factory with honest capabilities", () => {
    const source = createReviewSource("tapd", "/repo");
    expect(source.kind).toBe("tapd");
    expect(source.capabilities).toEqual(TAPD_REVIEW_CAPABILITIES);
    // No files/comments/verdict affordances: clicking a row only opens the web.
    for (const value of Object.values(source.capabilities)) {
      expect(value).toBe(false);
    }
  });

  test("list() maps webUrl onto ReviewItem and filters closed work items", async () => {
    const item: TAPDWorkItem = {
      id: "1001",
      type: "story",
      title: "评审 TAPD 接入",
      status: "in_progress",
      statusLabel: "进行中",
      priority: "high",
      priorityLabel: "高",
      owner: "alice",
      creator: "bob",
      iteration: "2026-10",
      createdTime: "2026-10-01T09:00:00Z",
      updatedTime: "2026-10-02T10:30:00Z",
      boundCommits: [],
      url: "https://tapd.example.com/workitem/1001",
    };
    const closed: TAPDWorkItem = {
      ...item,
      id: "1002",
      status: "closed",
      statusLabel: "已关闭",
      url: "https://tapd.example.com/workitem/1002",
    };
    const backend: TAPDReviewBackend = {
      fetchTAPDWorkItems: async () => ({
        workspaceId: "1",
        totalCount: 2,
        storiesCount: 2,
        tasksCount: 0,
        bugsCount: 0,
        items: [item, closed],
        isConnected: true,
        message: "",
      }),
    };
    const result = await createTAPDReviewSource("/repo", backend).list();
    expect(result.items).toHaveLength(1);
    expect(result.items[0]!.title).toBe("评审 TAPD 接入");
    expect(result.items[0]!.updatedAt).toBe("2026-10-02T10:30:00Z");
    expect(result.items[0]!.webUrl).toBe("https://tapd.example.com/workitem/1001");
    expect(result.total).toBe(1);
    expect(result.hasMore).toBe(false);
  });

  test("an unconnected workspace yields an honest empty list plus the backend notice", async () => {
    const notice = "TAPD OpenAPI 未连接";
    const backend: TAPDReviewBackend = {
      fetchTAPDWorkItems: async () => ({
        workspaceId: "1",
        totalCount: 0,
        storiesCount: 0,
        tasksCount: 0,
        bugsCount: 0,
        items: [],
        isConnected: false,
        message: notice,
      }),
    };
    const result = await createTAPDReviewSource("/repo", backend).list();
    expect(result.items).toHaveLength(0);
    expect(result.message).toBe(notice);
  });

  test("a transport failure degrades to the same empty state, never a throw", async () => {
    const backend: TAPDReviewBackend = {
      fetchTAPDWorkItems: async () => {
        throw new Error("rpc down");
      },
    };
    const result = await createTAPDReviewSource("/repo", backend).list();
    expect(result.items).toHaveLength(0);
    expect(result.message).toBe("");
  });
});

// ------------------------------------------------------------ flag predicate --

describe("reviewFlow flag predicate used by the guard", () => {
  test("dark by default and only true after a real backend answer", () => {
    resetFeatureFlagsForTest();
    expect(isFeatureEnabled("reviewFlow")).toBe(false);
    featureFlags.reviewFlow = true;
    expect(isFeatureEnabled("reviewFlow")).toBe(true);
    resetFeatureFlagsForTest();
    expect(isFeatureEnabled("reviewFlow")).toBe(false);
  });
});

// -------------------------------------------------------------------- i18n --

describe("review.mode.tapd i18n", () => {
  test("present in both locales with the lowercase key literal", () => {
    expect(Object.hasOwn(zhCN, "review.mode.tapd")).toBe(true);
    expect(Object.hasOwn(enUS, "review.mode.tapd")).toBe(true);
    expect(String(zhCN["review.mode.tapd"]).length).toBeGreaterThan(0);
    expect(String(enUS["review.mode.tapd"]).length).toBeGreaterThan(0);
  });

  test("sits beside the existing review.mode.* keys in both catalogs", () => {
    const zhKeys = Object.keys(zhCN);
    const enKeys = Object.keys(enUS);
    for (const keys of [zhKeys, enKeys]) {
      const label = keys.indexOf("review.mode.label");
      const local = keys.indexOf("review.mode.local");
      const remote = keys.indexOf("review.mode.remote");
      const tapd = keys.indexOf("review.mode.tapd");
      expect(local).toBe(label + 1);
      expect(remote).toBe(local + 1);
      expect(tapd).toBe(remote + 1);
    }
  });
});
