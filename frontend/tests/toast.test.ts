import { beforeEach, describe, expect, test } from "bun:test";
import {
  dismissToast,
  pushToast,
  reportError,
  resetToasts,
  runToastAction,
  toasts,
  useToast,
} from "../src/composables/useToast";

beforeEach(() => resetToasts());

describe("toast store", () => {
  test("notify from any component lands in the one store", () => {
    // Two independent `useToast()` calls used to own two separate refs, so the
    // second toast silently replaced state the first view was rendering.
    const a = useToast();
    const b = useToast();
    a.notify("first");
    b.notify("second");
    expect(toasts.value.map((t) => t.text)).toEqual(["first", "second"]);
    expect(a.toastText.value).toBe("second");
    expect(b.showToast.value).toBe(true);
  });

  test("caps the stack and drops the oldest entry", () => {
    for (const text of ["one", "two", "three", "four"]) pushToast(text, { duration: 0 });
    expect(toasts.value.map((t) => t.text)).toEqual(["two", "three", "four"]);
  });

  test("repeating a message refreshes instead of stacking duplicates", () => {
    pushToast("Saved", { duration: 0 });
    pushToast("Saved", { duration: 0 });
    expect(toasts.value).toHaveLength(1);
  });

  test("a different tone is a different toast", () => {
    pushToast("Saved", { duration: 0 });
    pushToast("Saved", { duration: 0, tone: "danger" });
    expect(toasts.value).toHaveLength(2);
  });

  test("dismiss removes exactly one entry", () => {
    const first = pushToast("one", { duration: 0 });
    pushToast("two", { duration: 0 });
    dismissToast(first);
    expect(toasts.value.map((t) => t.text)).toEqual(["two"]);
  });

  test("an empty message is not a toast", () => {
    expect(pushToast("   ", { duration: 0 })).toBe(0);
    expect(toasts.value).toHaveLength(0);
  });

  test("undo runs the action and closes the toast", () => {
    let undone = false;
    pushToast("Discarded 3 files", {
      duration: 0,
      action: { label: "Undo", run: () => (undone = true) },
    });
    const id = toasts.value[0]!.id;
    runToastAction(id);
    expect(undone).toBe(true);
    expect(toasts.value).toHaveLength(0);
  });

  test("a throwing action is reported, not swallowed", () => {
    pushToast("Broken", {
      duration: 0,
      action: {
        label: "Undo",
        run: () => {
          throw new Error("fatal: nothing to undo");
        },
      },
    });
    runToastAction(toasts.value[0]!.id);
    const texts = toasts.value.map((t) => t.text);
    expect(texts.some((t) => t.includes("fatal: nothing to undo"))).toBe(true);
  });

  test("reportError keeps git's own wording", () => {
    const original = console.error;
    console.error = () => {};
    try {
      reportError("Load commits", new Error("fatal: unable to read tree abc"));
    } finally {
      console.error = original;
    }
    expect(toasts.value[0]!.tone).toBe("danger");
    expect(toasts.value[0]!.text).toBe("Load commits: fatal: unable to read tree abc");
  });

  test("reportError survives a non-Error rejection", () => {
    const original = console.error;
    console.error = () => {};
    try {
      reportError("Save", "boom");
    } finally {
      console.error = original;
    }
    expect(toasts.value[0]!.text).toBe("Save: boom");
  });
});
