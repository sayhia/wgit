# wgit 功能梳理与新增功能调研

> 日期：2026-10-23
> 范围：根目录全部 Go 源码（124 个 `.go`，其中 99 个测试文件）+ `frontend/src`（67 个 `.vue`、15 个 `.ts`，约 39.6k 行）
> 方法：静态核对 RPC 定义与调用点、逐文件读实现、竞品功能对照（GitKraken / Fork / Tower / GitButler / SmartGit 2026 现状）

---

## 一、现状梳理

### 1.1 规模基线

| 维度 | 数值 | 备注 |
| --- | --- | --- |
| 后端 RPC | **327** | README 与 `architecture-audit.md` 记 315，已滞后 12 个（分支洞察、ref 对比、批量 stash 等） |
| RPC 分布 | `git_service.go` 165 · `git_studio_engine.go` 44 · `git_lfs_lock.go` 25 · `git_engine_asset.go` 16 · 其余 15 文件 77 | 无状态值接收器，可任意拆分 |
| 后端代码 | 27,045 行（非测试） | `git_studio_engine.go` 单文件 10,379 行 |
| Go 测试 | 99 个测试文件 / 189 个 `Test*` | `t.Skip` 16 处（依赖 git-lfs / gh / POSIX） |
| 前端 | 67 个 `.vue` + 15 个 `.ts`，39.6k 行 | 无任何前端测试 |
| 主视图 | 7 个（工作区 / 历史 / 分支 / 储藏 / 评审 / 工具 / 设置） | `v-if` 切换，非路由 |
| 工具子页 | 13 个（reflog / clean / remotes / worktrees / lfs / submodules / sparse / disk / maintenance / studio / engine / bisect / release） | 单文件最大 `EngineQualitySection.vue` 1,580 行 |
| 命令面板 | 88 条命令 / 15 个分类 | 有收藏、MRU、拖拽排序、分类折叠 |
| 可自定义快捷键 | 9 条 | 另有 12 条硬编码不可改 |

### 1.2 能力矩阵（按定位归类）

- **大型仓库**：blobless/浅克隆、增量分页历史、虚拟滚动、SVG 多车道 DAG（24 车道）、服务端筛选、`ls-files` 模糊快开、`git grep` 全仓搜索、4MB/6 万行 diff 截断保护、稀疏检出 cone、worktree。
- **资产仓库**：LFS 全链路（track / lock / prune / filter / 调优 / 校验 / 审计报表）、引擎识别（Unity / Unreal / Godot / Cocos）、40+ 项引擎与资产质检、DCC 锁巡检、美术一键交付。
- **协作与合规**：变更评审会话（`.git/wgit-review`）、Git Notes 批注、TAPD 关联、CI 流水线、凭据保险库（AES-256-GCM）、密钥扫描、隐私合规、Webhook、离线队列。
- **历史改写**：交互式变基、批量 cherry-pick、squash、reset、revert（含 no-commit）、bisect、reflog 救砖。
- **发布**：发布向导、语义化版本建议、附注/签名标签、Changelog 生成、Bundle/Patch/ZIP 导出。

覆盖率上，**327 个 RPC 全部有 UI 入口**（`architecture-audit.md` 已核验，后端专属 0）。

---

## 二、诊断：三个结构性问题

### 2.1 广度已经饱和，横向扩张边际收益趋零

327 个接口、13 个工具子页、88 条命令。再加第 328 个「XX 审计」，用户感知不到——README 已经 263 行、能力矩阵 13 节，新用户读不完，老用户记不住。

**判断：下一阶段的价值不在「多一个功能」，而在「让已有功能真正可用」。**

### 2.2 定位承诺与实际能力存在硬缺口

项目定位是「游戏与多媒体重度资产仓库」，但资产这一侧最关键的三个日常动作全部缺失：

| 日常动作 | 现状 | 缺口 |
| --- | --- | --- |
| 「这张贴图改了什么？」 | 工作区 diff 显示 `(无内容变更或为二进制文件)` | 无图片对比、无尺寸/体积变化、无差值视图 |
| 「这个 PSD 早该进 LFS，怎么迁？」 | `ScanHistoryLargeBlobs` 能扫出历史大对象，但只能看 | 无 `git lfs migrate import`，历史包袱永远清不掉 |
| 「这个模型面数涨了多少？」 | `LintModelMesh` 只对当前文件做阈值告警 | 无「旧版 vs 新版」对比 |

对比竞品：**Fork 的核心卖点之一就是图片 diff**（side-by-side / overlay / swipe），GitKraken 有 AI 冲突解决，而 wgit 在这个最该有差异化的方向上只有一句「二进制文件不可直接预览」。

### 2.3 36 个 Audit 接口只报不修，是最大的功能空转

实测统计：`Audit*` / `Lint*` / `Inspect*` / `Detect*` 系列共 36 个，其中**只有 5 组有配套写操作**（`CheckUnityMetaSync→AutoFixUnityMeta`、`AuditIgnoreRules→ApplyIgnoreSanitizer`、`AuditSceneStreamMerge→ResolveSceneStreamConflict`、`GetReleasePlan→CreateRelease`、`GetBranchInsights→DeleteMergedBranches`）。

其余 14 项是断头路——用户看到「LOD 梯队不达标」「材质实例覆盖过多」「僵尸锁 7 个」「裸密钥 3 处」，界面上找不到任何一个能修的按钮：

| 只读接口 | 缺的写操作 |
| --- | --- |
| `AuditLocalizationParity` | 生成缺失 key / 回填语言文件 |
| `AuditNativePluginsAbi` | 升级/降级/替换插件 |
| `AuditLodStreaming` | 批量重导 / 改写配置 |
| `AuditTextureChannelPacking` | 通道重排 / POT 转换 |
| `AuditLightmapBakeSettings` | 写回 `.unity` / Lightmass 配置 |
| `AuditMaterialInstanceOverrides` | 批量重置覆盖参数 |
| `AuditPhysicsMeshCollision` | 减面 / 替换碰撞体 |
| `AuditSymbolsAndDefines` | 修复宏冲突 |
| `DetectAssetCyclicDependencies` | 断环建议落地 |
| `AuditCommitContentSafety` | 规则可配置（现为 5 个硬编码词） |
| `ScanStagedSecrets` | 自动 unstage + 写 `.gitignore` + 白名单 |
| `CheckStagedNonLfsLargeFiles` | 一键转 LFS + 重置 index |
| `GetLfsLockActivity` | 批量破僵尸锁 |
| `RunRepoHealthCheck` | 执行 `action` 建议（现只返回字符串） |

### 2.4 9 个伪实现需要处理

「演示型接口」在 README 里已声明是样例数据，但如果 UI 上仍以正常面板呈现，会误导用户。清单：

`GetFullParityStatusSummary`（5 域全部硬编码 100%/120 轮）· `GetPinnacleParityStatusSummary`（6 域 100%/140 轮）· `FetchTAPDWorkItems`（永远 `TotalCount=0`，无 HTTP 调用）· `DiffWorldDataLayers`（Actor 级字段全占位）· `ManageStudioWebhooks`（只写 JSON，从不投递）· `InspectCredentialVault`（`KmsEncryptionEnabled` 恒 true）· `AnalyzePackageSizeBudget`（阈值硬编码 + 固定比例估算）· `GetStudioCIPipelines`（只支持 GitHub Actions，分支回落 `"main"`）· `GetCustomActions`（无配置时返回 6 条 macOS 专用演示动作）。

### 2.5 体验层欠账

- **破坏性操作 77 处原生 `confirm()`**，阻塞式、无法撤销、样式与 design-system 脱节。
- **Toast 只有 28 行**：无堆叠、无分级、无操作按钮，9 个视图各持一个实例，后到覆盖先到。
- **UI 原语只有 Toast**：39 处手写 `role="dialog"`，Button / Input / Select / Tabs 全靠 Tailwind 复制。
- **i18n 完全没有**：无 `vue-i18n`，文案全硬编码中文。
- **字体令牌未落地**：`public/Inter-Medium.ttf` 与授权文件在仓库里，但全项目无 `@font-face`，Inter 从未生效。
- **设置页「承诺 > 实现」**：`settingsIndex.ts` 索引里宣称有「强调色与色板」「界面密度与字号」「动效开关」「SSH allowedSigners」「无效配置清理」，对应组件均无实现——搜得到、点进去是空的。
- **列表交互只覆盖两处**：右键菜单仅历史提交行，多选仅历史与变更列表，分支/标签/储藏/评审/工具页全部纯鼠标。
- **无前端测试**：质量保障只有 `vue-tsc --noEmit`。

---

## 三、竞品对照（2026 现状）

| 能力 | GitKraken | Fork | Tower | GitButler | **wgit** |
| --- | --- | --- | --- | --- | --- |
| 图片 / 二进制 diff | 有 | **强（overlay/swipe）** | 有 | 弱 | **无** |
| 内置终端 | 有 | 有 | 有 | 有 CLI | **无**（仅唤起系统终端） |
| In-app PR/MR 评审 | 强（4 平台） | 跳浏览器 | 有 PR 视图 | 仅 GitHub | **无**（仅跳浏览器） |
| 冲突预防（合并前预演） | 有 Conflict Prevention | 无 | 无 | 有 | **无**（有 `PreviewMerge` 但未做重叠预警） |
| 操作日志 + 撤销 | 部分 | 无 | 无 | **Unlimited undo** | **无** |
| Stacked / 并行分支 | 无 | 无 | 无 | **核心** | **无** |
| `rerere` 冲突复用 | 无 | 无 | 无 | 有 | **无** |
| AI 提交拆分 / 冲突建议 | 有（付费） | 无 | 无 | 有 | **无** |
| CI 状态就地显示 | 有 | 有限 | 无 | 有 | 有（Studio 页，非就地） |
| 大仓性能优化 | 中（Electron） | 强（原生） | 强 | 中 | **强**（增量+虚拟滚动+截断） |

**结论：wgit 在「大型仓库性能」和「游戏资产质检广度」两项上已经领先主流竞品；落后的是「二进制资产可视化」「PR 闭环」「撤销/安全网」「终端」四块。**

---

## 四、新增功能建议

分四档。A 档是差异化护城河，B 档是补齐已有断头路，C 档是对齐竞品，D 档是工程欠账。

### A 档 · 差异化（游戏/资产仓库独有，竞品普遍没有）

#### A1. 二进制资产差异查看器 ★最高优先级

把「二进制文件不可预览」变成可读的视觉差异。

- **图片**（png/jpg/tga/psd/exr 首帧）：side-by-side、叠加（透明度滑杆）、滑动擦除、像素差值热力图；同时显示 尺寸变化 / 体积变化 / 色彩位深变化。
- **音频**（wav/ogg/mp3）：波形对比 + 时长/采样率/声道数变化（`InspectAudioFile` 已有单文件解析，扩展为双版本）。
- **模型**（fbx/obj/glb）：面数 / 顶点数 / 骨骼数 / 包围盒 变化表（`LintModelMesh` 已能解析单版本）。
- 实现路径：Go 侧 `git show <commit>:<path>` 取出旧 blob → 解码 → 返回双版本 base64 + 元数据；前端 canvas 合成。复用现有 `GetImageBase64` 的取 blob 逻辑与 `SourcePanorama` 的缩放组件。
- 入口：工作区 diff 顶部（与「外部对比」并列）、提交检视器的变更文件、评审工作区的文件详情、文件历史时间线。
- 复杂度：**L**（要新增 Go 解码层 + 4 种对比模式的前端组件），但这是整个项目最该有的一个功能。

#### A2. 资产变更影响分析

美术改一张贴图前，先知道它会被谁用到。

- 输入：一个资产路径（或一次提交里的资产变更集）。
- 输出：直接引用（材质/预制体/蓝图）→ 间接引用（场景/关卡）→ 受影响文件数、责任人（`git log` 最近修改者）、是否会跨模块。
- 复用 `InspectAssetReferences`（已能扫引用）扩展成反向索引；结果可按「受影响严重度」排序，并可一键导出 Markdown 交给主美。
- 复杂度：**M**（后端解析已有基础，主要是索引构建与前端树形展示）。

#### A3. LFS 迁移向导（`git lfs migrate import`）

与 `ScanHistoryLargeBlobs` 天然配对，把「扫出来了」变成「清掉了」。

- 流程必须带安全网：扫描 → 选规则（扩展名/阈值/提交范围）→ **dry-run 预览将重写的提交数** → 自动创建 `git bundle` 回滚点（已有 `CreateGitBundle`）→ 执行 `git lfs migrate import --include=... --everything` → 校验 → 提示强推。
- 必须明确警告：重写历史后所有人要重新 clone；`--everything` 与 `--above=` 的区别要在 UI 上写清楚。
- 同时补 `git lfs migrate export`（反向迁出，合规/归档场景）。
- 复杂度：**M**（命令简单，风险控制与文案是重点）。

#### A4. LFS 锁协同看板

现在有 `GetLfsLocks`、`GetLfsLockActivity`、`CheckRemoteLocksConflict`，但没有一个「我关心的文件被谁锁了」的视图。

- 看板：按「我锁的 / 别人锁的 / 僵尸锁（>N 小时无活动）/ 冲突」四栏；每条显示锁持有人、锁定时间、最后活动、文件体积、所属资产类型。
- 关键动作：**批量破僵尸锁**（补 `GetLfsLockActivity` 缺的写操作）、「我要改的文件被锁了」→ 一键生成请求消息（复制到 IM）。
- 可挂在工具箱 LFS 页顶部，也可做成工作区侧边抽屉。
- 复杂度：**M**。

#### A5. 引擎工程一键 Bootstrap

新建/接手一个 Unity 或 Unreal 仓库时，把该配的东西一次配好：`.gitattributes`（引擎预设）、`.gitignore`（引擎模板）、LFS 跟踪规则、`UnityYAMLMerge`、`core.autocrlf=false`、`core.longpaths`（Windows）。

- 现有 `GetGameEngineGitAttributesPreset` / `GenerateGitignoreTemplate` / `ApplyIgnoreSanitizer` / `ConfigureUnityYamlMerge` 已具备全部零件，缺的是「一个向导串起来 + 体检确认」。
- 产出：一份「工程就绪度」评分 + 逐项修复按钮。
- 复杂度：**S**（纯编排，几乎不用新写底层）。

---

### B 档 · 闭环（把断头路接上，投入小、感知强）

#### B1. 统一 `Audit → Fix` 框架 ★

与其一个个补，不如定一个协议：

- `Audit*` 返回值统一加 `fixable: bool` 与 `fixId`；新增 `ApplyAuditFix(repoPath, fixId, items []string) (*FixResult, error)`。
- `FixResult` 统一含 `applied / skipped / failed / backupPath / gitDiffSummary`，保证每次修复都可回滚（写文件前备份到 `.git/wgit-fix-backup/<ts>/`）。
- 前端做一个通用 `<AuditResultPanel>`：左列问题、右列「修复选中」，底部统一展示 diff 摘要。
- 复杂度：**M**（框架一次投入，之后每个审计接一个 adapter）。

#### B2. 密钥扫描 → 一键处置

`ScanStagedSecrets` 现在只报。补：自动 `git restore --staged`、按模式追加 `.gitignore`、生成 `.wgit-secret-allowlist`（带过期时间）、并把命中行号定位到编辑器。同时把 5 个硬编码关键词换成可配置规则文件（`~/.wgit/secret-rules.toml`）。

- 复杂度：**S**。

#### B3. 暂存区大文件 → 一键转 LFS

`CheckStagedNonLfsLargeFiles` → 「全部转 LFS」：`lfs track` 写 `.gitattributes` → `git rm --cached` → `git add` → 重新校验。要保证 index 重写不丢暂存区其它内容。

- 复杂度：**S**。

#### B4. 仓库健康分 → 真修复

`RunRepoHealthCheck` 现在返回 `action: "settings" | "fsck"`，但没有任何按钮执行它。补：点击直接跳转对应设置项并高亮，或直接跑 `RunMaintenance` / `git fsck --lost-found`，把结果落成报告文件。

- 复杂度：**S**。

#### B5. 冲突预防（合并前预演）

已有 `PreviewMerge`，但没有「重叠预警」。补：

- 合并/变基前，用 `git merge-tree --write-tree <A> <B>`（git 2.38+，**无副作用**，不碰工作区与 index）预测冲突文件清单。
- 叠加「队友重叠」：查目标分支最近 N 天改动过相同文件的人，提示「你和 @张三 都在改 `X.cs`」。
- UI：点 Merge 前先弹「预演结果」——0 冲突 / N 个冲突文件（列出）/ 与你重叠的 3 个人。
- 这是 GitKraken 收费版的核心卖点，wgit 有全部底层能力却没串起来。复杂度：**S**。

---

### C 档 · 对齐竞品

| 编号 | 功能 | 说明 | 复杂度 |
| --- | --- | --- | --- |
| C1 | **内置终端** | xterm.js + `creack/pty`（纯 Go，三平台可用）。价值：把「复制 hash → 切终端 → 敲命令」的往返消掉；也是 wgit 作为「工作台」而非「查看器」的分水岭 | M |
| C2 | **PR/MR 就地评审** | GitHub REST + GraphQL（GitLab 同构）。列 PR、看 diff、发评论、合并。现有 `GetRemoteWebInfo` 已能解析仓库归属，`ChangeReview` 的会话/行级评论数据模型可直接复用为 PR 评论的本地镜像 | L |
| C3 | **操作日志 + 撤销** | 每次写操作前记录 `HEAD` / 目标 ref / 关键文件的 ref 快照到 `.git/wgit-oplog.json`，支持「撤销上一步」。参考 GitButler 的 operations log。对重写历史类操作（rebase / reset / migrate）尤其有价值，是**安全网级功能** | M |
| C4 | **fixup + autosquash** | `git commit --fixup=<hash>` + `git rebase -i --autosquash`。交互式变基页加「标记为 fixup」按钮，提交列表显示 fixup 徽标并在变基时自动收起。日常工作流最高频的整理动作 | S |
| C5 | **range-diff** | `git range-diff A1..A2 B1..B2`，force-push 后对比两组提交「哪些变了、哪些是新增」。分支对比页加第三个 tab。评审者刚需 | S |
| C6 | **后台维护** | `git maintenance start`（定时 gc/commit-graph/prefetch）+ 手动 `git commit-graph write --reachable` / `git multi-pack-index write` / `git repack -adf`。大仓 100k commits 下 `git log` 能快数倍，且现有 `GetMaintenanceStats` 只有「看」没有「做」 | S |
| C7 | **reveal 类补齐** | `git worktree move/repair/prune`、`submodule deinit/foreach`、`bisect run/skip/log`、`verify-tag`（现在只验 commit 不验 tag）、hook 新建/编辑（现在只能启停） | S |

---

### D 档 · 工程与体验欠账

| 编号 | 事项 | 具体做法 |
| --- | --- | --- |
| D1 | **统一 ConfirmDialog + 撤销** | 77 处原生 `confirm()` 全量替换为 `<ConfirmDialog>`（支持「危险操作需输入仓库名」三档强度）；删除类操作改为「先执行 → Toast 带 5s 撤销」，撤销走后端 `undoLast` |
| D2 | **UI 原语库** | 把 `design-system/index.html` 里已有的 13 类组件示例落成真组件：Button / Input / Select / Tabs / Table / Tooltip / Dropdown / Popover / Checkbox / Dialog / Toast(堆叠+分级+action) / Badge / Skeleton |
| D3 | **字体令牌落地** | `style.css` 加 `@font-face` 引 `Inter-Medium.ttf`（文件已在仓库，白放了大半年）；把 `design-system/tokens.css` 的 181 个令牌真正接进 Tailwind v4 `@theme` |
| D4 | **设置页对齐** | `settingsIndex.ts` 索引与实现二选一：要么删掉「强调色/密度/字号/动效/allowedSigners」的虚假条目，要么补实现（建议补「强调色 + 字号 + 密度」三项，成本低、感知强） |
| D5 | **前端测试** | 引入 vitest + @vue/test-utils，先覆盖 5 条关键路径：callBackend 错误上抛、暂存/丢弃状态机、快捷键冲突检测、命令面板排序、评审会话状态流转 |
| D6 | **视图状态持久化** | 当前主视图 tab、滚动位置、筛选条件、历史/评审分栏宽度（工作区已做，其余未做） |
| D7 | **i18n 骨架** | 引入 `vue-i18n`，先只抽 `components/ui` 与 7 个主视图的标题层，文案 key 用 `域.组件.语义`；不追求一次抽完 |
| D8 | **右键菜单 + 列表键盘导航** | 把历史提交行的右键菜单抽成复用 hook，铺到 分支 / 标签 / 储藏 / 变更 / 源码树；列表加 `role="listbox"` + roving tabindex |
| D9 | **架构文档同步** | `architecture-audit.md` 的 315 更新为 327，否则「后端专属 0」的结论不再自证 |

---

### E 档 · 考虑但暂不建议

| 方向 | 为什么不建议现在做 |
| --- | --- |
| 继续加 `Audit*` 审计 | 已 36 个，边际价值≈0；先把 B1 闭环做了 |
| Stacked / 并行分支（GitButler 式） | 需要维护 virtual branch 元数据与 integration 分支，是**另一套版本控制模型**，与现有 327 个 RPC 的假设冲突，改造面过大 |
| AI 自动改代码 / AI 拆提交 | 项目定位本地化 + 高可信，AI 误操作在资产仓库代价高；若做，建议只做「提交信息建议」且默认关闭、用户自带 key |
| 云同步 / 团队 SaaS | 需要服务端，偏离纯本地工具定位；现有 Git Notes + 评审会话文件已能支撑轻协作 |
| 移动端 / Web 版 | 与「大仓本地性能」定位矛盾 |

---

## 五、建议的迭代路径

### 迭代 1（把断头路接上，建立信任）
`B1 框架` + `B2 密钥处置` + `B3 大文件转 LFS` + `B4 健康分修复` + `B5 冲突预防` + `D1 ConfirmDialog`

理由：全部是 S 档、复用现有 RPC、用户当天就能感知「以前只能看，现在能修」。B1 一次投入后，后续每个审计接 adapter 即可。

### 迭代 2（打出差异化）
`A1 二进制资产差异查看器` + `A3 LFS 迁移向导` + `C6 后台维护` + `C4 fixup/autosquash`

理由：A1 是唯一能拉开与 Fork/GitKraken 差距的功能；A3 解决资产仓库最痛的历史包袱；C6 让大仓性能优势兑现成数字。

### 迭代 3（补齐竞品必备）
`C1 内置终端` + `C3 操作日志与撤销` + `A2 影响分析` + `A4 锁看板`

### 迭代 4（工程偿还）
`D2 UI 原语库` + `D5 前端测试` + `D3 字体令牌` + `D7 i18n` + `C2 PR 评审`

理由：C2 是唯一需要外部 API + 认证体系的大工程，应在 UI 原语和测试体系就位后再动。

---

## 六、可立刻开工的 10 件事（单点、低风险）

1. `style.css` 加 `@font-face` 引 Inter —— 一行 CSS，白放的字体终于生效。
2. `settingsIndex.ts` 删掉 5 个虚假索引项（或补实现），消除「搜得到点不到」。
3. `CheckStagedNonLfsLargeFiles` 加「全部转 LFS」按钮 —— 复用 `TrackLfsPattern`，闭环一步到位。
4. `GetLfsLockActivity` 加「批量为僵尸锁破锁」—— 复用 `UnlockFile(force)` 循环。
5. `RunRepoHealthCheck` 的 `action` 接上真按钮（跳设置 / 跑 maintenance）。
6. `PreviewMerge` 前置 `git merge-tree --write-tree` 预演，冲突文件先列出来。
7. 交互式变基页加 `commit --fixup` 标记 + `--autosquash` 勾选。
8. 分支对比页加 `range-diff` 第三 tab。
9. 维护页加 `git commit-graph write --reachable` + `multi-pack-index write` + `repack -adf` 三键。
10. `architecture-audit.md` 的 RPC 计数 315 → 327。

---

## 七、一句话结论

**wgit 的「广度」已经超过绝大多数商业 Git 客户端，「深度」却停在「能看见」而不是「能解决」。下一阶段应该做三件事：把 36 个审计的一半变成可执行的修复、把二进制资产差异做成竞品没有的护城河、把 77 处原生 confirm 和缺失的撤销栈补成安全网。**
