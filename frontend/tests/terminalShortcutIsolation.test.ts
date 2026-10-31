import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { isTerminalTarget } from "../src/utils/shortcuts";
import "./domSetup";

describe("terminal shortcut isolation", () => {
  test("recognizes the xterm host as terminal-owned focus", () => {
    const host = document.createElement("div");
    host.className = "xterm";
    const helper = document.createElement("textarea");
    host.appendChild(helper);

    expect(isTerminalTarget(helper)).toBe(true);
    expect(isTerminalTarget(host)).toBe(true);
    expect(isTerminalTarget(document.createElement("input"))).toBe(false);
    expect(isTerminalTarget(null)).toBe(false);
  });

  test("App defers Cmd/Ctrl+K to the terminal instead of opening the palette", () => {
    const app = readFileSync(
      fileURLToPath(new URL("../src/App.vue", import.meta.url)),
      "utf8"
    );
    expect(app).toContain("isTerminalTarget(e.target)");
    expect(app).toMatch(/isTerminalTarget\(e\.target\)\) return;/);
  });
});
