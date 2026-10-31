import { afterEach, beforeEach, describe, expect, spyOn, test } from "bun:test";
import { mount } from "@vue/test-utils";
import { readFileSync } from "fs";
import { resolve } from "path";
import ChangesList from "../src/components/workspace/ChangesList.vue";
import type { FileStatus } from "../src/types";
import "./domSetup";

const CHANGES_GROUPS_COLLAPSED_KEY = "wgit.changes.groups.collapsed.v1";

const changesListContent = readFileSync(
  resolve(__dirname, "../src/components/workspace/ChangesList.vue"),
  "utf-8"
);

function makeStaged(path: string): FileStatus {
  return {
    path,
    stagedStatus: "M",
    unstagedStatus: " ",
    isStaged: true,
    isLfs: false,
    additions: 10,
    deletions: 2,
    size: "1KB",
  };
}

function makeUnstaged(path: string): FileStatus {
  return {
    path,
    stagedStatus: " ",
    unstagedStatus: "M",
    isStaged: false,
    isLfs: false,
    additions: 5,
    deletions: 1,
    size: "2KB",
  };
}

function mountChangesList(options?: {
  staged?: FileStatus[];
  unstaged?: FileStatus[];
  conflicted?: string[];
}) {
  return mount(ChangesList, {
    props: {
      stagedFiles: options?.staged ?? [makeStaged("src/main.ts")],
      unstagedFiles: options?.unstaged ?? [makeUnstaged("src/App.vue")],
      conflictedFiles: options?.conflicted ?? ["src/config.json"],
      selectedPath: "",
      currentRepoPath: "/test/repo",
    },
  });
}

describe("Changes Groups Collapse: persisted group-level folding", () => {
  beforeEach(() => {
    localStorage.removeItem(CHANGES_GROUPS_COLLAPSED_KEY);
  });

  afterEach(() => {
    localStorage.removeItem(CHANGES_GROUPS_COLLAPSED_KEY);
  });

  describe("Source assertions on ChangesList.vue", () => {
    test("all three group headers are collapsible nodes with click toggles", () => {
      expect(changesListContent).toContain('data-testid="changes-group-header-conflicted"');
      expect(changesListContent).toContain('data-testid="changes-group-header-staged"');
      expect(changesListContent).toContain('data-testid="changes-group-header-unstaged"');
      expect(changesListContent).toContain(
        '@click="isConflictedCollapsed = !isConflictedCollapsed"'
      );
      expect(changesListContent).toContain('@click="isStagedCollapsed = !isStagedCollapsed"');
      expect(changesListContent).toContain('@click="isUnstagedCollapsed = !isUnstagedCollapsed"');
    });

    test("chevron rotates instead of swapping icons", () => {
      expect(changesListContent).toContain("transition-transform");
      expect(changesListContent).toContain("? '-rotate-90' : ''");
      expect(changesListContent).not.toContain("isConflictedCollapsed ? ChevronRight");
      expect(changesListContent).not.toContain("isStagedCollapsed ? ChevronRight");
      expect(changesListContent).not.toContain("isUnstagedCollapsed ? ChevronRight");
    });

    test("collapsed group bodies are not rendered (v-if, not v-show)", () => {
      expect(changesListContent).toContain('v-if="!isConflictedCollapsed"');
      expect(changesListContent).toContain('v-if="!isStagedCollapsed"');
      expect(changesListContent).toContain('v-if="!isUnstagedCollapsed"');
      expect(changesListContent).not.toContain('v-show="!isConflictedCollapsed"');
      expect(changesListContent).not.toContain('v-show="!isStagedCollapsed"');
      expect(changesListContent).not.toContain('v-show="!isUnstagedCollapsed"');
    });

    test("count badges stay bound to the filtered group lists", () => {
      expect(changesListContent).toContain("{{ filteredConflictedFiles.length }}");
      expect(changesListContent).toContain("{{ filteredStagedFiles.length }}");
      expect(changesListContent).toContain("{{ filteredUnstagedFiles.length }}");
    });

    test("persistence uses the agreed key and read/write paths are guarded", () => {
      expect(changesListContent).toContain(
        'CHANGES_GROUPS_COLLAPSED_KEY = "wgit.changes.groups.collapsed.v1"'
      );
      expect(changesListContent).toContain("loadChangesGroupsCollapsed");
      expect(changesListContent).toContain("persistChangesGroupsCollapsed");
      expect(changesListContent).toContain("localStorage.getItem(CHANGES_GROUPS_COLLAPSED_KEY)");
      expect(changesListContent).toContain(
        "localStorage.setItem(CHANGES_GROUPS_COLLAPSED_KEY, JSON.stringify(state))"
      );
      // 默认值：任何缺字段/异常都回退为布尔 false（全展开）。
      expect(changesListContent).toContain("parsed?.staged === true");
      expect(changesListContent).toContain("parsed?.unstaged === true");
      expect(changesListContent).toContain("parsed?.conflicted === true");
      expect(changesListContent).toContain("return { staged: false, unstaged: false, conflicted: false };");
      // 读写均 try/catch 吞错并注明理由。
      expect(changesListContent).toContain("watch([isStagedCollapsed, isUnstagedCollapsed, isConflictedCollapsed], persistChangesGroupsCollapsed)");
    });
  });

  describe("Collapse state logic (mounted with happy-dom)", () => {
    test("renders all three groups expanded by default with collapsible headers", () => {
      const wrapper = mountChangesList();

      expect(wrapper.find("[data-testid='changes-group-header-conflicted']").exists()).toBe(true);
      expect(wrapper.find("[data-testid='changes-group-header-staged']").exists()).toBe(true);
      expect(wrapper.find("[data-testid='changes-group-header-unstaged']").exists()).toBe(true);
      expect(wrapper.text()).toContain("main.ts");
      expect(wrapper.text()).toContain("App.vue");
      expect(wrapper.text()).toContain("config.json");
    });

    test("collapsing the staged group drops its body from the DOM but keeps the count badge", async () => {
      const wrapper = mountChangesList();
      const header = wrapper.find("[data-testid='changes-group-header-staged']");
      expect(header.text()).toContain("1");

      await header.trigger("click");
      await wrapper.vm.$nextTick();

      expect(wrapper.text()).not.toContain("main.ts");
      const collapsedHeader = wrapper.find("[data-testid='changes-group-header-staged']");
      expect(collapsedHeader.exists()).toBe(true);
      expect(collapsedHeader.text()).toContain("1");

      const stored = localStorage.getItem(CHANGES_GROUPS_COLLAPSED_KEY);
      expect(stored).toBeTruthy();
      expect(JSON.parse(stored!)).toEqual({ staged: true, unstaged: false, conflicted: false });
    });

    test("a collapsed group keeps its badge count fresh when new files arrive (no auto-expand)", async () => {
      const wrapper = mountChangesList();
      await wrapper.find("[data-testid='changes-group-header-staged']").trigger("click");
      await wrapper.vm.$nextTick();
      expect(wrapper.text()).not.toContain("main.ts");

      await wrapper.setProps({
        stagedFiles: [makeStaged("src/main.ts"), makeStaged("src/other.ts")],
      });
      await wrapper.vm.$nextTick();

      const header = wrapper.find("[data-testid='changes-group-header-staged']");
      expect(header.exists()).toBe(true);
      expect(header.text()).toContain("2");
      expect(wrapper.text()).not.toContain("main.ts");
      expect(wrapper.text()).not.toContain("other.ts");
    });

    test("second click re-expands the group", async () => {
      const wrapper = mountChangesList();
      const header = () => wrapper.find("[data-testid='changes-group-header-staged']");

      await header().trigger("click");
      await wrapper.vm.$nextTick();
      expect(wrapper.text()).not.toContain("main.ts");

      await header().trigger("click");
      await wrapper.vm.$nextTick();
      expect(wrapper.text()).toContain("main.ts");
    });

    test("collapsing and expanding other groups persists per-group flags", async () => {
      const wrapper = mountChangesList();
      await wrapper.find("[data-testid='changes-group-header-unstaged']").trigger("click");
      await wrapper.find("[data-testid='changes-group-header-conflicted']").trigger("click");
      await wrapper.vm.$nextTick();

      expect(JSON.parse(localStorage.getItem(CHANGES_GROUPS_COLLAPSED_KEY)!)).toEqual({
        staged: false,
        unstaged: true,
        conflicted: true,
      });
      expect(wrapper.text()).not.toContain("App.vue");
      expect(wrapper.text()).not.toContain("config.json");
      expect(wrapper.text()).toContain("main.ts");
    });

    test("restores collapsed groups from localStorage on remount", () => {
      localStorage.setItem(
        CHANGES_GROUPS_COLLAPSED_KEY,
        JSON.stringify({ staged: true, unstaged: false, conflicted: false })
      );

      const wrapper = mountChangesList();

      expect(wrapper.text()).not.toContain("main.ts");
      expect(wrapper.text()).toContain("App.vue");
      expect(wrapper.text()).toContain("config.json");
    });

    test("corrupted JSON falls back to all-expanded without throwing", () => {
      localStorage.setItem(CHANGES_GROUPS_COLLAPSED_KEY, "not-valid-json{{");

      const wrapper = mountChangesList();

      expect(wrapper.text()).toContain("main.ts");
      expect(wrapper.text()).toContain("App.vue");
      expect(wrapper.text()).toContain("config.json");
    });

    test("non-object persisted values fall back to all-expanded", () => {
      localStorage.setItem(CHANGES_GROUPS_COLLAPSED_KEY, "42");

      const wrapper = mountChangesList();

      expect(wrapper.text()).toContain("main.ts");
      expect(wrapper.text()).toContain("App.vue");
    });

    test("tolerates localStorage.getItem throwing at mount", () => {
      const spy = spyOn(localStorage, "getItem").mockImplementation(() => {
        throw new Error("storage denied");
      });
      try {
        const wrapper = mountChangesList();
        expect(wrapper.text()).toContain("main.ts");
        expect(wrapper.text()).toContain("App.vue");
        expect(wrapper.text()).toContain("config.json");
      } finally {
        spy.mockRestore();
      }
    });

    test("group headers are keyboard operable with Enter and Space", async () => {
      const wrapper = mountChangesList();
      const header = wrapper.find("[data-testid='changes-group-header-staged']");
      expect(header.attributes("role")).toBe("button");
      expect(header.attributes("tabindex")).toBe("0");
      expect(header.attributes("aria-expanded")).toBe("true");

      await header.trigger("keydown", { key: "Enter" });
      await wrapper.vm.$nextTick();
      expect(header.attributes("aria-expanded")).toBe("false");
      expect(wrapper.text()).not.toContain("main.ts");

      await header.trigger("keydown", { key: " " });
      await wrapper.vm.$nextTick();
      expect(header.attributes("aria-expanded")).toBe("true");
      expect(wrapper.text()).toContain("main.ts");
    });

    test("file rows are keyboard operable and emit select-file", async () => {
      const wrapper = mountChangesList();
      const row = wrapper
        .findAll("[role='button']")
        .find((item) => item.text().includes("App.vue"));
      expect(row).toBeTruthy();
      expect(row!.attributes("tabindex")).toBe("0");
      await row!.trigger("keydown", { key: "Enter" });
      const emitted = wrapper.emitted("select-file") as unknown[][] | undefined;
      expect(emitted?.length).toBe(1);
      expect((emitted![0]![0] as FileStatus).path).toBe("src/App.vue");
    });

    test("tolerates localStorage.setItem throwing while persisting a toggle", async () => {
      const spy = spyOn(localStorage, "setItem").mockImplementation(() => {
        throw new Error("quota exceeded");
      });
      try {
        const wrapper = mountChangesList();
        await wrapper.find("[data-testid='changes-group-header-staged']").trigger("click");
        await wrapper.vm.$nextTick();
        // 折叠在内存中仍然生效，只是放弃持久化。
        expect(wrapper.text()).not.toContain("main.ts");
        expect(wrapper.find("[data-testid='changes-group-header-staged']").text()).toContain("1");
      } finally {
        spy.mockRestore();
      }
    });
  });
});
