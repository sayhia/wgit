import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import {
  SHORTCUT_ACTIONS,
  bindingToString,
  effectiveBinding,
  findShortcutConflict,
  formatBinding,
  loadShortcutOverrides,
  matchesBinding,
  parseBinding,
  saveShortcutOverrides,
  type ShortcutOverrides,
} from "../src/utils/shortcuts";

/** Minimal localStorage so the tests never depend on a browser runtime. */
function installStorage(initial: Record<string, string> = {}) {
  const store = new Map(Object.entries(initial));
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => void store.set(key, value),
      removeItem: (key: string) => void store.delete(key),
      clear: () => store.clear(),
      key: (index: number) => Array.from(store.keys())[index] ?? null,
      get length() {
        return store.size;
      },
    },
  });
  return store;
}

function keyEvent(init: Partial<KeyboardEvent>): KeyboardEvent {
  return {
    key: "",
    metaKey: false,
    ctrlKey: false,
    shiftKey: false,
    altKey: false,
    ...init,
  } as KeyboardEvent;
}

beforeEach(() => installStorage());
afterEach(() => installStorage());

describe("parseBinding", () => {
  test("parses modifiers and key", () => {
    expect(parseBinding("Meta+Shift+O")).toEqual({ key: "O", meta: true, shift: true, alt: false });
    expect(parseBinding("Alt+S")).toEqual({ key: "S", meta: false, shift: false, alt: true });
    expect(parseBinding("Meta+Enter")).toEqual({ key: "Enter", meta: true, shift: false, alt: false });
  });

  test("treats Ctrl and Control as Meta so macOS and Windows share one registry", () => {
    expect(parseBinding("Ctrl+K")).toEqual(parseBinding("Control+K"));
    expect(parseBinding("Ctrl+K")).toEqual(parseBinding("Meta+K"));
  });

  test("returns null without a non-modifier key", () => {
    expect(parseBinding("Meta+Shift")).toBeNull();
    expect(parseBinding("")).toBeNull();
  });

  test("keeps the last non-modifier key when several are given", () => {
    // Not a supported syntax, but it must not throw or lose the key.
    expect(parseBinding("Meta+A+B")?.key).toBe("B");
  });
});

describe("bindingToString", () => {
  test("normalizes modifier order to Alt+Meta+Shift", () => {
    expect(bindingToString({ key: "o", meta: true, shift: true, alt: false })).toBe("Meta+Shift+O");
    expect(bindingToString({ key: "s", meta: false, shift: false, alt: true })).toBe("Alt+S");
    expect(bindingToString({ key: "K", meta: true, shift: false, alt: true })).toBe("Alt+Meta+K");
  });

  test("round-trips through parseBinding", () => {
    for (const action of SHORTCUT_ACTIONS) {
      const parsed = parseBinding(action.defaultBinding);
      expect(parsed).not.toBeNull();
      expect(bindingToString(parsed!)).toBe(action.defaultBinding);
    }
  });
});

describe("formatBinding", () => {
  test("renders chip text with glyphs", () => {
    expect(formatBinding("Meta+Shift+O")).toBe("⌘ ⇧ O");
    expect(formatBinding("Alt+S")).toBe("⌥ S");
  });

  test("maps Enter to the return glyph", () => {
    expect(formatBinding("Meta+Enter")).toBe("⌘ ↵");
  });

  test("falls back to the raw string when unparseable", () => {
    expect(formatBinding("Meta+Shift")).toBe("Meta+Shift");
  });
});

describe("matchesBinding", () => {
  test("matches exact modifier combinations", () => {
    expect(matchesBinding(keyEvent({ key: "k", metaKey: true }), "Meta+K")).toBe(true);
    expect(matchesBinding(keyEvent({ key: "k", ctrlKey: true }), "Meta+K")).toBe(true);
    expect(matchesBinding(keyEvent({ key: "K", metaKey: true, shiftKey: true }), "Meta+K")).toBe(false);
  });

  test("rejects when a required modifier is missing", () => {
    expect(matchesBinding(keyEvent({ key: "s", altKey: false }), "Alt+S")).toBe(false);
  });

  test("rejects when an extra modifier is present", () => {
    expect(matchesBinding(keyEvent({ key: "o", metaKey: true, shiftKey: true }), "Meta+O")).toBe(false);
  });

  test("ignores key case", () => {
    expect(matchesBinding(keyEvent({ key: "p", metaKey: true }), "Meta+P")).toBe(true);
  });

  test("handles Enter and Spacebar aliases", () => {
    expect(matchesBinding(keyEvent({ key: "Enter", metaKey: true }), "Meta+Enter")).toBe(true);
    expect(matchesBinding(keyEvent({ key: "Spacebar" }), "Space")).toBe(true);
  });
});

describe("shortcut registry integrity", () => {
  test("ids are unique", () => {
    const ids = SHORTCUT_ACTIONS.map((a) => a.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  test("default bindings are unique across customizable actions", () => {
    const bindings = SHORTCUT_ACTIONS.filter((a) => a.customizable).map((a) =>
      a.defaultBinding.toUpperCase()
    );
    expect(new Set(bindings).size).toBe(bindings.length);
  });

  test("every default binding parses", () => {
    for (const action of SHORTCUT_ACTIONS) {
      expect(parseBinding(action.defaultBinding)).not.toBeNull();
    }
  });

  test("component-owned actions declare an event name", () => {
    for (const action of SHORTCUT_ACTIONS.filter((a) => a.owner === "component")) {
      expect(action.eventName).toBeTruthy();
    }
  });
});

describe("overrides", () => {
  test("effectiveBinding prefers the override", () => {
    const action = SHORTCUT_ACTIONS[0]!;
    const overrides: ShortcutOverrides = { [action.id]: "Meta+J" };
    expect(effectiveBinding(action, overrides)).toBe("Meta+J");
    expect(effectiveBinding(action, {})).toBe(action.defaultBinding);
  });

  test("loadShortcutOverrides drops unknown ids and invalid bindings", () => {
    installStorage({
      "wgit.shortcuts.v1": JSON.stringify({
        "command-palette": "Meta+J",
        "not-an-action": "Meta+L",
        "open-repo": "Meta+Shift", // no key → invalid
      }),
    });
    const overrides = loadShortcutOverrides();
    expect(overrides["command-palette"]).toBe("Meta+J");
    expect(overrides["not-an-action"]).toBeUndefined();
    expect(overrides["open-repo"]).toBeUndefined();
  });

  test("loadShortcutOverrides survives malformed JSON", () => {
    installStorage({ "wgit.shortcuts.v1": "{not json" });
    expect(loadShortcutOverrides()).toEqual({});
  });

  test("loadShortcutOverrides rejects non-customizable actions", () => {
    const locked = SHORTCUT_ACTIONS.find((a) => !a.customizable);
    const payload = locked ? { [locked.id]: "Meta+1" } : { "command-palette": "Meta+J" };
    installStorage({ "wgit.shortcuts.v1": JSON.stringify(payload) });
    const overrides = loadShortcutOverrides();
    if (locked) expect(overrides[locked.id]).toBeUndefined();
  });

  test("saveShortcutOverrides round-trips", () => {
    saveShortcutOverrides({ "command-palette": "Meta+J" });
    expect(loadShortcutOverrides()["command-palette"]).toBe("Meta+J");
  });
});

describe("findShortcutConflict", () => {
  test("finds the action already bound to a binding", () => {
    const conflict = findShortcutConflict("Meta+K", {});
    expect(conflict?.id).toBe("command-palette");
  });

  test("is case-insensitive", () => {
    expect(findShortcutConflict("meta+k", {})?.id).toBe("command-palette");
  });

  test("ignores the action being rebound", () => {
    expect(findShortcutConflict("Meta+K", {}, "command-palette")).toBeUndefined();
  });

  test("respects overrides when detecting conflicts", () => {
    // Give open-repo the same binding as the palette.
    const conflict = findShortcutConflict("Meta+J", { "open-repo": "Meta+J" }, "command-palette");
    expect(conflict?.id).toBe("open-repo");
  });

  test("returns undefined for an unused binding", () => {
    expect(findShortcutConflict("Meta+Alt+9", {})).toBeUndefined();
  });
});
