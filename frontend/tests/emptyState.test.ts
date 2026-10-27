import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const SRC = join(import.meta.dir, "..", "src");
const read = (relative: string) => readFileSync(join(SRC, relative), "utf8");

describe("empty state contract", () => {
  test("UiEmpty offers icon + title + hint + one action slot", () => {
    // The contract is deliberately narrow: a list with no rows says what is
    // missing, in one sentence, and offers at most one way forward. Everything
    // else (three buttons, a paragraph of explanation) turns an empty state into
    // a second interface.
    const ui = read("components/ui/UiEmpty.vue");
    expect(ui).toContain("title: string");
    expect(ui).toContain("hint?: string");
    expect(ui).toContain("icon?: Component");
    expect(ui).toContain("<slot />");
  });

  test("the data lists share the component instead of hand-rolling one", () => {
    // Three different empty states used to mean three different visual
    // languages: a dashed card, a bare sentence, an icon block with two lines.
    for (const file of [
      "components/history/CommitGraphList.vue",
      "components/StashView.vue",
      "components/branches/LocalBranchList.vue",
    ]) {
      const source = read(file);
      expect(source).toContain("UiEmpty");
      expect(source).toMatch(/<UiEmpty[\s\S]{0,200}:title=/);
    }
  });

  test("empty states keep an icon rather than text alone", () => {
    // A bare sentence in the middle of a pane reads like a rendering failure.
    const history = read("components/history/CommitGraphList.vue");
    expect(history).toMatch(/<UiEmpty[\s\S]{0,160}:icon=/);
  });
});
