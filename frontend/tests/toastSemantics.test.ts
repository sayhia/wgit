import { beforeEach, describe, expect, test } from "bun:test";
import { mount } from "@vue/test-utils";
import Toast from "../src/components/ui/Toast.vue";
import { pushToast, resetToasts } from "../src/composables/useToast";
import "./domSetup";

beforeEach(() => {
  resetToasts();
});

describe("toast live-region semantics", () => {
  test("danger toasts interrupt as alerts so errors are not silent", () => {
    pushToast("push rejected", { tone: "danger", duration: 0 });
    const wrapper = mount(Toast);
    const item = wrapper.find('[data-testid="toast-item"]');
    expect(item.attributes("role")).toBe("alert");
    expect(item.attributes("aria-live")).toBe("assertive");
    wrapper.unmount();
  });

  test("informational toasts stay polite status messages", () => {
    pushToast("copied", { tone: "info", duration: 0 });
    const wrapper = mount(Toast);
    const item = wrapper.find('[data-testid="toast-item"]');
    expect(item.attributes("role")).toBe("status");
    expect(item.attributes("aria-live")).toBe("polite");
    wrapper.unmount();
  });
});
