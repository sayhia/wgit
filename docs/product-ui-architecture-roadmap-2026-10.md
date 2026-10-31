# wgit 产品 / UI / 架构改造方案（2026-10-29）

> 顾问产出，基于代码现状实测（非假设）。范围：功能拓展四象限、工作区页 UI 优化、面向 AI 辅助 / 团队协作评审 / 引擎资产工作流的架构预留、分阶段排期与风险。
> 约束：不换技术栈（Wails3 + Go + Vue 3 + TS + Tailwind v4），不擅改现有架构。

## 0. 现状盘点（方案的事实基础）

| 维度 | 现状 | 对方案的影响 |
| --- | --- | --- |
| 信息架构 | `App.vue`（1169 行）单页 6 Tab：workspace / history / branches / stashes / tools / review；无 vue-router | 新功能以「Tab + 子 Tab + 命令面板」接入，不引路由库 |
| 全局入口 | HeaderBar（仓库/分支/会话菜单）、RepoRail、ActivityDock、CommandPalette、ShortcutCheatSheet | 入口上浮的载体已齐备，缺的是「注册机制」而非新组件 |
| 工具页 | `ToolsView` 12 个异步子 Tab（reflog 默认、clean、remotes、worktrees、LFS、submodules、sparse、disk、maintenance、studio、engine、terminal） | LFS/Worktree/Engine 等高价值能力埋两层，「入口深」的主要来源 |
| AI 基础 | `frontend/src/services/localAiService.ts`（配置/测试连接/`requestConflictAdvice`）+ `workspace/AiConflictAdvisor.vue`；前端直连用户配置的本地 AI 端点 | AI 已有雏形但通道在前端：密钥管理、脱敏、审计、多功能复用都需要后端化 |
| 协作基础 | `ReviewView.vue` + `components/pr/`、TAPD 绑定测试（`git_tapd_bind_test.go`）、Studio webhook | 评审流有骨架，缺 Provider 抽象 |
| 资产/引擎基础 | `git_engine_asset.go`（资产审计/引用扫描）、`git_engine_art_qc.go`、DCC 锁（`git_lfs_lock.go`）、`EngineTab.vue` + 3 个 Section | 引擎资产工作流数据层最全，缺看板/审批流 |
| 后端服务 | 唯一注册 `GitService`（5163 行、约 200 方法）；`config.go` 仅 `{RecentRepos, LastRepo, Theme}`；server 构建模式（`-tags server`） | 服务层必须停止向 GitService 堆功能；配置层需版本化扩展 |
| 性能现状 | i18n 4,242 keys：zh-CN 静态打入首屏（约 564KB chunk），en 懒加载；vite `manualChunks` 仅按 locale 拆 | 首屏负载的最大单项是 zh-CN 全量字典 |
| 质量基线 | Go 405 测试 / 77.0% 覆盖；前端 597 测试；`docs/optimization-rounds-2026-10.md` 记录 3 轮优化在途 | 排期需与 20 轮优化战役合并执行 |

---

## 1. 功能拓展清单与四象限优先级

四象限按「用户价值 × 实现成本」划分；编号 F1–F9 供排期引用。

### 第一象限：高价值 × 低成本 —— 立即做（阶段 1）

| # | 功能 | 复用的现有基础 | 落点 |
| --- | --- | --- | --- |
| F1 | **AI 提交信息生成**：CommitComposer 内「生成信息」按钮，取 staged diff → AI 生成符合仓库惯用风格的 message（可编辑后提交） | `localAiService` 通道、`CommitComposer.vue`、后端已有 staged diff RPC | workspace 右栏提交区 |
| F2 | **AI 变更摘要**：HistoryView 提交检查器内「摘要此提交」，单提交 diff → 三行以内摘要，写入检查器头部 | 同上 + history 检查器 | `components/history/` |
| F3 | **工具入口上浮**：LFS / Worktree / Engine / Maintenance 四个高频子 Tab 提升为 CommandPalette 一级命令 + ActivityDock 常用位；ToolsView 保留全量 | CommandPalette、ActivityDock、`TOOLS_SUB_TABS` 数组加 `pinned` 字段 | `ToolsView.vue`、`CommandPalette.vue` |
| F4 | **仓库健康徽标**：RepoRail 每个仓库卡片显示 `git_health` 已有评分的迷你徽标（红/黄/绿），点击直达修复中心 | `git_health.go`、`RepoRail.vue`、`FixCenterSection.vue` | RepoRail |

### 第二象限：高价值 × 高成本 —— 规划做（阶段 2–3）

| # | 功能 | 复用的现有基础 | 缺口 |
| --- | --- | --- | --- |
| F5 | **AI 冲突解决升级**：现有 `AiConflictAdvisor` 增加「整仓冲突批次建议 + 风险标注」，与 `SemanticConflictModal` 联动 | AiConflictAdvisor、conflict hunk 后端 | 后端 AI 代理（见 §3.5）、批量任务队列 |
| F6 | **团队评审流**：ReviewView 升级为 Provider 化评审中心 —— TAPD 适配器先行（已有绑定测试），PR/MR 列表、评审状态、一键拉取评审分支 | `ReviewView.vue`、`components/pr/`、TAPD 绑定、Studio webhook | Provider 抽象接口 + 适配器注册表（见 §3.6） |
| F7 | **引擎资产工作流看板**：资产审批状态（锁定人/审批人）、资产预算超限告警、美术 QC 结果汇总 | `git_engine_asset.go`、`git_engine_art_qc.go`、DCC 锁、`EngineTab.vue` 的 TotalSizeStr/预算数据 | 看板组件 + 审批状态机（见 §3.7） |

### 第三象限：低成本 × 低价值 —— 顺手做（随轮次搭车）

- F8 命令面板覆盖度补全：把 Settings、ReleaseWizard、ShortcutCheatSheet 等既有能力全部注册为命令（纯注册工作）。
- F8b WorkspaceView 空状态引导：未打开仓库时直接给「打开/克隆/初始化」三卡（RepoOnboarding 已有，补入口一致性）。

### 第四象限：高成本 × 低价值 —— 明确不做

- 内置代码编辑器 / 完整 IDE 化（与 DiffViewer 定位冲突，维护成本极高）。
- 自实现 Git 传输协议（libgit2/git CLI 已覆盖）。
- 云端账号体系与团队空间（当前无服务端商业化信号；server 模式保留扩展点即可，见 §3.4）。

---

## 2. UI 布局优化建议（目标：工作区页 WorkspaceView + 全局导航）

### 2.1 WorkspaceView（116KB，日常主战场）

现状：左栏变更/源文件树，右栏 `activeRightTab`（diff / source）双态，提交区在右栏上方；顶部有 ConflictBanner 与 FlightRecorderHud。

**信息层级（改什么）**
1. 提交区前置固定：`CommitComposer` 常驻右栏顶部不折叠；「暂存全部 → 填信息 → 提交」是全应用第一高频路径，任何 tab 切换不得把它挤出视口。建议右栏改为「提交栏（固定高度，可收起至 1 行摘要）+ 内容区（diff/source）」两段式。
2. 冲突态接管层级：进入冲突时 `ConflictBanner` 升为右栏第一位并带「AI 解决」主按钮（AiConflictAdvisor 已有，缺显式入口按钮），普通 diff 工具栏降级。
3. 变更列表分组默认态：已暂存组默认折叠（只留计数徽标），未暂存/未跟踪展开 —— 准备提交时视线只需要未暂存组。`ChangesList.vue` 加组级 `collapsed` 状态并持久化到 sessionState。
4. AI 操作统一挂点：生成提交信息（F1）、解释此 diff、建议拆分提交，三个动作收进 CommitComposer 的溢出菜单（Sparkles 图标触发），避免工具栏再膨胀。

**间距与密度（怎么改）**
5. 统一三档间距 token：区块间 12px、控件间 6px、行内 4px（现有 Tailwind 类随机使用 gap-1/gap-2/gap-1.5，在 `styles/` 定义语义别名后逐段替换，不做一次性大改）。
6. diff 行高与字号保持 12px mono/leading-5 不动（这是做对了的），只把右栏工具栏两行（筛选行 + 操作行）在 <1440px 时合并为一行，低频操作（外部 diff、blame）入溢出菜单。

**高频操作前置（哪些动作升级）**
7. 右栏操作行排序按使用频率重排：`暂存`、`丢弃`、`diff 开关` 留在一级；`外部 diff`、`外部 merge`、`blame`、`时间机器` 收入 `⋯` 溢出菜单（都有现成 handler，纯移动）。
8. 键盘路径兜底：`Cmd+K` 命令面板 + 已有 `j/k` 导航之外，补 `Cmd+Enter` = 提交（在 composer 聚焦时）、`Cmd+Shift+S` = 暂存全部，并注册进 ShortcutCheatSheetModal。

**响应式（窗口变小时）**
9. <1280px：左栏（变更树）改为可收起侧栏（手风琴），右栏占满；<1024px：diff 与变更列表改为上下堆叠 + 顶部分段切换，避免双栏挤压出现横向滚动。命令面板作为小窗口下的操作兜底（F8 依赖此点，需先做）。

### 2.2 全局导航（治「功能多但入口深」）

10. `ToolsView` 12 个子 Tab 改两级：一级只留 Reflog、LFS、Worktree、Engine、Terminal（数据佐证的高频）；其余进「更多工具 ▾」下拉（`TOOLS_SUB_TABS` 加 `tier: 1 | 2` 字段实现，不改组件结构）。
11. `CommandPalette` 成为唯一完整操作索引：F3/F8 将所有 Tab、Modal、工具动作注册进命令表（新增 `src/commands/registry.ts`，单一数组 + 分组），新功能接入 = 注册一行，从机制上终结「入口深」。
12. `ActivityDock` 增加可点击结果：后台任务（fetch/lfs prune/maintenance）完成后，点击跳到对应 Tab 并高亮结果（现有事件总线已支持 `wgit:*` CustomEvent，复用 App.vue 的 deep-link 分发模式）。

---

## 3. 架构设计（为 AI / 协作 / 资产预留）

### 3.1 服务边界：冻结 GitService，域服务并列注册

`main.go:39` 现在只注册 `GitService`。新增能力一律新建 Service 并列注册，GitService 只减不增：

```go
Services: []application.Service{
    application.NewService(&GitService{}),   // 冻结：仅修 bug 与已承诺功能
    application.NewService(&AiService{}),    // F1/F2/F5：AI 统一代理
    application.NewService(&CollabService{}),// F6：评审 Provider
    application.NewService(&AssetService{}), // F7：资产工作流（先从 git_engine_asset 收拢）
},
```

前端对应新增 `services/aiService.ts`、`collabService.ts`、`assetService.ts`（走既有 `callBackend` 模式）。这样 server 模式与桌面模式天然共享同一套域服务。

### 3.2 配置化：版本化 AppConfig

`config.go` 的 `{RecentRepos, LastRepo, Theme}` 扩展为分节 + schemaVersion，向后兼容读取：

```go
type AppConfig struct {
    SchemaVersion int          `json:"schemaVersion"` // 1
    RecentRepos   []string     `json:"recentRepos"`
    LastRepo      string       `json:"lastRepo"`
    Theme         string       `json:"theme"`
    Flags         FeatureFlags `json:"flags"`     // §3.3
    AI            AIConfig     `json:"ai"`        // 端点/模型/密钥引用，密钥进 credential vault（已有 git_credential_vault 基础）
    Collab        CollabConfig `json:"collab"`    // provider 选择 + 凭据引用
}
```

读取时 `schemaVersion < 1` 走迁移函数补默认值；每个 section 独立 `*Config` 结构，新功能 = 新 section，不碰旧字段。

### 3.3 Feature Flag

- 定义单一来源：Go 侧 `FeatureFlags{ AiCommitMsg, AiSummary, AiConflictBatch, ReviewFlow, AssetBoard bool }`，默认值随版本收紧/放开。
- 前端 `composables/useFeatureFlags.ts`：启动时 `GetConfig()` 一次 + 响应式 ref；UI 按 flag 渲染为 **disabled + 提示** 而非隐藏（保住可发现性，也方便测试）。
- 所有新 UI 块包在 `<FeatureGate flag="aiCommitMsg">` 薄组件内；flag 关闭时组件不渲染，控制首屏回归面。
- server 模式预留：flag 可被 `WGIT_FLAG_*` 环境变量覆盖（一行 `os.Getenv` 覆盖逻辑），为将来托管部署免改代码。

### 3.4 权限模型（server 模式预留，桌面不阻塞）

- 桌面单用户 = 隐式 `admin`，代码路径零改动。
- 预留三层角色：`viewer`（只读面板）/ `operator`（日常 git 操作）/ `admin`（配置、hooks、maintenance）。
- 落点在 service 方法入口：`func (s *AiService) GenerateCommitMessage(...)` 内先过 `s.checkRole(ctx, RoleOperator)`；桌面模式该检查恒通过。
- 前端只做能力展示（capability 来自 `GetConfig()` 返回的 `effectiveRole`），**不做前端鉴权**——权限唯一裁决点在后端，前端 disabled 仅是体验。

### 3.5 AI 通道统一后端化（F1/F2/F5 共用）

现状 `localAiService.ts` 在前端直连端点：密钥暴露在渲染进程、无脱敏、无审计、三处 AI 功能会各自为政。改造：

```
前端 aiService.ts ──RPC──> Go AiService
                            ├─ 密钥从 credential vault 读取（不落 config.json 明文）
                            ├─ 出站前 maskCredentialURL / secret-scan 脱敏 diff 载荷（复用 git_exec.go 既有函数）
                            ├─ 统一超时/重试/取消（context）
                            ├─ 每次调用写 oplog（git_oplog 已有）：功能、模型、耗时、token 估算
                            └─ prompt 模板放 resources/prompts/*.md（配置化，可热改）
```

`localAiService.ts` 保留为该服务的薄封装（迁移期兼容 AiConflictAdvisor 现有调用）。

### 3.6 协作评审 Provider 抽象（F6）

```go
// git_review_provider.go
type ReviewProvider interface {
    Name() string
    ListOpenReviews(repo string) ([]ReviewSummary, error)
    FetchReviewBranch(repo string, id string) error
    CommentOnReview(repo string, id string, body string) error
}
var reviewProviders = map[string]ReviewProvider{} // "tapd"、"gitlab"、…，适配器注册表
```

- TAPD 适配器基于既有绑定实现；GitLab/GitHub 后续按同接口插入，CollabService 只面向接口。
- 评审凭据走 credential vault；webhook 入站复用 Studio webhook 的鉴权模式。

### 3.7 资产工作流状态机（F7）

- `AssetService` 收拢 `git_engine_asset.go` 的扫描结果 + DCC 锁状态，输出统一 `AssetItem{path, state: locked|review|approved|oversized, owner, size, budgetRatio}`。
- 审批状态机先做最小闭环：`locked → (QC 通过) → approved`，落库到仓库内 `.wgit/assets.json`（与 subtree 元数据同模式，git_subtree.go:31 的 `subtreeMetadataPath` 是现成参照），不引入数据库。
- 前端 `AssetBoard` 组件挂 ToolsView→Engine 子 Tab 顶部，数据轮询走 ActivityDock 既有后台任务机制。

### 3.8 性能预留（配合优化战役）

- i18n 按域拆分：`messages/zh-CN.ts` 拆为 `zh-CN/{app,history,branches,tools,review,engine}.ts`，vite `manualChunks` 已有现成模式，index.ts 按当前 Tab 懒加载对应域；i18n 契约测试（key 对齐）继续兜底。
- 新页面/重组件一律 `defineAsyncComponent`（ToolsView 已验证此模式）。

---

## 4. 分阶段实施排期（与 20 轮优化战役合并执行）

优化战役（`docs/optimization-rounds-2026-10.md`，第 4–20 轮在续）与功能排期共用同一批并行任务槽，避免两套节奏互相打断：

| 阶段 | 轮次 | 内容 | 交付判据 |
| --- | --- | --- | --- |
| 阶段 0：还债 + 地基 | 第 4–8 轮 | 吞错清理（63 Go / 31 前端 backlog）、`GetCommitDetailsBatch` P1 修复、i18n 按域拆分、F3+F8 命令注册表、F4 健康徽标；§3.2/3.3 配置与 flag 基建 | 每轮构建/测试全绿；命令面板可直达全部 Tab；首屏 i18n chunk < 200KB |
| 阶段 1：AI 上线 | 第 9–12 轮 | §3.5 AiService 后端代理、F1 提交信息生成、F2 变更摘要、F5 冲突批次建议（flag 灰度） | AI 功能默认开启但可 flag 关闭；密钥不出 Go 进程；oplog 有审计记录 |
| 阶段 2：评审流 | 第 13–16 轮 | §3.6 Provider 接口 + TAPD 适配器、ReviewView 升级、§3.4 角色检查接入 | TAPD 评审列表/拉取分支可用；接口有 mock 适配器测试 |
| 阶段 3：资产看板 | 第 17–20 轮 | §3.7 AssetService + AssetBoard 最小闭环、UI §2 整改收尾、文档（README/架构审计）终稿 | 资产状态机测试覆盖；20 轮总结报告含分级统计 |

每阶段结束做一次全量验证（Go build server+desktop / vet / test -race，前端 vue-tsc / bun test / 生产构建），不过不进入下一阶段。

### 风险清单

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| Wails 绑定生成滞后（新 Service 需重新生成 `frontend/bindings`） | 前端编译失败 | 每个 agent 任务内含 `wails3 generate` 步骤验收；CI 顺序固定为先生成后 typecheck |
| 大文件拆分（WorkspaceView 116KB、git_studio_engine 10.6k 行）引发回归 | 已修 bug 复发 | 只在测试覆盖的函数级抽离；每轮 `-race -shuffle` 全量测试；不做无方案搬家 |
| i18n 按域拆分漏 key | 界面出现裸 key | 契约测试先行（key 对齐断言），拆分后 597 前端测试必须全绿 |
| AI 功能依赖用户自备端点，体验不可控 | 差评归因到产品 | 全部 AI 入口走 flag；无配置时入口显示「配置本地 AI」引导而非报错 |
| server/桌面双模式行为分叉 | 功能在 server 缺失 | 域服务从第一天就两模式共用；server 构建纳入每轮验证（已是现状） |
| 三线并行（优化轮次 / AI / 评审）任务互踩文件 | 合并冲突 | 沿用现役约束：每轮任务文件不冲突 + 单一职责 + 4 agent 上限 |

---

## 5. 与在途工作的衔接

- 本方案不改变 20 轮优化的验收标准（构建/测试全绿才算完成）；第 4 轮已完成的审计结论（git_subtree 拆分干净、`git_service.go:3816` GetCommitDetailsBatch 吞错 P1、三套 format 函数待统一）直接并入阶段 0 任务池。
- 排期表中的轮次是「建议映射」而非硬边界：若某轮问题提前清零，按既定规则转向下一维度，功能任务自动顶替空槽。
