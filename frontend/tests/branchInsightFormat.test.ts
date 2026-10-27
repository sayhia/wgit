import { describe, expect, test } from "bun:test";
import { formatDaysAgo, formatInsightDay } from "../src/utils/branchInsightFormat";
import { locale, setWarnMissing } from "../src/i18n";
import type { BranchInsight } from "../src/types";

setWarnMissing(false);

/** Minimal fully-typed insight row; only `lastDate` matters to the formatter. */
function insight(lastDate: string): BranchInsight {
  return {
    name: "main",
    isCurrent: true,
    isRemote: false,
    shortHash: "abc1234",
    lastAuthor: "Ada",
    lastDate,
    lastSubject: "Initial commit",
    ahead: 0,
    behind: 0,
    hasUpstream: false,
    upstream: "",
    merged: false,
    protected: false,
    daysSinceCommit: 0,
    category: "active",
    suggestion: "",
  };
}

/** Run `fn` under `next` and always restore the previous locale. */
function inLocale<T>(next: "zh-CN" | "en-US", fn: () => T): T {
  const previous = locale.value;
  locale.value = next;
  try {
    return fn();
  } finally {
    locale.value = previous;
  }
}

describe("formatInsightDay", () => {
  test("returns an empty string when the insight is missing", () => {
    expect(formatInsightDay(undefined)).toBe("");
  });

  test("returns an empty string when lastDate is missing", () => {
    expect(formatInsightDay({} as BranchInsight)).toBe("");
  });

  test("returns an empty string for an empty lastDate", () => {
    expect(formatInsightDay(insight(""))).toBe("");
  });

  test("slices a Go timestamp to YYYY-MM-DD", () => {
    expect(formatInsightDay(insight("2026-01-02 10:00:00 +0800"))).toBe("2026-01-02");
    expect(formatInsightDay(insight("2026-10-24T12:00:00Z"))).toBe("2026-10-24");
  });

  test("keeps an already-compact day unchanged", () => {
    expect(formatInsightDay(insight("2026-01-02"))).toBe("2026-01-02");
  });

  test("keeps calendar-valid boundary days unchanged", () => {
    expect(formatInsightDay(insight("2026-12-31"))).toBe("2026-12-31");
    expect(formatInsightDay(insight("2024-02-29"))).toBe("2024-02-29");
  });

  test("falls back to the raw value when the prefix is not a day shape", () => {
    expect(formatInsightDay(insight("not-a-date"))).toBe("not-a-date");
    expect(formatInsightDay(insight("2026/01/02"))).toBe("2026/01/02");
    expect(formatInsightDay(insight("2026-1-2"))).toBe("2026-1-2");
    expect(formatInsightDay(insight("unknown"))).toBe("unknown");
  });

  test("rejects impossible calendar dates that match the day shape", () => {
    // A well-shaped but impossible day used to pass through untouched; the
    // round-trip check now falls back to the raw value, like a shape mismatch.
    expect(formatInsightDay(insight("2026-13-40"))).toBe("2026-13-40");
    expect(formatInsightDay(insight("2026-13-40 00:00:00"))).toBe("2026-13-40 00:00:00");
    expect(formatInsightDay(insight("2026-02-30"))).toBe("2026-02-30");
    // 2026 is not a leap year, so Feb 29 rolled over to Mar 1 before.
    expect(formatInsightDay(insight("2026-02-29"))).toBe("2026-02-29");
    expect(formatInsightDay(insight("2026-00-10"))).toBe("2026-00-10");
  });
});

describe("formatDaysAgo", () => {
  test("returns an empty string for unknown values", () => {
    expect(formatDaysAgo(undefined)).toBe("");
    expect(formatDaysAgo(null)).toBe("");
    expect(formatDaysAgo(Number.NaN)).toBe("");
  });

  test("formats ages in the source locale (zh-CN)", () => {
    inLocale("zh-CN", () => {
      expect(formatDaysAgo(0)).toBe("今天");
      expect(formatDaysAgo(-3)).toBe("今天");
      expect(formatDaysAgo(1)).toBe("1 天前");
      expect(formatDaysAgo(5)).toBe("5 天前");
      expect(formatDaysAgo(29)).toBe("29 天前");
      expect(formatDaysAgo(30)).toBe("1 个月前");
      expect(formatDaysAgo(364)).toBe("12 个月前");
      expect(formatDaysAgo(365)).toBe("1 年前");
      expect(formatDaysAgo(3650)).toBe("10 年前");
    });
  });

  test("formats ages in English with the same thresholds", () => {
    inLocale("en-US", () => {
      expect(formatDaysAgo(0)).toBe("Today");
      expect(formatDaysAgo(1)).toBe("1 day ago");
      expect(formatDaysAgo(5)).toBe("5 days ago");
      expect(formatDaysAgo(29)).toBe("29 days ago");
      // A count of exactly 1 uses the singular catalog key, so the old
      // "1 months ago" / "1 years ago" output is gone.
      expect(formatDaysAgo(30)).toBe("1 month ago");
      // 31 and 366 still bucket to a count of 1 (floor), so they stay singular.
      expect(formatDaysAgo(31)).toBe("1 month ago");
      expect(formatDaysAgo(364)).toBe("12 months ago");
      expect(formatDaysAgo(365)).toBe("1 year ago");
      expect(formatDaysAgo(366)).toBe("1 year ago");
      expect(formatDaysAgo(3650)).toBe("10 years ago");
    });
  });

  test("keeps the plural forms once the count reaches 2", () => {
    inLocale("en-US", () => {
      expect(formatDaysAgo(60)).toBe("2 months ago");
      expect(formatDaysAgo(730)).toBe("2 years ago");
    });
    inLocale("zh-CN", () => {
      expect(formatDaysAgo(60)).toBe("2 个月前");
      expect(formatDaysAgo(730)).toBe("2 年前");
    });
  });

  test("restores the previous locale after formatting", () => {
    const previous = locale.value;
    inLocale("en-US", () => {
      expect(formatDaysAgo(1)).toBe("1 day ago");
    });
    expect(locale.value).toBe(previous);
  });
});
