# wgit 设计规范系统 v1.3

> 面向 wgit（游戏研发 Git 工作台）的视觉与交互规范。
> 在线样式手册：`design-system/index.html` · 令牌源：`design-system/tokens.css`

---

## 0. 适用范围与原则

本规范覆盖 wgit 桌面端的全部界面：工作区、历史图谱、提交详情、80+ 弹窗与欢迎页。
新增界面必须复用本规范的令牌与组件；确需新增时，按第 11 节的流程提案。

四条原则，按优先级排序：

1. **信息优先**：状态、路径、增删行先可读，再谈装饰。
2. **密度可控**：4px 基线；紧凑行高 24px、舒适行高 32px；正文不低于 10px。
3. **状态可辨**：颜色永远与文字/图标同时出现，不单独承载语义。
4. **一致可预期**：同样的动作使用同样的控件、位置与文案结构。

### 主题策略

- 深色为默认主题，浅色为对等主题（不是"反色补丁"）。
- 两套主题共享同一组**语义令牌名**，只替换值；组件不写死颜色。
- 切换方式：`<html data-theme="light">`；缺省（无属性）即深色。

---

## 1. 令牌架构

### 1.1 命名规则

```
--ug-<类别>-<角色>[-<状态>]
```

| 类别 | 示例 | 说明 |
|---|---|---|
| `bg` | `--ug-bg-surface` | 表面层级 |
| `text` | `--ug-text-secondary` | 文本层级 |
| `border` | `--ug-border-default` | 边框 |
| `accent` / `success` / `warning` / `danger` / `info` | `--ug-accent-bg` | 语义色三件套（文本/底色/边框） |
| `type` | `--ug-type-feat` | 提交类型色 |
| `diff` | `--ug-diff-add-bg` | 差异视图 |
| `text-*`（字号） | `--ug-text-11` | 字体阶梯 |
| `space` / `radius` / `shadow` / `duration` / `ease` | `--ug-space-3` | 结构与动效 |
| `z` | `--ug-z-modal` | 层叠顺序 |

共 **155 个令牌**（深浅各一套值），分为 12 个类别。

### 1.2 语义层级

```
画布 canvas  →  卡片 surface  →  抬升 raised  →  浮层 overlay
#111113        #19191b          #222325        #222325
```

- 边框只有 `border-default`；悬停/强调升到 `border-strong`。
- 禁止在组件内部直接写 `#19191b` 之类的原始值。

---

## 2. 色彩系统

### 2.1 表面与文本

> 深色与浅色模式均已做全页面文本对比扫描（工作区 / 历史 / 分支 / 储藏 / 评审 / 工具箱全部子页签 / 设置）；浅色 muted 与部分色相 ink 已针对灰色表面加深到 AA。
> 调色板来源：Radix Colors Custom（默认 blue + gray 色阶）。语义色使用 Radix 对应色阶，
> 映射约定为 3/4 浅底、7 边框、9 实心、10 实心 hover、11 文本、12 高对比文本。

| 角色 | 深色 | 浅色 | 用途 |
|---|---|---|---|
| `bg-canvas` | `#111113` | `#f9f9fb` | 应用画布、历史表、差异区 |
| `bg-surface` | `#19191b` | `#fcfcfd` | 卡片、侧栏、弹窗 |
| `bg-raised` | `#222325` | `#eff0f3` | 次级按钮、表头、选中态 |
| `bg-overlay` | `#222325` | `#fcfcfd` | 悬浮层、弹窗页脚 |
| `border-default` | `#393a40` | `#d8d9e0` | 默认边框 |
| `border-strong` | `#46484f` | `#b9bbc6` | 悬停/强调边框 |
| `text-primary` | `#eeeef0` | `#1e1f24` | 标题、正文、关键数值 |
| `text-secondary` | `#b2b3bd` | `#62636c` | 说明、作者、次要信息 |
| `text-muted` | `#8b8d98` | `#6b6c75` | 元信息、路径、占位符 |
| `text-disabled` | `#5f606a` | `#b9bbc6` | 禁用态（不承载信息） |

### 2.2 语义色

| 语义 | 深色 | 浅色 | 使用场景 |
|---|---|---|---|
| accent | `#93b4ff` | `#395bc7` | 链接、选中、SHA、主操作 |
| success | `#3dd68c` | `#1f7d54` | 通过、新增、已锁、已暂存 |
| warning | `#ffca16` | `#9a5b00` | 待优化、未暂存、提醒 |
| danger | `#ff9592` | `#ce2c31` | 异常、删除、冲突 |
| purple | `#d19dff` | `#8145b5` | 标签、重构、LFS |
| pink | `#ff8dcc` | `#c2298a` | 美术资产 |
| cyan | `#4ccce6` | `#0d6f87` | 测试、性能 |
| indigo | `#9eb1ff` | `#3a5bc7` | 关卡、场景 |
| teal | `#0bd8b6` | `#007563` | 资产、音频 |
| violet | `#baa7ff` | `#6550b9` | UI、动效 |
| orange | `#ffa057` | `#cc4e00` | 引擎、构建 |

#### 六阶色阶

每个色相不是「三件套」而是**六阶**，覆盖从文字到实心按钮的全部载色需求：

| 令牌 | Radix step | 约束 |
|---|---|---|
| `--ug-<hue>` | 11 | 色相文本 / 描边，在 `bg-surface` 与 `bg-<hue>-bg` 上可读 |
| `--ug-<hue>-bg` | 3 | 浅底（色调按钮静止态、徽章） |
| `--ug-<hue>-bg-strong` | 4 | 强调浅底（容器/行 hover），只承载 `text-ink*` |
| `--ug-<hue>-strong` | 8–11 | 实心填充；success / danger / cyan 取更深阶，保证白字 ≥4.5:1 |
| `--ug-<hue>-strong-hover` | 7/12 | 实心 hover；统一比 `-strong` 更深一阶 |
| `--ug-warning-contrast` | — | 仅用于 amber 实心位：深色文字，避免白字落在亮黄上 |

**组件映射**：`bg-<hue>` 只用于浅底/描边；实心按钮必须用 `bg-<hue>-strong`，
hover 用 `hover:bg-<hue>-strong-hover`。amber 实心位文字使用 `text-warning-contrast`。

### 2.3 提交类型色

| 类型 | 深色文本 | 浅色文本 | 含义 |
|---|---|---|---|
| feat | `#3dd68c` | `#1f7d54` | 玩法、系统功能 |
| fix | `#ff9592` | `#ce2c31` | 缺陷修复 |
| art | `#ff8dcc` | `#c2298a` | 美术资产 |
| level | `#9eb1ff` | `#3a5bc7` | 关卡场景 |
| asset | `#0bd8b6` | `#007563` | 引擎资产 |
| docs | `#93b4ff` | `#395bc7` | 文档 |
| refactor | `#d19dff` | `#8145b5` | 重构 |
| perf | `#ffca16` | `#9a5b00` | 性能 |
| test | `#4ccce6` | `#0d6f87` | 测试 |
| ui | `#baa7ff` | `#6550b9` | 界面 |
| chore | `#b2b3bd` | `#62636c` | 杂项 |

类型徽章统一 17px 高、9px 等宽字、2px 圆角、32% 透明度描边。

### 2.4 差异色

| 令牌 | 深色 | 浅色 |
|---|---|---|
| `diff-add-bg` / `diff-add-text` | `#132d21` / `#3dd68c` | `#e6f6eb` / `#1f7d54` |
| `diff-del-bg` / `diff-del-text` | `#3b1219` / `#ff9592` | `#feebec` / `#ce2c31` |
| `diff-hunk-bg` / `diff-hunk-text` | `#172448` / `#93b4ff` | `#edf2fe` / `#395bc7` |

差异行不使用语义色底纹以外的颜色；行号列使用 `*-gutter` 令牌加深一档。

---

## 3. 字体与排版

### 3.1 字体族

```css
--ug-font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", ...;
--ug-font-mono: "SF Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, ...;
```

- **等宽体强制场景**：路径、Hash、分支名、行号、增删行、快捷键、Token 值。
- 中文与英文混排不额外调整字距；标题可用 `letter-spacing: -0.015em`。

### 3.2 字号阶梯

| 令牌 | 字号 | 行高 | 字重 | 用途 |
|---|---|---|---|---|
| `text-24` | 24px | 1.25 | 700 | 欢迎页主标题 |
| `text-20` | 20px | 1.3 | 700 | 弹窗主标题、区块标题 |
| `text-16` | 16px | 1.4 | 600 | 品牌名、空状态标题 |
| `text-14` | 14px | 1.45 | 500 | 提交详情标题、引导正文 |
| `text-13` | 13px | 1.45 | 400 | 正文、说明段落 |
| `text-12` | 12px | 1.45 | 400/500 | 输入框、文件行、提交标题 |
| `text-11` | 11px | 1.45 | 400/500 | 次要说明、按钮、表格 |
| `text-10` | 10px | 1.4 | 400/600 | 标签、表头、辅助信息（正文下限） |
| `text-9` | 9px | 1.3 | 400/700 | 纯元信息（增删数、时间、路径碎片） |

**约束**：`text-9` 不得承载唯一关键信息；≥ 10px 的文本必须满足 4.5:1 对比度。

---

## 4. 间距、圆角与层级

### 4.1 间距

4px 基线，允许 2px 与 6px 半档：

| 令牌 | 值 | 典型用途 |
|---|---|---|
| `space-0-5` | 2px | 图标内边距 |
| `space-1` | 4px | 紧凑元素 |
| `space-1-5` | 6px | 徽章与文字 |
| `space-2` | 8px | 默认元素间距 |
| `space-2-5` | 10px | 小卡片内边距 |
| `space-3` | 12px | 卡片内边距 |
| `space-4` | 16px | 弹窗内边距 |
| `space-6` | 24px | 区块间距 |
| `space-8` | 32px | 页面区块 |
| `space-12` | 48px | 欢迎页分区 |

### 4.2 圆角

`xs 2px`（徽章、热力格）· `sm 4px`（小控件）· `md 6px`（按钮、输入）·
`lg 8px`（卡片）· `xl 12px`（弹窗）· `pill 999px`（状态胶囊）。

### 4.3 阴影与层级

| 令牌 | 用途 |
|---|---|
| `shadow-card` | 卡片、选中标签 |
| `shadow-menu` | 下拉、右键菜单、Popover |
| `shadow-modal` | 弹窗 |
| `shadow-toast` | Toast |
| `shadow-focus` | 输入框 3px 聚焦光环 |

z-index：`sticky 10 → dropdown 30 → overlay 40 → modal 50 → toast 60 → menu 100`。

### 4.4 布局常量

| 区域 | 数值 |
|---|---|
| 顶栏 | 48px |
| 状态栏 | 28px |
| 左侧栏 | 240px |
| 暂存列 | 320px |
| 图谱列 | 176px |
| 弹窗宽度 | 820 / 860 / 880 / 940px，`max-width: calc(100vw - 32px)` |

### 4.5 材质：Liquid Glass

玻璃只用于**框架**（顶栏 / 仓库标签栏 / 活动栏）与**浮层**（模态、命令面板、快速打开、右键菜单、下拉菜单、Toast）。内容面板保持不透明——玻璃是浮在应用之上的材质，不是阅读面的滤镜。

| 变体 | 用途 | tint |
|---|---|---|
| `.glass-bar` | 窗口框架，无圆角、无投影 | 86%（深）/ 88%（浅） |
| `.glass-panel` | 有遮罩的浮层 | 同上 |
| `.glass-popover` | 直接压在内容上的菜单 | 94% |

四层构成，缺一层就不像玻璃：

1. 半透明 tint + `backdrop-filter: blur() saturate() brightness()`；
2. `inset 0 1px` 顶部高光 + `inset 0 -1px` 底部暗线（厚度感来自这两笔）；
3. `::before` 的指针跟随径向高光 + 顶部线性色晕（`@property` 注册 `--glass-x/--glass-y` 才能插值；`z-index: -1` 保证画在文字之下）；
4. 宽柔阴影 + 贴地阴影（浮层离开页面）。

约束：

- **拖拽条不是玻璃。** macOS 的 `-webkit-app-region: drag` 由 WKWebView 从元素「怎么绘制」推导，实测**任何** `.glass` 变体都会让窗口拖不动（伪元素、`isolation`、`backdrop-filter`、`fixed` 背景逐个排除后仍失败）。顶栏因此保持 `bg-surface/70 backdrop-blur-xl`；仓库标签栏、活动栏与所有浮层仍用材质。
- tint **不得低于 80% 不透明**（`tests/glass.test.ts` 断言）。这是可读性下限：背景最多漂移 20%，不会把弱化文本拖下 4.5。
- 交互态用 `--glass-raise` / `--glass-raise-strong`（主题墨色 7% / 12% 洗色），不要用不透明的 `bg-raised` —— 那会在玻璃上「打个洞」。
- `prefers-reduced-transparency: reduce` 与不支持 `backdrop-filter` 的引擎 → 退化为不透明表面并关掉高光；`prefers-reduced-motion: reduce` → 关闭高光过渡。
- 玻璃上的对比度按**实际渲染像素**验收（截取无文字条带取最亮/最暗像素），不靠 CSS 推算：`backdrop-filter` 让「画出来的背景」是 tint、极光与背后内容的混合。

---

## 5. 动效

| 令牌 | 值 | 用途 |
|---|---|---|
| `duration-fast` | 120ms | 悬停、按下 |
| `duration-base` | 160ms | 弹窗、Toast、展开 |
| `duration-slow` | 240ms | 主题切换、布局变化 |
| `duration-gauge` | 700ms | 健康评分环形进度 |
| `ease-out` | `cubic-bezier(.22,1,.36,1)` | 入场、到达 |
| `ease-in-out` | `cubic-bezier(.65,0,.35,1)` | 往复、主题过渡 |

规则：
- 不做弹跳、不做超过 300ms 的界面转场。
- 所有动效必须在 `prefers-reduced-motion: reduce` 下关闭。
- 动效不得阻塞输入；加载态用 spinner（700ms 线性旋转）。

---

## 6. 组件规范

### 6.1 按钮 `Button`

| 尺寸 | 高度 | 内边距 | 字号 | 圆角 |
|---|---|---|---|---|
| sm | 24px | 0 8px | 10px | 4px |
| md（默认） | 28px | 0 10px | 11px | 6px |
| lg | 32px | 0 14px | 12px | 6px |

变体：`primary`（accent-strong 底 + 白字）、`success`、`danger`、`secondary`（raised 底 + 边框）、`ghost`。
状态：default / hover（底色加深）/ active（下移 1px）/ focus-visible（2px 外环）/ disabled（45% 透明）/ loading（spinner + 文案）。
约束：每屏主按钮 ≤ 1；危险操作需二次确认；悬停色必须保持白字 ≥ 4.5:1（已验证 5.5:1）。

### 6.2 输入框 / 下拉 / 多行 `Input`

高度 30px，圆角 6px，画布底色，`border-default` 边框。
状态：hover → `border-strong`；focus → `border-accent` + `shadow-focus`；error → `border-danger` + 下方红色文案。
占位符使用 `text-muted`；标签使用 10px `text-secondary`。
多行输入最小 64px，可垂直缩放。

### 6.3 复选框 `Checkbox`

13px 原生控件，`accent-color` 跟随语义色：
默认 accent、成功场景 success、筛选场景 warning。禁用态 45% 透明。
分区全选支持 `indeterminate`；选中数变化时同步。

### 6.4 徽章 `Badge`

高度 17px，字号 9px，2px 圆角，等宽体。
- 文件状态：`M` 黄、`A/?` 绿、`D` 红、`R` 蓝。
- 资产标记：`LFS` 紫、`BIN` 黄、`🔒 owner` 红。
- 同步标记：`2↑ / 3↓` 中性。
- 文字型徽章（如 `NEW`、`7 变更文件`）可切换为无衬线 600。

### 6.5 类型徽章 `Type Chip`

解析 Conventional Commits 前缀后渲染；类型 + 可选 scope（`feat·battle`）。
不使用描边以外的装饰；同一行最多出现一个类型徽章。

### 6.6 标签页 `Tabs`

轨道 `bg-raised` + 1px 边框 + 2px 内边距；选中项回到 `bg-surface` + `shadow-card`。
高度 24px，字号 11px；不使用下划线样式。

### 6.7 表格 `Table`

表头：9px 大写、`bg-overlay`、吸顶；行高 28–32px；悬停整行 `bg-hover`。
数字与 Hash 使用等宽体；作者列使用 16px 彩色头像（基于名字哈希，颜色无语义）。
空状态：图标（26px）+ 标题（12px/600）+ 提示（10px）。

### 6.8 弹窗 `Modal`

结构：标题栏 44px / 内容区可滚动 / 页脚固定。圆角 12px，`shadow-modal`，遮罩 `bg-scrim` + 2px 模糊。
入场 160ms 上移淡入；Esc、遮罩点击、右上 ✕ 均可关闭。
宽度按内容选择 820/860/880/940；移动端 `calc(100vw - 32px)`。

### 6.9 Toast 与内联提示

Toast：右下角单条消息，默认 2.4s（App 级错误 4s），左侧 6px 语义色点，`max-w-md` + `break-words` 允许长错误换行；`shadow-toast`、`role="status"` + `aria-live="polite"`，不使用阻塞式原生 `alert`。
内联提示：`alert` 高度自适应，语义底色 + 语义文本 + 边框三件套；持续可见，不自动消失。

### 6.10 仪表盘 `Gauge`

96–108px 环形，轨道与进度均为 5px，圆角端点；分数 22px 等宽体，评级 9px。
颜色随分数变化：≥90 success、≥75 accent、≥60 warning、其余 danger。
分数与评级必须同时显示，颜色仅作辅助。

### 6.11 文件行 `File Row`

两行式（320px 列）：
- 首行：复选框 13px → 状态徽章 16px → LFS/BIN/锁标记 → 文件名（12px 等宽，可截断）→ 右侧 `+N −M`。
- 次行：目录（9px 等宽，缩进 22px）+ 悬停操作（显示/打开/加锁/暂存···）。
- 行高 ≈ 38px；悬停 `bg-hover`；选中左侧 2px accent 边框。
- 名称优先于目录：目录可截断，文件名必须尽量完整。

### 6.12 提交行 `Commit Row`

列结构：图谱节点 24px → 类型徽章 + 描述 + refs → 时间（9px）→ SHA（10px accent）。
图谱节点 8px，按分支着色，悬停加 3px 光晕；行高 32px。

### 6.13 工作区概览 `Workspace Summary`

四枚状态 chip（暂存/未暂存/未跟踪/冲突）+ 汇总行（文件数 + `+N −M`）+ 类别堆叠条（5px 高）。
chip 选中时用 1px `currentColor` 内描边，不使用填充色块。

---

## 7. 无障碍

### 7.1 对比度（WCAG AA，逐对断言）

文本与表面（`tests/contrast.test.ts` 按 6 强调色 × 深/浅共 12 组合逐对校验，全部 ≥ 4.5）：

| 组合 | 深色 | 浅色 |
|---|---|---|
| 主文本 / 画布 | 16.28 | 15.64 |
| 主文本 / 卡片 | 15.15 | 16.04 |
| 主文本 / 抬升面 | 13.57 | 14.43 |
| 次文本 / 卡片 | 8.42 | 5.82 |
| 次文本 / 抬升面 | 7.55 | 5.23 |
| 弱化文本 / 画布 | 5.71 | 5.59 |
| 弱化文本 / 卡片 | 5.32 | 5.73 |
| 弱化文本 / 抬升面 | 4.76 | 5.16 |
| 强调色 / 卡片 | 8.54 | 6.71 |

实心填充 + 标签墨色（`--ug-*-contrast` 由主题与强调色共同决定）：

| 组合 | 深色 | 浅色 |
|---|---|---|
| 标签 / 主按钮 | 5.21 | 5.21 |
| 标签 / 主按钮 hover | 6.23 | 6.35 |
| 白字 / 危险按钮 | 5.37 | 5.21 |
| 深字 / 警告按钮（琥珀） | 11.12 | 11.12 |

三条规则，都由实测反推：

1. **实心填充必须用自己声明的标签墨色。** `bg-accent*` 配 `text-accent-contrast`，不许写 `text-white`：teal / green / amber 在深色主题下是**深色**墨色，写死白字会掉到 2.62–3.74。源码守卫见 `tests/contrast.test.ts`。
2. **不要用透明度修饰文本。** `text-ink-muted/70` 会算成 `color-mix(… 70%, transparent)`，在抬升面上只有 3.1:1；`/60` 更低。需要弱化时用 `text-ink-muted` 本身，靠字号与字重分层级。唯一例外是 `/0`（悬停才显形）。
3. **徽章不要拿自身墨色当底色。** `bg-<hue>/15` + 同色文字会把比值压掉约 15%，浅色主题的 success / cyan / purple 墨色为此各压深了一档；计数徽标改用 1px 内描边（`ring-1 ring-inset`）而不是洗色 —— 白字叠在 15% 白底上会从 5.21 掉到 3.93。

**最差组合不是「卡片」而是「悬停行」。** 徽章常出现在 hover 行（`bg-hover-strong`）上，底色更暗，浅色墨色要按这个组合求解，而不是按卡片。

### 7.2 键盘与焦点

- `⌘K` 命令面板、`⌘⇧F` 代码检索、`Esc` 关闭弹窗、`?` 快捷键指南。
- 所有交互元素可通过 Tab 到达，焦点环 2px、偏移 2px。
- 弹窗打开后焦点进入首个可操作元素；关闭后返回触发按钮。表单控件优先使用可见 `<label>` / `aria-label`；长尾仅含 placeholder 的控件由 `installAriaLabelFallback` 自动补 `aria-label`，显式标签始终优先且不会被覆盖。全部 37 个弹窗遮罩（独立弹窗组件，以及 History / Branches / Stash / Workspace / Tools / ChangesList 内联弹窗）统一具备 `role="dialog"`、`aria-modal="true"`、可访问名称，并由 `useModalA11y` 提供初始焦点、Tab 循环、Esc 关闭与焦点返回。

### 7.3 动效与触控

- 尊重 `prefers-reduced-motion`；gauge 与弹窗动效全部关闭。
- 桌面点击目标 ≥ 24px；触控/移动端 ≥ 44px。

### 7.4 语义表达

- 颜色不单独承载语义：状态必须同时有文字（`通过/待优化/异常`）或图形（✓/⚠/✕）。
- 图表（热力图）提供 title/aria 文本；色阶之外给出数值。

---

## 8. 实施与迁移

### 8.1 引入（原生 CSS）

```css
@import "./design-system/tokens.css";
```

```html
<html data-theme="light">
```

### 8.2 在 Tailwind v4 中接入

`src/style.css` 已把令牌映射为语义工具类：

```css
@import "../design-system/tokens.css";
@import "tailwindcss";

@theme {
  --color-surface: var(--ug-bg-surface);
  --color-raised: var(--ug-bg-raised);
  --color-line: var(--ug-border-default);
  --color-ink: var(--ug-text-primary);
  /* ... */
}
```

对应工具类：

| 类别 | 类名 |
|---|---|
| 表面 | `bg-canvas` · `bg-surface` · `bg-raised` · `bg-overlay` · `bg-hover-strong` |
| 边框 | `border-line` · `border-line-strong` · `border-line-subtle` |
| 文本 | `text-ink` · `text-ink-secondary` · `text-ink-muted` |
| 语义 | `bg-accent-strong` · `bg-success-strong` · `bg-danger-strong` 及其 hover 变体 |
| 色相五阶 | `text-cyan` · `bg-cyan-bg` · `bg-cyan-bg-strong` · `bg-cyan-strong` · `bg-cyan-strong-hover` |
| 反色文字 | `text-inverse`（深/浅主题恒为 `#ffffff`，只用在实心填充上） |

**主题同步**：切换主题时同时设置 `data-theme` 与 `light` / `dark` class（由 `useTheme` 的 `apply()` 完成）。

**反色文字令牌**：浅色主题曾用 `html.light .text-white` 全局覆盖把「深底浅字」翻成「浅底深字」，
但这会把实心按钮上的白字也改成深字。该覆盖已删除；`text-white` 现在始终为白色，只用于实心填充或
深色渐变。需要随主题反转的正文使用 `text-ink*`，实心位使用 `text-inverse`。

### 8.3 首批迁移结果（v1.1）

- 表面：`bg-[#0d1117] / bg-[#161b22] / bg-[#21262d] / bg-[#1c2128] / bg-[#30363d]` → `bg-canvas / bg-surface / bg-raised / bg-overlay / bg-hover-strong`，共 **1,126 处**。
- 边框：`border-[#30363d] / border-[#21262d]` → `border-line / border-line-subtle`，共 **693 处**。
- 中性文本：`text-gray-400 / text-gray-500` → `text-ink-secondary / text-ink-muted`，共 **722 处**。
- 透明度：带 `/10`、`/15` 等后缀的 tint 由 Tailwind v4 的 `color-mix` 处理，深浅主题共用同一套 token。
- 迁移完成后，旧的 `html.light .bg-[#...]`、`.text-gray-*` 覆盖规则已删除。

### 8.3b 第二批迁移结果（v1.2）

第二批处理「彩色」：Tailwind 调色板工具类在 `index.html` + `src/main.ts` 中**归零**。

| 原始写法 | 数量 | 替换为 |
|---|---|---|
| `bg-cyan-600 hover:bg-cyan-500` | 11 | `bg-cyan-strong hover:bg-cyan-strong-hover` |
| `bg-amber-600/700 hover:bg-amber-500/600` | 13 | `bg-warning-strong hover:bg-warning-strong-hover` |
| `bg-indigo-600 hover:bg-indigo-500` | 7 | `bg-indigo-strong hover:bg-indigo-strong-hover` |
| `bg-teal-600 hover:bg-teal-500` | 4 | `bg-teal-strong hover:bg-teal-strong-hover` |
| `bg-yellow-600 hover:bg-yellow-500` | 4 | `bg-warning-strong hover:bg-warning-strong-hover` |
| `bg-purple-600/700/900 hover:bg-purple-*` | 5 | `bg-purple-strong/-bg` 组合 |
| `bg-red-800 hover:bg-red-700`、`bg-rose-*` | 4 | `bg-danger-bg hover:bg-danger-strong` |
| `bg-sky-600 hover:bg-sky-500` | 1 | `bg-accent-strong hover:bg-accent-strong-hover` |
| `bg-<hue>-900/950[/opacity] hover:bg-<hue>-800/900` | 23 | `bg-<hue>-bg text-<hue> hover:bg-<hue>-strong` |
| `text-<hue>-200/300/400/500` | 658 | `text-<hue>` |
| `border-<hue>-700/800` | — | `border-<hue>-border` |
| `from/to-<hue>-500/600`、`accent-<hue>-500` | 14 | `*-<hue>-strong` / `accent-<hue>` |

同时清掉 851 字符的 `html.light [class*="bg-emerald-9"]…` 一批色相覆盖规则 —— 令牌本身已带浅色值，
不再需要按类名做十六进制替换。

**两个必须记住的坑：**

1. `hover:bg-hover-strong` 里的 `hover-strong` 是**中性表面**令牌，不是语义实心填充。
   按 `hover:bg-<word>-strong` 做批量替换时它会误命中，把中性按钮的文字换成 `text-inverse`，
   浅色主题下变成白字配浅灰底（几乎不可读）。判定实心填充必须枚举色相名，不能用通配。
2. `-bg` 与 `-strong` 组合的 hover 目标要按**静止态**分流：静止态已是实心的走 `-strong-hover`，
   静止态是浅底的走 `-strong`。写成统一规则会让「浅底 → 更深实心」变成「浅底 → 仍是深色但不反白」。

**第三批（v1.2.1）：毛玻璃弹窗。** `src/style.css` 里有一条全局规则
`[id^="modal-"] > div:first-child { background-color: rgba(22,27,34,.94) }`，
它用硬编码深色给**所有**弹窗做磨砂面板。深色主题下看不出问题，浅色主题下就变成
「深色外壳 + 浅色内容」的割裂（实测 `panel-bg` 在 `data-theme="light"` 下仍是
`rgba(22,27,34,.94)`，而内部 `bg-canvas` 区域已是 `#f6f8fa`）。改为：

```css
background-color: color-mix(in srgb, var(--ug-bg-surface) 94%, transparent);
```

配 `html.light` 的阴影（`rgba(31,35,40,.3)`）与 97% 面板不透明度，并把 `!important`
边框从 `rgba(255,255,255,.1)` 换成 `var(--ug-border-default)`。验收数据：
深色 `color(srgb .086 .106 .133 / .94)`、浅色 `color(srgb 1 1 1 / .97)`。

> 教训：凡是带 `[id^=…]`、`[class*=…]` 的**全局外观规则**，都是主题迁移的高危区。
> 它们不写在组件里，批量替换类名时完全不会被命中，却会覆盖掉所有组件自己的
> `bg-*`（还常带 `!important`）。验收时必须逐主题取 `getComputedStyle` 的面板背景。

### 8.3c 第三批：材质与对比度（v1.3）

| 变更 | 内容 |
|---|---|
| 新增材质 | `src/styles/glass.css`：`.glass` / `.glass-bar` / `.glass-panel` / `.glass-popover` + `--glass-*` 令牌（tint、blur、saturate、brightness、sheen、bloom、edge、shadow、raise） |
| 新增映射 | `--color-accent-contrast`、`--color-success-contrast`（此前缺失，导致 `text-*-contrast` 是死类，组件一律写 `text-white`） |
| 实心色重算 | teal `#12a594→#0f766e`、pink `#d6409f→#c02682`、rose `#e0316b→#c2255c`、violet `#7c5cff→#6b4ae0`、浅色 teal `#0d9488→#0f766e` / green `#2f9e44→#237032`；深色墨色预设的 hover 改为**变亮** |
| 浅色墨色压深 | muted `#6b6c75→#63646d`、success `#1f7d54→#196644`、warning `#9a5b00→#834d00`、danger `#ce2c31→#a72428`、accent `#395bc7→#3453b5`、purple `#8145b5→#753fa5`、cyan `#0d6f87→#0b6277` |
| 清理 | 删除 `style.css` 里 2024 年的 `.glass-panel` / `.glass-pill` 旧块（与新材质撞名且覆盖它）；51 处 `text-ink-muted/NN` 改回全量墨色 |

### 8.4 存量映射

| 现有写法 | 替换为 |
|---|---|
| `bg-[#0d1117]` | `var(--ug-bg-canvas)` |
| `bg-[#161b22]` | `var(--ug-bg-surface)` |
| `bg-[#21262d]` | `var(--ug-bg-raised)` |
| `border-[#30363d]` | `var(--ug-border-default)` |
| `text-gray-400` | `var(--ug-text-secondary)` |
| `text-gray-500` | `var(--ug-text-muted)`（禁用改用 disabled） |
| `text-blue-400` | `var(--ug-accent)` |
| `rounded-xl` | `var(--ug-radius-xl)` |
| `text-[11px]` | `var(--ug-text-11)` |

迁移顺序：先替换颜色与边框（视觉风险最低），再替换字号与圆角，最后统一动效。
每批替换后跑一次浅色截图对比，避免 `!important` 覆盖冲突。

### 8.5 设计 QA 清单

- [ ] 所有颜色来自令牌，无硬编码 hex
- [ ] 深/浅两套主题逐一走查，6 个强调色各看一眼
- [ ] 正文对比度 ≥ 4.5:1，图形 ≥ 3:1；实心填充用 `text-*-contrast` 而非 `text-white`
- [ ] 文本没有透明度修饰（`text-ink-muted/70` 之类）
- [ ] 徽章文字不在「自身墨色 15% 底色」上
- [ ] 9px 文本不含唯一关键信息
- [ ] 焦点环可见，Tab 顺序合理
- [ ] 弹窗 Esc/遮罩可关闭，焦点可返回
- [ ] 玻璃只出现在框架与浮层；tint ≥ 80%；交互态用 `--glass-raise`
- [ ] 320px 列内文件名优先于目录
- [ ] 动效 ≤ 300ms，且尊重 reduced-motion
- [ ] 状态色配有文字或图标
- [ ] 与既有组件同位置、同文案结构

自动化替代人工走查的部分：`bun test tests`（对比度 12 组合 × 22 对、玻璃 9 条契约）、`contrast.cjs`（12 组合 × 6 视图逐元素合成背景）、`contrast-deep.cjs`（138 步 / 26 种布局）、`glass-pixels.cjs`（玻璃表面按渲染像素验收）。

---

## 9. 贡献流程

1. **提议**：说明使用场景、对比度验证结果、现有令牌为何不适用。
2. **评审**：命名符合 `--ug-<类别>-<角色>`；必须同时给出深浅两套值。
3. **发布**：更新 `tokens.css` 与 `index.html` 示例；记录版本；破坏性改动升主版本号。

### 版本记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.2.2 | 2026-10-19 | 补充 §7.1「中性文本与表面的组合边界」：`text-ink-muted` 只适用于 `bg-canvas` / `bg-surface`，放到 `bg-raised` 上实测深色 4.08:1、浅色 3.90:1，需升为 `text-ink-secondary`（4.95 / 5.48）|
| v1.2.1 | 2026-10-19 | 毛玻璃弹窗面板改用 `color-mix(var(--ug-bg-surface) 94%)` + 浅色阴影，修掉浅色主题下「深色外壳 + 浅色内容」的割裂；`!important` 边框改为 `--ug-border-default` |
| v1.2.0 | 2026-10-18 | 181 令牌。色相补全五阶（`-bg-strong` / `-strong` / `-strong-hover`）；实心悬停改为「变深」；确立 ghost→filled 色调按钮；新增 `text-inverse`；第二批迁移把彩色工具类从 `index.html` + `src/main.ts` 中清零，并删除全部色相 `html.light` 覆盖；132 项色阶实测全部 ≥ 4.5:1 |
| v1.1.0 | 2026-10-17 | 新增 `hover-strong` 令牌；应用接入 Tailwind `@theme`，完成表面 / 边框 / 中性文本迁移 |
| v1.0.0 | 2026-10-17 | 首个完整规范：154 令牌、双主题、13 组件、对比度实测 |
| v0.9.0 | 2026-10-17 | 从现有界面抽取表面层级、语义色与提交类型色 |
