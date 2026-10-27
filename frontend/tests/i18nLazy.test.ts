import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { locale, registerCatalog, setWarnMissing, t } from "../src/i18n";
import enUSModule from "../src/i18n/messages/en-US";

// Depending on the loader, the default export is either the dictionary or a
// namespace wrapping it; take whichever is there.
const realEnUS = ((enUSModule as unknown as { default?: unknown }).default ?? enUSModule) as Record<
  string,
  string
>;

const SOURCE = readFileSync(join(import.meta.dir, "..", "src/i18n/index.ts"), "utf8");

describe("lazy catalog", () => {
  test("a late-arriving catalog invalidates rendered strings", () => {
    // The catalog object is plain, so assigning into it is invisible to Vue.
    // Without a version ref that `translate()` reads, a component rendered
    // before the chunk arrives keeps the fallback language — the commit
    // composer stayed Chinese in an English UI because of exactly that.
    expect(SOURCE).toContain("catalogVersion");
    expect(SOURCE).toMatch(/loadLocale[\s\S]{0,400}catalogVersion\.value\+\+/);
    expect(SOURCE).toMatch(/translate\([\s\S]{0,300}void catalogVersion\.value/);
  });

  test("registering a catalog takes effect immediately", () => {
    setWarnMissing(false);
    const previous = locale.value;
    locale.value = "en-US";
    try {
      registerCatalog("en-US", { "test.lazy.key": "Late arrival" });
      expect(t("test.lazy.key")).toBe("Late arrival");
    } finally {
      // `registerCatalog` swaps the whole dictionary, so the real one has to go
      // back — otherwise every later test that asks for English gets the stub.
      registerCatalog("en-US", realEnUS);
      locale.value = previous;
      setWarnMissing(true);
    }
  });
});
