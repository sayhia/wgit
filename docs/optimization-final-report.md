# wgit 20 轮优化交付报告（2026-10-26 ~ 2026-10-27）

> 目标：按「缺陷/边界、错误处理、可维护性、性能、安全、日志、测试、文档」逐轮审查，输出 P0–P2 问题清单；每轮并行 3–8 个文件不冲突任务；统一构建/测试/回读验证；记录发现、改动、验证与遗留；不换技术栈、不擅改架构。20 轮后交付本报告。

## 1. 结论

- **20 轮全部执行完毕**：第 1–18 轮为代码优化，第 19 轮为最终审计与全量回归，第 20 轮为交付验收。
- **P0 未出现；P1 全部修复**。最终全量门禁通过：Go 500 测试 / 78.8% 覆盖率、`-race -shuffle`、server + desktop 构建；前端 614 测试 / 41 文件、typecheck、生产构建；i18n 4,259 × 2 对齐。
- **未提交、未推送**：工作树保留全部改动，等待用户确认。
- 详细逐轮记录见 [optimization-rounds-2026-10.md](optimization-rounds-2026-10.md)。

## 2. 基线与最终指标

| 指标 | 基线（第 1 轮前） | 最终 | 变化 |
| --- | --- | --- | --- |
| Go 测试函数 | 397 | 500 | +103 |
| Go 语句覆盖率 | 76.7% | 78.8% | +2.1pp |
| 前端单元测试 | 597 / 38 文件 | 614 / 41 文件 | +17 / +3 |
| i18n key（zh/en 各） | 3,808（文档旧值） | 4,259 | +451 |
| RPC / facade / UI 引用 | 388 / 388 / 379 | 388 / 388 / 381 | +2 UI 引用 |
| 构建 | — | `go build -tags server .` + `go build .` 通过 | — |
| 门禁 | — | `go vet`、`-race -shuffle`、`vue-tsc`、生产构建全绿 | — |

## 3. 20 轮主题

| 轮次 | 主题 | 主要落点 |
| --- | --- | --- |
| 1 | 错误处理 + 文档 | 合并预检/资产检查部分失败可见化；健康体检 git-dir 失败；基线对账 |
| 2 | 性能 + 可维护性 | 项目详情/分支洞察并行；合并预览 warning 去重；重复格式化函数清理 |
| 3 | 安全 + 日志 | Hook 名称白名单；活动日志 URL 凭据脱敏；文档基线 |
| 4 | 测试 + 文档 | 重复 helper 清理；RPC/文档一致性；回归测试 |
| 5 | 性能 + 边界/测试 | 健康体检 6 项诊断并行；git-dir 边界 |
| 6 | 文案边界 + 真实性 + 性能 | 英文单复数/日历校验；分支洞察 statusWarning；多远端并行 |
| 7 | 测试覆盖 + 安全 | 裸令牌脱敏（GitHub/GitLab/AWS/key=value）；版本建议/MIME 测试 |
| 8 | 缺陷 + 安全（URL）+ 日志 | 预发布/大写 V 版本线；Web URL 逐段转义；console 内容日志移除 |
| 9 | 安全（错误通道）+ 真实性 + 测试 | ExecGit 错误脱敏；发布分支读取失败不再伪装 detached；Swift 反解测试 |
| 10 | 缺陷 + 性能 + 错误处理 + 测试 | Swift 长度前缀解析；Git Notes 并行；工作树清理错误传播；LFS 锁安全测试 |
| 11 | 真实性 + 安全边界 + 性能 + 测试 | LFS 锁错误传播/未跟踪冲突；引擎启动失败；批量提交详情并行；TTF 溢出/回绕加固 |
| 12 | 边界/内存 + 正确性 + 性能 | revision/HEAD 文件读取上限；TTF format 4 字形表；历史大对象并行 |
| 13 | 内存边界 + 性能 + 安全 | 差异读取上限；Unity `.meta` 并行；批量丢弃单进程分类；二进制差异错误脱敏 |
| 14 | 真实性 + 性能 + 内存边界 | 评审提交失败警示；策略审计并行；版本内容上限；LFS 属性并行探测 |
| 15 | 性能 + 真实性 + 内存 + 文档 | 子模块配置批量读取；只读保护失败可见；插件头 16 字节读取；i18n 基线修正 |
| 16 | 安全 + 性能 + 真实性 | LFS 锁归属精确匹配；忽略修复批量移出；Unity 元数据读取失败传播 |
| 17 | 真实性 + 性能（批量操作） | 忽略修复调用方消费失败；标签批量删除/单次远端 push；Unity 自动配对单次 add |
| 18 | 一致性 + 导出真实性 + 清理 | 评审 Markdown 导出警示；LFS 归属统一；死 i18n key 清理 |
| 19 | 最终审计与全量回归 | 基线/安全/工作树审计；全量门禁复跑；无新增 P0/P1 |
| 20 | 交付验收 | 本报告、台账补齐、完成审计 |

## 4. 完整改动清单（按维度）

### 4.1 真实性 / 错误处理

- 合并预览、资产元数据/引用：`Promise.allSettled` + 弹窗 warning/单次失败 Toast。
- 健康体检、Studio 拓扑、分支洞察、发布计划：读取失败写入 warnings/阻塞检查，不再伪装成功。
- 评审会话：提交列表读取失败返回 `Warning/WarningKey/WarningParams`，详情 UI 与 Markdown 导出均展示。
- LFS 锁：`CheckRemoteLocksConflict`、`LockAllModifiedLfs`、`SyncLockableFilePermissions` 锁源失败不再吞成“无锁”；chmod/Walk 失败计入 `FailedCount/FailedFiles`。
- 工作树清理：`os.RemoveAll`/prune 失败组合返回错误，不再假成功。
- 忽略修复：批量 `git rm --cached`，失败后用 `ls-files` 复核；`FailedFiles` + 部分失败 Toast；HealthFix/Bootstrap 消费失败状态。
- Unity 元数据：`ls-files` 失败返回错误；自动配对失败用 `diff --cached` 复核。
- 引擎启动：`cmd.Run()` 传播失败，非 macOS/Windows 明确报不支持。
- 批量丢弃：单次 `ls-files -z` 分类，并保留目录输入的 restore 语义。

### 4.2 安全

- Hook 名称白名单，阻断 `../`、绝对路径与选项注入。
- 活动日志与 `ExecGit` 错误通道：URL userinfo/query + GitHub/GitLab/AWS/key=value 裸令牌脱敏。
- `execGitBytes` 错误 stderr 同样脱敏；二进制 stdout 保持原样。
- GitHub/GitLab Web URL 按路径段 `url.PathEscape`，分支斜杠保留。
- TTF cmap：uint64 边界、format 12 越界/回绕钳制、format 4 idDelta/idRangeOffset 正确寻址。
- 原生插件审计只读 16 字节头，避免超大二进制整文件入内存。
- LFS 锁归属统一 trim + name/email `EqualFold` 精确匹配，杜绝 `Al` 误认 `Alice`。

### 4.3 性能

- 并行化：健康体检 6 项、项目详情、分支洞察、多远端状态、Git Notes、批量提交详情、历史大对象、Unity `.meta`、推送前策略审计、LFS `check-attr`。
- 批量化：子模块 URL/path 配置、标签本地删除 + 远端 push、Unity 自动配对、忽略修复移出、批量丢弃分类。
- 上限化：版本文件/差异/内容读取均设 5MB（图片 30MB）上限，超限返回明确错误。

### 4.4 可维护性 / 文档

- 删除重复 helper、死类型、死 i18n key（`branch.days.*`）。
- 英文单复数、UTC 日历校验、semver 预发布/大写 V、Swift 长度前缀。
- README、architecture-audit、逐轮台账持续同步；最终 i18n 4,242 × 2、Go 500 / 78.8%、前端 610 / 39。

### 4.5 测试

- Go 397 → 500 测试，新增覆盖：脱敏、Hook 白名单、版本解析、URL 转义、TTF、LFS 锁、批量操作、内存上限、失败传播、并行顺序/race。
- 前端 597 → 614，新增分支洞察格式化、i18n 契约、评审警示、密钥处置与工作树移动接线等断言。
- 所有轮次均跑 `go vet`、`go test -cover`、`-race -shuffle`、双构建、`vue-tsc`、`bun test tests`、生产构建。

## 5. 分级统计

| 级别 | 数量 | 说明 |
| --- | --- | --- |
| P0 | 0 | 未出现 |
| P1 | 11 行 / 约 9 个独立问题 | 全部修复；`ToggleGitHook` 跨轮重复登记 |
| P2 | 52 个独立问题 + 15 次延后 backlog 登记 | 全部登记、修复或在后续建议中保留 |
| P3 | 8 个独立问题 + 5 次延后登记 | 清理/一致性为主 |
| 并行任务 | 62 项 | 每轮 2–5 个 agent，单轮并发 ≤ 4 |

## 6. 验证证据（最终轮）

- `go vet .`：通过。
- `go test -cover -count=1 .`：500 测试，覆盖率 78.8%。
- `go test -race -shuffle=on -count=1 .`：通过。
- `go build -tags server .`、`go build .`：通过。
- `cd frontend && bun run typecheck`：通过。
- `bun test tests`：614 通过 / 0 失败，41 文件。
- `bun run build`：通过（仅既有 chunk 体积提示）。
- 安全扫描：非测试源码未检出真实 `ghp_`/`glpat-`/`AKIA` 令牌；非测试 Go 0 个 TODO/FIXME。

## 7. 后续建议

1. **7 个未接线 RPC**：4 个兼容别名（Commit / StashSave / AddWorktree / GetFileBlame，已由 `*Advanced/WithOptions` 取代）+ 3 个独特能力（SplitSubtree、UpdateSubtreeMetadata、UnlinkSubtree）。`RemediateStagedSecrets` 已接入 CleanTab、`MoveWorktree` 已接入 WorktreesTab。建议将 3 个 Subtree 能力接回 Subtree UI，别名明确标注兼容层或删除。
2. **i18n chunk（约 564KB）**：按语言/领域拆包并延迟加载，测量首屏 TTI 后再合入。
3. **三套体积格式化**：`formatBytes`/`formatByteSize`/`formatAssetSize` 语义不同，统一前需先定义单位与小数规范。
4. **超大文件**：`git_studio_engine.go`（约 10.6k 行）、`WorkspaceView.vue`、`HistoryView.vue`（约 100KB）建议按域拆分，保持方法名集合与测试字节级等价。
5. **剩余边界**：评审警示未进入会话列表摘要；LFS 身份读取 best-effort；批量 `git rm` 极端数量 ARG_MAX；`GetCommitDetailsBatch` 对全失败仍静默；`DeleteMergedBranches` 部分失败未上报；工作树 unlock 失败仅在后继失败时暴露。
6. **发布纪律**：保持每轮门禁串行；提交/推送需用户明确确认；继续用真实仓库做 E2E 冒烟。

## 8. 约束遵守

- 未更换技术栈、未改动架构；仅增量 helper、并行池、批量命令与上限校验。
- 未提交、未推送；工作树保留全部改动（含并行会话既有的 branding/subtree 等）。
- 每轮测试未通过不算完成；最终全量门禁全绿。

## 9. 20 轮后跟进（2026-10-27）

- **RPC 接线**：
  - `RemediateStagedSecrets` 接入 CleanTab 暂存区密钥扫描。发现敏感信息后新增「移出暂存区」按钮：确认后批量 `git restore --staged`、写入 `.gitignore`，并静默重扫刷新列表；失败保留旧结果并显示后端错误。
  - `MoveWorktree` 接入 WorktreesTab。非主/未锁定/非当前工作树显示移动按钮，弹窗填写不存在的新目录，确认后调用 `git worktree move` 并刷新列表。
- **测试**：新增 `frontend/tests/cleanSecretsRemediation.test.ts` 与 `frontend/tests/worktreeMove.test.ts`（组件接线 + 双语 key 契约）。前端 610 → 614 测试 / 39 → 41 文件；i18n 4,242 → 4,259。
- **验证**：`vue-tsc`、`bun test tests`（614/0）、`bun run build` 通过。未做浏览器端 E2E（需要真实暂存密钥仓库与工作树）；本次仅验证了接线契约与构建。
- **剩余**：3 个独特能力（SplitSubtree / UpdateSubtreeMetadata / UnlinkSubtree）与 4 个兼容别名待决策。
