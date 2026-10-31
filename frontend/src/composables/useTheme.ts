import { ref } from "vue";

export type ThemeMode = "dark" | "light" | "system";

const STORAGE_KEY = "wgit-theme";
const HIGH_CONTRAST_KEY = "wgit-high-contrast";
const REDUCE_TRANSPARENCY_KEY = "wgit-reduce-transparency";

const mode = ref<ThemeMode>("dark");
const highContrast = ref<boolean>(false);
const reduceTransparency = ref<boolean>(false);
let initialized = false;

function resolve(modeValue: ThemeMode): "dark" | "light" {
  if (modeValue === "system") {
    return typeof window !== "undefined" && window.matchMedia?.("(prefers-color-scheme: light)").matches ? "light" : "dark";
  }
  return modeValue;
}

function apply() {
  if (typeof document === "undefined") return;
  const resolved = resolve(mode.value);
  const root = document.documentElement;
  root.classList.toggle("light", resolved === "light");
  root.classList.toggle("dark", resolved === "dark");
  root.dataset.theme = resolved;
  root.classList.toggle("high-contrast", highContrast.value);
  root.classList.toggle("reduce-transparency", reduceTransparency.value);
}

export function initTheme() {
  if (initialized) return;
  initialized = true;

  if (typeof localStorage !== "undefined") {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === "dark" || saved === "light" || saved === "system") {
      mode.value = saved;
    }

    const savedHighContrast = localStorage.getItem(HIGH_CONTRAST_KEY);
    if (savedHighContrast !== null) {
      highContrast.value = savedHighContrast === "true";
    } else if (typeof window !== "undefined" && window.matchMedia?.("(forced-colors: active)").matches) {
      highContrast.value = true;
    }

    const savedReduceTransparency = localStorage.getItem(REDUCE_TRANSPARENCY_KEY);
    if (savedReduceTransparency !== null) {
      reduceTransparency.value = savedReduceTransparency === "true";
    } else if (typeof window !== "undefined" && window.matchMedia?.("(prefers-reduced-transparency: reduce)").matches) {
      reduceTransparency.value = true;
    }
  }

  if (typeof window !== "undefined" && window.matchMedia) {
    // Resource lifecycle note: intentional app-lifetime singleton. initTheme()
    // is guarded by `initialized`, so exactly one change listener exists for
    // the whole session and it tracks the app process — no removal needed.
    window
      .matchMedia("(prefers-color-scheme: light)")
      .addEventListener("change", () => {
        if (mode.value === "system") apply();
      });
  }
  apply();
}

export function useTheme() {
  initTheme();

  function setThemeMode(next: ThemeMode) {
    mode.value = next;
    if (typeof localStorage !== "undefined") {
      localStorage.setItem(STORAGE_KEY, next);
    }
    apply();
  }

  function setHighContrast(val: boolean) {
    highContrast.value = val;
    if (typeof localStorage !== "undefined") {
      localStorage.setItem(HIGH_CONTRAST_KEY, String(val));
    }
    apply();
  }

  function setReduceTransparency(val: boolean) {
    reduceTransparency.value = val;
    if (typeof localStorage !== "undefined") {
      localStorage.setItem(REDUCE_TRANSPARENCY_KEY, String(val));
    }
    apply();
  }

  return {
    mode,
    setThemeMode,
    highContrast,
    setHighContrast,
    reduceTransparency,
    setReduceTransparency,
  };
}

