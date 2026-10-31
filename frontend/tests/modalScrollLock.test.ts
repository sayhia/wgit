import { describe, expect, test } from "bun:test";
import { defineComponent, nextTick } from "vue";
import { mount } from "@vue/test-utils";
import { useModalA11y } from "../src/composables/useModalA11y";
import "./domSetup";

const Panel = defineComponent({
  props: { open: { type: Boolean, default: false } },
  setup(props) {
    const { containerRef } = useModalA11y(
      () => props.open,
      () => {}
    );
    return { containerRef };
  },
  template: '<div v-if="open" ref="containerRef"><button type="button">ok</button></div>',
});

describe("useModalA11y body scroll lock", () => {
  test("locks while a modal is open and restores after close", async () => {
    document.body.style.overflow = "";
    const wrapper = mount(Panel, { props: { open: false } });
    expect(document.body.style.overflow).toBe("");

    await wrapper.setProps({ open: true });
    await nextTick();
    expect(document.body.style.overflow).toBe("hidden");

    await wrapper.setProps({ open: false });
    await nextTick();
    expect(document.body.style.overflow).toBe("");
    wrapper.unmount();
  });

  test("nested modals keep the lock until the last one closes", async () => {
    document.body.style.overflow = "";
    const outer = mount(Panel, { props: { open: true } });
    const inner = mount(Panel, { props: { open: true } });
    await nextTick();
    expect(document.body.style.overflow).toBe("hidden");

    await inner.setProps({ open: false });
    await nextTick();
    expect(document.body.style.overflow).toBe("hidden");

    await outer.setProps({ open: false });
    await nextTick();
    expect(document.body.style.overflow).toBe("");
    outer.unmount();
    inner.unmount();
  });

  test("restores a pre-existing overflow value instead of clearing it", async () => {
    document.body.style.overflow = "scroll";
    const wrapper = mount(Panel, { props: { open: true } });
    await nextTick();
    expect(document.body.style.overflow).toBe("hidden");

    await wrapper.setProps({ open: false });
    await nextTick();
    expect(document.body.style.overflow).toBe("scroll");
    wrapper.unmount();
    document.body.style.overflow = "";
  });

  test("unmounting an open modal releases the lock", async () => {
    document.body.style.overflow = "";
    const wrapper = mount(Panel, { props: { open: true } });
    await nextTick();
    expect(document.body.style.overflow).toBe("hidden");

    wrapper.unmount();
    await nextTick();
    expect(document.body.style.overflow).toBe("");
  });
});
