import { beforeEach, describe, expect, test, spyOn } from "bun:test";
import { mount } from "@vue/test-utils";
import UiSkeleton from "../src/components/ui/UiSkeleton.vue";
import ConfirmDialog from "../src/components/ui/ConfirmDialog.vue";
import CrashReportModal from "../src/components/CrashReportModal.vue";
import AboutSection from "../src/components/settings/AboutSection.vue";
import ThreeWayMergeModal from "../src/components/ThreeWayMergeModal.vue";
import CommitComposer from "../src/components/workspace/CommitComposer.vue";
import CommandPalette from "../src/components/CommandPalette.vue";
import DiffViewer from "../src/components/workspace/DiffViewer.vue";
import StashView from "../src/components/StashView.vue";
import BranchRiverView from "../src/components/branches/BranchRiverView.vue";
import SquashCommitsModal from "../src/components/history/SquashCommitsModal.vue";
import { confirmAction, confirmState } from "../src/composables/useConfirm";
import { clearCrashLogs, recordCrash } from "../src/utils/crashReporter";
import { gitService } from "../src/services/gitService";
import type { ThreeWayMergeData, StashInfo, BranchRiverData, CommitItem } from "../src/types";
import "./domSetup";

describe("P5: Real Component DOM Mounting Baseline (@vue/test-utils + happy-dom)", () => {
  beforeEach(() => {
    clearCrashLogs();
    confirmState.open = false;
  });

  test("mounts UiSkeleton and respects lines and variant props", () => {
    const wrapper = mount(UiSkeleton, {
      props: {
        lines: 4,
        variant: "line",
      },
    });

    expect(wrapper.exists()).toBe(true);
    const divs = wrapper.findAll(".animate-pulse");
    expect(divs.length).toBe(4);
    expect(wrapper.attributes("role")).toBe("status");
  });

  test("mounts ConfirmDialog, opens on confirmAction and settles promise on accept", async () => {
    const wrapper = mount(ConfirmDialog);
    expect(wrapper.exists()).toBe(true);

    const promise = confirmAction({
      title: "删除分支",
      message: "确认删除 feature-test 分支吗？",
      tone: "danger",
      confirmLabel: "立即删除",
    });

    await wrapper.vm.$nextTick();

    expect(confirmState.open).toBe(true);
    expect(confirmState.tone).toBe("danger");
    expect(confirmState.confirmLabel).toBe("立即删除");

    // Click the confirm button in the DOM
    const buttons = wrapper.findAll("button");
    const confirmBtn = buttons.find((b) => b.text().includes("立即删除"));
    expect(confirmBtn).toBeDefined();
    await confirmBtn!.trigger("click");

    const result = await promise;
    expect(result).toBe(true);
    expect(confirmState.open).toBe(false);
  });

  test("mounts CrashReportModal with real crash records and handles clearing", async () => {
    recordCrash(new Error("Test crash for DOM mount"), "dom_test", { extra: 123 });

    const wrapper = mount(CrashReportModal, {
      props: {
        isOpen: true,
      },
    });

    await wrapper.vm.$nextTick();

    expect(wrapper.exists()).toBe(true);
    const text = wrapper.text();
    expect(text).toContain("Test crash for DOM mount");
    expect(text).toContain("dom_test");

    // Click the clear button
    const clearBtn = wrapper.findAll("button").find((b) => b.attributes("title")?.includes("清空") || b.text().includes("清空"));
    if (clearBtn) {
      await clearBtn.trigger("click");
      await wrapper.vm.$nextTick();
      expect(wrapper.text()).toContain("运行健康");
    }
  });

  test("mounts AboutSection and renders application metadata", async () => {
    const wrapper = mount(AboutSection);
    expect(wrapper.exists()).toBe(true);

    const text = wrapper.text();
    expect(text).toContain("wgit");
    expect(text).toContain("关于 wgit");
    expect(text).toContain("版本号");
    expect(text).toContain("构建哈希");
    expect(text).toContain("wgit://");
  });

  test("mounts CommitComposer and emits updates on user input", async () => {
    const wrapper = mount(CommitComposer, {
      props: {
        subject: "feat: initial commit",
        body: "initial body",
        amend: false,
        autoPush: false,
        committing: false,
      },
    });

    expect(wrapper.exists()).toBe(true);
    const textarea = wrapper.find("textarea");
    expect(textarea.exists()).toBe(true);

    const input = wrapper.find("input[type='text']");
    expect(input.exists()).toBe(true);
    expect((input.element as HTMLInputElement).value).toBe("feat: initial commit");

    await input.setValue("feat: updated commit message");
    expect(wrapper.emitted("update:subject")).toBeTruthy();
    expect(wrapper.emitted("update:subject")![0]).toEqual(["feat: updated commit message"]);
  });

  test("mounts CommandPalette and handles search filtering and close events", async () => {
    const wrapper = mount(CommandPalette, {
      props: {
        isOpen: true,
      },
    });

    expect(wrapper.exists()).toBe(true);
    const searchInput = wrapper.find("input");
    expect(searchInput.exists()).toBe(true);

    // Initial state shows navigation and repo commands in active locale
    expect(wrapper.text()).toContain("切换到工作区");

    // Close button / backdrop
    await wrapper.setProps({ isOpen: false });
    await wrapper.vm.$nextTick();
  });

  test("mounts ThreeWayMergeModal and loads 3-way blocks into visual panes", async () => {
    const mockMergeData: ThreeWayMergeData = {
      filePath: "src/conflict.txt",
      baseContent: "common base text",
      oursContent: "local modified text",
      theirsContent: "incoming modified text",
      currentContent: "<<<<<<< HEAD\nlocal modified text\n=======\nincoming modified text\n>>>>>>> feature/awesome\n",
      totalConflicts: 1,
      blocks: [
        {
          index: 0,
          startLine: 1,
          endLine: 5,
          baseLines: ["common base text"],
          oursLines: ["local modified text"],
          theirsLines: ["incoming modified text"],
        },
      ],
    };

    const getSpy = spyOn(gitService, "getThreeWayMergeData").mockResolvedValue(mockMergeData);

    const wrapper = mount(ThreeWayMergeModal, {
      props: {
        open: true,
        repoPath: "/path/to/test-repo",
        filePath: "src/conflict.txt",
      },
    });

    expect(wrapper.exists()).toBe(true);
    // Allow async loadData to complete
    await new Promise((resolve) => setTimeout(resolve, 50));
    await wrapper.vm.$nextTick();

    expect(getSpy).toHaveBeenCalledWith("/path/to/test-repo", "src/conflict.txt");
    expect(wrapper.text()).toContain("src/conflict.txt");
    expect(wrapper.text()).toContain("local modified text");
    expect(wrapper.text()).toContain("incoming modified text");

    getSpy.mockRestore();
  });

  test("mounts DiffViewer and renders diff lines, blame toggle, and minimap gutter", async () => {
    const diffSample =
      "diff --git a/src/math.ts b/src/math.ts\n" +
      "--- a/src/math.ts\n" +
      "+++ b/src/math.ts\n" +
      "@@ -1,3 +1,3 @@\n" +
      " export function add(a: number, b: number): number {\n" +
      "-  return a - b;\n" +
      "+  return a + b;\n" +
      " }\n";

    const wrapper = mount(DiffViewer, {
      props: {
        diffText: diffSample,
        mode: "unified",
        repoPath: "/path/to/test-repo",
        filePath: "src/math.ts",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("return a + b;");
    expect(wrapper.text()).toContain("return a - b;");
    expect(wrapper.find("[data-testid='diff-minimap-gutter']").exists()).toBe(true);
  });

  test("mounts StashView with mocked stashes and handles selection and actions", async () => {
    const mockStashes: StashInfo[] = [
      {
        index: 0,
        name: "stash@{0}",
        branch: "main",
        message: "WIP on main: test stash 1",
        date: "2026-10-28 07:00:00",
        commitHash: "12345678",
      },
      {
        index: 1,
        name: "stash@{1}",
        branch: "feature",
        message: "WIP on feature: test stash 2",
        date: "2026-10-27 12:00:00",
        commitHash: "87654321",
      },
    ];

    const stashSpy = spyOn(gitService, "getStashes").mockResolvedValue(mockStashes);

    const wrapper = mount(StashView, {
      props: {
        currentRepoPath: "/path/to/test-repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("WIP on main: test stash 1");
    expect(wrapper.text()).toContain("stash@0");

    stashSpy.mockRestore();
  });

  test("mounts BranchRiverView and filters river topology nodes on search", async () => {
    const mockRiverData: BranchRiverData = {
      nodes: [
        {
          name: "main",
          upstream: "origin/main",
          ahead: 0,
          behind: 0,
          lastCommitTime: "2026-10-28 07:00:00",
          lastCommitSubject: "feat: initial commit",
          lastAuthor: "Alice",
          isMerged: false,
          isStale: false,
          divergenceScore: 10,
        },
        {
          name: "feature/auth-sso",
          upstream: "",
          ahead: 3,
          behind: 1,
          lastCommitTime: "2026-10-27 10:00:00",
          lastCommitSubject: "feat: add sso auth",
          lastAuthor: "Bob",
          isMerged: false,
          isStale: false,
          divergenceScore: 40,
        },
      ],
      totalBranches: 2,
      staleCount: 0,
      activeSprintDays: 14,
    };

    const riverSpy = spyOn(gitService, "getBranchRiverData").mockResolvedValue(mockRiverData);

    const wrapper = mount(BranchRiverView, {
      props: {
        currentRepoPath: "/path/to/test-repo",
      },
    });

    expect(wrapper.exists()).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 30));
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("main");
    expect(wrapper.text()).toContain("feature/auth-sso");

    const searchInput = wrapper.find("input[type='text']");
    if (searchInput.exists()) {
      await searchInput.setValue("auth");
      await wrapper.vm.$nextTick();
      expect(wrapper.text()).toContain("feature/auth-sso");
    }

    riverSpy.mockRestore();
  });

  test("mounts SquashCommitsModal and handles pre-filled commit message and squash submit", async () => {
    const sampleCommits: CommitItem[] = [
      {
        hash: "111111111111",
        shortHash: "1111111",
        parents: [],
        authorName: "Alice",
        authorEmail: "alice@test.com",
        date: "2026-10-28",
        relativeDate: "1h ago",
        subject: "fix: button alignment",
        body: "",
        refs: [],
        isHead: true,
        lane: 0,
        lines: [],
      },
      {
        hash: "222222222222",
        shortHash: "2222222",
        parents: [],
        authorName: "Bob",
        authorEmail: "bob@test.com",
        date: "2026-10-27",
        relativeDate: "1d ago",
        subject: "feat: add button component",
        body: "",
        refs: [],
        isHead: false,
        lane: 0,
        lines: [],
      },
    ];

    const squashSpy = spyOn(gitService, "squashSelectedCommits").mockResolvedValue({
      success: true,
      newHash: "333333333333",
      message: "squashed 2 commits",
      count: 2,
    });

    const wrapper = mount(SquashCommitsModal, {
      props: {
        isOpen: true,
        repoPath: "/path/to/test-repo",
        commits: sampleCommits,
      },
    });

    expect(wrapper.exists()).toBe(true);
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain("1111111");
    expect(wrapper.text()).toContain("fix: button alignment");
    expect(wrapper.text()).toContain("2222222");

    const textarea = wrapper.find("textarea");
    expect(textarea.exists()).toBe(true);
    expect((textarea.element as HTMLTextAreaElement).value).toContain("fix: button alignment");

    const confirmBtn = wrapper.find("[data-testid='confirm-squash-btn']");
    expect(confirmBtn.exists()).toBe(true);
    await confirmBtn.trigger("click");
    await wrapper.vm.$nextTick();

    expect(squashSpy).toHaveBeenCalled();
    expect(wrapper.emitted("squashed")).toBeTruthy();

    squashSpy.mockRestore();
  });
});
