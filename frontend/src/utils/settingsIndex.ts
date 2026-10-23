/**
 * Lightweight index of discoverable settings items for the Settings search box.
 * Category ids must match SettingsView's SettingsCategory values.
 */

export interface SettingsIndexItem {
  category: string;
  label: string;
  keywords: string[];
}

export const SETTINGS_CATEGORY_LABELS: Record<string, string> = {
  appearance: "外观与主题",
  identity: "Git 身份认证",
  signing: "GPG / SSH 防伪签名",
  proxy: "网络与加速代理",
  tools: "外部对比与合并工具",
  config: "高级 Git 配置",
  shortcuts: "快捷键与架构信息",
};

export const SETTINGS_ITEM_INDEX: SettingsIndexItem[] = [
  { category: "appearance", label: "主题模式（深色 / 浅色 / 跟随系统）", keywords: ["主题", "深色", "浅色", "主题模式", "appearance", "theme", "dark", "light"] },
  { category: "appearance", label: "品牌强调色与色板", keywords: ["强调色", "品牌色", "配色", "accent", "color", "palette"] },
  { category: "identity", label: "Git 用户名", keywords: ["用户名", "user", "name", "identity"] },
  { category: "identity", label: "Git 邮箱", keywords: ["邮箱", "email", "mail"] },
  { category: "signing", label: "GPG 提交签名密钥", keywords: ["gpg", "签名", "密钥", "signing", "key"] },
  { category: "signing", label: "SSH 签名格式与公钥探测", keywords: ["ssh", "签名格式", "公钥", "signing", "key"] },
  { category: "signing", label: "提交签名开关", keywords: ["签名开关", "sign", "commit", "gpgsign"] },
  { category: "proxy", label: "HTTP / HTTPS 代理", keywords: ["http", "https", "代理", "proxy"] },
  { category: "proxy", label: "SOCKS5 代理", keywords: ["socks", "socks5", "代理", "proxy"] },
  { category: "tools", label: "外部 Diff 对比工具", keywords: ["diff", "对比", "外部工具", "tool"] },
  { category: "tools", label: "外部合并工具 (Merge Tool)", keywords: ["merge", "合并", "mergetool", "tool"] },
  { category: "tools", label: "自定义外部命令与路径", keywords: ["自定义", "命令", "路径", "custom", "command", "path"] },
  { category: "config", label: "核心 Git 配置项", keywords: ["配置", "core", "config", "git config"] },
  { category: "config", label: "自定义配置项", keywords: ["自定义配置", "custom", "config"] },
  { category: "shortcuts", label: "快捷键录制与替换", keywords: ["快捷键", "录制", "键位", "shortcut", "keybinding", "record"] },
  { category: "shortcuts", label: "快捷键冲突检测", keywords: ["冲突", "检测", "conflict", "shortcut"] },
  { category: "shortcuts", label: "快捷键配置导入 / 导出 / 自检", keywords: ["导入", "导出", "自检", "config", "import", "export", "json"] },
  { category: "shortcuts", label: "恢复默认快捷键", keywords: ["恢复默认", "重置", "reset", "shortcut"] },
];
