import { ref } from "vue";

/**
 * Brand accent presets.
 *
 * The palette lives in CSS (`[data-accent="..."]` blocks in style.css) so that
 * a theme switch re-applies the correct light/dark values without any JS. This
 * module only owns the selection and the persistence.
 */
export type AccentId = "blue" | "violet" | "teal" | "green" | "amber" | "rose";

export interface AccentOption {
  id: AccentId;
  /** i18n key — resolve with `t()` at the call site so locale switches apply. */
  labelKey: string;
  /** Representative solid colour, used for the swatch in settings. */
  swatch: string;
  /** i18n key — resolve with `t()`. */
  hintKey: string;
}

export const ACCENT_OPTIONS: AccentOption[] = [
  { id: "blue", labelKey: "settings.accent.blue", swatch: "#3d63dd", hintKey: "settings.accent.blue.hint" },
  { id: "violet", labelKey: "settings.accent.violet", swatch: "#7048e8", hintKey: "settings.accent.violet.hint" },
  { id: "teal", labelKey: "settings.accent.teal", swatch: "#0d9488", hintKey: "settings.accent.teal.hint" },
  { id: "green", labelKey: "settings.accent.green", swatch: "#2f9e44", hintKey: "settings.accent.green.hint" },
  { id: "amber", labelKey: "settings.accent.amber", swatch: "#e8890c", hintKey: "settings.accent.amber.hint" },
  { id: "rose", labelKey: "settings.accent.rose", swatch: "#e0316b", hintKey: "settings.accent.rose.hint" },
];

const STORAGE_KEY = "wgit-accent";
const accent = ref<AccentId>("blue");
let initialized = false;

function apply() {
  const root = document.documentElement;
  if (accent.value === "blue") {
    // Blue is the design-token default; leaving the attribute off keeps
    // tokens.css as the single source of truth.
    delete root.dataset.accent;
  } else {
    root.dataset.accent = accent.value;
  }
}

export function initAccent() {
  if (initialized) return;
  initialized = true;
  const saved = localStorage.getItem(STORAGE_KEY) as AccentId | null;
  if (saved && ACCENT_OPTIONS.some((o) => o.id === saved)) {
    accent.value = saved;
  }
  apply();
}

export function useAccent() {
  initAccent();

  function setAccent(next: AccentId) {
    if (accent.value === next) return;
    accent.value = next;
    localStorage.setItem(STORAGE_KEY, next);
    apply();
  }

  return { accent, setAccent, options: ACCENT_OPTIONS };
}
