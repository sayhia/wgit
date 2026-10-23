# wgit Design System

产品视觉与交互规范的唯一真源，包含设计令牌、在线样式手册与书面规范。

## 文件

| 文件 | 说明 |
|---|---|
| `tokens.css` | 181 个设计令牌，深色默认 / 浅色通过 `data-theme="light"` 切换；11 个色相各带五阶色阶 |
| `index.html` | 可交互的在线规范：色彩、排版、间距、动效、13 个组件与产品组合模式 |
| `DESIGN_SYSTEM.md` | 书面规范：令牌架构、组件规格、无障碍结论、迁移与贡献流程 |

## 本地预览

```bash
cd design-system
python3 -m http.server 8080
# 打开 http://127.0.0.1:8080/index.html
```

页面为纯静态资源，无外部依赖；直接双击 `index.html` 也可打开。

## 验收记录

- 无头 Chrome 渲染：`window.__errs = []`，17 个章节、12 个色板、11×5 色阶表全部渲染。
- 主题切换：设置 → 外观与主题（深色 / 浅色 / 跟随系统），偏好写入 `localStorage["wgit-theme"]`；启动前由 `index.html` 内联脚本应用 `data-theme` 与 `html.light/dark`，避免闪烁。正文背景 `#111113` → `#f9f9fb`，色阶来自 Radix Colors Custom。
- 交互：标签页切换、弹窗开关、Toast、色值复制均可用。
- 对比度：关键 9 组组合深/浅两套均 ≥ 4.5:1（AAA/AA），详见 `DESIGN_SYSTEM.md` 第 7 节。
- 色阶实测（v1.2）：无头 Chrome 渲染真实工具类，逐元素取 `getComputedStyle` 并用相对亮度公式求比值，
  11 色相 × 6 状态（文本 / 徽章 / 色调静止 / 色调 hover / 实心 / 实心 hover）× 2 主题 = **132 项**，
  最低 4.52:1（浅色 success 实心），全部通过 AA。

## 约束

- 组件禁止硬编码颜色；新增颜色必须先提案并补齐深浅两套值。
- 修改令牌后需重新走查浅色主题与对比度表。
