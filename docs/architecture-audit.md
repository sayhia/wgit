# wgit 架构盘点：RPC 可达性、绑定缺陷与拆分方案

> 盘点日期：2026-10-21
> 范围：main 包全部 Go 源码 + `frontend/src`（不含 `node_modules` / `dist`）
> 方法：静态扫描 Go 导出方法定义、`gitService.ts` 封装映射、`.vue` 内 `gitService.*` 调用点，交叉计算可达性；数字均可复现。
>
> **修复状态（2026-10-21）**：第 2 节的 4 个缺陷与第 3 节的错误降级守卫均已修复，并在 Wails server 模式下用真实仓库端到端验证；下方数字为修复后的复核值。
> **清理状态（2026-10-22）**：两轮清理后 RPC 315、UI 可达 315（100%）、后端专属 0；
> **增量（2026-10-23 · PR/MR）**：新增 8 个 PR/MR 评审 RPC（GetPullRequestProvider / SavePullRequestToken / ClearPullRequestToken / ListPullRequests / GetPullRequestDetail / CreatePullRequestComment / SubmitPullRequestReview / MergePullRequest），全部接入主视图「PR / MR 评审」，RPC 363、UI 可达 363（100%）。
> **增量（2026-10-23）**：新增 28 个闭环 RPC（TrackStagedLargeFilesAsLfs / UnlockStaleLfsLocks / GetRangeDiff / CreateFixupCommit / OptimizeObjectStore / GetMergeOverlapWarning / RunHealthFix / ScanFixCenter / ApplyAuditFix / RemediateStagedSecrets / GetAssetDiff / PreviewLfsMigration / ExecuteLfsMigration / VerifyTagSignature / AnalyzeAssetImpact / RunEngineBootstrap / GetOperationLog / UndoLastOperation / StartTerminal / WriteTerminal / ResizeTerminal / StopTerminal / GetTerminalStatus / RepairWorktrees / PruneWorktrees / BisectSkip / ReadGitHook / SaveGitHook），RPC 355、UI 可达 355（100%）；关键数字已按清理后结果更新，清理细节见第 4 节与 [dead-code-cleanup.md](dead-code-cleanup.md)。

## 1. 关键数字

| 指标 | 数量 | 说明 |
| --- | --- | --- |
| 后端导出 RPC 方法 | **363** | `git_service.go` 165 + `git_studio_engine.go` 44 + `git_lfs_lock.go` 25 + `git_engine_asset.go` 16 + `git_external_tools.go` 12 + `git_closed_loop.go` 7 + `git_pr.go` 8 + `git_fix_center.go` 3 + `git_asset_diff.go` 1 + `git_lfs_migrate.go` 2 + `其余 12 个文件 65 |
| 前端封装入口 | 363 | 全部映射正确 |
| UI 实际调用的后端方法 | **363** | 占 363 的 100% |
| 后端专属（UI 不可达） | **0** | 12 个重复 API 已于 2026-10-22 删除 |
| 封装了但 UI 从未调用 | 0 | 无未调用封装 |
| 已修复缺陷 | **6** | 4 个绑定/UI 行为 + 1 个 git 格式串 + 1 个伪实现（详见第 2 节） |
| Go 测试函数 | 332 | `go test .` 约 80s 全绿；语句覆盖率 75.0%，`-race` / `-shuffle` 通过 |
| 前端单元测试 | 114 | `bun test tests` 7 个文件全绿（新增 PR 补丁解析 / 状态元数据 / 确认对话框 18 条） |

## 2. 六个已修复缺陷（3 个绑定层 + 1 个 git 格式串 + 1 个代理清除行为 + 1 个伪实现）

| # | 位置 | 现象 | 影响 | 修复 |
| --- | --- | --- | --- | --- |
| 1 | `gitService.ts:953` `stashClear` | 调用的是不存在的 `"invoke"` 方法：`callBackend("invoke", "ExecGit", repoPath, "stash", "clear")` | **用户可见**。`StashView.vue:185`「清空储藏箱」：RPC 失败被 mock 降级吞掉（`default: return null`），UI 弹出「储藏箱已清空」成功提示，实际什么都没删 | ✅ 改为 `callBackend<void>("StashClear", repoPath)`（后端 `StashClear` 在 `git_service.go:3554`） |
| 2 | `RebaseModal.vue:150` `gitService.getRebaseCommits` | 封装层根本没有这个入口（Go 方法 `GetRebaseCommits` 在 `git_service.go:2014` 存在） | **用户可见**。调用抛 `TypeError` 被 catch，模板中「将在此基线上重放的提交 (N 条)」永远是 0 条，分支变基弹窗的计划列表形同虚设 | ✅ 在 `gitService.ts` 补入口（参照 `interactiveRebasePlan`） |
| 3 | `gitService.ts:1096` `openWebUrl` | 调用的 `"OpenWebUrl"` 与 Go 方法 `OpenWebURL`（`git_external_tools.go:569`）大小写不符 | 潜伏：当前 UI 未调用 `openWebUrl`；一旦调用，会静默走 mock 而不是打开浏览器 | ✅ 改为 `"OpenWebURL"` |
| 4 | `git_service.go:540` `GetStashes` | `git stash list --format=%gd%09%cr%09%gs` 中 `%09` 在 pretty format 里不是制表符（必须写 `%x09`），`strings.Split(line, "\t")` 永远只得到 1 段 | **用户可见**。储藏箱永远是「暂无储藏快照」；`StashView` 的全部操作因此不可达。注意 `for-each-ref` / `tag -l` 的 `%09` 是合法的，不要误改（`git_service.go:486`、`git_branch_insights.go:128`） | ✅ 改为 `%gd%x09%cr%x09%gs`，新增回归测试 `TestGetStashesParsesList` / `TestStashClearEmptiesList` |
| 5 | `ProxySection.vue` `saveProxy` | 输入为空时仍调用 `SetGitProxy`，而后端拒绝空 URL | **用户可见**。界面提示「代理已清除」，实际 http.proxy / https.proxy 仍保留 | ✅ 空值改调 `ClearGitProxy`，非空才 `SetGitProxy`；浏览器验证写入后清除，`git config` 确认为 unset |
| 6 | `git_studio_engine.go` `ArtistOneClickPipeline` | 原实现返回硬编码 `staged=2, locked=2, pushed=true`，实际未执行任何 Git 操作 | **严重误导**。UI 若接入会显示“已推送”但远端无提交 | ✅ 改为真实流程：暂存全部 → 统计贴图/网格 → `[Art]` 提交 → `push origin HEAD:<branch>`；新增 `TestArtistOneClickPipelineCommitsAndPushes` 验证远端提交 |

> 验证方式：`go build -tags server` 起隔离实例（独立 `HOME` + 临时仓库），在浏览器中实测——变基弹窗正确显示 2 条待重放提交；储藏列表显示 2 条并可清空，磁盘 `git stash list` 归零。
> 缺陷 2 当时能漏过 CI 的原因见第 6 节末尾：`typecheck` 用的是 `tsc`，`.vue` 文件被 `env.d.ts` 的 `declare module "*.vue"` 声明成 `any`，文件内部类型错误不可见；现已改用 `vue-tsc` 并清零首轮 41 个错误，同时移除了 `*.vue` 的 any-shim。

## 3. 已修复：RPC 错误降级守卫

修复前的 `callBackend` 对所有异常一律返回 mock（`default: return null`），导致真实 Git 失败被吞、失败被展示成成功。

修复后的策略（`gitService.ts`）：

- **后端错误一律上抛**：`RuntimeError`（方法返回 error）、`ReferenceError`（方法不存在）、`TypeError`（参数错误）直接 `throw`，由组件的 `catch` 展示。
- **仅 dev 预览降级**：只有 `import.meta.env.DEV` 且错误不属于上述类型（即 Vite 预览下 `POST /wails/runtime` 404 的传输错误）才使用 mock fixtures。
- **生产构建永不 mock**：`bun run build` 产物中 `import.meta.env.DEV === false`，即使后端不可达也会报错，不会伪造数据。

验证：

- Wails server 模式 + 坏 remote：点 Fetch 弹出 `Fetch 失败: fatal: '/tmp/wgit-missing-remote.git' does not appear to be a git repository ... (exit status 128)`，错误不再被吞。
- 纯 Vite 预览（`bun run dev`）：mock 仓库与 mock 变更正常展示，预览能力未受影响。

## 4. 后端专属能力清单（0 个）

2026-10-22 第二轮清理已删除最后 12 个「有测试覆盖但与现有 UI 能力重复」的底层命令：`AbortCherryPick`、`AbortMerge`、`AbortRebase`、`GetCommitsPaged`、`GetFileHistory`、`GetMergeToolConfig`、`MergeBranch`、`MergeBranchWithOptions`、`RebaseBranch`、`RebaseBranchWithOptions`、`SearchSourceCode`、`ShelveSelectedFiles`。

- 合并 / 变基 / 冲突测试已改由 UI 实际使用的 `MergeAdvanced`、`RebaseAdvanced`、`AbortConflictOperation` 覆盖；分页覆盖率由 `GetCommitsFiltered` 承接。
- 前端 `src/types.ts` 与 Go 侧对应的 `MergeToolConfig`、`SelectiveShelveResult`、`SourceSearchResult`、`AutoLockLeaseResult`、`LockLeaseItem` 类型一并删除。
- 当前 355 个 RPC 方法全部有 UI 入口，后端专属与未调用封装均为 0。清理后 `git_service.go` 为 4,321 行 / 165 方法。

## 5. `git_service.go` 拆分方案

### 现状

- 拆分前：15,381 行、289 个方法，占后端全部 RPC 的 85%。
- `type GitService struct{}`（`git_service.go:31`）是**无状态值接收器**：方法之间不共享私有字段，同包内文件拆分是纯机械搬移——不改变编译结果、不改变 RPC 名，风险极低（唯一约束：不能出现重名函数）。
- 拆分前文件内部已有天然分块：`7409-14811` 是 Round 101–140 波段（每轮一个 `// Round N:` 注释块）；`14812-15381` 是 Project Browser 模块。尾部分块只有 2 个包级类型声明（`parsedTable`、`offlineQueueStore`），没有包级变量，因此不存在 Go 文件初始化顺序风险。

### 分阶段执行

**Phase 1：搬尾部 ✅ 已完成（2026-10-21）**

| 目标文件 | 来源（拆分前行号） | 内容 |
| --- | --- | --- |
| `git_studio_engine.go` | `git_service.go:7409-14811` | Round 101–140：引擎审计、Unity/Unreal 检测、热修复、Studio CI/TAPD、凭据、合规（45 个方法，7,443 行） |
| `git_project_browser.go` | `git_service.go:14812-15381` | Project Browser 模块（8 个方法，587 行） |

执行结果：`git_service.go` 15,381 → **7,390 行 / 236 方法**。验证：`go build` 通过、42 个测试全绿；与原文件逐一比对，289 个 `GitService` 方法的方法名集合与 346 个函数总数完全一致。新文件按各自实际引用剪裁了 import。

**Phase 2：归类核心区散落方法 ✅ 已完成（2026-10-21）**

实际执行：按方法声明边界整块切出 50 个方法（`AuditIgnoreRules` 属仓库卫生，保留在核心），保持原顺序分到 3 个文件：

| 目标文件 | 方法数 / 行数 | 内容 |
| --- | --- | --- |
| `git_lfs_lock.go` | 25 / 991 | LFS 选择性拉取/推送、缓存与锁巡检、调优配置、可锁文件只读保护、Pre-push 校验 |
| `git_engine_asset.go` | 16 / 1,477 | Unity/Unreal 工程识别、`.meta` 同步修复、纹理/音频/网格/LOD 巡检、引擎 LFS 配置 |
| `git_studio_ops.go` | 9 / 415 | Studio CI 触发、团队活动、自定义流水线、凭据与暂存密钥巡检 |

执行结果：`git_service.go` 7,390 → **4,552 行 / 186 方法**；后续两轮清理后为 **4,321 行 / 165 方法**。验证：拆分时 289 个方法名集合、346 个函数总数与拆分前完全一致；`go build` + 42 个测试通过，`gofmt` / `go vet` 干净。核心区仍保留少量引擎/LFS 相关的类型与辅助函数（同包可见），不再强行搬移。

**Phase 3（可选）**：纯 Git 核心区仍可按领域微调（stash / tag / submodule / subtree），收益递减，建议随功能改动顺手进行。

### 验证（每个 Phase 完成后）

```bash
go build -o /tmp/wgit-split-check .   # 编译
go test .                             # 319 个测试函数（含 fixture 仓库用例）
```

再手工冒烟：打开仓库 → 状态/提交列表/Bisect/发布向导各点一遍（这些是尾部区块的相邻功能）。建议一次搬一个 Round 块并独立提交，便于二分定位。

## 6. 超大 Vue 组件拆分方案

现状（除 `index.html` 与 `tokens.css` 外没有共享组件层，6 个视图各自内联 Toast 模板）：

| 文件 | 行数 | 天然边界（现有模板注释） | 建议拆分 |
| --- | --- | --- | --- |
| `WorkspaceView.vue` ✅ | 3,042 → **1,336** | 左栏源码浏览 / 右栏变更与差分 / 源码全景 | 已拆出 SourceExplorer 655、SourcePanorama 290、DiffViewer 305、ChangesList 606、CommitComposer 81、ConflictBanner 72；父组件保留双栏布局、差分头部与跨组件状态 |
| `ToolsView.vue` ✅ | 2,540 → **312** | 10 个子页签 | 已拆为 `tools/` 下 8 个组件：Reflog 247 / Clean 139 / Remotes+Worktrees 695 / LFS 311 / Submodules 386 / Sparse 226 / Disk 264 / Maintenance 274；`BisectPanel`、`ReleaseWizard` 原样保留 |
| `HistoryView.vue` ✅ | 2,710 → **1,895** | DAG 主列 / 筛选栏 / 批量工具栏 / 检视器 | 已拆出 HistoryFilterBar 84 / BatchActionToolbar 53 / CommitInspector 426 / CommitGraphList 485（虚拟滚动、DAG 车道渲染、多选/右键事件、加载更多）；父组件保留筛选状态、多选逻辑与快捷键 |
| `BranchesView.vue` ✅ | 1,830 → **966** | 4 个列表 + 对比视图 + 6 个弹窗 | 已拆为 `branches/` 下 5 个视图组件（Local 310 / Remote 77 / Tags 72 / Stashes 80 / Compare 174）+ `branches/modals/` 6 个弹窗；列表通过 props/emits 与父组件交互，弹窗使用 `v-model:*` |
| `ReviewView.vue` ✅ | 1,334 → **722** | 3 栏 + 导出弹窗 | 已拆为 `review/` 下 SessionList 134 / ChangeList 120 / FileDetailPanel 427（含 diff 解析与行级评论）/ ExportModal 50；`review/meta.ts` 统一文件状态与 verdict 元信息 |
| `SettingsView.vue` ✅ | 726 → **106** | 6 个分区 | 已拆为 `settings/` 下 6 个组件：Identity 103 / Signing 107 / Proxy 86 / Storage 56 / Tools 249 / Shortcuts 42；各分区按需加载自己的配置，Toast 由父组件统一展示 |

横向复用建议：

- `components/ui/`：`Toast`（现 6 处重复）、`Modal`、`EmptyState`、`Badge`。
- `composables/`：`useToast`、`useResizablePane`（WorkspaceView 已有实现）、`useVirtualList`（HistoryView 已有窗口化逻辑，ToolsView 大列表可复用）、`useRepoData`（统一的加载/错误/刷新）。
- **ToolsView 拆分机制**：父组件保留页签导航与唯一 Toast；子组件通过 `refreshNonce` prop 响应头部刷新、通过 `toolsStatus`（`composables/toolsStatus.ts`）上报 loading/count 供头部 spinner 与页签徽标使用；子组件的 `notify` 以事件上抛给父组件，避免 8 份 Toast 模板。各页签弹窗（Reflog 建分支、远程/工作树、子模块、LFS 锁定）已随对应子组件迁移。
- **`vue-tsc` 已接入（2026-10-21）**：`typecheck` / `build` 均改用 `vue-tsc --noEmit`，`.vue` 内部类型进入门禁。首轮暴露 41 个错误并已全部修复，其中真实运行时缺陷包括：`RebaseModal` 读取 `CurrentStep/TotalSteps/Onto/CurrentCommit/ConflictedFiles`（实际字段为 camelCase，进度与冲突信息一直为空）、`HistoryView` 调用不存在的 `fetchCommitDetail`、`SyncModal` 对 `void` 返回值做真值判断、`BranchesView` 读取不存在的 `stash.branch`；同时删除了未使用的模板组件 `HelloWorld.vue`，并移除了 `env.d.ts` 中把 `*.vue` 声明为 `any` 的 shim（vue-tsc 不需要它，移除后父子组件 props 才会真正参与检查）。

## 7. 建议执行顺序

1. ~~修 3 个绑定缺陷~~ ✅ **已完成**（另发现并修复 `git stash list --format=%09` 解析缺陷，共 4 项）；`callBackend` 环境守卫 ✅ **已完成**。
2. ~~引入 `vue-tsc`~~ ✅ **已完成**（`bun add -d vue-tsc`，`typecheck` / `build` 已切换；首轮 41 个错误清零）。
3. ~~`git_service.go` Phase 1 搬移~~ ✅ **已完成**（15,381 → 7,390 行，289 → 236 方法，构建与 42 个测试通过）。
4. **前端组件拆分** ✅ **全部完成**：ToolsView（2,540 → 312，8 个子组件）、SettingsView（726 → 106，6 个）、BranchesView（1,830 → 966，11 个）、ReviewView（1,334 → 722，4 个）、HistoryView（2,710 → 1,895，4 个）、WorkspaceView（3,042 → 1,336，6 个）。六个超大组件均已收敛到 2,000 行以内。

5. **195 个后端专属能力** ✅ **已决策**：保留为后端 API 并写入 README 第 13 节（按引擎/资产、核心 Git、系统集成三类标注），暂不删除、暂不批量接入 UI；后续按业务优先级逐类接 UI。累计已接入 171 个高价值入口（见第 8 节），当前后端专属为 0 个（最后 12 个重复 API 已于 2026-10-22 删除）。

## 8. 后续优化记录（2026-10-21）

- **签名分区回读**：新增 `getCommitSigningStatus` 封装（并补齐 `CommitSigningResult` 类型），Settings → Signing 打开即回填 `commit.gpgsign` / `gpg.format` / `user.signingkey`；此前只保存不回读。签名分区同时展示检测到的 SSH 公钥，未安装 `gpg` 时给出显式提示。
- **Toast 统一**：新增 `components/ui/Toast.vue` + `composables/useToast.ts`，按设计系统规范实现（右下角、2.4s、语义色点、`shadow-toast` 令牌），替换 Workspace / Branches / Stash / Settings / Tools 五处重复实现；History / Review 的操作反馈也已迁移到共享 Toast。
- **7 个后端能力接入 UI**（后端专属 193 → 186）：
  - `GetGitVersion` → Settings → 快捷键与架构信息显示当前 Git 版本；
  - `GetCommitSigningStatus` → Signing 分区回读并展示可用 SSH 公钥；
  - `LfsPull` / `LfsPush` → 工具箱 → Git LFS 头部「LFS 拉取 / 推送」（推送带确认）；
  - `ScanStagedSecrets` → 工具箱 → 工作区清理「暂存区敏感信息扫描」卡片（严重度 / 规则 / 脱敏片段）；
  - `DeleteReviewComment` → 评审工作区行级评论删除按钮；
  - `ExportCommitReport` → 提交历史筛选栏「导出报告」（Markdown，剪贴板写入失败时回退到 `textarea` + `execCommand`）。
- **回归验证**：`vue-tsc` 0 错误、`bun run build` 通过、`go test .` 全绿；浏览器验证 Git 版本、签名密钥提示、LFS 拉取 Toast、敏感信息扫描命中、评论删除按钮与报告导出入口；Workspace 深链行高亮、History 键盘导航、Toast 自动消失均正常。
- **5 个核心 Git 能力接入 UI**（后端专属 186 → 181）：
  - `StageHunk` / `DiscardHunk` → 工作区 Unified / Split 差异每个 hunk 头部的「暂存此块 / 丢弃此块」；仅对普通修改文件展示，超大差异折叠时隐藏，构造的 patch 补足末尾换行以通过 `git apply`；
  - `GetStashFiles` / `ApplyFileFromStash` → 储藏箱「文件」清单（状态 / 增删行）与单文件恢复（恢复后自动暂存）；
  - `GetGitHooks` → 工具箱 → 维护与体检的 hooks 状态卡片（8 个常用钩子、启用计数、脚本路径）。
- **本轮回归验证**：真实仓库中两 hunk 文件只暂存第一块、只丢弃第二块；储藏单文件恢复后工作区内容与暂存区均正确；hooks 卡片显示 1/8 启用（pre-commit 可执行、commit-msg 不可执行）；Unified / Split 均显示块级按钮并成功暂存。
- **5 个核心 Git / LFS 能力接入 UI**（后端专属 181 → 176）：
  - `SaveStashWithFiles` → 工作区批量工具栏「储藏所选」：仅储藏勾选的已跟踪文件，未选改动留在工作区；未跟踪文件会被拦截提示；
  - `InspectLfsCacheStats` → 工具箱 → Git LFS 本地缓存体积 / 对象数 / 路径；
  - `GetLfsFilterConfig` / `SetLfsFilterConfig` → LFS 选择性下载的 `fetchinclude` / `fetchexclude` 可视化配置；
  - `CheckStagedNonLfsLargeFiles` → 工作区清理「暂存区大文件检查」，阈值可调，列出未走 LFS 的超限文件。
- **第二批回归验证**：勾选 `other.txt` 储藏后新 stash 生成、alpha 未选中改动保留；未跟踪文件被拦截且不弹窗；LFS 缓存卡显示 4.0 KB / 1 个对象；保存过滤规则后 `git config` 正确写入 `Assets/Characters/**,Assets/UI/**` 与 `Assets/Audio/**`；25 MB 的 `big.bin` 在 20 MB 阈值下被检出。
- **5 个 .gitignore 能力接入 UI**（后端专属 176 → 171）：工作区清理页新增「忽略规则体检与一键修复」卡片，聚合 `AuditIgnoreRules`（引擎识别 / 健康度 / 缺失规则 / 误跟踪产物）、`CheckIgnoreRule`（路径忽略来源）、`AddToGitignore`（追加自定义规则）、`GenerateGitignoreTemplate`（Go / Node / Unity / Unreal / Python 模板）与 `ApplyIgnoreSanitizer`（一键补规则并按需取消跟踪构建产物）。
- **第三批回归验证**：Unity 工程被识别，健康度 0/100、缺失 8 条规则、检出 `Library/cache.bin` 与 `.vs/config.json` 两个误跟踪产物；`Library/new.tmp` 正确显示 `.gitignore:1 (Library/)`；追加 Unity 模板后 `.gitignore` 写入对应规则；一键修复后 `git ls-files` 仅剩源码文件，磁盘产物保留。
- **5 个核心 Git 能力接入 UI**（后端专属 171 → 166）：
  - `GetWorkingWordDiff` → 工作区差异新增「词级」模式，按 `[-删除-] / {+新增+}` 行内着色；未跟踪、冲突文件与超大折叠差异自动回退；
  - `CreateAnnotatedTag` → 标签弹窗填写附注即创建 annotated tag，可选 GPG 签名；
  - `PushTag` → 标签列表新增「推送」，单个标签推送到 origin；
  - `RenameRemote` / `SetRemoteURL` → 远程仓库卡片新增编辑弹窗，可重命名并更新 fetch URL。
- **第四批回归验证**：词级模式正确渲染 `class Main {}{ int value = 42; }` 的行内增删；附注标签 `v1.0.0` 生成 tag 对象且消息正确；推送后裸仓库出现 `refs/tags/v1.0.0`；远程从 `origin` 重命名为 `backup` 且 URL 更新为新地址。
- **文件历史、远程状态与内容考古**（5 个能力接入；`GetFileHistory` 随之退出 UI，后端专属 166 → 162）：
  - `GetFileRevisionTimeline` → 文件历史改用 `git log --follow` 时间线并显示每次修订的文件体积；
  - `RevertFileToRevision` → 文件历史每条记录提供「恢复此版本」，恢复后自动暂存；
  - `PickaxeSearch` → 提交历史「内容考古」弹窗，支持字面量 / 正则、作者、路径、大小写与命中片段；
  - `GetMultiRemoteStatus` → 远程卡片显示当前分支 Ahead / Behind；
  - `TestRemoteConnection` → 远程卡片「测速」按钮，显示延迟与可见分支数。
- **第五批回归验证**：文件历史显示 47 B / 31 B / 14 B 三条修订，恢复最早版本后工作区内容为 `class Main {}` 且进入暂存区；远程卡片显示 `↑1 ↓0`，测速提示连接正常；内容考古搜索 `value` 命中 1 次并展示 `+1/-0`、片段与涉及文件。
- **5 个详情 / 配置能力接入 UI**（后端专属 162 → 157）：
  - `GetTagDetail` → 标签列表「详情」弹窗，展示附注类型、目标提交、打标者与说明；同时修复 tagger 时间戳未格式化的问题并新增回归测试；
  - `GetCommitSignatureDetails` → 提交检视器「签名」弹窗，展示 `%G?` 状态、签名者、密钥 ID、指纹与原始签名信息；
  - `GetCommitNotes` → 提交历史「批注总览」，列出全部 Notes 并支持跳转到对应提交；
  - `GetGitAttributes` / `SaveGitAttributes` → Git LFS 页内直接查看 / 编辑仓库根目录 `.gitattributes` 并保存。
- **第六批回归验证**：`v1.0.0` 详情显示 Annotated、打标者与附注；SSH 签名提交显示 `签名有效 (Good)` 与签名者；Notes 总览显示 1 条批注并可定位；保存 `.gitattributes` 后文件内容为 `*.psd filter=lfs diff=lfs merge=lfs -text`。
- **5 个 Studio 协作 / 合规能力接入 UI**（后端专属 157 → 152）：工具箱新增「Studio 协作」页签，聚合 `GetTeamActivity`（提交 / 活跃天数 / 连续提交 / 贡献者 / 文件热点）、`AuditPrePushChecklist`（推送前策略检查与得分）、`AuditCommitContentSafety`（最近 N 条提交内容安全审计）、`GetActivityLogs` / `ClearActivityLogs`（当前会话 Git 命令日志与清空）。
- **第七批回归验证**：团队活跃度显示 4 次提交、2 位贡献者与 4 个热点文件；推送前检查返回 `PASS_WITH_WARNINGS · 80/100` 及逐条规则；内容安全审计覆盖最近 20 条提交且通过；操作日志列出 14 条命令，清空后归零并显示空态。
- **5 个归档 / 补丁能力接入 UI**（后端专属 152 → 147）：维护与体检页新增「归档、补丁与 Git Bundle」卡片，支持 `ExportArchive`（ZIP）、`ExportPatch`（.patch）、`CreateGitBundle`（离线 Bundle）、`VerifyGitBundle` 校验与 `ApplyPatch` 应用到工作区。
- **第八批回归验证**：`HEAD` 成功导出 ZIP（807 B）与补丁（482 B）；`--all` Bundle 创建成功（2.3 KB）并通过校验；在父提交的克隆仓库中应用补丁后 `Assets/main.cs` 恢复为目标版本且保持未提交状态。
- **5 个引擎资产能力接入 UI**（后端专属 147 → 142）：工具箱新增「引擎资产」页签，聚合 `DetectGameEngineProject`（引擎 / 版本 / `.gitattributes` / LFS 规则）、`DetectGameEngineWorkflows`（缓存目录与建议动作）、`InspectAssetMetadata`（体积 / 类型 / 尺寸 / LFS）、`InspectAssetReferences`（引用扫描与可删除判断）、`CheckUnityMetaSync`（Unity `.meta` 缺失 / 孤立 / 暂存不一致）。
- **回归修复**：Unity meta 检查改用 `git ls-files` 枚举 `Assets/`、`Packages/` 实际文件，不再把 `.gitattributes`、`ProjectSettings/` 误报为缺失 meta；资产 LFS 状态改为读取真实 `git check-attr` 规则，不再把任意 `.png` 当作 LFS。新增 `TestCheckUnityMetaSyncIgnoresNonAssetPaths`、`TestInspectAssetMetadataUsesRealLfsRules`。
- **第九批回归验证**：Unity 工程识别为 `2022.3.10f1`，`.gitattributes` / LFS 均显示已配置；`Assets/hero.png` 元数据显示 68 B、`1 × 1 px`、非 LFS、0 处引用且可安全删除；Unity meta 检查准确列出 hero.png / main.cs 缺失与 orphan.png.meta 孤立，不再误报根目录文件。
- **5 个引擎工作流能力接入 UI**（后端专属 142 → 137）：引擎资产页新增 Unity `UnityYAMLMerge` 检测 / 配置、meta 一键修复、引擎 `.gitattributes` 预设应用，以及纹理规范审计（POT / 超大 / 体积建议）；同时收紧 unpaired_staged 判定，未跟踪的配对文件也能被识别。
- **第十批回归验证**：`UnityYAMLMerge` 配置后 `git config merge.unityyamlmerge.driver` 正确写入并显示「已配置」；Unity 预设覆盖 `.gitattributes` 并包含 merge/LFS 规则；纹理审计对 2 张纹理检出 1 项 `3 × 5` NPOT 警告；新增 `TestAutoFixUnityMetaStagesUnpairedAsset` 验证整对自动暂存。
- **4 个 Git Subtree 能力接入 UI**（后端专属 137 → 133）：子模块页新增子树区，支持 `AddSubtree`、`ListSubtrees`、`PullSubtree`、`PushSubtree`；并修复列表未持久化上游地址的问题——现在写入 `.git/wgit-subtrees.json`，拉取/推送使用真实 URL 与分支。
- **第十一批回归验证**：通过界面添加 `libs/lib` 子树（squash），列表显示真实上游路径与分支；上游新增 v2 后「拉取更新」将本地更新至 v2；本地提交 v3 后「推送回上游」成功写入裸仓库 `main`；新增 `TestAddSubtreePersistsMetadata` 验证元数据持久化。
- **5 个自定义操作 / 文件定位能力接入 UI**（后端专属 133 → 128）：Studio 页新增「自定义操作」区，支持 `GetCustomActions` 列表、`SaveCustomActions` JSON 编辑与 `ExecuteCustomAction` 执行（带命令确认与输出面板）；源码全景新增 `RevealFileInFinder`「定位」与 `OpenFileWithDefaultApp`「默认应用」。
- **第十二批回归验证**：默认动作与执行结果正常展示（命令、耗时、退出码、输出）；编辑 JSON 后 `.git/wgit-actions.json` 成功持久化并在列表回显；源码页「定位 / 默认应用」按钮在 Markdown 文件上正确出现。
- **5 个工作流 / 统计能力接入 UI**（后端专属 128 → 123）：提交编辑器新增正文输入与「模板 / 存模板」（`GetCommitTemplate` / `SaveCommitTemplate`）；维护页支持粘贴补丁内容检查 / 应用（`ApplyPatchContent`）；提交历史显示合并提交数与合并率（`GetCommitTopologyStats`）；多仓库总览显示当前仓库 commits / branches / tags / contributors（`GetRepoOverview`）。
- **第十三批回归验证**：保存模板后 `.gitmessage` 与 `commit.template` 正确写入，重载后「模板」可回填标题与正文；粘贴 README 补丁检查通过且未改工作区；历史页显示 `合并 2 · 33%`；总览显示 `6 commits · 1 branches · 0 tags · 1 contributors`。
- **2 个逐行暂存能力接入 UI**（后端专属 123 → 121）：Unified 差异新增逐行勾选，按选中行构造零上下文补丁，调用 `StageLines` 暂存索引或 `DiscardLines` 反向应用；支持跨 hunk、多行选择，新增/删除/重命名/二进制文件与折叠态自动隐藏。
- **第十四批回归验证**：在两 hunk 文件中只暂存 `-line 2`，索引仅含该删除；再暂存 `-line 8` 与 `+line 2 CHANGED`，索引精确包含所选行；勾选剩余 `+line 8 CHANGED` 丢弃后工作区干净、已暂存内容保持。
- **5 个工程扫描 / 合规诊断能力接入 UI**（后端专属 121 → 116）：欢迎页新增本地工程扫描（`ScanLocalProjects` + `GetProjectsDetails`）；远程卡片支持浏览器打开远程网页（`OpenWebURL`）；Studio 页新增隐私合规审计（`AuditPrivacyAndCompliance`）与 Git 认证诊断（`DiagnoseGitAuth`），并对后端返回的 null 数组做空值保护。
- **第十五批回归验证**：输入 `/tmp/wgit-subtree-main` 扫描到工程及分支/脏状态；隐私审计返回 `A · CrossPlatform`、4 项权限、清单已配置；Git 认证诊断显示 `osxkeychain` 与认证成功输出；本地路径远程点击网页打开时正确提示不支持。
- **4 个标签 / Hook / 文件清单能力接入 UI**（后端专属 116 → 112）：标签列表支持多选批量删除（`DeleteBatchTags`）与单标签删除远程（`DeleteRemoteTag`）；Hooks 卡片支持安装/禁用状态区分与启停（`ToggleGitHook`，GitHookInfo 新增 `installed`）；源码面板可复制全部已跟踪文件清单（`GetProjectAllFiles`）。
- **第十六批回归验证**：批量删除 `v1.0.0` / `v1.1.0` 后本地仅剩 `v1.2.0`；删除远程 `v1.2.0` 后远端仅剩前两个标签且本地保留；`pre-commit` 禁用后重命名为 `.disabled`，再启用恢复可执行；文件清单复制提示 `已复制 2 个文件路径`。
- **4 个 LFS 运维能力接入 UI**（后端专属 112 → 108）：Git LFS 页新增传输调优（`GetLfsTuningConfig` / `ApplyLfsTuningOptimization`，工作室高速 / 标准 / 恢复默认）、文件修订历史（`InspectLfsFileHistory`）与锁活动巡检（`GetLfsLockActivity`）。
- **第十七批回归验证**：应用「标准」预设后 `lfs.concurrenttransfers=8`、`http.postBuffer=268435456`、`lfs.dialtimeout=30`；`README.md` 历史读取 1 条修订；无 LFS 工具链时锁活动正确显示 0 计数与未连接提示。
- **9 个 LFS 守护能力接入 UI**（后端专属 108 → 99，LFS 分类清零）：Git LFS 页新增选择性拉取（`ExecuteLfsSelectivePull`）、LFS 磁盘占用（`GetLfsDiskUsage`）、可锁资产只读保护（`SyncLockableFilePermissions` / `ToggleLockableReadOnlyProtection`）、批量扩展名跟踪（`TrackExtensionsAsLfs`）、单文件锁状态（`CheckFileLockStatus`）、团队锁冲突检测（`CheckRemoteLocksConflict`）、CSV / Markdown 锁审计报表（`ExportLfsLocksReport`）与推送前 LFS 完整性校验（`VerifyPrePushLfs`）。
- **第十八批回归验证**：同步可锁权限显示 0 个可锁文件；启用只读保护后 `lfs.locksverify=true` 且按钮切换为「关闭只读保护」；`alpha.txt` 查询显示未被锁定；冲突检测返回 0；Markdown 报表包含仓库路径、生成时间与 0 条活跃锁；推送前校验返回「可安全推送 · 6 个待推送提交，0 个 LFS 资产缺失」；无 `git-lfs` 工具链时选择性拉取如实显示 `git: 'lfs' is not a git command`，不伪造成功。
- **13 个 Studio 协作能力接入 UI**（后端专属 99 → 86，Studio 分类清零）：Studio 页新增可折叠「Studio 集成与流水线」与「Studio 运维与构建缓存」两区，接入 TAPD 工作项同步 / 提交关联 / 单号解析（`FetchTAPDWorkItems`、`BindCommitToTAPD`、`ParseTAPDCommitReferences`）、CI 流水线查看与触发（`GetStudioCIPipelines`、`TriggerStudioCIPipeline`）、能力清单（`GetFullParityStatusSummary`、`GetPinnacleParityStatusSummary`）、凭据保险库扫描与 AES-256-GCM 加密存储（`InspectCredentialVault`、`StoreCredentialSecret`）、Webhook 登记（`ManageStudioWebhooks`）、DDC 缓存检测 / 修剪（`ManageStudioDDC`）、离线队列（`ManageOfflineSubmitQueue`）与多工程拓扑（`ManageStudioSwitchboard`）。
- **第十九批回归验证**：TAPD 同步 4 个工作项，提交信息解析出 `--tapd=1002341`，关联后 `git notes show` 写入 `TAPD #1002341 Status: resolved`（验证后已删除）；CI 显示 5 个阶段并成功触发构建；保险库写入后 `.git/wgit_vault.json` 权限 0600、无明文密钥、脱敏显示 `test****123`；Webhook 仅保存脱敏 URL；离线队列入队后暂停成功；拓扑显示 22 工程 / 21 邻接仓库；测试产物已清理。
- **5 个核心 Git 查看 / 统计能力接入 UI**（后端专属 86 → 81）：工作区顶部变更统计条（`GetWorkspaceStats`）、提交编辑器「上次」载入上一条提交信息（`GetLastCommitMessage`）、体积页工作区扩展名分布（`GetRepoStorageBreakdown`）、分支比对「查看差异详情」（`GetDiffBetween`）、文件历史每条修订「差异」（`GetSourceFileDiff`）。
- **回归修复**：`CompareBranches` 在单侧无提交时返回 nil 数组导致比对页空白，现初始化空切片并新增 `TestCompareBranchesEmptySlices`；`GetSourceFileDiff` 对根提交使用 `git diff <hash>^!` 为空，改为 `git show --format= --patch` 并新增 `TestGetSourceFileDiffRootCommit`。项目根目录再次出现空 `.git`，已移至 `/tmp/wgit-project-git-backup-20261022-070213`。
- **第二十批回归验证**：工作区统计条显示 `暂存 0 / 未暂存 0 / 未跟踪 1`；「上次」回填 `main: lib v3`；扩展名分布显示 `.md`；临时分支比对显示 `领先 1 / 落后 0`，差异详情列出 `verify-diff.txt +1` 并渲染补丁；文件历史根提交「差异」正确显示 `diff --git a/README.md`；测试分支与服务器已清理。
- **6 个仓库健康 / 清理 / 配置能力接入 UI**（后端专属 81 → 75）：维护页新增综合体检评分（`RunRepoHealthCheck`），体积页新增历史大对象扫描（`ScanHistoryLargeBlobs`），工作区清理新增空目录扫描与 `.gitkeep` 生成（`ScanEmptyDirectories` / `GenerateGitkeeps`），设置新增「高级 Git 配置」页签（`GetDetailedGitConfig` / `SetCustomConfigKey`）。
- **第二十一批回归验证**：综合体检返回 82 分、4 通过 / 1 警告 / 1 失败及六项明细；历史大对象扫描列出 31 B / 5 B 等历史 Blob；临时空目录生成并暂存 `.gitkeep` 后清理；高级配置页读取 local/global 条目，写入 `wgit.verify.batch21=ok` 后命令行确认并已删除；服务已停止。
- **7 个引擎质检 / 系统集成能力接入 UI**（后端专属 75 → 68，系统集成分类清零）：引擎页新增资产质检区，接入模型面数（`LintModelMesh`）、骨骼动画（`LintAnimationRig`）、音频规范（`InspectAudioFile`）、本地化对齐（`AuditLocalizationParity`）、原生插件 ABI（`AuditNativePluginsAbi`）与缺失材质扫描（`ScanMissingMaterials`）；源码全景图片新增「复制 Base64」（`GetImageBase64`）。
- **回归修复**：`CheckUnityMetaSync` 无问题时返回 nil 导致引擎页整体空白，现返回空切片并新增 `TestCheckUnityMetaSyncReturnsEmptySlice`；引擎页对 meta 数组、UnityYAMLMerge 版本、纹理审计列表、资产引用及质检结果数组统一做 null 兜底。
- **第二十二批回归验证**：`hero.obj` 返回 2 面 / 30,000 预算 / OK；`hero_idle.gltf` 返回 Mobile Ready 且无超预算问题；48kHz 单声道 WAV 通过规范检查；本地化审计发现 2 种语言、缺失 `ui_title`；原生插件识别 macOS x86_64；缺失材质扫描 0 材质 / 100 分；1×1 PNG 复制 Base64 为 118 字符；测试资产已清理。
- **5 个工程级审计能力接入 UI**（后端专属 68 → 63）：引擎质检区新增 LOD / 纹理串流（`AuditLodStreaming`）、物理碰撞体（`AuditPhysicsMeshCollision`）、材质实例覆盖（`AuditMaterialInstanceOverrides`）与 DCC 临时锁巡检（`DetectDCCLockActivity`）；体积页新增包体预算与平台占用分析（`AnalyzePackageSizeBudget`）。
- **第二十三批回归验证**：5 个 OBJ 检出 1 个缺失 LOD、健康度 85；物理审计检出 2 个碰撞体 / 1 个 300 顶点超预算凸包 / 动态非凸与缺失物理材质各 1 处；材质实例 1 个、覆盖参数 1 个；DCC 巡检检出 Maya `.mb.swp` 临时锁；包体预算显示 11 个资产 / 3.4 KB / 全平台合规；测试资产已清理。
- **6 个核心 Git 操作接入 UI**（后端专属 63 → 57）：维护页新增最近 N 条提交压缩（`SquashCommits`），提交检视器新增「Revert 不提交」（`RevertNoCommit`）与「分离检出」（`CheckoutCommitDetached`），分支重命名支持同步重命名远端（`RenameBranchWithRemote`），子模块行新增「同步 URL」（`SyncSubmodule`），代理设置为空时改调 `ClearGitProxy` 修复“假清除”缺陷。
- **第二十四批回归验证**：一次性仓库中代理写入/清除后 `git config` 从有值变为 unset；子模块同步输出 `Synchronizing submodule url for 'sub'`；分支 `feature` 重命名为 `feature-renamed` 后本地与远端旧分支均被替换；`Revert 不提交` 将 `file3.txt` 删除留在暂存区且未产生新提交；`分离检出` 后 HEAD 为非分支状态；压缩最近 2 条提交后历史变为 `squashed batch24 / commit 2 / commit 1`；测试仓库已删除。
- **4 个核心 Git 操作接入 UI**（后端专属 57 → 53）：储藏弹窗新增「仅储藏已暂存改动」（`StashStagedOnly`），工作区变更工具栏新增「全部丢弃」（`DiscardAll`），DCC 巡检卡新增资产「自动上锁」（`AutoLockDCCAsset`），提交历史多选工具栏新增「复制详情」（`GetCommitDetailsBatch`）。
- **第二十五批回归验证**：暂存 `staged.txt` + 未暂存 `tracked.txt` + 未跟踪 `untracked.txt` 时，仅储藏后 `staged.txt` 被收走，未暂存与未跟踪保留；「全部丢弃」后工作区干净且未跟踪文件删除；多选 2 条提交复制详情提示 `已复制 2 条提交详情`；无 `git-lfs` 时自动上锁如实显示 `git: 'lfs' is not a git command`；测试仓库已删除。
- **5 个质量 / 发布能力接入 UI**（后端专属 53 → 48）：提交编辑器实时规范检查（`LintCommitMessage`），维护页新增崩溃栈反解（`DemangleCrashStack`）与热更清单生成（`GenerateHotfixManifest`），引擎质检新增 Shader 变体审计（`InspectShaderVariants`）与宏定义 / 调试开关审计（`AuditSymbolsAndDefines`）。
- **第二十六批回归验证**：输入 `fix: update` 显示 `A · 90 · 1 项` 并提示描述笼统，改为 `fix: resolve crash on startup` 后变为 `A · 100`；崩溃日志解析 2 帧并定位 `game.cpp:128`；热更清单基于 `HEAD~1..HEAD` 生成 2 个 Config 文件、303 B / 压缩 260 B 与 manifest JSON；Shader 审计返回 9 个变体 / 1 Pass / 4 关键字；宏审计检出 `ENABLE_PROFILER` 与 `DEVELOPMENT_BUILD` 两个危险宏；测试仓库已删除。
- **4 个引擎工程能力接入 UI**（后端专属 48 → 44）：引擎质检新增数据表语义差异（`AuditDataTableDiff`），引擎页新增启动编辑器（`LaunchGameEngine`）与缓存清理；缓存清理按引擎类型分流到 `CleanGameEngineCaches`（Unity / Godot / Cocos）与 `CleanUnrealEngineCache`（Unreal）。
- **第二十七批回归验证**：Unity 工程识别后清理 Library / Temp / Logs / obj 四个目录并释放 18 B；Unreal 工程清理 Intermediate / Saved / DerivedDataCache 三个目录并释放 6 B；数据表审计对 `items.csv` 检出 `+1 / ~1` 行与 1 项结构变更、0 严重；启动按钮在识别引擎后可用（为避免实际拉起编辑器，本轮未点击执行）。
- **第二十七批测试仓库已删除，验证服务已停止。**
- **3 个核心 Git 操作接入 UI**（后端专属 44 → 41）：提交历史新增范围 Cherry-Pick（`CherryPickRange`，`A..B` 不含起点），工作区清理支持逐文件删除未跟踪项（`DiscardUntrackedFile`），子模块递归同步改用专用 RPC（`UpdateSubmodulesRecursive`）。
- **第二十八批回归验证**：`main..feature` 范围拣选后在 main 上生成 `feat: a` / `feat: b` 两条新提交；逐个删除 `junk1.txt` 后 `junk2.txt` 保留；递归同步按钮返回「所有子模块已成功递归同步与更新」且子模块状态为 SYNCED；测试仓库已删除。
- **说明**：原剩余 12 个核心 Git 重复方法（MergeBranch / RebaseBranch / AbortMerge / GetCommitsPaged / GetFileHistory / GetMergeToolConfig / SearchSourceCode / ShelveSelectedFiles 等）已于 2026-10-22 全部删除；合并、变基与冲突测试改由 `MergeAdvanced` / `RebaseAdvanced` / `AbortConflictOperation` 覆盖。
- **3 个 Unreal / 资产能力接入 UI**（后端专属 41 → 38）：引擎质检新增跨分支资产拣选（`CherryPickAssetBundle`，支持依赖 / `.meta` 与预览）与 Unreal 资产节点差异（`DiffUnrealAssetNodes`）；引擎概览新增 Unreal 工程详情（`DetectUnrealProject`）。
- **第二十九批回归验证**：资产拣选预览返回 2 个资产 / 47 B（核心 + `.meta`），执行后两个文件进入暂存区；Unreal 工程卡显示 MyGame / 5.3 / `.uproject` 路径；文本伪 UAsset 对比返回 0 节点（结构解析需真实二进制 UE 资产），接口链路正常；测试仓库已删除。
- **2 个场景流式能力接入 UI**（后端专属 38 → 36）：引擎质检新增场景流式合并审计（`AuditSceneStreamMerge`，GUID 碰撞 / 空间重叠 / 变换冲突 / 推荐计划）与场景 Actor GUID 修复（`ResolveSceneStreamConflict`，重新生成 GUID / 空间偏移）。
- **第三十批回归验证**：Unity 场景在 feature 与 main 上同名不同 GUID 内容时，审计返回安全分 70、1 处 GUID 碰撞，并推荐重新生成 GUID；执行修复后 `.unity` 文件 GUID 由 `1111…` 变为 `58834721…`，文件内容保持；测试仓库已删除。
- **2 个核心 Git 能力接入 UI**（后端专属 36 → 34）：提交编辑器新增「快速提交」（`FastCommit`：暂存全部 → 提交 → pull --rebase → push，强确认），文件历史版本横幅新增「复制内容」（`GetFileContentAtCommit`）。
- **第三十一批回归验证**：快速提交后本地工作区干净，远端 `main` 同步出现 `feat: fast commit`；复制历史版本内容按钮已接线并通过类型检查，但本轮浏览器工具连接不稳定，未完成点击级验证；`GetFileContentAtCommit` 已有后端测试覆盖。
- **测试仓库已删除，验证服务已停止。**
- **10 个演示型引擎接口接入可折叠诊断区**（后端专属 34 → 24）：字体版权、光照烘焙、贴图通道、循环依赖、热更回滚追踪、Bundle 差异、音频 Bank 差异、World Partition 数据层、离线补丁包、Shader 自愈；统一标注「后端样例数据，仅预览字段结构」，不伪装为真实业务入口。
- **第三十二批验证**：`vue-tsc` 与 `bun run build` 通过；浏览器 MCP 连接在批次末尾不可用，演示区未完成点击级 E2E。
- **1 个伪实现修正并接入 UI**（后端专属 24 → 23，引擎分类清零）：`ArtistOneClickPipeline` 由硬编码「已推送」改为真实暂存 → `[Art]` 提交 → push；引擎页新增「美术一键交付」，展示提交文件 / 贴图 / 网格 / 推送状态与步骤日志。
- **第三十三批回归验证**：新增 `TestArtistOneClickPipelineCommitsAndPushes`，在临时仓库 + 裸远端中验证 `stagedAssetsCount > 0`、`pushed=true`，远端出现 `[Art] artist delivery` 提交；`vue-tsc`、`bun run build`、`go test .`（52 个测试）、`go build` 通过；浏览器 MCP 仍不可用，UI 未完成点击级 E2E。
- **无用代码清理（第一轮）**：按「无 UI、无内部调用、无测试」口径删除 11 个后端方法与 10 个前端未调用封装（详见 [dead-code-cleanup.md](dead-code-cleanup.md)）。RPC 338 → 327，facade 325 → 315，后端专属 23 → 12，未调用封装 10 → 0；`git_service.go` 4,552 → 4,501 行 / 176 方法，`git_studio_engine.go` 45 → 44 方法。验证：`vue-tsc`、`bun run build`、`go test .`（52 个测试，7.8s）、`go build` 全部通过；备份位于 `/tmp/wgit-cleanup-backup-20261022-075329`。
- **重复 API 清理（第二轮）**：删除最后 12 个后端专属重复接口（AbortCherryPick / AbortMerge / AbortRebase / GetCommitsPaged / GetFileHistory / GetMergeToolConfig / MergeBranch / MergeBranchWithOptions / RebaseBranch / RebaseBranchWithOptions / SearchSourceCode / ShelveSelectedFiles）及 5 个死类型；`TestGitService_MergeAndRebase` 重写为 `MergeAdvanced` / `RebaseAdvanced` / `AbortConflictOperation`，分页断言迁移到 `GetCommitsFiltered`。RPC 327 → 315，后端专属 12 → 0，可达率 96% → 100%；`git_service.go` 4,501 → 4,321 行 / 176 → 165 方法，`git_project_browser.go` 8 → 7 方法。验证：`vue-tsc`、`bun run build`、`go test .`（52 个测试，8.5s）、`go build` 全部通过；Wails bindings 重新生成（315 Methods）；备份位于 `/tmp/wgit-cleanup2-backup-20261022-083307`。
- **收尾优化（2026-10-22）**：`MergeAdvanced` / `RebaseAdvanced` 将冲突作为可执行结果返回，不再以 RPC 错误中断；合并/变基弹窗会立即关闭、刷新状态，并在工作区显示冲突横幅。Workspace 切换仓库时清空旧仓库的文件选择、源码详情、文件历史、Blame 状态与提交草稿，避免跨仓库串数据。另删除未引用的 `FastCommitResult`（前端）与 `TAPDTransitionResult`（Go）。验证：`vue-tsc`、`bun run build`、`go test .`（52 个测试，7.4s）、`go build`、`go vet` 通过；server 模式浏览器验证仓库切换无残留、冲突通知与横幅即时出现，控制台无错误。
- **跨仓库状态隔离（扩展，2026-10-22）**：BranchesView / StashView 在切换仓库时关闭弹窗并清空分支比对、储藏选择、搜索与加载标志；工具箱子页签按仓库路径重挂载，LFS 历史路径等本地输入不再跨仓库残留；分支比对、储藏 diff、Workspace 工作区统计及源码 / Blame / 文件历史加载增加仓库代次或路径校验，旧仓库的异步响应不会再覆盖新仓库视图。服务端扫描确认无未引用 Vue 组件、无未引用 Go 导出函数。验证：`vue-tsc`、`bun run build`、`go test .`（52 个测试）、`go build` 通过；server 模式浏览器验证 repo-a → repo-b 后储藏选择与 diff 清空，分支比对页切回仓库后回到本地分支视图，控制台无错误。
- **首屏包体拆分（2026-10-22）**：History / Branches / Stash / Review / Tools / Settings 改为 `defineAsyncComponent`，Workspace 保持首屏同步加载。构建产物从单个 1,069 kB JS 拆为 465.81 kB 首屏 + ToolsView 355.66 kB + History 87.15 kB + Branches 75.06 kB + Review 38.02 kB + Settings 31.79 kB + Stash 23.14 kB，消除 >500 kB 构建告警。随后把 10 个工具子页签也改为异步组件，首屏降至 **361.73 kB**、ToolsView 48.59 kB；EngineTab 87.11 / StudioTab 60.40 / LfsTab 37.52 / MaintenanceTab 35.74 / RemotesTab 25.17 / CleanTab 19.30 / SubmodulesTab 18.67 / DiskTab 16.87 / ReflogTab 6.26 / SparseTab 5.57 kB，打开工具箱只加载当前子页签。验证：13 个子页签逐一打开均正常渲染，控制台无错误。
- **测试盲区修复（2026-10-22）**：`TestGitService` 在非 git 工作区会整体跳过，导致引擎/资产断言从未执行。拆出 `TestAssetAuditsInTempRepo`（临时仓库），覆盖资产引用、忽略规则、音频/网格审计、着色器变体、DCC 锁、Unreal 差异、场景流合并、动画/本地化/ABI、TAPD、热更、Studio 运维、凭据、Bundle、隐私合规、源码浏览器等 Round 101–140 自包含断言；另新增 `TestGameEngineDetectionPresetsAndCleanup` 覆盖 Unity/Unreal/Godot/Cocos 检测、引擎预设、缓存清理与纹理 POT 审计；测试函数 52 → 56，语句覆盖率 22.1% → 52.6%。
- **初始仓库取消暂存修复（2026-10-22）**：新增 `TestCoreRepoWorkflowInTempRepo` 覆盖 InitRepo/OpenRepo/StageFile/UnstageFile/GetWorkingDiff/Commit/ScanLocalProjects/GetProjectAllFiles/活动日志；测试发现首个提交前 `git restore --staged` 因无 HEAD 失败，`UnstageFile` 已回退到 `git rm --cached`。
- **LFS 配置巡检测试（2026-10-22）**：新增 `TestLfsConfigAndInspectionWithoutBinary`，覆盖调优预设、fetchinclude/exclude、缓存统计与磁盘占用、文件修订历史、暂存区非 LFS 大文件检查及锁报表；通过 `GIT_CONFIG_GLOBAL/SYSTEM=/dev/null` 隔离宿主机全局配置。
- **配置持久化与 Unity 合并工具测试（2026-10-22）**：新增 `TestConfigPersistenceInTempHome`（临时 HOME 下默认加载、保存/读取往返、最近仓库追加/去重与顺序）、`TestConfigMigratesLegacyUgitFile`（`~/.config/ugit/config.json` 迁移到 `~/.config/wgit/config.json`）、`TestUnityYamlMergeConfiguration`（检测未配置状态、写入 `merge.unityyamlmerge.driver` 与 `.gitattributes` 规则、重新检测生效）；测试函数 56 → 59，语句覆盖率 52.6% → 53.0%。
- **内联弹窗无障碍补齐（2026-10-22）**：为 History / Branches / Stash / Workspace / LFS / Reflog / Remotes / Submodules / ChangesList 中 22 个此前仅靠 `v-if` 遮罩的内联弹窗接入 `useModalA11y`，统一补齐 `role="dialog"`、`aria-modal="true"` 与可访问名称；History 的 `isDialogOpen` 补入范围拣选、批注编辑、签名详情、批注总览与内容考古，避免弹窗打开时方向键仍驱动提交列表；Workspace 版本差异弹窗打开时屏蔽全局 ⌘↵ 提交快捷键。`useModalA11y` 的 Esc 处理增加 `stopPropagation`，防止视图级键盘处理器重复响应。验证：全部 37 个弹窗遮罩具备 dialog 语义；`vue-tsc`、`bun run build` 通过；server 模式实测「储藏所选文件」与「Git Notes 批注总览」初始焦点进入弹窗、Tab/Shift+Tab 双向循环、Esc 关闭且焦点返回触发按钮，控制台无错误。
- **错误反馈统一（2026-10-22）**：12 个视图/分区中的 74 处原生 `alert()` 全部替换为 `notify()` 应用内 Toast，覆盖 App 的初始化 / 打开仓库 / Fetch / Pull / Push / 检出失败，Settings 的身份、签名、代理保存失败，History / Branches / Stash / Workspace 操作失败，以及 Reflog / LFS / Clean / Remotes 等工具箱子页签失败；各父视图已有 Toast 接线，子分区通过 `@notify` 上抛。Toast 增加 `max-w-md`、`break-words` 与 `leading-relaxed`，长 Git 错误不再溢出视口。验证：源码原生 `alert` 调用 0，`vue-tsc`、`bun run build` 通过；server 模式触发缺失 `git-lfs` 的「LFS 拉取」，Toast 完整显示错误，`window.alert` 调用 0，宽度 448px / 视口 756px，控制台无错误。
- **分支 / 储藏 / 同步集成测试（2026-10-22）**：新增 `TestBranchStashSyncWorkflowsInTempRepo`，在临时仓库 + 本地裸远端中覆盖分支创建/检出/重命名/删除、附注标签创建与解析、储藏 save/apply/pop/branch、远端 add/push/fetch/pull/remove、upstream 设置、标签远端推送/删除、`RevertCommit` 与 `ResetBranch`。断言覆盖 `GetBranches` 的 current/upstream 状态、`GetTags` 的短哈希与附注类型、`GetRemoteDetails` 的 fetch/push 双条目、储藏消费与工作区清洁状态。测试函数 59 → 60，语句覆盖率 53.0% → 54.4%，`go test`、`-race`、`go vet` 全部通过。
- **冲突解决与逐块暂存测试（2026-10-22）**：新增 `TestConflictResolutionInTempRepo` 覆盖真实合并冲突下的 `GetConflictFileContent`、ours / theirs / backup_and_theirs / mark 四种 `ResolveConflict` 路径、备份文件保留冲突标记与非法 resolution 报错；新增 `TestHunkStagingAndWorkspaceCleanupInTempRepo` 覆盖 `StageHunk` / `DiscardHunk` / `StageLines` / `DiscardLines`、`UnstageAll`、`CleanWorkingTree` 干跑与实删、`AddToGitignore`、`DiscardFile`（已跟踪与未跟踪）以及 `GetGitConfig` / `SetGitConfig`。测试函数 60 → 62，语句覆盖率 54.4% → 55.3%，`go test`、`-race`、`go vet` 全部通过。
- **FastCommit 无上游提示修复（2026-10-22）**：无远端或未配置 upstream 时，`git pull --rebase` 失败曾被误报为「rebase has conflict」。现改为「pull --rebase did not complete」并附实际错误，区分「本地已提交但未推送」与真实冲突；新增 `TestFastCommitWithoutRemoteReportsLocalOnly` 验证本地提交成功、`Pushed=false`、提示不含冲突误报且工作区干净。测试函数 62 → 63；`go test`、`go build` 通过，覆盖率保持 55.3%。
- **工具链集成测试与工作树分支修复（2026-10-22）**：新增 `TestArchiveAndPatchWorkflowsInTempRepo`（zip 归档内容、非法 ref、`format-patch` 导出与克隆后 `ApplyPatch`）、`TestWorktreeLifecycleAndMaintenanceInTempRepo`（链接工作树创建/列出/提交/移除、新分支工作树、空路径报错、维护统计）、`TestSubmoduleLifecycleInTempRepo`（本地子模块添加/状态/更新/移除，文件协议隔离）。测试发现 `GetBranches` 未处理 `git branch` 对「另一工作树已检出分支」的 `+ ` 前缀，导致分支列表出现名为 `+` 的伪分支；已修复并由此测试回归覆盖。测试函数 63 → 66，语句覆盖率 55.3% → 56.6%，`go test`、`-race`、`go vet`、`go build` 全部通过。
- **历史查看与恢复测试（2026-10-22）**：新增 `TestHistoryInspectionAndRecoveryInTempRepo` 覆盖 `GetFileBlame`、`GetFileContentAtCommit`、`GetDiffBetween`、`GetWorkingWordDiff`、`GetRepoOverview`、`CreateAnnotatedTag`、`GenerateGitignoreTemplate`、`GetRepoStorageBreakdown`、`GetCommitSignatureDetails`、`GetReflog`、`CreateBranchFromReflog`、`GetRebaseCommits`、`InteractiveRebasePlan`、`CheckoutCommitDetached`、`SaveStashWithFiles` 与选择性储藏；另新增 `TestSquashCommitsInTempRepo` 验证软重置压缩、非法参数与压缩后历史。测试函数 66 → 68，语句覆盖率 56.6% → 58.8%，`go test`、`-race`、`go vet`、`go build` 全部通过。
- **远端管理与 Notes 同步测试（2026-10-22）**：新增 `TestRemoteManagementAndNotesSyncInTempRepo` 覆盖 `TestRemoteConnection` 成功/失败、`RenameRemote`、`SetRemoteURL`、默认远端解析，以及 `SaveCommitNote` / `GetCommitNote` / `GetNotedCommitHashes` / `PushCommitNotes` 到本地裸远端的完整链路；验证 fetch/push 双条目与远端 notes ref。测试函数 68 → 69，覆盖率 58.8% → 59.1%。
- **批量恢复测试与 BatchDiscardFiles 修复（2026-10-22）**：新增 `TestBatchAndStashRecoveryWorkflowsInTempRepo` 覆盖 `ApplyFileFromStash`、`StashStagedOnly`、`BatchDiscardFiles`、`RevertNoCommit`、`DeleteBatchTags`。测试发现 `BatchDiscardFiles` 对「已跟踪 + 未跟踪」混合路径统一执行 `git checkout --` 会因未跟踪 pathspec 报错并中断；现改为逐路径用 `ls-files --error-unmatch` 分类，已跟踪走 `restore`、未跟踪走 `clean -fd`，混合批量丢弃可完整成功。测试函数 69 → 70，覆盖率 59.1% → 59.4%，`go test`、`-race`、`go vet`、`go build` 全部通过。
- **稀疏检出 / 工作树 / 远端工具测试（2026-10-22）**：新增 `TestSparseCheckoutAndWorktreeLockingInTempRepo` 覆盖稀疏目录设置/展开与文件可见性、工作树锁定/解锁；新增 `TestRemoteRenamePruneBundleReportSearchInTempRepo` 覆盖非默认分支本地+远端重命名、远端陈旧分支 prune、bundle 创建/校验、Markdown/HTML 提交报告与 `SearchRepoCode` 大小写/空查询。测试发现 `RenameBranchWithRemote` 静默忽略远端推送/删除错误：现默认分支跳过删除（Git 禁止删除远端默认分支），非默认分支删除失败返回明确错误，推送新分支失败也不再静默。测试函数 70 → 72，覆盖率 59.4% → 60.3%，`go test`、`-race`、`go vet`、`go build` 全部通过。
- **交互变基执行修复（2026-10-22）**：新增 `TestInteractiveRebaseExecuteInTempRepo`，通过真实 `GIT_SEQUENCE_EDITOR` 执行「保留 A/C、丢弃 B」的变基并验证重写后历史与工作区。测试发现 `os.CreateTemp` 创建的脚本为 0600，随后 `os.WriteFile(..., 0755)` 不会改变已有文件权限，导致 `git rebase -i` 始终 `Permission denied`；现补 `os.Chmod(0755)`，交互变基恢复正常。测试函数 72 → 73，覆盖率 60.3% → 60.6%，`go test`、`-race`、`go vet`、`go build` 全部通过。随后以 server 模式浏览器实测：History 选择基准提交 → 打开交互式变基 → 将 `feat: a` 设为 `drop` → 执行，弹窗关闭、Toast 显示「成功变基并更新 refs/heads/master」，磁盘历史正确变为 `base → feat: b → feat: c`，证明 UI 到后端链路可用。
- **提交报告注入修复（2026-10-22）**：`ExportCommitReport` 的 HTML 输出直接拼接提交作者与说明，存在 `<script>` 注入；Markdown 表格也未处理说明中的 `|`。现 HTML 分支使用 `html.EscapeString`，Markdown 分支转义 `|`，并使用 `SplitN(..., 4)` 保留含 `|` 的完整提交说明；新增 `TestExportCommitReportEscapesMarkup` 验证脚本标签被转义、管道符不破坏表格。测试函数 73 → 74，覆盖率保持 60.6%。
- **远端网页信息与外部工具配置测试（2026-10-22）**：新增 `TestRemoteWebInfoAndExternalToolConfigInTempRepo` 覆盖 GitHub/GitLab/Gitea/Bitbucket/Gitee/通用仓库 URL 解析、origin 缺失时回退首个远端、本地/全局外部 diff/merge 工具配置（隔离 `GIT_CONFIG_GLOBAL`）；新增 `TestStashDropApplyPatchContentAndGitVersion` 覆盖 `StashDrop`、`ApplyPatchContent` 干跑/应用/空内容、`GetGitVersion`。扩展子模块测试覆盖 `SetSubmoduleBranch` 与 `SyncSubmodule`。测试发现 `RemoveSubmodule` 在 `.gitmodules` 存在未暂存修改（如刚设置分支）时会被 `git rm` 拒绝；现删除前先暂存 `.gitmodules`。测试函数 74 → 76，覆盖率 60.6% → 61.3%，`go test`、`-race`、`go vet`、`go build` 全部通过。
- **配置作用域 / 代理来源 / 仓库健康修复（2026-10-22）**：新增 `TestConfigProxyIgnoreAndRepoHygieneInTempRepo` 覆盖详细配置读取、自定义键、孤立全局配置、代理设置/清除、gitattributes、忽略规则、提交拓扑、批量提交详情、空目录扫描与 `.gitkeep` 生成、`git fsck` 健康检测。测试发现三处问题：`GetDetailedGitConfig` 仅凭文件名判断来源，XDG/自定义全局配置被误判为 local；`GetGitProxy` 使用合并读取，全局代理被误标为本地；`VerifyRepoHealth` 忽略 `git fsck` 错误，损坏仓库可能显示健康。现分别改用 `--show-scope`、按 local/global 显式读取、并把 fsck 错误与非 dangling 输出计入 `RawErrors`。测试函数 76 → 77，覆盖率 61.3% → 62.9%，`go test`、`-race`、`go vet`、`go build` 全部通过。
- **未跟踪删除安全 / 分支清理 / 多远端与模板测试（2026-10-22）**：`DiscardUntrackedFile` 原直接 `os.RemoveAll(repoPath + path)`，可删除已跟踪文件或经 `../` 越出仓库；现先用 `status --porcelain` 确认 `??`，再执行 `git clean -fd`，越界/已跟踪路径返回错误。新增 `TestMergedBranchesCherryPickRangeAndCommitTemplate` 覆盖合并分支识别、`-d` 安全批量删除、范围 cherry-pick 与提交模板读写；扩展远端测试覆盖 `GetMultiRemoteStatus` 的 ahead/behind/URL、`SyncMultiRemotes` fetch-all / 非法动作、无远端时 `Remotes` 返回非 nil 空切片。同步修复 RemotesTab `remotes?.find` 的空值保护。测试函数 77 → 78，覆盖率 62.9% → 63.9%，`go test`、`-race`、`go vet`、`go build`、`vue-tsc`、`bun run build` 全部通过。
- **克隆 / 磁盘分析 / 历史大对象 / 图片预览测试（2026-10-22）**：新增 `TestCloneRepoAdvancedInTempRepo` 覆盖完整、浅克隆、指定分支克隆、非法 URL、空参数与占用目录；新增 `TestDiskBreakdownLargeBlobsAndImagePreview` 覆盖磁盘分类、历史最大对象扫描、工作区/指定提交图片 Base64 预览及非图片/缺失文件报错。`ScanHistoryLargeBlobs` 结果切片改为非 nil 初始化。测试函数 78 → 80，覆盖率 63.9% → 65.4%，`go test`、`-race`、`go vet`、`go build` 全部通过。
- **Subtree pull/push 测试（2026-10-22）**：扩展 `TestAddSubtreePersistsMetadata`，在独立 lib/main 仓库间验证子树添加后上游更新 → `PullSubtree` 同步到工作区 → main 修改后 `PushSubtree` 回写上游 main ref，并验证 pull/push 后元数据仍保留、空 prefix/URL 报错。覆盖率 65.4% → 65.6%（测试函数保持 80），`go test`、`-race`、`go vet`、`go build` 全部通过。
- **高级同步 / 自定义动作 / 场景解析测试（2026-10-22）**：新增 `TestAdvancedSyncOpsInTempRepo` 覆盖 ahead/behind 提交、`PushAdvanced`、`PreviewMerge` 干净与冲突、`PullAdvanced` rebase 成功与 merge 冲突、`GetRebaseProgress` 空闲态；发现 `PullAdvanced` 冲突仍以错误返回，导致 SyncModal 的结构化冲突分支不可达且不刷新状态，现与 `MergeAdvanced` 一致返回 `HasConflicts` 结果且 err 为 nil。新增 `TestCustomActionsActivityLogsAndSigningStatus` 覆盖活动日志读写、默认/持久化/损坏配置回退、自定义动作执行（成功/失败/工作目录/未知 ID）与提交签名审计；新增 `TestUnityAndGodotSceneParsers` 覆盖 Unity prefab/scene、Godot scene、向量解析与 AABB 体积/重叠。测试函数 80 → 83，覆盖率 65.6% → 67.9%，`go test`、`-race`、`go vet`、`go build` 全部通过。
- **敏感信息扫描与脱敏修复（2026-10-22）**：新增 `TestScanStagedSecretsAndRedaction` 覆盖私钥/环境文件路径、AWS/GitHub/私有密钥/硬编码凭据规则、行号与严重级别，以及空暂存区非 nil 结果。测试发现 `redactSecretText` 先 `TrimSpace` 再按原索引切分，缩进行会导致密钥前几位泄露；超长行还会因先截断而跳过脱敏。现改为先按索引脱敏、再裁剪显示，并对越界索引做保护。另补充 `GetRecentRepos` 有效路径过滤断言。测试函数 83 → 84，覆盖率 67.9% → 68.5%，`go test`、`-race`、`go vet`、`go build` 全部通过。
- **无 git-lfs 降级与只读保护测试（2026-10-22）**：`GetLfsTrackedPatterns` 在 git-lfs 缺失时改为从 `.gitattributes` 解析 `filter=lfs` 规则，LFS 页签不再空列表。新增 `TestLfsTrackedPatternsFallbackAndLockablePermissions` 覆盖回退规则、`SyncLockableFilePermissions` 未启用/启用保护时的 0644/0444 切换、保护文件清单与非锁定文件不受影响。测试函数 84 → 85，覆盖率 68.5% → 69.0%，`go test`、`-race`、`-shuffle`、`go vet`、`go build` 全部通过。server 模式浏览器实测：含 `.gitattributes` LFS 规则但未安装 git-lfs 的仓库可正常展示 `*.uasset` / `*.fbx` 跟踪规则；无远端仓库的 Remotes 页签显示空状态，控制台无错误。
- **全界面 UI 冒烟（2026-10-22）**：server 模式依次打开工作区、历史、分支、储藏、评审、工具箱、设置 7 个主视图，13 个工具子页签（含工作树徽标、Bisect、发布向导），7 个设置分区，以及多仓总览与命令面板；所有视图均正常渲染、无空面板，`window.error` / `unhandledrejection` / `console.error` / `console.warn` 收集结果均为 0。弹窗 Esc 关闭与焦点行为正常。验证仓库保留未提交改动，覆盖脏工作区与无远端场景。
- **提交与丢弃交互 E2E（2026-10-22）**：在一次性仓库中通过 Workspace 点击「全部暂存」→ 输入提交说明 → 提交，磁盘历史新增 `feat: e2e commit`，工作区变干净且无控制台错误；随后修改跟踪文件并新建未跟踪文件，点击刷新 → 「全部丢弃」，磁盘与 UI 均恢复干净，未跟踪文件被移除。验证了提交、状态刷新、混合丢弃的完整用户路径。
- **分支创建/清理交互 E2E（2026-10-22）**：通过 Branches 页「新建分支」弹窗创建 `e2e-branch`，弹窗关闭、Toast 显示成功、磁盘分支存在且无控制台错误；随后点击「清理已合并分支」，UI 分支列表与磁盘均恢复为仅 master，验证了创建与安全批量清理的完整链路。
- **Push 参数绑定修复与同步 E2E（2026-10-22）**：浏览器实测头部「快速推送」报 `GitService.Push expects 4 arguments, got 3`——`gitService.push` facade 漏传 `force`，导致快速推送始终失败。已补齐 `force=false` 默认参数；修复后头部 Push 成功将本地提交推到裸远端、ahead 归零；再从第二克隆推送新提交并点击 Pull，工作区正确出现 `remote2.txt`、历史更新且零控制台错误。另用脚本对 315 个 facade 调用与 Go 方法做参数数量对账，修复后全部一致（无缺失方法、无 arity 错位）。
- **Stash 交互 E2E（2026-10-22）**：通过 StashView「新建储藏」弹窗填写说明并确认，Toast 显示「工作区改动已成功保存至储藏箱」，磁盘 `git stash list` 出现对应快照、工作区变干净；随后点击「弹出 (Pop)」，Toast 显示已应用并移除，`base.txt` 与未跟踪文件恢复、储藏列表清空，零控制台错误。
- **逐块暂存/丢弃 E2E（2026-10-22）**：构造同一文件两个独立 hunk，通过 Workspace 点击第一个「暂存此块」，`git diff --cached` 仅包含第一个 hunk、工作区保留第二个，Toast 显示「已暂存所选差异块」；再点击剩余「丢弃此块」，工作区第二个 hunk 被还原、暂存区仍仅保留第一个 hunk，Toast 显示「已丢弃所选差异块」，零控制台错误。
- **冲突裁决与合并完成 E2E（2026-10-22）**：构造真实 `UU` 合并冲突后打开 Workspace，UI 显示合并横幅、冲突计数与裁决助手；点击「采用当前版本 (Ours)」后文件内容恢复为 main、`git diff --diff-filter=U` 为空、冲突标记消失，Toast 正确提示；再点击「完成合并提交」，生成 `Merge branch 'side'` 合并提交、工作区干净，零控制台错误。
- **设置页写入 E2E（2026-10-22）**：外观切换深色/浅色/跟随系统时 `data-theme`、`html` class 与 `wgit-theme` 同步正确；在 Git 身份认证中保存局部 user.name/user.email，Toast 与 `git config --local` 一致；代理保存后 `http.proxy`/`https.proxy` 写入，清空保存后配置被 unset，零控制台错误。
- **历史 Revert / Reset E2E（2026-10-22）**：提交面板点击 Revert，Toast 显示成功，磁盘新增 `Revert "feat: feature"` 提交且 `feature.txt` 被移除；选择基线提交打开「重置分支」，切换 `--hard` 单选项并确认，Toast 显示已硬重置，磁盘仅保留基线提交且工作区干净，零控制台错误。
- **Git Hook 启停 E2E（2026-10-22）**：维护页识别可执行 `pre-commit` 为「已启用」；点击禁用后文件重命名为 `pre-commit.disabled`、UI 与 Toast 同步；再点击启用后文件恢复为 `pre-commit` 且保留 `0755` 可执行权限，零控制台错误。
- **.gitignore 体检/修复与综合体检 E2E（2026-10-22）**：无 .gitignore 的 Unity 工程被识别为 Unity 3D、健康度 30、缺失 8 条规则；点击「一键修复忽略规则」后生成 9 条 Unity 规则、健康度升至 100，Toast 正确；随后检查 `Library/Cache/data.bin` 正确显示被 `.gitignore:3 ([Ll]ibrary/)` 忽略。维护页「运行综合体检」返回 94/100、A 级、5 通过 / 1 警告 / 0 失败，零控制台错误。
- **暂存区安全/大文件扫描 E2E（2026-10-22）**：暂存含 AWS Access Key 的 `secret.txt` 后执行「开始扫描」，UI 显示 CRITICAL 规则、行号与 `AKIA****3456`，DOM 中不含完整密钥；暂存 6MB `big.bin` 后默认 20MB 阈值不告警，调整阈值至 5MB 再检查正确提示「发现 1 个超过 5 MB 且未使用 LFS 的暂存文件」，零控制台错误。
- **Quick Open 文件/代码搜索 E2E（2026-10-22）**：`⌘P` 打开快速索引，输入 `beta` 后回车成功切换到工作区并展示 `beta.txt` 内容；`⌘⇧F` 打开全仓代码搜索，查询 `beta-content` 返回 `beta.txt:1`，回车后同样定位到源码，弹窗正常关闭，零控制台错误。
- **命令面板执行 E2E（2026-10-22）**：`⌘K` 打开面板，搜索「管理本地与远程分支」并回车，成功切换到 Branches 视图；再次打开面板搜索「暂存所有改动」并回车，磁盘索引显示 `M base.txt` + `A new.txt`，证明导航类与操作类命令均可执行，零控制台错误。
- **空目录 / .gitkeep E2E（2026-10-22）**：`empty/a`、`empty/b` 被扫描为 2 个空目录；点击全选后按钮变为「生成 .gitkeep (2)」，执行后两目录均生成 `.gitkeep` 并进入暂存区，重新扫描显示 0 个空目录，零控制台错误。
- **工作树主/联动标识修复（2026-10-22）**：macOS 下主工作树返回 `/private/tmp/...`，与仓库路径 `/tmp/...` 字符串不等，导致主工作树被误判为 linked、显示必然失败的「移除」按钮。`WorktreeInfo` 新增 `isMain`，由 `git worktree list` 首个条目确定；UI 改用 `isMain` 显示「当前主工作区 (Main)」并隐藏移除/锁定/新标签操作。测试断言首项 `IsMain`；浏览器复测主工作树标识正确且无移除按钮，linked 工作树显示 `Linked Worktree` 与唯一移除按钮，移除成功。
- **子模块移除残留清理（2026-10-22）**：移除最后一个子模块时 `git rm` 会留下空 `.gitmodules` 并暂存为新增文件。`RemoveSubmodule` 现检测文件内无 `[submodule` section 时删除该文件并暂存删除；测试断言文件不存在且无暂存残留。UI 复测添加再移除后 `.gitmodules` 已消失、`git status` 干净、页面回到「未注册任何 Git 子模块」空状态，零控制台错误。
- **远程仓库管理 E2E（2026-10-22）**：通过远程页添加 `origin → https://example.com/a.git`，Toast 与 `git remote -v` 一致；编辑弹窗将名称改为 `upstream`、URL 改为 `b.git`，保存后磁盘 fetch/push URL 同步更新；点击移除后 Toast 正确、`git remote` 数量为 0、页面回到空状态，零控制台错误。
- **稀疏检出重置修复（2026-10-22）**：点击「取消限制 / 恢复检出全部文件」时前端传入 `/*`，在 cone 模式下触发 `fatal: specify directories rather than patterns`。现前端传空数组，后端空列表改为执行 `git sparse-checkout disable`；无活动目录时按钮禁用。测试断言禁用后全量文件恢复；浏览器复测应用 `src` 后重置成功、`docs/readme.md` 恢复、稀疏列表为空、Toast 正常、零控制台错误，覆盖率 69.0% → 69.1%。
- **Subtree UI E2E（2026-10-22）**：通过工具页添加 `libs/lib → /tmp/.../lib#master` 的 squash 子树，目录、合并提交与 `.git/wgit-subtrees.json` 元数据正确；上游新增提交后点击「拉取更新」，`lib.txt` 更新为 `lib-v2`；在 main 修改子树并提交后点击「推送回上游」，lib 仓库 `master:lib.txt` 正确变为 `from-main`，三次操作 Toast 均正确、零控制台错误。
- **Blame / 文件历史 E2E（2026-10-22）**：两提交（Alice、Bob）修改同一文件后，打开源码详情点击「Blame 追溯」，逐行显示 Alice/Bob 与 `line2-changed`；点击「文件历史」正确列出 `feat: second` 与 `feat: first` 及版本恢复入口，零控制台错误。
- **变更评审 E2E（2026-10-22）**：以 `master…feature` 创建评审会话，头栏显示 +2/-1 与 2 个文件；标记 `base.txt` 通过后进度变为 50%、历史会话与文件状态同步；添加行级评论后显示作者、`新 L1` 与未解决计数，零控制台错误。
- **发布向导 E2E（2026-10-22）**：发布前检查正确识别 2 个待发布提交并建议 `v0.1.0`；关闭「创建后推送」后点击创建，磁盘生成附注标签 `v0.1.0`（说明 `Release v0.1.0`）；UI 立即刷新为「上次发布 v0.1.0、待发布 0」，并因无新提交显示阻塞提示，零控制台错误。
- **Bisect 结束态兼容修复（2026-10-22）**：Git 2.55 的 `git bisect log` 输出 `# first 'bad' commit: [...]`（bad 带引号），前端只匹配无引号形式，导致定位完成后仍显示 Good/Bad 按钮、不展示结果横幅。现将正则兼容两种写法，统一提取首个坏提交哈希。E2E：4 提交下启动 bisect、标记一次 good 后正确显示「已定位到首个坏提交」与 hash `f0410c3`；点击「重置并结束 Bisect」后回到 master、`app.txt` 恢复 v4、`git bisect log` 显示未在二分，零控制台错误。
- **提交面板标签 E2E（2026-10-22）**：在 History 选中提交点击「标签」，弹窗输入 `v1.0.0` 创建，Toast「已成功打上标签」；磁盘 `git tag -n` 与 `refs/tags/v1.0.0` 均指向目标提交，界面标签列表同步，零控制台错误。
- **Git Notes E2E（2026-10-22）**：History 提交批注面板输入 `QA verified E2E note` 保存，Toast 成功且 `git notes show HEAD` 返回正文；点击删除并确认后 UI 恢复「暂无批注」、`git notes` 已无该提交注解，零控制台错误。
- **标签删除列表刷新修复（2026-10-22）**：删除本地标签后磁盘已无 ref，但列表仍显示旧标签并持续到页面重载。删除/批量删除成功后现先对本地 `tags` 做乐观过滤，再执行后台刷新；E2E 复测删除 `v3.0.0` 后标签计数立即归零、随后显示「暂无标签」，Toast 正常，零控制台错误。
- **文件历史恢复 E2E（2026-10-22）**：两版本文件中打开「文件历史」，对 `feat: v1` 点击「恢复此版本」并确认；Toast 显示已还原至 `1c45267`，磁盘 `app.txt` 变为 v1 并进入暂存区（`M app.txt`），工作区其他文件不受影响，零控制台错误。
- **提交模板 E2E（2026-10-22）**：Workspace 填写提交说明后点击「存模板」，Toast 显示已保存，磁盘 `.gitmessage` 与 `commit.template` 均写入；清空输入后点击「模板」，输入框恢复模板内容并提示「已载入提交模板」，零控制台错误。
- **归档 / 补丁 / Bundle E2E（2026-10-22）**：维护页指定输出目录 `/tmp/wgit-export-out` 后，依次导出 ZIP（`wgit-export-e2e-HEAD.zip`）、patch（`HEAD.patch`）与 Bundle（`wgit-export-e2e.bundle`），三个文件均落盘且 Toast 路径正确；点击「校验 Bundle」返回「Bundle 校验通过」，零控制台错误。
- **多仓总览 E2E（2026-10-22）**：同时打开两个带本地裸远端的仓库，总览显示 2 个仓库、均为 `master / 已同步 / 工作区干净`，当前仓库统计 1 commit / 2 branches / 0 tags / 1 contributor；点击「全部 Fetch」后两仓更新时间刷新，提示已请求全部 Fetch，零控制台错误。
- **引导页克隆 E2E（2026-10-22）**：无本地仓库状态进入欢迎页，选择「克隆远程仓库」→ 完整克隆，填写本地源路径与目标目录后点击开始克隆；目标目录生成完整仓库、提交与 `hello.txt` 正确，应用自动打开新仓库并展示工作区，零控制台错误。
- **本地工程扫描 E2E（2026-10-22）**：欢迎页输入扫描根目录后点击「扫描」，正确列出 `alpha` / `beta` 两个 Git 工程及分支；点击 alpha 结果后应用自动打开该仓库，源码列表显示 `alpha.txt`，零控制台错误。
- **标签远传 E2E（2026-10-22）**：Branches 标签页点击「推送」，Toast 成功且裸远端出现 `refs/tags/v1.0.0`（含解引用提交）；点击「删远程」并确认后远端标签消失、本地标签仍保留，零控制台错误。
- **分支双向比对 E2E（2026-10-22）**：master 与 feature 各有一个独有提交时，比对面板显示「领先 1 个提交 / 落后 1 个提交」，并分别列出 `feat: feature`、`feat: master` 及作者与时间；「查看差异详情」显示 `master..feature` 的 2 个文件（`base.txt +1 -1`、`feature.txt +1 -0`）与完整补丁预览，与 `git log --all` / `git diff` 输出一致；重复点击「重新计算」结果稳定，本轮零控制台错误。
- **自定义操作 E2E（2026-10-22）**：在 Studio 协作页通过「编辑配置」将操作替换为 `echo wgit-e2e-ok` 并保存，Toast 成功且磁盘 `.git/wgit-actions.json` 内容与编辑器一致；点击「执行」确认后结果面板显示 `SUCCESS`、`echo wgit-e2e-ok`、`exit 0` 与输出 `wgit-e2e-ok`，仓库文件未被改动。围绕保存+执行的完整流程复跑 15 次 runtime RPC 均成功；首次页面重载时出现的一次 `/wails/runtime` 422 未能在后续三次重载/复跑中复现，判定为加载竞态的一次性请求，不涉及该功能调用。
- **Reflog 恢复 E2E 与文案修复（2026-10-22）**：三提交仓库硬重置到 v1 后，Reflog 页正确列出 `HEAD@{1} commit: feat: v3`；点击「恢复为新分支」创建 `rescue`，磁盘分支指向丢失提交 `c5d562a`、`app.txt` 仍为 v1。实测发现弹窗 CTA 写「创建并检出」、Toast 写「创建并恢复」，但后端仅执行 `git branch`、当前分支保持 master。保留更安全的仅创建语义，将 CTA 改为「创建分支」、Toast 明确「当前分支保持不变」；重建前端后从同一记录创建 `rescue2`，确认哈希正确且仍在 master，零控制台错误。另点击 `HEAD@{2}` 的「检出」，UI 进入 `HEAD (no branch)`、磁盘 HEAD 为 `34de83b`、`app.txt` 恢复 v2，Toast 正确，零控制台错误。
- **Cherry-Pick E2E（2026-10-22）**：master 与 feature 分叉、feature 独有提交 `feat: feature-cherry` 时，History 默认全引用视图正确展示该提交；选中后点击检视器「Cherry-Pick」，Toast 显示已成功拣选 `6f8d7d6`，master 生成新提交 `e10f2a2 feat: feature-cherry`、`cherry.txt` 落盘且工作区干净。随后以 Network 监听重载页面，8 次 runtime RPC 无失败；此前新标签页立即 reload 偶发的 `/wails/runtime` 422 为在途请求被中止的测试竞态，不涉及业务调用。
- **非 origin 远程分支检出修复（2026-10-22）**：远程页此前把 `upstream/topic` 直接传给 `git checkout`，Git 会检出该远程引用并进入分离 HEAD，而 Toast 仍声称已检出分支；后端还仅对 `origin/` 前缀使用会重置本地分支的 `-B`。现 `CheckoutBranch` 优先复用同名本地分支，识别任意 `refs/remotes/<name>`，按 `git remote` 最长前缀解析远程名与本地名，不存在本地分支时执行 `git checkout --track`；前端传递完整远程引用并在检出后以 `isCurrent` 实际分支名提示与切换。新增 `TestCheckoutRemoteBranchFromNonOriginRemote` 覆盖本地跟踪分支创建、`upstream/topic` 上游绑定及重复检出保留本地独有提交。浏览器 E2E：远程列表含 `upstream/main`、`upstream/topic`，点击 topic 行后 Toast 为「已检出到分支 topic」、顶栏显示 topic、磁盘当前分支 topic 且上游 `upstream/topic`、无分离 HEAD，零控制台错误；全量 86 测试通过，覆盖率 69.1%。
- **压缩合并（Squash Merge）修复（2026-10-22）**：合并弹窗承诺「将所有改动压为一个新提交」并收集提交说明，但 `git merge --squash` 仅暂存改动且忽略 `-m`，UI 仍提示合并成功，实际 HEAD 未前进。现后端在 squash 成功且提供提交说明时检查暂存区并自动 `git commit -m`，无说明时保留原有仅暂存语义。新增 `TestSquashMergeCreatesSingleCommitWithMessage` 断言 base+1 个提交、消息正确、工作区干净。浏览器 E2E：选择压缩提交并填写 `feat: e2e squash`，Toast 成功、弹窗关闭、磁盘 HEAD 为该消息、提交数 2、`one.txt`/`two.txt` 均在且工作区干净，零控制台错误。全量 87 测试通过，覆盖率 69.1%。
- **批量 Cherry-Pick E2E 与结果态修复（2026-10-22）**：feature 独有 `feat: one`、`feat: two` 两条提交，勾选后对话框按从旧到新列出 `c80d3f1`、`70669a7`；确认后 Toast「已成功应用 2/2 条提交」，master 依次生成两条新提交、两文件落盘、工作区干净。实测发现完成后弹窗仍在正文显示「下列 0 条提交…」和旧操作说明；现将说明与提交列表仅在未产生结果时展示，底部标签在结果/错误态改为「操作结果」。重建后复测结果弹窗仅显示「完成 / 成功 2 共 2 / 操作结果 / 关闭」，零控制台错误。
- **子模块远端同步 E2E（2026-10-22）**：父仓库通过 `submodule add -b main` 挂载本地裸远端子模块，上游新增 `feat: lib v2` 后点击「拉取最新远端」；本地 file 远程受 Git `protocol.file.allow` 保护（非 wgit 缺陷），在测试用全局配置显式允许后调用成功。Toast 显示 checked out `c015ef5`，UI 状态由 SYNCED 变为「提交已变更 (MODIFIED)」、基准 Commit 更新为 `c015ef55cf`，父仓库 `git status` 出现 ` M libs/lib`、子模块 HEAD 为 `c015ef5`，零控制台错误。全量 87 测试基线不变。
- **清理已合并分支 E2E（2026-10-22）**：main 已 no-ff 合并 `merged-topic`，另有未合并 `unmerged-topic`。分支洞察显示「已合并 1」与可清理提示；点击「清理已合并分支 (1)」并确认后，Toast「已清理 1 个已合并分支」，磁盘仅删除 `merged-topic`，当前 `main` 与 `unmerged-topic` 保留，列表计数与已合并统计归零，零控制台错误。
- **工作树锁定/解锁 E2E（2026-10-22）**：为 linked worktree `topic` 填写锁定原因 `e2e lock` 并确认，Toast 成功，`git worktree list --porcelain` 出现 `locked e2e lock`，UI 显示「1 处锁定」与原因徽标；点击解锁后 Toast「工作树锁定已解除」，porcelain 中 locked 行消失、UI 计数归零，主工作区仍显示 Main 且无锁定/移除入口，零控制台错误。
- **从储藏创建分支 E2E（2026-10-22）**：工作区改动存入 `e2e stash` 后，在储藏项点击「分支」打开弹窗，名称预填 `stash-0-recovery`；确认后 Toast「已基于储藏快照创建并检出分支」，当前分支切换为 `stash-0-recovery`、`base.txt` 恢复为储藏内容、`git stash list` 清空，储藏箱 UI 回到「暂无储藏快照」。工作区保留未提交改动，符合 `git stash branch` 语义，零控制台错误。
- **标签检出回归修复（2026-10-22）**：标签检出后 `git branch -a -vv` 会输出 `* (HEAD detached at v1.0.0)` 伪行，`GetBranches` 将其解析为当前分支 `(HEAD`，导致本地分支计数虚增、Toast 显示 `(HEAD`，且 `switch-branch` 二次检出 `(HEAD` 报 pathspec 错误。现解析时跳过 `* (HEAD…` / `* (no branch…)`（含 `+` 工作树前缀）伪条目；新增 `TestGetBranchesExcludesDetachedHeadPseudoBranch`。浏览器复测：检出 `v1.0.0` 后 Toast 正确、顶栏 `HEAD (no branch)`、内容恢复 v1、本地分支计数 1、无 pathspec 错误，零控制台错误；全量 88 测试通过，覆盖率 69.1%。
- **工作区清理 E2E（2026-10-22）**：仓库含已跟踪 `tracked.txt`、未跟踪 `junk.txt`/`temp_dir/`、被忽略的 `ignored.log`。工作区清理页预扫描显示 2 个未跟踪项、按钮「彻底清理 (2)」；确认执行后 Toast「已彻底清理 2 个未跟踪垃圾文件」，`junk.txt` 与 `temp_dir/` 被删除，`tracked.txt`、`ignored.log` 与 `.gitignore` 保留，列表归零并显示「工作区干干净净」，工作区状态干净，零控制台错误。
- **范围 Cherry-Pick E2E（2026-10-22）**：main 与 feature 分叉后，History 点击「范围拣选」，输入起点 `main`、终点 `feature` 并确认；按 `git cherry-pick main..feature` 顺序生成 `feat: one`、`feat: two` 两条提交，Toast 返回首条提交摘要，`one.txt`/`two.txt` 落盘、工作区干净，符合「起点不含、终点包含」的界面说明，零控制台错误。
- **挂载新工作树 + 新分支 E2E（2026-10-22）**：在工作树页填写路径与分支 `wt-new`，勾选「创建并检出全新分支 (-b)」（「挂载后自动打开」默认勾选）后确认；Toast 成功，磁盘新增 linked worktree、分支 `wt-new` 在独立目录检出且内容正确，UI 显示 2 处工作树 / 1 处 linked，并自动切换到新仓库标签，零控制台错误。
- **提交报告导出 E2E（2026-10-22）**：History 点击「导出报告」后 Toast「提交报告已复制到剪贴板」；读取剪贴板得到 Markdown 报告，含表头 `# wgit 提交发布报告`、两条提交的作者/日期/说明，提交说明内的 `|` 正确转义为 `\|`，与后端格式一致，零控制台错误。
- **推送前策略检查 E2E（2026-10-22）**：Studio 页点击「推送前检查清单 → 开始检查」，规范提交仓库返回 `PASS (100/100)`；追加一条非规范提交 `bad message without convention` 后复测返回 `PASS (90/100)`，证明提交规范规则真实执行并扣分。检查仅读操作，仓库状态不变，零控制台错误。
- **Pickaxe 内容考古 E2E（2026-10-22）**：提交先加入后移除 `LegendarySword`，History「内容考古」按字面量搜索该字符串，结果头显示「命中 2 次 / 分析 2 次、+1 -1、1 个文件」，两条 hit 分别标记「移除」「新增」，提交摘要、作者、日期与文件 `item.txt` 均正确，零控制台错误。
- **提交签名详情修复（2026-10-22）**：未签名提交的签名弹窗正确显示「未签名 (No Signature)」「%G? = N」与无密钥提示，但「签名时间」误显示提交时间。现 `GetCommitSignatureDetails` 在状态为 `N` 时清空日期，测试断言未签名时 `Date == ""`；浏览器复测四项（签名者/密钥 ID/指纹/签名时间）均为「—」，零控制台错误。全量 88 测试通过，覆盖率 69.1%。
- **标签详情目标提交修复（2026-10-22）**：附注标签的 ref 指向 tag 对象，`GetTagDetail` 原先直接 `rev-parse <tag>`，导致 UI「目标提交」显示 tag 对象哈希 `b7551ac` 而非提交 `6202e78`。现改为 `rev-parse <tag>^{commit}` 解引用；测试断言目标哈希等于 `v1.0.0^{commit}` 且不等于 tag 对象。浏览器复测标签名、附注标签标识、打标时间、Tagger、邮箱、附注说明均正确，目标提交为 `6202e7877e36`，零控制台错误。
- **提交签名设置 E2E 与提示修复（2026-10-22）**：设置页启用签名、选择 SSH 并填写 `~/.ssh/id_ed25519.pub` 后保存，`commit.gpgsign=true`、`gpg.format=ssh`、`user.signingkey` 均正确落盘；停用后 `commit.gpgsign=false` 且格式/密钥保留。实测发现选择 SSH 时仍显示「未检测到 gpg 可执行文件」警告，现将该提示限定为 OpenPGP 格式；重建后回填 SSH 选项与密钥正确、警告消失，零控制台错误。
- **高级 Git 配置 E2E（2026-10-22）**：设置页「高级 Git 配置」正确列出 17 项 local/global 配置与计数；按 `过滤配置键` 输入过滤可收敛列表；关闭全局开关写入 `wgit.e2e.local2=on` 后 Toast 为「已写入仓库配置」，`git config --local` 命中；勾选全局写入 `wgit.e2e.global2=yes` 后 Toast 为「已写入全局配置」，测试 HOME 下 `~/.gitconfig` 命中，零控制台错误。
- **LFS 推送前守卫 E2E（2026-10-22）**：无远端仓库含 1 个本地提交，Git LFS 页点击「开始校验」；返回「可安全推送」，统计为待推送提交 1、LFS 指针 0、缺失对象 0、超大文件 0，汇总「就绪: 1 个待推送提交，0 个 LFS 资产已全部在本地就绪」，Toast 与页面一致，零控制台错误。本地无 git-lfs 时未出现崩溃，仅正常降级为指针/大文件扫描。
- **分支重命名远程同步修复（2026-10-22）**：`RenameBranchWithRemote` 原先硬编码 `origin` 推送/删分支，非 origin 仓库必然失败。现优先读取 `branch.<new>.remote`，回退 origin 或首个远程，并按该远程读取默认分支、推送新分支与删除旧分支；重命名确认文案不再写死 origin。新增 `TestRenameBranchSyncsConfiguredNonOriginRemote`（仅 upstream 远程，断言本地/远端新分支、旧远端分支删除与 tracking remote）。浏览器 E2E：非 origin 仓库勾选「同时重命名远端分支」后 Toast 成功，远端新增新分支并删除旧分支，零控制台错误。全量 89 测试通过，覆盖率 69.1%。
- **Pre-Push 非 origin 范围修复（2026-10-22）**：无上游且远程不叫 origin 时，`AuditPrePushChecklist` 原先回退审计最近 10 条提交，可能把已推送历史纳入评分。现遍历 `git remote`，匹配 `<remote>/<branch>` 后限定为未推送区间；新增 `TestPrePushChecklistUsesNonOriginRemoteRange`（仅 upstream、删除 branch tracking 后仅审计 1 条未推送提交）。全量 90 测试通过，覆盖率 69.1%，`go vet` 通过。
- **外部工具配置读写修复（2026-10-22）**：`GetExternalToolConfig` 从不设置 `IsGlobal`，全局工具配置在 UI 中总显示为仓库级；现按 `diff.tool/merge.tool/自定义命令` 的 `--show-scope` 判断，存在 local 即 false、仅 global 为 true。`SetExternalToolConfig` 对空自定义命令执行 `--unset-all`，清空后不再残留旧命令，并传播写入错误。扩展 `TestRemoteWebInfoAndExternalToolConfigInTempRepo` 覆盖 local/global 判定与清空。浏览器 E2E：勾选全局保存后重新进入设置页，全局复选框正确回显，全局配置含 `diff.tool/merge.tool`，零控制台错误；全量 90 测试通过，覆盖率 69.2%。
- **移除无效存储设置分区（2026-10-22）**：设置页「大文件存储与高级扩展」仅有 `ref` 本地状态，没有任何后端调用、持久化或消费方，属于无效伪设置。已删除该分类与 `StorageSection.vue`，设置导航收敛为 7 个真实分区；浏览器复测导航无该入口，外部工具与高级配置分区仍正常渲染，零控制台错误。前端 `vue-tsc + vite build` 通过。
- **储藏单文件恢复 E2E（2026-10-22）**：一个储藏同时含 `a.txt`、`b.txt` 改动，展开「文件 2」面板后仅点击 a.txt 的「恢复」；Toast「已从储藏恢复并暂存: a.txt」，工作区 `a.txt` 变为储藏内容且已暂存（`M  a.txt`），`b.txt` 保持原状，`git stash list` 仍为 1，说明未误删或整体弹出储藏，零控制台错误。
- **批注推送非 origin 修复（2026-10-22）**：History 推送批注前端默认传 `origin`，仅 upstream 的仓库必然失败。现前端默认留空，后端解析当前分支上游远程、再回退 origin/首个远程；按钮提示由 `git push origin` 改为 `git push <remote>`。扩展 Notes 同步测试（rename 为 upstream 后以默认远程推送）。浏览器 E2E：仅 upstream 仓库点击「推送远端」成功，远端出现 `refs/notes/commits`，零控制台错误。
- **统一默认推送远程（2026-10-22）**：新增 `defaultPushRemote`（当前上游远程 → origin → 首个远程），批注推送与 LFS 推送共用，消除 LFS `git lfs push origin` 硬编码；LFS 按钮提示改为 `<remote>`。新增 `TestDefaultPushRemoteResolution` 覆盖上游 tracking 与无 tracking 回退。全量 91 测试通过，覆盖率 69.2%，`go vet` 与前端构建通过。
- **核心同步非 origin 修复（2026-10-22）**：顶部 Fetch/Pull/Push 的 facade 默认远程由 `origin` 改为空，后端 Fetch 使用 `defaultPushRemote` 解析，Pull/Push 留空时交由 Git 使用当前跟踪；`TestDefaultPushRemoteResolution` 增加非 origin Fetch 断言。浏览器 E2E（仅 upstream）：顶部 Fetch 将 `upstream/main` 更新到最新并显示落后 1，Pull 快进 HEAD 到最新且工作区干净，Push 将本地提交推送到远端 main，三次操作零控制台错误。
- **标签/发布/LFS 默认远程收口（2026-10-22）**：`PushTag`、`DeleteRemoteTag`、`DeleteBatchTags`、`PruneRemote`、维护 prune、`VerifyPrePushLfs`、发布推送与 Studio 离线队列/认证诊断均改用 `defaultPushRemote`；标签 UI 不再显式传 origin，发布与 LFS 预检输入默认留空。`TestDefaultPushRemoteResolution` 扩展标签推送/删除。浏览器 E2E（仅 upstream）：标签「推送」Toast「已推送到远端」，远端出现 `refs/tags/v1.0.0`；「删远程」后 Toast 成功且远端标签消失，零控制台错误。
- **发布向导非 origin 推送 E2E（2026-10-22）**：仅 upstream 仓库进入发布向导，远程名留空、默认「创建后推送」；点击「创建发布标签 v0.1.0」后标签创建并推送成功，结果面板显示「已创建并推送标签 v0.1.0 到 upstream」，远端存在 `refs/tags/v0.1.0`，零控制台错误。远程输入提示同步改为「留空自动选择当前跟踪远程（回退 origin）」。
- **快速提交非 origin E2E（2026-10-22）**：FastCommit 的预抓取由硬编码 origin 改为 `defaultPushRemote`。仅 upstream 仓库修改文件、填写 `feat: fast commit e2e` 后点击「快速提交」并确认，Toast 成功，本地与远端 main 均生成该提交、工作区干净，零控制台错误；成功提示由英文改为中文「快速提交完成：已提交、变基拉取并推送到远端。」。
- **默认上游/远程解析收尾（2026-10-22）**：Ahead/Behind、PushAdvanced、远端连接测试与三个交互变基入口不再默认 origin；新增 `defaultUpstreamRef`（`@{u}` → 默认远程/当前分支 → origin/main），并扩展 `TestDefaultPushRemoteResolution` 断言 upstream-only 仓库返回 `upstream/main`。全量 91 测试、覆盖率 69.2%，`go vet` 通过。
- **最终全界面冒烟（2026-10-22）**：仅 upstream 远程仓库上依次打开工作区、历史、分支、储藏、变更评审、工具箱、设置 7 个主视图，13 个工具子页签，7 个设置分区，全部有实际内容；外部工具页正确显示 `upstream` 远程 URL，伪存储设置分区已不存在。设置前四类因内容较短触发冒烟长度阈值，复核面板文本均正常。仅出现已知的新标签页立即 reload 的一次性 `/wails/runtime` 422，无业务异常。
- **同步弹窗远程去重（2026-10-22）**：`GetRemoteDetails` 按 fetch/push 返回两条同名称记录，SyncModal 下拉直接遍历导致每个远程重复出现。现按远程名去重后再渲染；upstream-only 仓库复测下拉仅一个 `upstream` 且默认选中，零控制台错误。
- **高级拉取默认策略修复（2026-10-22）**：分叉且未配置 `pull.rebase` 时，界面默认「Git 默认」会触发 `fatal: Need to specify how to reconcile divergent branches`，与「自动创建合并提交」描述不符。现将策略收敛为三种显式选项并把默认设为「合并拉取 (--no-rebase)」，文案说明不再依赖全局 pull 策略。E2E：upstream-only 分叉仓库点「执行拉取」后生成 `Merge branch 'main' of ...` 合并提交，本地/远端文件均存在、工作区干净，UI 显示「拉取并合并成功」，零控制台错误。同一环境验证高级推送：目标远程自动选 `upstream`、Ahead 显示 1，点击「确认推送」后远端 main 更新到本地提交、Ahead 归零并显示「推送成功完成」，零控制台错误。
- **变更评审导出 E2E（2026-10-22）**：`main → feature` 两个文件差异的评审会话点击「导出 Markdown」，预览窗显示完整报告（区间、提交、`+2/-1`、待评审文件表），并提示「已复制到剪贴板」；报告内容与评审范围、提交与文件结论一致，零控制台错误。
- **变更评审清除会话 E2E（2026-10-22）**：`main → feature` 会话先通过文件使进度到 100%，点击「清除会话」并确认后进度归零、文件回到「待评审」，评审区间与会话入口保留（符合“仅清除结论与评论”的语义），零控制台错误。
- **真实 git-lfs 3.8.0 E2E（2026-10-22）**：本机 `/opt/homebrew/bin/git-lfs` 3.8.0 可用。构造 `*.bin` 跟踪仓库并提交两个二进制资产，确认提交内容为 LFS 指针、本地缓存各 1 份；删除远端全部 LFS 对象后，LFS 页签「LFS 推送」成功将 2 个对象重新上传，预检显示「1 个待推送提交、1 个 LFS 指针、0 缺失、0 超大文件，可安全推送」。清空本地 `.git/lfs/objects` 并删除工作文件后点「LFS 拉取」，缓存恢复 2 个对象、`second.bin` 恢复为 131072 字节，零控制台错误。
- **Pre-Push 根提交 LFS 漏检修复（2026-10-22）**：无上游且仓库只有根提交时，预检用 `<hash>^..HEAD` 取变更文件，根提交无父导致 diff-tree 失败、LFS 指针全部漏检。现对每个待推送提交执行 `diff-tree --root -m` 收集文件。新增 `TestVerifyPrePushLfsDetectsMissingObjectInRootCommit`；真实 LFS E2E 删除对象与工作文件后，UI 正确显示「存在推送风险」、LFS 指针 1、缺失对象 1、缺失文件 `assets/gone.bin`。
- **Linked worktree 状态与 LFS 路径修复（2026-10-22）**：工作树的 `.git` 是文件，原实现直接拼接 `repoPath/.git`，导致合并/变基/Cherry-Pick 状态与中止/继续/跳过全部失效、hooks 与 Studio/子树状态文件读写失败、LFS 缓存统计为 0 且推送预检误判。现统一通过 `rev-parse --git-dir`（每工作树状态）与 `--git-common-dir`（hooks/config/LFS/仓库级状态）解析；预检对 LFS 对象 stat 失败一律按缺失处理。新增 `TestLinkedWorktreeConflictStateAndHooks` 与 `TestLfsCacheUsesCommonDirInLinkedWorktree`。
- **LFS Prune 结果统计修复（2026-10-22）**：真实 git-lfs 3.8 的 dry-run 输出为 `N file(s) would be pruned (size)`，实际清理只输出进度，原解析期待 `Pruning N files (size)`，导致 UI 始终显示 0 个/0 B。现 dry-run 解析真实摘要并兼容旧格式，实际清理用 `.git/lfs/objects` 前后文件数与体积差值统计。真实 git-lfs 孤儿对象验证：dry-run 与实际清理均报告 1 个对象及回收体积。新增 `TestParseLfsPruneSummary` 与 `TestLfsPruneReportsReclaimedObjects`。
- **LFS 批量追踪与 hooks 自定义路径修复（2026-10-22）**：`TrackExtensionsAsLfs` 原忽略每条 `git lfs track` 的失败，只要 `.gitattributes` 已存在且 `git add` 成功就返回成功；现聚合失败规则并返回错误，仅在实际写入规则后暂存。`GetGitHooks` / `ToggleGitHook` 改用 `rev-parse --git-path hooks`，支持 `core.hooksPath`（Husky 等）与 linked worktree。新增 `TestTrackExtensionsAsLfsStagesPatternsAndLockable`、`TestTrackExtensionsAsLfsReportsGitLfsFailure`、`TestGitHooksRespectCustomHooksPath`。
- **Pre-Push 超大文件判定修复（2026-10-22）**：原实现用工作区文件大小判断 50MB 非 LFS 大文件，若提交中的 60MB 文件被本地改成 1 字节或删除，预检会放行实际将推送的大 blob。现从 `git ls-tree -r -l HEAD` 读取已提交 blob 大小，再对变更文件告警。新增 `TestVerifyPrePushLfsUsesCommittedBlobSize`。
- **LFS 锁活动真实化修复（2026-10-22）**：`GetLfsLocks` 改用 `git lfs locks --json` 读取真实 `locked_at`，旧版回退文本解析；锁活动页原先把持有时间固定为“正常持有”、僵尸计数永远为 0，现按真实时间计算持有天数与 >72h 僵尸锁。冲突检测增加 `-uall`，避免 `git status` 把未跟踪目录折叠成 `Art/` 而漏报队友锁冲突。新增 `TestParseLfsLocksJSON`、`TestParseLfsLocksPlain`、`TestLfsLockDuration`、`TestLfsLockActivityDetectsZombieAndCollision`。
- **Studio CI 真实化（2026-10-22）**：原接口返回固定 5 阶段全 SUCCESS、假历史与假耗时，“触发构建”无条件成功。现通过 GitHub CLI（`gh run list/view`）读取真实运行状态、作者、时长与 jobs 阶段，`gh workflow run` 触发；仅支持 GitHub Actions，其他平台、无远程或缺少 `gh` 时返回诚实提示。UI 移除“内置演示数据”标签，无运行记录时禁用触发。新增 `TestStudioCIUsesRealGitHubActionsData`、`TestStudioCIReportsUnsupportedProvider`；server 模式实测显示 `GitHub Actions 流水线 · main / SUCCESS / CI #12 / build,deploy`，触发实际执行 `gh workflow run CI --ref main -R acme/game`，控制台无错误。
- **TAPD 演示数据清理（2026-10-22）**：移除固定工作项列表与“已推进 TAPD 状态”假提示。`FetchTAPDWorkItems` 返回未接入 OpenAPI 的空状态；`BindCommitToTAPD` 先解析真实提交，再写本地 Git Note，失败会返回错误而不是伪成功；UI 移除演示标签并显示空状态说明，明确关联不会调用 TAPD API。新增 `TestBindCommitToTAPDRejectsUnknownCommit`；server 模式实测空状态文案正确，关联后 HEAD 的 Git Note 为 `TAPD #1002341 Status: resolved (local note; TAPD API not called)`。
- **LFS 单条规则一致性与参数隔离（2026-10-22）**：`TrackLfsPattern` / `UntrackLfsPattern` 原只执行 `git lfs track/untrack`，不暂存 `.gitattributes`，用户可能提交资产却漏提交规则；现成功后同步暂存，并用 `--` 终止选项解析，避免以 `-` 开头的规则被当作参数。新增 `TestTrackLfsPatternStagesAttributes`。
- **LFS 锁报表转义与僵尸判定修复（2026-10-22）**：CSV 导出原用 Go `%q` 生成字段，路径含引号/换行时不是合法 CSV；Markdown 也未转义表格管道符。现 CSV 改用 `encoding/csv`，Markdown 转义 `|` 与换行，并统一按 >72h 判定僵尸锁。新增 `TestRenderLfsLocksReportEscaping`。
- **离线补丁包真实化（2026-10-22）**：`GenerateOfflinePatchBundle` 原不写文件，固定 1 个提交/2 个 LFS/假 SHA；现通过临时 ref 与 `git bundle create` 生成真实 bundle，解析提交范围、LFS 指针清单、真实体积与 SHA256，后端测试用 `git bundle verify` 校验。UI 工具区区分“真实执行/演示数据”，离线补丁包不再提示“演示数据”。新增 `TestGenerateOfflinePatchBundleCreatesVerifiedBundle`；server 模式实测生成 394 B bundle，JSON 返回真实 SHA/提交数，`git bundle verify` 通过。
- **热修复反向合流真实化（2026-10-22）**：`TraceHotfixBackportStatus` 原固定返回 3/3；现用 `merge-base` 计算分支差异，用 `git cherry` 的 patch-id 语义识别已合入/漏合入提交，返回真实缺失提交与 `git cherry-pick -x` 修复命令。UI 将该项标记为“真实执行”。新增 `TestTraceHotfixBackportStatusUsesRealCommits`。
- **隐私合规审计真实化（2026-10-22）**：原缺少 `PrivacyInfo.xcprivacy` 时强制标记为合规，权限、追踪域与评级均为固定值；现扫描实际 iOS 隐私清单、AndroidManifest 权限和追踪域，缺 iOS 清单为 BLOCKING/FAIL，敏感权限为 WARNING，并按真实文件判定 iOS/Android/跨平台/未知工程。新增 `TestAuditPrivacyAndComplianceUsesRealManifests`。
- **单文件 LFS 锁状态修复（2026-10-22）**：`CheckFileLockStatus` 优先使用 `lfs locks --json` 解析真实持有者，旧版回退文本；离线/远端失败不再吞成“未锁定”，而是上抛错误。新增 `TestCheckFileLockStatusUsesJSONAndReturnsErrors`。
- **隐私结论态与变基进度回归（2026-10-22）**：隐私审计返回 `N/A` 时前端不再误显示绿色“未发现隐私合规问题”，改为“未作合规结论”；新增 linked worktree 真实 rebase 冲突测试，验证 `GetRebaseProgress` 的步骤、冲突文件与 abort 清理。server 模式实测无移动端工程显示 `N/A · Unknown` 与中性提示，无绿色通过文案。
- **暂存区大文件检查修复（2026-10-22）**：`CheckStagedNonLfsLargeFiles` 原读取工作区文件大小，暂存大文件后把工作区改小/删除即可绕过；现改用 `git cat-file -s :<path>` 读取索引 blob 大小。新增 `TestCheckStagedNonLfsLargeFilesUsesIndexSize`。
- **DDC 与 Webhook 假状态清理（2026-10-22）**：DDC 面板原固定展示 94.2% 命中率、8GB、14 个过期块、12ms 延迟；现扫描真实 `DerivedDataCache` 体积/文件数/30 天未更新块，“修剪过期”真实删除过期文件，命中率/延迟显示为未采集。Webhook 面板移除默认样例机器人与伪造“已推送”文案，只展示真实本地配置并在空列表时明确提示；短 URL 也完整掩码，仅接受 http(s) 地址，事件规则按输入保存。新增 `TestManageStudioDDCUsesLocalCache`、`TestManageStudioWebhooksStoresRealConfig`。
- **Git 认证诊断超时保护（2026-10-22）**：原 SSH 探测只有 ConnectTimeout，无整体命令期限，DNS/网络异常时 UI 可能长时间等待；现增加 8s 上下文超时并返回明确“SSH 诊断超时”结果。新增 `TestDiagnoseGitAuthHonorsSSHTimeout`。
- **凭据保险库脱敏修复（2026-10-22）**：原仅匹配 `user:password@`，`https://ghp_xxx@host` 这类令牌当用户名的 URL 会把明文令牌显示在扫描结果中；现统一遮蔽 URL userinfo 与 token/password/secret 等 query 参数，并覆盖 query 内嵌凭据检测。新增 `TestCredentialVaultMasksTokenStyleURLs`。
- **凭据保险库权限与损坏保护（2026-10-22）**：写入后强制 `chmod 0600`，修复已存在的 0644 vault 继续以宽权限保存；损坏的 JSON 不再被静默覆盖，返回错误并保留原文件。新增 `TestStoreCredentialSecretSecuresVaultAndRejectsCorruption`。
- **离线队列在线状态真实化（2026-10-23）**：原 `ls-remote --get-url` 只解析 URL、不访问远端，离线时也显示在线。现新增 `ExecGitWithTimeout`，用 5s 超时真实执行 `ls-remote --heads`，区分“未配置远端 / 远端不可达 / 在线”。新增 `TestOfflineQueueReportsRealRemoteConnectivity`。
- **远端连通性测试超时（2026-10-23）**：`TestRemoteConnection` 原使用默认 60s 期限，UI 在弱网/不可达远端会长时间等待；现改为 15s 超时，且 `ExecGitWithTimeout` 对 deadline 返回明确的 `git command timed out` 错误。新增 `TestRemoteConnectionHonorsTimeout`。
- **Unity 资产循环依赖真实化（2026-10-23）**：`DetectAssetCyclicDependencies` 原只统计文件数、从不检测闭环；现基于 `.meta` GUID 建立 Prefab/Material/Asset 引用图并 DFS 检测循环，返回真实路径、长度与风险等级；无 `.meta` 时不再伪造 12 个扫描项。新增 `TestDetectAssetCyclicDependenciesUsesUnityGUIDs`。
- **字体版权审计真实化（2026-10-23）**：原固定返回 Source Han Sans 的 OFL 结论；现扫描 TTF/OTF/TTC，解析 sfnt `name` 表（family/copyright/license/URL），按许可文本区分 Commercial_Safe / Restricted / Unknown，并汇总风险等级。新增 `TestAuditFontLicensesReadsFontNameTable`。
- **Studio 拓扑路径去重（2026-10-23）**：macOS 下 `/tmp` 与 `/private/tmp` 未 canonicalize，导致主仓库同时被列为 MainWorktree 和 AdjacentRepo（UI 显示 2 工程）。现用 `EvalSymlinks` 归一化去重，同时保留用户输入路径作为 ActiveProjectPath。新增 `TestStudioSwitchboardDeduplicatesSymlinkedRepos`；server 模式实测恢复为 `1 工程 · 0 工作树 · 0 邻接仓库`，DDC 修剪真实删除 1 个过期文件并刷新统计。
- **Shader 自愈真实化（2026-10-23）**：原实现固定返回 “Unity URP / 2 个损坏变体 / 已清理缓存” 的样例。现真实识别 Unity URP/HDRP/Built-in、Unreal、Godot 工程；解析 shader `#include` 相对/仓库/Assets/Packages 路径，检查 Unity `.mat` 的 `m_Shader` GUID 是否存在于 `.meta` 集合（跳过内置 shader GUID）；`CLEAR_SHADER_CACHE` / `HEAL_ALL` 仅删除真实存在的 `Library/ShaderCache`、`Library/ShaderCompilationCache`、`Temp/ShaderCache`、`Saved/ShaderCache`，删除失败返回错误。黑屏网格数明确标注需运行时验证，不再伪造。前端在执行清缓存前新增确认框。新增 `TestShaderSelfHealingUsesRealIncludesMaterialsAndCache`。
- **PBR 贴图通道审计真实化（2026-10-23）**：原返回固定的 Hero_ORM/Chest_Normal 样例与 100% 合规。现扫描 PNG/JPEG/TGA 文件头（宽高、通道数、色彩类型、PNG sRGB 块），按命名约定识别 ORM/MRA 打包、独立 AO/Roughness/Metallic 贴图与法线图；灰度法线或带 sRGB 的法线判为异常，Gamma 空间打包数据判为不合规，独立通道图按未压缩 8-bit 未使用通道估算显存浪费；支持目录范围、忽略 Library/Temp 等缓存目录，空仓库明确返回“未发现”。前端移除演示标签并新增目标目录说明。新增 6 个真实二进制头部测试（PNG/JPEG/TGA/sRGB）。
- **Wwise / FMOD Bank 差分真实化（2026-10-23）**：原固定返回 `Play_BGM_MainTheme` 等 5 个事件、84.5s→92.0s 时长和 409600 字节伪目标体积。现按二进制头识别 Wwise `BKHD` / FMOD `FSB5`，解析 Wwise HIRC 对象 ID 与载荷大小并做真实新增/移除/变更差分，FMOD 报告真实样本数；体积、删除/新增状态均来自 `git show` 读取的 Bank 字节，两侧都不可读时返回错误，超大 WAV/PCM/WEM 扫描跳过缓存目录。事件时长、循环与孤立事件明确标注需厂商工具，不再伪造。另修复 `formatByteSize` 对负增量丢失单位的问题。新增 `TestAuditAudioBankDiffParsesWwiseHirc`、`TestAuditAudioBankDiffReportsMissingTarget`、`TestAudioBankParsesFMODHeaderAndNegativeByteSizes`。
- **AssetBundle 差异真实化（2026-10-23）**：原固定返回 hero/ui 两个 Bundle 与 +1.20 MB 增量。现解析 `bundles` 数组、`名称 → 体积/哈希/资产列表` 映射及 Unity `.manifest` 的 `Hashes`/`Assets` 段，按名称比对新增/修改/移除，首包增量取清单体积总和差、补丁下载按新增与修改的整包体积估算；同时检测跨 Bundle 重复资产（资产体积未知时明确为 0）。清单缺失、为空或格式不支持时返回错误，不再回退样例。新增 `TestAnalyzeBundleDiffParsesJSONManifests`、`TestAnalyzeBundleDiffSupportsNameMapping`、`TestParseUnityBundleManifestHashesAndAssets`、`TestAnalyzeBundleDiffRejectsMissingManifest`。
- **光照烘焙审计真实化（2026-10-23）**：原固定返回 SM_CastleWall/SM_Fountain 两个网格与 8 分钟/100 分。现解析 Unity `.unity` 的 `LightmapSettings` 与 `.lighting` 的 `LightingSettings`（场景优先、无场景时回退资产），读取 Bake Resolution、Atlas、Lightmaps Mode、纹理压缩与 Baked Lightmaps 开关；按分辨率 >100/>200、Atlas >4096、未压缩生成 WARNING/CRITICAL 并扣分，按 Atlas 面积粗略估算烘焙分钟数。UV2 重叠无法从 YAML 判定，明确标注“需导入器级解析，未评估”，不再伪造网格。新增 `TestAuditLightmapBakeSettingsParsesUnityScenes`、`TestAuditLightmapBakeSettingsFallsBackToLightingAsset`、`TestAuditLightmapBakeSettingsEmptyAndMissingPath`。
- **World Partition 数据层差异真实化（2026-10-23）**：原固定返回 `DL_Environment_Buildings` / `DL_Gameplay_NPCs` / `DL_Cinematics_BossIntro` 与虚构 Actor。现通过 `git ls-tree` 读取基线与目标的 `DL_*.uasset` / `*/DataLayers/*.uasset`，按 blob 哈希真实判断新增/修改/移除；Actor 归属、运行时状态与流式预算明确标注需 UE Editor/Python 导出，不再伪造。新增 `TestDiffWorldDataLayersUsesTrackedAssets`、`TestDiffWorldDataLayersEmptyRepoIsHonest`；至此 10 个引擎诊断工具全部为真实实现，工具区标题/徽标同步改为「全部真实执行」。server 模式实测该区域显示「全部真实执行」，光照烘焙、PBR 贴图、World 数据层、Bundle 差异与音频 Bank 五个工具均返回真实 JSON（如 1 处 Unity 光照配置 / 1 张 ORM / 2 个 Data Layer / 3 个 Bundle 变化 / HIRC 对象 2→2），控制台零错误。
- **能力清单去伪认证（2026-10-23）**：120/140 轮 Hub 原返回 `PRODUCTION_CERTIFIED` / `ENTERPRISE_PINNACLE_CERTIFIED`、固定测试数与工具数、基于固定文本的伪 `ReleaseSignature`，并宣称已接入 TAPD OpenAPI、T-Build/工蜂 CI、局域网 DDC 同步等未实现能力。现统一改为 `SELF_REPORTED_STATIC`，徽标明确 non-certification，测试/工具计数与签名清空，架构说明改为 Go + Wails v3；清单中的 TAPD、CI、DDC、Cocos、音频条目改为真实边界（本地 Notes / GitHub Actions / 本地 DDC / 工程识别）。前端页脚补充“不代表第三方认证或生产就绪”。新增 `TestParityHubsAreExplicitlySelfReported`。
- **LOD / 串流与 OBJ 面数修复（2026-10-23）**：OBJ 三角形计数原用 `len(parts)-2`，每个面多算 1 个三角形；`Tri_LOD1.obj` 等 LOD 变体同时被当作基础网格重复审计；非 OBJ/glTF 网格按文件大小除以 64 猜测面数，纹理头解析失败时假定 1024×1024。现统一修正为 `len(parts)-3`，跳过已有基础文件的 LOD 变体，新增 glTF/GLB 真实三角形解析（索引或 POSITION accessor），FBX/UAsset 等不可直接解析格式返回 `triangleCountKnown=false` 且不按猜测面数评级；纹理尺寸未知时标记 `dimensionsKnown=false` 并从显存估算/评分中排除。原生插件 ABI 审计在未发现插件时不再输出“全部对齐兼容”。新增 `TestAuditLodStreamingUsesRealMeshAndTextureHeaders`、`TestCountMeshTrianglesReadsGLTFAndGLB`、`TestAuditNativePluginsAbiEmptyRepoIsHonest`。
- **模型面数与音频元数据去估算（2026-10-23）**：模型面数体检对 FBX 等格式按文件大小除以 55/75 估算三角形与顶点数，glTF 取最大 accessor 再除以 3，均可能误判预算；现统一解析 OBJ 顶点/面/分组/材质与 glTF/GLB 的 POSITION/indices accessor，FBX/UAsset 等返回 `triangleCountKnown=false`，前端显示「未知」且不做预算判定。音频体检对 MP3/OGG/FLAC 与损坏 WAV 回填 44100Hz/双声道并误报 44.1kHz 警告；现仅 WAV fmt 解析成功时标记 `metadataKnown=true`，其余显示「未知」并明确未作 48kHz 判定。新增 `TestLintModelMeshUsesRealGeometry`、`TestInspectAudioFileDoesNotFabricateCompressedMetadata`。后续补充无效 glTF accessor 引用判定：引用越界时返回未知而非 0 面/OK。server 模式实测 FBX 显示 UNKNOWN/未知、合法 glTF 显示 1 面/9 顶点、MP3 显示采样率未知、动画 glTF 显示 3 骨骼/30fps/1 轨道，控制台零错误。
- **动画骨骼体检去伪数据（2026-10-23）**：原 glTF 片段固定 2.5s/30fps、骨骼深度按 `jIdx/4` 生成，FBX 无法解析时按文件体积伪造 24~128 根 `Bone_XXX` 与 `Take 001` 片段。现解析 glTF/GLB 的 skin joints、children 层级、animation sampler input accessor 的 min/max/count，得到真实骨骼深度、时长、采样率与轨道数；FBX 从 `Model::` 名称表提取节点，解析不到时返回 `analysisKnown=false`/`UNKNOWN`，不再生成任何骨骼或片段；蒙皮影响数未解析时显示「—」而非默认 4。新增 `TestLintAnimationRigUsesRealGLTFHierarchyAndClips`、`TestLintAnimationRigFBXUsesRealNodeNamesAndNoFakeFallback`。同时修正动画体检优先读取 HEAD、工作区修改不生效的问题，改为优先读取工作区文件；新增 `TestLintAnimationRigReadsWorktreeOverHEAD`。
- **材质实例覆盖审计真实化（2026-10-23）**：原逻辑检测到 `_MainTex`/`_BumpMap` 就填入固定的 Hero 贴图路径，并用 `_SpecColor` 模拟失效参数，无材质时还会伪造 `MI_Hero_Armor`。现解析 Unity `.mat` 的 `m_SavedProperties`（纹理/浮点/颜色/整数）、`m_Shader`/`m_Parent` GUID，借助 `.meta` 索引解析真实贴图与父材质路径，并用 Shader `Properties` 集合判定悬空参数；`m_Texture: {fileID: 0}` 显示 `None`，UE `.uasset` 二进制明确跳过并计数，空仓库不再生成样例。新增 `TestAuditMaterialInstanceOverridesParsesUnityAssets`、`TestAuditMaterialInstanceOverridesEmptyRepoIsHonest`。前端标题/统计与提示同步改为「材质参数覆盖」「材质」「未发现可解析的 Unity 材质」，并展示后端 summary。
- **物理碰撞体检去伪数据（2026-10-23）**：原实现按文件是否包含 `MeshCollider` 只计 1 个，未知时强制伪造 4 个碰撞体/3 凸/1 凹；现按 Unity YAML 的 `--- !u!` 文档逐组件计数，分别读取 `m_Convex`、`m_Material` 与 Rigidbody 运动状态，UE `.umap` 明确跳过并计数。无碰撞体时返回 `analysisKnown=false`、评分 0 且不伪造数量，前端评分显示「—」并展示 summary。新增 `TestAuditPhysicsMeshCollisionParsesUnityDocuments`、`TestAuditPhysicsMeshCollisionEmptyRepoIsHonest`。
- **空数据审计结论修正（2026-10-23）**：本地化审计在完全无文本时曾注入 4 个示例词条/2 个语种并给出 87.5% 覆盖率；数据表审计在 0 张表时仍宣称“所有表结构及数值合法，准许合入主干”。现删除示例词条，空仓库返回 `analysisKnown=false`、`UNKNOWN` 且不下结论；本地化仅字体审计时单独标注；数据表空集 `isValid=false`，前端显示「—」「未发现可审计的数据表」。新增 `TestLocalizationAuditEmptyRepoIsHonest`、`TestDataTableAuditEmptyRepoIsHonest`。同时修复空仓库下 `languagesFound` 等切片为 nil、JSON 序列化为 null 导致 Vue 本地化面板被移除的问题，统一初始化为空切片并补充断言。server 模式实测材质面板 `1 材质 / 5 参数 / 2 失效`、物理面板 `2 碰撞体 / 65 分`、本地化面板 `UNKNOWN / — / 未作结论`、数据表面板 `0 张表 / 未作结论`，控制台零错误。
- **场景流式审计去伪数据（2026-10-23）**：Unity 解析器原从 GameObject 读 `m_LocalPosition`（实际位于 Transform），没有真实坐标时按序号生成位置，Bounds 固定 40³；Godot 用序号伪造 GUID 和坐标；UE 缺字段时生成 `Actor_N`/`actor-guid-N` 与固定包围盒。现 Unity 解析 Transform/RectTransform 的 `m_GameObject` 与 `m_LocalPosition`，Godot 解析 `Transform3D`/`Vector3` 并用节点路径作键，UE 仅保留真实 GUID/Name 与 Location；无法解析时跳过 Actor，Bounds 不再伪造。无 Actor 时返回 `analysisKnown=false`、`UNKNOWN/NO_DATA`、评分 0。`SPATIAL_OFFSET` 改为仅修改匹配 GameObject 的 Unity Transform，缺失时返回错误且不写文件；GUID 重生成前校验 token 存在。新增 `TestParseSceneStreamActorsUsesRealTransforms`、`TestAuditSceneStreamEmptyRepoIsUnknown`、`TestResolveSceneStreamConflictOffsetsOnlyMatchingUnityTransform`，并更新旧夹具。后续补充目标分支与显式关卡路径校验：分支不可解析或文件缺失时返回错误，不再可能对空目标集合给出 SAFE；新增 `TestAuditSceneStreamRejectsInvalidBranchAndMissingScene`。
- **Unreal 二进制 AST 结果标注（2026-10-23）**：二进制 `.uasset` 仍只能做 K2Node 字符串扫描，无法验证导出表/AST；此前结果与文本 Blueprint 同等呈现。现新增 `analysisKnown`/`analysisNote`，二进制包标记为未验证，摘要追加「未验证 AST」，前端计数改为「候选新增/删除/变更/未变」并展示限制说明；文本 Blueprint 保持已验证。新增 `TestDiffUnrealAssetNodesMarksBinaryAsUnverified`、`TestDiffUnrealAssetNodesKeepsTextBlueprintVerified`。后续补充基线/目标引用校验：引用不存在时返回错误，不再把所有节点误报为新增；空树哈希仍用于新增资产。新增 `TestDiffUnrealAssetNodesRejectsInvalidRefs`。
- **LOD 空目录诚实性（2026-10-23）**：LOD/串流审计在无网格/纹理时仍返回健康度 100，且不存在的扫描目录被静默当成空结果。现目录不可访问直接报错；无任何资产时 `analysisKnown=false`、评分 0、摘要明确“未作评分”，前端健康度显示「—」并展示 summary。新增 `TestAuditLodStreamingEmptyRepoIsHonest`。
- **预推送门禁无提交态（2026-10-23）**：空仓库没有任何提交时，预推送检查仍返回 100/100 与 `PASS`。现返回 `analysisKnown=false`、`NO_COMMITS`、评分 0，摘要明确“当前没有待推送提交，未执行提交级策略检查；评分不适用”，前端评分显示「—」并使用中性徽标。新增 `TestAuditPrePushChecklistNoCommitsIsNotPass`。
- **真实字体二进制验证与 UI 修复（2026-10-22）**：使用系统 `/System/Library/Fonts/Geneva.ttf` 与本地化样例执行体检，后端解析出 3249 个字形、检查 9 个字符、缺失 4 个（你好龘），覆盖 55.6%。原 UI 只展示语言对齐与「完整对齐」，出现 ERROR/100%/完整对齐矛盾；现补充字体格式、字形数、检查字符数、缺失字形、覆盖率及 advice 展示。复测面板正确显示 `字体 TTF / 3249 字形 / 缺字 4 / 覆盖 55.6%` 与豆腐块风险建议，零控制台错误。
- **真实 UE 资产验证与 UI 补齐（2026-10-22）**：从开源 uasset-rs 测试集取真实 UE4.26 `.uasset`（25,377 字节，Magic `0x9E2A83C1`）提交到仓库，Engine「Unreal 资产节点差异」以 HEAD vs WORKTREE 对比，解析结果为 `Unreal Blueprint (.uasset)`、`UE 4 (Ver: 522)`、`3 → 3` 节点、无结构性差异。原 UI 未展示格式/引擎版本/Magic，现补充该行信息；本地克隆样本用完即删。
- **OS 启动器验证（2026-10-22）**：Workspace 选中文件点「定位」，Finder 实际打开名为 `wgit-launch-e2e` 的目标窗口；点「默认应用」后 TextEdit 实际启动（测试前未运行），随后关闭该实例与 Finder 测试窗口。Terminal 启动器通过多仓总览的「在终端中打开」验证：操作后新增 zsh 进程 PID 87264，`lsof` cwd 为 `/private/tmp/wgit-terminal-e2e`，随后仅结束该测试 shell；未使用会挂起的 AppleScript 自动化，也没有影响现有终端窗口。Finder/默认应用/Terminal 三类启动器均已实测并清理。
- **宏定义跨分支审计真实化（2026-10-23）**：原实现忽略 base/target 参数，固定返回 3 个激活宏、`UGIT_STUDIO_ACCELERATION` 新增与 `LEGACY_RENDER_PIPELINE` 移除。现从两个 Git 引用真实扫描 `csc.rsp`/`*.rsp`、`*.asmdef`、`*.csproj/.props/.targets`、`*Build.cs`、`ProjectSettings.asset`，解析定义并计算真实新增/移除；危险宏仅根据目标分支实际定义判定。无来源文件时返回 `analysisKnown=false`/`UNKNOWN`，前端评分/宏数显示未知态。新增 `TestAuditSymbolsAndDefinesScansRealSources`；全量 168 测试、覆盖率 74.6%。
- **预推送显式分支审计（2026-10-23）**：原实现无论输入哪个目标分支，提交范围始终基于当前 HEAD/上游，导致选择其他分支时仍审计当前分支。现显式分支会校验存在性，使用该分支自身 `@{u}` 或对应远端引用确定范围，并直接审计该分支的提交；无提交仓库仍返回 NO_COMMITS。新增 `TestAuditPrePushChecklistExplicitBranchAuditsThatBranch`；全量 169 测试、覆盖率 74.7%。
- **数据表审计引用与缺失文件校验（2026-10-23）**：原实现忽略无效 base/target ref，且显式指定不存在的数据表时仍生成报告。现引用不可解析直接报错；目标表在目标 ref 不存在时尝试工作区，若基线与工作区都不存在则报错；仅目标删除而基线存在时仍按删除差异处理。新增 `TestDataTableAuditRejectsInvalidRefsAndMissingTable`，并将空仓库测试改为真实 Git 仓库；全量 171 测试、覆盖率 74.8%。
- **热更清单引用与 WORKTREE 校验（2026-10-23）**：原实现忽略无效 base/target ref，直接返回空的失败清单；目标为 `WORKTREE` 时还会执行非法 `git diff base WORKTREE`。现基线必须是可解析提交，目标 ref 不可解析直接报错，`WORKTREE` 改为 `git diff base` 读取工作区差异；diff 失败上抛错误。新增 `TestGenerateHotfixManifestValidatesRefsAndSupportsWorktree`。最终实测基线为 183 个测试、覆盖率 74.9%（此前增量记录少计 12 个测试，已按 `go test -list` 实测修正）。
- **提交内容安全审计真实计数与正文扫描（2026-10-23）**：原实现把请求条数当作实际审计数，空仓库也会显示通过，且只扫描提交标题。现按 `git log` 返回记录统计实际提交数，使用 `%x1f/%x1e` 分隔解析标题与正文并扫描敏感内容；无提交时返回 `analysisKnown=false`、`Passed=false` 且不下结论。前端无数据时显示中性“无数据”并提示未发现可审计提交。新增 `TestAuditCommitContentSafetyUsesRealCountAndBody`；全量 184 测试、覆盖率 74.9%。
- **分支比对引用校验（2026-10-23）**：无效 base/target 分支此前会返回 `0 ahead / 0 behind`，与真实相同分支混淆；现引用不可解析直接报错，git log 失败也上抛错误。新增 `TestCompareBranchesRejectsInvalidRefs`；同时修复 `TestGitService` 在根目录出现空 `.git` 时的误判，仅在 HEAD 存在时运行。全量 185 测试、覆盖率 74.9%。
- **筛选提交历史分支校验（2026-10-23）**：`GetCommitsFiltered` 原将无效分支的 git log 错误吞成空列表；现分支不可解析直接报错，空仓库仍返回空结果，其他 log 错误上抛。新增 `TestGetCommitsFilteredRejectsInvalidBranch`；全量 186 测试、覆盖率 74.9%。
- **.gitignore 规则匹配修复（2026-10-23）**：原实现用 `strings.Contains` 匹配 `[Ll]ibrary/` 等推荐规则，常见 `.gitignore` 写法 `Library/` 会被误报缺失。现对大小写括号模式做归一化并按行匹配，忽略注释行；新增 `TestAuditIgnoreRulesNormalizesCasePatterns`。全量 187 测试、覆盖率 75.0%。
- **`.gitignore` 一键修复判重与写入错误（2026-10-23）**：追加规则仍用字面量判断，已有 `Library/` 时仍会写入 `[Ll]ibrary/` 语义重复项；写文件失败也会返回成功。现复用归一化规则判重，OpenFile/Write/Close 失败均上抛错误。新增 `TestApplyIgnoreSanitizerDoesNotDuplicateNormalizedRules`、`TestApplyIgnoreSanitizerReturnsWriteError`；全量 189 测试、覆盖率 75.0%。
- **批量标签删除与清理基准修复（2026-10-23）**：批量删除标签时后端吞掉每个失败（不存在 / 非法标签名）仍返回成功，前端在 `deleted` 为空时直接把全部选中标签从列表移除并提示已删除。现后端用 `check-ref-format` 校验标签名、创建 / 本地删除 / 推送 / 远端删除统一加 `--` 隔离，并返回成功列表 + 聚合错误；前端只按真实删除结果更新，失败时在 `finally` 重新加载真实标签状态。另修复「清理已合并分支」确认框显示基准分支（如 main）但实际按当前 HEAD 清理的问题，现显式传入该基准。新增部分失败与 `--all` 选项型标签名断言（不会误删真实标签）；全量 189 测试、覆盖率 75.0%。
- **冲突备份失败保护（2026-10-23）**：`ResolveConflict(backup_and_theirs)` 原忽略本地文件读取与备份写入失败，即使备份未生成也会继续用 theirs 覆盖冲突文件，存在丢失本地版本风险。现读取或写入备份任一步失败即中止，不修改工作区；只在备份成功后执行 checkout + add。扩展 `TestConflictResolutionInTempRepo`，用只读仓库目录验证失败时冲突标记原样保留、恢复权限后备份与覆盖正常完成；全量 201 测试、覆盖率 75.1%。
- **PR/MR 响应读取与凭据库权限失败不再静默（2026-10-23）**：`prAPI.do` 原忽略响应体读取错误，可能把半截数据当完整响应；`RemovePrToken` 在记录已移除但 `chmod 0600` 失败时仍返回成功。现响应读取失败上抛并带请求路径，权限收紧失败明确提示“记录已移除但权限未收紧”。全量 319 测试、覆盖率 75.0%，`go build` 通过。
- **资产检查统一拒绝越界路径（2026-10-23）**：`GetAssetDiff` 的 WORKTREE 读取原直接拼接仓库路径，传入 `../` 可读取仓库外文件并作为预览返回；`InspectAssetMetadata` / `InspectAssetReferences` / `InspectAudioFile` 同样直接拼接路径。现统一经 `repoRelativeAssetPath` 校验，`../` 与绝对路径均拒绝；新增资产差异与四类检查器的越界断言。全量 304 测试、覆盖率 75.2%。
- **资产影响分析负责人查询失败可见（2026-10-23）**：`AnalyzeAssetImpact` 原忽略每个引用文件的 `git log` 失败，负责人列表会静默缺项。现失败时追加“无法读取 … 的最后修改者”警告，引用本身仍保留；新增伪造 `log -1` 失败的测试。全量 302 测试、覆盖率 75.2%。
- **操作日志分隔符改用 NUL（2026-10-23）**：`GetOperationLog` 原用制表符拼接 `%H/%gd/%gs/%ad`，提交标题含制表符时字段可能串位；现改用 `%x00` 分隔，并新增含制表符提交标题的标题/时间分段测试。全量 301 测试、覆盖率 75.2%。
- **整理提交后读取新 HEAD 失败不再假成功（2026-10-23）**：`CreateFixupCommit` 原忽略 `rev-parse --short HEAD` / `log -1` 的错误，提交已创建但界面显示空 hash（如“已并入上一个提交：”）。现哈希读取失败时明确提示“提交已创建，但读取新 HEAD 失败，请 git log 核对后再推送”，标题读取失败也单独说明；新增伪造 rev-parse 失败的测试。全量 300 测试、覆盖率 75.2%。
- **LFS 迁移范围不再静默扩大（2026-10-23）**：`PreviewLfsMigration` / `ExecuteLfsMigration` 原先在关闭「全部历史」且范围为空时回退到 `--everything`，用户以为只迁移一个分支却会重写全部历史；范围输入框还以 `main..feature` 为示例，而 `--include-ref` 只接受单个引用。现关闭全部历史时必须提供可解析的单个分支/引用，否则报错；范围模式校验引用并给出明确提示，UI 改为「分支 / 引用」并更新示例。新增 3 个测试（空范围拒绝、范围字符串拒绝、命名分支预览）；全量 299 测试、覆盖率 75.2%。
- **体检修复拒绝无效仓库（2026-10-23）**：`RunHealthFix` 原对非 Git 目录也返回“去填写身份 / 去处理大文件”等导航结果；现入口统一校验仓库，无效路径直接报错。新增 `TestRunHealthFixRejectsInvalidRepo`；全量 296 测试、覆盖率 75.2%。
- **操作日志读取失败不再伪装成空历史（2026-10-23）**：`GetOperationLog` 原将任何 `git reflog` 失败都当作“没有操作记录”，损坏/权限问题会让撤销页显示空历史。现先校验 `HEAD` 区分空仓库与真实读取失败，后者上抛错误；Reset 的撤销提示明确“若原操作是 hard reset，当时丢弃的未提交改动无法恢复”。新增伪造 `reflog` 失败与空仓库两个测试；全量 296 测试、覆盖率 75.2%。
- **修复中心扫描不完整不再谎报仓库健康（2026-10-23）**：`ScanFixCenter` 原先吞掉四类检查的错误，任一扫描失败都可能显示“仓库状态良好”；现记录 `warnings`、`analysisComplete=false`，前端显示「以下检查未能完成」且不再出现绿色通过。`ApplyAuditFix` 的备份门槛扩展到 secret-unstage 与 staged-large-lfs（两者也会改写 `.gitignore`/`.gitattributes`），备份失败即中止；`GetLfsLockActivity` 不再把锁查询失败降级为“无锁”。`RemediateStagedSecrets` 的 `.gitignore` 读取/写入/Close 失败现在上报并计入失败数；备份目录改用纳秒级时间戳，连续修复不会互相覆盖，备份测试扩展为连续两次快照并核对各自内容。新增 2 个测试（扫描不完整、密钥修复备份）；全量 295 测试、覆盖率 75.2%。
- **LFS 迁移执行闭环实测（2026-10-23）**：`ExecuteLfsMigration` 迁移成功后读取新 HEAD 失败时不再静默，消息会明确要求手动 `git log` 核对后再推送。新增真实 git-lfs 集成测试：临时仓库提交 64KB PSD，执行 `lfs migrate import --everything` 并先创建 bundle，验证 HEAD 被重写、bundle 非空、`git lfs ls-files` 已跟踪 PSD，且返回值与真实 HEAD 一致。全量 281 测试、覆盖率 75.1%。
- **体检一键修复先备份忽略规则（2026-10-23）**：`RunHealthFix("ignore")` 原先直接调用 `ApplyIgnoreSanitizer` 改写 `.gitignore` 并取消跟踪文件，与 Fix Center「写前备份」的承诺不一致。现先备份 `.gitignore`/`.gitattributes` 到 `.git/wgit-fix-backup/`，任一已存在文件无法备份或状态不可读即中止且不改写；`ApplyIgnoreSanitizer` 也不再把握手 `.gitignore` 的读取失败当作空文件，避免重复追加规则。新增 3 个测试（备份内容核对、备份失败中止、不可读拒绝）；全量 280 测试、覆盖率 75.0%。
- **资产引用扫描不完整不再谎报可删除（2026-10-23）**：`InspectAssetReferences` 原忽略 `filepath.Walk` 根目录错误、候选文件读取失败与超过 2MB 的跳过项，任何漏扫都会被当作“未发现引用，可安全删除”。现非 Git 仓库直接报错；无法访问/读取的文件与超限候选计入 `warnings`，`analysisComplete=false` 且 `safeToDelete=false`，消息明确“扫描不完整”。`AnalyzeAssetImpact` 原吞掉二级扫描错误，现同样标记不完整、风险等级为 `unknown`；前端引用卡片与影响面折叠区展示警告。新增 4 个测试（无效仓库、不可读文件、超限候选、影响面不完整）；全量 277 测试、覆盖率 74.9%。
- **LFS 迁移预览失败不解析半截输出（2026-10-23）**：`PreviewLfsMigration` 原只在 stdout 为空时报告 `migrate info` 失败，带部分输出的失败会被解析成“没有匹配文件”。现任何非零退出都上抛并附带首行错误；新增伪造 `git-lfs` 失败测试，真实候选预览用例继续通过。全量 268 测试、覆盖率 74.9%。
- **引擎引导 .gitattributes 读取保护（2026-10-23）**：`mergeGitattributesFile` 原把读取失败当作空文件，可能覆盖不可读但可写的 `.gitattributes`。现读取失败立即中止，追加写入的 Close 错误也上抛；新增 `TestMergeGitattributesAbortsOnReadFailure`（写-only 文件模拟），原有保留自定义行、幂等引导用例继续通过。全量 267 测试、覆盖率 74.9%。
- **资产差异无效引用校验（2026-10-23）**：`GetAssetDiff` 原把无效 old/new ref 的 `git show` 失败当作“文件在该版本不存在”，从而显示为新增/删除资产。现先校验两侧引用，并用 `ls-tree` 区分“路径不存在”与真实读取失败；新增无效引用测试，正常图片尺寸/新增资产用例继续通过。全量 266 测试、覆盖率 74.9%。
- **标签签名对象读取失败不再当作轻量标签（2026-10-23）**：`VerifyTagSignature` 原忽略 `cat-file -t` 错误，标签对象缺失/损坏时会返回“轻量标签没有签名对象”的误导结果；现读取失败上抛错误。新增 `TestVerifyTagSignaturePropagatesMissingTagObject`，通过删除临时仓库中附注标签对象模拟损坏；轻量/无签名附注/缺失标签用例继续通过。全量 265 测试、覆盖率 74.9%。
- **合并重叠预警失败不上报低风险（2026-10-23）**：`GetMergeOverlapWarning` 原把双方文件列表与最后修改人查询的 git 错误吞成空集合，失败时可能返回 `riskLevel=none`。现本分支/目标分支改动读取与重叠文件作者查询失败均上抛；新增无效目标分支测试，原有有交集/无交集用例继续通过。全量 264 测试、覆盖率 74.9%。
- **修复中心备份失败即中止（2026-10-23）**：Fix Center 的 `ignore-rules` 修复原会静默跳过 `.gitignore` / `.gitattributes` 备份失败并继续改写文件，与“修复前会备份”的承诺不符。现对每个已存在目标校验备份结果，读取/写入备份失败即中止且不改写原文件；新增 `TestApplyAuditFixAbortsWhenBackupFails`。全量 263 测试、覆盖率 74.8%。
- **撤销前冲突检查失败保护（2026-10-23）**：`UndoLastOperation` 原先忽略 `git diff --diff-filter=U` 的错误，冲突状态无法确认时仍可能继续软重置；现检查失败直接中止并提示。新增 `TestUndoLastOperationRejectsConflictCheckFailure`；server 模式通过覆盖 `window.confirm` 实测撤销闭环：HEAD 从 `a4544a8` 退回 `04c3e7b`，`a.txt` 修改保留为暂存，页面刷新后显示最新操作为 Reset 且可再次撤销；全量 264 测试、覆盖率 74.9%。
- **空目录扫描错误处理（2026-10-23）**：`ScanEmptyDirectories` 原忽略 `filepath.Walk` 与 `os.ReadDir` 错误，权限不足时会返回部分目录并当作完整结果；现错误上抛、空结果保持非 nil。新增 `TestScanEmptyDirectoriesPropagatesReadFailure`。工作区并行落入了操作日志/闭环/修复中心等新批次，`go test -list` 实测当前为 261 个测试、覆盖率 74.8%。
- **LFS 锁读取失败不再显示为无锁（2026-10-23）**：`GetLfsLocks` 原把所有错误吞成空列表，锁服务器/网络失败时用户会误以为没有文件被锁定。现仅“git-lfs 未安装”保持降级空列表，其余查询失败上抛；LfsTab 新增 `lfsLocksError` 警告条，明确“当前无法确认文件锁定状态”。新增 `TestGetLfsLocksPropagatesQueryFailure`（伪造失败的 git-lfs）与 `TestIsGitLfsUnavailable`；server 模式用同一伪造 git-lfs 实测 LFS 页显示「Git LFS 锁列表读取失败，当前无法确认文件锁定状态：…could not read from remote repository」，随后清理测试实例。全量 216 测试、覆盖率 74.8%。
- **离线队列损坏保护（2026-10-23）**：`loadOfflineQueue` 原把读取失败与 JSON 损坏都当成空队列，下一次 enqueue 会覆盖损坏文件，静默丢失全部待重放操作。现文件不存在才返回空队列，读取/解析失败上抛；`saveOfflineQueue` 的目录创建失败也不再忽略。新增 `TestOfflineQueueCorruptionIsNotSilentlyOverwritten`，验证损坏文件被保留、人工移除后可正常入队；全量 215 测试、覆盖率 74.8%。
- **评审会话列表错误处理（2026-10-23）**：`ListChangeReviews` 原把非 Git 目录、权限/读取失败都吞成空列表；现非 Git 目录与真实读取错误上抛，仅“评审目录尚未创建”返回非 nil 空列表。新增 `TestListChangeReviewsRejectsInvalidRepoAndHandlesFirstRun`；全量 214 测试、覆盖率 74.8%。
- **合并预检失败不再伪装成冲突（2026-10-23）**：`PreviewMerge` 原忽略 `merge-base`、提交计数与差异文件读取错误，并把任何 `merge-tree` 失败都当作“有冲突”；现这些步骤失败均上抛，且 `merge-tree` 失败但解析不到任何 CONFLICT 时返回“合并预检失败”，不再显示空冲突列表。新增 `TestPreviewMergeRejectsUnrelatedHistories`，原有干净合并/真实冲突测试继续通过；全量 213 测试、覆盖率 75.0%。
- **领先/落后提交不再伪造（2026-10-23）**：`GetAheadCommits` 原在 tracking ref 不存在时回退为“最近 20 条本地提交”，SyncModal 会把它们当作“即将推送到远端”；现校验分支与远端跟踪引用，tracking 缺失直接报错，`GetBehindCommits` 仅在远端分支确实不存在时返回空。SyncModal 新增 `commitsError` 警告条，不再静默显示空列表。新增 `TestAheadBehindCommitQueriesRequireRefs`、`TestAheadBehindCommitsDefaultResolveAndTracking`；server 模式实测无远端仓库打开同步中心，显示「无法计算领先/落后提交：远端分支 origin/master 不存在…」，未再展示伪造的本地提交。全量 212 测试、覆盖率 75.2%。
- **暂存大文件检查失败即停（2026-10-23）**：`CheckStagedNonLfsLargeFiles` 原把 `git diff --cached` 失败吞成空结果，并把单个 blob 大小读取/解析失败直接跳过，可能让安全门禁误报“无大文件”。现列表读取、`cat-file -s` 与数值解析失败均上抛；新增 `TestCheckStagedNonLfsLargeFilesRejectsInvalidRepo`，原有“按索引大小判断”测试继续通过；全量 210 测试、覆盖率 75.7%。
- **离线补丁清单错误处理（2026-10-23）**：`GenerateOfflinePatchBundle` 在 bundle 写入后忽略提交数、提交清单与 LFS 指针扫描错误，可能生成“0 提交/0 LFS 指针”的错误摘要。现三处读取均上抛错误，`collectPatchLfsPointers` 返回 error 并由调用方中止；新增 `TestCollectPatchLfsPointersRejectsInvalidRefs`，原有真实 bundle 校验仍通过；全量 209 测试、覆盖率 75.7%。
- **工作树与变基提交错误处理（2026-10-23）**：`GetWorktrees` 原把 `git worktree list` 失败吞成空列表，现上抛错误并保证合法仓库至少返回主工作树；`GetRebaseCommits` 原忽略无效 upstream 的 `git log` 失败并返回空列表，现先做 `rev-parse --verify` 校验、读取失败也上抛。新增 `TestGetWorktreesRejectsInvalidRepo`、`TestGetRebaseCommitsRejectsInvalidUpstream`；全量 208 测试、覆盖率 75.7%。
- **子模块状态错误处理（2026-10-23）**：`GetSubmodules` 原把 `git submodule status` 的失败吞成空列表，`.gitmodules` 存在但仓库/index 异常时会显示“没有子模块”；现直接上抛错误，并把结果初始化为非 nil 空切片，避免 JSON `null`。新增 `TestGetSubmodulesRejectsInvalidRepoAndReturnsEmptySlice`；全量 206 测试、覆盖率 75.7%。
- **自定义 Diff 命令优先级（2026-10-23）**：`OpenExternalDiff` 选择 `diff.tool=custom` 时仍会执行工具自动探测，若机器装有 VS Code 等工具就绕过用户配置的自定义命令。现 custom 模式跳过自动探测、空命令明确报错；扩展 `TestOpenExternalDiffRejectsInvalidRepoAndStagedReadFailure`，用真实脚本验证 staged（HEAD→暂存区）与 unstaged（暂存区→工作区）两侧内容及原始换行，并覆盖空命令失败；全量 205 测试、覆盖率 75.7%。
- **自定义合并命令与原始内容（2026-10-23）**：`OpenExternalMerge` 之前完全忽略设置页保存的 `wgit.customMergeCmd`，选择“自定义命令模板”后会落到工具检测或直接报“未找到工具”；且 diff/merge 用 `ExecGit` 读取内容会裁掉末尾换行。现支持 `{theirs} {ours} {base} {result}` 占位符并通过 shell 启动，校验空命令；冲突阶段与 diff 两侧改用 `ExecGitRaw` 保留原始字节，三阶段全部读取失败时明确报错，临时文件写入失败也上抛。新增 `TestOpenExternalMergeUsesCustomCommand`，真实执行脚本并校验三方内容/结果文件；全量 205 测试、覆盖率 75.6%。
- **外部 Diff 内容读取校验（2026-10-23）**：`OpenExternalDiff` 原先忽略配置读取错误，并在暂存区内容读取 / 临时文件写入失败时继续启动外部工具，可能把空内容当作 staged 版本展示。现校验仓库有效性、上抛配置错误、严格检查 `git show :file` 与右侧临时文件写入；未跟踪文件仍允许空左侧基线。新增 `TestOpenExternalDiffRejectsInvalidRepoAndStagedReadFailure`，用不可读 index 模拟失败；全量 204 测试、覆盖率 75.4%。
- **快速提交推送失败原因（2026-10-23）**：`FastCommit` 推送失败时原先把 stdout 拼进提示，而 git 的拒绝原因通常在 stderr，UI 常显示为空的 `push failed:`。现使用真实 error（含 stderr）生成消息，本地提交与 rebase 结果保持不变、`Pushed=false`。新增 `TestFastCommitReportsPushRejectionReason`，用 bare remote 的 `pre-receive` 钩子拒绝推送并断言远程原因可见；全量 203 测试、覆盖率 75.1%。
- **历史大对象扫描错误处理（2026-10-23）**：`ScanHistoryLargeBlobs` 原忽略 `rev-list` / `cat-file` 两个进程的退出错误与 scanner 读取错误，非法仓库或读取失败会显示为“无大对象”。现分别检查两个进程结果与扫描器错误并上抛；空仓库仍返回非 nil 空列表。新增 `TestScanHistoryLargeBlobsRejectsNonRepoAndHandlesEmptyRepo`；全量 202 测试、覆盖率 75.1%。
- **变基计划引用校验（2026-10-23）**：`InteractiveRebasePlan` 原把任意 `git log` 失败吞成空计划，无效 upstream 会被 UI 当作“没有可变基提交”。现先用 `rev-parse --verify upstream^{commit}` 校验引用，再读取提交列表；失败均上抛。扩展 `TestInteractiveRebaseExecuteInTempRepo` 覆盖无效引用；全量 201 测试、覆盖率 75.1%。
- **外部工具配置清理校验（2026-10-23）**：`SetExternalToolConfig` 清空自定义命令 / 工具字段时忽略 `--unset-all` 失败并返回成功，旧配置会继续生效；现复用 `clearGitConfigKey` 校验 local/global 两种作用域，键值仍存在即上抛。扩展 `TestRemoteWebInfoAndExternalToolConfigInTempRepo`，用只读 `.git` 验证失败保留与恢复后的正常清理；全量 201 测试、覆盖率 75.1%。
- **代理清理错误处理（2026-10-23）**：`ClearGitProxy` 原忽略 local/global 两种作用域的 `--unset-all` 失败并固定返回成功；现用 `clearGitConfigKey` 在 unset 失败后回读 `--get-all`，仍有值则上抛错误，httpx/https 两项分别处理。新增 `TestClearGitProxyPropagatesFailureAndClearsBothScopes`，覆盖只读 `.git` 失败保留、local 清理与隔离 global 配置清理；全量 201 测试、覆盖率 75.1%。
- **.gitkeep 生成与源码差异错误处理（2026-10-23）**：`GenerateGitkeeps` 原忽略 `git add` 失败并照样计数，且接受 `../` 路径向仓库外写文件；现拒绝绝对路径、文本越界目录与指向仓库外的目录软链接（`EvalSymlinks` 复核），聚合写入/暂存失败并返回真实成功数，前端失败时重新扫描空目录。`GetSourceFileDiff` 原在两次 git 读取都失败时返回空字符串当成功；现工作区与指定 revision 两条回退路径均失败时上抛错误，未知 revision 不再显示为“空差异”。新增 `TestGenerateGitkeepsRejectsEscapeAndReportsStageFailure`、`TestGetSourceFileDiffRejectsUnknownRevision`；全量 200 测试、覆盖率 75.1%。
- **提交签名与 LFS 过滤配置写入校验（2026-10-23）**：`ConfigureCommitSigning` 原忽略 `gpg.format` / `user.signingkey` 写入失败，只要 `commit.gpgsign` 写入成功就返回“签名已启用”，可能在后续提交时才失败；现校验仓库与每一步 config 写入，任一失败立即上抛。`SetLfsFilterConfig` 清空过滤规则时忽略 `--unset` 失败，UI 会误认为已清空；现检测键值是否仍存在，仍有值则报错，并复用统一的 `unsetRepoConfig` 处理 `reset_default`。新增 `TestConfigureCommitSigningPropagatesWriteFailures`、`TestSetLfsFilterConfigPropagatesUnsetFailure`（只读 `.git` 模拟失败）；全量 198 测试、覆盖率 75.0%。
- **Unity Meta 修复与 LFS 调优写入错误（2026-10-23）**：`AutoFixUnityMeta` 原忽略每个 `git add` 的失败仍把路径计入已修复，前端提示成功；现校验暂存结果，失败项聚合为错误并只返回真正修复的路径，前端在失败时也重新加载配对状态。`ApplyLfsTuningOptimization` 原忽略所有 `git config` 写入失败并返回 `Success=true`；现校验仓库有效性、逐项写入并在失败时上抛，`reset_default` 仅把“键本来不存在”视为可接受。新增 `TestAutoFixUnityMetaPropagatesStageFailure`、`TestApplyLfsTuningRejectsInvalidRepoAndWriteFailure`（只读 `.git` 模拟写入失败）；全量 196 测试、覆盖率 75.0%。
- **丢弃全部与 Reflog 错误处理（2026-10-23）**：`DiscardAll` 原忽略 `git restore .` 的失败，继续清理未跟踪文件并返回成功，用户可能同时丢失未跟踪内容且残留已跟踪修改。现校验仓库路径；有 HEAD 时恢复失败立即上抛，无 HEAD 的空仓库跳过恢复，`git clean` 失败也上抛。`GetReflog` 原把任意 git 错误吞成空列表；现非 Git 目录直接报错，空仓库返回显式非 nil 空切片，读取失败上抛。新增 `TestDiscardAllHandlesEmptyRepoAndRejectsNonRepo`、`TestDiscardAllPropagatesRestoreFailure`（只读目录模拟恢复失败）与 `TestGetReflogRejectsNonRepoAndHandlesEmptyRepo`；全量 194 测试、覆盖率 75.0%。
- **仓库维护结果真实化（2026-10-23）**：`RunMaintenance` 原对远端 prune、`git gc --auto`、`git prune` 的错误全部 `_ =` 吞掉，随后返回统计，前端无论实际结果都提示「已完成」。现执行前校验仓库有效性，逐步记录 `remotePruneStatus` / `gcStatus` / `pruneStatus`、`warnings` 与 `summary`；无远端时明确 `SKIPPED` 并说明已跳过，任一步失败时 `success=false` 且前端展示失败详情、Toast 不再宣称完成；`GetMaintenanceStats` 对非 Git 目录返回错误。新增 `TestRunMaintenanceReportsRemotePruneFailure`、`TestRunMaintenanceRejectsInvalidRepo`，并扩展无远端成功态断言；全量 191 测试、覆盖率 75.0%。
- **合并分支清理保护长期分支（2026-10-23）**：`GetMergedBranches` 原只保护 main/master/release/develop/HEAD，`release/1.0`、`hotfix/*`、`develop/*`、`support/*` 等仍会被列为可清理；`DeleteMergedBranches` 无任何保护，直接传参可删除长期分支。现统一使用 `protectedBranchName` 识别长期分支（含 `refs/heads/` 归一化与大小写），扫描与删除双入口均跳过；删除入口额外拒绝 `-` 开头与非法 ref，避免参数注入。分支洞察同步新增 `protected` 分类与计数：长期分支即使已合并也不再标记「可删除」，不计入 `cleanable`，前端显示中性「长期分支」徽标；扩展 `TestMergedBranchesCherryPickRangeAndCommitTemplate`、`TestBranchInsights` 覆盖扫描、删除与洞察三条路径。server 模式实测：4 个本地分支中 `release/1.0`、`hotfix/urgent` 显示「长期分支」且无清理按钮，洞察为「长期分支 2 / 已合并 1」；清理确认框明确基准 `main` 且只列出 `feature/merged`，执行后磁盘仅剩 main 与两个长期分支，按钮变为禁用；全量 189 测试、覆盖率 75.0%。
