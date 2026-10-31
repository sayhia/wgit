/**
 * AiProfilesCard / AiKeysCard — settings redesign Task 5.
 *
 * Source-level assertions pin the extraction contract; the mounted cases drive
 * the real components against the real aiService wrapper with only the Wails
 * runtime mocked (same pattern as aiControlPlaneBehavior.test.ts). Load
 * failures must surface as role="alert" banners per card, and each card must
 * reload when `refreshKey` changes.
 */
import { beforeEach, describe, expect, mock, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { flushPromises, mount } from "@vue/test-utils";
import "./domSetup";
import { t } from "../src/i18n";
import { confirmState, resetConfirm, settleConfirm } from "../src/composables/useConfirm";
import { resetToasts, toasts } from "../src/composables/useToast";
import type { AIAdvancedConfig, AIKeyEntry, AIProfile } from "../src/services/aiService";

const read = (path: string): string =>
  readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

const BASE_ADVANCED: AIAdvancedConfig = {
  taskModels: {},
  repoOverrides: {},
  profileNames: {},
  inputUsdPerMTok: 2.5,
  outputUsdPerMTok: 10,
  monthlyBudgetUsd: 20,
  retryCount: 1,
  rateLimitPerMinute: 0,
  fallbackBaseUrl: "",
  fallbackModel: "",
  fallbackProtocol: "openai",
};

const PROFILE_A: AIProfile = {
  id: "p1",
  name: "Auto A",
  mode: "api",
  baseUrl: "https://api.openai.com/v1",
  model: "gpt-4o",
  protocol: "openai",
  active: true,
};
const PROFILE_B: AIProfile = {
  id: "p2",
  name: "Auto B",
  mode: "api",
  baseUrl: "https://api.anthropic.com/v1",
  model: "claude-sonnet-4-5",
  protocol: "anthropic",
  active: false,
};

const KEY_CURRENT: AIKeyEntry = {
  origin: "https://api.openai.com:443",
  fingerprint: "abcd1234",
  current: true,
  available: true,
};
const KEY_OTHER: AIKeyEntry = {
  origin: "https://api.anthropic.com:443",
  fingerprint: "efgh5678",
  current: false,
  available: false,
};

/** Automatic profile labels the fake backend reports without an override. */
const AUTOMATIC_NAMES: Record<string, string> = { p1: "Auto A", p2: "Auto B" };

let advanced: AIAdvancedConfig = BASE_ADVANCED;
let profiles: AIProfile[] = [];
let keys: AIKeyEntry[] = [];
let profileLoads = 0;
let keyLoads = 0;
let activated: string[] = [];
let deletedProfiles: string[] = [];
let deletedKeyOrigins: string[] = [];
let savedKeys: Array<{ origin: string; apiKey: string }> = [];
let savedAdvanced: AIAdvancedConfig | null = null;
let failProfiles = false;
let failKeys = false;
let failSaveKey = false;

function resetState(): void {
  advanced = { ...BASE_ADVANCED, profileNames: {} };
  profiles = [];
  keys = [];
  profileLoads = 0;
  keyLoads = 0;
  activated = [];
  deletedProfiles = [];
  deletedKeyOrigins = [];
  savedKeys = [];
  savedAdvanced = null;
  failProfiles = false;
  failKeys = false;
  failSaveKey = false;
  resetConfirm();
  resetToasts();
}

async function respond(name: string, args: unknown[]): Promise<unknown> {
  switch (name) {
    case "main.AiService.ListAIProfiles":
      profileLoads += 1;
      if (failProfiles) throw new Error("profiles down");
      return profiles.map((profile) => ({ ...profile }));
    case "main.AiService.ListAIKeys":
      keyLoads += 1;
      if (failKeys) throw new Error("keys down");
      return keys.map((entry) => ({ ...entry }));
    case "main.AiService.ActivateAIProfile": {
      const id = String(args[0] ?? "");
      activated.push(id);
      profiles = profiles.map((profile) => ({ ...profile, active: profile.id === id }));
      return null;
    }
    case "main.AiService.DeleteAIProfile": {
      const id = String(args[0] ?? "");
      deletedProfiles.push(id);
      profiles = profiles.filter((profile) => profile.id !== id);
      return null;
    }
    case "main.AiService.DeleteAIKeyForOrigin": {
      const origin = String(args[0] ?? "");
      deletedKeyOrigins.push(origin);
      keys = keys.filter((entry) => entry.origin !== origin);
      return null;
    }
    case "main.AiService.SaveAIKeyForOrigin": {
      if (failSaveKey) throw new Error("keychain denied");
      const origin = String(args[0] ?? "");
      const apiKey = String(args[1] ?? "");
      savedKeys.push({ origin, apiKey });
      keys = [
        ...keys.filter((entry) => entry.origin !== origin),
        { origin, fingerprint: "9999cafe", current: false, available: true },
      ];
      return null;
    }
    case "main.AiService.GetAIAdvancedConfig":
      return { ...advanced, profileNames: { ...(advanced.profileNames || {}) } };
    case "main.AiService.SaveAIAdvancedConfig": {
      const payload = args[0] as AIAdvancedConfig;
      savedAdvanced = payload;
      advanced = { ...payload, profileNames: { ...(payload.profileNames || {}) } };
      profiles = profiles.map((profile) => ({
        ...profile,
        name: payload.profileNames?.[profile.id] || AUTOMATIC_NAMES[profile.id] || profile.name,
      }));
      return null;
    }
    default:
      return null;
  }
}

mock.module("@wailsio/runtime", () => ({
  Call: {
    ByName: (name: string, ...args: unknown[]) => respond(name, args),
    ByID: () => Promise.resolve(null),
  },
  Events: { On: () => () => {}, Emit: () => {}, Off: () => {} },
  Window: { Name: () => "test" },
  Screens: {},
}));

const AiProfilesCard = (await import("../src/components/settings/AiProfilesCard.vue")).default;
const AiKeysCard = (await import("../src/components/settings/AiKeysCard.vue")).default;

beforeEach(resetState);

describe("settings AI card extraction (source)", () => {
  test("profile card owns rename and delete controls", () => {
    const src = read("../src/components/settings/AiProfilesCard.vue");
    expect(src).toContain("ai-profile-rename");
    expect(src).toContain("activateAIProfile");
    expect(src).toContain("deleteAIProfile");
    // Surface + reload + error contract.
    expect(src).toContain("SettingsCard");
    expect(src).toContain("listAIProfiles");
    expect(src).toContain("getAIAdvancedConfig");
    expect(src).toContain("saveAIAdvancedConfig");
    expect(src).toContain("refreshKey");
    expect(src).toContain('role="alert"');
    expect(src).not.toContain("AiControlPlaneSection");
  });

  test("key card owns multi-origin deletion", () => {
    const src = read("../src/components/settings/AiKeysCard.vue");
    expect(src).toContain("ai-keys-card");
    expect(src).toContain("deleteAIKeyForOrigin");
    expect(src).toContain("saveAIKeyForOrigin");
    expect(src).toContain("ai-key-origin");
    expect(src).toContain("ai-key-value");
    expect(src).toContain("ai-key-add");
    expect(src).toContain("SettingsCard");
    expect(src).toContain("listAIKeys");
    expect(src).toContain("refreshKey");
    expect(src).toContain('role="alert"');
    expect(src).not.toContain("AiControlPlaneSection");
  });
});

describe("AiProfilesCard (mounted)", () => {
  test("renders active state and switches an inactive profile", async () => {
    profiles = [{ ...PROFILE_A }, { ...PROFILE_B }];
    const wrapper = mount(AiProfilesCard);
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-profiles-card']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-profiles-card']").attributes("id")).toBe(
      "ai-section-profiles"
    );
    expect(wrapper.text()).toContain("Auto A");
    expect(wrapper.text()).toContain(t("ai.profile.active"));

    const target = wrapper
      .findAll("button")
      .find((button) => button.text().includes("Auto B"));
    expect(target).toBeTruthy();
    await target!.trigger("click");
    await flushPromises();

    expect(activated).toEqual(["p2"]);
    expect(wrapper.emitted("changed")).toHaveLength(1);
    expect(wrapper.text()).toContain(t("ai.profile.active"));
  });

  test("renames a profile through the advanced config overlay", async () => {
    profiles = [{ ...PROFILE_A }, { ...PROFILE_B }];
    const wrapper = mount(AiProfilesCard);
    await flushPromises();

    await wrapper.findAll("[data-testid='ai-profile-rename']")[1]!.trigger("click");
    const input = wrapper.find("[data-testid='ai-profile-rename-input']");
    expect(input.exists()).toBe(true);
    await input.setValue("Production");
    await input.trigger("keydown", { key: "Enter" });
    await flushPromises();

    expect(savedAdvanced?.profileNames).toEqual({ p2: "Production" });
    expect(wrapper.text()).toContain("Production");
    expect(wrapper.find("[data-testid='ai-profile-rename-input']").exists()).toBe(false);
    expect(wrapper.emitted("changed")).toHaveLength(1);
  });

  test("an empty name removes the override and restores the automatic label", async () => {
    advanced = { ...BASE_ADVANCED, profileNames: { p1: "Temporary" } };
    profiles = [{ ...PROFILE_A, name: "Temporary" }];
    const wrapper = mount(AiProfilesCard);
    await flushPromises();
    expect(wrapper.text()).toContain("Temporary");

    await wrapper.find("[data-testid='ai-profile-rename']").trigger("click");
    await wrapper.find("[data-testid='ai-profile-rename-input']").setValue("");
    await wrapper
      .find("[data-testid='ai-profile-rename-input']")
      .trigger("keydown", { key: "Enter" });
    await flushPromises();

    expect(savedAdvanced?.profileNames).toEqual({});
    expect(wrapper.text()).toContain("Auto A");
    expect(wrapper.emitted("changed")).toHaveLength(1);
  });

  test("surfaces load failures as an alert and retries", async () => {
    failProfiles = true;
    const wrapper = mount(AiProfilesCard);
    await flushPromises();

    const banner = wrapper.find("[data-testid='ai-profiles-card'] [role='alert']");
    expect(banner.exists()).toBe(true);
    expect(banner.text()).toContain("profiles down");

    failProfiles = false;
    profiles = [{ ...PROFILE_A }];
    await wrapper.find("[data-testid='ai-profiles-retry']").trigger("click");
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-profiles-card'] [role='alert']").exists()).toBe(false);
    expect(wrapper.text()).toContain("Auto A");
  });

  test("deletes after confirmation and emits changed", async () => {
    profiles = [{ ...PROFILE_A }];
    const wrapper = mount(AiProfilesCard);
    await flushPromises();

    await wrapper
      .findAll("button")
      .find((button) => button.attributes("aria-label") === t("ai.profile.delete"))!
      .trigger("click");
    expect(confirmState.open).toBe(true);
    settleConfirm(true);
    await flushPromises();

    expect(deletedProfiles).toEqual(["p1"]);
    expect(wrapper.emitted("changed")).toHaveLength(1);
    expect(wrapper.text()).toContain(t("ai.profile.empty"));
  });

  test("reloads when refreshKey changes without reloading on mount twice", async () => {
    profiles = [{ ...PROFILE_A }];
    const wrapper = mount(AiProfilesCard, { props: { refreshKey: 0 } });
    await flushPromises();
    expect(profileLoads).toBe(1);

    profiles = [{ ...PROFILE_B }];
    await wrapper.setProps({ refreshKey: 1 });
    await flushPromises();

    expect(profileLoads).toBe(2);
    expect(wrapper.text()).toContain("Auto B");
  });
});

describe("AiKeysCard (mounted)", () => {
  test("renders origin, fingerprint, current and unavailable state", async () => {
    keys = [{ ...KEY_CURRENT }, { ...KEY_OTHER }];
    const wrapper = mount(AiKeysCard);
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-keys-card']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-keys-card']").attributes("id")).toBe("ai-section-keys");
    expect(wrapper.text()).toContain(KEY_CURRENT.origin);
    expect(wrapper.text()).toContain(KEY_CURRENT.fingerprint);
    expect(wrapper.text()).toContain(t("settings.ai.keys.current"));
    expect(wrapper.text()).toContain(KEY_OTHER.origin);
    expect(wrapper.text()).toContain(KEY_OTHER.fingerprint);
    expect(wrapper.text()).toContain(t("settings.ai.keys.unavailable"));
  });

  test("deletes the confirmed origin only and emits changed", async () => {
    keys = [{ ...KEY_CURRENT }, { ...KEY_OTHER }];
    const wrapper = mount(AiKeysCard);
    await flushPromises();

    const deleteButtons = wrapper
      .findAll("button")
      .filter((button) => button.attributes("aria-label") === t("settings.ai.keys.delete"));
    expect(deleteButtons).toHaveLength(2);
    await deleteButtons[1]!.trigger("click");
    expect(confirmState.open).toBe(true);
    settleConfirm(true);
    await flushPromises();

    expect(deletedKeyOrigins).toEqual([KEY_OTHER.origin]);
    expect(wrapper.text()).not.toContain(KEY_OTHER.origin);
    expect(wrapper.text()).toContain(KEY_CURRENT.origin);
    expect(wrapper.emitted("changed")).toHaveLength(1);
  });

  test("surfaces load failures and the empty state, then reloads on refreshKey", async () => {
    failKeys = true;
    const wrapper = mount(AiKeysCard, { props: { refreshKey: 0 } });
    await flushPromises();
    expect(wrapper.find("[data-testid='ai-keys-card'] [role='alert']").text()).toContain(
      "keys down"
    );

    failKeys = false;
    keys = [{ ...KEY_CURRENT }];
    await wrapper.find("[data-testid='ai-keys-retry']").trigger("click");
    await flushPromises();
    expect(wrapper.find("[data-testid='ai-keys-card'] [role='alert']").exists()).toBe(false);
    expect(wrapper.text()).toContain(KEY_CURRENT.origin);
    expect(keyLoads).toBe(2);

    keys = [];
    await wrapper.setProps({ refreshKey: 1 });
    await flushPromises();
    expect(keyLoads).toBe(3);
    expect(wrapper.text()).toContain(t("settings.ai.keys.empty"));
  });

  test("saves a key for an arbitrary origin, reloads the list and clears the form", async () => {
    keys = [{ ...KEY_CURRENT }];
    const wrapper = mount(AiKeysCard);
    await flushPromises();
    expect(keyLoads).toBe(1);
    expect(wrapper.find("[data-testid='ai-key-add']").attributes("disabled")).toBeDefined();

    const originInput = wrapper.find("[data-testid='ai-key-origin']");
    const valueInput = wrapper.find("[data-testid='ai-key-value']");
    await originInput.setValue(" https://api.example.com/v1 ");
    await valueInput.setValue("sk-test-123");
    expect(wrapper.find("[data-testid='ai-key-add']").attributes("disabled")).toBeUndefined();

    await wrapper.find("[data-testid='ai-key-add']").trigger("click");
    await flushPromises();

    // RPC payload carries the trimmed origin and the key verbatim.
    expect(savedKeys).toEqual([{ origin: "https://api.example.com/v1", apiKey: "sk-test-123" }]);
    // The successful save reloads the list and renders the new origin.
    expect(keyLoads).toBe(2);
    expect(wrapper.text()).toContain("https://api.example.com/v1");
    // The form is cleared for the next entry.
    expect((originInput.element as HTMLInputElement).value).toBe("");
    expect((valueInput.element as HTMLInputElement).value).toBe("");
    expect(toasts.value.some((toast) => toast.text === t("settings.ai.keys.added"))).toBe(true);
    expect(wrapper.emitted("changed")).toHaveLength(1);
  });

  test("a failed save toasts danger, keeps the draft and never emits changed", async () => {
    failSaveKey = true;
    const wrapper = mount(AiKeysCard);
    await flushPromises();
    const loadsBefore = keyLoads;

    await wrapper.find("[data-testid='ai-key-origin']").setValue("https://api.example.com/v1");
    await wrapper.find("[data-testid='ai-key-value']").setValue("sk-bad");
    await wrapper.find("[data-testid='ai-key-add']").trigger("click");
    await flushPromises();

    expect(wrapper.emitted("changed")).toBeUndefined();
    expect(keyLoads).toBe(loadsBefore);
    expect(toasts.value.find((toast) => toast.tone === "danger")?.text).toContain(
      "keychain denied"
    );
    expect((wrapper.find("[data-testid='ai-key-value']").element as HTMLInputElement).value).toBe(
      "sk-bad"
    );
  });

  test("requires both origin and key before calling the backend", async () => {
    const wrapper = mount(AiKeysCard);
    await flushPromises();

    await wrapper.find("[data-testid='ai-key-origin']").setValue("https://api.example.com/v1");
    await wrapper.find("[data-testid='ai-key-value']").setValue("   ");
    await wrapper.find("[data-testid='ai-key-add']").trigger("click");
    await flushPromises();

    expect(savedKeys).toEqual([]);
    expect(wrapper.emitted("changed")).toBeUndefined();
  });
});
