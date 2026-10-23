import { ref } from "vue";
import zhCN from "./messages/zh-CN";
import enUS from "./messages/en-US";

/**
 * Minimal i18n — no vue-i18n, no build plugin.
 *
 * Design:
 *   · `translate()` is a pure function; the module-level `locale` ref is the
 *     only state, so components re-render automatically when it changes.
 *   · Missing keys fall back to the source language, then to the key itself.
 *     Returning the key (instead of an empty string) makes gaps obvious in the
 *     UI rather than silently blanking labels.
 *   · Catalogs are plain objects, so adding a locale is one file + one entry.
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

const MESSAGES: Record<Locale, MessageDict> = {
  "zh-CN": zhCN as unknown as MessageDict,
  "en-US": enUS as unknown as MessageDict,
};

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
    if (isSupportedLocale(saved)) locale.value = saved;
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
}

function applyHtmlLang(): void {
  try {
    document.documentElement.lang = locale.value;
  } catch {
    /* non-DOM environment (tests) */
  }
}
