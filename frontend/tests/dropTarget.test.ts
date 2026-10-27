import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const ROOT = join(import.meta.dir, "..", "..");

describe("folder drop", () => {
  const app = readFileSync(join(ROOT, "frontend/src/App.vue"), "utf8");
  const mainGo = readFileSync(join(ROOT, "main.go"), "utf8");
  const css = readFileSync(join(ROOT, "frontend/src/style.css"), "utf8");

  test("the window enables native file drops", () => {
    // Without this the platform layer never reports a drop, and the HTML5 route
    // cannot substitute: WKWebView leaves `File.path` undefined.
    expect(mainGo).toContain("EnableFileDrop: true");
    expect(mainGo).toContain("events.Common.WindowFilesDropped");
    expect(mainGo).toContain('EmitEvent("wgit:open-repo-path"');
  });

  test("the root element is the drop target", () => {
    const root = app.match(/<template>\s*<div([\s\S]*?)>/);
    expect(root).not.toBeNull();
    expect(root![1]).toContain("data-file-drop-target");
    // The hint rides in an attribute so the CSS overlay can stay translated.
    expect(root![1]).toContain(":data-drop-hint=");
  });

  test("nothing reads File.path any more", () => {
    // `(file as any).path` is Electron-only. Leaving it in place made the drop
    // look implemented while doing nothing on macOS.
    expect(app).not.toMatch(/\.path\b.*dataTransfer|dataTransfer[\s\S]{0,200}\.path/);
    expect(app).not.toContain("as any)?.path");
  });

  test("the drag affordance is painted from the runtime class", () => {
    expect(css).toContain(".file-drop-target-active::after");
    expect(css).toContain("content: attr(data-drop-hint)");
    expect(css).toContain("pointer-events: none");
  });

  test("the drop path is wired to the existing repository validation", () => {
    expect(app).toContain('addEventListener("wgit:open-repo-path"');
    expect(app).toContain("await openRepo(path)");
  });
});
