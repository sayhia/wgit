import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const SRC = join(import.meta.dir, "..", "src");
const read = (relative: string) => readFileSync(join(SRC, relative), "utf8");

/**
 * Hand-rolled `role="dialog"` shells duplicate the focus trap, Esc handling
 * and overlay styling that UiModal already owns. New overlays must build on
 * the primitive. The search palettes, full-page cards and split editors have
 * all migrated; this file keeps them from regressing.
 */
describe("modal convergence contract", () => {
  test("migrated modals delegate the dialog shell to UiModal", () => {
    for (const file of [
      "components/CrashReportModal.vue",
      "components/lfs/LockNotificationModal.vue",
      "components/CommandPalette.vue",
      "components/QuickOpen.vue",
      "components/RepoOnboarding.vue",
      "components/RepoDashboard.vue",
      "components/RebaseModal.vue",
      "components/SyncModal.vue",
      "components/assets/AssetDiffModal.vue",
    ]) {
      const source = read(file);
      expect(source).not.toContain('role="dialog"');
      expect(source).not.toContain("useModalA11y");
      expect(source).toContain("<UiModal");
    }
  });

  test("search palettes keep their top placement through the bare variant", () => {
    for (const file of ["components/CommandPalette.vue", "components/QuickOpen.vue"]) {
      const source = read(file);
      expect(source).toContain('variant="bare"');
      expect(source).toContain('align="top"');
      expect(source).toContain("close-on-backdrop");
    }
  });

  test("full-page cards keep the canvas overlay without the glass material", () => {
    const onboarding = read("components/RepoOnboarding.vue");
    expect(onboarding).toContain('overlay="canvas-strong"');
    expect(onboarding).toContain('surface="plain"');
    expect(onboarding).toContain('variant="bare"');
    const dashboard = read("components/RepoDashboard.vue");
    expect(dashboard).toContain('overlay="canvas"');
    expect(dashboard).toContain('variant="bare"');
  });

  test("split editors keep their own headers inside the bare panel", () => {
    for (const file of [
      "components/RebaseModal.vue",
      "components/SyncModal.vue",
      "components/assets/AssetDiffModal.vue",
    ]) {
      const source = read(file);
      expect(source).toContain('variant="bare"');
      expect(source).toContain('panel-class="max-h-');
    }
  });

  test("UiModal owns the focus trap and stays the single a11y entry", () => {
    const ui = read("components/ui/UiModal.vue");
    expect(ui).toContain("useModalA11y");
    // bare variant: overlay + panel only, body owns the layout
    expect(ui).toContain('variant?: "default" | "bare"');
    expect(ui).toContain("closeOnBackdrop");
    expect(ui).toContain('align?: "center" | "top"');
    expect(ui).toContain("items-start px-4 pb-4 pt-24");
    expect(ui).toContain('overlay?: "dim" | "canvas" | "canvas-strong"');
    expect(ui).toContain('surface?: "glass" | "plain"');
  });
});
