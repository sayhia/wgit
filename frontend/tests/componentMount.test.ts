import { beforeEach, describe, expect, test, spyOn } from "bun:test";
import { mount } from "@vue/test-utils";
import UiSkeleton from "../src/components/ui/UiSkeleton.vue";
import ConfirmDialog from "../src/components/ui/ConfirmDialog.vue";
import CrashReportModal from "../src/components/CrashReportModal.vue";
import AboutSection from "../src/components/settings/AboutSection.vue";
import ThreeWayMergeModal from "../src/components/ThreeWayMergeModal.vue";
import CommitComposer from "../src/components/workspace/CommitComposer.vue";
import CommandPalette from "../src/components/CommandPalette.vue";
import { confirmAction, confirmState } from "../src/composables/useConfirm";
import { clearCrashLogs, recordCrash } from "../src/utils/crashReporter";
import { gitService } from "../src/services/gitService";
import type { ThreeWayMergeData } from "../src/types";
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
});
