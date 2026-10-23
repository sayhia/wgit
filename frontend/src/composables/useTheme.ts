import { ref } from "vue";

export type ThemeMode = "dark" | "light" | "system";

const STORAGE_KEY = "wgit-theme";
const mode = ref<ThemeMode>("dark");
let initialized = false;

function resolve(modeValue: ThemeMode): "dark" | "light" {
  if (modeValue === "system") {
    return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
  }
  return modeValue;
}

function apply() {
  const resolved = resolve(mode.value);
  const root = document.documentElement;
  root.classList.toggle("light", resolved === "light");
  root.classList.toggle("dark", resolved === "dark");
  root.dataset.theme = resolved;
}

export function initTheme() {
  if (initialized) return;
  initialized = true;
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved === "dark" || saved === "light" || saved === "system") {
    mode.value = saved;
  }
  window
    .matchMedia("(prefers-color-scheme: light)")
    .addEventListener("change", () => {
      if (mode.value === "system") apply();
    });
  apply();
}

export function useTheme() {
  initTheme();

  function setThemeMode(next: ThemeMode) {
    mode.value = next;
    localStorage.setItem(STORAGE_KEY, next);
    apply();
  }

  return { mode, setThemeMode };
}
