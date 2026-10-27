import { ref } from "vue";
import zhCN from "./messages/zh-CN";

/**
 * Minimal i18n — on-demand dynamic splitting with synchronous test safety.
 *
 * Design:
 *   · `translate()` is a pure function; the module-level `locale` ref is the
 *     only state, so components re-render automatically when it changes.
 *   · Missing keys fall back to the source language, then to the key itself.
 *   · Catalogs are dynamically split: English is loaded on-demand, saving ~280 kB
 *     from the initial bundle.
 *   · In test environments (Bun/Node), catalogs are pre-seeded so synchronous
 *     assertions pass with zero latency.
 */

export type Locale = "zh-CN" | "en-US";
export type MessageDict = Record<string, string>;
export type MessageParams = Record<string, string | number>;

export const LOCALES: { id: Locale; label: string }[] = [
  { id: "zh-CN", label: "简体中文" },
  { id: "en-US", label: "English" },
];

const FALLBACK_LOCALE: Locale = "zh-CN";
const STORAGE_KEY = "wgit.locale";

const DYNAMIC_LOADERS: Record<Locale, () => Promise<{ default: unknown }>> = {
  "zh-CN": () => Promise.resolve({ default: zhCN }),
  "en-US": () => import("./messages/en-US"),
};

const MESSAGES: Record<Locale, MessageDict> = {
  "zh-CN": zhCN as unknown as MessageDict,
  "en-US": {} as MessageDict,
};

// Catalogs are dynamically split: English is loaded on-demand, saving ~280 kB
// In test environments, catalogs are pre-seeded via tests/setup.ts
export function registerLocaleMessages(target: Locale, dict: MessageDict): void {
  MESSAGES[target] = dict;
  catalogVersion.value++;
}

/**
 * Bumped whenever a catalog arrives.
 *
 * `MESSAGES` is a plain object, so assigning into it is invisible to Vue. Every
 * `t()` call reads this ref, which makes the whole tree re-render once the
 * chunk lands. Without it, a component rendered *before* the catalog arrived
 * keeps showing the fallback language until something else happens to re-render
 * it — the commit composer stayed Chinese in an English UI for exactly this
 * reason.
 */
const catalogVersion = ref(0);

export async function loadLocale(target: Locale): Promise<void> {
  if (Object.keys(MESSAGES[target] || {}).length > 0) return;
  const loader = DYNAMIC_LOADERS[target];
  if (loader) {
    const mod = await loader();
    MESSAGES[target] = (mod as { default: MessageDict }).default;
    catalogVersion.value++;
  }
}

export function registerCatalog(target: Locale, dict: MessageDict): void {
  MESSAGES[target] = dict;
  catalogVersion.value++;
}

export const locale = ref<Locale>(FALLBACK_LOCALE);

/** Tests turn this off to keep output clean. */
let warnMissing = true;
export function setWarnMissing(value: boolean) {
  warnMissing = value;
}

/** Replaces `{name}` placeholders; unknown placeholders are left untouched. */
export function interpolate(template: string, params?: MessageParams): string {
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) => {
    const value = params[name];
    return value === undefined ? match : String(value);
  });
}

/**
 * Looks a key up in `localeOverride` (defaults to the active locale), with a
 * per-key fallback to the source language.
 */
export function translate(key: string, params?: MessageParams, localeOverride?: Locale): string {
  // Touch the version so a late-arriving catalog invalidates every rendered
  // string instead of leaving the UI half-translated.
  void catalogVersion.value;
  const active = localeOverride ?? locale.value;
  const activeDict = MESSAGES[active] ?? MESSAGES[FALLBACK_LOCALE];

  let raw = activeDict[key];
  if (raw === undefined) raw = MESSAGES[FALLBACK_LOCALE][key];
  if (raw === undefined) {
    if (warnMissing) console.warn(`[i18n] missing key: ${key}`);
    return key;
  }
  return interpolate(raw, params);
}

/** Shorthand used in templates: `t("common.close")`. */
export function t(key: string, params?: MessageParams): string {
  return translate(key, params);
}

export function isSupportedLocale(value: unknown): value is Locale {
  return typeof value === "string" && Object.prototype.hasOwnProperty.call(MESSAGES, value);
}

export function initLocale(): void {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (isSupportedLocale(saved)) {
      locale.value = saved;
      if (Object.keys(MESSAGES[saved] || {}).length === 0) {
        void loadLocale(saved);
      }
    }
  } catch {
    /* storage disabled — keep the default */
  }
  applyHtmlLang();
}

export function setLocale(next: Locale): void {
  if (!isSupportedLocale(next)) return;
  locale.value = next;
  try {
    localStorage.setItem(STORAGE_KEY, next);
  } catch {
    /* ignore quota errors */
  }
  applyHtmlLang();
  if (Object.keys(MESSAGES[next] || {}).length === 0) {
    void loadLocale(next);
  }
}

function applyHtmlLang(): void {
  try {
    document.documentElement.lang = locale.value;
  } catch {
    /* non-DOM environment (tests) */
  }
}

/**
 * True when `key` exists in the active catalog (or the fallback one).
 *
 * Backend payloads may carry an optional i18n key next to their rendered text.
 * Checking first keeps an unknown key from leaking into the UI as raw
 * `release.check.commits.title`.
 */
export function hasMessage(key: string): boolean {
  if (!key) return false;
  const activeDict = MESSAGES[locale.value] ?? MESSAGES[FALLBACK_LOCALE];
  return activeDict[key] !== undefined || MESSAGES[FALLBACK_LOCALE][key] !== undefined;
}

/**
 * Renders a backend-supplied message: prefers `key` from the catalog, falls back
 * to the plain text the backend already rendered.
 *
 * The backend keeps emitting authoritative text (logs, CLI, other clients), so
 * a missing or stale key degrades to a readable sentence instead of a raw key.
 */
export function backendText(
  key: string | null | undefined,
  fallback: string,
  params?: MessageParams
): string {
  if (key && hasMessage(key)) return t(key, params);
  return fallback;
}
