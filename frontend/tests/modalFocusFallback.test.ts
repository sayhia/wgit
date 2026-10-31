import { describe, expect, test } from "bun:test";
import { defineComponent, nextTick } from "vue";
import { mount } from "@vue/test-utils";
import { useModalA11y } from "../src/composables/useModalA11y";
import "./domSetup";

const EmptyPanel = defineComponent({
  props: { open: { type: Boolean, default: false } },
  setup(props) {
    const { containerRef } = useModalA11y(
      () => props.open,
      () => {}
    );
    return { containerRef };
  },
  template: '<div v-if="open" ref="containerRef"><p>read only</p></div>',
});

describe("useModalA11y focus fallback", () => {
  test("focuses the dialog container when nothing inside is focusable", async () => {
    const trigger = document.createElement("button");
    document.body.appendChild(trigger);
    trigger.focus();

    const wrapper = mount(EmptyPanel, { props: { open: true }, attachTo: document.body });
    await nextTick();
    await nextTick();

    expect(document.activeElement).toBe(wrapper.element);
    expect(wrapper.element.getAttribute("tabindex")).toBe("-1");

    wrapper.unmount();
    trigger.remove();
  });
});
