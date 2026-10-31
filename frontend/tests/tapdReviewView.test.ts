import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { flushPromises, mount } from "@vue/test-utils";
import TapdReviewView from "../src/components/review/TapdReviewView.vue";
import {
  TAPD_REVIEW_CAPABILITIES,
  type ReviewDetail,
  type TAPDReviewSource,
} from "../src/services/reviewSource";
import type { TAPDWorkItem } from "../src/types";
import "./domSetup";

const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

const ITEM: TAPDWorkItem = {
  id: "1002341",
  type: "story",
  title: "按块应用冲突建议",
  status: "in_progress",
  statusLabel: "进行中",
  priority: "high",
  priorityLabel: "高",
  owner: "alice",
  creator: "bob",
  iteration: "2026-10",
  createdTime: "2026-10-01T09:00:00Z",
  updatedTime: "2026-10-02T10:30:00Z",
  description: "把冲突建议按块写回文件。",
  boundCommits: ["abc1234"],
  url: "https://tapd.example.com/workitem/1002341",
};

function detailOf(item: TAPDWorkItem): ReviewDetail {
  return {
    summary: {
      id: item.id,
      number: 1002341,
      title: item.title,
      state: "open",
      isDraft: false,
      author: item.owner,
      sourceBranch: "",
      targetBranch: "",
      createdAt: item.createdTime,
      updatedAt: item.updatedTime,
      commentCount: 0,
      additions: 0,
      deletions: 0,
      changedFiles: 0,
      webUrl: item.url,
      labels: [],
      checksState: "",
      mergeable: false,
      mergeableState: "",
      verdict: "pending",
      progress: 0,
    },
    body: item.description ?? "",
    files: [],
    comments: [],
    checks: [],
    commitCount: 0,
    baseSha: "",
    headSha: "",
    startSha: "",
    filesTruncated: false,
    message: "",
    native: item,
  };
}

function fakeSource(items: TAPDWorkItem[], message = ""): TAPDReviewSource {
  return {
    kind: "tapd",
    capabilities: TAPD_REVIEW_CAPABILITIES,
    async list() {
      return { items: [], total: items.length, hasMore: false, message, webBaseUrl: "", nativeItems: items };
    },
    async detail(id) {
      const item = items.find((row) => row.id === id);
      if (!item) throw new Error("not found");
      return detailOf(item);
    },
  };
}

describe("TapdReviewView read-only surface", () => {
  test("lists work items and renders their native details", async () => {
    const wrapper = mount(TapdReviewView, {
      props: { currentRepoPath: "/repo", source: fakeSource([ITEM]) },
    });
    await flushPromises();

    expect(wrapper.find("[data-testid='tapd-review-view']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='tapd-item-1002341']").text()).toContain("按块应用冲突建议");
    expect(wrapper.find("[data-testid='tapd-detail']").text()).toContain("把冲突建议按块写回文件");
    expect(wrapper.find("[data-testid='tapd-detail']").text()).toContain("alice");
    expect(wrapper.find("[data-testid='tapd-detail']").text()).toContain("abc1234");
    expect(wrapper.find("[data-testid='tapd-open']").exists()).toBe(true);
  });

  test("shows the backend's honest empty message instead of demo rows", async () => {
    const message = "TAPD OpenAPI 同步尚未接入；当前仅支持本地单号解析与 Git Notes 关联。";
    const wrapper = mount(TapdReviewView, {
      props: { currentRepoPath: "/repo", source: fakeSource([], message) },
    });
    await flushPromises();

    const empty = wrapper.find("[data-testid='tapd-review-empty']");
    expect(empty.exists()).toBe(true);
    expect(empty.text()).toContain(message);
    expect(wrapper.findAll("[data-testid^='tapd-item-']")).toHaveLength(0);
  });

  test("the app routes the TAPD mode to this view from all review switches", () => {
    const app = read("../src/App.vue");
    const local = read("../src/components/ReviewView.vue");
    const remote = read("../src/components/pr/PrView.vue");
    expect(app).toContain("LazyTapdReview");
    expect(app).toContain('reviewMode === \'remote\'');
    expect(app).toContain("savedMode === \"tapd\"");
    for (const source of [local, remote]) {
      expect(source).toContain('"local" | "remote" | "tapd"');
      expect(source).toContain("@click=\"emit('switch-mode', 'tapd')\"");
      expect(source).toContain('t("review.mode.tapd")');
    }
  });

  test("mode switch exposes local, remote and TAPD", async () => {
    const wrapper = mount(TapdReviewView, {
      props: { currentRepoPath: "/repo", source: fakeSource([]) },
    });
    await flushPromises();
    const buttons = wrapper.findAll("[role='group'][aria-label] button");
    expect(buttons).toHaveLength(3);
    await buttons[0]!.trigger("click");
    await buttons[1]!.trigger("click");
    expect(wrapper.emitted("switch-mode")?.[0]).toEqual(["local"]);
    expect(wrapper.emitted("switch-mode")?.[1]).toEqual(["remote"]);
  });
});
