import { afterEach, describe, expect, spyOn, test } from "bun:test";
import { flushPromises, mount } from "@vue/test-utils";
import GitBundleModal from "../src/components/tools/GitBundleModal.vue";
import { gitService } from "../src/services/gitService";
import "./domSetup";

afterEach(() => {
  Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true });
});

describe("GitBundleModal copy feedback", () => {
  test("shows an alert instead of pretending a failed clipboard write succeeded", async () => {
    const exportSpy = spyOn(gitService, "createGitBundleAdvanced").mockResolvedValue({
      outputPath: "/tmp/repo.bundle",
      sizeHuman: "1.2 MB",
      cloneHelp: "git clone repo.bundle repo",
      verifyHelp: "git bundle verify repo.bundle",
    } as never);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: async () => Promise.reject(new Error("clipboard denied")) },
      configurable: true,
    });
    try {
      const wrapper = mount(GitBundleModal, {
        props: { open: true, currentRepoPath: "/repo" },
      });
      await wrapper.find("[data-testid='bundle-export']").trigger("click");
      await flushPromises();
      await wrapper.find("[data-testid='bundle-copy-path']").trigger("click");
      await flushPromises();

      const alert = wrapper.find("[role='alert']");
      expect(alert.exists()).toBe(true);
      expect(alert.text()).toContain("clipboard denied");
      expect(wrapper.find("[data-testid='bundle-copy-path']").attributes("aria-label")).toBeTruthy();
    } finally {
      exportSpy.mockRestore();
    }
  });
});
