import { afterEach, describe, expect, spyOn, test } from "bun:test";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import AppearanceSection from "../src/components/settings/AppearanceSection.vue";
import IdentitySection from "../src/components/settings/IdentitySection.vue";
import ProxySection from "../src/components/settings/ProxySection.vue";
import SigningSection from "../src/components/settings/SigningSection.vue";
import { gitService } from "../src/services/gitService";
import { useTheme } from "../src/composables/useTheme";
import { useAccent } from "../src/composables/useAccent";
import { t } from "../src/i18n";

const wrappers: VueWrapper[] = [];
const restores: Array<() => void> = [];
const repo = "/fixture/repository";

function track<T extends VueWrapper>(wrapper: T): T {
  wrappers.push(wrapper);
  return wrapper;
}

function stub(name: keyof typeof gitService, result: unknown) {
  const spy = spyOn(gitService, name).mockImplementation(async () => result as never);
  restores.push(() => spy.mockRestore());
  return spy;
}

afterEach(() => {
  wrappers.splice(0).forEach((wrapper) => wrapper.unmount());
  restores.splice(0).forEach((restore) => restore());
});

describe("core settings retain their behavior after presentation changes", () => {
  test("identity loads current values and saves trimmed values with the selected scope", async () => {
    stub("getGitConfig", { userName: "Alice", userEmail: "alice@example.com", isGlobal: false });
    const save = stub("setGitConfig", undefined);
    const wrapper = track(mount(IdentitySection, { props: { currentRepoPath: repo } }));
    await flushPromises();
    expect(wrapper.find('input[type="text"]').element.value).toBe("Alice");
    await wrapper.find('input[type="text"]').setValue(" Bob ");
    await wrapper.find('input[type="email"]').setValue(" bob@example.com ");
    await wrapper.find('input[type="checkbox"]').setValue(true);
    await wrapper.find("form").trigger("submit");
    await flushPromises();
    expect(save).toHaveBeenCalledWith(repo, "Bob", "bob@example.com", true);
    expect(wrapper.emitted("notify")?.[0]?.[0]).toBe(t("settings.identity.saved.global"));
  });

  test("proxy saves an address and clears it when the address is empty", async () => {
    stub("getGitProxy", { httpProxy: "", httpsProxy: "", isGlobal: false });
    const save = stub("setGitProxy", undefined);
    const clear = stub("clearGitProxy", undefined);
    const wrapper = track(mount(ProxySection, { props: { currentRepoPath: repo } }));
    await flushPromises();
    await wrapper.find('input[type="text"]').setValue(" http://127.0.0.1:7890 ");
    await wrapper.find('input[type="checkbox"]').setValue(true);
    await wrapper.find("form").trigger("submit");
    await flushPromises();
    expect(save).toHaveBeenCalledWith(repo, "http://127.0.0.1:7890", true);
    await wrapper.find('input[type="text"]').setValue(" ");
    await wrapper.find("form").trigger("submit");
    await flushPromises();
    expect(clear).toHaveBeenCalledWith(repo, true);
  });

  test("signing can select a discovered SSH key and retain repository rule fields on save", async () => {
    stub("getCommitSigningStatus", {
      gpgSignEnabled: true, signingKey: "", gpgFormat: "ssh",
      availableSshKeys: ["~/.ssh/id_ed25519.pub"], hasGpgBinary: false,
      complianceRate: "", compliantCount: 0, totalChecked: 0, recentCommits: [],
    });
    stub("inspectRepoCompliance", {
      hasRulesFile: false,
      rules: { enforceSignedCommits: false, requireCommitMessagePattern: "^feat: .+", disallowDirectCommitBranches: ["main"], maxFileSizeMb: 25, requireEmailDomain: "@example.com" },
    });
    const signingSave = stub("configureCommitSigning", { message: "Saved" });
    const ruleSave = stub("saveRepoComplianceRules", undefined);
    const wrapper = track(mount(SigningSection, { props: { currentRepoPath: repo } }));
    await flushPromises();
    const key = wrapper.findAll('button[type="button"]').find((button) => button.text().includes("~/.ssh/id_ed25519.pub"));
    expect(key).toBeDefined();
    await key!.trigger("click");
    await wrapper.findAll("form")[0]!.trigger("submit");
    await flushPromises();
    expect(signingSave).toHaveBeenCalledWith(repo, true, "~/.ssh/id_ed25519.pub", "ssh");
    await wrapper.findAll("form")[1]!.trigger("submit");
    await flushPromises();
    expect(ruleSave).toHaveBeenCalledWith(repo, {
      enforceSignedCommits: false,
      requireCommitMessagePattern: "^feat: .+",
      disallowDirectCommitBranches: ["main"],
      maxFileSizeMb: 25,
      requireEmailDomain: "@example.com",
    });
  });

  test("appearance preserves theme, accent and accessibility selection", async () => {
    const theme = useTheme();
    const accent = useAccent();
    theme.setThemeMode("dark");
    theme.setHighContrast(false);
    theme.setReduceTransparency(false);
    accent.setAccent("blue");
    const wrapper = track(mount(AppearanceSection));
    const findButton = (text: string) => wrapper.findAll("button").find((button) => button.text().includes(text))!;
    await findButton(t("settings.theme.light")).trigger("click");
    expect(theme.mode.value).toBe("light");
    await findButton(t("settings.accent.violet")).trigger("click");
    expect(accent.accent.value).toBe("violet");
    await wrapper.findAll('input[type="checkbox"]')[0]!.setValue(true);
    await wrapper.findAll('input[type="checkbox"]')[1]!.setValue(true);
    expect(theme.highContrast.value).toBe(true);
    expect(theme.reduceTransparency.value).toBe(true);
    theme.setThemeMode("dark");
    theme.setHighContrast(false);
    theme.setReduceTransparency(false);
    accent.setAccent("blue");
  });
});
