# wgit 20 轮优化记录（2026-10-26 起）

> 目标：每轮按「缺陷/边界、错误处理、可维护性、性能、安全、日志、测试、文档」审查，输出 P0–P2 问题清单；并行派发 3–8 个文件不冲突任务；统一构建/测试/回读验证。不得换技术栈或擅改架构；测试不过不算完成。

## 基线（第 1 轮前实测）

- Git：HEAD `dfaf6b6`，origin/main `4edfa5c`，本地领先 24 个提交。
- Go：397 测试，覆盖率 76.7%。
- 前端：597 测试 / 38 文件。
- i18n：3,808 × 2（文档旧值）。

## 第 1 轮 — 错误处理 + 文档

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P1 | frontend/src/components/BranchesView.vue:846-853 | 合并预览与重叠分析各自 `.catch(() => null)`，失败后弹窗如同“无冲突/无重叠”，用户可盲合并 |
| P1 | frontend/src/components/tools/EngineTab.vue:223-233 | 资产元数据/引用分别吞错；引用失败仍提示“元数据已读取”，删除风险不可见 |
| P2 | git_health.go:266 | `rev-parse --git-dir` 失败被忽略，中断操作/冲突检测静默跳过 |
| P2 | git_studio_engine.go:4026,4042 | Studio 拓扑的分支/最近提交读取失败无提示 |
| P2 | docs/architecture-audit.md / README.md | 基线滞后（376/194/3,808 vs 实际 397/597/4,242） |
| P2（系统性） | 全仓 | 非测试 Go 仍有 63 处忽略 `ExecGit` 错误、前端 31 处空 catch / 静默 fallback；作为后续轮次 backlog |

### 并行任务（4 个，文件不冲突）

1. BranchesView 合并预检失败可见化；统一新增 3 个双语 key（`branch.toast.merge.preview.failed`、`engine.toast.asset.partial`、`health.workspace.detail.unverified`）。
2. EngineTab 资产检查改为 `Promise.allSettled`，部分失败保留成功数据并只发一次失败 Toast。
3. git_health 捕获 git-dir 失败，工作区维度降级为 warn(55) 并附 `health.workspace.detail.unverified`；新增伪 git 回归测试。
4. Studio 拓扑捕获分支/最近提交读取失败，并入 `statusWarning`；新增 log 失败回归测试。

### 改动

- `frontend/src/components/BranchesView.vue`：`allSettled` + `mergePreviewError` + 弹窗打开期间的 `role=alert` 顶部警告；repo 切换时清理。
- `frontend/src/components/tools/EngineTab.vue`：`allSettled`；失败清空旧面板；部分失败只提示 `engine.toast.asset.partial`。
- `frontend/src/i18n/messages/zh-CN.ts` / `en-US.ts`：+3 key，逐 key 对齐。
- `git_health.go` + `git_health_unverified_test.go`（新）：git-dir 失败时 workspace warn(55)，其余检查继续。
- `git_studio_engine.go`：分支/最近提交失败并入 `warnings`；`git_switchboard_test.go` 新增 `TestStudioSwitchboardReportsCommitMetadataFailure`。
- `docs/architecture-audit.md` / `README.md`：基线同步为 399 Go / 76.9% / 597 前端 / 4,242 i18n。

### 验证

- Go：`go vet` 通过；`go test -cover` 399 测试、76.9%；`-race -shuffle` 通过；server 与 desktop 构建通过。
- 前端：`vue-tsc`、597 测试、生产构建通过；i18n 契约测试通过（key 对齐、无硬编码中文）。
- 范围核对：4 个 agent 仅修改各自许可文件；未触碰另一并行会话在途的 `git_service.go` / `git_subtree.go` / `gitService.ts` / `types.ts`。

### 遗留 / 下一轮候选

- Studio 拓扑“分支读取失败”路径尚无直接测试（当前只覆盖 log 失败）。
- 合并预检警告浮层尚未做浏览器 E2E/视觉核对。
- 系统性忽略错误 backlog：63 处 Go / 31 处前端，按用户可见度逐轮清理。
- 下一轮建议维度：性能（包体/大列表渲染/重复 Git 调用）与可维护性（超大文件热点）。

## 第 2 轮 — 性能 + 可维护性

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 | git_project_browser.go:37 | 多仓扫描逐仓串行执行多条 git 命令，N 个仓库线性等待 |
| P2 | git_branch_insights.go:148 | 每条分支串行 `rev-list` 计算领先/落后 |
| P2 | BranchesView.vue（R1 产物） | 合并预检警告是 body Teleport 浮层，脱离弹窗组件 |
| P2（延后） | git_service.go:907/2998/5056 | formatBytes / formatByteSize / formatAssetSize 三套实现；`git_service.go` 正被并行会话拆分，未动 |
| P2（延后） | git_studio_engine.go 10.6k 行；WorkspaceView/HistoryView 约 100KB | 超大热点文件，需独立拆分方案 |
| P2（新） | frontend/dist/assets/i18n-*.js 564KB | 双语 4,242 key 全量打入单一 chunk，首页负载偏大 |

### 并行任务（3 个，文件不冲突）

1. `GetProjectsDetails` 改为最多 8 并发的有界 worker pool，按输入索引写回，保持顺序；新增顺序/空路径测试。实测 12 仓扫描 366-459ms → 110-119ms（约 3.3-4.1×）。
2. `GetBranchInsights` 仅并行 per-branch `aheadBehind`，预取与分类/排序逻辑不动；新增 9 分支测试（含超过并发上限、Behind/Ahead 断言）。
3. 合并预检警告移入 `MergeBranchModal`（新增 `previewError` prop），移除 BranchesView 的 Teleport/AlertTriangle；状态与重置逻辑不变。

### 改动

- `git_project_browser.go` + `git_project_details_parallel_test.go`（新）
- `git_branch_insights.go` + `git_branch_insights_parallel_test.go`（新）
- `frontend/src/components/BranchesView.vue`、`frontend/src/components/branches/modals/MergeBranchModal.vue`
- 文档基线：Go 402 / 77.0%

### 验证

- Go：`go vet`；`go test -cover` 402 测试 / 77.0%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、597 测试、生产构建通过。
- 并发测试均以 `-race` 运行；未触碰在途 `git_service.go` / `git_subtree*.go` / `gitService.ts` / `types.ts`。

### 遗留

- i18n chunk 564KB、index 408KB，下一轮性能候选（locale 延迟加载 / vendor 拆分需先评估架构影响）。
- 三套体积格式化函数待 `git_service.go` 并行会话收敛后统一。
- 超大文件拆分继续排队（不做无方案重构）。

## 第 3 轮 — 安全 + 日志

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P1 | git_hooks.go:104,239 | `RunGitHook` / `InstallSampleHook` 未校验 hookName，`../`、绝对路径可逃出 hooks 目录（执行/写入） |
| P1 | git_exec.go:46-49 | 活动日志原样保存 args/stdout/stderr，远程 URL 内嵌 token 可泄露到 UI |
| P1（延后） | git_service.go:2507 | `ToggleGitHook` 同样缺 hookName 校验；该文件正被并行会话拆分，下一轮补 |
| P2 | git_token 测试缺口 | Studio 拓扑“分支读取失败”路径无直接测试 |

### 并行任务（3 个，文件不冲突）

1. git_hooks.go 新增 `validHookName`：拒绝空名、`/`、`\`、`..`、绝对路径、前导 `-`、空白/控制字符与点号；仅允许字母/数字/`_`/`-`。接入 `RunGitHook` 与 `InstallSampleHook`；新增 4 组子测试（含 canary 校验与正常运行名）。
2. `recordGitCommand` 复用 `maskCredentialURL`：Command/Output/Error 入库前脱敏，Output 先脱敏后截断；新增活动日志脱敏测试（userinfo + query token + 输出/错误回显）。
3. 扩展 `git_switchboard_test.go`：相邻仓库 + 伪 git 仅让 `rev-parse --abbrev-ref` 失败，断言相邻节点 `StatusWarning` 含「读取当前分支失败」，主工作树不误报。

### 验证

- Go：`go vet`；`go test -cover` 405 测试 / 77.0%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、597 测试、生产构建通过（本轮无前端改动）。
- hook 校验做了红-绿验证（临时短路后遍历用例失败，恢复后通过）。

### 遗留

- `ToggleGitHook`（git_service.go）仍需接入同一 hookName 校验，待并行拆分收敛。
- 活动日志仅按 URL 凭据模式脱敏；独立裸令牌如需更强脱敏，下一轮评估。
- 第 2 轮遗留：i18n/index chunk 拆分、三套格式化函数合并、超大文件拆分。

## 第 4 轮 — 测试 + 文档

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P1（R3 遗留） | git_service.go:2507 | `ToggleGitHook` 未校验 hookName，可越界重命名/写入 |
| P2 | docs/architecture-audit.md / README.md | 文档仍写 363 RPC；实测后端 388、facade 388、UI 直达 379 |
| P2 | BranchesView.vue / branches/LocalBranchList.vue | `formatInsightDay/formatDaysAgo` 重复实现，且两处 i18n key 族不一致 |
| P2（新发现） | frontend/src/services/gitService.ts | 9 个 facade 封装零 UI 调用点：Commit / StashSave / AddWorktree / MoveWorktree / SplitSubtree / UpdateSubtreeMetadata / UnlinkSubtree / GetFileBlame / RemediateStagedSecrets |

### 并行任务（3 个，文件不冲突）

1. `ToggleGitHook` 改用共享 `resolveHookPaths` 白名单校验；无效名在任何文件操作前报「不支持的钩子名称」；新增 enable/disable 双向非法名 + canary 测试，并验证 `pre-commit` 启停流程不回归。
2. 文档 RPC 对账：后端 388（git_service 173 / studio 44 / lfs 25 / engine 16 / external 12 / closed_loop 8 / pr 8 / hooks 4 / 其余 98）、facade 388 双向零差异、UI 实际引用 379；README 363→388；修正旧的 per-file 合计与反引号不平衡。
3. 抽出 `frontend/src/utils/branchInsightFormat.ts`，LocalBranchList 复用；BranchesView 中两份 helper 实为无引用死代码，删除不引入未使用 import。保留实际渲染的 `branches.local.*` 文案族，输出逐值核对。

### 验证

- Go：`go vet`；`go test -cover` 407 测试 / 77.0%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、597 测试、生产构建通过；RPC parity 测试通过。
- 无效 hookName 覆盖空串、空白、`../../evil`、绝对路径、`..\evil`、非白名单 hook；canary 均不存在。

### 遗留

- 9 个零调用的 facade 封装需要下一轮逐一定性：接回 UI 或确认重复后删除（不得直接删而无证据）。
- `branch.days.*` 旧 i18n key 现已无引用，可在后续 i18n 清理轮评估移除。
- R2/R3 遗留：i18n/index chunk、三套体积函数、超大文件拆分、裸令牌增强脱敏。

## 第 5 轮 — 性能 + 边界/测试

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 性能 | git_health.go | 六项体检顺序执行，Git 调用延迟叠加 |
| P2 边界 | git_service.go ToggleGitHook | 启用不存在且无内容时落到 chmod ENOENT，提示不清晰 |
| P2 测试 | utils/branchInsightFormat.ts | 无直接单测，边界值未钉住 |
| 说明 | gitService.ts | 9 个零调用封装经核查多为高级变体兼容别名（Commit/StashSave/AddWorktree/GetFileBlame 由 *Advanced/WithOptions 取代；Subtree/MoveWorktree/Remediate 为独特能力但 UI 未接线），本轮不删除、不做无证据改动 |

### 并行任务（3 个，文件不冲突）

1. 健康体检并行化：六项检查改为最多 6 并发，索引写回；报告顺序、状态/分数/明细/汇总完全不变。测试耗时 0.62s → 0.16s；race 通过；新增 deterministic 顺序/自洽测试。
2. ToggleGitHook 边界：启用不存在 hook 且无脚本内容时返回「钩子不存在，无法启用」，不再落到 chmod ENOENT；提供内容时仍可新建。新增回归测试。
3. 新增 `frontend/tests/branchInsightFormat.test.ts`（11 测试）：覆盖日期切片/非法回退、天数 0/负/1/29/30/364/365/大值，中英双语输出与 locale 恢复。

### 单测暴露的文案边界（未改，下一轮候选）

- 英文月/年分支缺单数：30 天 → "1 months ago"，365 天 → "1 years ago"（P3 文案）。
- `formatDaysAgo(undefined/null/NaN)` 返回空串而非“今天”（实现如此，测试已按真实行为钉住）。
- `formatInsightDay` 仅校验 `YYYY-MM-DD` 形状，`2026-13-40` 会通过。
- 天分支不做取整（整数输入下无影响）。

### 验证

- Go：`go vet`；`go test -cover` 409 测试 / 77.0%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、608 测试 / 39 文件、生产构建通过。

### 遗留

- 英文 plural 文案修正；9 个未接线 RPC 的取舍（接回 UI 或明确废弃）；R2/R3/R4 的性能与 i18n 清理项继续排队。

## 第 6 轮 — 文案边界 + 真实性 + 性能

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P3 | utils/branchInsightFormat.ts | 英文缺单数：`1 months ago` / `1 years ago`；日期只校验形状 |
| P2 | git_branch_insights.go aheadBehind | per-branch `rev-list` 失败吞成 0/0，看起来“已同步” |
| P2 | git_service.go GetMultiRemoteStatus | 逐远端串行 2 次 rev-list，N 个远端线性等待 |
| P2（延后） | 三套体积格式化、i18n chunk 564KB、超大文件、9 个未接线封装 | 继续排队 |

### 并行任务（3 个，文件不冲突）

1. 英文单数 + 日期校验：新增 `branches.local.month.one` / `year.one`（zh/en 对齐），`formatDaysAgo` 在计数为 1 时使用单数；`formatInsightDay` 用 UTC 往返校验真实日历日（拒绝 2026-13-40、2026-02-30、非闰年 2/29）；测试更新为 13 条，31/366 按规则仍属单数，另加 60/730 复数覆盖。
2. 分支洞察真实性：`aheadBehind` 返回 error，`BranchInsight` 新增 `statusWarning`；失败时 Ahead/Behind=0 且给出「读取领先/落后失败」，不失败整体请求；LocalBranchList 显示 `!` 徽标；新增伪 git rev-list 失败测试，正常仓库断言空警告。
3. 多远端状态并行化：仅 per-remote `rev-parse + 2×rev-list` 用最多 8 并发，按索引写回，最终仍按名称排序；新增 3 远端成功排序测试 + 缺失跟踪分支测试，race 通过。

### 验证

- Go：`go vet`；`go test -cover` 412 测试 / 77.1%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试 / 39 文件、生产构建通过；i18n 4,244×2 对齐。

### 遗留

- 9 个未接线 RPC 的取舍；`branch.days.*` 旧 key 清理；R2–R5 的性能/i18n 清理项。

## 第 7 轮 — 测试覆盖 + 安全（裸令牌）

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 测试 | git_release.go suggestNextVersions | 版本建议核心逻辑零覆盖 |
| P2 测试 | git_project_browser.go getImageMimeType | 图片 MIME 映射零覆盖，易与 isImageExtension 漂移 |
| P2 安全 | git_exec.go recordGitCommand | 仅按 URL 凭据脱敏；裸 `ghp_`/`glpat-`/`AKIA`/`token=` 可泄露到活动日志 |
| P2（发现） | git_release.go semverTagRe | 预发布/构建后缀与大写 `V` 标签会回退 0.1.0，丢失既有版本线 |
| P2（延后） | Web URL 构造零覆盖且未做路径转义 | 下一轮评估 |

### 并行任务（3 个，文件不冲突）

1. 发布版本建议表驱动测试（21+5 子测试）：v/无 v、feat/art/ui/level/asset/audio/design 前缀与分隔符、非语义化标签、空标签、预发布/大写 V 回退等；覆盖率达到 100%，并记录 5 项真实行为异常。
2. 图片 MIME 表驱动测试（21 行 + isImageExtension 一致性守卫）：覆盖全部映射、未知/空/大写/无点/完整文件名；确认调用方已 lowerCase，函数契约按实际钉住；覆盖率 100%，无生产改动。
3. 活动日志裸令牌脱敏：新增 GitHub/GitLab/AWS/key=value 四类正则，`redactLogSecrets` 先做 URL 脱敏再裸令牌脱敏；扩展测试验证 `ghp_***`/`glpat-***`/`AKIA***`/`token=***&ok=1` 且普通文本不误伤。

### 验证

- Go：`go vet`；`go test -cover` 417 测试 / 77.2%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试、生产构建通过（本轮无前端改动）。

### 遗留

- 下一轮优先修发布版本建议：预发布/构建标签保留基础版本、支持大写 V、空标签行为统一；需补对应测试。
- 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件与 `branch.days.*` 清理继续排队。

## 第 8 轮 — 缺陷 + 安全（URL）+ 日志

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 缺陷 | git_release.go semverTagRe | 预发布/构建标签与大写 V 回退 0.1.0，丢失版本线 |
| P2 缺陷/安全 | git_external_tools.go Open*Web | 文件路径/分支未 URL 转义，空格或 `#` 会截断链接；五个入口零覆盖 |
| P3 日志 | WorktreesTab / HistoryView | prune 详情与变基全文写入 console，重复且可能泄露内容 |
| P2（延后） | 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件 | 继续排队 |

### 并行任务（3 个，文件不冲突）

1. 版本解析：正则改为 `^([vV]?)(\d+)\.(\d+)\.(\d+)(?:[-+][0-9A-Za-z.-]+)?$`，保留前缀大小写、接受预发布/构建后缀并按其数字基线升级（v1.2.3-rc.1 → v1.2.4/v1.3.0，V1.2.3 → V1.2.4）；测试补齐 build/大写 V/feature 组合。
2. Web URL：新增 `escapeWebPath`（按 `/` 分段 PathEscape）与五个纯 `build*WebURL`；文件路径 `art/my file#1.png` → `art/my%20file%231.png`，分支斜杠保留；GitHub/GitLab 模板、缺失远端、OpenWebURL 协议校验全部测试覆盖。
3. 日志：移除 WorktreesTab prune 详情与 HistoryView 变基全文 console 输出；无变量残留、无 UI 变化。

### 验证

- Go：`go vet`；`go test -cover` 424 测试 / 77.2%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试、生产构建通过。

### 遗留

- 9 个未接线 RPC 的取舍；`GetReleasePlan` 分支读取失败仍显示 HEAD (detached)；i18n chunk、体积函数、超大文件、`branch.days.*` 清理。

## 第 9 轮 — 安全（错误通道）+ 真实性 + 测试

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P1 安全 | git_exec.go:172-183 | 超时/失败分支把原始 stderr 直接塞进 error，凭据 URL 与裸令牌随 RPC 进入 UI；活动日志已脱敏但错误通道没有 |
| P1 真实性 | git_release.go:72-79 | `branch --show-current` 的错误被丢弃，空输出一律标 `HEAD (detached)`；读取失败被伪装成合法的分离头状态 |
| P2 测试 | git_studio_engine.go:5282,8999 | `demangleSwift` / `isJSONCollectionEmpty` 零覆盖；前者每次调用重复编译正则 |
| P2（发现，延后） | git_studio_engine.go demangleSwift | 捕获组允许数字，贪婪吞掉后续长度前缀，真实符号 `$s10MyAppName7MyClassC5runyyF` 无法分段；需按长度前缀解析，另行评估 |
| P2（延后） | 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件、`branch.days.*` | 继续排队 |

### 并行任务（3 个，文件不冲突）

1. 错误通道脱敏：`execGitTimeout` 在记录日志后用 `redactLogSecrets(errStr)` 的两处 error 返回；新增伪 git 测试覆盖退出 128 与 100ms 超时两条路径，先红后绿；测试预热规避首次 exec 冷启动导致超时早于 stderr 写入。
2. 发布分支真实性：改用 `rev-parse --abbrev-ref HEAD` 并保留错误；`HEAD` → `HEAD (detached)`（保持原 warn），空输出/读取失败 → `UNKNOWN` + 阻塞 fail 检查（`release.check.branch.detail.unknown`，zh/en 同步）；新增 detached / 非仓库 / 正常分支三组测试。
3. 纯函数测试与正则预编译：`swiftDemanglePartRe` 提升为包级变量；新增 15 条 `demangleSwift` 与 8 条 `isJSONCollectionEmpty` 表驱动测试，两者覆盖率 100%；记录贪婪匹配异常并保留现状。

### 验证

- Go：`go vet`；`go test -cover` 431 测试 / 77.3%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试 / 39 文件、生产构建通过；i18n 4,245×2 对齐。

### 遗留

- `demangleSwift` 的 Swift 长度前缀解析缺陷（P2，影响真实符号分段）需单独设计，不能只改字符类而不验证。
- 9 个未接线 RPC 的取舍；i18n chunk、体积函数、超大文件、`branch.days.*` 清理继续排队。

## 第 10 轮 — 缺陷 + 性能 + 错误处理 + 测试

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 缺陷 | git_studio_engine.go:5284 | `demangleSwift` 捕获组允许数字，贪婪吞掉后续长度前缀，真实 Swift 符号无法分段（第 9 轮遗留） |
| P2 性能 | git_notes.go:32-105 | 最多 200 条批注 × 2 条 git 命令串行执行，面板需等最多 400 个进程 |
| P2 错误处理 | git_worktree.go:143-146 | `PruneSingleWorktree` 回退分支忽略 `os.RemoveAll` 错误，目录未删仍返回成功 |
| P1 真实性 | git_lfs_lock.go:233 | `CheckRemoteLocksConflict` 把锁源网络/认证失败吞成“无冲突”nil error，用户可能继续编辑他人锁定的资产 |
| P2 测试 | CheckRemoteLocksConflict / UnlockStaleLfsLocks | 安全关键路径 0% 覆盖 |
| P3（发现，延后） | UnlockStaleLfsLocks | 仅空白路径列表通过校验，返回“没有成功破除任何锁”而非参数错误 |
| P3（发现，延后） | LockAllModifiedLfs / SyncLockableFilePermissions | 仍丢弃 `GetLfsLocks` 错误 |
| P2（发现，延后） | CheckRemoteLocksConflict | 只扫描 staged/unstaged，不含 untracked |
| P2（延后） | 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件、`branch.days.*` | 继续排队 |

### 并行任务（4 个 + 1 个补派，文件不冲突）

1. `demangleSwift` 改为按 rune 长度前缀消费，删除旧正则；22 条测试覆盖真实符号、过滤、截断/溢出、Unicode，覆盖率 100%。
2. Git Notes 改为 8 worker 有界池，预分配切片按索引写回，顺序、limit、失败回退语义不变；新增 12 条批注、limit、空仓库测试，race 通过。
3. `PruneSingleWorktree` 捕获 `RemoveAll` 与 prune 错误并按组合返回；新增只读父目录回归测试（root/Windows 跳过）与可写父目录控制用例。
4. LFS 锁安全测试：伪 git 覆盖冲突标记/降级与批量解锁成功/部分失败/无 git-lfs，两函数从 0% 提升；发现锁源失败被吞。
5. 补派：`CheckRemoteLocksConflict` 锁源失败改为返回 `检查远端 LFS 锁失败`（前端失败 Toast 早已存在），测试先红后绿。

### 验证

- Go：`go vet`；`go test -cover` 441 测试 / 77.8%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试 / 39 文件、生产构建通过（本轮无前端改动）。

### 遗留

- `demangleSwift` 仍是启发式名称扁平化，不处理泛型/替换标记，调用前依赖 `$s` 守卫。
- `CheckRemoteLocksConflict` 不含 untracked；`LockAllModifiedLfs` / `SyncLockableFilePermissions` 仍吞锁源错误；`UnlockStaleLfsLocks` 空白路径应报参数错误。
- 9 个未接线 RPC 的取舍；i18n chunk、体积函数、超大文件、`branch.days.*` 清理继续排队。

## 第 11 轮 — 真实性 + 安全边界 + 性能 + 测试

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P1 安全/健壮性 | git_studio_engine.go parseTTFCmap | `cmapOffset+cmapLength` uint32 溢出绕过边界检查导致切片 panic；format 12 `endChar=0xFFFFFFFF` 使循环回绕死循环 |
| P1 真实性 | git_lfs_lock.go LockAllModifiedLfs / SyncLockableFilePermissions | 锁源查询失败被吞成“无锁”，批量加锁可能抢他人锁、只读保护按错误清单同步 |
| P2 安全 | git_lfs_lock.go CheckRemoteLocksConflict | 只扫描 staged/unstaged，未跟踪资产在 `git add` 后才发现被他人锁定 |
| P2 真实性 | git_engine_asset.go LaunchGameEngine | `Start()` 错误全部忽略；Linux 不做任何事仍报“已启动” |
| P2 边界 | git_closed_loop.go UnlockStaleLfsLocks | 全空白路径列表通过校验，返回“没有成功破除任何锁” |
| P2 性能 | git_service.go GetCommitDetailsBatch | 每个 hash 串行 `git show`，50 条选择即 50 个串行进程 |
| P2 测试 | parseTTFCmap / CheckRemoteLocksConflict / UnlockStaleLfsLocks | 0% 覆盖 |
| P2（延后） | parseTTFCmap format 4 | 忽略 idDelta/idRangeOffset，区间内所有码点都被当成有字形，字形覆盖率高估 |
| P2（延后） | 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件、`branch.days.*` | 继续排队 |

### 并行任务（4 个 + 1 个补派，文件不冲突）

1. LFS 锁真实性：冲突扫描纳入 untracked；`LockAllModifiedLfs` / `SyncLockableFilePermissions` 传播锁源错误；全空白路径提前报错；伪 git 测试覆盖四类失败。
2. 引擎启动：改用 `cmd.Run()` 并传播启动失败；非 mac/win 返回“不支持”；测试覆盖未知引擎、缺失 `.uproject`、darwin 缺失 `project.godot`。
3. 批量提交详情：8 worker 有界池、预分配按索引写回；12 提交乱序 + 空/伪造 hash 测试，逐项断言顺序与字段，race 通过。
4. TTF cmap 测试：合成 format 4/12 字体与 9 类畸形输入，覆盖率 98.4%；发现两个 P1。
5. 补派加固：边界判断改 uint64；format 12 跳过 `start>end`、钳制 `end` 到 `0x10FFFF`；删除不可达分支；新增溢出 panic 与回绕死循环回归（先红后绿），覆盖率 100%。

### 验证

- Go：`go vet`；451 测试 / 78.3%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试 / 39 文件、生产构建通过（本轮无前端改动）。

### 遗留

- `parseTTFCmap` format 4 仍需按 idDelta/idRangeOffset/glyphIdArray 判断真实字形，避免覆盖率虚高。
- 批量提交详情对所有 hash 失败仍静默跳过（API 无错误通道）；后续可评估状态警告。
- 9 个未接线 RPC 的取舍；i18n chunk、体积函数、超大文件、`branch.days.*` 清理继续排队。

## 第 12 轮 — 边界/内存 + 正确性 + 性能

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 边界/内存 | git_project_browser.go:520-590 | `GetSourceFileDetail` 的 revision 读取与 HEAD fallback 无大小上限，整块 blob 读入内存，GB 级文件可 OOM |
| P2 正确性 | git_studio_engine.go parseTTFCmap | format 4 忽略 idDelta/idRangeOffset/glyphIdArray，区间内所有码点被视为有字形 |
| P2 性能 | git_service.go ScanHistoryLargeBlobs | 最多 100 个大对象逐条串行 `git log -1` |
| P3（发现，延后） | git_service.go GetFileContentAtCommit | 版本内容读取仍无大小上限 |
| P2（延后） | 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件、`branch.days.*` | 继续排队 |

### 并行任务（3 个，文件不冲突）

1. `GetSourceFileDetail`：新增 5MB/30MB 常量与 `gitBlobSize`；revision 文本/图片与 HEAD fallback 先查大小，超限返回真实 `Size` + `IsBinary` + 空内容；小文件改用 `ExecGitRaw` 保留原始首尾空白，`countBlobLines` 保持行数语义；6MB HEAD/worktree/fallback 与 2 行小文件测试。
2. `parseTTFCmap` format 4：按真实布局读取 endCode/startCode/idDelta/idRangeOffset/glyphIdArray；idRangeOffset 相对自身表项寻址、0 视为未映射、非 0 应用 idDelta；越界/截断安全跳过；新增 7 组用例，覆盖率 100%。
3. `ScanHistoryLargeBlobs`：8 worker 有界池、预分配按索引写回；12 个大对象测试验证降序、路径一一对应、提交元数据不串槽，race 通过。

### 验证

- Go：`go vet`；456 测试 / 78.3%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试 / 39 文件、生产构建通过（本轮无前端改动）。

### 遗留

- `GetFileContentAtCommit` 仍无大小上限（P3），需与调用方共同评估。
- revision 大小查询失败现在直接返回错误（不再回退整块 `git show`），需确认 UI 错误提示可接受。
- 9 个未接线 RPC 的取舍；i18n chunk、体积函数、超大文件、`branch.days.*` 清理继续排队。

## 第 13 轮 — 内存边界 + 性能 + 安全

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 边界/内存 | git_project_browser.go GetSourceFileDiff | `git diff`/`git show --patch` 全量缓冲后才截断，大 blob 可 OOM |
| P2 性能 | git_studio_engine.go CherryPickAssetBundle | 每个 `.meta` 串行 `git show`，大工程先等完整 GUID 扫描 |
| P2 性能 | git_service.go BatchDiscardFiles | 每文件一次 `ls-files`，500 文件即 500 个进程 |
| P2 安全 | git_asset_diff.go execGitBytes | 错误通道返回原始 stderr，绕过统一脱敏 |
| P3（本轮修正） | git_service.go BatchDiscardFiles | 单次 `ls-files` 分类会把“含跟踪文件的目录”误判为 untracked（优化引入的语义回归，已补 trackedDirs） |
| P2（延后） | 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件、`branch.days.*` | 继续排队 |

### 并行任务（4 个，文件不冲突）

1. `GetSourceFileDiff`：先查工作区/HEAD/revision/父提交 blob 大小，超 5MB 返回明确错误；覆盖大文件工作区、HEAD、父提交、未跟踪与 2 行小文件回归。
2. `CherryPickAssetBundle`：8 worker 扫描 `.meta`，按索引合并保持 last-wins；GUID 依赖 BFS 不变；MatA→TexB 依赖、`resolveDependencies=false`、40 个噪声文件测试，race 通过。
3. `BatchDiscardFiles`：单次 `ls-files -z` 分类，并用 trackedDirs 保留目录输入的 restore 语义；混合跟踪/未跟踪/空格路径/目录/空输入测试。
4. `execGitBytes`：stderr 经 `redactLogSecrets` 后返回；NUL 与首尾空白的二进制 stdout 保持不变；先红后绿。

### 验证

- Go：`go vet`；467 测试 / 78.4%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试 / 39 文件、生产构建通过（本轮无前端改动）。

### 遗留

- 大文件差异现在返回错误而非截断补丁，需确认 UI 对错误文案的展示。
- `execGitBytes` 脱敏会同时遮蔽合法的远端诊断信息（策略一致）。
- `GetFileContentAtCommit` 仍无大小上限。
- 9 个未接线 RPC 的取舍；i18n chunk、体积函数、超大文件、`branch.days.*` 清理继续排队。

## 第 14 轮 — 真实性 + 性能 + 内存边界

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 真实性 | git_review.go GetChangeReview | `git log` 失败被吞，UI 把失败显示成“该范围没有提交记录” |
| P2 性能 | git_studio_engine.go AuditPrePushChecklist | Rule 2/3 逐提交串行 `diff-tree`/`cat-file`/`ls-tree` |
| P3 边界/内存 | git_service.go GetFileContentAtCommit | 版本内容无上限，GB 级 blob 可 OOM |
| P2 性能 | git_lfs_lock.go LockAllModifiedLfs | 每候选文件串行 `check-attr`，500 文件即 500 进程 |
| P2（延后） | 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件、`branch.days.*` | 继续排队 |

### 并行任务（4 个，文件不冲突）

1. 评审警示：`ChangeReview` 新增 `Warning/WarningKey/WarningParams`；日志失败保留文件与差异并返回 `review.warning.commits.unavailable`；ChangeList 显示 amber `role=status`（折叠时仍可见），zh/en key 对齐；伪 git `log --no-merges` 失败测试。
2. 推送前策略审计：8 worker 逐提交并行 Rule 2/3，每槽独立结果，按提交顺序合并（先全部 Rule 2、再全部 Rule 3），首个错误按提交顺序返回；12 提交 + 缺失 LFS 指针测试，两次运行结果逐字节一致，race 通过。
3. `GetFileContentAtCommit`：先 `gitBlobSize` 查 5MB 上限，超限返回“跳过读取”；6MB、小文件与缺失路径测试。
4. LFS 批量加锁：8 worker 预计算 `check-attr`，加锁仍按原顺序串行；20 个 `.uasset` + 普通文件测试、单文件失败变体，锁调用次数与结果顺序断言，race 通过。

### 验证

- Go：`go vet`；475 测试 / 78.6%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试 / 39 文件、生产构建通过；i18n key 对齐。

### 遗留

- 评审警示只在详情视图展示，会话列表/导出未携带（两者不宣称空范围）。
- 策略审计提交内部的文件扫描仍串行，超大单提交仍可能慢。
- 9 个未接线 RPC 的取舍；i18n chunk、体积函数、超大文件、`branch.days.*` 清理继续排队。

## 第 15 轮 — 性能 + 真实性 + 内存 + 文档

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 性能 | git_service.go GetSubmodules | 每个子模块 1–2 次 `git config`，30 个子模块即 60 个进程 |
| P2 真实性 | git_lfs_lock.go SyncLockableFilePermissions | `chmod` 与 `filepath.Walk` 错误被吞，计数虚报；不存在仓库返回零计数“成功” |
| P2 内存/性能 | git_studio_engine.go AuditNativePluginsAbi | 每个插件整文件 `ReadFile`，只为读 4 字节 magic |
| P3 文档 | README / architecture-audit | 实际 i18n 已达 4,246，文档仍写 4,245 |
| P2（延后） | 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件、`branch.days.*` | 继续排队 |

### 并行任务（3 个，文件不冲突）

1. 子模块配置：`.gitmodules` 与本地配置各一次 `--get-regexp` 预取 url/path，按 name→path→url 解析；状态/顺序/缺失 URL 语义不变；两个本地 bare 远端子模块与无 `.gitmodules` 测试。
2. 只读保护真实性：先 `os.Stat` 校验仓库目录，捕获 `filepath.Walk` 错误；`chmod` 失败计入新增 `FailedCount/FailedFiles` 且不计入只读/可写；darwin 用 `chflags uchg` 实测失败路径，types.ts 同步字段。
3. 插件头读取：新增 `readFileHeader`（16 字节、短读容错），替换整文件 `ReadFile`；5MB 假 PE/未知 magic 集成测试与短文件/缺失文件单测。
4. 文档：i18n 基线修正为 4,246（README 与 architecture-audit）。

### 验证

- Go：`go vet`；482 测试 / 78.7%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试 / 39 文件、生产构建通过（types.ts 字段同步）。

### 遗留

- `filepath.Walk` 的子目录读取错误仍按原语义跳过，仅目录根与 Walk 自身错误致命。
- 子模块 name 与 path 重复时 URL 解析可能不确定（非现实 `.gitmodules` 形态）。
- 9 个未接线 RPC 的取舍；i18n chunk、体积函数、超大文件、`branch.days.*` 清理继续排队。

## 第 16 轮 — 安全 + 性能 + 真实性

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 安全 | git_lfs_lock.go SyncLockableFilePermissions | `strings.Contains(owner, userName)` 把 “Alice” 当成 “Al” 的锁，可能错误放开只读保护 |
| P2 性能/真实性 | git_service.go ApplyIgnoreSanitizer | 逐文件 `git rm --cached`，失败静默丢弃，UI 只报成功 |
| P2 真实性 | git_engine_asset.go CheckUnityMetaSync | `ls-files` 失败返回空结果/nil，误报“无元数据问题” |
| P3（发现，延后） | git_closed_loop.go | 忽略 `action.Success`，部分失败仍按成功格式输出 |
| P3（延后） | 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件、`branch.days.*` | 继续排队 |

### 并行任务（3 个，文件不冲突）

1. LFS 锁归属：读取 name/email，仅 `EqualFold` 精确匹配；`Al` 对 `Alice` 保持只读，大小写与邮箱匹配放开可写；现有 `me` 用例保持。
2. 忽略修复：批量 `git rm --cached`；失败后用 `ls-files` 复核实际剩余跟踪文件，区分成功与失败；新增 `FailedFiles`、`Success=false`、部分失败 Toast 与 zh/en key；成功/批量失败测试。
3. Unity 元数据：已跟踪集合与 Assets/Packages 枚举失败改为返回错误，不再空结果；伪 git 失败测试 + 真实仓库/空仓库控制组。

### 验证

- Go：`go vet`；489 测试 / 78.8%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试 / 39 文件、生产构建通过；i18n 4,247×2 对齐。

### 遗留

- `git_closed_loop.go` 仍未消费 `action.Success`，部分失败会按成功格式汇总。
- 生成物 `frontend/bindings/wgit/models.ts` 未重新生成（下次 Wails 构建同步 `failedFiles`）。
- 单次批量 `git rm` 在极端数量下可能触及 ARG_MAX，暂未分块。
- 9 个未接线 RPC 的取舍；i18n chunk、体积函数、超大文件、`branch.days.*` 清理继续排队。

## 第 17 轮 — 真实性 + 性能（批量操作）

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 真实性 | git_closed_loop.go RunHealthFix / git_engine_bootstrap.go | 第 16 轮的部分失败未被消费，仍按成功/applied 汇总 |
| P2 性能 | git_service.go DeleteBatchTags | 每标签一次 `tag -d`，远端每标签一次 `push`，N 个标签 N 次网络往返 |
| P2 性能 | git_engine_asset.go AutoFixUnityMeta | 每个未配对文件一次 `git add`（各自抢 index.lock） |
| backlog | 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件、`branch.days.*` | 继续排队 |

### 并行任务（3 个，文件不冲突）

1. 调用方真实性：`RunHealthFix` 使用 `action.Success` 并在 Output/Hint 报失败数；`RunEngineBootstrap` 仅在成功时标 applied，否则 failed 并带首个失败上下文；伪 git `rm --cached` 失败测试覆盖两处。
2. 标签批量删除：`tag --list` 一次建集合，单次 `tag -d` 删除全部存在标签，失败后重列判定幸存者；远端单次 `push --delete`，失败按标签聚合；去重、混合失败、bare 远端单次 push 测试。
3. Unity 自动配对：收集去重后单次 `git add`；失败后用 `diff --cached --name-only -z` 复核实际已暂存路径；10 个缺口断言仅 1 次 add，失败路径 fixed 为空并报聚合错误；同步调整现有失败测试对“调用前已暂存”路径的断言。

### 验证

- Go：`go vet`；496 测试 / 78.8%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试 / 39 文件、生产构建通过（本轮无前端改动）。

### 遗留

- 标签名校验仍逐标签执行本地 `check-ref-format`（允许）。
- 远端批量删除失败无法逐标签归因，统一按受影响标签聚合。
- AutoFix 失败校验可能把“调用前已暂存”的路径计入 fixed（其确实已暂存）。
- 9 个未接线 RPC 的取舍；i18n chunk、体积函数、超大文件、`branch.days.*` 清理继续排队。

## 第 18 轮 — 一致性 + 导出真实性 + 清理

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P3 一致性 | git_lfs_lock.go LockAllModifiedLfs / CheckRemoteLocksConflict | 锁归属仅大小写敏感地比较 name，与 SyncLockableFilePermissions 的 name/email EqualFold 不一致 |
| P3 真实性 | git_review.go ExportChangeReviewMarkdown | 导出遗漏 `Warning`，接收者不知道提交列表不完整 |
| P3 可维护性 | i18n `branch.days.*` | 5 个 key × 2 语言无任何引用 |
| backlog | 9 个未接线 RPC、i18n chunk、三套体积函数、超大文件 | 继续排队 |

### 并行任务（2 个 + 1 个手工清理）

1. 评审导出：`Warning` 非空时在统计行后写入 `> ⚠️ ...` blockquote；日志失败与正常仓库两类导出测试。
2. LFS 归属：新增 `lockOwnerIsCurrentUser`（trim、空 owner 非本人、name/email EqualFold），LockAllModifiedLfs / CheckRemoteLocksConflict / Sync 三处统一；大小写与邮箱匹配测试。
3. 清理 `branch.days.today/one/n/months/years` 共 10 条死 key，README 与 architecture-audit 同步为 4,242×2。

### 验证

- Go：`go vet`；500 测试 / 78.8%；`-race -shuffle` 通过；server + desktop 构建通过。
- 前端：`vue-tsc`、610 测试 / 39 文件、生产构建通过；i18n 4,242×2 对齐。

### 遗留

- LFS 身份读取仍是 best-effort，config 读取失败时无法识别本人锁（保守按他人处理）。
- 评审警示未进入会话列表摘要（详情与 Markdown 导出已覆盖）。
- 9 个未接线 RPC 的取舍；i18n chunk、体积函数、超大文件继续排队。

## 第 19 轮 — 最终审计与全量回归

### 审计清单

| 维度 | 结果 |
| --- | --- |
| 文档基线 | 实测 500 Go / 78.8% / 610 前端 / 39 文件 / i18n 4,242×2 / RPC 388 / facade 388 / UI 379（9 个未接线），与 README 和 architecture-audit 一致；生成绑定已含 `failedFiles` |
| 安全 | 全仓非测试源码未检出真实 `ghp_` / `glpat-` / `AKIA` 令牌；非测试 Go 0 个 TODO/FIXME |
| 工作树 | 102 项改动（含并行会话既有的 branding/subtree 等），本轮未提交、未推送 |
| 回归 | `go vet`；`go test -cover` 500 测试 / 78.8%；`-race -shuffle` 通过；server + desktop 构建通过；`vue-tsc`、610 测试、生产构建通过 |
| 结论 | 无新增 P0/P1，未凑改动；已知 P2 待办留作第 20 轮交付的后续建议 |

### 遗留（转入最终报告）

- 9 个未接线 RPC 的取舍。
- i18n chunk 564KB / 首屏分包。
- 三套体积格式化函数与超大文件拆分。
- 评审警示未进入会话列表摘要。
- LFS 身份读取 best-effort 边界。

## 第 20 轮 — 交付验收

### 完成审计

| 检查项 | 证据 |
| --- | --- |
| 20 轮完整 | 本文件第 1–20 轮均含问题清单、并行任务、验证与遗留 |
| 分级修复 | P0 = 0；11 行 P1 全部修复；P2/P3 已登记，剩余项转入后续建议 |
| 最终门禁 | Go 500 测试 / 78.8%；`-race -shuffle`；server + desktop 构建；前端 610 / 39；`vue-tsc`；生产构建；i18n 4,242×2 |
| 交付物 | [optimization-final-report.md](optimization-final-report.md)（完整改动清单、分级统计、验证证据、后续建议）；README/architecture-audit 基线最终同步 |
| 安全 | 非测试源码无真实令牌；非测试 Go 0 个 TODO/FIXME |
| 仓库状态 | 未提交、未推送；工作树保留 102 项改动（含并行会话既有改动） |

### 结论

20 轮优化目标完成。后续按最终报告第 7 节推进：9 个未接线 RPC 取舍、i18n chunk 拆包、体积函数统一、超大文件拆分，以及评审摘要/LFS 身份/ARG_MAX 等边界项。

## 20 轮后跟进 — RPC 接线（2026-10-27）

- `RemediateStagedSecrets` 接入 CleanTab 暂存区密钥扫描：新增「移出暂存区」按钮，确认后批量取消暂存并写入 `.gitignore`，静默重扫刷新；失败保留旧结果并显示后端错误。
- `MoveWorktree` 接入 WorktreesTab：非主/未锁定/非当前工作树显示移动按钮，弹窗填写不存在的新目录，确认后调用 `git worktree move` 并刷新列表。
- 新增 `frontend/tests/cleanSecretsRemediation.test.ts`、`frontend/tests/worktreeMove.test.ts`（组件接线 + 双语 key）；前端 610 → 614 测试 / 39 → 41 文件，i18n 4,242 → 4,259。
- UI 实际引用 379 → 381；未接线 RPC 9 → 7（4 个兼容别名 + 3 个独特能力：SplitSubtree / UpdateSubtreeMetadata / UnlinkSubtree）。
- 验证：`vue-tsc`、`bun test tests`（614/0）、`bun run build` 通过；未做浏览器 E2E（需真实暂存密钥仓库与工作树）。

---

# 第二轮战役（2026-10-29 起）

> 首轮 20 轮已于 2026-10-27 交付（上文第 1–20 轮）。本战役在同一约束下续做，轮次编号独立（R2·N），避免与历史记录混淆。

## R2·第 1 轮 — 错误处理收尾 + 命令注册表（阶段 0 起步）

> 基线漂移说明：本轮开始时并行会话将 HEAD 由 `dfaf6b6` 推进到 `32d5433`（首轮战役后段提交入库），前端测试 597 → 973（86 文件）。本轮基线以 `32d5433` 为准。首轮遗留的 100k 图基准因 `-short` 跳过从未全量执行，本轮首次全量运行即暴露两个真实 P1。

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P1 | git_service.go:4052（旧 :3816） | GetCommitDetailsBatch 单 hash `git show` 失败被 `continue` 静默跳过，调用方无法感知丢项 |
| P1 | git_studio_ops.go:61 | SSH 诊断超时被击穿：ssh 为脚本包装时 CommandContext 只杀直接子进程，孙进程持有输出管道致 CombinedOutput 阻塞至其退出（全量测试暴露，见验证） |
| P2 | git_subtree.go:123 | ListSubtrees 的 `git log` 失败静默返回空列表，无法区分「无子树」与「读不了仓库」 |
| P2 | git_service.go:203 | GetWorkingDiffAdvanced 未跟踪文件 no-index diff 失败时静默返回空串 |
| P2 | frontend types/gitService | SubtreeUpdateParams / SubtreeSplitParams 死类型（仅 import 出现，零使用） |
| P2 | frontend CommandPalette | 命令面板已接 registry 但 executeCommand 仍引用未声明字段（中断任务半成品），且缺 `tools.tab.lfs`、`cmd.cat.tools/help` 等 key |

### 问题清单（续）

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P1 | git_graph.go:26（BuildCommitGraph） | 车道泄漏 → O(n·lanes) 连线爆炸：first/second parent 指向已处理过的哈希（浅历史、截断日志、异常父序）时车道永不回收；100k 基准实测挂死（≈10¹⁰ 次连线），全量套件两次被杀的直接原因 |
| P2 | format_util.go | 缺 TB/PB 档位，≥1TiB 显示为 "1024.00 GB" |

### 并行任务（3 个；2 个因配额超限中断，由主线收尾）

1. ✅（agent 完成）ListSubtrees 错误传播 + 伪 git 回归测试（2 个新测试，含「无子树不误报」回归保护）。
2. ⚠️（agent 落盘后配额中断，主线验收+补漏）GetCommitDetailsBatch 增加 `WithStatus` 变体（worker pool 保序、Failed hash + 原因上报）；GetWorkingDiffAdvanced no-index 失败返回占位说明；新增 git_service_batch_error_test.go（3 个测试 7 个子用例）；前端补 `getCommitDetailsBatchWithStatus` 包装与类型（RPC 平价契约测试把关，506=506）。
3. ⚠️（agent 落盘后配额中断，主线收尾）src/commands/registry.ts 全应用命令注册表（95 命令 / 16 分组，仅复用 emitAction 与 window 事件两条既有通道）；CommandPalette executeCommand 接 `run(ctx)`；新增 tests/commandsRegistry.test.ts（id 唯一性、分组/labelKey 双语存在性、run 可调用）；i18n 补 4 个 key。

### 改动

- Go：git_subtree.go、git_subtree_test.go、git_parser.go（+BatchResult/FailedCommitHash 类型）、git_service.go（WithStatus + 占位文本）、git_service_batch_error_test.go（新）、git_studio_ops.go（WaitDelay 修复）、git_graph.go（车道泄漏修复）、git_benchmark_test.go（合成数据改为真实日志序）、format_util.go + format_util_test.go（TB/PB 档位 + 用例）。
- 前端：src/commands/registry.ts（新）、CommandPalette.vue、services/gitService.ts、types.ts、i18n zh/en（+4 key）、tests/commandsRegistry.test.ts（新）、frontend/bindings/wgit/*（wails3 generate 再生成）。
- 死类型清理：types.ts、gitService.ts。

### 验证

- Go：`go vet` 通过；desktop 与 server（-tags server）构建通过；`go test -timeout 1500s -cover .` 全量通过（TestLargeScaleCommitGraph100k 挂死 → 1.11s；TestDiagnoseGitAuthHonorsSSHTimeout 5.06s → 0.15s，均红→绿验证）。
- 前端：`vue-tsc` 通过；973 测试全绿（含新增 commandsRegistry 与 RPC 平价契约）；生产构建通过。
- 本轮 P1（SSH 超时击穿）为全量回归暴露的真实产品 bug，已修复并附带回归路径说明。

### 遗留 / 下一轮候选

- Go 非测试代码仍有 92 处忽略 `ExecGit` 错误（含 4 处 `filepath.Walk` 吞错、GetSubmodules 静默空 URL），按用户可见度分轮清理。
- BranchesView.vue:188-189 漂移/重叠分析 `.catch(() => null)` 失败后面板空态无提示（与第 1 轮合并预检同类，展示型）。
- format_util.go 缺 TB 单位（≥1TiB 显示为 "1024.00 GB"）；已有测试，补 TB 用例即可。
- 命令注册表落地后，ToolsView 两级化（tier 1/2）与 ActivityDock 可点击结果排队下一轮。

## R2·第 2 轮 — 可维护性 + 吞错批次 1 + 工具入口两级化

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 | git_service.go:2425/2592/4593/4621 | 4 处 `filepath.Walk` 吞错，磁盘/维护统计静默少算 |
| P2 | git_service.go:1312（GetSubmodules） | submodule URL 读取失败静默显示为空 |
| P2 | BranchesView.vue:186-199 | 漂移/重叠分析 `.catch(() => null)`，失败后空态无解释 |
| P2（新发现） | BranchesView.vue | `showDriftModal`/`driftReport`/`driftMatrixReport` 为死状态——分析有入口、有重试，但结果面板从未渲染（历史 Phase 58 只落了头部按钮） |
| P2 | ToolsView.vue | 14 个子 Tab 一字排开无层级，LFS/Worktree/Engine 与低频工具平铺（「入口深」主源） |

### 并行任务（3 个：2 agent + 1 主线）

1. ✅（agent）git_service.go 吞错批次 1：4 处 Walk 错误计数并入 Warnings/Summary/Recommendations（文案随既有 wire 语言）、RunMaintenance 重建 Warnings 时保留测量警告、GetSubmodules 不可解析 URL 置 `(unreadable)`；新增 git_service_walk_errors_test.go（4 测试，含 root 跳过逻辑）。
2. ✅（agent）BranchesView 漂移失败可见化：`driftError` + allSettled + 复用 describePreviewError 范式 + `role=alert` 警告横幅 + 重试按钮 + repoEpoch 竞态守卫 + `branch.drift.analysis.failed` 双语 key + 4 测试断言。
3. ✅（主线）ToolsView 两级化：数据驱动 `PRIMARY_TABS`(5：终端/Reflog/清理/工作树/LFS) + `MORE_TABS`(9) + 「更多 ▾」下拉（overlay 关闭、激活态回显、计数徽标保留、badgeClass 随项）；`tools.more` 双语 key；tests/toolsTwoTier.test.ts（4 组断言，含「模板不再有 14 个直赋按钮」防回归）。

### 验证

- Go：`go vet`、desktop/server 构建通过；`go test -timeout 1500s -cover .` → `ok 107.2s / 77.7%`。
- 前端：`vue-tsc`、981 测试（+8）、生产构建通过。

### 遗留 / 决议

- **决议：否决 i18n 按域拆分**。原 R1 战役遗留的「i18n chunk 偏大」经复核为低价值：桌面产物经 go:embed 本地加载（zh 308KB / gzip 93KB），仅 server 模式走网络且非主场景；拆分扰动 4,259 key 与 40+ 测试文件，性价比不足，不凑改动。
- BranchesView drift 死状态（面板缺失）升级为 R2·第 3 轮调查项：补渲染最小结果面板或移除死状态，二选一。
- 吞错 backlog：92 → 87 处（本轮 +4 Walk +1 Submodules）；批次 2 需先审计分级。


## R2·第 3 轮 — 死状态处置 + 健康徽标（F4）+ Feature Flag 基建（§3.2/3.3）

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P1（功能缺口） | BranchesView.vue | 漂移分析「从落地起就没有结果渲染」：driftReport/driftMatrixReport/showDriftModal 被计算但零模板引用（git 253b23e 证实为半成品，非历史删失）；用户点入口后无任何可见结果 |
| P2 | RepoRail.vue | 多仓切换无健康可见性，坏仓库要逐个点开才知道（F4 依据：git_health 数据现成） |
| P2（地基缺失） | config.go / 前端 | 功能无 flag 机制，后续 AI/评审/资产灰度无挂点 |

### 并行任务（3 个 agent，文件不冲突）

1. ✅ 漂移结果面板补渲染：确认死状态成因后按最小方案 a 实现——healthScore 分级徽章（≥80/≥50 low/medium/high）、建议列表 ≤5、重叠矩阵摘要（前 5 对 + 风险级）；删除纯死状态 showDriftModal；+20 个 `branch.drift.panel.*` 双语 key；扩展 branchManagementUx 测试。
2. ✅ RepoRail 健康徽标：后端仅 `RunRepoHealthCheck`（6 项诊断、成本高）→ 懒加载（400ms 防抖 + 逐个 drain 队列 + 组件级 Map 缓存 + 失败静默 + 卸载清理）；绿/黄/红三档圆点 + 复用既有 `health.summary.*` key 做 tooltip（零新增 i18n）；点击跳 maintenance/health 区并选中该仓库；新增 repoRailHealth.test.ts（9 测试）。
3. ✅ Feature Flag 基建：config.go 增 FeatureFlags(5 bool) + SchemaVersion=1 向后兼容（旧文件内存补版、不回写）；新增 `GetAppFlags` RPC（复查全仓无 GetConfig，且与 RPC 平价契约测试对称 +1）；前端 useFeatureFlags（单例 Promise、失败可重试、幂等 ensureFlagsLoaded）+ FeatureGate 组件（未知 key 恒 disabled 不抛错）+ 双侧 9+9 测试；未改 App.vue。

### 验证

- Go：`go vet`、desktop/server 构建通过；全量 `ok 101.4s / 77.7%`。
- 前端：`vue-tsc`、1002 测试（981 → 1002，+21）、生产构建通过。

### 遗留

- F1/F2 等 AI 功能挂到 flag（aiCommitMsg 等）在阶段 1 接线；flag 当前默认全 false。
- RepoRail 徽标与漂移面板均为源码断言测试，未做浏览器 E2E（沿用既有豁免口径）。


## R2·第 4 轮 — 吞错批次 2（分诊制）+ AiService 骨架（阶段 1 起步）

### 问题清单与分诊结果

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 | git_lfs_lock.go（13 处） | 12 处为「未配置身份/未设 key」合法状态（exit 1 常态），传播会造成回归——全部保留并补 fail-safe 注释；**1 处传播**：CheckStagedNonLfsLargeFiles 的 check-attr 失败会被误判为「非 LFS」→ 对 LFS 保护的资产报假警报（推送前大文件闸口） |
| P2 | git_external_tools.go（9 处）/ git_rebase_kanban.go（7 处） | 传播 5 处：GetExternalToolConfig 增加 rev-parse 守卫（仓库损坏不再伪装成配置成功）、OpenExternalDiff/Merge 兜底检测错误折叠进最终错误、PreviewRebaseTopology 文件读取失败不再伪装低风险、SplitCommitSurgically 5 处 staging/记录失败改为 Success=false；其余 11 处展示型保留+注释 |
| P1（范围外发现，下轮修） | git_lfs_lock.go VerifyPrePushLfs | `git log` 失败被 `err == nil` 守卫吞掉，把「读不了仓库」呈现为「与远端完全同步」——推送前检查失真 |
| P2（地基） | 前端 AI 通道 | localAiService 在渲染进程直连端点：密钥暴露、无脱敏、无审计（方案 §3.5） |

### 并行任务（3 个 agent）

1. ✅ LFS 锁吞错分诊（上表）：4 新测试（含 2 个「保留案例行为不变」锁定）。
2. ✅ 外部工具 + rebase 看板分诊：4 新测试；13 个相关既有测试全过。
3. ✅ AiService Go 骨架：main.go 注册第二 Service；`GetAIConfig`（脱敏：HasAPIKey/KeyFingerprint，绝不回明文）；`SaveAIConfig`（空 apiKey=保留）；`TestAIConnection`（10s 超时）；`complete`（OpenAI 兼容、出站 `redactAIPayload` 复用 redactLogSecrets + sk-/Bearer 模式、错误文本同脱敏、审计走 recordGitCommand 模式不含 prompt/key）。**vault 选型**：既有 vault 绑定 repoPath，无法承载全局 key → 明文仅存进程内存 + config.json 落 SHA-256 前 8 位指纹，Notice 字段向用户说明；`wails3 generate bindings -clean=true -ts -i` 再生成（2 Services / 510 Methods，unexported complete 正确未出现）。

### 验证

- Go：`go vet`、desktop/server 构建通过；全量 `ok 102.8s / 77.7%`。
- 前端：`vue-tsc`、1002 测试、生产构建通过（bindings 再生成后契约测试对称 +1）。

### 遗留

- VerifyPrePushLfs 失真 → R2·5 首任务。
- AI 明文 key 不落盘的限制（vault 全局化）→ 记入最终报告的后续建议。
- F1/F2 尚未接线（下一轮，挂 aiCommitMsg flag）。


## R2·第 5 轮 — 推送预检失真（P1）+ F1 AI 提交信息生成 + 吞错批次 3

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P1 | git_lfs_lock.go VerifyPrePushLfs | 4 处「检查失败 → 呈现肯定结论」：git log 失败→「与远端完全同步」；show/ls-tree 失败→「就绪可安全推送」（指针/大文件检查静默缺失）；rev-parse 失败→猜 "main" |
| P1（批次 3 实锤） | git_flight_recorder.go | manifest 读/写失败静默（快照条目全部丢失仍报成功）、临时 index 复制失败（快照静默缺文件）、unborn HEAD 垃圾值 "HEAD" 流入 commit-tree（设计的 root-commit 路径成死代码） |
| P2 | git_three_way_merge.go / git_studio_ops.go | 6 处分诊为设计内降级（stage 缺失常态、未配置 helper、unborn HEAD 友好降级），保留+注释 |
| P2（F1 落地） | CommitComposer.vue | 提交信息无 AI 辅助（方案 F1；aiCommitMsg flag 灰度） |

### 并行任务（3 个 agent）

1. ✅ VerifyPrePushLfs 失真修复：4 分支全部改为传播错误（调用方 LfsTab 已有 catch→toast）；新增 `isPathAbsentAtRevError` 区分「文件本不在 HEAD（合法跳过）」与真读仓失败；**变异验证**：临时还原旧守卫 → 新测试 FAIL → 还原修复 → PASS；5 新测试。
2. ✅ F1 端到端：Go `GenerateCommitMessage`（flag 门禁→repo 防御→空 diff 报错→200KB UTF-8 安全截断→防注入 prompt 封闭→出站脱敏→围栏剥离取 subject）；前端 FeatureGate 包裹的 Sparkles 按钮 + `isAIConfigured` 探测（未配置走引导不报错）+ 非空 subject 覆盖前 confirmAction + loading 态；RPC 走 `Call.ByName("main.AiService.…")`（callBackend 前缀硬编码 GitService，rpcCoverage 扫描器不动）；5 Go 测试 + 前端测试；bindings 2 Services/511 Methods。发现：i18n 契约测试强制全小写字面量，camelCase 语义 key 以 Object.defineProperty 别名兼容（沿用既有 workspace.composer.noVerify 模式）。
3. ✅ 吞错批次 3：10 处分诊（flight recorder 2 传播+1 设计修正+2 保留；three_way_merge 3 保留；studio_ops 3 保留）+ 特别审查补充 3 处非 ExecGit 实锤传播（manifest 读/写、临时 index）；8 新测试。

### 验证

- Go：`go vet`、desktop/server 构建通过；全量 `ok 104.0s / 77.7%`。
- 前端：`vue-tsc`、1016 测试、生产构建通过。

### 遗留

- F2 变更摘要（aiSummary flag）→ R2·6；AiConflictAdvisor 后端化（F5 第一步）→ R2·7。
- AI flag 默认全 false，需在设置页暴露开关（阶段 1 收尾项）。


## R2·第 6 轮 — git_service 吞错批次 4 + ActivityDock 可点击结果（§2.2-12）+ F2 AI 变更摘要

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 | git_service.go（23 处） | 其中 3 处直接影响数据完整/推送决策：DiscardFile/DiscardUntrackedFile 的 status 探测失败会把破坏性 clean 误判为 restore；getRepoInfo 远端列表失败让有远端的仓库显示为无远端；其余 20 处为「未配置 exit-1 常态」或展示型 |
| P2 | git_service.go GetCommitSigningStatus | 检查失败曾被呈现为「签名已关闭/0% 合规」的肯定性错误结论（仓库级 gate 修复） |
| P2 | ActivityDock.vue | 后台任务完成后无结果去向，用户不知道去哪看（方案 §2.2 第 12 条） |
| P2（F2） | CommitInspector.vue | 提交检查器无 AI 摘要（方案 F2；aiSummary flag） |

### 任务执行（3 个；2 agent 超时由主线收尾——超时原因：单任务体量过大触发 600s 失活上限）

1. ✅（agent）git_service.go 批次 4：**3 处传播**（两个 Discard* 的破坏性动作守卫、getRepoInfo 远端列表）+ **8 处仓库级 gate**（GetGitConfig/GetGitProxy/GetCommitSigningStatus：rev-parse --git-dir 失败浮出，per-key 吞错仅掩盖「未配置」exit-1 常态）+ 12 处保留+注释；7 新测试（含「探测失败后未执行破坏性动作」「未签名提交 verify-commit 保持 Status=N」等回归锁定）。
2. ✅（agent 落盘 + 主线收尾）ActivityDock 任务可点击：模块级任务仓（TASK_TARGETS 7 类映射 → 既有 deep-link 事件、MAX_TRACKED_TASKS/TTL 自动过期）、失败态无映射走既有 toast；**收尾 3 处**：TS2532 索引守卫、测试助手 scriptOf 在双 script SFC 中被模块块注释里的 `<script setup>` 字样带偏（改 lastIndexOf）、修复后 11/11 过。
3. ✅（agent 超时未落盘 → 主线全量重做）F2 端到端：Go `SummarizeCommit`（flag 门禁→空 hash 防御→message+stat+patch 组装（stat/patch 为 best-effort 富化，失败降级为按 message 摘要且注明）→150KB UTF-8 截断→防注入封闭→出站脱敏→剥围栏取 ≤3 行）；前端 aiSummaryRpcs（Call.ByName 全名路由）+ aiService 薄封装 + CommitInspector 的 FeatureGate 按钮/可折叠摘要卡/切提交清空/未配置 toast 引导；i18n +4 key（全小写字面量，无需别名）；3 Go 测试 + 5 前端测试；bindings 2 Services/512 Methods。

### 验证

- Go：`go vet`、desktop/server 构建通过；全量 `ok 111.2s / 77.8%`（覆盖率 77.7 → 77.8）。
- 前端：`vue-tsc`、1032 测试（+11 ActivityDock、+5 F2、批次 4 无前端）、生产构建通过。

### 遗留

- 吞错 backlog：git_service.go 23 处已清账（3+8+12），全仓非测试 ExecGit 吞错余约 45 处（多文件 ≤3 处的长尾）。
- AI 明文 key 的会话级限制需在设置页可视（阶段 1 收尾）；F5 冲突顾问后端化 → R2·7。


## R2·第 7 轮 — F5 冲突顾问后端化 + 吞错批次 5

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P1（安全/架构） | frontend/src/services/localAiService.ts | 冲突建议在渲染进程直连端点：密钥在 localStorage、无出站脱敏、无审计（方案 §3.5 待迁移项） |
| P1（批次 5 实锤） | git_closed_loop.go:110,172 | TrackStagedLargeFilesAsLfs：check-attr 探针失败 → 已跟踪文件被重跑整条转换链并误报 converted；转换后 size 读取失败 → newSize=0 绕过防线，未经校验的转换呈现为成功 |
| P2 | git_notes.go:104 等 | 分诊为诚实降级/exit-1 常态，保留+注释（notes/ref_compare/workspace 实际为 0 处 ExecGit 吞错） |

### 并行任务（2 个 agent）

1. ✅ F5 迁移：Go `GenerateConflictAdvice`（aiConflictBatch 门禁、ours/theirs 必填、120KB 截断（BASE 段垫底最先牺牲）、防注入封闭标记、出站脱敏+审计）；前端 requestConflictAdvice 改后端优先，签名/返回结构不变，启发式合成器保留为降级路径（confidence 85 vs 后端 90）；旧 localStorage key 迁移判断 + 复用 ai.notconfigured 一次性提示；`parseConflictAdvice` 导出 + `setConflictAdviceBackendForTest` seam；4 Go 测试 + 前端测试改写（原断言主题全保留）。
2. ✅ 批次 5：2 处传播修复 + 2 处保留注释；3 新测试（伪 git stub 代答 lfs 命令摆脱环境依赖；断言 `.gitattributes` 未产生证明写路径未执行）。

### 验证

- Go：`go vet`、desktop/server 构建通过；全量 `ok 117.0s / 77.8%`。
- 前端：`vue-tsc`、1035 测试（+3）、生产构建通过。

### 遗留

- 设置页尚未暴露 AI 配置与 flag 开关（R2·8）；明文 key 会话级限制需在该页可视。
- 吞错长尾余约 40 处（多文件 ≤3 处），批次 6 继续。


## R2·第 8 轮 — 设置页 AI/Flag 接线（阶段 1 收尾）+ 吞错批次 6

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2（地基悬空） | settings/LocalAiSection.vue | AI 配置仍写 localStorage（密钥暴露面），后端 SaveAIConfig/指纹/Notice 无 UI；5 个 flag 无开关，灰度机制无法操作 |
| P1（批次 6 实锤） | git_lfs_migrate.go:185 | 不可逆历史重写前 HEAD 读取失败被吞——无恢复锚点、无前后对照基线即"成功" |
| P1（批次 6 实锤） | git_stash_insights.go:75-76 | stash 第三父树读不出 → 未跟踪文件全部静默消失，不完整列表呈现为完整 |
| P2 | git_insights.go / stash numstat 等 4 处 | 分诊为「未配置 exit-1 常态/展示型诚实降级」，保留+注释 |

### 并行任务（2 个 agent）

1. ✅ 设置页接线：Go `SaveAppFlags`（Load→仅改 Flags→Save，round-trip + 不动其他字段测试）；LocalAiSection 双通道重写（后端为主：BaseURL/Model/Timeout/APIKey 表单、key 留空=保留、指纹+Notice 可视化、明文不回显；localStorage 降为迁移来源）；功能开关卡 5 toggle（乐观更新→失败回滚+danger toast→同步 useFeatureFlags 全局单例）；24 个双语 key；前端 1048 测试（+13）。
2. ✅ 批次 6：2 传播（迁移无基线即中止；stash ^3 树读取失败传播）+ 4 保留注释；4 新测试（断言 `lfs migrate` marker 不存在证明写路径未执行等）。

### 验证

- Go：`go vet`、desktop/server 构建通过；全量 `ok 108.1s / 77.8%`。
- 前端：`vue-tsc`、1048 测试、生产构建通过。

### 遗留

- 阶段 1（AI 三功能 + flag + 设置页）至此闭环；灰度开启由用户在设置页操作。
- 吞错长尾余约 36 处；前端 31 处空 catch 未分诊（R2·9）。


## R2·第 9 轮 — 前端吞错分诊批次 1 + 工作区变更列表折叠（§2.1-3）+ 中期 -race 回归

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2 | App.vue:219 refreshAllStatuses | getRepoStatus 失败被吞 → 仓库徽章假「干净」 |
| P2 | CommitComposer.vue:85/:318 | GPG 探测、lint/validate RPC 失败被吞 → 签名/lint 徽章静默消失读作「无问题」（无贴切 i18n key，按预案降级 console.error + 已标注态） |
| P2 | WorkspaceView 变更列表 | 已暂存组常占视线，无组级折叠（方案 §2.1-3） |
| P2（分诊保留） | 三文件其余 19 处 | localStorage/剪贴板/降级链路/best-effort，均诚实或有兜底——保留+注释 |

### 并行任务（2 个 agent）

1. ✅ 前端吞错批次 1：23 处分诊（4 修复 + 19 保留注释）；`app.toast.refresh.failed` 复用既有 key（双语已存在，零新增）；9 组测试断言（含「无新增 i18n key」守卫）。
2. ✅ ChangesList 分组折叠：持久化 `wgit.changes.groups.collapsed.v1`（损坏回退全展开）、v-if 折叠省 DOM、chevron 旋转、徽标计数保留且不自动展开（保守行为固化）；**发现并按实际适配**：上游把 untracked 并入 unstaged，实际三组为 conflicted/staged/unstaged；15 测试。

### 验证

- Go：中期回归 `go test -race -shuffle=on` 全量通过（245.7s，并发/洗牌序零竞争零失败）。
- 前端：`vue-tsc`、1072 测试（+24）、生产构建通过。

### 遗留

- 前端吞错余：SourceExplorer/ReviewView/CommandPalette/HistoryView/DiffViewer 等约 60 处待分诊（批次 2）。
- ReviewProvider（TAPD）与 AssetService（阶段 2/3）按排期推进。


## R2·第 10 轮 — 评审 Provider 契约正式化（阶段 2 起步）+ 前端吞错批次 2

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2（地基） | frontend/src/services/reviewSource.ts | 本地评审实现已存在但缺正式 Provider 契约与注册表；`createReviewSource` 未知 kind 静默落到 remote |
| P2 | SourceExplorer.vue:508 | 忽略规则列表读取失败被吞 → 已忽略路径显示为「未忽略」，菜单给出错误操作 |
| P2 | ReviewView.vue:121/:230 | LAN 状态探测、默认评审基线探测失败零可诊断性（静默回退 HEAD~20） |
| P2 | CommandPalette 7 处 | 全部为 localStorage 持久化，分诊保留+补原因注释 |

### 并行任务（2 个 agent）

1. ✅ Provider 契约：显式 `ReviewProvider`（形状逐字取自现 ReviewSource）+ `registerReviewProvider/getReviewProvider`（工厂制、repo 作用域、未知 kind→null 不抛错）；local/remote 注册为内置、行为零变化（ReviewView 无旁路，未改动）；5 组契约测试（含 tapd 注册演示 + 编译期赋值锁）。
2. ✅ 批次 2：21 处分诊（3 修复：gitignore 假空接线既有 `gitignore.error.load` key、LAN/默认基线可诊断化；18 保留注释）；7 测试。

### 验证

- Go：`go vet`、desktop/server 构建通过；全量 `ok 119.3s / 77.8%`。
- 前端：`vue-tsc`、1084 测试、生产构建通过。

### 遗留

- TAPD 外部适配器（数据源为 work-item 绑定而非评审列表，需产品口径）→ 排期后议；AssetService（阶段 3）→ R2·11。


## R2·第 11 轮 — AssetService 状态机（阶段 3 起步）+ 前端吞错批次 3

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2（地基） | 引擎资产 | 扫描/QC/锁数据分散在三个文件，无统一条目与审批状态；预算比无单资产口径 |
| P2 | HistoryView.vue:524/:807 | 拓扑统计失败徽章消失伪装「无合并提交」；硬重置前脏工作区警告静默消失 |
| P2 | DiffViewer.vue:780 | blame 失败 → 空 gutter 误读「该文件无历史」（修复走既有 reportError 危险 toast） |
| P2 | DiffViewer:257 / TerminalTab:126 | 语义大纲失败伪装「无符号」、终端状态探测失败静默降级——可诊断化（console.error + 状态标注） |

### 并行任务（2 个 agent）

1. ✅ AssetService（git_asset_service.go 新）：`GetAssetBoard` 组合既有能力（GetProjectAllFiles 单次 ls-files + assetKindOf + repoRelativeAssetPath 路径安全；GetLfsLocks 一次性取锁表内存匹配，零逐资产进程）；`.wgit/assets.json` 状态机（仿 subtree 元数据模式，损坏容错重建）；locked 强制与释放回落；SetAssetState 三重校验（状态合法/在扫描结果内/防越仓）；BudgetRatio=0 + Warnings 常驻说明（单资产预算口径缺失，如实呈现）；6 测试（505 资产验证 limit、stub git 注入锁表不依赖本机 git-lfs）；main.go 第三个 Service；bindings 3 Services/517 Methods。
2. ✅ 批次 3：17 处分诊（5 修复 + 12 保留注释）；9 测试。

### 验证

- Go：`go vet`、desktop/server 构建通过；全量 `ok 119.9s / 77.9%`。
- 前端：`vue-tsc`、1093 测试（+9）、生产构建通过。

### 遗留

- AssetBoard 看板 UI（EngineTab 顶部）→ R2·12；资产状态审批工作流（review→approved 的入口）随看板落地。
- 前端吞错长尾余约 40 处（分散在 tools/ 与低频组件）。


## R2·第 12 轮 — AssetBoard 看板（F7 最小闭环前端半）+ 覆盖率缺口补测（测试维度）

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2（F7 半成品） | tools/EngineTab.vue | R2·11 的 AssetService 无 UI，资产状态审批流程无法操作（assetBoard flag 悬空） |
| P2（测试维度） | 全仓 56 个 <50% 覆盖函数 | 其中 5 个高价值可测：detectSourceLanguage(14%)、detectAssetMime(23.5%)、getToolCandidates(18.8%)、parseBundleManifestEntryJSON 族、determineNodeCategory |

### 并行任务（2 个 agent）

1. ✅ AssetBoard.vue（新）：四状态计数徽章 + GeneratedAt + Warnings 黄条 + 500 条列表（等宽 path/状态徽章/owner/体积/状态下拉——仅 review↔approved 可切换，locked/tracked 系统判定禁用并提示）+ 乐观更新失败回滚 + 空态；EngineTab 顶部 FeatureGate 挂点（assetBoard flag）；assetService.ts 薄封装（Call.ByName 全名路由）；utils/byteSize.ts（前端无共享 formatByteSize）；16 双语 key；18 测试（mock.module 拦截 Call.ByName + 真实 mount；过程发现并修复 contrast 审计约束与 mock 并行污染 window 的测试稳定性问题）。
2. ✅ 覆盖率补测：56 个 <50% 函数筛出 Top5 + 不可测清单（GUI 对话框/shell 调起/LFS 网络交互——如实列出未强测）；7 表驱动测试覆盖 8 函数；**覆盖率 77.9% → 78.4%**（detectSourceLanguage 14→86%、其余 5 函数→100%）。

### 验证

- Go：`go vet`、desktop/server 构建通过；全量 `ok 110.7s / 78.4%`。
- 前端：`vue-tsc`、1111 测试（+18）、生产构建通过。

### 遗留

- 审批工作流（review→approved 的多人协作口径）依赖服务端模式权限模型，排期后议。
- 覆盖率缺口余 ~50 个低价值/不可测函数，后续轮次按需补。


## R2·第 13 轮 — 性能 N+1 审计 + 本战役新增 RPC 安全复查（维度轮换）

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2（性能） | git_insights.go PickaxeSearch | 逐命中串行 `git show`（20-60 命中 = 0.5-6s+） |
| P2（性能） | git_studio_ops.go GetStudioCIPipelines | 每条 CI 运行单独 `git log -1`（Windows 进程 spawn 30-50ms/次） |
| P1（安全） | git_ai_service.go SummarizeCommit | commitHash 未校验格式：`-` 开头可注入 git 选项（`--output=<path>` 可把提交内容写到任意可写路径） |
| P1（安全） | git_asset_service.go SetAssetState | 两窗口并发读-改-写丢更新 + 整文件非原子重写 |
| P2（安全） | config.go SaveConfig | `os.WriteFile` 并发保存撕裂 config.json |
| P2（安全） | git_ai_service.go aiHTTPCall | 构造请求错误原样回显含 userinfo 凭证的 URL 到前端 |

### 并行任务（2 个 agent）

1. ✅ N+1 审计：7 文件 14 处分诊（2 修复 + 12 处「不改」逐条给出量级与理由——固定小 N、回退链、已有 worker pool 的不算 N+1）；修复 A：PickaxeSearch 复用 commitDetailsWorkers 模式的 8 并发索引位写回（保序、错误语义不升级）；修复 B：CI 作者批量 `git show -s %H%x00%an` 单命令 + 批量失败回退逐 SHA（语义逐点一致）；12+2 新测试（-race 验证）。
2. ✅ 安全复查：10 RPC × 5 维度逐项结论；4 修复（SummarizeCommit 拒绝 `-` 开头 hash；SetAssetState 加 assetStatesMu 临界区 + writeFileAtomic 原子写；SaveConfig 改 writeFileAtomic；aiHTTPCall 构造错误过 redactLogSecrets）；2 专项「接受不修」结论（TestAIConnection SSRF——配置者=使用者、核心场景即本机 Ollama；config RMW 丢更新——后写者为准无安全影响）；7 安全回归测试（含 32 bool 组合遍历、并发写读不撕裂、选项注入拒绝且目标文件未创建）。

### 验证

- Go：`go vet`、desktop/server 构建通过；全量 `ok 117.5s / 78.4%`。
- 前端：`vue-tsc`、1111 测试、生产构建通过（本轮无前端改动）。

### 遗留

- 其余 12 处 N+1 均为固定小 N/回退链，已记录理由，无需再修。
- 前端吞错余 tools/ 低频组件约 40 处（批次 4 可选）。


## R2·第 14 轮 — 日志一致性（状态变更审计）+ 前端吞错批次 4

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2（审计缺口） | AssetService.SetAssetState / GitService.SaveAppFlags / AiService.SaveAISettings | 非 git 状态变更不进活动日志（ExecGit 自动记录覆盖不到它们），资产审批与灰度开关的「何时被谁改了」不可回溯 |
| P2 | LfsTab.vue:613 | detectLanLfsPeers 完全静默 `.catch(() => {})`（与批次 2 的 LAN 指示同类） |
| P2（分诊保留） | LfsTab 7 处分区加载 / StudioIntegrationSection:52 / SubmodulesTab:78 | 均已汇入聚合 toast 或 reportError/诚实降级——保留并补注释 |

### 任务（agent 因并发上限未启动，主线直做）

1. ✅ 状态变更审计接线：git_exec.go 新增 `recordStateChange`（同锁/同序列/同 200 条上限/同脱敏，Source="app"，前端 StudioTab 的 all/git/ai 过滤器下「全部」可见）；SetAssetState（成功记 `path → state`、落盘失败也记）、SaveAppFlags（记 5 个 flag 的 before→after）、SaveAISettings（记 mode/model/origin/keyChanged，**绝不记 key 明文**）；3 个审计测试（含全 feed 无 key 明文断言）。**过程发现**：v1 配置会被 applyAICompatibility 在内存默认点亮 AI 开关（有意设计），测试基线改用 v2 显式 flags。
2. ✅ 批次 4：三文件分诊（1 修复 console.error 可诊断化 + 2 保留注释 + 其余已合规）；4 测试（含「完全静默空 catch 不得回归」反向断言）。

### 验证

- Go：`go vet`、双构建通过；全量 `ok 178.9s / 77.8%`（并行会话新增在途 AI 代码使分母变大，覆盖率 78.4→77.8，全绿）。
- 前端：`vue-tsc`、1195 测试、生产构建通过（批次 4 +4；其余 +80 为并行 AI 会话新增）。

### 遗留

- 日志维度收尾：活动日志覆盖 git（自动）+ AI（recordAIOperation）+ 状态变更（recordStateChange）三源，口径一致。
- 前端吞错长尾仅剩低频组件零星处，批次制收官。


## R2·第 15 轮 — 文档基线终稿 + 缺陷扫尾守卫

### 实测基线（2026-10-30）

Go 源 108 / Go 测试 743；前端 102 文件 1,195 用例；i18n 5,510 × 2；Services 3（GitService 冻结 509 RPC + AiService 31 + AssetService 3 = 543，rpcCoverage 双向对账）；命令注册表 95/16。

### 并行任务（2 个 agent）

1. ✅ 文档同步：README（三服务架构、§16 新增能力 5 条、数字全量更新）、architecture-audit（§9 服务架构与配置基线 + 复核命令块）、新增 docs/big-files-assessment.md（Go/Vue 超 5k 行/80KB Top5 实测 + 「为何本轮不拆」+ 未来拆分准入条件：职责块 >8、域覆盖率 >70%、每步全绿可二分）。
2. ✅ 缺陷扫尾：全库 **0 panic / 0 TODO / 0 FIXME**（main.go 启动 log.Fatal 唯一豁免；TAPD OpenAPI 为文档化显式空态边界）；新增 3 个守卫测试（库代码禁 panic、Go/前端禁未完成标记，双向红绿自检）——审计固化为可长期执行的门禁。

### 验证

- `go vet`、`go build` 通过；DefectSweep 3 测试通过；文档数字与实测逐项核对一致。

### 遗留

- 大文件拆分按 docs/big-files-assessment.md 的准入条件择机执行（本轮不拆，理由已记录）。


## R2·第 16 轮 — 资源生命周期审计（内存/泄漏维度）

### 问题清单与分诊

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2（确凿泄漏） | review/LanReviewModal.vue:45 | EventSource（/api/events）无 onUnmounted：随 Tab 切换悬挂，闭包持有整棵组件作用域且后续事件继续对已卸载组件 emit |
| — | App.vue 17 个 window 监听、各 Tab 组件的 Observer/interval/拖拽监听 | 逐文件审计全部成对清理（审计通过清单入守卫测试） |
| — | 一次性 UI 反馈 setTimeout（~22 处）/ 应用级单例（crashReporter、useTheme、MutationObserver） | 有意模式，补 Resource lifecycle 注释 |

### 任务（1 个 agent）

✅ 修复 LanReviewModal 泄漏（onUnmounted(disconnectEventSource)）；单例注释 3 处；新增 tests/resourceLifecycle.test.ts（45 断言：修复点配对、19 文件 add/remove 计数相等、Observer/rAF/interval 逐一、全局无裸 WebSocket/EventSource 唯一性）。

### 验证

- 前端：`vue-tsc`、1240 测试（+45）、生产构建通过。
- Go：`go vet` 通过（本轮无 Go 改动）。

### 遗留

- adaptivePolling 有完整 stop() 但无生产调用方（并行 AI 会话的流式通道在途），留意后续接线时的清理。


## R2·第 17 轮 — Server 模式 Flag 环境覆盖（方案 §3.3 收尾）

### 问题清单

| 级别 | 位置 | 问题 |
| --- | --- | --- |
| P2（方案缺口） | config.go LoadConfig | server 模式无 GUI 无法开设置页，预留的 WGIT_FLAG_* 环境变量覆盖未实现 |

### 任务（1 个 agent）

✅ 三条返回路径（文件缺失/损坏回退/正常读取）统一过 `applyEnvFlagOverrides`：优先级 显式 env > applyAICompatibility > 文件值/默认值；`1/true/on/yes`、`0/false/off/no` 大小写不敏感，非法值忽略 + `[Flags]` 告警；只改内存不回写文件（契约测试钉死）。桌面模式零影响（未设变量 LookupEnv 全 miss）；`-tags server` 与桌面同源验证。

### 验证

- `go vet`、双构建通过；EnvFlag 11 用例 + 既有 Flags/Config 测试全绿。

### 遗留

- 无。flag 体系闭环：config 存取 → 设置页开关 → FeatureGate 门控 → server env 覆盖。


## R2·第 18 轮 — 终验（-race -shuffle 全量回归 + 双端全套）

- Go：`go test -race -shuffle=on -timeout 1800s .` → `ok 381.2s`（洗牌序零竞争零失败）。
- 前端：`vue-tsc` 0 错误；1,240 测试 / 103 文件全绿；生产构建通过（index 449KB / gzip 111KB）。
- 双构建：`go build .`（desktop）与 `go build -tags server .` 通过。

## R2·第 19 轮 — 分级统计汇编

### 问题分级统计（R2·第 1–17 轮问题清单表行合计）

| 级别 | 表行数 | 备注 |
| --- | --- | --- |
| P0 | **0** | 全程未出现数据丢失/崩溃级缺陷 |
| P1 | **13 行**（净 ~11 个独立问题；VerifyPrePushLfs 在发现/修复轮各计一次） | 全部修复并带回归测试；含 2 个安全级（选项注入、并发丢更新） |
| P2 | **47 行** | 修复 ~30 行；~17 行经分诊「保留+注释」（exit-1 常态/展示型 best-effort/有意设计），保留决定均记录理由 |

### 吞错清账（错误处理维度专项）

- Go 非测试 ExecGit 吞错：92 处基线 → 分诊 6 批共 ~60 处，传播/标注修复 19 处，其余「未配置 exit-1 常态/展示型」保留+注释；全程未发生一次语义回退（保留点均有注释与回归锁定）。
- 前端空 catch/静默 fallback：135 处基线 → 分诊 4 批 ~80 处，修复 13 处（假干净徽章、假空列表、假无符号等），其余良性保留+注释。

### 改动总量

- 工作区 104 个跟踪文件 +8054/−1841，另有 ~30 个新文件（Go 服务/测试 + 前端组件/composables/测试）。
- 注意：其中包含并行 AI 会话同期在途的 ai_stream/control-plane 工作区改动（同仓协作）；本战役口径的改动清单以各轮记录为准。


## R2·第 20 轮 — 交付轮

- 交付物：本记录（R2·1–20）、docs/product-ui-architecture-roadmap-2026-10.md（顾问方案）、docs/big-files-assessment.md（大文件评估）、README/architecture-audit 基线终稿。
- 最终验证门（R2·18）：Go `-race -shuffle` 全量 381.2s 通过 / desktop+server 构建通过 / 前端 1,240 测试 + vue-tsc + 生产构建通过。
- 覆盖率轨迹：77.0%（战役起点）→ 78.4%（本战役峰值）→ 77.8%（并行 AI 会话在途代码入分母后），方向健康。
- 后续建议（详见交付报告）：TAPD 评审适配的产品口径、AI key 的 vault 全局化、server 模式权限模型落地、大文件按准入条件拆分、E2E/视觉核对补位。


## 第二轮战役后 · 功能完善与 UI 升级批次 A–E（2026-10-30，验收：有条件通过/零阻塞）

| 批次 | 内容 | 交付 |
| --- | --- | --- |
| A | G1 server 权限模型：三层角色（viewer<operator<admin）、编译期桌面恒 admin、WGIT_ROLE env 覆盖（不回写）、8 个破坏性 RPC 门禁（ResetBranch/Push/DeleteBranch/DeleteBatchTags/CleanWorkingTree/ToggleGitHook=operator，OptimizeObjectStore/RemediateStagedSecrets=admin） | 9 测试；双构建 ✓ |
| B-1 | U1 提交区常驻：右栏底部 sticky 段（位置偏离方案"顶部"有据：ConflictBanner 第一位 + composer 弹出菜单贴顶防裁）、v-show 保活、收起摘要持久化 | 24 断言 |
| B-2 | U2 间距语义 token：spacing.css 四档（4/8/12/16，零视觉变化）+ @theme inline 注册；WorkspaceView 12 处试点；CSS +225B | 8 断言 |
| C | G4 批量详情失败可见化（HistoryView → WithStatus，failed toast）；G6 吞错长尾收官（11 处分诊：修复 WorktreesTab 预检假无冲突、SparseTab 假标准 clone、StudioTab activityLogs allSettled；**MaintenanceTab 顺带修真 bug：commitDetailCache.size → size()**） | 11+8 断言 |
| D | U3 DiffViewer ⋯ 溢出菜单（5 项收纳；HistoryView 已被并行会话覆盖，零改动）；U4 左栏收起（workspaceWidth<1280 生效，36px 竖条，独立持久化 key，与 paneSizes 互不覆写，shellLayout 钉住类反向保护）；U5 可访问性审计（10 处纯 aria 修复 + 结构性发现 8 项记待办） | 8+19+25 断言 |
| E | 验收子 Agent 独立审计：门禁全绿（Go ok 182.2s/77.9%；前端 1,383/0）；范围核对通过；测试可信度反转检验 3/3；**无阻塞项** | 有条件通过 |

### 验收修正（已采纳，条件项）

1. **回滚清单补正**：G4 回滚须连带 git_parser.go（BatchResult 类型）、gitService.ts、bindings/gitservice.ts+models.ts+index.ts、types.ts、style.css 的 @import 行、StudioTab.vue；共享文件（config.go/git_service.go/WorkspaceView.vue/DiffViewer.vue/SettingsView.vue/i18n）回滚必须 **hunk 级**，不可整文件 checkout；批次 A 四新文件 + spacing.css + 7 个新测试文件可整文件删除。
2. **口径修正**：U4 折叠区间实为 (0,1280)；G6 实含 StudioTab 修复与 MaintenanceTab 真 bug 修复；U5 文件级 diff 混有并行布局重构（hunk 级回滚原因）。
3. 外部红哨兵：glass.test.ts 曾因并行会话 UiModal surface 扩展瞬时红，已自愈（14/14）；复发时建议测试正则分支感知而非限制组件 API。

