# 设置页整页改版 · 设计文档

- 日期：2026-10-31
- 范围：前端设置页 IA、布局、组件拆分与样式统一；AI 设置深度重构
- 不改：后端 RPC、config schema、SQLite、权限、登录、密钥存储

## 1. 背景

`SettingsView.vue` 目前是“左侧 11 个平铺分类 + 右侧内容”，右侧内容被 `max-w-3xl mx-auto` 限制宽度。AI 设置又在右侧内部再套一层导航与折叠：

- `LocalAiSection.vue` 1023 行：快速配置、手动连接表单、旧配置迁移、高级 5 卡、5 个灰度开关、用量卡共用一层。
- `AiQuickSetup.vue` 740 行、`AiControlPlaneSection.vue` 801 行、`AiUsageCard.vue` 241 行。
- 连接配置存在两套编辑器；AI 内部导航与设置页主导航形成双层导航。
- 其他 10 个分区各自实现标题、卡片、间距与错误横幅，视觉不一致。

## 2. 目标与非目标

**目标**

1. 设置页按任务分组，AI 三个子项进入主导航，取消右侧嵌套导航。
2. 右侧内容满宽（移除 `max-w-3xl`），桌面/窄屏均无横向溢出。
3. 所有 11 个原分类与全部 AI 能力保留，无功能回退。
4. 统一分区标题、卡片、间距、按钮与状态样式；非 AI 分区不重写业务逻辑。
5. 搜索、命令面板与 `wgit:settings-section` 深链继续可用，并覆盖 AI 三个叶子。
6. AI 配置合并为一套连接编辑器；档案、密钥、路由、可靠性、用量、诊断、导入导出、旧迁移、灰度开关全部保留。

**非目标**

- 不改后端接口、配置字段、数据库与密钥存储。
- 不删除任何现有设置能力。
- 不在本轮引入新的 UI 框架或依赖。
- 不重写非 AI 分区的业务逻辑、RPC 调用或校验规则。

## 3. 信息架构

| 分组 | 叶子 id | 标题 | 说明 |
| --- | --- | --- | --- |
| 通用 | `appearance` | 外观 | 原样 |
| 通用 | `identity` | 身份 | 原样 |
| 通用 | `shortcuts` | 快捷键 | 从平铺分类移入通用组 |
| 代码与 Git | `signing` | 签名 | 原样 |
| 代码与 Git | `tools` | 外部工具 | 原样 |
| 代码与 Git | `aliases` | 别名 | 原样 |
| 代码与 Git | `hooks` | Git Hooks | 原样 |
| AI | `ai-connection` | 连接 | 编辑器 + 档案 + 多源密钥 + 旧迁移 |
| AI | `ai-behavior` | 行为 | 提交偏好 + 任务路由 + 可靠性 + 灰度 |
| AI | `ai-insights` | 用量与诊断 | 用量 + 诊断 + 导入导出 |
| 系统 | `proxy` | 代理 | 原样 |
| 系统 | `config` | 高级配置 | 原样 |
| 系统 | `about` | 关于 | 原样 |

兼容规则：

- 原 10 个叶子 id 不变。
- 旧 `localai` 导航/深链/命令面板入口映射到 `ai-connection`；`SettingsCategory` 接受 `localai`，但 `normalizeSettingsLeaf()` 统一规范化，`localai` 不再作为独立叶子渲染。
- `wgit:settings-section` 的 detail 仍接受原 id；映射表集中在 `SettingsView`。
- 分组标签使用新 i18n key；叶子标题复用现有 category key（AI 三个叶子新增 key）。
- 分组结构集中定义：`general`（通用）、`git`（代码与 Git）、`ai`（AI）、`system`（系统）；每项含 `id`、`labelKey`、`leaves[]`，导航与测试共用同一份数据。
- 设置搜索同时匹配叶子标题、分组标题与条目索引；条目索引的 `category` 必须使用新叶子 id，原 `localai` 条目拆到三个 AI 叶子。

## 4. 布局与响应式

**桌面（≥900px）**

- 左侧 240px 粘性导航：标题、搜索框、分组标签、叶子按钮。
- 右侧 `flex-1 min-w-0` 满宽内容，内边距 24/32px；不再居中限宽。
- 右侧顶部只渲染一次当前叶子的标题与说明；分区组件不再重复 `<h2>` + intro（保留卡片内部小标题）。

**窄屏（<900px）**

- 左侧变为顶部横向滚动的分组导航；搜索框保留在导航首行。
- 内容仍满宽，`min-w-0` 防止长路径/URL 撑破布局。
- 不引入抽屉/二级页面，保持单页切换模型。

**内容栅格**

- 长表单保持单列；短字段组（如身份名称/邮箱、超时/重试）在 ≥1280px 使用两列网格。
- 卡片宽度随内容区满宽，不设置最大宽度。

## 5. 组件架构

**外壳**

- `SettingsView.vue`：只负责导航状态、叶子映射、搜索索引、内容分发；从 405 行继续收敛。
- 新增 `components/settings/SettingsNav.vue`：分组渲染、搜索、历史记录、键盘与 `aria-current`；分组/叶子来自同一份 `SETTINGS_GROUPS` 常量。
- 头部渲染时序：批次 1 中 AI 三叶子使用 `SettingsPageHeader`；原 10 个分区暂时保留各自 `<h2>` + intro，避免重复。批次 3 删除这些旧头部后，统一由外壳渲染标题与说明。每个叶子在 `SETTINGS_CATEGORY_META` 标记 `header: "shell" | "section"`，批次 3 全部切到 `shell`。
- 新增 `components/settings/SettingsSearch.vue`：从 `SettingsView` 抽出搜索输入、结果列表、历史与高亮。
- 新增 `components/settings/SettingsPageHeader.vue`：叶子标题 + 说明（+ 可选右侧动作）。
- 新增 `components/settings/SettingsCard.vue`：统一 `surface + border + radius + spacing`；供新 AI 分区与后续渐进迁移使用。
- `utils/settingsIndex.ts`：新增分组、AI 三叶子与条目；原条目重新归属 AI 叶子。
- `commands/registry.ts`：保留原设置命令；如新增 AI 三叶子入口，仅追加事件，不删除旧 id。

**AI 分区**

- `AiConnectionSection.vue`：连接编辑器（`AiConnectionForm.vue`）+ `AiProfilesCard.vue` + `AiKeysCard.vue` + 旧配置迁移横幅。
- `AiBehaviorSection.vue`：提交偏好 + `AiTaskRoutingCard.vue` + `AiReliabilityCard.vue` + 灰度开关。
- `AiInsightsSection.vue`：`AiUsageCard.vue` + `AiDiagnosticsCard.vue` + `AiTransferActions.vue`。
- `AiConnectionForm.vue`：由 `AiQuickSetup.vue` 演化，合并手动连接表单独有字段；`compact` 模式供 `AiQuickSetupModal.vue` 复用。
- 删除 `AiControlPlaneSection.vue`、`AiQuickSetup.vue`、`LocalAiSection.vue`（在对应测试迁移完成后）。

**连接编辑器字段分区**

- 基础：模式（API/CLI）、供应商预设、端点/模型或 CLI 命令、API Key、保存/测试。
- 高级连接（折叠）：协议/Azure 版本、超时、自定义请求头、CLI 预设/参数/输入方式、清空密钥。
- 旧配置迁移横幅仅在检测到旧 localStorage 配置时出现，导入/丢弃按钮保留。

**行为分区**

- 提交偏好：语言、是否包含正文、自定义指令。
- 任务路由：commit/summary/conflict/explain/chat/review/release 模型覆盖。
- 可靠性：重试次数、速率限制、fallback base/protocol/model。
- 灰度开关：aiCommitMsg、aiSummary、aiConflictBatch、reviewFlow、assetBoard（原样保留，含无消费方的 reviewFlow）。

**用量与诊断**

- 用量卡（预算/成本/未知 token/按仓库筛选）。
- 诊断卡（只读、本地检查）。
- 配置导出/导入与保存动作；导入仍拒绝密钥字段。

## 6. 数据流与状态

- 不新增全局 store；每个叶子独立加载，避免一个请求失败拖垮整页。
- `SettingsView` 持有 `activeLeaf`、连接变更计数 `connectionRefreshKey`、用量刷新计数 `usageRefreshKey`。
- `AiConnectionSection` 保存后 `emit("changed")`；行为/用量叶子通过 refreshKey 重载。
- `AiBehaviorSection` 继续使用 `GetAIAdvancedConfig/SaveAIAdvancedConfig` 与 `GetAppFlags/SaveAppFlags`。
- `AiInsightsSection` 使用 `GetAIUsageSummary/GetAICallLogs/ClearAICallLogs` 与配置导入导出。
- 搜索索引只做导航，不触发 RPC。
- **保存语义按区隔离**：
  - 连接：`SaveAISettings`（含可选 Key 写入/清空），保存后刷新档案/密钥与依赖连接状态的卡。
  - 行为：`SaveAIAdvancedConfig`（提交偏好/路由/可靠性）与灰度开关的即时保存；两者互不触发对方的完整重载。
  - 用量与诊断：只读加载；导入/导出是独立动作，导入成功后才刷新配置与用量。
- 旧配置迁移只在连接区出现；导入/丢弃成功后刷新连接与档案，不强制跳转叶子。

## 7. 可访问性

- 导航使用 `<nav aria-label>`；分组用 `role="group"` + `aria-labelledby`；叶子按钮 `aria-current="page"`。
- 叶子切换后内容区 `role="region"` + `aria-labelledby` 指向标题；不强制抢焦点（搜索跳转除外）。
- 搜索保持现有 combobox 语义（`aria-activedescendant` 仅在有结果时存在）。
- 所有卡片标题层级统一为 `h3`/`h4`，不跳级。
- 现有焦点环、减少动效、高对比度/减少透明度行为保持不变。

## 8. 测试与验收

**单元/组件**

- `settingsShell.test.ts`：4 组 13 叶子渲染、`aria-current`、搜索结果跳转、旧 `localai` 深链映射、分组标题匹配。
- 既有分区测试逐项通过；AI 测试迁移到新组件，保留原 data-testid 或提供等价断言。
- `aiConfigDedup.test.ts` 更新：单一连接编辑器、单一档案管理、单一状态展示。
- `settingsIndex` 测试：所有叶子/条目都有 zh/en key，旧 id 可解析；AI 条目不再落在 `localai`。
- `normalizeSettingsLeaf("localai") === "ai-connection"`；未知 id 回退到 `appearance` 且不抛错。

**门禁**

- `bun test tests`、`bun run typecheck`、`bun run build`。
- 浏览器：smoke、contrast、responsive、popovers。
- 视觉：设置页基线更新（外观/身份/AI 三个叶子/窄屏）。
- 手工：1440 / 1024 / 768 / 390 四档检查导航、搜索、AI 保存/测试、导入导出、无横向溢出。

**验收标准**

- 13 个叶子全部可达，无空白页/控制台错误。
- 原 11 类设置能力与 AI 11 项能力逐项存在（见 §5）。
- 深链 `wgit:settings-section` 对原 id 与新 AI id 均生效。
- 设置搜索可命中 AI 连接/行为/用量条目并跳转。
- 无 backend/schema 改动。

## 9. 分批实施

1. **外壳**：SettingsNav/Search/PageHeader、分组 IA、满宽布局、深链与搜索兼容；现有 10 分区原样嵌入。
2. **AI 拆分**：连接/行为/用量三叶子，合并连接编辑器，拆档案/密钥/路由/可靠性/诊断/传输；删除旧 AI 外壳。
3. **样式统一**：10 个非 AI 分区迁移标题/卡片/间距/按钮/状态样式，不重写业务逻辑。
4. **验收与清理**：测试迁移、响应式/审计/视觉基线、删除废弃文件与死代码。

每批独立可回滚；旧组件在对应批次通过测试前保留。

## 10. 风险与回滚

- **深链/搜索回归**：集中映射 + 兼容测试；保留原 id。
- **AI 状态刷新**：连接保存后的 refreshKey 与 `changed` 事件需覆盖档案/密钥/用量；用行为测试锁定。
- **测试大迁移**：保留 data-testid，先迁移再删除旧组件。
- **视觉基线**：设置页截图会变化，按批次更新并人工核对，不盲目覆盖。
- **工作区并行改动**：本重构只动设置相关路径，不 reset、不 push；提交按批次确认。

## 11. 文件清单

**新增**

- `frontend/src/components/settings/SettingsNav.vue`
- `frontend/src/components/settings/SettingsSearch.vue`
- `frontend/src/components/settings/SettingsPageHeader.vue`
- `frontend/src/components/settings/SettingsCard.vue`
- `frontend/src/components/settings/AiConnectionSection.vue`
- `frontend/src/components/settings/AiConnectionForm.vue`
- `frontend/src/components/settings/AiBehaviorSection.vue`
- `frontend/src/components/settings/AiInsightsSection.vue`
- `frontend/src/components/settings/AiProfilesCard.vue`
- `frontend/src/components/settings/AiKeysCard.vue`
- `frontend/src/components/settings/AiTaskRoutingCard.vue`
- `frontend/src/components/settings/AiReliabilityCard.vue`
- `frontend/src/components/settings/AiDiagnosticsCard.vue`
- `frontend/src/components/settings/AiTransferActions.vue`

**修改**

- `frontend/src/components/SettingsView.vue`
- `frontend/src/components/settings/AiQuickSetupModal.vue`（改用 `AiConnectionForm`）
- `frontend/src/utils/settingsIndex.ts`
- `frontend/src/commands/registry.ts`（追加 `settings-ai-connection` / `settings-ai-behavior` / `settings-ai-insights`，保留原设置命令）
- `frontend/src/i18n/messages/zh-CN.ts`、`en-US.ts`（分组与 AI 叶子标题）
- 相关测试与 `e2e` 视觉基线

**删除**

- `frontend/src/components/settings/AiControlPlaneSection.vue`
- `frontend/src/components/settings/AiQuickSetup.vue`
- `frontend/src/components/settings/LocalAiSection.vue`

## 12. 已确认决策

- 整页改版；非 AI 分区只统一外壳与样式，不重写业务逻辑。
- 左侧分组导航 + 右侧满宽内容；AI 三子项进入主导航，取消右侧嵌套导航。
- 所有现有能力保留，包括旧配置迁移、多源密钥、导入导出与 5 个灰度开关。
- 不改后端 RPC、config schema、数据库与密钥存储。
