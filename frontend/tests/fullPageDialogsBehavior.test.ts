import { afterEach, describe, expect, spyOn, test } from "bun:test";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import RepoOnboarding from "../src/components/RepoOnboarding.vue";
import RepoDashboard from "../src/components/RepoDashboard.vue";
import { gitService } from "../src/services/gitService";
import "./domSetup";

let wrapper: VueWrapper | null = null;

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

describe("full-page dialogs through UiModal", () => {
  test("RepoOnboarding renders one canvas dialog and only closes when canClose allows it", async () => {
    const recents = spyOn(gitService, "getRecentRepos").mockResolvedValue([]);
    try {
      wrapper = mount(RepoOnboarding, { props: { canClose: false } });
      await flushPromises();
      const dialog = wrapper.find('[role="dialog"]');
      expect(dialog.exists()).toBe(true);
      expect(dialog.classes()).toContain("bg-canvas/95");

      document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
      await flushPromises();
      expect(wrapper.emitted("close")).toBeFalsy();

      await wrapper.setProps({ canClose: true });
      document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
      await flushPromises();
      expect(wrapper.emitted("close")).toBeTruthy();
      expect(wrapper.findAll('[role="dialog"]')).toHaveLength(1);
    } finally {
      recents.mockRestore();
    }
  });

  test("RepoDashboard keeps one dialog and closes through the shared shell", async () => {
    wrapper = mount(RepoDashboard, {
      props: { repos: [], activeRepo: null, open: true },
    });
    await flushPromises();
    const dialog = wrapper.find('[role="dialog"]');
    expect(dialog.exists()).toBe(true);
    expect(dialog.classes()).toContain("bg-canvas/80");

    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    await flushPromises();
    expect(wrapper.emitted("close-overlay")).toBeTruthy();
  });
});
