import { describe, expect, test } from "bun:test";
import {
  badgeClasses,
  buttonClasses,
  skeletonClasses,
  statusLabel,
  statusTone,
} from "../src/utils/uiVariants";

describe("buttonClasses", () => {
  test("defaults to a secondary small button", () => {
    const cls = buttonClasses();
    expect(cls).toContain("border-line");
    expect(cls).toContain("text-xs");
    expect(cls).toContain("cursor-pointer");
  });

  test("primary and danger use solid fills with white text", () => {
    expect(buttonClasses("primary")).toContain("bg-accent-strong");
    expect(buttonClasses("primary")).toContain("text-white");
    expect(buttonClasses("danger")).toContain("bg-danger-strong");
  });

  test("warning uses the amber solid fill with contrast text", () => {
    expect(buttonClasses("warning")).toContain("bg-warning-strong");
    expect(buttonClasses("warning")).toContain("text-warning-contrast");
  });

  test("purple is reserved for tag/release actions", () => {
    expect(buttonClasses("purple")).toContain("bg-purple-strong");
  });

  test("ghost has no background until hover", () => {
    const cls = buttonClasses("ghost");
    expect(cls).toContain("bg-transparent");
    expect(cls).toContain("hover:bg-raised");
  });

  test("sizes map to distinct padding and text scales", () => {
    expect(buttonClasses("secondary", "xs")).toContain("px-2");
    expect(buttonClasses("secondary", "sm")).toContain("px-3");
    expect(buttonClasses("secondary", "md")).toContain("px-4");
    expect(buttonClasses("secondary", "md")).toContain("text-sm");
  });

  test("every variant keeps the shared base contract", () => {
    for (const variant of ["primary", "secondary", "ghost", "danger", "warning", "purple"] as const) {
      const cls = buttonClasses(variant, "sm");
      expect(cls).toContain("disabled:opacity-40");
      expect(cls).toContain("whitespace-nowrap");
      expect(cls).toContain("rounded-lg");
    }
  });
});

describe("badgeClasses", () => {
  test("tones are visually distinct", () => {
    const tones = ["accent", "success", "warning", "danger", "muted", "purple"] as const;
    const seen = new Set(tones.map((t) => badgeClasses(t)));
    expect(seen.size).toBe(tones.length);
  });

  test("small badges use a larger type scale than xs", () => {
    expect(badgeClasses("accent", "xs")).toContain("text-[9px]");
    expect(badgeClasses("accent", "sm")).toContain("text-[10px]");
  });
});

describe("statusTone / statusLabel", () => {
  test("maps canonical health statuses", () => {
    expect(statusTone("pass")).toBe("success");
    expect(statusTone("warn")).toBe("warning");
    expect(statusTone("fail")).toBe("danger");
  });

  test("accepts common synonyms", () => {
    expect(statusTone("OK")).toBe("success");
    expect(statusTone("Success")).toBe("success");
    expect(statusTone("clean")).toBe("success");
    expect(statusTone("pending")).toBe("warning");
    expect(statusTone("ERROR")).toBe("danger");
  });

  test("falls back to a neutral tone for unknown values without throwing", () => {
    expect(statusTone("weird-value")).toBe("muted");
    expect(statusTone("")).toBe("muted");
  });

  test("labels are localized for the canonical four", () => {
    expect(statusLabel("pass")).toBe("通过");
    expect(statusLabel("warn")).toBe("待处理");
    expect(statusLabel("fail")).toBe("异常");
    expect(statusLabel("info")).toBe("提示");
  });

  test("unknown labels pass through unchanged", () => {
    expect(statusLabel("自定义状态")).toBe("自定义状态");
  });
});

describe("skeletonClasses", () => {
  test("line is short, block is tall, card is rounded-xl", () => {
    expect(skeletonClasses()).toContain("h-3");
    expect(skeletonClasses("block")).toContain("h-24");
    expect(skeletonClasses("card")).toContain("rounded-xl");
  });

  test("all variants animate", () => {
    for (const v of ["line", "block", "card"] as const) {
      expect(skeletonClasses(v)).toContain("animate-pulse");
    }
  });
});
