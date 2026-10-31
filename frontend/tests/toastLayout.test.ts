import { afterEach, beforeEach, describe, expect, spyOn, test } from "bun:test";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import Toast from "../src/components/ui/Toast.vue";
import { pushToast, resetToasts, toasts } from "../src/composables/useToast";
import "./domSetup";

let wrapper: VueWrapper | undefined;
const originalHeight = window.innerHeight;
beforeEach(() => resetToasts());
afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  resetToasts();
  Object.defineProperty(window, "innerHeight", { configurable: true, value: originalHeight });
});

describe("toast layout reservation", () => {
  test("measures the stack, bottom edge and gap, then releases space on close and unmount", async () => {
    Object.defineProperty(window, "innerHeight", { configurable: true, value: 400 });
    pushToast("An operation failed", { tone: "danger", duration: 0 });
    wrapper = mount(Toast, { attachTo: document.body });
    await flushPromises();
    const stack = wrapper.get('[data-testid="toast-stack"]');
    let height = 120;
    const bounds = spyOn(stack.element, "getBoundingClientRect").mockImplementation(
      () => new DOMRect(600, 380 - height, 340, height)
    );
    window.dispatchEvent(new Event("resize"));
    expect(document.documentElement.style.getPropertyValue("--ug-toast-reserve")).toBe("148px");

    height = 64;
    window.dispatchEvent(new Event("resize"));
    // Reservation follows the painted top edge, not an assumed toast count.
    expect(document.documentElement.style.getPropertyValue("--ug-toast-reserve")).toBe("92px");
    await wrapper.get('[data-testid="toast-item"] button[aria-label]').trigger("click");
    height = 0;
    window.dispatchEvent(new Event("resize"));
    expect(document.documentElement.style.getPropertyValue("--ug-toast-reserve")).toBe("");

    height = 120;
    window.dispatchEvent(new Event("resize"));
    wrapper.unmount();
    wrapper = undefined;
    expect(document.documentElement.style.getPropertyValue("--ug-toast-reserve")).toBe("");
    bounds.mockRestore();
  });

  test("the bounded stack keeps errors, undo and hover pausing functional", async () => {
    let undone = false;
    pushToast("Change can be undone", { duration: 5000, action: { label: "Undo", run: () => { undone = true; } } });
    pushToast("Detailed provider failure", { tone: "danger", duration: 0 });
    wrapper = mount(Toast);
    await flushPromises();
    const notices = wrapper.findAll('[data-testid="toast-item"]');
    expect(notices).toHaveLength(2);
    expect(notices[1]!.attributes("role")).toBe("alert");
    await notices[0]!.trigger("mouseenter");
    expect(toasts.value).toHaveLength(2);
    await wrapper.get('[data-testid="toast-undo-btn"]').trigger("click");
    expect(undone).toBe(true);
    expect(toasts.value.map((toast) => toast.text)).toEqual(["Detailed provider failure"]);
  });
});
