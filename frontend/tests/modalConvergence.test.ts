import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const SRC = join(import.meta.dir, "..", "src");
const read = (relative: string) => readFileSync(join(SRC, relative), "utf8");

/**
 * Hand-rolled `role="dialog"` shells duplicate the focus trap, Esc handling
 * and overlay styling that UiModal already owns. New overlays must build on
 * the primitive; these two were the first batch migrated (plan-2026-10-29 P1.2)
 * and this file keeps them from regressing.
 */
describe("modal convergence contract", () => {
  test("migrated modals delegate the dialog shell to UiModal", () => {
    for (const file of [
      "components/CrashReportModal.vue",
      "components/lfs/LockNotificationModal.vue",
    ]) {
      const source = read(file);
      expect(source).not.toContain('role="dialog"');
      expect(source).toContain("<UiModal");
    }
  });

  test("UiModal owns the focus trap and stays the single a11y entry", () => {
    const ui = read("components/ui/UiModal.vue");
    expect(ui).toContain("useModalA11y");
    // bare variant: overlay + panel only, body owns the layout
    expect(ui).toContain('variant?: "default" | "bare"');
    expect(ui).toContain("closeOnBackdrop");
  });
});
