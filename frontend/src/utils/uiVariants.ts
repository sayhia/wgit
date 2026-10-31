/**
 * UI primitive variants.
 *
 * Class strings live here as complete literals (never template-interpolated)
 * so Tailwind's content scanner can see them. Components import the helpers;
 * tests assert the mappings without touching the DOM.
 */

import { t } from "../i18n";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "warning" | "purple";
export type ButtonSize = "xs" | "sm" | "md";
export type BadgeTone = "accent" | "success" | "warning" | "danger" | "muted" | "purple";
export type BadgeSize = "xs" | "sm";
export type StatusKind = "pass" | "warn" | "fail" | "info";

const BUTTON_BASE =
  "ui-button inline-flex items-center justify-center gap-1.5 rounded-lg border font-medium transition-colors duration-150 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed select-none whitespace-nowrap";

const BUTTON_VARIANTS: Record<ButtonVariant, string> = {
  primary: "border-transparent bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast",
  secondary: "bg-surface hover:bg-raised border-line text-ink-secondary hover:text-ink",
  ghost: "border-transparent bg-transparent hover:bg-raised text-ink-muted hover:text-ink",
  danger: "border-transparent bg-danger-strong hover:bg-danger-strong-hover text-white",
  warning: "border-transparent bg-warning-strong hover:bg-warning-strong-hover text-warning-contrast",
  purple: "border-transparent bg-purple-strong hover:bg-purple-strong-hover text-white",
};

const BUTTON_SIZES: Record<ButtonSize, string> = {
  xs: "min-h-6 px-2 py-1 text-[10px]",
  sm: "min-h-7 px-3 py-1.5 text-xs",
  md: "px-4 py-2 text-sm",
};

export function buttonClasses(variant: ButtonVariant = "secondary", size: ButtonSize = "sm"): string {
  return `${BUTTON_BASE} ${BUTTON_VARIANTS[variant]} ${BUTTON_SIZES[size]}`;
}

const BADGE_BASE = "inline-flex items-center gap-1 rounded border font-medium whitespace-nowrap";

const BADGE_TONES: Record<BadgeTone, string> = {
  accent: "border-transparent bg-accent-bg text-accent",
  success: "border-transparent bg-success-bg text-success",
  warning: "border-transparent bg-warning-bg text-warning",
  danger: "border-transparent bg-danger-bg text-danger",
  muted: "bg-canvas border-line-subtle text-ink-secondary",
  purple: "border-transparent bg-purple-bg text-purple",
};

const BADGE_SIZES: Record<BadgeSize, string> = {
  xs: "px-1 py-0.5 text-[9px]",
  sm: "px-1.5 py-0.5 text-[10px]",
};

export function badgeClasses(tone: BadgeTone = "muted", size: BadgeSize = "xs"): string {
  return `${BADGE_BASE} ${BADGE_TONES[tone]} ${BADGE_SIZES[size]}`;
}

const STATUS_TONES: Record<StatusKind, BadgeTone> = {
  pass: "success",
  warn: "warning",
  fail: "danger",
  info: "accent",
};

/** i18n keys — resolved in `statusLabel()` so the locale switch applies. */
const STATUS_LABEL_KEYS: Record<StatusKind, string> = {
  pass: "status.pass",
  warn: "status.warn",
  fail: "status.fail",
  info: "status.info",
};

/** Maps a health/diagnostic status onto a badge tone. */
export function statusTone(status: string): BadgeTone {
  const key = normalizeStatus(status);
  return key ? STATUS_TONES[key] : "muted";
}

export function statusLabel(status: string): string {
  const key = normalizeStatus(status);
  return key ? t(STATUS_LABEL_KEYS[key]) : status;
}

/** Returns null for unknown values so callers can pass them through verbatim. */
function normalizeStatus(status: string): StatusKind | null {
  const value = (status || "").toLowerCase();
  if (value === "pass" || value === "ok" || value === "success" || value === "clean") return "pass";
  if (value === "warn" || value === "warning" || value === "pending") return "warn";
  if (value === "fail" || value === "error" || value === "danger") return "fail";
  if (value === "info" || value === "hint") return "info";
  return null;
}

/** Density-independent skeleton shimmer, used while data loads. */
export function skeletonClasses(height: "line" | "block" | "card" = "line"): string {
  const base = "rounded bg-raised animate-pulse";
  switch (height) {
    case "block":
      return `${base} h-24 w-full`;
    case "card":
      return `${base} h-32 w-full rounded-xl`;
    default:
      return `${base} h-3 w-full`;
  }
}
