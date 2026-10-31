export type SettingsGroupId = "general" | "git" | "ai" | "system";

export type SettingsLeaf =
  | "appearance"
  | "identity"
  | "shortcuts"
  | "signing"
  | "tools"
  | "aliases"
  | "hooks"
  | "ai-connection"
  | "ai-behavior"
  | "ai-insights"
  | "proxy"
  | "config"
  | "about";

export interface SettingsGroup {
  id: SettingsGroupId;
  labelKey: string;
  leaves: readonly SettingsLeaf[];
}

export const SETTINGS_GROUPS: readonly SettingsGroup[] = [
  { id: "general", labelKey: "settings.group.general", leaves: ["appearance", "identity", "shortcuts"] },
  { id: "git", labelKey: "settings.group.git", leaves: ["signing", "tools", "aliases", "hooks"] },
  { id: "ai", labelKey: "settings.group.ai", leaves: ["ai-connection", "ai-behavior", "ai-insights"] },
  { id: "system", labelKey: "settings.group.system", leaves: ["proxy", "config", "about"] },
];

export const SETTINGS_LEAVES = SETTINGS_GROUPS.flatMap((group) => group.leaves);

export const SETTINGS_PAGE_LABELS: Readonly<Record<SettingsLeaf, string>> = {
  appearance: "settings.category.appearance",
  identity: "settings.category.identity",
  shortcuts: "settings.category.shortcuts",
  signing: "settings.category.signing",
  tools: "settings.category.tools",
  aliases: "settings.category.aliases",
  hooks: "settings.category.hooks",
  "ai-connection": "settings.ai.connection.title",
  "ai-behavior": "settings.ai.behavior.title",
  "ai-insights": "settings.ai.insights.title",
  proxy: "settings.category.proxy",
  config: "settings.category.config",
  about: "settings.category.about",
};

export const SETTINGS_LEGACY_ALIASES: Readonly<Record<string, SettingsLeaf>> = {
  localai: "ai-connection",
};

export function normalizeSettingsLeaf(value: string | null | undefined): SettingsLeaf {
  const raw = (value ?? "").trim();
  const mapped = SETTINGS_LEGACY_ALIASES[raw] ?? raw;
  return (SETTINGS_LEAVES as readonly string[]).includes(mapped)
    ? (mapped as SettingsLeaf)
    : "appearance";
}
