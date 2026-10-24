/**
 * Locale-aware relative timestamps.
 *
 * Git's `%cr` placeholder is formatted by *git*, using the process locale — so a
 * Chinese system keeps returning "1 小时前" even after the UI is switched to
 * English. The backend also returns an ISO `date` for every item, so the UI
 * recomputes the relative label itself through `Intl.RelativeTimeFormat` and
 * only falls back to the backend string when the timestamp is unparseable.
 */

import { locale } from "../i18n";

const SECOND = 1;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;
const MONTH = 30 * DAY;
const YEAR = 365 * DAY;

/**
 * Formats `iso` as "3 hours ago" / "3 小时前".
 *
 * @param iso      ISO timestamp (or anything `Date.parse` accepts).
 * @param fallback Backend-provided label, used when `iso` cannot be parsed.
 * @param now      Reference time in ms — injectable so tests stay deterministic.
 */
export function relativeTime(
  iso: string | null | undefined,
  fallback = "",
  now: number = Date.now()
): string {
  if (!iso) return fallback;
  const parsed = Date.parse(iso);
  if (Number.isNaN(parsed)) return fallback || iso;

  const delta = (parsed - now) / 1000; // negative = in the past
  const abs = Math.abs(delta);
  const rtf = new Intl.RelativeTimeFormat(locale.value, { numeric: "auto" });

  if (abs < MINUTE) return rtf.format(Math.round(delta / SECOND), "second");
  if (abs < HOUR) return rtf.format(Math.round(delta / MINUTE), "minute");
  if (abs < DAY) return rtf.format(Math.round(delta / HOUR), "hour");
  if (abs < WEEK) return rtf.format(Math.round(delta / DAY), "day");
  if (abs < MONTH) return rtf.format(Math.round(delta / WEEK), "week");
  if (abs < YEAR) return rtf.format(Math.round(delta / MONTH), "month");
  return rtf.format(Math.round(delta / YEAR), "year");
}
