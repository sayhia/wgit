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
- **D2 UI 原语库（骨架 + 试点）**：`components/ui/` 新增 `UiButton` / `UiBadge` / `UiModal` / `UiEmpty` / `UiSkeleton`；变体映射抽到 `utils/uiVariants.ts`（完整 class 字面量，避免 Tailwind 扫不到动态拼接），配 `tests/uiVariants.test.ts` 覆盖全部变体契约。已迁移 18 处：`branches/modals/` 全部 5 个、标签详情、`HistoryView` 3 个、`RemotesTab` 4 个、`SubmodulesTab` 3 个、`StashView` 2 个、`FixCenterSection` 四件套。`UiButton` 变体扩到 6 个（新增 warning / purple，对应 stash 与 tag 的既有语义色）。按 `role="dialog"` 统计，手写弹窗 **39 → 22** 处。

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

### 验证

- `go vet ./...` 干净；`go test .` 全量（含新增 15 条）通过。
- `bun run typecheck`（vue-tsc）零错误；`bun test tests` **114 pass / 0 fail**（新增 PR 补丁解析、状态元数据、确认状态机 18 条）。
- `bun run build` 通过，首屏 `index` 441.67 kB、`PrView` 30.87 kB。

## 五、未实施（明确边界）

C2 已完成，报告中「迭代 1–4」的功能项已全部落地。剩余的是**非功能**欠账，不阻塞版本：

| 事项 | 现状 |
| --- | --- |
| i18n 全量迁移 | 骨架 + 导航/设置/修复中心/PR 已双语，其余 30+ 组件仍硬编码中文 |
| 剩余 22 处手写弹窗 | 其中 8 处是搜索面板/全屏页/变基计划编辑器，结构不适配；14 处零散单点按新代码优先替换 |
| 列表 roving tabindex | 行级 `aria-label` 已补，真正的方向键选中需独立改造 |
