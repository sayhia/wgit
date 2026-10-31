# wgit 设计规范系统 v1.4 · Workbench

> 面向 wgit（游戏研发 Git 工作台）的视觉与交互规范。
> 在线样式手册：`design-system/index.html` · 令牌源：`design-system/tokens.css`

---

## 0. 适用范围与原则

本规范覆盖 wgit 桌面端的全部界面：工作区、历史图谱、提交详情、80+ 弹窗与欢迎页。
新增界面必须复用本规范的令牌与组件；确需新增时，按第 9 节的流程提案。

四条原则，按优先级排序：

1. **信息优先**：状态、路径、增删行先可读，再谈装饰。
2. **密度可控**：4px 基线；紧凑行高 24px、舒适行高 32px；正文不低于 10px。
3. **状态可辨**：颜色永远与文字/图标同时出现，不单独承载语义。
4. **一致可预期**：同样的动作使用同样的控件、位置与文案结构。

视觉基调是中性的专业工作台：靠实体表面、边框与排版分层；颜色只表达选择、主操作或业务状态。内容区、导航框架不使用渐变、光晕、指针高光或持续模糊。

### 主题策略

- 深色为默认主题，浅色为对等主题（不是"反色补丁"）。
- 两套主题共享同一组**语义令牌名**，只替换值；组件不写死颜色。
- 切换方式：`<html data-theme="light">`；缺省（无属性）即深色。
- 六个强调色只切换强调色族，不改变中性表面与正文层级。
- 实际材质以 `src/styles/glass.css` 为准；旧 `glass-*` 名称是兼容接口，不代表继续使用 Liquid Glass 视觉。

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

`tokens.css` 当前包含 **185 个唯一 `--ug-*` 令牌名**；主题相关的颜色与阴影分别提供深浅值，间距、圆角、排版等结构令牌共用。

### 1.2 语义层级

- `canvas`：工作区底色；`code`：源码阅读面。
- `chrome`：顶栏、仓库标签栏、活动栏；使用实体填色。
- `surface`：内容面板、表单、卡片；`inset`：嵌入式输入或分段轨道。
- `raised` / `hover`：控件与悬停；`selected`：中性选中背景。
- `overlay`：弹窗、菜单、命令面板的浮层基色，由材质样式选择 96% / 98% 混合。

这些令牌表达用途，不是必须逐档变亮的色阶。边框默认使用 `border-default`；分隔线用 `border-subtle`，悬停或强调用 `border-strong`。组件不写死十六进制颜色。
---

## 2. 色彩系统

### 2.1 表面与文本

> 表面采用中性 Workbench 层级；业务语义色保留对应色阶。以下是 `tokens.css` 的当前值。强调色预设另见 `src/accent-presets.css`；实际可访问性需同时校验预设、控件状态与最终渲染背景。

| 角色 | 深色 | 浅色 | 用途 |
|---|---|---|---|
| `bg-canvas` | `#181a1e` | `#f4f5f7` | 应用画布、工作区 |
| `bg-chrome` | `#1c1f24` | `#eef0f4` | 顶栏、仓库栏、活动栏 |
| `bg-surface` | `#202227` | `#ffffff` | 内容面板、设置卡片 |
| `bg-inset` | `#17191d` | `#f7f8fa` | 嵌入式输入、分段轨道 |
| `bg-code` | `#181a1e` | `#fbfcfd` | 源码阅读面 |
| `bg-raised` / `bg-hover` | `#2a2d33` | `#ebedf1` | 控件、悬停行 |
| `bg-hover-strong` | `#343943` | `#e1e5ec` | 加强悬停 |
| `bg-selected` / `bg-active` | `#303844` | `#e5eaf2` | 中性选中背景 |
| `bg-overlay` | `#25282e` | `#ffffff` | 浮层基色 |
| `bg-scrim` | `rgba(8,12,20,.5)` | `rgba(22,30,44,.28)` | 模态遮罩 |
| `border-default` | `#383d46` | `#d6dce5` | 默认边框 |
| `border-strong` | `#596271` | `#a8b3c2` | 悬停、强调边框 |
| `border-subtle` | `#2e333c` | `#e5e9ef` | 轻量分隔线 |
| `text-primary` | `#edf0f5` | `#202630` | 标题、正文、关键数值 |
| `text-secondary` | `#b7beca` | `#505c6e` | 说明、作者、次要信息 |
| `text-muted` | `#9aa3b2` | `#5b6678` | 元信息、路径、占位符 |
| `text-disabled` | `#677180` | `#939dac` | 禁用态 |

### 2.2 语义色

| 语义 | 深色 | 浅色 | 使用场景 |
|---|---|---|---|
| accent | `#93b4ff` | `#3453b5` | 链接、选中、SHA、主操作 |
| success | `#3dd68c` | `#196644` | 通过、新增、已锁、已暂存 |
| warning | `#ffca16` | `#834d00` | 待优化、未暂存、提醒 |
| danger | `#ff9592` | `#a72428` | 异常、删除、冲突 |
| purple | `#d19dff` | `#753fa5` | 标签、重构、LFS |
| pink | `#ff8dcc` | `#c2298a` | 美术资产 |
| cyan | `#4ccce6` | `#0b6277` | 测试、性能 |
| indigo | `#9eb1ff` | `#3a5bc7` | 关卡、场景 |
| teal | `#0bd8b6` | `#007563` | 资产、音频 |
| violet | `#baa7ff` | `#6550b9` | UI、动效 |
| orange | `#ffa057` | `#cc4e00` | 引擎、构建 |

#### 六阶色阶

每个色相提供文字、浅底、边框、实心及交互态角色。按语义选择：

| 令牌 | Radix step | 约束 |
|---|---|---|
| `--ug-<hue>` | 11 | 色相文本 / 描边，在 `bg-surface` 与 `bg-<hue>-bg` 上可读 |
| `--ug-<hue>-bg` | 3 | 浅底（色调按钮静止态、徽章） |
| `--ug-<hue>-bg-strong` | 4 | 强调浅底（容器/行 hover），只承载 `text-ink*` |
| `--ug-<hue>-strong` | 8–11 | 实心填充；success / danger / cyan 取更深阶，保证白字 ≥4.5:1 |
| `--ug-<hue>-strong-hover` | 7/12 | 实心 hover；保持可辨差异与标签对比度，不假定所有预设一律变深 |
| `--ug-warning-contrast` | — | 仅用于 amber 实心位：深色文字，避免白字落在亮黄上 |

**组件映射**：`bg-<hue>` 只用于浅底/描边；实心按钮必须用 `bg-<hue>-strong`，
hover 用 `hover:bg-<hue>-strong-hover`。主操作用 `text-accent-contrast`，amber 实心位用 `text-warning-contrast`；正文、表头、普通卡片保持中性色。

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

`xs 2px`（徽章、热力格）· `sm 4px` · `md 6px` · `lg 8px`（按钮、输入、菜单）·
`xl 12px`（卡片、弹窗）· `pill 999px`（状态胶囊）。

Tailwind 的 `rounded-sm/md/lg/xl` 分别映射到同名令牌；`rounded-2xl` 同样使用 12px，`rounded-3xl` 为 16px。`rounded-full` 保留圆形语义，图谱节点和状态点不随面板圆角缩放。

### 4.3 阴影与层级

阴影用于区分浮层高度；普通内容卡片优先使用实体表面与边框。禁止彩色阴影、顶部反光和内外多重高光。以下为当前完整令牌值：

| 令牌 | 深色 | 浅色 | 用途 |
|---|---|---|---|
| `shadow-card` | `0 1px 2px rgba(0,0,0,.08)` | `0 1px 2px rgba(22,34,52,.035)` | 轻量卡片 |
| `shadow-menu` | `0 4px 18px rgba(0,0,0,.24), 0 1px 4px rgba(0,0,0,.16)` | `0 5px 20px rgba(22,34,52,.12), 0 1px 4px rgba(22,34,52,.06)` | 下拉、右键菜单 |
| `shadow-modal` | `0 16px 48px rgba(0,0,0,.32), 0 2px 8px rgba(0,0,0,.16)` | `0 18px 56px rgba(22,34,52,.18), 0 2px 8px rgba(22,34,52,.08)` | 弹窗、命令面板 |
| `shadow-toast` | `0 6px 24px rgba(0,0,0,.24)` | `0 6px 24px rgba(22,34,52,.13)` | Toast |
| `shadow-focus` | `0 0 0 2px rgba(147,180,255,.35)` | `0 0 0 2px rgba(61,99,221,.24)` | 输入框焦点环 |

`shadow-card/menu/modal` 可直接作为工具类；`shadow-xs/sm` 映射卡片，`shadow-md/lg` 映射菜单，`shadow-xl/2xl` 映射弹窗，避免存量页面使用另一套强度。`--ug-shadow-glow` 仅为兼容名称，其值是 `0 0 0 1px currentColor` 的描边，不是发光效果。

z-index 基础令牌：`sticky 10 → dropdown 30 → overlay 40 → modal 50 → toast 60 → menu 100`。组件自身存在局部层叠上下文，不能仅靠增大子元素的 z-index 跨越被裁切的父容器。

### 4.4 布局常量

| 区域 | 当前基础值 / 约束 |
|---|---|
| 顶栏 | 48px |
| 仓库标签栏 | 36px |
| 活动栏 | 52px |
| 桌面最小窗口 | 960 × 640px |
| 源码 / 变更列 | 默认各 320px，可拖拽；实际宽度按容器与折叠状态调整 |
| 弹窗 | 默认 `max-w-lg`，按内容覆盖；始终保留视口边距与内容滚动 |

`tokens.css` 内的 sidebar / stage / graph 基础常量不是所有页面的固定布局。长标题和工具栏应按所在容器压缩、换行或提供菜单，不通过裁切页面隐藏操作。

### 4.5 材质：中性 Workbench

材质来源为 `src/styles/glass.css`。内容区与框架使用实体填色；透明度只保留在浮层。没有渐变、饱和度增强、亮度滤镜、指针跟随高光或彩色光晕。

| 表面 | 实际样式 | 背景 / 效果 |
|---|---|---|
| 内容 | `bg-canvas` / `bg-code` / `bg-surface` / `bg-inset` | 100% 实体底色，不加模糊 |
| 框架 | `.surface-chrome`；旧 `.glass-bar` | `--ug-bg-chrome`，无模糊、阴影、圆角或动效 |
| 模态浮层 | `.glass.glass-panel`；`.surface-panel` 为面板变体别名 | `--ug-bg-overlay` 96% + 透明 4%；12px 模糊，12px 圆角，`shadow-modal` |
| 菜单浮层 | `.glass.glass-popover`；`.surface-popover` 为菜单变体别名 | `--ug-bg-overlay` 98% + 透明 2%；12px 模糊，8px 圆角，`shadow-menu` |

`.glass` 提供浮层的基础填色与 12px 模糊；`.surface-panel` / `.surface-popover` 单独使用只提供变体的填色、边框、圆角与阴影，不单独开启模糊。深浅主题使用相同透明度，颜色由各主题的 `overlay` 令牌决定。

约束：

- 顶栏、仓库栏、活动栏使用实体 chrome。macOS 拖拽区保留 `-webkit-app-region`，不要加覆盖拖拽面的伪元素或第二层模糊。
- `glass-*` 与 `--glass-*` 是存量兼容接口。`--glass-raise` / `--glass-raise-strong` 分别映射实体 `bg-hover` / `bg-selected`；不再通过主题墨色洗色模拟反光。
- 浮层不得叠加 `backdrop-blur-*`、彩色渐变、内高光或指针特效；遮罩用 `bg-scrim`，自身不模糊。
- `prefers-reduced-transparency`、应用内降低透明度以及不支持模糊的引擎，使用实体 overlay。高对比度关闭模糊和阴影、强化边框；forced-colors 使用系统 `Canvas` / `CanvasText`。
- 深浅主题与所有强调色的对比度需同时验证；浮层额外检查实际渲染像素及背后文字是否透出。没有极光背景参与合成。

### 4.6 数据面板的三态：loading / empty / error

任何一个「装数据的框」都必须能画出三种样子，而且**顺序固定**：`loading` → `error` → `empty`。error 必须排在 empty 之前，否则「加载失败」会被渲染成「没有数据」——这是本项目踩过的真实 bug（历史页把 `git log` 失败显示成「仓库暂无提交」）。

| 态 | 用什么 | 规则 |
| --- | --- | --- |
| loading | `UiSkeleton`（`variant="rows"` 对齐真实行高） | 不用转圈占位：骨架要和最终内容同高，否则数据到达时列表会跳 |
| error | 图标 + 一句结论 + **后端原文** + 重试 | git 的报错是诊断数据，不要翻译成「操作失败」；原文才让人能搜 |
| empty | `UiEmpty`：图标 + 一句解释 + **最多一个**操作 | 三个按钮的空态等于第二个界面 |

`UiEmpty` 的契约就是这三件事：`icon` / `title` / `hint` + 默认插槽放一个操作。列表类页面（历史、分支、储藏）已经统一，新页面直接复用，不要再手写一套。

### 4.7 新增浮层的四步检查

1. 复用 `UiModal` / `UiPopover` / `UiContextMenu` 等共享组件，选择模态或菜单的高度级别。
2. 浮层只保留一处材质声明；页面内容和框架使用实体表面。旧 `data-glass` 可留作兼容标记，不再驱动指针特效。
3. 检查视口边界、可滚动内容和层叠上下文；不要用 `!important`、重复背景或重复模糊修补冲突。
4. 提供明确名称、键盘关闭、合理的初始焦点与焦点返回；允许遮罩关闭时使用共享组件的对应选项。

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

### 6.1 按钮 `UiButton`

| 尺寸 | 最小高度 | 水平 / 垂直内边距 | 字号 | 圆角 |
|---|---|---|---|---|
| xs | 24px | 8px / 4px | 10px | 8px |
| sm（默认） | 28px | 12px / 6px | 12px | 8px |
| md | 按内容与内边距计算 | 16px / 8px | 14px | 8px |

变体与实际类映射以 `src/utils/uiVariants.ts` 为准：`primary`、`secondary`、`ghost`、`danger`、`warning`、`purple`。主操作文字使用 `text-accent-contrast`，警告使用 `text-warning-contrast`，中性操作使用实体 surface/raised，不带光晕。

悬停只改变底色、边框与文字，不让整个工作台的按钮在按下时缩放。键盘焦点为可见外环，禁用态降低透明度并保留名称；加载时 spinner 与文字同时显示。优先保持一个明确主操作。

### 6.2 输入框 / 下拉 / 多行

单行控件以内容与垂直内边距决定高度，统一 8px 圆角、实体 canvas/inset 底色和 `border-default`。焦点使用 `border-accent` 与 2px `shadow-focus`；错误文案在控件下方并可被辅助技术读出。

占位符使用 `text-muted`，可见标签使用 12px `text-secondary`。优先关联 `<label>`，不能只靠示例 placeholder 命名。长路径和模型名允许控件收缩，外层设置卡片不横向溢出。

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

分段轨道使用实体 `bg-inset` 与 1px 边框；选中项用中性 `bg-selected`。页面标签页可用单一强调色下划线表达当前位置，不能再叠渐变或发光。标签保持整词，窄列允许滚动或成组换行。

### 6.7 表格 `Table`

表头：实体 `bg-surface` / `bg-chrome`，吸顶；行高 28–32px；悬停整行 `bg-hover`。表头文字保持足够对比度，不用浮层材质。
数字与 Hash 使用等宽体；作者列使用 16px 彩色头像（基于名字哈希，颜色无语义）。
空状态：图标（26px）+ 标题（12px/600）+ 提示（10px）。

### 6.8 弹窗 `Modal`

结构：标题 / 可滚动内容 / 固定页脚；12px 圆角，96% 中性浮层底色与 `shadow-modal`，遮罩只用 `bg-scrim`。浮层本身统一 12px 模糊，plain 变体使用实体 surface。
Esc 和关闭按钮由共享组件管理；遮罩关闭由 `closeOnBackdrop` 显式选择，不能假定所有对话框都可点遮罩关闭。宽度通过 `max-w-*` 按内容设置，始终保留视口边距。

### 6.9 Toast 与内联提示

Toast：右下角单一消息宿主，最多显示 3 条；默认 2.4s，可按操作覆盖。中性 98% 浮层、8px 圆角、左侧 6px 语义色点与 `shadow-toast`，长错误可换行。普通消息使用 `status` / `polite`，danger 使用 `alert` / `assertive`；悬停或键盘聚焦暂停计时，不使用原生阻塞 `alert`。
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
图谱节点按分支着色；悬停通过节点/线条几何变化或行背景强调，不使用发光过滤。列表行高遵循虚拟滚动的实际行高。

### 6.13 工作区概览 `Workspace Summary`

四枚状态 chip（暂存/未暂存/未跟踪/冲突）+ 汇总行（文件数 + `+N −M`）+ 类别堆叠条（5px 高）。
chip 选中时用 1px `currentColor` 内描边与中性 `bg-selected`；状态色点或文字保留业务含义。

---

## 7. 无障碍

### 7.1 对比度（WCAG AA，逐对断言）

以下是按当前默认蓝色令牌计算的 sRGB 对比度，不代表半透明浮层的最终像素值。`tests/contrast.test.ts` 覆盖全部 6 个强调色 × 深/浅组合；浏览器审计另行检查实际状态与合成背景。

| 组合 | 深色 | 浅色 |
|---|---|---|
| 主文本 / 画布 | 15.25 | 13.94 |
| 主文本 / 卡片 | 13.93 | 15.20 |
| 主文本 / 抬升面 | 12.08 | 12.97 |
| 次文本 / 卡片 | 8.51 | 6.78 |
| 次文本 / 抬升面 | 7.38 | 5.78 |
| 弱化文本 / 画布 | 6.85 | 5.32 |
| 弱化文本 / 卡片 | 6.26 | 5.81 |
| 弱化文本 / 抬升面 | 5.43 | 4.95 |
| 强调色 / 卡片 | 7.74 | 6.88 |

实心填充与标签墨色：

| 组合 | 深色 | 浅色 |
|---|---|---|
| 标签 / 主按钮 | 5.21 | 5.21 |
| 标签 / 主按钮 hover | 6.23 | 6.35 |
| 白字 / 危险按钮 | 5.37 | 5.21 |
| 深字 / 警告按钮（琥珀） | 11.12 | 11.12 |

规则：

1. **实心填充必须用自己声明的标签墨色。** `bg-accent*` 配 `text-accent-contrast`，不许写 `text-white`：teal / green / amber 在深色主题下是**深色**墨色，写死白字会掉到 2.62–3.74。源码守卫见 `tests/contrast.test.ts`。
2. **不要用透明度修饰文本。** `text-ink-muted/70` 会混入背景，使既定的对比度失效。需要弱化时使用 `text-ink-muted`，靠字号与字重分层级。悬停显形的装饰是例外；唯一可操作入口仍需支持键盘聚焦显示。
3. **徽章不要拿自身墨色当底色。** `bg-<hue>/15` + 同色文字会把比值压掉约 15%，浅色主题的 success / cyan / purple 墨色为此各压深了一档；计数徽标改用 1px 内描边（`ring-1 ring-inset`）而不是洗色 —— 白字叠在 15% 白底上会从 5.21 掉到 3.93。

**最差组合不是「卡片」而是「悬停行」。** 徽章常出现在 hover 行（`bg-hover-strong`）上，底色更暗，浅色墨色要按这个组合求解，而不是按卡片。

### 7.2 键盘与焦点

- `⌘K` 命令面板、`⌘⇧F` 代码检索、`Esc` 关闭弹窗、`?` 快捷键指南。
- 所有交互元素可通过 Tab 到达，焦点环 2px、偏移 2px。
- 弹窗打开后焦点进入首个可操作元素；关闭后返回触发按钮。表单控件优先使用可见 `<label>` / `aria-label`；长尾仅含 placeholder 的控件由 `installAriaLabelFallback` 自动补 `aria-label`，显式标签始终优先且不会被覆盖。共享弹窗应具备 `role="dialog"`、`aria-modal="true"`、可访问名称，并由 `useModalA11y` 提供初始焦点、Tab 循环、Esc 关闭与焦点返回。

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
| 表面 | `bg-canvas` · `bg-chrome` · `bg-surface` · `bg-inset` · `bg-raised` · `bg-selected` · `bg-overlay` |
| 边框 | `border-line` · `border-line-strong` · `border-line-subtle` |
| 文本 | `text-ink` · `text-ink-secondary` · `text-ink-muted` |
| 语义 | `bg-accent-strong` · `bg-success-strong` · `bg-danger-strong` 及其 hover 变体 |
| 色相五阶 | `text-cyan` · `bg-cyan-bg` · `bg-cyan-bg-strong` · `bg-cyan-strong` · `bg-cyan-strong-hover` |
| 反色文字 | `text-inverse`（只用于确定需要白字的实心填充；强调色用 `text-accent-contrast`） |
| 阴影 | `shadow-card` · `shadow-menu` · `shadow-modal` · `shadow-toast` |

**主题同步**：切换主题时同时设置 `data-theme` 与 `light` / `dark` class（由 `useTheme` 的 `apply()` 完成）。

**反色文字令牌**：浅色主题曾用 `html.light .text-white` 全局覆盖把「深底浅字」翻成「浅底深字」，
但这会把实心按钮上的白字也改成深字。该覆盖已删除；`text-white` 现在始终为白色，只用于实心填充或
确定的深底。需要随主题反转的正文使用 `text-ink*`；实心位优先使用对应的 `text-*-contrast`。

### 8.3 历史迁移记录（v1.1）

本节及 v1.2/v1.3 记录描述当时的实现；当前材质与数值以第 2、4 节为准。

- 表面：`bg-[#0d1117] / bg-[#161b22] / bg-[#21262d] / bg-[#1c2128] / bg-[#30363d]` → `bg-canvas / bg-surface / bg-raised / bg-overlay / bg-hover-strong`，共 **1,126 处**。
- 边框：`border-[#30363d] / border-[#21262d]` → `border-line / border-line-subtle`，共 **693 处**。
- 中性文本：`text-gray-400 / text-gray-500` → `text-ink-secondary / text-ink-muted`，共 **722 处**。
- 透明度：带 `/10`、`/15` 等后缀的 tint 由 Tailwind v4 的 `color-mix` 处理，深浅主题共用同一套 token。
- 迁移完成后，旧的 `html.light .bg-[#...]`、`.text-gray-*` 覆盖规则已删除。

### 8.3b 历史迁移记录（v1.2）

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

### 8.3c 历史记录：Liquid Glass 与对比度（v1.3）

> 历史实现：下述 saturate、sheen、bloom、指针与光晕效果已在 v1.4 移除，不应据此新建界面。

| 变更 | 内容 |
|---|---|
| 新增材质 | `src/styles/glass.css`：`.glass` / `.glass-bar` / `.glass-panel` / `.glass-popover` + `--glass-*` 令牌（tint、blur、saturate、brightness、sheen、bloom、edge、shadow、raise） |
| 新增映射 | `--color-accent-contrast`、`--color-success-contrast`（此前缺失，导致 `text-*-contrast` 是死类，组件一律写 `text-white`） |
| 实心色重算 | teal `#12a594→#0f766e`、pink `#d6409f→#c02682`、rose `#e0316b→#c2255c`、violet `#7c5cff→#6b4ae0`、浅色 teal `#0d9488→#0f766e` / green `#2f9e44→#237032`；深色墨色预设的 hover 改为**变亮** |
| 浅色墨色压深 | muted `#6b6c75→#63646d`、success `#1f7d54→#196644`、warning `#9a5b00→#834d00`、danger `#ce2c31→#a72428`、accent `#395bc7→#3453b5`、purple `#8145b5→#753fa5`、cyan `#0d6f87→#0b6277` |
| 清理 | 删除 `style.css` 里 2024 年的 `.glass-panel` / `.glass-pill` 旧块（与新材质撞名且覆盖它）；51 处 `text-ink-muted/NN` 改回全量墨色 |

### 8.3d Workbench 重构（v1.4）

- 新增实体 `chrome` / `inset` / `selected` 角色，更新深浅中性面板与正文层级。
- `glass-*` 保留兼容类名，框架改为实体；浮层统一 96% / 98% 中性底色与单层 12px 模糊。
- 移除背景极光、指针光斑、彩色光晕、反光边缘及重复模糊；源码与差异阅读面保持实体。
- 阴影和圆角通过令牌映射到存量 Tailwind 类；表单与共享组件采用统一的边框、密度、焦点处理。
- 合并 Prism 深浅主题规则及原生文件拖放提示规则，保留业务状态、图片棋盘格与减少动画支持。

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
- [ ] 框架与内容为实体；浮层为中性 96% / 98% 底色与单层 12px 模糊
- [ ] 无装饰渐变、彩色阴影、反光边缘或指针特效；减少透明度与高对比度可正常降级
- [ ] 320px 列内文件名优先于目录
- [ ] 动效 ≤ 300ms，且尊重 reduced-motion
- [ ] 状态色配有文字或图标
- [ ] 与既有组件同位置、同文案结构

验证入口：`bun test tests`（设计令牌、共享材质与组件行为）、`bun run audit:contrast`（主题与预设的实际文本背景）、`bun run audit`（布局、浮层与其他浏览器检查）。浮层材质的像素审计入口为 `e2e/glass-pixels.cjs`；测试名称保留兼容命名，验收规则以当前脚本为准。旧版本的测试项数和截图结论不能替代本轮运行。

---

## 9. 贡献流程

1. **提议**：说明使用场景、对比度验证结果、现有令牌为何不适用。
2. **评审**：命名符合 `--ug-<类别>-<角色>`；必须同时给出深浅两套值。
3. **发布**：更新 `tokens.css` 与 `index.html` 示例；记录版本；破坏性改动升主版本号。

### 版本记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.4.0 | 2026-10-31 | 中性 Workbench：实体 chrome/内容表面、96%/98% 浮层、12px 单层模糊；去除光晕与指针特效，统一圆角/阴影及深浅语法配色 |
| v1.2.2 | 2026-10-19 | 补充 §7.1「中性文本与表面的组合边界」：`text-ink-muted` 只适用于 `bg-canvas` / `bg-surface`，放到 `bg-raised` 上实测深色 4.08:1、浅色 3.90:1，需升为 `text-ink-secondary`（4.95 / 5.48）|
| v1.2.1 | 2026-10-19 | 毛玻璃弹窗面板改用 `color-mix(var(--ug-bg-surface) 94%)` + 浅色阴影，修掉浅色主题下「深色外壳 + 浅色内容」的割裂；`!important` 边框改为 `--ug-border-default` |
| v1.2.0 | 2026-10-18 | 181 令牌。色相补全五阶（`-bg-strong` / `-strong` / `-strong-hover`）；实心悬停改为「变深」；确立 ghost→filled 色调按钮；新增 `text-inverse`；第二批迁移把彩色工具类从 `index.html` + `src/main.ts` 中清零，并删除全部色相 `html.light` 覆盖；132 项色阶实测全部 ≥ 4.5:1 |
| v1.1.0 | 2026-10-17 | 新增 `hover-strong` 令牌；应用接入 Tailwind `@theme`，完成表面 / 边框 / 中性文本迁移 |
| v1.0.0 | 2026-10-17 | 首个完整规范：154 令牌、双主题、13 组件、对比度实测 |
| v0.9.0 | 2026-10-17 | 从现有界面抽取表面层级、语义色与提交类型色 |
