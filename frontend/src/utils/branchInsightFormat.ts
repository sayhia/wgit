/**
 * Display helpers shared by the branch insight surfaces.
 *
 * Both were copied inside BranchesView / LocalBranchList and could drift while
 * formatting the same BranchInsight payload. The `branches.local.*` catalog
 * family is kept because the live insight list renders those strings; moving
 * the functions here therefore leaves every visible label unchanged.
 */
import { t } from "../i18n";
import type { BranchInsight } from "../types";

/**
 * True when `day` is a real calendar date in YYYY-MM-DD form.
 *
 * Parsed as UTC and round-tripped through the components: `Date.UTC` silently
 * rolls overflow (month 13, Feb 30) into the next period, so comparing the
 * parsed year/month/day back is what rejects impossible dates. UTC is used on
 * purpose — a local-time parse would shift the day under a non-UTC timezone and
 * reject dates that are actually valid.
 */
function isCalendarDay(day: string): boolean {
  const [year, month, date] = day.split("-").map(Number) as [number, number, number];
  const parsed = new Date(Date.UTC(year, month - 1, date));
  // Date.UTC maps years 0-99 to 1900-1999; undo that before comparing.
  parsed.setUTCFullYear(year);
  return (
    parsed.getUTCFullYear() === year &&
    parsed.getUTCMonth() === month - 1 &&
    parsed.getUTCDate() === date
  );
}

/** ISO/date strings -> YYYY-MM-DD for compact display. */
export function formatInsightDay(ins?: BranchInsight): string {
  if (!ins?.lastDate) return "";
  const day = ins.lastDate.slice(0, 10);
  if (!/^\d{4}-\d{2}-\d{2}$/.test(day)) return ins.lastDate;
  return isCalendarDay(day) ? day : ins.lastDate;
}

/** Human-readable age for `daysSinceCommit`; empty when the value is unknown. */
export function formatDaysAgo(days?: number | null): string {
  if (days === undefined || days === null || Number.isNaN(days)) return "";
  if (days <= 0) return t("branches.local.days.today");
  if (days === 1) return t("branches.local.days.one");
  if (days < 30) return t("branches.local.days.n", { count: days });
  if (days < 365) {
    const months = Math.floor(days / 30);
    // "1 months ago" is ungrammatical; zh-CN is unaffected either way.
    return months === 1
      ? t("branches.local.month.one")
      : t("branches.local.months.n", { count: months });
  }
  const years = Math.floor(days / 365);
  return years === 1
    ? t("branches.local.year.one")
    : t("branches.local.years.n", { count: years });
}
