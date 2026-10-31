import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

/**
 * R2 settings wiring: the backend AI config channel (SaveAIConfig) and the
 * five feature-flag toggles (GetAppFlags / SaveAppFlags) reach the settings
 * page, with rollback-on-failure for flag saves and bilingual copy for every
 * new string. Source-assertion style (house pattern: aiSummary.test.ts) plus
 * direct catalog checks.
 */

const repoRoot = fileURLToPath(new URL("../..", import.meta.url));
const read = (path: string) => readFileSync(`${repoRoot}/${path}`, "utf8");

const configGoSrc = read("config.go");
const configFlagsTestGoSrc = read("config_flags_test.go");
const gitServiceSrc = read("frontend/src/services/gitService.ts");
const aiServiceSrc = read("frontend/src/services/aiService.ts");
const connectionSrc = read("frontend/src/components/settings/AiConnectionForm.vue");
const behaviorSrc = read("frontend/src/components/settings/AiBehaviorSection.vue");
const gitBindingsSrc = read("frontend/bindings/wgit/gitservice.ts");

const dict = (m: object) => m as Record<string, string>;

describe("Go backend: GitService.SaveAppFlags", () => {
  test("RPC exists on GitService in config.go with Load→mutate→Save semantics", () => {
    expect(configGoSrc).toContain("func (GitService) SaveAppFlags(flags FeatureFlags) error");
    // Only the Flags field is replaced; everything else rides through.
    expect(configGoSrc).toContain("cfg.Flags = flags");
    expect(configGoSrc).toContain("SaveConfig(cfg)");
  });

  test("table-driven coverage: round-trip plus AI/RecentRepos preservation", () => {
    expect(configFlagsTestGoSrc).toContain("func TestSaveAppFlags(t *testing.T)");
    expect(configFlagsTestGoSrc).toContain("SaveAppFlags");
    expect(configFlagsTestGoSrc).toContain("t.Run(tt.name");
    expect(configFlagsTestGoSrc).toContain("LoadConfig()");
  });
});

describe("frontend services: SaveAppFlags / SaveAIConfig wiring", () => {
  test("saveAppFlags is a callBackend wrapper paired with the GitService RPC", () => {
    expect(gitServiceSrc).toContain('callBackend<void>("SaveAppFlags", flags)');
    expect(gitServiceSrc).toContain("saveAppFlags: (flags: FeatureFlags): Promise<void>");
  });

  test("the SaveAppFlags binding is regenerated on the Wails bindings surface", () => {
    expect(gitBindingsSrc).toContain("SaveAppFlags");
  });

  test("saveAISettings carries the mode, CLI fields and the keep-key contract", () => {
    expect(aiServiceSrc).toContain("export function saveAISettings");
    expect(aiServiceSrc).toContain('"SaveAISettings"');
    // Blank apiKey reaches the backend as "" = "keep the existing key";
    // clearApiKey is the explicit delete path.
    expect(aiServiceSrc).toContain('apiKey: input.apiKey ?? ""');
    expect(aiServiceSrc).toContain("clearApiKey: input.clearApiKey ?? false");
    expect(aiServiceSrc).toContain("export function clearAIKey");
    expect(aiServiceSrc).toContain("export function detectAICLIs");
    expect(aiServiceSrc).toContain("export function testAIConnectionResult");
  });
});

describe("settings page: backend AI config (primary) + localStorage (compat)", () => {
  test("the shared connection form loads sanitized backend state and exposes both modes", () => {
    expect(connectionSrc).toContain("getAIConfig,");
    expect(connectionSrc).toContain("saveAISettings");
    expect(connectionSrc).toContain("await getAIConfig()");
    expect(connectionSrc).toContain("ai.quick.mode.api");
    expect(connectionSrc).toContain("ai.quick.mode.cli");
    expect(connectionSrc).toContain("detectAICLIs");
    expect(connectionSrc).toContain("testAIConnectionResult");
    // The settings page and compact wizard share one connection editor.
    expect(connectionSrc).toContain('data-testid="ai-quick-setup"');
    expect(connectionSrc).not.toContain("localai-backend-status");
  });

  test("saving goes only through the backend and never echoes the key", () => {
    expect(connectionSrc).toContain("await saveAISettings(savePayload(current))");
    expect(connectionSrc).toContain('form.apiKey = "";');
    expect(connectionSrc).not.toContain("saveLocalAiConfig");
    expect(connectionSrc).not.toContain("localStorage");
  });

  test("the API key can be cleared from the OS secret store", () => {
    expect(connectionSrc).toContain("clearAIKey()");
    expect(connectionSrc).toContain("settings.localai.key.clear");
    expect(connectionSrc).toContain("settings.localai.key.clearconfirm");
  });
});

describe("settings page: five feature-flag toggles with rollback", () => {
  test("all five flags have toggles reading GetAppFlags and saving SaveAppFlags", () => {
    // FLAG_DEFS carries every flag key once, and the toggle rows render with
    // dynamic per-flag test ids: one loop, five switches.
    for (const key of ["aiCommitMsg", "aiSummary", "aiConflictBatch", "reviewFlow", "assetBoard"]) {
      expect(behaviorSrc).toContain(`key: "${key}"`);
    }
    expect(behaviorSrc).toContain('data-testid="`flags-toggle-${def.key}`"');
    expect((behaviorSrc.match(/key: "/g) ?? []).filter(Boolean).length).toBeGreaterThanOrEqual(5);
    expect(behaviorSrc).toContain("featureFlagsApi.getFeatureFlags()");
    expect(behaviorSrc).toContain("featureFlagsApi.saveAppFlags({ ...flags })");
  });

  test("a failed flag save rolls the toggle back and reports via the error toast", () => {
    // Optimistic update after snapshotting the previous set…
    expect(behaviorSrc).toContain("const snapshot = { ...flags };");
    expect(behaviorSrc).toContain("flags[key] = !flags[key];");
    // …rolled back in the catch, with a danger toast on the shared channel.
    expect(behaviorSrc).toContain("Object.assign(flags, snapshot)");
    expect(behaviorSrc).toContain('pushToast(t("settings.flags.save.failed"');
    expect(behaviorSrc).toContain('tone: "danger"');
  });

  test("a successful flag save also refreshes the app-wide flag singleton", () => {
    expect(behaviorSrc).toContain("Object.assign(appFeatureFlags, flags)");
  });
});

describe("settings page: model picker", () => {
  test("fetches endpoint models without saving the form and keeps free-text fallback", () => {
    expect(aiServiceSrc).toContain("export function listAIModels");
    expect(connectionSrc).toContain("listAIModels(");
    expect(connectionSrc).toContain('data-testid="ai-quick-model"');
    expect(connectionSrc).toContain('data-testid="ai-quick-model-fetch"');
    expect(connectionSrc).toContain('list="ai-quick-model-options"');
    expect(connectionSrc).toContain('<datalist id="ai-quick-model-options"');
    expect(connectionSrc).toContain("apiOrigin(");
  });
});

describe("settings page: provider and commit preferences", () => {
  test("protocol, headers and CLI env controls are wired to SaveAISettings", () => {
    for (const id of [
      "ai-connection-protocol",
      "localai-azure-version",
      "ai-connection-headers",
      "localai-cli-env",
      "localai-deep-test-btn",
    ]) {
      expect(connectionSrc).toContain(`data-testid="${id}"`);
    }
    expect(connectionSrc).toContain("testAIDeepResult");
    expect(connectionSrc).toContain("form.protocol");
    expect(connectionSrc).toContain("extraHeaders");
    expect(connectionSrc).toContain("form.cliEnvMode");
    expect(connectionSrc).toContain("commitIncludeBody");
    expect(connectionSrc).toContain("commitInstructions");
    for (const id of ["localai-commit-language", "localai-commit-body", "localai-commit-instructions"]) {
      expect(behaviorSrc).toContain(`data-testid="${id}"`);
    }
  });
});

describe("i18n: new settings keys ship in both locales", () => {
  const NEW_KEYS = [
    "settings.localai.backend.title",
    "settings.localai.backend.desc",
    "settings.localai.timeout.label",
    "settings.localai.save.btn",
    "settings.localai.save.succeeded",
    "settings.localai.save.failed",
    "settings.localai.status.configured",
    "settings.localai.status.unconfigured",
    "settings.localai.status.fingerprint",
    "settings.localai.status.key.kept",
    "settings.localai.key.originchanged",
    "settings.localai.legacy.title",
    "settings.localai.legacy.desc",
    "settings.localai.legacy.import",
    "settings.localai.legacy.discard",
    "settings.localai.legacy.imported",
    "settings.localai.legacy.discarded",
    "settings.localai.legacy.discardconfirm",
    "settings.localai.legacy.originwarning",
    "settings.localai.legacy.climode",
    "settings.localai.legacy.incomplete",
    "settings.localai.model.pick",
    "settings.localai.model.custom",
    "settings.localai.model.fetch",
    "settings.localai.model.fetching",
    "settings.localai.model.found",
    "settings.localai.model.hint",
    "settings.localai.model.fetchfailed",
    "settings.localai.protocol.label",
    "settings.localai.protocol.azureversion",
    "settings.localai.protocol.openai.hint",
    "settings.localai.protocol.azure.hint",
    "settings.localai.protocol.anthropic.hint",
    "settings.localai.protocol.gemini.hint",
    "settings.localai.headers.title",
    "settings.localai.headers.hint",
    "settings.localai.headers.invalid",
    "settings.localai.env.label",
    "settings.localai.env.inherit",
    "settings.localai.env.minimal",
    "settings.localai.env.hint",
    "settings.localai.deep.btn",
    "settings.localai.deep.hint",
    "settings.localai.commit.title",
    "settings.localai.commit.desc",
    "settings.localai.commit.language",
    "settings.localai.commit.auto",
    "settings.localai.commit.zh",
    "settings.localai.commit.en",
    "settings.localai.commit.body",
    "settings.localai.commit.bodyhint",
    "settings.localai.commit.instructions",
    "settings.localai.commit.instructionshint",
    "settings.flags.title",
    "settings.flags.desc",
    "settings.flags.save.failed",
    "settings.flags.aicommitmsg.label",
    "settings.flags.aicommitmsg.desc",
    "settings.flags.aisummary.label",
    "settings.flags.aisummary.desc",
    "settings.flags.aiconflictbatch.label",
    "settings.flags.aiconflictbatch.desc",
    "settings.flags.reviewflow.label",
    "settings.flags.reviewflow.desc",
    "settings.flags.assetboard.label",
    "settings.flags.assetboard.desc",
  ];

  test("every new key exists, is non-empty and stays lowercase in both catalogs", () => {
    for (const key of NEW_KEYS) {
      expect(key).toBe(key.toLowerCase());
      for (const [name, catalog] of [
        ["zh-CN", dict(zhCN)],
        ["en-US", dict(enUS)],
      ] as const) {
        const value = catalog[key];
        expect(typeof value).toBe("string");
        expect((value ?? "").length).toBeGreaterThan(0);
        void name;
      }
    }
  });

  test("interpolation placeholders survive in both languages", () => {
    for (const key of ["settings.localai.save.failed", "settings.flags.save.failed"]) {
      expect(dict(zhCN)[key]).toContain("{message}");
      expect(dict(enUS)[key]).toContain("{message}");
    }
    expect(dict(zhCN)["settings.localai.status.fingerprint"]).toContain("{fingerprint}");
    expect(dict(enUS)["settings.localai.status.fingerprint"]).toContain("{fingerprint}");
  });
});
