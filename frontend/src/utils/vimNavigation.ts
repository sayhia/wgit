import { ref } from "vue";

export type VimInputMode = "NORMAL" | "INSERT";

const VIM_STORAGE_KEY = "wgit.editor.vimMode";

export function loadVimModePreference(): boolean {
  try {
    if (typeof localStorage === "undefined") return false;
    return localStorage.getItem(VIM_STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

export function saveVimModePreference(enabled: boolean): void {
  try {
    if (typeof localStorage === "undefined") return;
    localStorage.setItem(VIM_STORAGE_KEY, enabled ? "true" : "false");
  } catch {
    /* ignore storage failure */
  }
}

export const isVimModeEnabled = ref<boolean>(loadVimModePreference());
export const currentVimMode = ref<VimInputMode>("NORMAL");

export function setVimModeEnabled(enabled: boolean) {
  isVimModeEnabled.value = enabled;
  saveVimModePreference(enabled);
  if (!enabled) {
    currentVimMode.value = "NORMAL";
  }
  if (typeof window !== "undefined") {
    window.dispatchEvent(
      new CustomEvent("wgit:vim-mode-changed", { detail: { enabled } })
    );
  }
}

export function toggleVimMode() {
  setVimModeEnabled(!isVimModeEnabled.value);
}

export function setVimInputMode(mode: VimInputMode) {
  currentVimMode.value = mode;
}

export interface VimKeyHandlers {
  onNext?: () => void;
  onPrev?: () => void;
  onToggleStage?: () => void;
  onStage?: () => void;
  onDiscard?: () => void;
  onFocusCommit?: () => void;
  onEnterInsert?: () => void;
  onExitInsert?: () => void;
  onNextTab?: () => void;
  onPrevTab?: () => void;
}

let lastKey = "";
let lastKeyTime = 0;

/**
 * Dispatches Vim navigation actions when Vim mode is active.
 * Returns true if the key event was intercepted and handled.
 */
export function handleVimKeydown(
  e: KeyboardEvent,
  handlers: VimKeyHandlers
): boolean {
  if (!isVimModeEnabled.value) return false;

  const target = e.target as HTMLElement | null;
  const isInput =
    target &&
    (target.tagName === "INPUT" ||
      target.tagName === "TEXTAREA" ||
      Boolean((target as any).isContentEditable));

  if (isInput) {
    if (e.key === "Escape") {
      target?.blur?.();
      currentVimMode.value = "NORMAL";
      handlers.onExitInsert?.();
      e.preventDefault?.();
      return true;
    }
    currentVimMode.value = "INSERT";
    return false;
  }

  // Not in input: we are in NORMAL mode
  if (currentVimMode.value !== "NORMAL") {
    currentVimMode.value = "NORMAL";
  }

  // Avoid interfering with Command / Control / Alt combos
  if (e.metaKey || e.ctrlKey || e.altKey) return false;

  const now = Date.now();
  const key = e.key;

  // Handle 'gt' and 'gT' tab navigation sequence
  if (lastKey === "g" && now - lastKeyTime <= 1000) {
    lastKey = "";
    if (key === "t") {
      e.preventDefault?.();
      handlers.onNextTab?.();
      return true;
    }
    if (key === "T") {
      e.preventDefault?.();
      handlers.onPrevTab?.();
      return true;
    }
  }

  if (key === "g") {
    lastKey = "g";
    lastKeyTime = now;
    return true;
  }
  lastKey = "";

  switch (key) {
    case "j":
    case "ArrowDown":
      e.preventDefault?.();
      handlers.onNext?.();
      return true;

    case "k":
    case "ArrowUp":
      e.preventDefault?.();
      handlers.onPrev?.();
      return true;

    case " ":
    case "x":
      e.preventDefault?.();
      handlers.onToggleStage?.();
      return true;

    case "s":
      e.preventDefault?.();
      handlers.onStage?.();
      return true;

    case "d":
      e.preventDefault?.();
      handlers.onDiscard?.();
      return true;

    case "c":
      e.preventDefault?.();
      currentVimMode.value = "INSERT";
      handlers.onFocusCommit?.();
      return true;

    case "i":
    case "a":
      e.preventDefault?.();
      currentVimMode.value = "INSERT";
      handlers.onEnterInsert?.();
      return true;

    case "Escape":
      e.preventDefault?.();
      currentVimMode.value = "NORMAL";
      handlers.onExitInsert?.();
      return true;

    default:
      return false;
  }
}
