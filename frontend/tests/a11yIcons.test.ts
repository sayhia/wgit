/**
 * a11y icon guard — U5 incremental slice 1.
 *
 * Scope: self-closing lucide icons in src/components/ui/*.vue and
 * src/components/branches/*.vue (modals excluded). Every icon sitting inside
 * an interactive element (button/a/label) that ALREADY has an accessible name
 * (aria-label/title attribute, or a non-icon text sibling) got
 * aria-hidden="true" (48 additions across 9 files). Icons that are the sole
 * content of an UNNAMED interactive element must NEVER get aria-hidden (that
 * would make the control nameless) — this slice found none of those in scope;
 * the reverse-protection tests below pin the invariant at three sampled sites
 * so a future refactor that strips a button's naming source cannot silently
 * leave a hidden icon behind.
 */
import { describe, test, expect } from "bun:test";
import { readFileSync } from "fs";
import { resolve } from "path";

const read = (rel: string) => readFileSync(resolve(__dirname, "..", rel), "utf-8");

/** aria-hidden="true" additions per file in this slice (floor guards). */
const ADDED_ARIA_HIDDEN: Record<string, number> = {
  "src/components/ui/UiModal.vue": 1,
  "src/components/ui/AiReasoningPanel.vue": 1,
  "src/components/ui/Toast.vue": 1,
  "src/components/branches/RemoteBranchList.vue": 6,
  "src/components/branches/StashesList.vue": 7,
  "src/components/branches/BranchRiverView.vue": 5,
  "src/components/branches/TagsList.vue": 5,
  "src/components/branches/BranchCompare.vue": 11,
  "src/components/branches/LocalBranchList.vue": 11,
};

describe("a11y icons: aria-hidden floors (never decrease)", () => {
  for (const [file, floor] of Object.entries(ADDED_ARIA_HIDDEN)) {
    test(`${file} keeps >= ${floor} aria-hidden decorative markers`, () => {
      const count = (read(file).match(/aria-hidden/g) ?? []).length;
      expect(count).toBeGreaterThanOrEqual(floor);
    });
  }

  test("the touched files keep >= 48 aria-hidden decorative markers in total", () => {
    const total = Object.keys(ADDED_ARIA_HIDDEN).reduce(
      (sum, f) => sum + (read(f).match(/aria-hidden/g) ?? []).length,
      0,
    );
    expect(total).toBeGreaterThanOrEqual(48);
  });
});

/** Extract the full `<button …>…</button>` region containing position `at`. */
function buttonRegionAround(src: string, at: number): string {
  const open = src.lastIndexOf("<button", at);
  expect(open).toBeGreaterThan(-1);
  let depth = 0;
  let i = open;
  while (i < src.length) {
    const nextOpen = src.indexOf("<button", i);
    const nextClose = src.indexOf("</button>", i);
    if (nextClose < 0) throw new Error("unbalanced <button> region");
    if (nextOpen >= 0 && nextOpen < nextClose) {
      depth++;
      i = nextOpen + 7;
    } else {
      depth--;
      i = nextClose + 9;
      if (depth === 0) return src.slice(open, i);
    }
  }
  throw new Error("unbalanced <button> region");
}

describe("a11y icons: reverse protection (sampled sites)", () => {
  // Each sampled aria-hidden icon must live in a button whose accessible name
  // comes from something other than the icon itself (text sibling, or an
  // aria-label/title attribute for icon-only buttons). If that naming source
  // disappears later, these tests fail and the aria-hidden must be reviewed —
  // an aria-hidden icon as an unnamed button's only content = nameless button.

  test("RemoteBranchList merge button: title attr + text sibling keep the name source", () => {
    const src = read("src/components/branches/RemoteBranchList.vue");
    const button = buttonRegionAround(src, src.indexOf("<GitMerge"));
    expect(button).toContain('aria-hidden="true"');
    expect(button).toContain(`:title="t('branches.remote.merge.hint')"`);
    // non-icon content (the text sibling) must remain in the button
    expect(button.replace(/<GitMerge[^>]*\/>/, "")).toMatch(/\S/);
    expect(button).toContain('t("branches.remote.merge")');
  });

  test("AiReasoningPanel toggle: text sibling keeps the name source", () => {
    const src = read("src/components/ui/AiReasoningPanel.vue");
    const button = buttonRegionAround(src, src.indexOf("<ChevronDown"));
    expect(button).toContain('aria-hidden="true"');
    expect(button).toContain('{{ t("ai.chat.reasoning") }}');
  });

  test("LocalBranchList delete button: icon-only button is named via aria-label", () => {
    // The row delete button's only DOM content is the Trash2 icon; it is safe
    // only because the button itself carries an explicit aria-label.
    const src = read("src/components/branches/LocalBranchList.vue");
    const labelAt = src.indexOf(`:aria-label="t('branches.local.delete.hint')"`);
    expect(labelAt).toBeGreaterThan(-1);
    const button = buttonRegionAround(src, labelAt);
    expect(button).toContain("<Trash2");
    expect(button).toContain('aria-hidden="true"');
    expect(button.replace(/<Trash2[^>]*\/>/, "").replace(/<button[^>]*>/, "").replace("</button>", "").trim())
      .toBe(""); // documents it IS an icon-only button — guarded by the aria-label above
  });
});
