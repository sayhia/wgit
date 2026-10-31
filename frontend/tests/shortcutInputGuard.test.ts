import { describe, expect, test } from "bun:test";
import { isTypingTarget, matchesBinding } from "../src/utils/shortcuts";
import "./domSetup";

function keydown(init: KeyboardEventInit, isComposing = false): KeyboardEvent {
  const event = new KeyboardEvent("keydown", init);
  Object.defineProperty(event, "isComposing", { value: isComposing });
  return event;
}

describe("shortcut input guards", () => {
  test("ignores bindings while an IME composition is active", () => {
    expect(matchesBinding(keydown({ key: "k", metaKey: true }), "Meta+K")).toBe(true);
    expect(matchesBinding(keydown({ key: "k", metaKey: true }, true), "Meta+K")).toBe(false);
  });

  test("recognizes every editable control as a typing target", () => {
    const input = document.createElement("input");
    const textarea = document.createElement("textarea");
    const select = document.createElement("select");
    const editable = document.createElement("div");
    editable.setAttribute("contenteditable", "true");
    const nested = document.createElement("span");
    editable.appendChild(nested);
    const button = document.createElement("button");
    const plain = document.createElement("div");

    expect(isTypingTarget(input)).toBe(true);
    expect(isTypingTarget(textarea)).toBe(true);
    expect(isTypingTarget(select)).toBe(true);
    expect(isTypingTarget(editable)).toBe(true);
    expect(isTypingTarget(nested)).toBe(true);
    expect(isTypingTarget(button)).toBe(false);
    expect(isTypingTarget(plain)).toBe(false);
    expect(isTypingTarget(null)).toBe(false);
  });
});
