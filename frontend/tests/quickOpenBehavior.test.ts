import { afterEach, describe, expect, test } from "bun:test";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import QuickOpen from "../src/components/QuickOpen.vue";
import "./domSetup";

let wrapper: VueWrapper | null = null;

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

describe("QuickOpen through UiModal", () => {
  test("opens from the global event, renders one dialog and closes on Escape", async () => {
    wrapper = mount(QuickOpen, { props: { currentRepoPath: "" } });
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false);

    window.dispatchEvent(new CustomEvent("wgit:quick-open", { detail: { mode: "files" } }));
    await flushPromises();

    const dialog = wrapper.find('[role="dialog"]');
    expect(dialog.exists()).toBe(true);
    expect(dialog.attributes("aria-label")).toContain("快速打开");
    expect(wrapper.findAll('[role="dialog"]')).toHaveLength(1);
    expect(wrapper.find("input").exists()).toBe(true);

    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    await flushPromises();

    expect(wrapper.find('[role="dialog"]').exists()).toBe(false);
  });

  test("closes when the overlay itself is pressed", async () => {
    wrapper = mount(QuickOpen, { props: { currentRepoPath: "" } });
    window.dispatchEvent(new CustomEvent("wgit:quick-open", { detail: { mode: "code" } }));
    await flushPromises();

    await wrapper.find('[role="dialog"]').trigger("mousedown");
    await flushPromises();

    expect(wrapper.find('[role="dialog"]').exists()).toBe(false);
  });
});
