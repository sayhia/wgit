import { afterEach, describe, expect, test } from "bun:test";
import { defineComponent, ref, nextTick } from "vue";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import UiModal from "../src/components/ui/UiModal.vue";
import "./domSetup";

let wrapper: VueWrapper | null = null;

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

describe("UiModal shell", () => {
  test("focuses inside, closes on Escape and returns focus to the trigger", async () => {
    const Host = defineComponent({
      components: { UiModal },
      setup() {
        const open = ref(false);
        return { open };
      },
      template: `
        <button id="trigger" @click="open = true">open</button>
        <UiModal :open="open" title="Test" @update:open="open = $event">
          <button id="inside">inside</button>
        </UiModal>
      `,
    });
    wrapper = mount(Host, { attachTo: document.body });
    const trigger = wrapper.find("#trigger");
    (trigger.element as HTMLElement).focus();
    await trigger.trigger("click");
    await flushPromises();

    expect(wrapper.find('[role="dialog"]').exists()).toBe(true);

    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    await flushPromises();
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false);
    expect(document.activeElement?.id).toBe("trigger");
  });

  test("backdrop only dismisses when close-on-backdrop is enabled", async () => {
    wrapper = mount(UiModal, {
      props: { open: true, title: "Test", closeOnBackdrop: false },
      slots: { default: "<button>inside</button>" },
    });
    await wrapper.find('[role="dialog"]').trigger("mousedown");
    expect(wrapper.emitted("update:open")).toBeFalsy();

    wrapper.unmount();
    wrapper = mount(UiModal, {
      props: { open: true, title: "Test", closeOnBackdrop: true },
      slots: { default: "<button>inside</button>" },
    });
    await wrapper.find('[role="dialog"]').trigger("mousedown");
    expect(wrapper.emitted("update:open")?.[0]).toEqual([false]);
  });

  test("canvas overlay and plain surface preserve the full-page card look", () => {
    wrapper = mount(UiModal, {
      props: {
        open: true,
        title: "Onboarding",
        variant: "bare",
        overlay: "canvas-strong",
        surface: "plain",
        width: "max-w-3xl",
      },
      slots: { default: "<div>content</div>" },
    });
    const overlay = wrapper.find('[role="dialog"]');
    expect(overlay.classes()).toContain("bg-canvas/95");
    expect(overlay.classes().some((name) => name.startsWith("backdrop-blur"))).toBe(false);
    const panel = overlay.element.firstElementChild as HTMLElement;
    expect(panel.className).toContain("rounded-[var(--ug-radius-xl)]");
    expect(panel.className).toContain("bg-surface");
    expect(panel.className).not.toContain("glass-panel");
  });
});
