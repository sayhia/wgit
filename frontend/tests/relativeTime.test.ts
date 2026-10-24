import { describe, expect, test } from "bun:test";
import { relativeTime } from "../src/utils/relativeTime";
import { locale, setWarnMissing } from "../src/i18n";

setWarnMissing(false);

const NOW = Date.parse("2026-10-24T12:00:00Z");
const ago = (seconds: number) => new Date(NOW - seconds * 1000).toISOString();

function inLocale<T>(next: "zh-CN" | "en-US", fn: () => T): T {
  const previous = locale.value;
  locale.value = next;
  try {
    return fn();
  } finally {
    locale.value = previous;
  }
}

describe("relativeTime", () => {
  test("formats the past in English", () => {
    inLocale("en-US", () => {
      expect(relativeTime(ago(30), "", NOW)).toBe("30 seconds ago");
      expect(relativeTime(ago(60 * 5), "", NOW)).toBe("5 minutes ago");
      expect(relativeTime(ago(3600 * 3), "", NOW)).toBe("3 hours ago");
      expect(relativeTime(ago(86400), "", NOW)).toBe("yesterday");
      expect(relativeTime(ago(86400 * 3), "", NOW)).toBe("3 days ago");
      expect(relativeTime(ago(86400 * 400), "", NOW)).toBe("last year");
    });
  });

  test("formats the past in Chinese", () => {
    inLocale("zh-CN", () => {
      expect(relativeTime(ago(3600), "", NOW)).toContain("小时前");
      // numeric:"auto" renders -2 days as 前天, -3 days as 3天前.
      expect(relativeTime(ago(86400 * 2), "", NOW)).toBe("前天");
      expect(relativeTime(ago(86400 * 3), "", NOW)).toContain("天前");
      expect(relativeTime(ago(86400 * 400), "", NOW)).toContain("年");
    });
  });

  test("formats the future with the opposite wording", () => {
    inLocale("en-US", () => {
      expect(relativeTime(new Date(NOW + 3600_000).toISOString(), "", NOW)).toBe("in 1 hour");
    });
  });

  test("falls back to the backend label when the timestamp is unusable", () => {
    expect(relativeTime("", "刚刚", NOW)).toBe("刚刚");
    expect(relativeTime(null, "刚刚", NOW)).toBe("刚刚");
    expect(relativeTime("not-a-date", "刚刚", NOW)).toBe("刚刚");
    // No fallback: echo the raw value rather than showing nothing.
    expect(relativeTime("not-a-date", "", NOW)).toBe("not-a-date");
  });

  test("a missing timestamp with no fallback renders empty", () => {
    expect(relativeTime(undefined, "", NOW)).toBe("");
  });
});
