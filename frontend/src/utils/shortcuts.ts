/**
 * Central keyboard-shortcut registry.
 *
 * Only actions flagged `customizable` may be rebound through Settings; the
 * remaining entries are documented defaults (handled by their owning views).
 * Bindings are stored as strings such as "Meta+Shift+O" / "Alt+S".
 */

import { t } from "../i18n";

export interface ShortcutAction {
  id: string;
  /** i18n key — resolve through `shortcutLabel()` so language switches apply. */
  labelKey: string;
  /** i18n key — resolve through `shortcutDescription()`. */
  descriptionKey: string;
  defaultBinding: string;
  customizable: boolean;
  /** app = handled directly by App.vue; component = handled by the owning view. */
  owner: "app" | "component";
  /** For component-owned actions: window event dispatched when a custom binding fires. */
  eventName?: string;
  eventDetail?: unknown;
}

export interface ParsedBinding {
  key: string;
  meta: boolean;
  shift: boolean;
  alt: boolean;
}

export type ShortcutOverrides = Record<string, string>;

const STORAGE_KEY = "wgit.shortcuts.v1";

const MODIFIER_KEYS = new Set(["Meta", "Control", "Alt", "Shift"]);

export const SHORTCUT_ACTIONS: ShortcutAction[] = [
  {
    id: "command-palette",
    labelKey: "shortcuts.action.command-palette.label",
    descriptionKey: "shortcuts.action.command-palette.description",
    defaultBinding: "Meta+K",
    customizable: true,
    owner: "app",
  },
  {
    id: "open-repo",
    labelKey: "shortcuts.action.open-repo.label",
    descriptionKey: "shortcuts.action.open-repo.description",
    defaultBinding: "Meta+O",
    customizable: true,
    owner: "app",
  },
  {
    id: "repo-dashboard",
    labelKey: "shortcuts.action.repo-dashboard.label",
    descriptionKey: "shortcuts.action.repo-dashboard.description",
    defaultBinding: "Meta+Shift+O",
    customizable: true,
    owner: "app",
  },
  {
    id: "interactive-rebase",
    labelKey: "shortcuts.action.interactive-rebase.label",
    descriptionKey: "shortcuts.action.interactive-rebase.description",
    defaultBinding: "Meta+R",
    customizable: true,
    owner: "app",
  },
  {
    id: "quick-open-files",
    labelKey: "shortcuts.action.quick-open-files.label",
    descriptionKey: "shortcuts.action.quick-open-files.description",
    defaultBinding: "Meta+P",
    customizable: true,
    owner: "component",
    eventName: "wgit:quick-open",
    eventDetail: { mode: "files" },
  },
  {
    id: "code-search",
    labelKey: "shortcuts.action.code-search.label",
    descriptionKey: "shortcuts.action.code-search.description",
    defaultBinding: "Meta+Shift+F",
    customizable: true,
    owner: "component",
    eventName: "wgit:quick-open",
    eventDetail: { mode: "code" },
  },
  {
    id: "toggle-diff-split",
    labelKey: "shortcuts.action.toggle-diff-split.label",
    descriptionKey: "shortcuts.action.toggle-diff-split.description",
    defaultBinding: "Alt+S",
    customizable: true,
    owner: "component",
    eventName: "wgit:workspace-toggle-diff-split",
  },
  {
    id: "expand-all-tree",
    labelKey: "shortcuts.action.expand-all-tree.label",
    descriptionKey: "shortcuts.action.expand-all-tree.description",
    defaultBinding: "Meta+Shift+E",
    customizable: true,
    owner: "component",
    eventName: "wgit:source-toggle-expand-all",
  },
  {
    id: "commit",
    labelKey: "shortcuts.action.commit.label",
    descriptionKey: "shortcuts.action.commit.description",
    defaultBinding: "Meta+Enter",
    customizable: true,
    owner: "component",
    eventName: "wgit:workspace-commit",
  },
];

export function loadShortcutOverrides(): ShortcutOverrides {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    const parsed = raw ? JSON.parse(raw) : {};
    const out: ShortcutOverrides = {};
    for (const action of SHORTCUT_ACTIONS) {
      if (!action.customizable) continue;
      const value = parsed?.[action.id];
      if (typeof value === "string" && parseBinding(value)) out[action.id] = value;
    }
    return out;
  } catch {
    return {};
  }
}

export function saveShortcutOverrides(overrides: ShortcutOverrides): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(overrides));
  } catch {
    /* ignore quota / privacy-mode errors */
  }
}

export function effectiveBinding(action: ShortcutAction, overrides: ShortcutOverrides): string {
  return overrides[action.id] || action.defaultBinding;
}

export function isActionCustomized(actionId: string, overrides: ShortcutOverrides = loadShortcutOverrides()): boolean {
  return !!overrides[actionId];
}

export function parseBinding(binding: string): ParsedBinding | null {
  const parts = binding.split("+").filter(Boolean);
  if (parts.length === 0) return null;
  const parsed: ParsedBinding = { key: "", meta: false, shift: false, alt: false };
  for (const part of parts) {
    if (part === "Meta" || part === "Ctrl" || part === "Control") parsed.meta = true;
    else if (part === "Shift") parsed.shift = true;
    else if (part === "Alt") parsed.alt = true;
    else parsed.key = part;
  }
  if (!parsed.key) return null;
  return parsed;
}

export function bindingToString(binding: ParsedBinding): string {
  const parts: string[] = [];
  if (binding.alt) parts.push("Alt");
  if (binding.meta) parts.push("Meta");
  if (binding.shift) parts.push("Shift");
  parts.push(binding.key.length === 1 ? binding.key.toUpperCase() : binding.key);
  return parts.join("+");
}

/** Normalize a keydown event into a binding; returns null for bare modifiers. */
export function eventToBinding(event: KeyboardEvent): ParsedBinding | null {
  if (MODIFIER_KEYS.has(event.key)) return null;
  let key = event.key;
  if (key === " " || key === "Spacebar") key = "Space";
  else if (key === "Escape") return null;
  else if (key.length === 1) key = key.toUpperCase();
  return {
    key,
    meta: event.metaKey || event.ctrlKey,
    shift: event.shiftKey,
    alt: event.altKey,
  };
}

function keyMatch(eventKey: string, bindingKey: string): boolean {
  if (eventKey === bindingKey) return true;
  if (eventKey === "Spacebar" && bindingKey === "Space") return true;
  if (eventKey === "Enter" && bindingKey === "Enter") return true;
  return eventKey.toLowerCase() === bindingKey.toLowerCase();
}

export function matchesBinding(event: KeyboardEvent, binding: string): boolean {
  const parsed = parseBinding(binding);
  if (!parsed) return false;
  if (!keyMatch(event.key, parsed.key)) return false;
  const meta = event.metaKey || event.ctrlKey;
  if (meta !== parsed.meta) return false;
  if (event.shiftKey !== parsed.shift) return false;
  if (event.altKey !== parsed.alt) return false;
  return true;
}

/** Human-readable chip text, e.g. "⌘ ⇧ O", "⌥ S". */
export function formatBinding(binding: string): string {
  const parsed = parseBinding(binding);
  if (!parsed) return binding;
  const parts: string[] = [];
  if (parsed.meta) parts.push("⌘");
  if (parsed.shift) parts.push("⇧");
  if (parsed.alt) parts.push("⌥");
  parts.push(parsed.key === "Enter" ? "↵" : parsed.key);
  return parts.join(" ");
}

/** Find another action already bound to the given binding (conflict check). */
export function findShortcutConflict(
  binding: string,
  overrides: ShortcutOverrides,
  ignoreActionId?: string
): ShortcutAction | undefined {
  return SHORTCUT_ACTIONS.find(
    (action) =>
      action.id !== ignoreActionId &&
      effectiveBinding(action, overrides).toUpperCase() === binding.toUpperCase()
  );
}

/** Resolve an action's display label in the active locale. */
export function shortcutLabel(action: ShortcutAction): string {
  return t(action.labelKey);
}

/** Resolve an action's one-line description in the active locale. */
export function shortcutDescription(action: ShortcutAction): string {
  return t(action.descriptionKey);
}
