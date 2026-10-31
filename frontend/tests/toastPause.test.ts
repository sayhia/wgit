import { beforeEach, describe, expect, test } from "bun:test";
import {
  pauseToast,
  pushToast,
  resetToasts,
  resumeToast,
  toasts,
} from "../src/composables/useToast";

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

beforeEach(() => {
  resetToasts();
});

describe("toast timing control", () => {
  test("hover/focus pauses auto-dismiss and resumes the remaining time", async () => {
    const id = pushToast("saving failed", { tone: "danger", duration: 80 });
    pauseToast(id);
    await sleep(140);
    expect(toasts.value.some((toast) => toast.id === id)).toBe(true);

    resumeToast(id);
    await sleep(120);
    expect(toasts.value.some((toast) => toast.id === id)).toBe(false);
  });

  test("resuming a toast with no remaining time dismisses it immediately", async () => {
    const id = pushToast("expired", { duration: 30 });
    await sleep(60);
    resumeToast(id);
    expect(toasts.value.some((toast) => toast.id === id)).toBe(false);
  });
});
