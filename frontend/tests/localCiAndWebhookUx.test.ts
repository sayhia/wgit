import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const localCiModalFile = fileURLToPath(
  new URL("../src/components/tools/LocalCiModal.vue", import.meta.url)
);
const webhookSectionFile = fileURLToPath(
  new URL("../src/components/settings/TeamWebhookSection.vue", import.meta.url)
);
const toolsViewFile = fileURLToPath(
  new URL("../src/components/ToolsView.vue", import.meta.url)
);
const toolsSectionFile = fileURLToPath(
  new URL("../src/components/settings/ToolsSection.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);
const typesFile = fileURLToPath(
  new URL("../src/types.ts", import.meta.url)
);

describe("Phase G4: Local CI Sandbox Runner & Team Webhook Hub UX", () => {
  const ciSrc = readFileSync(localCiModalFile, "utf8");
  const webhookSrc = readFileSync(webhookSectionFile, "utf8");
  const toolsViewSrc = readFileSync(toolsViewFile, "utf8");
  const toolsSectionSrc = readFileSync(toolsSectionFile, "utf8");
  const gitServiceSrc = readFileSync(gitServiceFile, "utf8");
  const typesSrc = readFileSync(typesFile, "utf8");

  test("LocalCiModal renders command pipeline, presets, and execution results", () => {
    expect(ciSrc).toContain("rawCommands");
    expect(ciSrc).toContain("applyPreset");
    expect(ciSrc).toContain("handleRunCi");
    expect(ciSrc).toContain("run-ci-btn");
    expect(ciSrc).toContain("ci-results");
    expect(ciSrc).toContain("toggleStep");
    expect(ciSrc).toContain("ci.modal.title");
  });

  test("TeamWebhookSection renders endpoint configuration and test dispatch", () => {
    expect(webhookSrc).toContain("webhookUrl");
    expect(webhookSrc).toContain("eventType");
    expect(webhookSrc).toContain("secret");
    expect(webhookSrc).toContain("handleTestDispatch");
    expect(webhookSrc).toContain("dispatch-webhook-btn");
    expect(webhookSrc).toContain("webhook-result");
    expect(webhookSrc).toContain("webhook.section.title");
  });

  test("ToolsView embeds LocalCiModal and ToolsSection embeds TeamWebhookSection", () => {
    expect(toolsViewSrc).toContain("open-local-ci-btn");
    expect(toolsViewSrc).toContain("LocalCiModal");
    expect(toolsSectionSrc).toContain("TeamWebhookSection");
  });

  test("gitService exposes Phase G4 RPC endpoints", () => {
    expect(gitServiceSrc).toContain("runLocalCiSandbox:");
    expect(gitServiceSrc).toContain("dispatchTeamWebhook:");
  });

  test("types.ts defines Phase G4 interfaces", () => {
    expect(typesSrc).toContain("export interface LocalCiStepResult");
    expect(typesSrc).toContain("export interface LocalCiResult");
    expect(typesSrc).toContain("export interface WebhookDispatchResult");
  });

  test("i18n parity for Local CI and Webhook keys", () => {
    const requiredKeys = [
      "ci.modal.title",
      "ci.modal.desc",
      "ci.commands.label",
      "ci.commands.placeholder",
      "ci.btn.run",
      "ci.btn.running",
      "ci.status.all.passed",
      "ci.status.failed",
      "ci.preset.typecheck",
      "ci.preset.test",
      "ci.preset.build",
      "webhook.section.title",
      "webhook.section.desc",
      "webhook.url.label",
      "webhook.url.placeholder",
      "webhook.event.label",
      "webhook.event.push",
      "webhook.event.tag",
      "webhook.event.merge",
      "webhook.event.ci",
      "webhook.secret.label",
      "webhook.secret.placeholder",
      "webhook.btn.test",
      "webhook.btn.sending",
      "webhook.toast.success",
      "webhook.toast.failed",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
    }
  });
});
