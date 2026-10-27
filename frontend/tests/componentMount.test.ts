import { beforeEach, describe, expect, test } from "bun:test";
import { mount } from "@vue/test-utils";
import UiSkeleton from "../src/components/ui/UiSkeleton.vue";
import ConfirmDialog from "../src/components/ui/ConfirmDialog.vue";
import CrashReportModal from "../src/components/CrashReportModal.vue";
import AboutSection from "../src/components/settings/AboutSection.vue";
import { confirmAction, confirmState } from "../src/composables/useConfirm";
import { clearCrashLogs, recordCrash } from "../src/utils/crashReporter";
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
});
