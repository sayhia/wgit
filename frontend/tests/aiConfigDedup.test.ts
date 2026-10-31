import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

describe("AI settings de-duplication", () => {
  test("quick setup no longer duplicates profile management", () => {
    const src = read("../src/components/settings/AiConnectionForm.vue");
    expect(src).not.toContain("ai-quick-profile-");
    expect(src).not.toContain("listAIProfiles");
    expect(src).not.toContain("activateAIProfile");
  });

  test("the dedicated profiles card is the single profile manager", () => {
    const src = read("../src/components/settings/AiProfilesCard.vue");
    expect(src).toContain('data-testid="ai-profiles-card"');
    expect(src).toContain("ai-profile-rename");
  });

  test("connection leaves retain notices and migration without duplicating a status card", () => {
    const form = read("../src/components/settings/AiConnectionForm.vue");
    const leaf = read("../src/components/settings/AiConnectionSection.vue");
    expect(form).toContain("saved?.notice");
    expect(form).not.toContain('data-testid="localai-backend-status"');
    expect(leaf).toContain('data-testid="localai-legacy-banner"');
    expect(leaf).toContain("<AiConnectionForm");
  });

  test("compact setup uses the same editor as settings", () => {
    const wizard = read("../src/components/settings/AiQuickSetupModal.vue");
    expect(wizard).toContain('import AiConnectionForm from "./AiConnectionForm.vue"');
    expect(wizard).toContain("<AiConnectionForm");
    expect(wizard).toContain("compact");
    expect(wizard).not.toContain("saveAISettings");
  });
});
