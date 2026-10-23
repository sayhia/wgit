# wgit — 面向大型项目的现代化 Git 桌面客户端

基于 **Wails3（Go 后端）+ Vue 3 + TypeScript + Tailwind CSS v4** 构建的跨平台 Git 桌面工作台，参考腾讯 UGit 的工程化能力，重点解决 **大型单体仓库（Monorepo）/ 游戏与多媒体重度资产仓库 / 多分支多子仓项目** 的日常痛点：海量历史、几百个分支、超大差异、大文件与稀疏检出、快速克隆与代码导航。

## 快速开始

```bash
# 1. 构建前端产物（dist 会被嵌入 Go 二进制）
cd frontend && bun install && bun run build

# 2. 构建桌面端二进制
cd .. && go build -o bin/wgit .

# 3. 启动（或直接双击 bin/wgit.app）
./bin/wgit
```

开发模式（前端热更新）：

```bash
wails3 dev            # 桌面壳 + Vite 热更新
# 或仅前端：cd frontend && bun run dev
```

## 大型项目能力矩阵

### 1. 仓库接入与克隆加速
- **Blobless 部分克隆**（`--filter=blob:none`）：首次克隆仅下载目录树与按需对象，超大仓库体积可降至 10%–30%。
- **浅克隆**（`--depth=N`）、指定分支、递归子模块；完整的克隆进度提示。
- **欢迎页 / 最近仓库 / 初始化新仓库**：`bin/wgit` 首次启动即可打开、克隆或 `git init`；支持输入目录扫描本地 Git 工程并展示分支 / 引擎 / 脏状态。

### 2. 提交历史（海量历史可交互）
- 提交总数 + 已加载数量显示；**300 条起步、按需增长**的增量加载，服务端保持 DAG 车道连续。
- **服务端筛选**：分支范围（全部引用 / 指定分支）、作者、关键词、日期区间，300ms 防抖；筛选结果 skip/limit 分页加载。
- **虚拟滚动**：固定行高窗口化渲染，数千条提交依旧流畅；`j/k`、↑↓ 键盘导航自动滚动到选中行。
- **SVG 多车道 DAG 图谱**：合并曲线、分支指针、标签、签名状态一览。
- 提交检查器保留全部操作：Cherry-Pick、Revert、建分支/标签、Reset（soft/mixed/hard）、复制 Hash、变更文件清单。
- 历史页头显示合并提交数与合并率；多仓库总览显示当前仓库的提交 / 分支 / 标签 / 贡献者概览。
- **内容考古**：`git log -S/-G` 检索字符串或正则的新增 / 移除 / 改写，支持作者、路径、大小写过滤，并展示命中片段与涉及文件。
- **签名校验与批注总览**：提交检视器展示 `%G?` 状态、签名者 / 密钥 / 指纹；批注总览列出仓库全部 Git Notes 并支持跳转提交。

### 3. 分支治理（几百个分支也不乱）
- **分支洞察面板**：自动分类 `current / active / merged / unpushed / stale`，统计已合并可清理、未推送、长期停滞数量。
- **排序**：当前分支置顶，支持最近活动 / 名称 / Ahead 数排序；Ahead/Behind 与上游跟踪分支徽章。
- **一键清理已合并分支**（`git branch -d`）、批量删除、设置上游；标签/储藏/分支比对标签页完整保留。
- **储藏箱文件级操作**：展开储藏文件清单（状态 / 增删行），可只恢复单个文件到工作区并暂存；工作区多选后也可只储藏所选文件，其余改动保留。
- **标签发布、清理与详情**：填写附注即创建 annotated tag，可选 GPG 签名；标签列表支持单标签推送、删除远程、多选批量删除本地标签，并可查看标签类型、目标提交、打标者与附注。
- 大列表分页渲染（默认 400 条 + 显示更多），246+ 远程分支搜索无卡顿。

### 4. 快速导航与检索
- **⌘P / Ctrl+P 快速打开文件**：服务端 `git ls-files` + 模糊打分（多关键词 AND、子序列匹配、名称/目录加权），100k 文件级 Monorepo 无需下发全量文件列表。
- **⌘⇧F / Ctrl+Shift+F 全仓代码搜索**：`git grep` 服务端检索，大小写开关、结果行内预览，回车直接打开到指定行。
- 打开文件自动切换到工作区并滚动/高亮目标行；源码全景支持在文件管理器中定位、用系统默认应用打开或复制图片 Base64；源码面板可复制全部已跟踪文件路径清单。

### 5. 工作区大仓库保护
- **逐块 / 逐行暂存与丢弃**：hunk 头部可「暂存此块 / 丢弃此块」；Unified 模式还可勾选单个增删行，跨 hunk 选择后精确暂存或丢弃，其余改动留在工作区。
- **词级差异**：在 Unified / Split 之外提供「词级」模式，按 `[-删除-] / {+新增+}` 行内着色，适合评审文案或小范围代码改动。
- **文件历史时间线**：`git log --follow` 追溯文件每次修订并显示文件体积，可查看该修订的版本内容 / 文件差异、复制历史版本完整内容，或一键恢复到任一历史版本（自动暂存）。
- **超大差异折叠**：单文件差异超过 4000 行自动折叠渲染，显示行数/增删统计，可手动展开全部；后端 RPC 层同时对 diff 负载做 4MB / 6 万行截断保护。
- **超大列表渲染上限**：源码树 / 平铺列表 1500 行、变更分组 300 行上限 + “显示更多”，搜索始终基于全量数据。
- 文件大小并行采集、非 UTF-8 安全路径解析（`git ls-files -z` / `status -z`，中文、空格、特殊字符路径不再丢失）。
- Blame 超大文件 3 万行上限。

### 6. Monorepo / 多仓工程
- **子模块**：状态徽章（已同步/变更/未初始化/冲突）、递归同步（专用 RPC）、单仓远端更新、URL 同步（`git submodule sync`）、添加/移除/切换跟踪分支。
- **Git Subtree**：以普通目录嵌入外部仓库，支持添加、列表、拉取上游更新与推送回上游（元数据持久化在 `.git/wgit-subtrees.json`）。
- **稀疏检出（Cone 模式）**：目录白名单可视化编辑、常用预设、一键应用/恢复全量检出，显著降低磁盘占用。
- **多工作树 Worktree**：并行检出多个分支，添加/移除/锁定状态。
- **Git LFS**：跟踪规则、独占锁、批量锁定、缓存清理（prune）、本地缓存体积 / 对象统计、`fetchinclude` / `fetchexclude` 选择性下载过滤、并发 / 带宽调优预设、单文件版本历史恢复、锁活动巡检、按过滤规则选择性拉取、可锁资产只读保护、团队锁冲突检测、锁审计报表与推送前 LFS 完整性校验，并可直接编辑 `.gitattributes`。
- **多远程同步**：逐个远程 Fetch（--prune）/ Prune 失效引用、一键全部 Fetch、Ahead / Behind 状态、延迟测速、浏览器打开远程网页；支持重命名远程与修改 fetch URL。

### 7. 仓库健康与维护
- 五维体积透视（总占用 / .git / LFS / 工作区 / 构建缓存）、工作区扩展名体积分布 Top 10、历史大对象扫描、包体预算与平台占用分析、大文件扫描与一键 LFS 跟踪。
- `git gc` / LFS prune / `git fsck --full` 体检、综合仓库体检评分、Reflog 救砖恢复、工作区清理（含空目录扫描与 .gitkeep 生成）、崩溃栈反解、热更清单生成、维护统计。
- **Git 钩子管理**：维护页展示 8 个常用客户端钩子的安装 / 启用状态与脚本路径，支持一键禁用（重命名为 `.disabled`）与重新启用。
- **暂存区大文件检查**：自定义阈值，列出已暂存但未走 Git LFS 的超限文件，避免误提交；未跟踪文件扫描支持逐个删除或一键全清。
- **储藏与丢弃**：支持仅储藏已暂存改动（未暂存内容保留）；工作区可一键丢弃全部未提交改动与未跟踪文件（强确认）。
- **忽略规则体检与一键修复**：识别引擎类型与健康度，列出缺失规则和误跟踪构建产物；支持路径忽略来源检查、自定义规则、Go / Node / Unity / Unreal / Python 模板与一键补全。
- **Studio 协作与合规**：团队活跃度 / 贡献者 / 文件热点、推送前策略检查、最近提交内容安全审计、隐私清单与权限合规、Git 认证诊断、当前会话 Git 操作日志（可清空），以及仓库级自定义操作列表 / JSON 编辑 / 执行。
- **Studio 集成与运维**：TAPD 工作项同步 / 提交关联（写入 Git Notes）/ 提交信息单号解析、CI 流水线阶段查看与触发、后端能力清单；凭据暴露扫描与 AES-256-GCM 本地保险库、Webhook 登记（URL 脱敏）、DDC 缓存检测 / 修剪、离线操作队列（入队 / 暂停 / 重放）与多工程工作树拓扑。
- **归档、补丁与 Bundle**：按提交 / 引用导出 ZIP 或 `.patch`、创建与校验离线 Bundle，可把补丁文件或粘贴的补丁内容应用到工作区。
- **提交模板**：提交编辑器支持载入 `commit.template` / `.gitmessage`、一键载入上次提交信息，也可把当前标题与正文一键保存为仓库模板；输入时实时显示 Conventional Commits 规范评分 / 等级与问题提示；支持「快速提交」（暂存全部 → 提交 → pull --rebase → push）。
- **引擎资产洞察**：自动识别 Unity / Unreal / Godot / Cocos 工程，检查 `.gitattributes` 与 LFS 规则，提供资产元数据 / 引用扫描、Unity `.meta` 配对审计与一键修复、UnityYAMLMerge 检测 / 配置、引擎预设应用与纹理规范审计，以及模型面数 / 骨骼动画 / 音频规范 / 本地化对齐 / 原生插件 ABI / 缺失材质 / LOD 串流 / 物理碰撞 / 材质实例覆盖 / DCC 临时锁 / Shader 变体 / 宏定义 / 数据表语义差异 / 跨分支资产拣选 / Unreal 资产节点差异 / 场景流式合并审计 / 场景 GUID 修复等资产与工程质检工具；支持一键启动引擎编辑器、按引擎类型清理可重建缓存，DCC 巡检卡可直接对指定资产发起 Git LFS 自动上锁；「美术一键交付」自动暂存、生成 `[Art]` 提交并推送，并展示贴图 / 网格数量与步骤日志；剩余演示型接口收在可折叠「内置演示接口」区，明确标注后端样例数据、仅用于预览字段结构。
- Git 钩子、提交模板、签名配置、代理配置等工程化设置；高级 Git 配置页可检索 local / global 条目并写入任意配置键。

### 8. 提交工作流（历史改写与回归定位）
- **多选提交**：复选 / Ctrl·Cmd 点选 / Shift 区间选择，选中数量与清空操作常驻工具栏，可批量 Cherry-Pick 或复制完整提交详情；历史页支持 `A..B` 范围拣选；工具箱维护页支持压缩最近 N 条提交。
- **提交检视操作**：Revert（生成反向提交）、Revert 不提交（回滚到暂存区）、分离 HEAD 检出、批量 Cherry-Pick、交互式变基。
- **批量 Cherry-Pick**：按时间顺序（旧→新）把选中的多个提交批量移植到当前分支，冲突时明确报告失败提交并进入常规冲突解决流程。
- **交互式变基 (Interactive Rebase)**：选择 base（上游分支 / 本地分支 / 所选提交父提交），加载 `git rebase -i` 计划，逐条编辑 `pick / reword / edit / squash / fixup / drop` 后执行；中断冲突后可沿用现有跳/继续/终止能力。
- **Git Bisect 二分定位**：工具盒子页「二分定位」——输入 bad / good 版本启动二分，Great/Good 一键标记、实时显示当前提交/已判定次数/good 候选、定位到首个坏提交后复制 hash 或重置；附带 bisect log。
- **命令面板（⌘K）统一入口**：克隆/初始化/欢迎页、全部大型项目工具、交互式变基、文件与代码搜索均可在命令面板直接跳转执行。

### 9. 多仓库工作区（Multi-Repo）
- **仓库标签栏**：同时打开多个仓库（大型项目常见：客户端 / 服务端 / 引擎 / 工具链），每个标签显示脏状态、↑↓ 与快照快捷键（⌘/Ctrl+1..9 切换），支持关闭、水平滚动。
- **打开仓库 / 克隆 / 初始化** 入口收敛到标签栏右侧；标签页持久化到本地，重启后自动恢复并校验路径有效性。
- **多仓库总览（⌘⇧O）**：汇总所有已打开仓库的分支、领先/落后、暂存/未暂存/未跟踪/冲突变更；支持单仓 Fetch、全部 Fetch、30s 自动刷新、并发上限 4 的状态加载、搜索过滤、点击切换/关闭仓库。

### 10. 变更评审与发布
- **变更评审工作区（ActivityDock「变更评审」）**：基于 `base...head` 的评审会话——历史会话列表、逐文件结论（通过 / 需修改 / 待评审）、评审备注、行级评论（点击 diff 行定位行号、解决/重新打开）、提交范围、进度与 verdict、导出 Markdown、清除会话；会话数据保存在 `.git/wgit-review`，不污染工作区与历史。
- **发布向导（工具箱「发布向导」）**：发布前检查（待发布提交 / 工作区清洁度 / 发布分支 / 远端同步 / 提交签名，含阻塞项）、语义化版本建议、附注标签创建（可选推送与 GPG/SSH 签名）、Studio Changelog 自动生成（分类统计 + Markdown 预览/复制）。
- **提交图谱升级**：动态车道宽度（最多 24 车道，超出折叠提示「…+N 车道」）+ 图谱/简洁模式切换（持久化），合并曲线 / HEAD 环 / 合并提交环；提交检查器新增「变更补丁」预览（± 高亮、超大补丁截断提示、4000 行渲染上限 + 显示全部、自动换行）。

### 11. 外部比对/合并工具与托管平台联动
- **外部差异与合并工具自动探测**：自动识别本地已安装的 Cursor、VS Code / Insiders、Beyond Compare、IntelliJ IDEA、FileMerge (opendiff)、Sublime Merge 等；支持自定义命令模板与 Git `diff.tool` / `merge.tool` 配置持久化。
- **工作区一键调用**：单文件差异顶部「外部对比」唤醒双栏比对；冲突文件顶部「外部合并工具」自动拉起三方裁决。
- **代码托管平台全链路联动**：基于 `origin` 智能识别 GitHub / GitLab / Gitee / Gitea / Bitbucket；提交历史支持「在网页端查看提交」、源码全景支持「网页查看该文件」、分支对比支持「发起 Web PR / MR」与差异详情（文件级增删统计 + 补丁预览）；GitHub / GitLab 仓库还可直接在主视图「PR / MR 评审」里列 PR、看补丁、发评论、批准与合并（见 §15）。

### 12. 多工作树协同与 Git Notes 批注系统
- **Git Worktree 并行开发工作台**：工具箱专属子页——全面掌握主工作区与各关联工作树分支、HEAD Commit、锁定与路径状态；支持一键新建（可关联或新创分支并自动在新标签页打开）、一键打开为 wgit 标签页、一键唤起系统终端与访达、工作树锁定防误删及安全修剪。
- **Git Notes 提交无侵入批注**：不改写提交 Hash 即可给任何 Commit 追加 QA 验收、回归排查或代码审查日志；提交历史图谱中直观呈现金色 Note 徽标，检视面板支持 Markdown 批注编辑、删除与一键推送至远端 (`refs/notes/commits`)。

## 13. 核心 Git 高级能力（全量接入 UI）

后端 `GitService` 共提供 **363 个 RPC 方法**，均已提供 UI 入口（100%）；2026-10-23 新增 8 个 PR/MR 评审接口（`GetPullRequestProvider` / `SavePullRequestToken` / `ClearPullRequestToken` / `ListPullRequests` / `GetPullRequestDetail` / `CreatePullRequestComment` / `SubmitPullRequestReview` / `MergePullRequest`）；2026-10-22 已删除 12 个与现有 UI 能力重叠的重复底层命令（MergeBranch / RebaseBranch / AbortMerge / GetCommitsPaged / GetFileHistory / GetMergeToolConfig / SearchSourceCode / ShelveSelectedFiles 等），合并、变基与冲突测试已改由 `MergeAdvanced` / `RebaseAdvanced` / `AbortConflictOperation` 覆盖。

其中 171 项为此前仅后端、现已接入 UI 的能力：Git 版本探测（设置 → 架构信息）、提交签名状态回读（设置 → 签名）、LFS 拉取 / 推送（工具箱）、暂存区敏感信息扫描（工具箱 → 工作区清理）、行级评论删除（评审工作区）、提交报告导出（提交历史）、逐块暂存 / 丢弃（工作区差异）、储藏文件清单与单文件恢复（储藏箱）、Git Hooks 状态检查（维护与体检）、按文件储藏所选改动（工作区批量操作）、LFS 缓存统计与选择性下载过滤（工具箱）、暂存区非 LFS 大文件检查（工作区清理）、忽略规则体检 / 路径检查 / 自定义规则 / 模板追加 / 一键修复（工作区清理）、词级差异（工作区）、附注 / 签名标签创建（分支与标签）、单标签推送（分支与标签）、远程重命名与 URL 修改（工具箱）、文件历史时间线与版本恢复（工作区）、内容考古（提交历史）、多远程 Ahead/Behind 与延迟测速（工具箱）、标签详情（分支与标签）、提交签名校验（提交历史）、Git Notes 批注总览（提交历史）、`.gitattributes` 编辑（工具箱 → Git LFS）、团队活跃度 / 推送前检查 / 提交内容安全 / 操作日志（工具箱 → Studio 协作）、ZIP / Patch / Bundle 导出、校验与应用（工具箱 → 维护与体检）、引擎检测 / 资产元数据与引用 / Unity meta 审计与修复 / UnityYAMLMerge / 引擎预设 / 纹理审计（工具箱 → 引擎资产）、Git Subtree 添加 / 拉取 / 推送（工具箱 → 子模块）、子模块 URL 同步（工具箱 → 子模块）、自定义操作列表 / 编辑 / 执行（工具箱 → Studio 协作）、文件定位 / 默认应用打开（源码全景）、提交模板载入 / 保存（工作区）、补丁内容检查 / 应用（工具箱 → 维护与体检）、提交拓扑与仓库概览（历史 / 多仓库总览）、逐行暂存 / 丢弃（工作区差异）、本地工程扫描（欢迎页）、远程网页打开（工具箱 → 远程仓库）、隐私合规与 Git 认证诊断（工具箱 → Studio 协作）、标签批量 / 远程删除（分支与标签）、Hook 启停（维护与体检）、文件路径清单复制（源码面板）、LFS 调优预设 / 文件版本历史 / 锁活动巡检 / 选择性拉取 / 可锁资产只读保护 / 批量扩展名跟踪 / 单文件锁状态 / 团队锁冲突检测 / 锁审计报表 / 推送前 LFS 校验（工具箱 → Git LFS）、TAPD 工作项同步 / 提交关联 / 单号解析、CI 流水线查看与触发、后端能力清单（工具箱 → Studio 协作 → Studio 集成）、凭据保险库扫描与加密存储、Webhook 登记、DDC 缓存检测与修剪、离线提交队列、多工程拓扑（工具箱 → Studio 协作 → Studio 运维）、工作区变更统计条（工作区）、上次提交信息载入（提交编辑器）、扩展名体积分布与历史大对象扫描（体积与大文件）、分支差异详情与文件历史版本差异（分支与标签 / 源码全景）、空目录扫描与 .gitkeep 生成（工作区清理）、综合仓库体检评分（维护与体检）、高级 Git 配置查看 / 写入（设置）、模型面数 / 骨骼动画 / 音频规范 / 本地化对齐 / 原生插件 ABI / 缺失材质质检 / LOD 串流 / 物理碰撞 / 材质实例覆盖 / DCC 临时锁（工具箱 → 引擎资产）、包体预算与平台占用分析（体积与大文件）、图片 Base64 复制（源码全景）、最近提交压缩（维护与体检）、Revert 不提交 / 分离检出 / 多选复制详情（提交历史）、分支重命名同步远端（分支与标签）、代理清除（设置）、仅储藏暂存区（储藏箱）、全部丢弃（工作区）、DCC 资产自动上锁 / Shader 变体 / 宏定义审计（引擎资产）、提交规范实时代码检查（提交编辑器）、崩溃栈反解 / 热更清单（维护与体检）、数据表语义差异 / 启动引擎编辑器 / 引擎缓存清理（引擎资产）、范围 Cherry-Pick（提交历史）、逐文件删除未跟踪项（工作区清理）、子模块递归更新（子模块）、跨分支资产拣选 / Unreal 资产节点差异 / Unreal 工程详情 / 场景流式合并审计 / 场景 GUID 修复（引擎资产）、快速提交（提交编辑器）、历史版本内容复制（文件历史）、10 项引擎演示诊断接口（引擎资产 → 内置演示接口，明确标注样例数据）、美术一键交付（引擎资产）。

## 14. 诊断闭环（把「只能看」接上「能修」）

- **暂存区大文件一键转 LFS**（`TrackStagedLargeFilesAsLfs`）：按扩展名写入 `.gitattributes`（PSD/FBX/uasset 等自动 `--lockable`），从 index 摘除后重新 add 让 clean filter 重写为指针，并逐项回读暂存对象大小验证；失败项给出原因，工作区文件不受影响。
- **僵尸 LFS 锁批量破除**（`UnlockStaleLfsLocks`）：从锁活动巡检中挑出长期无活动的锁，一次 `lfs unlock --force` 全量释放，逐条返回持有人 / 锁定时长 / 结果，避免美术互相堵资产。
- **range-diff 补丁对照**（`GetRangeDiff`）：`git range-diff` 按补丁内容而非 hash 匹配，force-push 后能区分「等价 / 已改写 / 新增 / 已移除」；分支对比页内置折叠面板，默认 `base..target` 对 `base..HEAD`。
- **fixup / squash 提交整理**（`CreateFixupCommit`）：提交检视器一键生成 `fixup!` 提交，配合两个变基弹窗已有的 `--autosquash` 默认勾选，改动自动折叠回目标提交。
- **对象库优化**（`OptimizeObjectStore`）：commit-graph（跳过提交遍历）、multi-pack-index（跳过包扫描）、repack、pack-refs 四键可选，返回每项耗时与 `.git` 体积变化，让十万级提交的大仓 `git log` / `blame` 明显变快。
- **合并前队友重叠预警**（`GetMergeOverlapWarning`）：与 `PreviewMerge` 的 `merge-tree --write-tree` 无副作用预演并行执行——前者答「会不会冲突」，后者答「谁在跟我改同一批文件」，合并弹窗同时展示，冲突从「事后解决」变成「事前沟通」。
- **健康体检项直接可修**（`RunHealthFix`）：身份 / 签名 / 冲突 / 大文件这类需要人决策的维度返回 `NeedsInput` 并跳转到对应面板；对象库与忽略规则则真实执行 `git fsck --full` + `gc --prune=now`、补全规则并取消跟踪误提交产物，修复后自动复检。
- **品牌强调色**：6 套预设（远山蓝 / 紫藤 / 青瓷 / 苔绿 / 落日 / 胭脂），通过 `[data-accent]` 覆盖 accent 色族，深/浅主题各有独立取值，切换主题无需 JS 重算；成功 / 警告 / 危险语义色固定不随强调色漂移。
- **二进制资产差异查看器**（`GetAssetDiff`）：把 "Binary files differ" 变成四种可视化对比——并排、叠加（透明度滑杆）、滑动擦除、逐像素差值热力图（洋红标出改动区，阈值 ΔRGB > 24）；同时给出尺寸 / 体积 / 格式变化。支持 PNG / JPG / GIF / BMP / WebP 内联渲染，TGA / PSD / EXR 退化为元数据对比并提示用 DCC 工具。入口在工作区差异头部，暂存改动读 index 版本（`:0`）、未暂存读工作区。
- **LFS 历史迁移向导**（`PreviewLfsMigration` / `ExecuteLfsMigration`）：先用 `git lfs migrate info` 只读预览「哪些模式会动多少文件、共多大」，再强制「预览 → 创建回滚 Bundle → 迁移 → 提示强推」四步；未预览时执行按钮禁用，执行前二次确认重写风险。
- **修复中心**（`ScanFixCenter` / `ApplyAuditFix` / `RemediateStagedSecrets`）：把暂存区密钥、未走 LFS 的大文件、僵尸锁、忽略规则缺口聚合成一张「可修问题」清单，每项一键处置；密钥处置会 `git restore --staged` 后按需写入 `.gitignore`（去重）；改动文件前快照到 `.git/wgit-fix-backup/<时间戳>/`。
- **发布标签验签**（`VerifyTagSignature`）：补上 `git verify-tag` 这一环——此前只有提交验签，发布标签本身是否被篡改无从确认。轻量标签直接给出「没有签名对象」的解释，附注标签回读 GPG 状态并提取签名者。
- **资产影响分析**（`AnalyzeAssetImpact`）：从被改资产出发，顺着引用扫两级（贴图 → 材质 → 预制体/场景），每条引用附最近修改者与时间，按「直接优先、引用次数降序」排序，给出 none/low/medium/high 风险等级与负责人清单；二级展开上限 20 个直接引用（每次跳转都是一次全仓扫描），超限会明确标注截断。
- **引擎工程一键 Bootstrap**（`RunEngineBootstrap`）：把 `.gitattributes` 引擎预设（按行合并，不覆盖自定义）、`.gitignore` 补齐（可选取消跟踪已提交的构建产物）、UnityYAMLMerge、LFS 扩展名跟踪串成一次幂等配置，完成后重新体检并给出 0–100 就绪度。
- **操作日志与一键撤销**（`GetOperationLog` / `UndoLastOperation`）：直接读 `git reflog` 并翻译成人话（提交 / 修正提交 / 合并 / Cherry-Pick / Reset / 变基 / 切换分支…），标注哪些可一键撤销。撤销用 `reset --soft` 回到上一个位置——改动全部退回暂存区，绝不 hard reset 抹掉工作区；变基与切换分支明确拒绝并给出替代路径，有未解决冲突时直接拦住。入口在工具箱 → Reflog 页顶部。
- **UI 原语库（起步）**：`components/ui/` 下 `UiButton`（4 变体 × 3 尺寸）、`UiBadge`（6 色调）、`UiModal`（内置焦点陷阱/Esc/遮罩，header-body-footer 插槽）、`UiEmpty`、`UiSkeleton`。变体到 class 的映射集中在 `utils/uiVariants.ts`（完整字面量，Tailwind 能扫到），并带 60 条前端单测。`UiButton` 现有 6 个变体（primary / secondary / ghost / danger / warning / purple）。已迁移 18 处：`branches/modals/` 全部 5 个、标签详情、历史页 3 个（含 soft/mixed/hard 重置）、远程页 4 个、子模块页 3 个、储藏页 2 个、`FixCenterSection`；手写弹窗 **39 → 22** 处。剩余分两类：搜索面板 / 全屏页 / 变基计划编辑器等特殊结构（不适配 modal 模型，保持原样），以及 14 处零散单点，按新代码优先逐步替换。
- **列表右键菜单**（`useContextMenu` + `UiContextMenu`）：分支 / 标签 / 储藏三个列表统一支持右键，菜单按视口边界自动回夹（`open()` 先用估算尺寸夹一次，挂载后再实测一次），Esc / 外部点击 / 滚动 / 缩放都会关闭；支持 <kbd>↑</kbd><kbd>↓</kbd> 移动、<kbd>Home</kbd>/<kbd>End</kbd> 跳首尾、<kbd>Enter</kbd>/<kbd>Space</kbd> 执行，鼠标悬停与键盘焦点共用同一个高亮游标，禁用项自动跳过。菜单项随列表语义不同：分支给切换/合并/变基/上游/删除，标签给详情验签/检出/推送/删远程，储藏给应用/弹出/文件清单/删除。分支 / 标签 / 储藏的行均带 `aria-label`（含名称与当前态），屏幕阅读器可直接读出每一行是什么。
- **可访问名覆盖**：604 个 `<button>` 中，**纯图标按钮的 `aria-label` 缺口为 0**（97 处：92 处由静态 `title` 自动补齐，5 处关闭按钮补「关闭」）。应用壳层加了「跳到主内容」skip link（聚焦时显现，`sr-only focus:not-sr-only`），主内容区带 `id` 与 `tabindex="-1"`。
- **i18n 骨架**：`src/i18n/` 自带轻量实现（不引入 vue-i18n）——`translate()` 是纯函数，模块级 `locale` ref 驱动重渲染；缺失 key 回退到源语言再回退到 key 本身（保证缺口在界面上可见而不是空白）。现有 `zh-CN` / `en-US` 双语，覆盖导航、通用词汇、外观与语言设置、修复中心；设置 → 外观页可实时切换，`<html lang>` 与 `localStorage` 同步。测试强制「英文必须覆盖全部中文 key」「key 全小写且点分层」两条契约，防止翻译漏项。
- **内置终端**（`StartTerminal` / `WriteTerminal` / `ResizeTerminal` / `StopTerminal` / `GetTerminalStatus`）：用真实 PTY（`creack/pty`）在**当前仓库目录**下拉起登录 shell，输出经 Wails 事件推给前端 xterm.js。切走工具页不杀进程，输出保留 128KB 回放缓冲，重进自动续上；`ResizeTerminal` 同步 PTY 窗口尺寸，vim / htop / git log 分页器排版正确。终端主题色实时读设计令牌，跟随深/浅与强调色切换。
- **工作树修复与清理**（`RepairWorktrees` / `PruneWorktrees`）：仓库目录被移动后，各工作树的 `.git` 指针会失效，`worktree repair` 重新链接；`worktree prune --expire=now` 立即清掉目录已删除的条目（git 默认等 3 个月，用户点「清理」不该等到下个季度）。
- **二分跳过不可测提交**（`BisectSkip`）：中间某个提交编译不过时，`git bisect skip` 让会话继续而不是卡死；日志判定计数同步纳入 skip。
- **钩子脚本读写**（`ReadGitHook` / `SaveGitHook`）：此前只能启停，改脚本要去文件管理器。现在可直接编辑并保存，启用时自动写入 `0755`（git 不执行非可执行钩子），禁用时落为 `.disabled`；钩子名走白名单，杜绝通过名称拼出任意可执行路径。
- **统一确认对话框**（`confirmAction` + `ConfirmDialog`）：88 处原生 `window.confirm()` 全部替换为 Promise 化的应用内对话框——删除 / 丢弃 / 清空 / 强制 / 覆盖类操作自动标红危险色，删除未合并分支（输入分支名）、清空储藏箱（输入「清空」）、LFS 历史重写（输入 `MIGRATE`）三处要求输入确认词才能继续；Esc 取消、焦点陷阱、关闭后焦点归还由 `UiModal` 统一处理，不再阻塞渲染线程。

可达性盘点、缺陷修复记录与热点文件拆分方案见 [docs/architecture-audit.md](docs/architecture-audit.md)。

## 15. PR / MR 就地评审（GitHub / GitLab）

- **列表与筛选**（`ListPullRequests`）：从 `origin` 自动识别 GitHub / GitLab，按「待合并 / 已关闭 / 已合并 / 全部」四态拉取（GitHub 的 merged 需先查 closed 再按 `merged` 过滤），按更新时间倒序 50 条；标题、作者、源/目标分支、标签、草稿、CI 状态、增删行数一行内呈现，支持按标题 / 作者 / 分支本地过滤。
- **详情与文件补丁**（`GetPullRequestDetail`）：PR 描述、提交数、逐文件补丁（新旧行号、hunk 头、`\ No newline at end of file` 标记）、讨论（行内评论与普通讨论分别标注）、CI 检查（GitHub check-runs + combined status，GitLab pipelines）合并为一份详情；文件超过 100 个或单文件补丁超过 200KB 时明确标注截断，不静默丢内容。
- **就地评论**（`CreatePullRequestComment`）：点补丁左侧行号即可锚定「路径:行号」，评论直接落到对应行（GitHub `pulls/{n}/comments`；GitLab `discussions` + `position`），也可发普通讨论；`⌘/Ctrl + Enter` 提交。
- **评审与合并**（`SubmitPullRequestReview` / `MergePullRequest`）：批准 / 请求修改 / 仅评论三档；GitHub 走原生 review event，GitLab 没有原生 Request changes 时以带 🔴 前缀的讨论记录并明确告知；合并支持 merge / squash / rebase，平台拒绝时原样回传原因（分支保护、冲突、不可合并）。
- **令牌四种来源**：环境变量（`GITHUB_TOKEN` / `GH_TOKEN` / `GITLAB_TOKEN` / `GL_TOKEN` / `WGIT_PR_TOKEN`）→ 本地加密保险库 → `gh auth token` → `git credential fill`。令牌只进不出：后端只返回 `hasToken` 与来源描述，界面永远拿不到明文；保存前先打 `/user` 在线校验，401/403 直接拒绝且不落盘，只有网络失败才降级为「已保存但未校验」。存储复用 AES-256-GCM 保险库（`.git/wgit_vault.json`，`0600`），清除令牌会重写保险库并保留其他记录。
- **自建实例与降级**：`WGIT_PR_PROVIDER` / `WGIT_PR_API_BASE` 可覆盖域名识别与 API 根地址（自建 GitLab / GitHub Enterprise）；Gitee / Bitbucket / Gitea 等暂未内置 API 的托管平台会说明原因并保留「在浏览器打开」出口，而不是报一个看不懂的错误。

## 界面预览

| 场景 | 截图 |
| --- | --- |
| 提交历史 · 分页 / 筛选 / 虚拟滚动 / DAG | [02-history.png](docs/screenshots/02-history.png) |
| 服务端历史筛选（总数 2302 条） | [03-history-filtered.png](docs/screenshots/03-history-filtered.png) |
| ⌘P 快速打开文件 | [05-quickopen-results.png](docs/screenshots/05-quickopen-results.png) |
| 分支洞察与清理 | [07-branches.png](docs/screenshots/07-branches.png) |
| 多远程 Fetch / Prune 与工作树 | [09-tools-remotes.png](docs/screenshots/09-tools-remotes.png) |
| Blobless / 浅克隆接入 | [11-onboarding-clone.png](docs/screenshots/11-onboarding-clone.png) |
| 多选提交与批量操作 | [14-history-multiselect.png](docs/screenshots/14-history-multiselect.png) |
| 交互式变基计划编辑器 | [15-rebase-plan.png](docs/screenshots/15-rebase-plan.png) |
| Git Bisect 二分定位 | [18-bisect-active.png](docs/screenshots/18-bisect-active.png) |
| 命令面板（⌘K）统一入口 | [19-command-palette.png](docs/screenshots/19-command-palette.png) |
| 多仓库标签栏（wgit + project-alpha） | [20-multi-repo-rail.png](docs/screenshots/20-multi-repo-rail.png) |
| 多仓库总览（聚合状态 / 批量 Fetch） | [21-repo-dashboard.png](docs/screenshots/21-repo-dashboard.png) |
| 变更评审工作区（逐文件结论 / 行级评论） | [22-change-review.png](docs/screenshots/22-change-review.png) |
| 发布向导与 Studio Changelog | [23-release-wizard.png](docs/screenshots/23-release-wizard.png) |
| 稠密 DAG 车道图谱 | [24-dense-graph.png](docs/screenshots/24-dense-graph.png) |
| 提交补丁预览（inspector） | [25-commit-patch.png](docs/screenshots/25-commit-patch.png) |
| 最小窗口 960×640 · 欢迎页完整可见 | [28-onboarding-960x640.png](docs/screenshots/28-onboarding-960x640.png) |
| 最小窗口 · 工作区头部与提交框自适应 | [30-workspace-960x640.png](docs/screenshots/30-workspace-960x640.png) |
| 最小窗口 · 分支页头部自动换行 | [32-branches-960x640.png](docs/screenshots/32-branches-960x640.png) |
| 最小窗口 · 命令面板（含工作树/Studio/引擎命令） | [29-command-palette-960x640.png](docs/screenshots/29-command-palette-960x640.png) |
| 命令面板直达 · 扫描本地 Git 工程 | [34-onboarding-scan-mode.png](docs/screenshots/34-onboarding-scan-mode.png) |
| Git Notes 批注总览（命令面板直达） | [35-git-notes-overview.png](docs/screenshots/35-git-notes-overview.png) |
| 最小窗口 · 变更评审三栏自适应布局 | [36-review-960.png](docs/screenshots/36-review-960.png) |
| ⌘K 直达 · 仓库健康体检报告（自动滚动定位） | [37-tools-health-report.png](docs/screenshots/37-tools-health-report.png) |
| ⌘K 直达 · 热更清单 (Hotfix Manifest) | [38-tools-hotfix.png](docs/screenshots/38-tools-hotfix.png) |
| ⌘K 直达 · 归档 / 补丁 / Git Bundle 离线备份 | [39-tools-archive-bundle.png](docs/screenshots/39-tools-archive-bundle.png) |
| 最小窗口 · 评审评论与详情列再优化 | [40-review-960-widths.png](docs/screenshots/40-review-960-widths.png) |
| ⌘K 直达 · Unity Meta 配对检查分区 | [43-engine-unity-meta-deeplink.png](docs/screenshots/43-engine-unity-meta-deeplink.png) |
| 命令面板 · Git LFS 分类命令（搜索过滤） | [44-palette-lfs-commands.png](docs/screenshots/44-palette-lfs-commands.png) |
| 命令面板 · 最近使用排序（持久化 + 紧凑分组） | [45-palette-recent-persisted.png](docs/screenshots/45-palette-recent-persisted.png) |
| 命令面板 · 收藏置顶（★ 持久化 + 一键取消） | [46-palette-pinned.png](docs/screenshots/46-palette-pinned.png) |
| 命令面板 · 分组折叠（点击分类标题收起，键盘导航自动跳过） | [47-palette-collapsed.png](docs/screenshots/47-palette-collapsed.png) |
| 命令面板 · 收藏排序（↑↓ 调整顺序并持久化） | [48-palette-pin-reorder.png](docs/screenshots/48-palette-pin-reorder.png) |
| Git Hooks 管理 · 状态 + 定位/启用/禁用 | [49-hooks-reveal.png](docs/screenshots/49-hooks-reveal.png) |
| 命令面板 · 一键折叠全部 / 展开全部 | [50-palette-collapse-all.png](docs/screenshots/50-palette-collapse-all.png) |
| 命令面板 · 收藏拖拽排序（持久化） | [51-palette-drag-reorder.png](docs/screenshots/51-palette-drag-reorder.png) |
| Git Hooks 管理 · 批量启用/禁用 + 完成提示 | [52-hooks-bulk-toggle.png](docs/screenshots/52-hooks-bulk-toggle.png) |
| 源码浏览器 · 文件操作菜单（gitignore/取消跟踪/重命名/删除） | [53-file-actions-menu.png](docs/screenshots/53-file-actions-menu.png) |
| 命令面板 · 加权搜索结果（收藏优先 + 前缀匹配） | [54-search-ranked.png](docs/screenshots/54-search-ranked.png) |
| 源码浏览器 · 在 Finder 显示 / 移除 .gitignore 规则 | [55-file-menu-finder-gitignore.png](docs/screenshots/55-file-menu-finder-gitignore.png) |
| 源码浏览器 · 目录级操作菜单（忽略目录 / Finder） | [56-dir-actions-menu.png](docs/screenshots/56-dir-actions-menu.png) |
| 命令面板 · 搜索命中词高亮 | [57-search-highlight.png](docs/screenshots/57-search-highlight.png) |
| .gitignore 规则管理面板（查看 / 追加 / 移除） | [58-gitignore-editor.png](docs/screenshots/58-gitignore-editor.png) |
| .gitignore 规则管理 · 追加规则后 | [59-gitignore-editor-add.png](docs/screenshots/59-gitignore-editor-add.png) |
| 目录批量操作（暂存整个目录 / 取消暂存 / 忽略 / Finder） | [60-dir-batch-stage.png](docs/screenshots/60-dir-batch-stage.png) |
| .gitignore 面板 · 分组展示 + 内联重命名 | [61-gitignore-editor-groups.png](docs/screenshots/61-gitignore-editor-groups.png) |
| 命令面板 · 选中项强调色左侧条 | [62-palette-selected-accent.png](docs/screenshots/62-palette-selected-accent.png) |
| 源码树 · 展开/折叠全部（⌘⇧E + 按仓库持久化） | [63-tree-expanded-persisted.png](docs/screenshots/63-tree-expanded-persisted.png) |
| .gitignore 规则 · 上移/下移调整优先级 | [64-gitignore-move-rules.png](docs/screenshots/64-gitignore-move-rules.png) |
| 设置 · 快捷键一览（全局 / 工作区分组，⌘⇧E、⌥S 等） | [65-shortcuts-settings.png](docs/screenshots/65-shortcuts-settings.png) |
| .gitignore 规则 · 一步撤销（添加/移除/修改/排序） | [66-gitignore-undo.png](docs/screenshots/66-gitignore-undo.png) |
| 版本对比 · 任意分支 / 标签 / Commit（含交换按钮） | [67-compare-refs.png](docs/screenshots/67-compare-refs.png) |
| 版本对比 · 差异详情（文件列表 + 补丁预览） | [68-compare-refs-diff.png](docs/screenshots/68-compare-refs-diff.png) |
| 源码浏览器 · 平铺模式按顶层目录分组 | [69-flat-grouped.png](docs/screenshots/69-flat-grouped.png) |
| 版本对比 · 三点对比（merge-base，文件可点击跳转） | [70-compare-three-dot.png](docs/screenshots/70-compare-three-dot.png) |
| 源码浏览器 · 平铺分组折叠（按仓库持久化） | [71-flat-group-collapsed.png](docs/screenshots/71-flat-group-collapsed.png) |
| 版本对比 · 单文件差异预览（可跳转工作区） | [72-compare-file-diff.png](docs/screenshots/72-compare-file-diff.png) |
| 设置 · 快捷键自定义录制（冲突检测 + 持久化） | [73-shortcuts-customize.png](docs/screenshots/73-shortcuts-customize.png) |
| 平铺模式 · 一键折叠全部分组 | [74-flat-all-collapsed.png](docs/screenshots/74-flat-all-collapsed.png) |
| 设置 · 全部快捷键可录制（含文件/搜索/差分/源码树/提交） | [75-shortcuts-all-customizable.png](docs/screenshots/75-shortcuts-all-customizable.png) |
| 快捷键录制 · 实时修饰键预览 + 冲突行高亮 | [76-shortcut-conflict-highlight.png](docs/screenshots/76-shortcut-conflict-highlight.png) |
| 设置 · 分类搜索过滤（支持中文/英文关键词） | [77-settings-search.png](docs/screenshots/77-settings-search.png) |
| 快捷键配置 · JSON 导出 / 导入面板 | [78-shortcut-export-panel.png](docs/screenshots/78-shortcut-export-panel.png) |
| 工作区 · 面板宽度持久化（双击分隔条恢复默认） | [79-pane-persisted.png](docs/screenshots/79-pane-persisted.png) |
| Git Hooks · 单条切换局部高亮反馈（1.8s 自动淡出） | [80-hooks-toggle-highlight.png](docs/screenshots/80-hooks-toggle-highlight.png) |
| 工作区 · 拖拽调节时实时宽度提示（变更栏 260px） | [81-resize-width-hint.png](docs/screenshots/81-resize-width-hint.png) |
| 设置 · 设置项级搜索（如「邮箱」定位 Git 身份认证） | [82-settings-item-search.png](docs/screenshots/82-settings-item-search.png) |
| 设置搜索 · 命中词高亮 + 键盘选择 | [83-settings-search-mark.png](docs/screenshots/83-settings-search-mark.png) |
| 设置搜索 · 分类名命中高亮（如「代理」） | [84-settings-category-mark.png](docs/screenshots/84-settings-category-mark.png) |
| 工作区布局预设 · 专注源码（480 / 240） | [85-layout-focus-source.png](docs/screenshots/85-layout-focus-source.png) |
| 设置搜索 · 最近搜索记忆（点击复用） | [86-settings-search-history.png](docs/screenshots/86-settings-search-history.png) |
| 工作区头部 · 三种布局预设快捷切换 | [87-layout-preset-header.png](docs/screenshots/87-layout-preset-header.png) |
| 设置搜索 · 最近搜索键盘选择（↑↓ + Enter） | [88-settings-history-keyboard.png](docs/screenshots/88-settings-history-keyboard.png) |
| 工作区 · 命名自定义布局预设（最多 5 套，可应用/删除） | [89-layout-presets-popover.png](docs/screenshots/89-layout-presets-popover.png) |

## 目录结构

```
main.go                # Wails3 入口（嵌入 frontend/dist）
config.go              # 全局配置（~/.config/wgit，兼容 ~/.config/ugit）
dialog_helper.go       # 原生目录选择（Wails → AppleScript / PowerShell / Zenity 降级）
git_service.go         # GitService 核心：仓库/提交/分支/暂存/远程/变基/bisect（165 个 RPC 方法）
git_lfs_lock.go        # LFS 深度维护与可锁资产守护（25 个方法）
git_engine_asset.go    # 引擎/资产管线巡检：Unity/Unreal、纹理/音频/网格（16 个方法）
git_studio_ops.go      # Studio CI / 团队活动 / 自定义流水线 / 凭据巡检（9 个方法）
git_studio_engine.go   # Round 101–140：引擎/资产管线审计、Studio CI/TAPD、凭据与合规（44 个方法）
git_project_browser.go # 工程项目浏览：多工程元数据、源码全景、代码搜索（7 个方法）
git_exec.go            # git CLI 执行封装
git_parser.go          # 类型定义与 porcelain/log/diff 解析
git_graph.go           # 提交 DAG 车道布局与 git log 解析
git_large_repo.go      # 大型项目专用：提交计数、模糊文件搜索、-z 路径解析、并行 stat、diff 截断
git_sync_ops.go        # Pull/Push/Merge/Rebase 高级选项与冲突流程
git_external_tools.go  # 外部对比/合并工具探测与托管平台联动
git_review.go          # 变更评审会话（.git/wgit-review）
git_release.go         # 发布向导与 Studio Changelog
git_notes.go           # Git Notes 批注
git_health.go          # 仓库体检（fsck / gc / LFS prune）
git_workspace.go       # 工作区统计
git_insights.go        # 团队活动、Pickaxe 检索、提交信息规范
git_branch_insights.go # 分支洞察（已合并/未推送/停滞分类）
git_stash_insights.go  # 储藏拆解（文件清单）
git_*_test.go          # 单元测试；git_large_repo_live_test.go / git_repo_stress_test.go 为真实仓库实测（见下）
frontend/
  design-system/       # 设计令牌（181 个）与在线样式手册
  src/
    App.vue            # 多仓库标签状态、视图切换、跨视图事件总线（open-file / tools-tab / history-rebase / onboarding）
    components/        # Workspace / History / Branches / Stash / Review / Tools / Settings
                       # RepoRail（多仓标签）、RepoDashboard（多仓总览）、QuickOpen（⌘P/⌘⇧F）
                       # RepoOnboarding（打开/克隆/初始化）、BisectPanel、ReleaseWizard
    components/tools/  # ToolsView 的 8 个页签子组件（Reflog/Clean/Remotes/Lfs/Submodules/Sparse/Disk/Maintenance）
    components/settings/ # SettingsView 的 6 个分区组件（Identity/Signing/Proxy/Storage/Tools/Shortcuts）
    components/branches/ # BranchesView 的 5 个列表/对比组件 + modals/ 下 6 个弹窗
    components/review/  # ReviewView 的会话列 / 文件列 / 文件详情（含 diff）/ 导出弹窗
    components/history/ # HistoryView 的筛选栏 / 批量工具栏 / 提交检视器 / DAG 虚拟列表
    components/workspace/ # WorkspaceView 的源码浏览器 / 源码全景 / 差分渲染 / 变更列表 / 提交器 / 冲突横幅
    components/ui/     # 通用 UI 原语（Toast）
    composables/       # 跨组件 UI 状态（toolsStatus / useToast 等）
    services/gitService.ts # Wails RPC 封装 + 浏览器预览 mock
docs/                  # 界面截图与架构盘点（architecture-audit.md）
```

## 测试与验证

```bash
go test .                     # Go 单元测试（含大型项目解析/搜索/截断/bisect/批量 cherry-pick 用例）
cd frontend && bun run test   # 前端单元测试（Bun 内置 runner，46 个用例 / ~50ms）
cd frontend && bun run typecheck && bun run build

# 针对真实大型仓库的实测（需自备包含数百分支/数千提交的仓库）
WGIT_LARGE_REPO=/path/to/big-repo go test -run TestLiveLargeRepo -count=1 -v .

# 10 万文件 Monorepo 压测（Python 生成 fixture：20×25 目录 ×200 文件）
WGIT_STRESS_REPO=/tmp/wgit-stress-repo go test -run TestStressLargeMonorepo -count=1 -v .
```

> `typecheck` 使用 `vue-tsc`，覆盖 `.vue` 组件内部类型；`bun run build` 也会先执行同一套类型检查。
> 前端测试放在 `frontend/tests/`（不在 `tsconfig` 的 include 内，因此不参与打包），用 Bun 自带的 `bun:test`，不引入 vitest / jsdom；覆盖快捷键注册表（解析、冲突、覆写持久化）、资产差值阈值与资格判定、设置搜索索引的结构约束。

实测参考：
- 2302 commits / 276 branches 仓库：提交总数 19ms、300 条历史 33ms、筛选查询 40ms、分支列表 11ms、分支洞察 226ms、文件搜索 9ms、代码搜索 34ms。
- 100,000 文件 Monorepo：源码清单 578ms（5.8µs/文件）、RPC JSON 10.2MB（marshal 22ms）、服务端文件搜索 84ms、工作区状态 86ms。
