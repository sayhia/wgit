import { t } from "../../i18n";

/** Presentation metadata for PR states, file statuses and check results. */

export function prStateLabel(state: string, isDraft = false): string {
  if (isDraft && state === "open") return t("pr.draft");
  switch (state) {
    case "merged":
      return t("pr.state.merged");
    case "closed":
      return t("pr.state.closed");
    default:
      return t("pr.state.open");
  }
}

export function prStateChip(state: string, isDraft = false): string {
  if (isDraft && state === "open") return "bg-surface border border-line text-ink-muted";
  switch (state) {
    case "merged":
      return "bg-purple/12 text-purple border border-purple/25";
    case "closed":
      return "bg-danger/12 text-danger border border-danger/25";
    default:
      return "bg-success/12 text-success border border-success/25";
  }
}

/** Dot colour for CI state; empty state renders as neutral grey. */
export function checksDot(state: string): string {
  switch (state) {
    case "success":
      return "bg-success-strong";
    case "failure":
      return "bg-danger-strong";
    case "pending":
      return "bg-warning-strong";
    default:
      return "bg-ink-muted/40";
  }
}

export function checksLabel(state: string): string {
  switch (state) {
    case "success":
      return "CI ✓";
    case "failure":
      return "CI ✕";
    case "pending":
      return "CI …";
    default:
      return "CI –";
  }
}

export function fileStatusLabel(status: string): string {
  switch (status) {
    case "added":
      return "新增";
    case "removed":
      return "删除";
    case "renamed":
      return "重命名";
    case "copied":
      return "复制";
    case "changed":
      return "属性变更";
    default:
      return "修改";
  }
}

export function fileStatusChip(status: string): string {
  switch (status) {
    case "added":
      return "bg-success/12 text-success";
    case "removed":
      return "bg-danger/12 text-danger";
    case "renamed":
      return "bg-warning/12 text-warning";
    default:
      return "bg-accent/12 text-accent";
  }
}

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/** "3 分钟前" style relative time; falls back to the raw string when unparsable. */
export function formatRelativeTime(iso: string, now: number = Date.now()): string {
  if (!iso) return "";
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return iso;
  const delta = Math.max(0, now - then);
  if (delta < MINUTE) return t("pr.time.just.now");
  if (delta < HOUR) return t("pr.time.minutes", { count: Math.floor(delta / MINUTE) });
  if (delta < DAY) return t("pr.time.hours", { count: Math.floor(delta / HOUR) });
  if (delta < 30 * DAY) return t("pr.time.days", { count: Math.floor(delta / DAY) });
  return iso.slice(0, 10);
}
