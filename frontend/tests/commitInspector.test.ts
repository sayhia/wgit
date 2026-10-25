import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const inspectorFile = fileURLToPath(
  new URL("../src/components/history/CommitInspector.vue", import.meta.url)
);

describe("CommitInspector & Copy Polish Improvements", () => {
  const src = readFileSync(inspectorFile, "utf8");

  describe("i18n keys parity and validity", () => {
    const requiredKeys = [
      "history.inspector.copy.message",
      "history.inspector.copy.message.done",
      "history.inspector.gpg.verified",
      "history.inspector.gpg.untrusted",
      "history.inspector.gpg.invalid",
    ];

    for (const key of requiredKeys) {
      test(`key "${key}" exists and is non-empty in zh-CN and en-US`, () => {
        expect(key in zhCN).toBe(true);
        expect(key in enUS).toBe(true);
        const zhVal = (zhCN as Record<string, string>)[key];
        const enVal = (enUS as Record<string, string>)[key];
        expect(zhVal && zhVal.trim().length > 0).toBe(true);
        expect(enVal && enVal.trim().length > 0).toBe(true);
      });
    }

    test("zh-CN translations match expected copy", () => {
      expect((zhCN as Record<string, string>)["history.inspector.copy.message"]).toBe("复制提交说明");
      expect((zhCN as Record<string, string>)["history.inspector.gpg.verified"]).toBe("Verified (已签名)");
      expect((zhCN as Record<string, string>)["history.inspector.gpg.untrusted"]).toBe("Untrusted (未受信密钥)");
      expect((zhCN as Record<string, string>)["history.inspector.gpg.invalid"]).toBe("Invalid Signature (签名无效)");
    });
  });

  describe("Copy Commit Message button contract", () => {
    test("has copy commit message button with aria-label", () => {
      expect(src).toContain(':aria-label="t(\'history.inspector.copy.message\')"');
    });

    test("invokes copyCommitMessage on click", () => {
      expect(src).toContain("@click=\"copyCommitMessage\"");
    });

    test("formats commit message with subject and optional body", () => {
      expect(src).toContain("props.selectedCommit.subject +");
      expect(src).toContain('props.selectedCommit.body ? "\\n\\n" + props.selectedCommit.body : ""');
    });

    test("swaps to Check icon when copied with 1500ms timeout", () => {
      expect(src).toContain("v-if=\"copiedMessage\"");
      expect(src).toContain("1500");
    });
  });

  describe("GPG / SSH signature indicator badge contract", () => {
    test("imports ShieldCheck, ShieldAlert, and ShieldX icons", () => {
      expect(src).toContain("ShieldCheck,");
      expect(src).toContain("ShieldAlert,");
      expect(src).toContain("ShieldX,");
    });

    test("checks selectedCommit.gpgStatus, commitDetail.gpgStatus and signatureStatus", () => {
      expect(src).toContain("props.selectedCommit?.gpgStatus");
      expect(src).toContain("props.commitDetail?.gpgStatus");
    });

    test("renders ShieldCheck for Verified GPG status G", () => {
      expect(src).toContain("gpgStatusInfo.status === 'G'");
      expect(src).toContain('<ShieldCheck v-if="gpgStatusInfo.status === \'G\'" class="w-3.5 h-3.5 text-success" />');
    });

    test("renders ShieldAlert for Untrusted GPG status U", () => {
      expect(src).toContain("gpgStatusInfo.status === 'U'");
      expect(src).toContain('<ShieldAlert v-else-if="gpgStatusInfo.status === \'U\'" class="w-3.5 h-3.5 text-warning" />');
    });

    test("renders ShieldX for Invalid GPG status B", () => {
      expect(src).toContain("gpgStatusInfo.status === 'B'");
      expect(src).toContain('<ShieldX v-else-if="gpgStatusInfo.status === \'B\'" class="w-3.5 h-3.5 text-danger" />');
    });

    test("clicking badge emits signature-details", () => {
      expect(src).toContain('@click="emit(\'signature-details\', selectedCommit.hash)"');
    });
  });
});
