# 落地记录：诊断闭环 + 资产差异化（2026-10-23）

> 依据：[feature-research-2026-10.md](feature-research-2026-10.md)
> 范围：把报告里「可立刻开工的 10 件事」全部做完，并追加 B 档闭环框架与 A 档两项护城河能力。
> 验证：`go vet` 干净 · `go test .` 全绿（39–80s）· `vue-tsc --noEmit` 零错误 · `bun run build` 通过。

---

## 一、RPC 变化

**327 → 355（+28）**，全部有 UI 入口。

| 文件 | 新增 RPC |
| --- | --- |
| `git_closed_loop.go`（新建，8） | `TrackStagedLargeFilesAsLfs` `UnlockStaleLfsLocks` `GetRangeDiff` `CreateFixupCommit` `OptimizeObjectStore` `GetMergeOverlapWarning` `RunHealthFix` `VerifyTagSignature` |
| `git_fix_center.go`（新建，3） | `ScanFixCenter` `ApplyAuditFix` `RemediateStagedSecrets` |
| `git_asset_diff.go`（新建，1） | `GetAssetDiff` |
| `git_lfs_migrate.go`（新建，2） | `PreviewLfsMigration` `ExecuteLfsMigration` |
| `git_engine_bootstrap.go`（新建，1） | `RunEngineBootstrap` |
| `git_asset_impact.go`（新建，1） | `AnalyzeAssetImpact` |
| `git_oplog.go`（新建，2） | `GetOperationLog` `UndoLastOperation` |
| `git_terminal.go`（新建，5） | `StartTerminal` `WriteTerminal` `ResizeTerminal` `StopTerminal` `GetTerminalStatus` |
| `git_maintenance_extra.go`（新建，5） | `RepairWorktrees` `PruneWorktrees` `BisectSkip` `ReadGitHook` `SaveGitHook` |

配套测试：`git_closed_loop_test.go`（15 个用例）、`git_fix_center_test.go`（7 个）、`git_asset_diff_test.go`（6 个）、`git_engine_bootstrap_test.go`（7 个）、`git_oplog_test.go`（5 个）、`git_terminal_test.go`（5 个）、`git_maintenance_extra_test.go`（10 个）。

---

## 二、报告条目 → 落地位置

### 「可立刻开工的 10 件事」

| # | 事项 | 落地位置 |
| --- | --- | --- |
| 1 | Inter 字体 `@font-face` | `frontend/src/style.css`（声明 400–600 覆盖，含 `--font-sans` 与 body 栈） |
| 2 | `settingsIndex` 虚假索引清理 | 删 6 项（密度/字号/动效/默认身份/代理环境变量/无效配置清理）；「强调色」保留并补了实现 |
| 3 | 暂存大文件一键转 LFS | `CleanTab.vue` + `TrackStagedLargeFilesAsLfs` |
| 4 | 僵尸锁批量破除 | `LfsTab.vue` + `UnlockStaleLfsLocks` |
| 5 | 健康分真的可修 | `MaintenanceTab.vue` + `RunHealthFix`（需决策项跳转，可写项执行） |
| 6 | 合并前冲突预演 | 已有 `PreviewMerge`（merge-tree）；本轮补「队友重叠」`GetMergeOverlapWarning` 并在 `MergeBranchModal` 展示 |
| 7 | fixup + autosquash | `CommitInspector` 加「整理为 Fixup」+ `CreateFixupCommit`；两个变基弹窗的 autosquash 早已默认勾选 |
| 8 | range-diff | `BranchCompare.vue` 折叠面板 + `GetRangeDiff` |
| 9 | 对象库优化三键 | `MaintenanceTab.vue` + `OptimizeObjectStore`（commit-graph / MIDX / repack / pack-refs） |
| 10 | 文档计数同步 | README、`architecture-audit.md`、`dead-code-cleanup.md` 全部更新到 341 |

### B 档 · 闭环

- **A4（部分）LFS 锁协同**：批量破僵尸锁之外，每条他人锁/僵尸锁可一键复制「锁释放请求」文案（含文件、持有人、锁定时长），直接粘贴到 IM 找人解锁。完整看板未做。
- **B1 统一 Audit→Fix 框架**：`ScanFixCenter` 把「暂存区密钥 / 未走 LFS 大文件 / 僵尸锁 / 忽略规则缺口」聚合成一张清单（含 `fixable` / `fixLabel` / `targets` / `needsReview`）；`ApplyAuditFix` 按 `fixId` 分发到既有写操作，写文件前快照到 `.git/wgit-fix-backup/<ts>/`。前端 `FixCenterSection.vue` 挂在维护页顶部。
- **B2 密钥处置**：`RemediateStagedSecrets` — `git restore --staged` 后将路径写入 `.gitignore`（自动去重），可选写 `.git/wgit-secret-allowlist`。

### A 档 · 差异化

- **A1 二进制资产差异查看器**：`GetAssetDiff` + `AssetDiffModal.vue`。四种模式——并排、叠加（透明度滑杆）、滑动擦除、逐像素差值热力图（ΔRGB > 24 标洋红，底图压暗）。同时回读尺寸/体积/格式变化。PNG/JPG/GIF/BMP/WebP 内联渲染；TGA/PSD/EXR 退化为元数据并提示用 DCC 工具。入口在工作区差异头部。
  - 暂存改动读 index blob（`git show :0:<path>`），未暂存读工作区。
  - 差值计算上限 400 万像素、预览 payload 上限 12MB，超限给出明确降级说明。
- **A3 LFS 历史迁移向导**：`PreviewLfsMigration`（`git lfs migrate info` 只读预览）+ `ExecuteLfsMigration`（可先建 bundle 回滚点）。未预览时执行按钮 disabled，执行前两次确认重写风险，完成后提示 `push --force --all --tags` 与协作者重 clone。

### A 档补完（第二轮）

- **A2 资产影响分析**（`AnalyzeAssetImpact`）：从被改资产出发顺着引用扫两级（贴图 → 材质 → 预制体/场景），每条引用附最近修改者与时间，按「直接优先、引用次数降序」排序，输出 none/low/medium/high 风险等级与负责人清单。二级展开上限 20 个直接引用（每跳都是一次全仓扫描），超限标注截断。UI 在资产对比弹窗底部折叠区。
- **C3 操作日志 + 撤销**（`GetOperationLog` / `UndoLastOperation`）：读 `git reflog` 并翻译成人话，标注可撤销性；撤销用 `reset --soft`（改动退回暂存区，绝不 hard），变基/切换分支拒绝并给替代路径，有冲突直接拦住。UI 在工具箱 → Reflog 页顶部。
- **A5 引擎工程一键 Bootstrap**（`RunEngineBootstrap`）：`.gitattributes` 预设按行合并（绝不覆盖手写规则）、`.gitignore` 补齐（可选取消跟踪已提交的构建产物）、UnityYAMLMerge、LFS 扩展名跟踪，一次幂等配置；完成后重新体检给 0–100 就绪度。UI 在引擎资产页顶部。

### 顺带补齐
- **D8 无障碍（部分）**：右键菜单支持 ↑↓ / Home / End / Enter / Space，`nextEnabledIndex` / `edgeEnabledIndex` 抽为纯函数并测 9 条用例；分支 / 标签 / 储藏的行补 `aria-label`（含名称与当前态）；**604 个按钮中纯图标按钮的 `aria-label` 缺口清零**（92 处由 `title` 自动补齐、5 处关闭按钮手工补）；应用壳层加「跳到主内容」skip link。

  未做且刻意不做半套：这三个列表的行在模板里是 fragment 顶层兄弟，没有可以挂 `role="list"` 的父容器；只加 `role="listitem"` 而没有 list 父级，对 AT 来说比不加更混乱，因此改为行级 `aria-label`。真正的键盘可达需要 roving tabindex + 方向键选中，是独立改造，留待后续。
- **D7 列表右键菜单**：新增 `composables/useContextMenu.ts`（定位 + 视口回夹 + 生命周期）与 `components/ui/UiContextMenu.vue`（Teleport、`role="menu"`、Esc/外部点击/滚动关闭、二次实测校正）。已铺到 `LocalBranchList`、`TagsList`、`StashesList`；菜单支持 ↑↓ / Home / End / Enter / Space 键盘操作，`nextEnabledIndex` 与 `edgeEnabledIndex` 抽为纯函数并被 18 条前端用例覆盖（边界夹取 + 键盘游标算术）。`ChangesList` 未铺：它已有复选框批量栏 + hover 操作按钮，右键收益低于前三者。
- **C7 剩余补齐**：`RepairWorktrees`（工作树 `.git` 指针失效后重新链接）、`PruneWorktrees`（`--expire=now` 立即清理，git 默认等 3 个月）、`BisectSkip`（不可构建的提交让二分卡死）、`ReadGitHook` / `SaveGitHook`（钩子从「只能启停」到「能改脚本」，启用写 0755、禁用落 `.disabled`、名称白名单防路径穿越）。

  手工验证记录：`git worktree prune -v --expire=now` 在临时仓库中输出了 `Removing worktrees/wt-feature: gitdir file points to non-existent location`，并确认 `worktree list` 只剩主工作树。该行为在 `go test` 环境无法稳定复现（同一 git 版本、同一调用顺序，独立程序可复现、测试进程不可），故单测只断言「调用成功 + 返回结构」，不做 git 侧删除的深度断言。
- **C1 内置终端**：`creack/pty`（Go）+ `@xterm/xterm` / `@xterm/addon-fit`（前端）。PTY 单例挂在包级变量上（`GitService` 保持无状态），输出走 `window.EmitEvent("terminal:output")`。128KB 循环回放缓冲、`ResizeTerminal` 同步 PTY 尺寸、切页不杀进程、主题色读 CSS 令牌。UI 为工具箱子页「终端」+ 命令面板入口。
- **D1 i18n（骨架 + 首批文案）**：`src/i18n/` 自建轻量实现，零依赖。`translate(key, params, localeOverride)` 纯函数 + `locale` ref；缺失 key 按「当前语言 → 源语言 → key 本身」三级回退。首批接入导航栏（6 项）、`UiModal` 关闭按钮、外观设置的标题/主题/强调色/语言选择、修复中心（标题/按钮/空态）。英文目录与中文逐 key 对齐，测试用「英文必须覆盖全部中文 key」「key 全小写点分层」两条契约锁住。未做：其余 30+ 组件文案仍在硬编码，随 UI 改动逐步迁移。
- **D2 UI 原语库（骨架 + 试点）**：`components/ui/` 新增 `UiButton` / `UiBadge` / `UiModal` / `UiEmpty` / `UiSkeleton`；变体映射抽到 `utils/uiVariants.ts`（完整 class 字面量，避免 Tailwind 扫不到动态拼接），配 `tests/uiVariants.test.ts` 覆盖全部变体契约。已迁移 18 处：`branches/modals/` 全部 5 个、标签详情、`HistoryView` 3 个、`RemotesTab` 4 个、`SubmodulesTab` 3 个、`StashView` 2 个、`FixCenterSection` 四件套。`UiButton` 变体扩到 6 个（新增 warning / purple，对应 stash 与 tag 的既有语义色）。按 `role="dialog"` 统计，手写弹窗 **39 → 22** 处（第五轮已收尾到 7 处，见下）。

剩余 22 处已到结构性终点：其中 `QuickOpen` / `CommandPalette` 是搜索面板、`RepoOnboarding` / `RepoDashboard` 是全屏页、`RebaseModal` 是变基计划编辑器、`AssetDiffModal` 有自定义宽幅 header——这些套不进「固定 header + 滚动 body + 固定 footer」的模型，强行迁移会破坏布局。真正适合继续迁的只剩 14 处零散单点（`ExportModal`、`LfsTab`、`ReflogTab`、`GitignoreEditor` 等），边际收益明显下降。
- **D5 前端测试**：`frontend/tests/` 三个文件共 46 个用例（`bun test`，约 50ms）。选 Bun 内置 runner 而不是 vitest：零新依赖、零配置，直接跑 TS。覆盖快捷键注册表（解析/格式化/匹配/冲突/覆写持久化容错）、资产差值（阈值、资格判定、灰度）、设置索引（分类归属、label 唯一、keyword 小写、禁止幽灵条目回归）。为此把 `AssetDiffModal` 里的像素差值逻辑抽到 `utils/assetDiff.ts`，测试直接覆盖生产代码。
- **D6 视图页签持久化**：主视图（工作区/历史/分支/储藏/评审/工具箱/设置）选择写入 `wgit.active-tab`，重启后回到上次的位置；恢复只在仓库已打开时生效，避免落到需要仓库的空面板。

- **C7 标签验签**：`VerifyTagSignature`（此前只有 `verify-commit`）。轻量标签直接解释「没有签名对象」，附注标签回读 GPG 状态并提取签名者。UI 在标签详情弹窗。
- **D4 品牌强调色**：6 套预设（远山蓝 / 紫藤 / 青瓷 / 苔绿 / 落日 / 胭脂），`accent-presets.css` 用 `[data-accent]` 属性覆盖 accent 色族，深/浅各有独立取值；语义色（成功/警告/危险）固定不漂移。
- **命令面板 +3 条**：对象库优化、暂存区大文件转 LFS、range-diff 对照。

---

## 三、关键实现决策（改动时勿破坏）

1. **强调色用 CSS 属性而非 JS 内联变量**。`[data-accent="violet"]` 与 `[data-accent="violet"][data-theme="light"]` 两套规则，切主题时浏览器自动重解析，无需 JS 重算；`blue` 不设属性，保持 `tokens.css` 为唯一真源。
2. **`git rm --cached` 必须带 `-f`**。新加文件与 HEAD 内容不同，不加 `-f` 会被 git 拒绝，导致整个 LFS 转换流程卡在第一步。
3. **LFS 迁移预览解析四段格式**：`pattern \t size \t "1/1 file" \t "100%"`。文件数取第二段分数的分母。
4. **高危操作强制先预览**。`ExecuteLfsMigration` 的 UI 按钮在 `migratePreview` 为空时 disabled，避免用户凭记忆重写历史。
5. **修复一律可回滚**。文件类修复前写入 `.git/wgit-fix-backup/<ts>/`，结果里回传 `backupDir`。

---

## 四、第三轮：C2 PR/MR 就地评审 + D1 统一确认对话框

### C2 · PR / MR 就地评审（RPC 355 → 363，+8）

新增 `git_pr.go`，零第三方依赖（GitHub REST v3 / GitLab v4 都是纯 JSON over HTTP）：

| RPC | 作用 |
| --- | --- |
| `GetPullRequestProvider` | 解析 `origin` → host / owner / repo / 托管商，回报令牌是否存在及来源（**不返回令牌**） |
| `SavePullRequestToken` | 先打 `/user` 在线校验，401/403 直接拒绝且不落盘；通过后用 AES-256-GCM 写入 `.git/wgit_vault.json`（0600） |
| `ClearPullRequestToken` | 删除该 host 的令牌记录并重写保险库，保留其他记录 |
| `ListPullRequests` | 四态筛选（open/closed/merged/all），GitHub 的 merged 先查 closed 再按 `merged` 过滤 |
| `GetPullRequestDetail` | PR + 逐文件补丁 + 行内评论 + 普通讨论 + CI（GitHub check-runs/combined status，GitLab pipelines） |
| `CreatePullRequestComment` | 普通讨论或行内评论（GitHub `pulls/{n}/comments`；GitLab `discussions` + position） |
| `SubmitPullRequestReview` | approve / request_changes / comment；GitLab 无原生 Request changes，降级为带 🔴 前缀的讨论并说明 |
| `MergePullRequest` | merge / squash / rebase；平台拒绝时原样回传原因 |

令牌解析顺序：`WGIT_PR_TOKEN` / `GITHUB_TOKEN` / `GH_TOKEN` / `GITLAB_TOKEN` / `GL_TOKEN` → 本地保险库 → `gh auth token` → `git credential fill`。所有请求 25s 超时、响应体 8MB 上限、单文件补丁 200KB 截断，错误按 401 / 403 限流 / 权限不足 / 404 / 422 翻译成中文。

前端新增主视图「PR / MR 评审」（`components/pr/`）：左侧四态筛选 + 本地搜索 + 列表；右侧详情（描述 / 文件 / 讨论 / 检查四页签）、点补丁行号锚定行内评论、`⌘/Ctrl+Enter` 提交、批准 / 请求修改 / 仅评论、合并方式选择。接入活动栏、命令面板（`goto-pr`）与页签持久化；`PrView` 异步分包 30.87 kB。

测试：`git_pr_test.go` 15 条（远端解析 7 组、令牌来源与不回传、GitHub/GitLab 列表与详情映射、行内/普通评论 payload、评审与合并、403/401 翻译、保险库加密往返与清除、无令牌降级）；全部走 `httptest`，不触网。

### D1 · 统一确认对话框（88 处原生 confirm 清零）

- 新增 `composables/useConfirm.ts`：`confirmAction(options) => Promise<boolean>`，单实例、后到取消先到（避免悬挂 Promise），支持 `requireText` 二次确认。
- 新增 `components/ui/ConfirmDialog.vue`：复用 `UiModal` 的焦点陷阱 / Esc / 焦点归还；按 tone 切换图标与按钮色。
- 27 个文件、88 处 `window.confirm()` 全部改为 `await confirmAction(...)`（TS AST 改写 + `vue-tsc` 校验，仅 1 处所在函数需补 `async`）。
- 48 处破坏性文案（删除 / 丢弃 / 清空 / 强制 / 覆盖 / 移除 / 撤销 / 重置 / 破锁 / 永久 / 重写）自动标为 `danger`；两处含「不会删除」的否定句改回 `warning`。
- 三处不可逆操作要求输入确认词：删除未合并分支（输入分支名）、清空储藏箱（输入「清空」）、LFS 历史重写（输入 `MIGRATE`）。

### 第四轮补完：列表键盘导航 + PR 分页 + 测试自检修复

- **列表键盘导航**（`composables/useRovingList.ts`）：分支 / 标签 / 储藏三张列表改为 roving tabindex——整张列表只占一个 Tab 位，<kbd>↑</kbd><kbd>↓</kbd> 逐行移动，<kbd>Home</kbd>/<kbd>End</kbd> 跳首尾，<kbd>Enter</kbd> 执行主操作（分支检出 / 标签详情 / 储藏应用）。行内按钮保持独立 Tab；`@keydown.enter.self` 确保在「删除」按钮上按回车不会冒泡成整行动作。未套 `role="listbox"`：行内嵌交互控件时，listbox/option 语义反而会误导 AT，保留行级 `aria-label` + `tabindex` 更诚实。
- **PR 列表分页**（`ListPullRequests` + `page`）：后端透传 `page`，前端在列表底部提供「加载更多」，追加而非替换；切换状态筛选 / 刷新 / 换仓库都会重置到第 1 页。补 `TestListPullRequestsForwardsPage` 锁住 `page` / `per_page` 的透传。
- **测试自检修复**：`TestGitService` 过去只在作者那台「内容齐全」的仓库里通过，换到新 clone 会连环失败——已去掉对 `package.json`、`feature/fast-commit`、`v0.1.0`、`HEAD~5`、`hotfix/v1.0.1`、`resources/icon.png`、CI 五阶段假数据的硬编码依赖，改为探测实际存在的文件 / 分支 / 标签，并让 CI 断言检查「无配置时不编造运行记录」。`git_asset_audit_test.go` 的两次 `git commit` 由忽略错误改为 `runGitTest` 快速失败，避免以后再次出现「timeline 为空」这种指错方向的断言。

### 第五轮：手写弹窗收尾（22 → 7）

`UiModal` 增加 `bodyClass` 插槽参数（全出血正文、`space-y-*` 间距），然后把剩余 14 处零散弹窗全部迁完：

| 文件 | 弹窗 |
| --- | --- |
| `HistoryView.vue` | 范围 Cherry-Pick、批量 Cherry-Pick、交互式变基计划、Git Notes 编辑、签名详情、批注总览、内容考古（7 个） |
| `BranchesView.vue` | 版本对比文件差异 |
| `WorkspaceView.vue` | 工作区版本差异 |
| `tools/ReflogTab.vue` | 从历史时刻创建恢复分支 |
| `tools/LfsTab.vue` | LFS 独占锁定 |
| `workspace/ChangesList.vue` | 批量储藏所选文件 |
| `GitignoreEditor.vue` | `.gitignore` 规则编辑器 |
| `review/ExportModal.vue` | 评审报告 Markdown 预览 |
| `branches/modals/MergeBranchModal.vue` | 高级分支合并 |

迁移统一处理了三件事：**进行中的操作不允许 Esc/遮罩关闭**（`@update:open` 守卫 `isExecuting*` 状态）、**关闭按钮与焦点归还交给 `UiModal`**、**按钮改用 `UiButton` 变体**（warning / purple / danger 与既有语义色对齐）。同时清掉 10 处迁移后遗留的 `useModalA11y` 绑定与导入。

剩余 7 处 `role="dialog"` 是结构例外，不再强迁：命令面板、快速打开、全屏引导页、多仓总览、高级同步弹窗、高级变基弹窗、二进制资产差异（自定义宽幅 header）。

### 第六轮：i18n 外壳 + 两个完整视图

在原有「导航 / 通用词汇 / 外观设置 / 修复中心 / PR」基础上继续抽取，本轮完成：

| 范围 | 文件 | 条目 |
| --- | --- | --- |
| 应用外壳 | `HeaderBar.vue` | 30（项目切换、克隆/初始化、分支切换、搜索、Fetch/Pull/Push/Rebase、刷新、变基进度插值） |
| 应用外壳 | `RepoRail.vue` | 12（标签页提示、脏标记、多仓总览、打开目录） |
| 储藏箱整页 | `StashView.vue` | 70（列表/搜索/四个动作、空态、文件清单、差异加载、两个弹窗、全部 toast 与确认文案） |
| 变更评审整页 | `ReviewView.vue` | 29（标题、进度、文件通过数、导出/清除/刷新、错误与 toast、确认删除评论/清除会话） |

要点：

- **带参文案统一走 `{name}` 插值**，不再用模板字符串拼中文：`t("stash.toast.applied", { index })`、`t("header.sync.rebasing", { step, total })`。
- **保留后端枚举值不译**：`ReviewView` 里 `case "可以合并"` / `case "需修改"` 是后端返回值的匹配分支，翻译会直接让逻辑失效，已刻意跳过。
- **新增第三条 i18n 契约测试**：扫描 `src/**/*.{vue,ts}` 里所有 `t("...")`，断言 key 都存在于中文目录。当前 222 个在用 key 全部命中，后续拼错 key 会直接红。
- 迁移后用脚本复核：外壳与两个视图的**非注释中文清零**；全仓剩余中文 30,440 字（83 个文件），主要集中在 `MaintenanceTab` / `LfsTab` / `WorkspaceView` / `HistoryView` 四个大文件。

### 热修：PR 列表保存 token 后白屏

- **现象**：在「PR / MR 评审」里填完令牌并保存后，整个视图消失（不是空列表，是白屏）。
- **根因**：Go 的 nil slice 会序列化成 JSON `null`。没有标签的 PR 得到 `"labels": null`，而列表模板直接读 `item.labels.length`——渲染函数抛 `TypeError`，Vue 把整个组件卸载掉。保存 token 前列表是空的，所以问题只在第一次加载真实 PR 时暴露。
- **修复**：
  - `mapGhSummary` / `mapGlSummary` 始终把 `Labels` 初始化为 `[]string{}`（GitLab 侧同时兼容 API 显式返回 `null`）。
  - 模板改为 `item.labels?.length` 与 `(item.labels ?? []).slice(0, 3)`；详情面板对 `files / comments / checks` 同样加 `?.` 与 `?? []` 兜底。
  - 回归测试 `TestListPullRequestsNormalizesMissingLabels` / `TestListPullRequestsNormalizesNullGitlabLabels` 直接断言序列化结果是 `"labels":[]`。
- **顺带**：`TestOpenExternalMergeUsesCustomCommand` 的异步脚本等待从 3s 放宽到 10s——满负载跑全量时会假失败。

### 第七轮：评审模块入口合并

**判断**：两个模块的重合点在「逐文件 diff + 行级评论 + 结论」这套交互，约占 UI 的六成；但数据层差异很大——本地评审是 `.git/wgit-review` 里的离线会话（逐文件 verdict、进度、导出），远端 PR 是 GitHub/GitLab 的在线对象（token、CI、review event、合并）。因此先做**入口级合并**，不做数据层重写。

- 活动栏去掉独立的「PR / MR」项，只保留「评审」；主视图顶部新增「本地范围 / 远端 PR」来源切换。
- 两个来源各自保留原有实现与 header（本地：base/head + 进度 + 导出；远端：provider + token + CI），切换时整块替换，避免把两套状态机硬塞进一个组件。
- 选择持久化到 `wgit.review-mode`；旧的 `wgit.active-tab = "pr"` 会自动映射到「评审 → 远端 PR」，老用户不会丢上下文。
- 命令面板保留两条直达命令（`goto-review` → 本地，`goto-pr` → 远端），只是落到同一个 tab 的不同来源。
- 数据层已统一：两个来源实现同一份 `ReviewSource` 契约（见第九轮），视图只依赖接口与 `capabilities`，不再直接调用 `gitService` 的评审方法。

### 第八轮：i18n 两个最大视图（Workspace / History）

| 范围 | 文件 | 新增 key |
| --- | --- | --- |
| 工作区整页 | `WorkspaceView.vue` | 181（页签、统计、布局预设、差分模式/缩放/图像背景、Blame、文件历史、冲突裁决、暂存/丢弃/差异块、提交面板、模板、快速提交、全部 toast 与确认） |
| 提交历史整页 | `HistoryView.vue` | 206（图谱与车道、筛选摘要、计数、Cherry-Pick 范围/批量、交互式变基计划、Git Notes、签名详情、内容考古、右键菜单、全部 toast 与确认） |

要点：

- **模块级常量不能直接存 `t()` 结果**：`WORKSPACE_LAYOUT_LABELS` 原本是模块加载时求值的对象，语言切换后不会更新。改为 `WORKSPACE_LAYOUT_KEYS` + `workspaceLayoutLabel()` 惰性求值。
- **操作名抽成函数**：中止/继续/跳过三处原来各自拼 `isRebasing ? "变基" : ...`，抽出 `currentOperationName()`，否则同一句中文要在三个地方翻译。
- **后端枚举值继续不译**：`HistoryView` 里 `hit.change === '新增' / '移除'` 是匹配后端返回值的分支，翻译会直接让内容考古的颜色标记失效，保留并保持原样。
- 多行模板字符串（fixup 确认、范围拣选确认）合并为单个 key，避免 `+` 拼接的片段被逐段翻译后语序错乱。

累计：i18n 目录从 227 → **616 条**（zh / en 逐 key 对齐），全仓硬编码中文从 30,440 字降到 **27,127 字 / 83 个文件**。

### 第九轮：ReviewSource 适配器（数据层统一）

前面只合并了入口，数据层仍是两套。这一轮补上：`services/reviewSource.ts` 定义一份契约，本地会话与远端 PR 各自实现，视图只认接口。

**契约**

| 成员 | 说明 |
| --- | --- |
| `list(query)` | 统一列表：`ReviewItem[]`（`id` + 平台字段 + `verdict` + `progress`），并附 `nativeItems` 给来源自己的列表组件 |
| `detail(id)` | 统一详情：文件 / 评论 / 检查 / 提交数，`native` 保留来源原始负载 |
| `diff(id, path)` | 单文件补丁；本地按需拉取，远端复用已缓存详情里的 patch |
| `addComment(id, input)` | 行内或普通评论 |
| `setFileVerdict?(id, path, verdict, note)` | 逐文件结论（仅本地） |
| `submitVerdict?(id, verdict, body)` | 整体结论（仅远端）；`pending` = 仅评论 |
| `resolveComment?` / `deleteComment?` | 评论生命周期（仅本地） |
| `merge?(id, method)` / `clear?` / `exportMarkdown?` | 平台特有动作 |

**三处设计决策**

1. **能力用 `capabilities` 声明，不用 `if (kind === "remote")`**。视图问「你能逐文件下结论吗」，而不是「你是哪家平台」。本地 `perFileVerdict: true / overallVerdict: false`，远端反过来；`merge` / `checks` / `token` 同理。
2. **统一的是结论语义，不是字符串**。UI 的 `approve` / `request_changes` / `comment` 先归一到 `approved` / `changes` / `pending`，再由 `reviewEventOf()` 翻回平台事件。以后接 Gitee 只需在自己的适配器里映射一次。
3. **保留 `native` 逃生口而不是削平模型**。本地会话的「逐文件通过数 / 进度 / 提交清单」在远端没有对应物，硬塞进统一模型只会两边都别扭。`native` 明确标注「只有来源自己的视图可以读」。

**错误也统一**：`ReviewSourceError` 带 `not_found` / `no_token` / `unsupported` / `invalid_id` 四种码，视图按码出本地化文案。这样 GitHub 的 404 和「本地会话文件没了」显示同一句「未找到评审会话」。

**改造范围**：`PrView` 的 8 个 `gitService.*PullRequest*` 调用、`ReviewView` 的 9 个 `*ChangeReview*` 调用全部改走适配器；`reviewSource.ts` 的后端依赖通过 `Pick<typeof gitService, …>` 注入，测试用假后端跑，不触网。

**接第三个源的成本**：新增一个 `xxxReviewSource()`（实现同一接口 + 映射到 `ReviewItem`/`ReviewDetail`），在 `createReviewSource()` 里加一个分支。视图零改动。

### 第十轮：i18n 命令面板 + 工作区清理页

| 范围 | 文件 | 新增 key |
| --- | --- | --- |
| 命令面板 | `CommandPalette.vue` | 129（93 条命令标题、17 个分类、19 条面板外壳文案） |
| 工作区清理 | `tools/CleanTab.vue` | 94（清理 / 敏感信息扫描 / 暂存大文件 / 忽略规则体检 / 空目录 + 全部 toast 与确认） |

**命令面板的关键改动**：`commands` 是模块级数组，直接在数组里写 `t()` 会在 import 时求值，切语言后不更新。改为**标题存 key**（`title: "cmd.goto-workspace"`），由 `titleOf()` / `categoryLabel()` 惰性解析；搜索也改成匹配「用户实际看到的文字」，而不是 key。

**分类改成稳定 id**（`nav` / `repo` / `git` / `lfs` …），顺带修掉一个潜在 bug：折叠状态持久化在 `localStorage` 里，原来存的是中文分类名，切到英文后折叠状态会全部失效；现在存的是 key，与语言无关。

**新增一条契约测试**：`t()` 扫描看不到 `t(cmd.title)` 这种动态调用，所以另加一条直接从 `CommandPalette.vue` 源码里抽 `title: "cmd.*"` 与 `category: "*"`，断言两者都能在中文目录里找到 —— 拼错 key 会红，而不是界面上显示 `cmd.xxx`。

**注**：`services/gitService.ts` 里还有 842 个中文字符，但那是**浏览器预览用的 mock 数据**（`bun dev` 无 Wails 时的假提交、假分支、假评审），不属于界面文案，不需要翻译。

累计：i18n 目录 616 → **841 条**；全仓硬编码中文 27,127 → **25,020 字 / 82 个文件**（其中 `gitService.ts` 842 字为 mock 数据）。

### 第十一轮：i18n 两个最大工具页（Maintenance / LFS）

| 范围 | 文件 | 新增 key |
| --- | --- | --- |
| 仓库维护与健康 | `tools/MaintenanceTab.vue` | 226（GC、LFS prune、fsck、hooks、归档/补丁/Bundle、综合体检、对象库优化、压缩提交、崩溃反解、热更清单） |
| Git LFS | `tools/LfsTab.vue` | 212（规则、锁、缓存、调优、修订历史、历史迁移向导、只读保护、推送前校验） |

这两个文件是全仓最难迁移的：单文件 1,500 行、字符串密集、大量「状态机文案」（`isXxx ? "处理中..." : "执行"`）和多段确认弹窗。

**做法**：把「动作名」抽成 key 而不是拼接中文，例如 `t(enable ? "maint.confirm.hook.enable" : "maint.confirm.hook.disable", { name })`；多段 `+` 拼接的确认弹窗（LFS 迁移的两道确认、僵尸锁破除）整段合并成一条带 `\n` 的 key，避免逐段翻译后语序错乱。

**踩坑记录**：Python 脚本里写 `"\n"` 会真的换行，写进 TS 双引号字符串直接语法错误。这两轮共踩了 7 次，已写了一个通用修复脚本（把跨行字符串重新合并并转义），并把「写目录后必须跑 i18n 契约测试」固化成流程。

累计：i18n 目录 841 → **1,279 条**；全仓硬编码中文 25,020 → **21,426 字 / 82 个文件**。

### 第十二轮：i18n 四个主视图/工具页

| 范围 | 文件 | 新增 key |
| --- | --- | --- |
| 引擎资产质检 | `tools/EngineQualitySection.vue` | 209 |
| 源码浏览器 | `workspace/SourceExplorer.vue` | 72 |
| 远程与工作树 | `tools/RemotesTab.vue` | 105 |
| 分支页 | `BranchesView.vue` | 121 |

**分支页踩了一个坑，值得记下来**：先用「按行号替换」做，但前面把 `CATEGORY_META` 常量块换成了 9 行更长的版本，后续所有行号整体偏移，结果把 90 多处写错位置，文件语法崩掉。恢复办法是从 git 取回该文件，改用**字符串精确匹配替换**（每个片段断言 `count === 1`），完全不依赖行号 —— 后续几个文件都改用这种方式，没有再出错。

另一处：Python 里写 `\"` 才能生成 TS 字符串里的转义引号，写成 `"` 会让值里的引号提前闭合。已加了一个批量转义脚本，并复核全目录引号平衡。

累计：i18n 目录 1,279 → **1,786 条**；全仓硬编码中文 21,426 → **17,549 字 / 81 个文件**。

### 第十三轮：i18n 引擎页 / 多仓总览 / 二分定位

| 范围 | 文件 | 新增 key |
| --- | --- | --- |
| 引擎页 | `tools/EngineTab.vue` | 117 |
| 多仓总览 | `RepoDashboard.vue` | 69 |
| 二分定位 | `BisectPanel.vue` | 60 |

`BisectPanel` 又踩了一次行号偏移（`JUDGE_LABEL` 常量块少了一行），同样用 `git checkout` + 字符串匹配重做。**结论已经明确：只要文件里要先做一次「块替换」，就不能再按行号改后续行** —— 一律用字符串匹配，`assert count === 1`（重复项显式处理）。

累计：i18n 目录 1,786 → **2,032 条**；全仓硬编码中文 17,549 → **15,437 字 / 79 个文件**（其中 `gitService.ts` 842 字为浏览器预览 mock）。

### 第十四轮：i18n 变基弹窗 / 子模块 / 欢迎页 / Studio 页

| 范围 | 文件 | 新增 key |
| --- | --- | --- |
| 变基弹窗 | `RebaseModal.vue` | 51 |
| 子模块与子树 | `tools/SubmodulesTab.vue` | 82 |
| 欢迎页 | `RepoOnboarding.vue` | 57 |
| Studio 协作 | `tools/StudioTab.vue` | 86 |

`RepoOnboarding` 里 `msg.includes("不是一个有效的 Git 仓库")` 这类**是匹配 git 输出的判定逻辑，不是界面文案**，保持原样未译 —— 翻译会让「目录不是仓库」的识别直接失效。

累计：i18n 目录 2,032 → **2,308 条**；全仓硬编码中文 15,437 → **13,495 字 / 78 个文件**。

### 第十五轮：i18n 发布向导 / 变更列表 / 同步弹窗

| 范围 | 文件 | 新增 key |
| --- | --- | --- |
| 发布向导 | `ReleaseWizard.vue` | 71 |
| 变更列表 | `workspace/ChangesList.vue` | 58 |
| 同步弹窗 | `SyncModal.vue` | 56 |

`ChangesList` 里的状态元数据是**函数返回值**（`switch (status)` 里 `return { title: ... }`），每次渲染都会重新执行，所以直接在函数里调 `t()` 就是响应式的 —— 不需要像模块级常量那样额外包一层。

累计：i18n 目录 2,308 → **2,493** 条；全仓硬编码中文 13,495 → **11,371 字 / 76 个文件**。

### 第十六轮：i18n Studio 运维页 / 体积页

| 范围 | 文件 | 新增 key |
| --- | --- | --- |
| Studio 运维 | `tools/StudioOpsSection.vue` | 85 |
| 体积与大文件 | `tools/DiskTab.vue` | 57 |

**修正上一轮的一个误判**：`SourceExplorer` 剩余的 428 个中文字符**全部是代码注释**（`// 源码搜索：…`、`<!-- 10 万级仓库… -->`），没有界面文案，该文件实际已完成。之前的统计把注释也算进去了，后续判断「是否完成」要看**字符串字面量和模板文本**，而不是中文字符总数。

累计：i18n 目录 2,493 → **2,635** 条；中文字符（含注释）13,495 → **10,420**。

### 第十七轮：i18n 收尾 —— 剩余 40+ 组件全量迁移

上一轮把「是否完成」的判断口径改成**字符串字面量 + 模板文本**之后，重扫剩余中文（排除注释）得到 9,174 字，其中 `services/gitService.ts` 的 769 字是**开发预览 fixture**（`getDevMockResponse()`，仅在 Vite 独立预览、没有 Wails 运行时才会走到，桌面端不可达）。真正待迁移约 8,400 字 / 40 个组件。

| 批次 | 文件 | 处理方式 |
| --- | --- | --- |
| 设置 | `settings/ShortcutsSection`、`utils/shortcuts`、`utils/settingsIndex`、`SettingsView`、`ToolsSection`、`AdvancedConfigSection`、`SigningSection`、`IdentitySection`、`AppearanceSection`、`composables/useAccent`、`ProxySection` | 注册表（快捷键 9 条动作、设置索引 18 条、强调色 6 组）改为**存 key、渲染时 `t()`**，语言切换即时生效 |
| 历史 | `history/CommitInspector`、`CommitGraphList`、`HistoryFilterBar`、`BatchActionToolbar` | 大补丁预览、Git Notes、批量操作全部双语 |
| 分支 | `branches/BranchCompare`、`LocalBranchList`、`RemoteBranchList`、`TagsList`、`StashesList`、`MergeBranchModal`、`RebaseBranchModal`、`RenameBranchModal`、`CreateBranchModal`、`CreateStashModal`、`CreateTagModal` | 右键菜单、roving tabindex 行、range-diff、变基/合并/重命名弹窗 |
| 评审 | `review/FileDetailPanel`、`ChangeList`、`SessionList`、`ExportModal`、`review/meta.ts`、`ReviewView` | `StatusMeta.label` → `labelKey`；`verdictTextOf()` 内部 `t()`，`verdictChipOf()` 改为按 **key** 判定颜色（否则切语言后色块会错位） |
| 工具 | `tools/EngineDemoSection`、`EngineBootstrapSection`、`StudioIntegrationSection`、`ReflogTab`、`SparseTab`、`TerminalTab`、`FixCenterSection`、`ToolsView`、`StudioOpsSection`、`DiskTab` | 诊断接口目录、TAPD/CI、reflog 救砖、稀疏检出、终端、修复中心 |
| 其它 | `App.vue`、`QuickOpen`、`GitignoreEditor`、`assets/AssetDiffModal`、`utils/assetDiff`、`workspace/DiffViewer`、`SourcePanorama`、`ConflictBanner`、`CommitComposer`、`ChangesList`、`ui/ConfirmDialog`、`ui/UiSkeleton`、`ui/UiContextMenu`、`pr/meta.ts`、`pr/PrFileDiff` | 包含「跳到主内容」无障碍跳转链接、右键菜单 aria-label、骨架屏 aria-label |

**这一轮踩到的坑（都已修）**：

1. **`v-for` 循环变量遮蔽 `t`**：`TagsList` 原来是 `v-for="(t, index) in visible"`，`ToolsSection` 是 `v-for="t in detectedTools"`。加了 `t("...")` 之后 `t` 变成数据对象，模板直接炸。两处都把循环变量改名为 `tag` / `tool`。
2. **key 必须全小写**：`engine.demo.texturePacking.name`、`engine.demo.hotfixBackport.name` 被契约测试拦下，统一小写成 `texturepacking` / `hotfixbackport`（组件与目录同步改）。
3. **目录写入的多行文案**：`
` 在 Python 里被解码成真换行，写进 TS 字符串就语法错误（本轮又踩一次）。`addkeys.py` 的 `esc()` 现在会转义 `
` / `
` / `	`，并保留了一个「按行合并 + 重新转义」的修复脚本。
4. **`t()` 返回值参与逻辑判断**：`ReviewView` 的 `progressBarClass` 原来 `switch (verdictText.value)` 比中文字面量，翻译后立刻失效；改成 `switch (verdictKeyOf(...))`。

**新增第四条 i18n 契约测试**：`no hardcoded UI copy` —— 递归扫描 `src/**/*.{vue,ts}`，剥掉 `<!-- -->` / `/* */` / `//` 注释后断言不再出现 CJK 字符。白名单只有三类：语言目录本身、语言名（`简体中文` 属于 endonym，任何语言下都保持中文）、以及**后端协议值**（`不是有效的 Git 仓库` 用于匹配 Go 侧报错、`新增/移除/修改` 是 pickaxe 的分类回传值）。这样「以后谁再写死一句中文」会直接红。

最终口径：

| 指标 | 值 |
| --- | --- |
| i18n 目录 | **3,702 条** × 2（zh / en 逐 key 对齐） |
| 界面硬编码中文 | **0**（守卫测试覆盖） |
| 仍含中文的源码 | 4 个文件 / 46 字，全部是协议值、注释或语言名 |

### 第十八轮：运行时验证 + 后端文案协议

第十七轮之后跑了**真实浏览器冒烟测试**（Playwright + 系统 Chrome，连 Vite 开发服务器），这一步抓出了单测和类型检查都看不到的问题。

#### 1. 冒烟测试覆盖

`/tmp/smoke/smoke.cjs`（浅层）+ `deep.cjs`（深层）：中英双语各走一遍

- 7 个主视图（工作区 / 历史 / 分支 / 储藏 / 评审 / 工具 / 设置）
- 14 个工具子页（终端 → 发布向导）
- 7 个设置分区
- 命令面板（⌘K，含输入后过滤）、快速打开（⌘P）

断言三条：**无 console error / 无 `[i18n] missing key` / 英文模式下无残留中文**。首轮结果：错误 0、缺 key 0 —— 但英文模式下历史页整列时间戳是「刚刚 / 3 小时前」。

#### 2. 根因：`%cr` 由 git 自己本地化

后端用 `git log --format=...%cr...` 拿相对时间，`%cr` 是 **git 按进程 locale 渲染**的，所以系统是中文时切到英文界面仍然是中文。修法：前端不再依赖 `relativeDate`，改用 `utils/relativeTime.ts` 从 ISO `date` 重新算，走 `Intl.RelativeTimeFormat(locale)`：

```ts
relativeTime(iso, fallback, now = Date.now())  // 30 秒前 / 3 hours ago / 昨天 / last year
```

接入 6 处：提交图谱行、提交检视器、版本对比两栏、同步弹窗、变基弹窗。后端字符串保留为 `fallback`（时间戳解析不了时用）。附带 5 条单测（中英 / 未来时间 / 非法输入 / 空值）。

#### 3. 后端文案协议：`key + params`，文本兜底

浏览器预览用的是 dev fixture，所以「发布向导的检查项是中文」这条线索指向了真实问题：**Go 侧生成的可读文案是中文的**，英文界面下会原样显示。

本轮建立了统一协议，先落到发布计划（`ReleaseCheck` / `ReleasePlan`）：

```go
type ReleaseCheck struct {
    Key       string            `json:"key"`
    Title     string            `json:"title"`      // 已渲染文本，CLI/日志可独立使用
    TitleKey  string            `json:"titleKey,omitempty"`
    Detail    string            `json:"detail"`
    DetailKey string            `json:"detailKey,omitempty"`
    Params    map[string]string `json:"params,omitempty"`
    ...
}
```

前端新增 `backendText(key, fallback, params)`：**key 在目录里就用目录，否则回退到后端文本**，未知 key 不会把 `release.check.commits.title` 原样漏到界面上。发布向导的 5 项检查（提交 / 工作区 / 分支 / 远端 / 签名）+ 总结句共 20 条 key，全部双语。

外部工具简介同理：Go 侧 `git_external_tools.go` 的 7 个工具（cursor / vscode / beyondcompare / idea / sublimemerge / opendiff / meld）按 id 映射到目录，后端描述作兜底。

#### 4. 跨语言契约测试（两条新的）

- **Go → 目录**：扫 `git_release.go` 里的 `"release.check.*"` / `"release.summary.*"` 字面量，断言每个都在中英目录里存在。
- **Go id → 目录**：扫 `git_external_tools.go` 的 `ID: "..."`，断言前端映射表 + 英文目录都覆盖。

Go 侧同时补断言：每个 `ReleaseCheck` 必须带 `TitleKey` / `DetailKey`，且 `DetailKey` 用到的占位符必须都在 `Params` 里 —— 少一个参数就会渲染出 `{count}` 原文。

#### 5. 复测结果

英文模式下残留中文只剩三类，都属「不该翻译」：**提交标题与 changelog 条目**（用户数据）、**dev fixture 的磁盘建议**（仅浏览器预览存在，真实后端没有该字段）、**`简体中文`**（语言自称）。

### 第十九轮：分支洞察与标点收尾

第十八轮留下的最后一块「后端中文」在分支页头部：`BranchInsightSummary.Hint` 是 Go 侧拼的句子（`有 1 个分支已合并进 main，可以安全清理`）。

#### 1. 不新增协议，改成前端重算

`summary` 里已经带了 `merged / unpushed / stale / staleDays / baseBranch` 五个计数，前端完全能自己拼这句话 —— 而且分类文案 `branch.cat.<category>.desc` 早就在目录里了。所以：

- `LocalBranchList` 新增 `insightHint` computed，按「merged → unpushed → stale → healthy」优先级用计数拼句；
- 行悬停提示从 `insight.suggestion`（后端中文）改为 `row.meta.desc`（已本地化的分类描述）；
- `BranchesView` 的清理确认框同样改用 `meta.desc`，并顺手修掉一处硬编码的「。」—— 改成 `branch.cleanup.reason`（`{text}。` / `{text}.`）；
- 删掉不再使用的 `branch.cleanup.suggestion` 键，并从 `branch.confirm.cleanup.one` 模板里移除 `{suggestion}` 占位符。

**收益**：后端不需要改一行代码，分支页在英文下彻底干净。

#### 2. 守卫测试补上 CJK 标点

原来的 `no hardcoded UI copy` 只匹配 `[\u4e00-\u9fff]`（汉字），**漏掉了标点** —— 而 `。`、`、`、`；` 恰恰是最容易在字符串拼接里漏掉的一类。本轮把范围扩到：

```
[\u4e00-\u9fff\u3000-\u303f\uff01-\uff60]
```

扩完立刻抓出 5 处：`join('、')`（引擎页两处）、`join('；')`（插件问题列表两处）、`title="content = git log -S；regex = git log -G"`（历史页）。全部改成走目录，并顺手把散落的 4 个分隔符键（`settings.shortcuts.list.separator` / `assets.impact.owners.separator` / `branches.merge.modal.overlap.separator` / `engine.bootstrap.list.separator`）收敛成两个共享键 `common.list.comma` / `common.list.semicolon`。

白名单只多了一条：`App.vue` 与 `RepoOnboarding.vue` 里从后端报错中提取目录路径的正则 `/['"「]([^'"」]+)['"」]/` —— 它必须包含中文引号才能匹配。

#### 3. 复测

双语冒烟再跑一遍：`errors 0 / missingKeys 0`，英文模式残留中文只剩**提交标题、diff 内容、评审评论正文**（fixture 用户数据）与 **`简体中文`**（语言自称）。

### 第二十轮：操作日志与修复中心

继续按「持久展示的卡片文案要迁移、一次性 toast 与逐文件明细先保留」的界线推进。

#### 1. 操作日志（Reflog 页）—— 零 Go 改动

`classifyReflog` 早就返回了稳定的 `action` key（`commit` / `amend` / `merge` / `reset` / `cherry-pick` …），只是前端一直在显示它旁边的中文 `actionLabel`。于是：

- 新增 `actionLabel(entry)` / `undoHint(entry)`，按 `reflog.action.<key>` / `reflog.undo.hint.<key>` 取目录，未知 key 回退后端文本；
- 撤销横幅那句话改为从 `entries` 重算（`reflog.oplog.message.undoable/single/not.undoable`），不再用后端的 `message`；
- 撤销成功后的 toast 也自己拼（`reflog.undo.done` + reset 时的 `reflog.undo.done.hard`），因为 `UndoResult` 里的 hash/action 都是结构化字段。

#### 2. 修复中心 —— 用 `key + params`

`FixCenterItem` 与 `FixCenterResult` 按第十八轮的协议补字段：

```go
TitleKey / SummaryKey / FixLabelKey + Params
MessageKey / MessageParams
```

4 个条目（敏感信息 / 大文件 / 僵尸锁 / 忽略规则）共 12 条 key，加上 3 条摘要句。参数都是纯数字或尺寸，`owners` 用 `", "` 连接以保证中英都读得通。

顺手给浏览器预览补了 `ScanFixCenter` 的 fixture（同样带 key + params），这样开发预览和桌面端走同一条本地化路径。

#### 3. 边界（写清楚，避免下次重复讨论）

**迁移**：持久显示的卡片标题/摘要/按钮、页面级摘要句、可执行项的标签。
**不迁移**：一次性 toast 里的执行结果（`AuditFixResult.Message`）、逐文件明细（`Details`）、`Warnings`、以及所有 `fmt.Errorf` 的报错文本 —— 这些更接近 git 自己的输出，属于「诊断数据」，与提交标题、diff 内容同类。

#### 4. 新增两条跨语言契约测试

- 扫 `git_oplog.go` 的 `return "<key>", "` → 断言 `reflog.action.<key>` 存在，且当 Go 侧带 hint 时 `reflog.undo.hint.<key>` 也存在；
- 扫 `git_fix_center.go` 的 `TitleKey/SummaryKey/FixLabelKey` → 断言都在中英目录里。

Go 侧同步补断言：`ScanFixCenter` 返回的每个 item 必须三个 key 齐全，敏感信息项的 `hits/files/critical` 参数不能为空。

### 第二十一轮：合并重叠与资产影响

两处都是「弹窗里常驻的一句话」，按第二十轮划的界线属于要迁移的一侧。

| 结构 | 变体 | 参数 |
| --- | --- | --- |
| `MergeOverlapResult.Message` | 4（无交集 / 1–2 文件 / 3–5 文件 / 高冲突） | `mine,target,days` / `count,files` / `count,authors` / `count,people` |
| `AssetImpactResult.Message` | 5（不完整 / 无引用 / ≤3 / ≤10 / 高影响） | `total,direct,indirect,owners,limit` |

资产影响那句在 Go 里会按条件追加两段后缀（「二级引用已按前 N 个截断」「扫描不完整，实际影响可能更多」）。这里没有把后缀也塞进 params，而是新增 `MessageSuffixKeys []string`：**Go 仍然是「哪些备注适用」的唯一判定方**，前端只按顺序取 key 拼接：

```ts
const base = backendText(info.messageKey, info.message, info.messageParams);
const suffixes = (info.messageSuffixKeys ?? [])
  .map((key) => backendText(key, "", info.messageParams))
  .join("");
return info.messageKey ? base + suffixes : base;
```

`backendText(key, "")` 在目录缺 key 时返回空串，所以后缀丢了也不会出现 `assets.impact.message.truncated` 这种原文。列表参数（文件名、作者）在 Go 侧统一用 `", "` 连接，中英都读得通。

**踩坑**：替换 `switch` 分支时把 `switch {` 重复插了一次，gofmt 直接报 `expected '}', found 'switch'` —— 改 Go 的 switch 块时，old_string 要从 `case` 开始，别带上 `switch {`。

契约测试补两条（扫 `git_closed_loop.go` / `git_asset_impact.go` 的 key 字面量），Go 侧也断言 key 与参数齐全。

### 第二十二轮：仓库健康报告

健康体检是最后一块「持久显示的后端文案」——6 个维度 × 标题 + 详情，外加一句总结。

#### 1. 标题 key 由维度 key 派生

`HealthDimension.Key` 本来就是稳定的（`identity` / `objects` / `ignore` / `largefiles` / `signing` / `workspace`），所以 `TitleKey` 直接在 `add()` 里拼成 `health.dim.<key>.title`，调用点不用多传一个参数。

#### 2. `add()` 多收一个 `healthLoc`

```go
type healthLoc struct {
    detailKey  string
    params     map[string]string
    suffixKeys []string
}
```

20 个调用点各自补一个 `healthLoc{...}`。`ignore` 与 `signing` 的详情是「基础句 + 条件后缀」（补齐规则 / 误跟踪产物 / 合规率），沿用第二十一轮的 `suffixKeys` 机制。

#### 3. 「进行中的操作」用 slug 传参

`MERGE_HEAD → 合并未完成` 这张表原本只存中文标签。改成 `map[string][2]string{文件: {中文兜底, slug}}`，`Params["operation"]` 传 slug（`merge` / `rebase.interactive` / `cherry.pick` …），前端自己解析 `health.workspace.op.<slug>`。

#### 4. 顺手修掉 `topFileNames` 的硬编码

原来 `strings.Join(names, "、")` 再拼 `" 等 N 个"`。改成 `", "` 连接 + `" …"` 省略号 —— 语言中立，也不用再翻译。

#### 5. 预览 fixture

给 `RunRepoHealthCheck` 补了 6 个维度的 fixture（带 key + params + 后缀），这样浏览器里点「Run health check」就能验证整条本地化链路。截图里那 6 行详情就是走 key 渲染的。

契约测试补三条：`git_health.go` 里的 `health.*` key、6 个维度标题 key、以及「进行中操作」的 slug。

### 第二十三轮：无障碍扫描 + 两个测试/仓库卫生问题

#### 1. 用浏览器扫了一遍「无名称的可交互元素」

写了个 Playwright 脚本遍历 7 个主视图 + 命令面板 / 快速打开 / 同步弹窗，找出所有没有可访问名称的 `button / a[href] / input / select / textarea`（判定顺序：`aria-label` → `aria-labelledby` → 文本 → `title` → 内含 `img[alt]` → 最近的 `<label>` 文本）。

首轮报 5 个，逐条看下来：

- **真问题 2 个**：变更列表里每行文件的勾选框既没有 `<label>` 包裹也没有 `aria-label`，读屏只会念「复选框」。补 `workspace.changes.select.file`（`选择文件 {name}`）。
- **误报 3 个**：评审页的 base / head / 行号输入框其实被 `<label>` 包着（隐式标签），是脚本没算这种情况。脚本补上 `el.closest("label")` 判定后归零。

最终：**无名称可交互元素 0 个**（含三个浮层）。

#### 2. `TestGitService` 依赖工作区干净

`-race -shuffle=on` 跑出一个失败：`ExecuteCustomAction(".", "act_diff_check")` 在**当前仓库**执行 `git diff --check`，而工作区里 `frontend/bindings/wgit/models.ts` 有两行 JSDoc 带行尾空格 → 命令返回 2 → 断言 `Success` 失败。

也就是说这个用例把「开发者本地有没有行尾空格」变成了测试结果的一部分。改成在 `t.TempDir()` 里初始化一个干净仓库再执行，测试就不再依赖工作区状态。顺手把那两行行尾空格清掉，`git diff --check` 现在干净。

复跑 `go test -race -shuffle=on .`：**exit 0，107.7s**。

### 第二十四轮：配色对比度（WCAG AA）

上一轮的无障碍扫描只回答了「每个控件有没有名字」，没回答「文字看不看得清」。这轮补上对比度，分两层测：

| 层级 | 做法 | 覆盖 |
| --- | --- | --- |
| token | 读 `data-theme` × `data-accent` 下**解析后**的 CSS 变量，算「实心填充 + 标签墨色」「墨色 + 表面」的比值 | 12 组合 × 21 组配对 |
| 界面 | 逐元素取 `color` 与**合成后**的背景（半透明层沿祖先链压到 canvas），按字号/字重选 4.5:1 或 3:1 | 12 组合 × 6 主视图 + 6 组合 × （14 工具子页 + 7 设置分区 + 2 浮层）= 138 步 / 26 种布局 |

首轮结果：token 41 处、界面 374 处。归并后只有四类真问题：

**1. 两个 contrast token 从来没映射到 Tailwind。** `--ug-accent-contrast` 在 tokens.css 和 6 个强调色 preset 里都定义了（teal / green / amber 在深色主题下是**深色**墨色），但 `@theme` 里没有对应项，于是 `text-accent-contrast` 是个不存在的类，组件清一色写 `text-white`。结果：teal 实心按钮 3.74、green 3.45、amber 2.62、rose 4.36。`text-success-contrast` 同理（LfsTab / CleanTab 用了 3 次，全是死类）。

**2. 实心色的 hover 方向搞反了。** 深色墨色的预设（teal / green / amber）需要 hover **变亮**才能保住对比度，原来的 hover 反而更深：teal `#0f766e` 只有 3.09、green `#26863a` 只有 3.70。

**3. 浅色主题的小字吃紧。** `--ug-text-muted` 在 raised 上的 9px 是 4.33，`--ug-success` 是 4.24 —— 都卡在 4.5 边缘下面。

**4. 4 处装饰性竖线拿边框色当文字色。** `<span class="text-line">|</span>` 的对比度是 1.66（深色）/ 1.34（浅色），既看不清也不该按文字来测。

修法：

- `@theme` 补 `--color-accent-contrast` / `--color-success-contrast`；`uiVariants.primary` 改 `text-accent-contrast`；61 处组件的 `text-white` 同步替换（源码守卫测试锁住这条）。
- 实心色按「白字 ≥ 4.5」重算：teal `#12a594→#0f766e`、pink `#d6409f→#c02682`、rose `#e0316b→#c2255c`、violet `#7c5cff→#6b4ae0`、浅色 teal `#0d9488→#0f766e`、浅色 green `#2f9e44→#237032`；深色墨色的 hover 改为变亮（teal `#14b8a6`、green `#37b24d`、amber `#f59f00`）。
- 浅色墨色整体压深一档：muted `#6b6c75→#63646d`、success `#1f7d54→#1a6b48`、warning `#9a5b00→#834d00`、danger `#ce2c31→#a72428`、accent `#395bc7→#3453b5`，5 个强调色 preset 的 ink 同步压深。
- 4 处 `|` 换成 `w-px h-3 bg-line` 的真分隔线（`aria-hidden`）；同步徽标 `bg-white/20 text-white` → `bg-accent-contrast/15 text-accent-contrast`；品牌渐变 `from-accent` → `from-accent-strong`。

**两个测量陷阱（都踩过）**：

1. **切主题后立刻测会读到插值中间色。** 颜色有 `transition`，120ms 后取到的是「白 → 深青」的 92% 中间值 `rgb(24,51,49)`，凭空报出 4.39。脚本改为先注入 `transition: none !important; animation: none !important`。
2. **导航失败会被误读成「全部通过」。** 深度脚本原本先点「English」再点英文 dock 标签，但设置页没打开、语言没切，dock 仍是中文标签 → 后续 6 个组合都在测同一页，「0 失败」是假的。改为 `addInitScript` 写 `localStorage["wgit.locale"]`，并记录每步 `textNodes`：`distinctLayouts = 26` 才能证明真的走了 26 种布局。

契约测试 `tests/contrast.test.ts`（14 条）：解析 `tokens.css` + `accent-presets.css`，按 12 组合断言 21 组配对 ≥ 4.5；再加一条源码守卫 —— `bg-accent*` 的 class 属性里不允许出现 `text-white`。

最终：token **41 → 0**、界面 **374 → 0**、深度 **138 步 → 0**。

### 第二十五轮：Liquid Glass 材质

玻璃只给**框架**和**浮层**：顶栏、仓库标签栏、活动栏（dock），以及全部模态 / 命令面板 / 快速打开 / 同步 / 变基 / 右键菜单 / Toast。内容面板保持不透明 —— 玻璃是浮在应用之上的材质，不是阅读面的滤镜，否则每一行 diff 都要和背景糊在一起。

#### 1. 材质由四层叠加而成

`src/styles/glass.css` 定义 `.glass` + 两个强度变体（`.glass-bar` 框架 / `.glass-panel` 浮层）：

| 层 | 作用 |
| --- | --- |
| `background-color` + `backdrop-filter: blur() saturate() brightness()` | 半透明 tint 叠在过饱和的模糊背景上 |
| `box-shadow: inset 0 1px 0 <高光>, inset 0 -1px 0 <暗线>` | 上下两道 1px —— 这层才是「厚度感」的来源 |
| `::before` 径向高光（指针跟随）+ 顶部线性色晕 | 液体的那一半：高光跟着鼠标走 |
| 外层宽柔阴影 + 贴地阴影 | 浮层离开页面 |

指针跟随用 `@property --glass-x/--glass-y` 注册两个自定义属性，浏览器才能**插值**它们 —— 高光在采样点之间滑过去，而不是跳。`::before` 用 `z-index: -1` 画在隔离层内，压在 tint 之上、文字之下，所以再亮的高光也不会糊住标签。

应用根上加了一层 `app-backdrop` 极光（三层 ≤13% alpha 的强调色径向渐变），玻璃才有东西可折射 —— 否则 blur 的是一块纯色，看起来只是「浅一点的条」。

#### 2. 可访问性是这个材质的一部分

- tint 恒定 ≥ 80% 不透明（`--glass-tint` 的 alpha 写在测试里），把「背景漂移」限制在小数点后一位；
- `prefers-reduced-transparency: reduce` → 退化为不透明表面、去掉 `backdrop-filter` 与高光；
- 不支持 `backdrop-filter` 的引擎 → 同样的退化（`@supports not (...)`）；
- `prefers-reduced-motion: reduce` → 关掉高光过渡。

用 CDP 的 `Emulation.setEmulatedMedia` 实测降级：正常态 `oklab(0.214 0.001 -0.004 / 0.86)` + `blur(26px) saturate(1.75) brightness(0.9)`；降级态 `rgb(25,25,27)` + `none` + 高光 `display: none`。

#### 3. 顺带修掉的盲区：审计读不懂 OKLab

要给玻璃测对比度，就得让脚本读得到 `--glass-tint`。这时发现审计的解析只认 `rgb()/rgba()`，而 `color-mix(in oklab, …)` 的计算值序列化成 **`oklab(L a b / alpha)`** —— 解析失败后代码 `continue`，于是**所有带透明度修饰的文字都被静默跳过**。补上 OKLab→sRGB 转换后，界面审计立刻从 0 涨到 **182 处**真实违例：上一轮那个「0 处」有一部分是盲区。

修法：

- 47 处 `text-ink-muted/70|60|50|80` 全部改回全量墨色（`/0` 的 hover 显形技巧保留）；
- 计数徽标不能拿「自身墨色的 15% 当底色」—— 白字被自己压到 3.93，改成 1px 内描边（`ring-1 ring-inset ring-accent-contrast/40`）；
- 浅色 success `#1a6b48→#196644`、cyan `#0d6f87→#0b6277`、purple `#8145b5→#753fa5`，才能在「15% 自色底 + hover 行」这个最差组合上过 4.5。

#### 4. 玻璃的对比度是量出来的，不是算出来的

`backdrop-filter` 让「实际画出来的背景」变成 tint、极光与背后内容的混合，CSS 层面的推算不够。所以另写了一个采样脚本：截取每个玻璃表面的无文字条带，取最亮/最暗像素当最差情况，再和墨色 token 比。

| 表面 | 主题 | 背景范围 | primary | secondary | muted |
| --- | --- | --- | --- | --- | --- |
| 顶栏 | 深 | `#151516–#18191c` | 15.2 | 8.4 | 5.3 |
| 活动栏 | 深 | `#171719–#18181b` | 15.3 | 8.5 | 5.4 |
| 顶栏 | 浅 | `#f1f2f6–#f6f7f9` | 14.7 | 5.3 | 5.3 |
| 活动栏 | 浅 | `#f9fafc–#fbfcfe` | 15.8 | 5.7 | 5.6 |
| 命令面板 | 深 | `#0e0e10–#121214` | 16.2 | 9.0 | 5.7 |

#### 5. 契约测试

`tests/glass.test.ts`（9 条）：tint ≥ 80%、两套主题都有 sheen/edge/bloom、`.glass` 必须带 `backdrop-filter` 且 `::before` 的 `z-index: -1`、两种降级都存在、reduced-motion 关过渡、每个玻璃元素必须选强度变体、玻璃元素不能同时带不透明背景工具类、`style.css` 不得重复定义玻璃类。`tests/contrast.test.ts` 追加一条：`text-ink-muted/NN`（NN>0）不允许出现。

#### 6. 覆盖到所有真正的浮层，并把交互态也做进材质

第一版只做了「最显眼的几处」。补齐后又加了 4 处：多仓总览面板、资产差异弹窗、工作区布局菜单、源码树右键菜单 —— 判据是「有没有遮罩」：有遮罩的用 `.glass-panel`（86%），直接浮在内容上的用 `.glass-popover`（94%）。

dock 的选中/悬停态原来是 `bg-raised`（不透明），在玻璃条上像打了个洞。改成 `--glass-raise` / `--glass-raise-strong`（主题墨色的 7% / 12% 洗色），悬停时材料仍然是透的。

#### 7. 撞名：`style.css` 里早就有 `.glass-panel`

给顶栏下拉菜单也套上玻璃之后，菜单变成「半透明到能看清背后的文件名」—— 但计算样式显示 `rgba(17,17,19,0.75)` + `blur(20px) saturate(1.8)`，跟新材质完全不是一个值。查下去发现 `style.css` 里早有一段 2024 年的 “Glassmorphism Containers”，定义了 `.glass-panel` / `.glass-pill` 加 `html.light .glass-panel`，用的是硬编码深色值 —— 它按源码顺序覆盖了新材质。

两个类全项目**零引用**（所有使用点都是这次新加的 `glass glass-panel`），所以直接删掉旧块，并补一条守卫：`style.css` 不允许再出现 `.glass-panel` / `.glass-pill` / `.glass-bar` / `.glass {`。材质只能有一个来源。

**顺带发现的第二件事**：菜单直接浮在内容上（没有遮罩），86% 的 tint 不足以压住背后的文件名。所以补了一个 `.glass-popover` 变体（94% tint + 30px blur），专给下拉菜单与右键菜单；模态 / 命令面板有 `bg-black/60` 遮罩，仍用 86%。

### 第二十六轮：启动屏双语化、性能基线、设计规范补齐

#### 1. 启动屏漏了一句中文

`index.html` 的 boot indicator 写着 `正在加载工作区...`，是纯静态文本 —— 英文界面下依然显示中文。它跑在 bundle 之前，用不了 i18n，所以只能内联两套文案：

```js
var LABELS = { "zh-CN": "正在加载工作区…", "en-US": "Loading workspace…" };
document.getElementById("app-boot-label").textContent = LABELS[saved] || LABELS["zh-CN"];
```

顺手把浅色下的标签色从 `#70717a` 对齐到 token 值 `#63646d`。三条契约锁住：两套文案都在、markup 里那个 div 必须是空的（否则又会写死一种语言）、脚本必须读 `wgit.locale`。

实测（`waitUntil: "commit"` 抢在 Vue 挂载前读）：`zh-CN → "正在加载工作区…"`、`en-US → "Loading workspace…"`。

#### 2. 性能基线：先解决「测不了」

想在 `vite preview`（生产包）上量性能，发现打不开工作区：`callBackend` 的 dev fixture 分支被 `import.meta.env.DEV` 门控，生产包里没有后端就只剩欢迎页。**这是架构上的一个事实**：浏览器里能跑的只有 dev server 或带 Go 后端的壳。

所以基线在 dev server 上取，并把结论限定在**相对比较**上：

| 指标 | 值 |
|---|---|
| 首屏 FCP | 88ms |
| 视图切换（点击 → 双 rAF 后已绘制） | workspace 96 / history 84 / branches 57 / tools 62 / settings 63ms |
| 提交列表滚动 2s | 120 帧，avg 16.7ms，p95 19.6ms，**卡顿帧 0，长任务 0** |

**玻璃的开销**：同一段滚动，注入 `* { backdrop-filter: none !important }` 再测一遍 —— `avg 16.7 / p95 19.7 / 卡顿 0`，与开玻璃**完全一致**。`backdrop-filter` 只在静态合成时花一次，滚动时不会每帧重算；至少在这个数据量下不需要为它做取舍。

#### 3. 设计规范文档补齐

`design-system/DESIGN_SYSTEM.md` 停留在 v1.1，没有材质层，7.1 的对比度表还是上一版的数字（甚至写着「`text-ink-muted` 不能放抬升面，4.08/3.90」，而这条早就不成立了）。这轮补成 v1.3：

- **4.5 材质：Liquid Glass** —— 三个变体、四层构成、tint ≥ 80% 的下限、交互态用 `--glass-raise`、两路降级、按渲染像素验收；
- **7.1 对比度** —— 换成当前实测表（12 组合逐对断言），并写下三条由实测反推的规则：实心填充用自己的 contrast 墨色、文本不许透明度修饰、徽章不许拿自身墨色当底色；补一句「最差组合是悬停行不是卡片」；
- **8.3c** —— v1.3 的迁移记录（新增材质与映射、实心色重算、浅色墨色压深、删除撞名的旧 `.glass-panel`）；
- **8.5 QA 清单** —— 加三条（对比度用 contrast token、文本无透明度修饰、玻璃只出现在框架与浮层），并列出四个自动化脚本替代人工走查。

### 第二十七轮：强制色彩降级 + 端到端构建验证

#### 1. Windows 高对比度下玻璃会「漏底」

用 `emulateMedia({ forcedColors: "active" })` 实测：`.glass` 的 86% tint 被系统映射成 **86% 透明的 Canvas**，于是页面内容从面板里透出来，边界消失；`backdrop-filter` 还在跑但完全没有视觉收益。

补了一段 `@media (forced-colors: active)`：

```css
.glass { background-color: Canvas; border-color: CanvasText;
         backdrop-filter: none; box-shadow: none; }
.glass::before { display: none; }
.app-backdrop { background-image: none; }
[aria-current="page"] { outline: 1px solid Highlight; outline-offset: -1px; }
```

最后一条是顺带发现的：dock 的选中态原本只是 `--glass-raise-strong`（12% 墨色洗色），强制色彩下等于没有，导航会失去「我在哪」的唯一线索 —— 改用系统 `Highlight` 描边。

实测前后：`rgba(0,0,0,0.86)` + `blur(26px)` → `rgb(0,0,0)` + `none`（浅色对称），命令面板从「半透明糊」变成「实底 + 1px 描边」。加了一条契约测试锁住这四行。

#### 2. 终于跑了一次真正的端到端

前面所有验证都在浏览器里。这轮把桌面端也跑通了：

| 步骤 | 结果 |
|---|---|
| `go build -o bin/wgit .` | 22.9 MB；`strings` 确认新前端已嵌入（`glass-panel` ×11、`app-boot-label` ×3、`glass-tint` ×1） |
| `wails3 build` | 走完整链路（Vite 构建 → `go build -tags production -trimpath -ldflags="-w -s"`），产出 15.6 MB |
| 启动二进制 | WKWebView 加载 `index.html` 与全部 assets，日志打出 `[main.ts] Vue app successfully mounted!`，15 行日志内 **0 error / 0 panic** |

顺带确认：`wails3 build` 用默认 SDK 也能过（只有 `built for newer 'macOS' version (13.0)` 的常规告警），不需要手动指定 `SDKROOT`；那是 `go test` 走 cgo 链接时才会踩到的坑。

环境限制：沙箱里 `screencapture` 拿不到窗口（`could not create image from display`），所以窗口截图欠奉，证据是运行日志。

### 第二十八轮：加载失败不再伪装成「空」

#### 1. 一个读代码读出来的 bug

`HistoryView.loadCommits` 的 catch 是这么写的：

```ts
} catch (err) {
  console.error("Failed to load commits:", err);
  commits.value = [];
  hasMore.value = false;
```

于是后端一失败，列表渲染的是**空状态**——「仓库暂无提交」。用户看到的是「这个仓库没有历史」，而不是「加载失败了，可以重试」。这类问题比崩溃更难查：界面没有任何异常信号。

改法：`loadError` 状态 + `CommitGraphList` 新增 `error` prop 与 `retry` 事件。错误块排在 loading 之后、empty 之前——顺序很重要，否则空状态会先命中：

```vue
<div v-else-if="error"> ⚠️ 提交历史加载失败 / 后端原文 / [重试] </div>
<div v-else-if="commits.length === 0"> … 空状态 … </div>
```

错误块直接展示后端原文（`fatal: unable to read tree …`）。这和既定规则一致：git 的报错属于**诊断数据**，保留原文比翻译成「加载失败」更有用。

#### 2. 怎么验的

没有 DOM 测试环境（项目里没装 `@vue/test-utils`），所以用「临时探针 + 截图 + 回滚」：

1. 在 fixture 的 `case "GetCommits"` 里插一行 `throw new Error("fatal: unable to read tree …")`；
2. 打开历史页，断言页面出现 `Could not load commit history` 并截图；
3. 删掉探针，`diff` 对比备份文件确认**逐字节一致**，再跑一遍确认恢复正常。

截图里能看到：警告图标 + 标题 + 后端原文 + 重试按钮，替代了原来的「暂无提交」。

#### 3. 其余视图有没有同样的问题

顺着查了一遍：`BranchesView.loadData` 用 `Promise.allSettled` 聚合失败并 `notify` 一条 toast；工作区状态失败也是 toast；只有历史页是「静默清空」。所以这一处补完，主视图就没有「失败当空」的路径了。

**顺带记一个失败的工具尝试**：本来想写个「点遍所有按钮、看谁抛错」的扫描脚本，跑了 10 分钟还没结束（每个按钮都要定位 + 点击 + Esc + 两段等待，14 个子页 × 28 个按钮 ≈ 400 次交互），而且容易卡在确认框上。放弃了——这种扫描要么做成 CI 里的慢任务，要么不做，半吊子版本只会浪费时间。

### 第二十九轮：窗口拖不动了 —— 玻璃盖住了拖拽区

用户报「窗口无法拖动」。这是玻璃那一轮引入的回归，原因很具体：

macOS 的 `-webkit-app-region: drag` 由 **WKWebView 自己按命中测试算拖拽区**。`.glass` 当时用了一个 `::before` 铺满整个表面画高光，命中测试就把这个伪元素当成最顶层盒子 —— 而它身上没有 `-webkit-app-region`，于是整条顶栏都不再是拖拽区。

顺带查清 Wails v3 的实现：`runtime/dist/appregion.js` 里的 `--wails-non-client-region` 只在 **Windows** 上启用（`if (os === "windows")`），macOS 完全走 WebKit 原生机制，所以 CSS 里的任何合成层都可能把它打掉。

修法分三步，把拖拽条上的合成因素全部清掉：

| 改动 | 原因 |
|---|---|
| 高光从 `::before` 改成 `.glass` 自身的 `background-image` | 背景绘制不参与命中测试，视觉完全一致（像素级复测：`#151516–#18191c`，与改前一致） |
| 去掉 `isolation: isolate` | 它只为伪元素的 `z-index: -1` 服务，现在不需要了，且会新建层叠上下文 |
| `.glass-bar` 去掉 `backdrop-filter` | `backdrop-filter` 会把元素提升为合成层，是打掉 `-webkit-app-region` 的经典原因。条状玻璃本来贴着不透明面板，模糊几乎看不出来，材质靠 tint + 高光 + 边缘 + 接触阴影成立；面板/菜单保留模糊 |
| `app-backdrop` 去掉 `background-attachment: fixed` | 根节点不滚动（内层面板滚），`fixed` 只会多一次合成层提升 |

三条契约锁住，防止以后再犯：`.glass-bar` 必须 `backdrop-filter: none`、CSS 里不许出现 `.glass::before`、`.glass` 不许 `isolation: isolate`。

**第一次修复没修好**：用户复测仍然拖不动。于是把「拖拽条上的合成因素」全部移除——高光改 `background-image`、去掉 `isolation`、`.glass-bar` 去 `backdrop-filter`、根节点去 `background-attachment: fixed`——**还是不行**。

那就说明不是某一个属性，而是**这块材质本身**：macOS 的拖拽区由 WebKit 从元素「怎么绘制」推导，`.glass` 无论哪个变体都会让它失效。所以最终做法是承认这条边界：

| 表面 | 处理 |
|---|---|
| 顶栏（拖拽条） | **不用玻璃**，回到玻璃之前的 `bg-surface/70 backdrop-blur-xl` |
| 仓库标签栏 / 活动栏 | 保留 `.glass-bar`（不在拖拽路径上） |
| 模态 / 命令面板 / 快速打开 / 菜单 / Toast | 保留完整材质（含模糊与指针高光） |

加了第四条契约：顶栏的 `<header>` 标签上必须带 `-webkit-app-region: drag`，且**不允许出现 `glass` 类**——防止以后有人「顺手统一一下」又把它玻璃化。

**诚实说明**：这个沙箱里没法合成鼠标拖拽（没有 `cliclick`、没有 Quartz），所以两轮修复都只能靠「移除已知会打掉拖拽区的因素」+ 用户实测。已重新 `wails3 build` 出 `bin/wgit`。

### 验证

- `go vet ./...` 干净；`go test ./...` 全绿（`ok wgit`，含新增 17 条 PR/MR 用例）。
- `bun run typecheck`（vue-tsc）零错误；`bun test tests` **193 pass / 0 fail**
- 冒烟：0 console error / 0 missing key
- 对比度：12 组合 × 6 视图 0 处；玻璃表面像素 muted 5.24–5.58（与去模糊前一致）（PR 补丁解析、状态元数据、确认状态机、roving tabindex、i18n 四条契约 —— 含新增的「无硬编码文案」守卫、命令面板 key 契约、ReviewSource 适配器）。
- `bun run build` 通过，首屏 `index` 440.92 kB、`PrView` 31.38 kB、`HistoryView` 84.88 kB（弹窗迁移后比迁移前小 4.8 kB）。

## 五、未实施（明确边界）

C2 已完成，报告中「迭代 1–4」的功能项已全部落地。剩余的是**非功能**欠账，不阻塞版本：

| 事项 | 现状 |
| --- | --- |
| i18n 全量迁移 | **已完成**：3,671 key × 2，界面零硬编码，`no hardcoded UI copy` 守卫测试锁定 |
| 剩余 7 处手写弹窗 | 命令面板 / 快速打开 / 全屏引导 / 多仓总览 / 高级同步 / 高级变基 / 资产差异，均因结构（全屏、自定义 header、分栏编辑器）不适配 `UiModal`，保持原样（但都已复用 `useModalA11y`，焦点陷阱与 Esc 行为一致） |
| 其余 RPC 的后端文案 | 发布计划、外部工具、修复中心、操作日志、合并重叠、资产影响、健康体检、**`ClosedLoop` 全部动作（LFS 转换 / 破锁 / range-diff / fixup / 优化 / 验签）** 已迁移：结果结构带 `MessageKey` + `Params`，前端用 `backendText()` 消费，`closed.loop.*` 21 条 key 双语齐备并有守卫测试扫描 Go 源码 |
| 后端执行结果与逐文件明细 | 一次性 toast（`AuditFixResult.Message`）、`Details`、`Warnings`、`fmt.Errorf` 报错文本按「诊断数据」保留原文，与提交标题、diff 内容同类 |

### 补充：RPC 覆盖守卫（2026-10-25）

`docs/architecture-audit.md` 的「后端专属 0」一直是人工统计的结论，加一个后端方法忘了前端封装，这个数字就悄悄失真。

`frontend/tests/rpcCoverage.test.ts` 把它变成自动断言：扫仓库根目录所有非 `_test.go` 的 Go 文件提取 `func [(...)] GitService) X`，扫 `services/gitService.ts` 提取 `callBackend<T>("X")`，然后双向比对——

- 后端有、前端没有 → 该能力在界面上不可达，测试红并列出方法名
- 前端有、后端没有 → 死封装，调用时必然运行时报错，测试红
- 两边数量不等 → 单独一条断言兜底

守卫本身也验证过：临时加一个 `func (GitService) ProbeUnwrappedRpcForGuardTest()` 后两条测试变红并指名该方法，删除后恢复全绿。同时有一条「扫描器确实扫到了东西」的前置断言（两集合都必须 > 300），避免正则失效导致空集合互相「相等」的假通过。

### 补充：`ClosedLoop` 文案迁移（2026-10-25）

`git_closed_loop.go` 的 6 个结果结构（`LfsConvertResult` / `LfsBulkUnlockResult` / `RangeDiffResult` / `FixupCommitResult` / `ObjectStoreOptimizeResult` / `TagVerifyResult`）各增加 `MessageKey` 与 `Params`，20 处赋值同时填写两者。后端 `Message` 保持原样（日志与测试仍按中文断言），只在旁边加一个可翻译的 key，所以这是一次纯增量改动，不改任何既有行为。

前端 `MaintenanceTab`（对象库优化）、`HistoryView`（fixup）、`BranchesView`（标签验签）改用 `backendText(res.messageKey, res.message, res.params)`；`BranchCompare` 的 range-diff 本来就由前端拼句子，无需改动。`closed.loop.*` 共 21 条 key × 2 语言，`tests/i18n.test.ts` 新增一条扫描 Go 源码的守卫：后端发出的每个 key 必须在两个 catalog 里都存在。
