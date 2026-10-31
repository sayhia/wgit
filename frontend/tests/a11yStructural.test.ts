/**
 * U5 structural a11y round: fix-point locks + behavior tests.
 *
 * Covers four approved findings plus the in-scope tab strip:
 *  1. ShortcutCheatSheetModal never wired UiModal's update:open, so the
 *     built-in close button and Escape were dead — both paths must now
 *     forward to the component's close event.
 *  2. CommitInspector Cherry-Pick / Revert button labels were hardcoded
 *     English; they now resolve i18n keys (new keys, zh/en aligned).
 *  3. MergeBranchModal merge-mode buttons expose toggle-button group
 *     semantics (named group + aria-pressed) instead of raw buttons.
 *  4. SetUpstreamModal remote-branch picker group is labelled via
 *     aria-labelledby instead of a dangling <label>.
 *  5. ToolsView primary tab strip exposes tablist/tab + aria-selected.
 */
import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { readFileSync } from "fs";
import { resolve } from "path";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import "./domSetup";
import { locale, setWarnMissing, t, type Locale } from "../src/i18n";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import ShortcutCheatSheetModal from "../src/components/ShortcutCheatSheetModal.vue";
import MergeBranchModal from "../src/components/branches/modals/MergeBranchModal.vue";
import SetUpstreamModal from "../src/components/branches/modals/SetUpstreamModal.vue";

const read = (rel: string) => readFileSync(resolve(__dirname, "..", rel), "utf-8");

let wrapper: VueWrapper | null = null;
// bun runs every test file in one process: locale is global state, so it has
// to be restored or every later file that expects the zh-CN default breaks.
let previousLocale: Locale = "zh-CN";

beforeEach(() => {
  setWarnMissing(false);
  previousLocale = locale.value;
  locale.value = "en-US";
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  locale.value = previousLocale;
});

// ---------------------------------------------------------------------------
// 1. ShortcutCheatSheetModal: built-in close button and Escape
// ---------------------------------------------------------------------------

describe("ShortcutCheatSheetModal close wiring (U5 #1)", () => {
  function mountOpen() {
    wrapper = mount(ShortcutCheatSheetModal, {
      props: { open: true },
      attachTo: document.body,
    });
    return flushPromises();
  }

  test("Escape closes the modal (UiModal's update:open is forwarded as close)", async () => {
    await mountOpen();
    expect(wrapper!.find('[role="dialog"]').exists()).toBe(true);

    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    await flushPromises();

    expect(wrapper!.emitted("close")).toBeTruthy();
  });

  test("the built-in header close button closes the modal", async () => {
    await mountOpen();
    const dialog = wrapper!.find('[role="dialog"]');
    const closeLabel = t("common.close");
    // UiModal's own header button is the first close-labelled button inside
    // the dialog (the body renders a second, working one of its own).
    const builtIn = dialog
      .findAll("button")
      .find((b) => b.attributes("aria-label") === closeLabel);
    expect(builtIn).toBeTruthy();

    await builtIn!.trigger("click");
    await flushPromises();

    expect(wrapper!.emitted("close")).toBeTruthy();
  });

  test("fix-point: the dead @close listener stays gone, update:open is wired", () => {
    const src = read("src/components/ShortcutCheatSheetModal.vue");
    expect(src).toContain("@update:open=");
    expect(src).not.toContain("@close=");
  });
});

// ---------------------------------------------------------------------------
// 2. CommitInspector: Cherry-Pick / Revert button labels
// ---------------------------------------------------------------------------

describe("CommitInspector Cherry-Pick/Revert labels (U5 #2)", () => {
  test("new keys exist in both locales (no reusable main-label key existed)", () => {
    expect(enUS["history.inspector.cherry.pick"]).toBe("Cherry-Pick");
    expect(enUS["history.inspector.revert"]).toBe("Revert");
    expect(zhCN["history.inspector.cherry.pick"]).toBe("Cherry-Pick");
    expect(zhCN["history.inspector.revert"]).toBe("Revert");
    // the hint keys the buttons already used stay intact
    expect(enUS["history.inspector.cherry.pick.hint"]).toBeTruthy();
    expect(enUS["history.inspector.revert.hint"]).toBeTruthy();
    expect(zhCN["history.inspector.cherry.pick.hint"]).toBeTruthy();
    expect(zhCN["history.inspector.revert.hint"]).toBeTruthy();
  });

  test("new keys sit at the same aligned position in both locale files", () => {
    const en = read("src/i18n/messages/en-US.ts");
    const zh = read("src/i18n/messages/zh-CN.ts");
    expect(en).toContain(
      '"history.inspector.cherry.pick": "Cherry-Pick",\n  "history.inspector.cherry.pick.hint"'
    );
    expect(zh).toContain(
      '"history.inspector.cherry.pick": "Cherry-Pick",\n  "history.inspector.cherry.pick.hint"'
    );
    expect(en).toContain(
      '"history.inspector.revert": "Revert",\n  "history.inspector.revert.hint"'
    );
    expect(zh).toContain(
      '"history.inspector.revert": "Revert",\n  "history.inspector.revert.hint"'
    );
  });

  test("button labels come from i18n, no hardcoded English remains", () => {
    const src = read("src/components/history/CommitInspector.vue");
    expect(src).toContain('t("history.inspector.cherry.pick")');
    expect(src).toContain('t("history.inspector.revert")');
    expect(src).not.toContain(">Cherry-Pick</span>");
    expect(src).not.toContain(">Revert</span>");
  });
});

// ---------------------------------------------------------------------------
// 3. MergeBranchModal: merge-mode toggle-button group
// ---------------------------------------------------------------------------

describe("MergeBranchModal mode group semantics (U5 #3)", () => {
  const baseProps = {
    open: true,
    source: "feature/x",
    currentBranch: "main",
    preview: null,
    loadingPreview: false,
    overlap: null,
    loadingOverlap: false,
    mode: "no-ff" as const,
    strategy: "" as const,
    autostash: false,
    message: "",
    merging: false,
  };

  test("mode buttons form a named group with aria-pressed state", async () => {
    wrapper = mount(MergeBranchModal, { props: baseProps });
    await flushPromises();

    const group = wrapper!.find('[role="group"]');
    expect(group.exists()).toBe(true);
    expect(group.attributes("aria-label")).toBe(t("branches.merge.modal.mode"));

    const pressed = wrapper!.findAll('button[aria-pressed]');
    expect(pressed.length).toBe(4);
    // exactly the current mode reports pressed
    const on = pressed.filter((b) => b.attributes("aria-pressed") === "true");
    expect(on.length).toBe(1);

    await pressed[1].trigger("click"); // squash
    expect(wrapper!.emitted("update:mode")?.[0]).toEqual(["squash"]);
  });

  test("fix-point: aria-pressed follows the mode switch", async () => {
    wrapper = mount(MergeBranchModal, { props: { ...baseProps, mode: "squash" as const } });
    await flushPromises();

    const pressed = wrapper!.findAll('button[aria-pressed]');
    expect(pressed[0].attributes("aria-pressed")).toBe("false"); // no-ff
    expect(pressed[1].attributes("aria-pressed")).toBe("true"); // squash
  });
});

// ---------------------------------------------------------------------------
// 4. SetUpstreamModal: remote-branch picker group label
// ---------------------------------------------------------------------------

describe("SetUpstreamModal group labelling (U5 #4)", () => {
  test("picker group is labelled via aria-labelledby pointing at the label", () => {
    wrapper = mount(SetUpstreamModal, {
      props: {
        isOpen: true,
        branch: null,
        availableRemoteBranches: ["origin/main", "origin/dev"],
      },
    });

    const group = wrapper!.find(
      '[role="group"][aria-labelledby="wgit-upstream-select-label"]'
    );
    expect(group.exists()).toBe(true);
    const label = group.find("#wgit-upstream-select-label");
    expect(label.exists()).toBe(true);
    expect(label.text()).toBe(t("branches.upstream.modal.select.label"));
  });

  test("fix-point: the picker label carries its id in the source", () => {
    const src = read("src/components/branches/modals/SetUpstreamModal.vue");
    expect(src).toContain('aria-labelledby="wgit-upstream-select-label"');
    expect(src).toContain('id="wgit-upstream-select-label"');
  });
});

// ---------------------------------------------------------------------------
// 5. ToolsView: primary tab strip tablist semantics
// ---------------------------------------------------------------------------

describe("ToolsView tab strip semantics (U5 #5)", () => {
  test("the pill strip is a labelled tablist and tabs carry selection state", () => {
    const src = read("src/components/ToolsView.vue");
    expect(src).toContain('role="tablist"');
    expect(src).toContain(":aria-label=\"t('nav.tools')\"");
    expect(src).toContain('role="tab"');
    expect(src).toContain(':aria-selected="activeSubTab === def.id"');
    expect(src).toContain(':aria-selected="activeIsMore"');
  });
});
