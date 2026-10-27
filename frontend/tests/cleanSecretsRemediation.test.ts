import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const cleanTabFile = fileURLToPath(
  new URL("../src/components/tools/CleanTab.vue", import.meta.url)
);

const requiredKeys = [
  "clean.secrets.remediate",
  "clean.secrets.remediating",
  "clean.confirm.secrets.remediate",
  "clean.toast.secrets.remediated",
  "clean.toast.secrets.remediate.failed",
];

describe("Staged secret remediation wiring", () => {
  const src = readFileSync(cleanTabFile, "utf8");

  test("offers remediation for scanned findings", () => {
    expect(src).toContain("handleRemediateSecrets");
    expect(src).toContain("remediateStagedSecrets(props.currentRepoPath, paths, true, false)");
    expect(src).toContain('t("clean.confirm.secrets.remediate"');
    expect(src).toContain("isRemediatingSecrets");
    expect(src).toContain('v-if="secretFindings.length > 0"');
  });

  test("zh-CN and en-US contain all remediation keys", () => {
    const zh = zhCN as Record<string, string>;
    const en = enUS as Record<string, string>;
    for (const key of requiredKeys) {
      expect(zh[key]?.length ?? 0).toBeGreaterThan(0);
      expect(en[key]?.length ?? 0).toBeGreaterThan(0);
    }
  });
});
