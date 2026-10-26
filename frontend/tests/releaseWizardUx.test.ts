import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const releaseWizardFile = fileURLToPath(
  new URL("../src/components/ReleaseWizard.vue", import.meta.url)
);
const createTagModalFile = fileURLToPath(
  new URL("../src/components/branches/modals/CreateTagModal.vue", import.meta.url)
);
const branchesViewFile = fileURLToPath(
  new URL("../src/components/BranchesView.vue", import.meta.url)
);
const typesFile = fileURLToPath(
  new URL("../src/types.ts", import.meta.url)
);

describe("Release Wizard & Tagging UX (Round 8)", () => {
  describe("ReleaseWizard SemVer Quick Preset Chips", () => {
    const src = readFileSync(releaseWizardFile, "utf8");

    test("renders Patch, Minor, and Major quick version chips", () => {
      expect(src).toContain("plan.patchVersion");
      expect(src).toContain("plan.minorVersion");
      expect(src).toContain("plan.majorVersion");
      expect(src).toContain("applySuggestedVersion");
      expect(src).toContain("release.semver.patch.hint");
      expect(src).toContain("release.semver.minor.hint");
      expect(src).toContain("release.semver.major.hint");
    });
  });

  describe("CreateTagModal Tag Type Badge & SemVer Presets", () => {
    const src = readFileSync(createTagModalFile, "utf8");

    test("declares optional lastTag prop", () => {
      expect(src).toContain("lastTag?: string");
    });

    test("calculates and displays semver presets from lastTag", () => {
      expect(src).toContain("semverPresets");
      expect(src).toContain("semverPresets.patch");
      expect(src).toContain("semverPresets.minor");
      expect(src).toContain("semverPresets.major");
    });

    test("renders dynamic Tag Type badge differentiating lightweight, annotated, and signed tags", () => {
      expect(src).toContain("tagTypeBadge");
      expect(src).toContain("branches.tag.type.label");
      expect(src).toContain("branches.tag.type.lightweight");
      expect(src).toContain("branches.tag.type.annotated");
      expect(src).toContain("branches.tag.type.signed");
    });
  });

  describe("BranchesView Integration", () => {
    const src = readFileSync(branchesViewFile, "utf8");

    test("passes latest tag as last-tag to CreateTagModal", () => {
      expect(src).toContain(":last-tag=\"tags[0]?.name\"");
    });
  });

  describe("Types Definition Contract", () => {
    const src = readFileSync(typesFile, "utf8");

    test("ReleasePlan interface has optional semver fields", () => {
      expect(src).toContain("patchVersion?: string");
      expect(src).toContain("minorVersion?: string");
      expect(src).toContain("majorVersion?: string");
    });
  });

  describe("i18n completeness for Round 8", () => {
    const requiredKeys = [
      "release.semver.patch.hint",
      "release.semver.minor.hint",
      "release.semver.major.hint",
      "branches.tag.type.label",
      "branches.tag.type.lightweight",
      "branches.tag.type.annotated",
      "branches.tag.type.signed",
    ];

    for (const key of requiredKeys) {
      test(`zh-CN contains ${key}`, () => {
        expect((zhCN as Record<string, string>)[key]).toBeDefined();
        expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      });

      test(`en-US contains ${key}`, () => {
        expect((enUS as Record<string, string>)[key]).toBeDefined();
        expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      });
    }
  });
});
