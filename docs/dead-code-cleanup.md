# 无用代码清理方案（2026-10-22）

## 判定口径

同时满足以下三条才视为「无用后端实现」：

1. 前端 UI 未调用；
2. Go 生产代码无内部调用（排除方法定义与注释）；
3. 无 Go 测试引用。

仅满足 1、但有测试或内部调用的，归为「重复 API」，默认保留。

## 后端：11 个可直接删除

| 方法 | 文件 | 依据 |
| --- | --- | --- |
| `CloneRepo` | `git_service.go` | 无 UI、无内部调用、无测试；欢迎页使用 `CloneRepoAdvanced` |
| `ContinueCherryPick` | `git_service.go` | 无 UI、无内部调用、无测试；冲突横幅使用 `ContinueConflictOperation` |
| `ContinueMerge` | `git_service.go` | 同上 |
| `ContinueRebase` | `git_service.go` | 同上 |
| `PushForceWithLease` | `git_service.go` | 无 UI、无内部调用、无测试；UI 使用 `PushAdvanced` |
| `PushWithOptions` | `git_service.go` | 同上 |
| `RebaseOnto` | `git_service.go` | 无 UI、无内部调用、无测试；UI 使用 `RebaseAdvanced` |
| `SetMergeToolConfig` | `git_service.go` | 无 UI、无内部调用、无测试；外部工具页使用 `SetExternalToolConfig` |
| `SkipCherryPick` | `git_service.go` | 无 UI、无内部调用、无测试；冲突横幅使用 `SkipConflictOperation` |
| `SkipRebase` | `git_service.go` | 同上 |
| `ManageAutoLockLeaseTimeout` | `git_studio_engine.go` | 伪实现（始终返回全零租约）；仅有一个只检查非 nil 的测试块 |

## 后端：12 个「重复但被引用」曾暂保留（第二轮已全部删除）

`AbortMerge`、`AbortRebase`、`AbortCherryPick`、`MergeBranch`、`MergeBranchWithOptions`、`RebaseBranch`、`RebaseBranchWithOptions`、`GetCommitsPaged`、`GetFileHistory`、`GetMergeToolConfig`、`SearchSourceCode`、`ShelveSelectedFiles`。

第一轮因被 `TestGitService` / `TestGitService_MergeAndRebase` 直接覆盖而暂留；2026-10-22 第二轮已全部删除，测试改为覆盖 UI 实际调用的高级 API。

## 前端：10 个未调用封装

`abortMerge`、`abortRebase`、`cloneRepo`、`getCommitsPaged`、`getFileHistory`、`mergeBranch`、`mergeBranchWithOptions`、`openWebUrl`（大小写重复别名，UI 使用 `openWebURL`）、`rebaseBranch`、`rebaseBranchWithOptions`。

同步删除仅服务这些封装的 dev mock 分支：`GetCommitsPaged`、`GetFileHistory`。

## 预期结果

- 后端 RPC：338 → 327
- 前端封装：325 → 315
- UI 可达：315（不变）
- 必填项：后端专属 23 → 12，未调用封装 10 → 0
- README / 审计文档同步：删除对应条目与分类描述

## 执行结果（2026-10-22）

已按本方案执行：

1. 备份：`/tmp/wgit-cleanup-backup-20261022-075329`
2. 删除 11 个后端方法及 `ManageAutoLockLeaseTimeout` 测试块
3. 删除 10 个前端封装（`abortMerge`、`abortRebase`、`cloneRepo`、`getCommitsPaged`、`getFileHistory`、`mergeBranch`、`mergeBranchWithOptions`、`openWebUrl`、`rebaseBranch`、`rebaseBranchWithOptions`）与 `GetCommitsPaged` / `GetFileHistory` dev mock 分支
4. 验证通过：`vue-tsc`、`bun run build`、`go test .`（52 个测试，7.8s）、`go build`
5. 最终计数：RPC 327、facade 315、UI 315、后端专属 12、未调用封装 0；`git_service.go` 4,501 行 / 176 方法，`git_studio_engine.go` 44 方法

## 第二轮清理：12 个重复 API 全部删除（2026-10-22）

删除范围：

1. 后端方法：`AbortCherryPick`、`AbortMerge`、`AbortRebase`、`GetCommitsPaged`、`GetFileHistory`、`GetMergeToolConfig`、`MergeBranch`、`MergeBranchWithOptions`、`RebaseBranch`、`RebaseBranchWithOptions`、`SearchSourceCode`、`ShelveSelectedFiles`
2. 死类型：Go 侧 `MergeToolConfig`、`SelectiveShelveResult`、`SourceSearchResult`、`AutoLockLeaseResult`、`LockLeaseItem`；前端 `src/types.ts` 同名类型。补充扫描又删除 `FastCommitResult`（前端未引用）与 `TAPDTransitionResult`（Go 未引用）
3. 测试调整：`TestGitService_MergeAndRebase` 改用 `MergeAdvanced`、`RebaseAdvanced`、`AbortConflictOperation`；`TestGitService` 移除文件历史 / 分页 / merge tool / 所选文件储藏 / 源码搜索断言；大仓库 live test 的分页断言改用 `GetCommitsFiltered`
4. 重新生成 Wails bindings（315 Methods / 253 Models）

验证与计数：

- `vue-tsc --noEmit`、`bun run build`、`go test .`（52 个测试，8.5s）、`go build` 全部通过
- RPC 327 → 315，facade 315 → 315，UI 可达 315（100%），后端专属 12 → 0，未调用封装 0
- `git_service.go` 4,501 → 4,321 行 / 176 → 165 方法；`git_project_browser.go` 8 → 7 方法
- 备份：`/tmp/wgit-cleanup2-backup-20261022-083307`

## 保留项

无。当前 363 个 RPC 方法全部有 UI 入口（2026-10-23 新增 28 个闭环能力 + 8 个 PR/MR 评审接口均已接入 UI）；后续新增后端能力时应同步提供 UI 入口或明确标注为内部 API。
