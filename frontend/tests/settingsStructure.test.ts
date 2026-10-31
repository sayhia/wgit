import { describe, expect, test } from "bun:test";
import {
  SETTINGS_GROUPS,
  SETTINGS_LEAVES,
  normalizeSettingsLeaf,
} from "../src/utils/settingsStructure";

describe("settings structure", () => {
  test("exposes four non-empty groups covering thirteen unique leaves", () => {
    expect(SETTINGS_GROUPS.map((group) => group.id)).toEqual([
      "general",
      "git",
      "ai",
      "system",
    ]);
    expect(SETTINGS_GROUPS.every((group) => group.leaves.length > 0)).toBe(true);
    expect(new Set(SETTINGS_LEAVES).size).toBe(13);
  });

  test("normalizes the legacy localai leaf and rejects unknown ids", () => {
    expect(normalizeSettingsLeaf("localai")).toBe("ai-connection");
    expect(normalizeSettingsLeaf("ai-behavior")).toBe("ai-behavior");
    expect(normalizeSettingsLeaf("nope")).toBe("appearance");
    expect(normalizeSettingsLeaf(undefined)).toBe("appearance");
  });
});
