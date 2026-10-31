import { afterEach, describe, expect, test } from "bun:test";
import { writeClipboardText } from "../src/utils/clipboard";
import "./domSetup";

const originalDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");

function stubClipboard(writeText: (text: string) => Promise<void>) {
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText },
  });
}

afterEach(() => {
  if (originalDescriptor) {
    Object.defineProperty(navigator, "clipboard", originalDescriptor);
  } else {
    delete (navigator as { clipboard?: unknown }).clipboard;
  }
});

describe("writeClipboardText", () => {
  test("resolves null after a successful write", async () => {
    const seen: string[] = [];
    stubClipboard(async (text) => {
      seen.push(text);
    });

    expect(await writeClipboardText("git status")).toBeNull();
    expect(seen).toEqual(["git status"]);
  });

  test("returns the error message instead of rejecting when the host denies the write", async () => {
    stubClipboard(async () => {
      throw new Error("Clipboard write denied");
    });

    expect(await writeClipboardText("git status")).toBe("Clipboard write denied");
  });
});
