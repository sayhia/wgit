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
