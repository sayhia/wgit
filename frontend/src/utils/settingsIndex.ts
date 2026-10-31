import { SETTINGS_PAGE_LABELS, type SettingsLeaf } from "./settingsStructure";

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
  category: SettingsLeaf;
  /** i18n key — resolve with `t()`. */
  labelKey: string;
  keywords: string[];
}

export const SETTINGS_CATEGORY_LABEL_KEYS = SETTINGS_PAGE_LABELS;

/**
 * Search aliases per leaf. Bilingual on purpose (see the module comment); both
 * the navigation filter and the search box read the same map so a query never
 * matches a leaf in one place but not the other.
 */
export const SETTINGS_LEAF_KEYWORDS: Readonly<Record<SettingsLeaf, readonly string[]>> = {
  appearance: ["主题", "配色", "字体", "外观", "appearance", "theme", "color"],
  identity: ["身份", "用户名", "邮箱", "认证", "identity", "user", "email", "name"],
  signing: ["签名", "密钥", "秘钥", "gpg", "ssh", "signing", "sign"],
  tools: ["对比", "合并", "外部工具", "beyond", "compare", "merge", "diff", "tool"],
  aliases: ["别名", "alias", "快捷指令", "aliases", "命令", "command"],
  hooks: ["钩子", "hook", "hooks", "pre-commit", "commit-msg", "pre-push", "husky", "脚本", "script"],
  "ai-connection": ["ai", "ollama", "大模型", "端侧", "openai", "localai", "api", "cli", "模型", "连接", "密钥", "profile"],
  "ai-behavior": ["ai", "生成", "路由", "重试", "限流", "prompt", "功能开关", "fallback", "behavior", "generation"],
  "ai-insights": ["ai", "用量", "成本", "诊断", "预算", "导入", "导出", "usage", "diagnostics", "budget", "import", "export"],
  proxy: ["代理", "网络", "加速", "proxy", "http", "socks"],
  config: ["配置", "高级", "config", "core", "设置"],
  about: ["关于", "版本", "诊断", "崩溃", "about", "version", "system", "info"],
  shortcuts: ["快捷键", "键盘", "键位", "shortcut", "keybinding", "架构", "版本", "keys"],
};

export interface SettingsHighlightPart {
  text: string;
  hit: boolean;
}

/** Split text into plain/highlighted segments for a settings query. */
export function highlightParts(text: string, query: string): SettingsHighlightPart[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return [{ text, hit: false }];
  const haystack = text.toLowerCase();
  const parts: SettingsHighlightPart[] = [];
  let cursor = 0;
  while (cursor < text.length) {
    const match = haystack.indexOf(needle, cursor);
    if (match < 0) {
      parts.push({ text: text.slice(cursor), hit: false });
      break;
    }
    if (match > cursor) parts.push({ text: text.slice(cursor, match), hit: false });
    parts.push({ text: text.slice(match, match + needle.length), hit: true });
    cursor = match + needle.length;
  }
  return parts.length > 0 ? parts : [{ text, hit: false }];
}


export const SETTINGS_ITEM_INDEX: SettingsIndexItem[] = [
  { category: "about", labelKey: "settings.about.title", keywords: ["关于", "版本", "诊断", "崩溃", "系统信息", "about", "version", "system", "info", "diagnostics"] },
  { category: "ai-connection", labelKey: "settings.ai.connection.title", keywords: ["ai", "ollama", "模型", "openai", "localai", "api", "cli", "endpoint", "连接", "服务"] },
  { category: "ai-connection", labelKey: "ai.profile.title", keywords: ["ai", "档案", "配置档案", "profile", "配置", "重命名"] },
  { category: "ai-connection", labelKey: "settings.ai.keys.title", keywords: ["ai", "密钥", "key", "api key", "多 key", "端点"] },
  { category: "ai-behavior", labelKey: "settings.localai.commit.title", keywords: ["ai", "提交", "生成", "语言", "commit", "prompt", "instructions"] },
  { category: "ai-behavior", labelKey: "settings.ai.tasks.title", keywords: ["ai", "任务", "路由", "模型", "routing", "review", "release", "任务路由"] },
  { category: "ai-behavior", labelKey: "settings.ai.reliability.title", keywords: ["ai", "可靠性", "重试", "限流", "fallback", "备用", "retry"] },
  { category: "ai-behavior", labelKey: "settings.flags.title", keywords: ["ai", "功能", "开关", "灰度", "flags", "冲突"] },
  { category: "ai-insights", labelKey: "ai.usage.title", keywords: ["ai", "用量", "成本", "预算", "budget", "usage", "cost", "token"] },
  { category: "ai-insights", labelKey: "settings.ai.diagnostics.title", keywords: ["ai", "仓库 ai", "诊断", "diagnostics", "状态", "错误"] },
  { category: "ai-insights", labelKey: "settings.ai.transfer.title", keywords: ["ai", "导入", "导出", "import", "export", "迁移"] },
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
  { category: "hooks", labelKey: "settings.item.hooks.manager", keywords: ["git hooks", "钩子", "pre-commit", "commit-msg", "pre-push", "husky", "lefthook"] },
  { category: "hooks", labelKey: "settings.item.hooks.editor", keywords: ["脚本编辑", "hook editor", "自定义脚本", "dry run", "测试运行"] },
  { category: "config", labelKey: "settings.item.config.core", keywords: ["配置", "core", "config", "git config"] },
  { category: "config", labelKey: "settings.item.config.custom", keywords: ["自定义配置", "custom", "config"] },
  { category: "shortcuts", labelKey: "settings.item.shortcuts.record", keywords: ["快捷键", "录制", "键位", "shortcut", "keybinding", "record"] },
  { category: "shortcuts", labelKey: "settings.item.shortcuts.conflict", keywords: ["冲突", "检测", "conflict", "shortcut"] },
  { category: "shortcuts", labelKey: "settings.item.shortcuts.transfer", keywords: ["导入", "导出", "自检", "config", "import", "export", "json"] },
  { category: "shortcuts", labelKey: "settings.item.shortcuts.reset", keywords: ["恢复默认", "重置", "reset", "shortcut"] },
];
