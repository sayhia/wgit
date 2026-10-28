/**
 * Lightweight index of discoverable settings items for the Settings search box.
 * Category ids must match SettingsView's SettingsCategory values.
 *
 * Labels are stored as i18n keys and resolved at render time so switching the
 * language re-renders the index without rebuilding it. `keywords` stay
 * bilingual on purpose: they are search aliases, not UI copy, and letting a
 * Chinese-speaking user type "主题" while the UI is English (or vice versa) is
 * the whole point of keeping them untranslated.
 */

export interface SettingsIndexItem {
  category: string;
  /** i18n key — resolve with `t()`. */
  labelKey: string;
  keywords: string[];
}

export const SETTINGS_CATEGORY_LABEL_KEYS: Record<string, string> = {
  appearance: "settings.category.appearance",
  identity: "settings.category.identity",
  signing: "settings.category.signing",
  proxy: "settings.category.proxy",
  tools: "settings.category.tools",
  aliases: "settings.category.aliases",
  config: "settings.category.config",
  shortcuts: "settings.category.shortcuts",
  localai: "settings.category.localai",
  about: "settings.category.about",
};

export const SETTINGS_ITEM_INDEX: SettingsIndexItem[] = [
  { category: "about", labelKey: "settings.about.title", keywords: ["关于", "版本", "诊断", "崩溃", "系统信息", "about", "version", "system", "info", "diagnostics"] },
  { category: "localai", labelKey: "settings.localai.title", keywords: ["ai", "ollama", "端侧", "模型", "openai", "localai", "prompt", "冲突"] },
  { category: "appearance", labelKey: "settings.item.appearance.theme", keywords: ["主题", "深色", "浅色", "主题模式", "appearance", "theme", "dark", "light"] },
  { category: "appearance", labelKey: "settings.item.appearance.accent", keywords: ["强调色", "品牌色", "配色", "accent", "color", "palette"] },
  { category: "identity", labelKey: "settings.item.identity.user", keywords: ["用户名", "user", "name", "identity"] },
  { category: "identity", labelKey: "settings.item.identity.email", keywords: ["邮箱", "email", "mail"] },
  { category: "signing", labelKey: "settings.item.signing.gpg", keywords: ["gpg", "签名", "密钥", "signing", "key"] },
  { category: "signing", labelKey: "settings.item.signing.ssh", keywords: ["ssh", "签名格式", "公钥", "signing", "key"] },
  { category: "signing", labelKey: "settings.item.signing.toggle", keywords: ["签名开关", "sign", "commit", "gpgsign"] },
  { category: "proxy", labelKey: "settings.item.proxy.http", keywords: ["http", "https", "代理", "proxy"] },
  { category: "proxy", labelKey: "settings.item.proxy.socks", keywords: ["socks", "socks5", "代理", "proxy"] },
  { category: "tools", labelKey: "settings.item.tools.diff", keywords: ["diff", "对比", "外部工具", "tool"] },
  { category: "tools", labelKey: "settings.item.tools.merge", keywords: ["merge", "合并", "mergetool", "tool"] },
  { category: "tools", labelKey: "settings.item.tools.custom", keywords: ["自定义", "命令", "路径", "custom", "command", "path"] },
  { category: "aliases", labelKey: "settings.aliases.title", keywords: ["别名", "alias", "快捷指令", "aliases", "命令", "command"] },
  { category: "config", labelKey: "settings.item.config.core", keywords: ["配置", "core", "config", "git config"] },
  { category: "config", labelKey: "settings.item.config.custom", keywords: ["自定义配置", "custom", "config"] },
  { category: "shortcuts", labelKey: "settings.item.shortcuts.record", keywords: ["快捷键", "录制", "键位", "shortcut", "keybinding", "record"] },
  { category: "shortcuts", labelKey: "settings.item.shortcuts.conflict", keywords: ["冲突", "检测", "conflict", "shortcut"] },
  { category: "shortcuts", labelKey: "settings.item.shortcuts.transfer", keywords: ["导入", "导出", "自检", "config", "import", "export", "json"] },
  { category: "shortcuts", labelKey: "settings.item.shortcuts.reset", keywords: ["恢复默认", "重置", "reset", "shortcut"] },
];
