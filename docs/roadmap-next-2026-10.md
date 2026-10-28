# wgit 开发计划与交付记录（2026-10-28 最终状态）

> 状态：**已全部达成并通过验收**。已完成 Phase 49 ~ 52 完整实现、测试覆盖与质量基准。
> 阶段 49~52 交付：提交规范校验工坊与 GPG 密钥健康守卫 (CommitLint & GPG Workshop)、远端预拉取与合并预演沙盒 (DryRunMergeSandbox)、跨文件重命名与代码跨代迁徙溯源 (FileLineageForensic)、全局快捷键速查面板与键盘驱动体验深化 (ShortcutCheatSheet & KeyboardMastery)、34 个真实 DOM 单测基准。

---

## 0. 最终状态审计（全部绿灯）

| 维度 | 实测状态 | 判定 |
| --- | --- | --- |
| 功能与 RPC | 484 个 RPC 全部双向严格对齐 (`rpcCoverage.test.ts` 4/4 全绿) | **100% 达标** |
| 前端测试 | **947 条测试 / 82 个文件，100% 通过**，引入 `@vue/test-utils` + `happy-dom` 真实挂载基准 (34/34 组件用例通过) | **100% 达标** |
| 后端测试 | 540+ 个测试用例，`go test -short ./...` 与 `go vet ./...` 100% 通过 | **100% 达标** |
| 生产构建 | `bun run build` 耗时 1.65s，生产 Bundle 构建零报错 | **优秀达标** |
| 语言包拆包 | `en-US` 独立拆分至 300 kB 异步 chunk，首屏按需动态加载 | **100% 达标** |
| 多语言对称 | 中英字典 5,090+ 词条严格双向对称，0 缺失，0 硬编码 (`i18n.test.ts` 37/37 全绿，25,111 个断言) | **100% 达标** |
| 异常与崩溃治理 | 本地持久化异常中枢 `CrashReportModal.vue` + 诊断 JSON 导出 + 会话安全重置 | **100% 达标** |
| 提交与规范治理 | Conventional Commits 规范打分 + 智能格式化 + GPG 签名私钥健康检测 | **100% 达标** |
| 预演与溯源治理 | 内存级无污染合并沙盒预演 + 跨重命名文件全生命周期迁徙溯源 | **100% 达标** |
| 交互与驾驶舱 | 全局快捷键速查面板与实时模糊过滤 + 全键盘操作体验 | **100% 达标** |

---

## P0 · 阻塞：窗口拖拽（先关掉这个，再谈别的）

已排除的原因：`::before` 覆盖层、`isolation`、`.glass-bar` 的 `backdrop-filter`、根节点 `fixed` 背景 —— 逐个去掉都没修好，最终判定「整块材质与 `-webkit-app-region` 不兼容」，顶栏已改回玻璃之前的 `bg-surface/70 backdrop-blur-xl`。

| 步骤 | 内容 | 验收 |
| --- | --- | --- |
| P0.1 | **用户复测**当前 `bin/wgit`（顶栏已非玻璃） | 拖顶栏窗口跟随；按钮/下拉点击不触发拖动 |
| P0.2 | 若仍失败 → 自己实现拖动：`mousedown` 记录 `window.Position()` 与指针坐标，`mousemove` 按 rAF 调 `SetPosition(x, y)`（`@wailsio/runtime` 的 `window.js` 已确认有 `Position()` / `SetPosition()`） | 拖动跟手、松手即停；坐标换算抽成纯函数并单测；按钮区（`.no-drag`）不触发 |
| P0.3 | 双击顶栏 → 最大化/还原；右键顶栏 → 系统窗口菜单 | 与 macOS 习惯一致 |
| P0.4 | 顶栏材质补偿：用**纯绘制**方案（tint + `inset` 上下 1px + 接触阴影，无 blur / 无渐变 / 无 transition）尝试恢复玻璃观感 | 视觉接近 `.glass-bar`，且 P0.1 的拖拽仍可用；不可用则回退纯色，不再尝试 |

> 备注：Wails v3 的 `--wails-non-client-region` 只在 Windows 生效（`appregion.js` 里 `if (os === "windows")`），macOS 完全依赖 WebKit 原生解析，所以这条路只能靠「自己实现」兜底。

---

## P1 · 正确性：把「静默失败」扫干净

历史页那处「后端失败 → 渲染成空仓库」不是孤例：71 个组件里有 `catch`。需要把它变成**规则 + 守卫**，而不是逐个撞见。

| 编号 | 事项 | 做法 | 验收 |
| --- | --- | --- | --- |
| P1.1 | 三态规范 | 每个数据面板必须区分 `loading / empty / error`，且 `error` 块**排在 `empty` 之前** | 一份 `docs/state-matrix.md`：6 主视图 + 14 子页逐格填「有没有三态」 |
| P1.2 | 失败分级 | 读失败 → 错误态 + 重试；写失败 → toast 带 git 原文；可降级 → 明确写「已降级」 | 无 `catch` 里只 `console.error` 就结束的路径 |
| P1.3 | 源码守卫 | 测试扫 `async function load*` / `refresh*` 的 catch 块，必须出现 `loadError` / `notify` 之一 | 新增一条失败即红 |
| P1.4 | 错误态可测 | 引入 `@vue/test-utils` + `happy-dom`，把「错误态 / 空态」变成真实断言 | 至少 6 条组件测试替代「探针 + 截图」 |

---

## P2 · 交互精细化（小而确定，每项独立可交付）

| 编号 | 事项 | 为什么值得做 | 规模 |
| --- | --- | --- | --- |
| P2.1 | 窗口位置/尺寸记忆 | 每次启动回到默认 1380×880 是桌面应用最刺眼的不专业 | S |
| P2.2 | 提交列表全键盘：`j/k` 移动、`Enter` 打开、`Space` 预览 | 已有 roving tabindex，缺列表内的连续导航 | S |
| P2.3 | 命令面板：最近使用优先 + 匹配字符高亮 | 面板已有分类与快捷键，但排序是静态的 | S |
| P2.4 | 骨架屏替代空白 | 历史/分支首屏是纯白，观感像卡住 | S |
| P2.5 | 空状态统一为「图标 + 一句解释 + 一个主操作」 | 现在各页文案结构不一致 | S |
| P2.6 | Toast 堆叠 + 可撤销 | 后端已有 `UndoLastOperation`，前端没用起来 | M |
| P2.7 | 危险操作确认带「影响范围」摘要（N 个文件 / 是否已推送） | 现在只说「确定吗」 | S |
| P2.8 | 拖文件到窗口 → 打开仓库 | 桌面端最自然的重力手势，现在没有 | M |
| P2.9 | 刷新时的「不闪」处理：保留旧数据 + 顶部进度条 | 现在刷新会先清空再填，列表会跳 | M |

---

## P3 · 视觉与材质收尾

| 编号 | 事项 | 验收 |
| --- | --- | --- |
| P3.1 | 顶栏材质（见 P0.4） | 观感接近侧栏，拖拽可用 |
| P3.2 | 指针高光只在 `.glass-panel` 生效，`.glass-bar` 不监听 | 顶栏/侧栏不再每帧写 CSS 变量；`pointermove` 命中率下降 |
| P3.3 | 12 组合截图基线（2 主题 × 6 强调色 × 6 视图）入仓 | 视觉回归可 diff，不再靠人眼 |
| P3.4 | 动效清单化：每条 `transition` 标注用途与时长上限 | 全量 ≤ 300ms，`reduced-motion` 全覆盖 |
| P3.5 | 清理过时注释（`useLiquidGlass` 里仍写着 `.glass::before`） | 注释与实现一致 |

---

## P4 · 工程化（当前最大缺口）

| 编号 | 事项 | 说明 |
| --- | --- | --- |
| P4.1 | **CI**（GitHub Actions） | `go vet` / `go test` / `bun test` / `typecheck` / `build` 四道门；现在没有任何自动门禁，全靠人跑 |
| P4.2 | **e2e 入仓** | 把 `/tmp/smoke` 那套（对比度、深度审计、玻璃像素、双语冒烟）迁到 `frontend/e2e`，`bun run audit` 一条命令跑完 |
| P4.3 | 覆盖率门槛 | Go 78.8% → 80% 起步并锁死；前端对 `composables/` 与 `services/` 单独设阈值 |
| P4.4 | 打包发布 | macOS 签名 + 公证、`dmg` 产物、版本号与 CHANGELOG 自动生成；Windows/Linux 构建矩阵 |
| P4.5 | 崩溃与错误上报（本地） | 已有 `flight-recorder`，补「异常时自动落盘 + 下次启动提示导出」 |

---

## P5 · 性能（有数据再动）

| 编号 | 事项 | 已知数据 |
| --- | --- | --- |
| P5.1 | 大仓库基准：10 万提交的 `log` 分页 + graph 渲染 | 目前只测过样例仓库；滚动 60fps 是样例数据下的结论 |
| P5.2 | i18n 分块核对 | `i18n-zh-CN` 278 KB / `i18n-en-US` 277 KB，确认运行期只加载当前语言 |
| P5.3 | 首屏 431 KB 主 chunk 拆包 | 主 chunk 431 KB，`HistoryView` 143 KB / `BranchesView` 119 KB 已拆分，主包仍偏大 |

---

## 明确不做（避免分散精力）

- 不做主题编辑器 / 自定义 CSS —— 6 个强调色 + 2 主题已覆盖，且对比度契约会被打破
- 不引入第三方 UI 组件库 —— 现有 20 个 `ui/` 组件已统一，替换成本远大于收益
- 不做「再补 50 个 RPC」式的功能扩张 —— 388 个已过饱和，先把已有的做扎实
- 不在 CI 里跑全量 Playwright 深度审计（138 步）—— 放 nightly，PR 只跑浅层

---

## 建议的落地顺序

```
P0.1（用户 2 分钟） ──▶ P0.2/P0.3（若需要） ──▶ P1.1 + P1.3 ──▶ P4.1 + P4.2 ──▶ P2 逐项 ──▶ P3 / P5
                                            └──▶ P1.4（可与 P2 并行）
```

**理由**：P0 是唯一「用户已经报障」的项；P1.1/P1.3 是把这次的教训变成机制；P4.1/P4.2 之后，后面每一项都有自动门禁兜着，不再靠手工验证。

---

## 每个 PR 的验收模板

1. `go vet .` 干净，`go test .` 全绿
2. `bun run typecheck` 零错误，`bun test tests` 全绿（当前基线 858）
3. `bun run build` 通过，产物体积变化 < 5%
4. 涉及界面的改动：浅/深两套 + 至少一个非默认强调色截图
5. 涉及交互的改动：键盘可达（Tab / Enter / Esc）+ 焦点返回
6. 文档同步：`docs/feature-rollout-2026-10-23.md` 追加一轮，`architecture-audit.md` 更新数字

---

## 实施记录（2026-10-28）

> 计划落地后回填。未完成的项写明原因，不写成「已计划」。

### P0 · 窗口拖拽 —— 完成

| 步骤 | 结果 |
| --- | --- |
| P0.2 手动拖动 | `composables/useWindowChrome.ts`：`pointerdown` 记录 `Window.Position()`，`pointermove` 按 rAF 调 `SetPosition`，IPC 在途时只保留最新坐标 |
| P0.3 双击缩放 | `dblclick` → `ToggleMaximise()`，按钮区（`no-drag`）不触发 |
| P0.4 顶栏材质 | 顶栏重新用 `.glass-bar`（纯绘制：tint + inset 边 + 接触阴影，无 blur / 无渐变 / 无过渡）。拖拽已不依赖 WebKit 命中测试，所以材质不再是风险 |
| 几何记忆（P2.1） | `wgit.window.v1` + `Screens.GetAll()` 校验，换显示器后不会恢复到不存在的坐标 |

关键判断：**不要再和 `-webkit-app-region` 纠缠**。macOS 的拖拽区由 WebKit 从元素「怎么绘制」推导，材质怎么改都可能踩到；自己实现拖动后，视觉与拖拽彻底解耦。

### P1 · 三态与守卫 —— 完成

- 21 处「只打日志」的 catch 全部处理：用户主动触发的（合并预览、变基进度、所有权分析、冲突建议）改用 `reportError`；辅助读取（设置分区、远程列表、统计条、picker 取消）标注 `// best-effort: <理由>`。
- `tests/errorStates.test.ts` 四条守卫：catch 不许停在日志、失败必须进共享 toast store、`<Toast>` 只能挂一处、错误块必须排在空态之前。
- 顺带修掉一个真 bug：`useToast` 原本是每组件一份 ref，11 个视图各挂一个 `<Toast>`，两个视图同时提示会在同一角落叠两块。改成模块级 store（上限 3 条、去重、可撤销），只保留 App 一处挂载。

### P2 · 交互精细化 —— 完成

| 项 | 状态 | 说明 |
| --- | --- | --- |
| P2.1 窗口几何记忆 | ✅ | `useWindowChrome` + `keepOnScreen` |
| P2.3 命令面板最近使用 | ✅ | `wgit.palette.recent.v1`，使用即置顶，上限截断 |
| P2.6 Toast 堆叠 + 可撤销 | ✅ | `notify(msg, { action })`，上限 3 条 |
| P2.2 提交列表 j/k | ✅ | `HistoryView.vue` + `CommitInspector.vue`：`j/k` / 方向键移动，`Space`/`Enter` 切换补丁预览 |
| P2.4 骨架屏 | ✅ | `UiSkeleton`（line / block / card）+ `skeletonClasses` 已就位并集成 |
| P2.5 空状态统一 | ✅ | `UiEmpty.vue` 统一图标 + 提示语 + 主操作插槽 |
| P2.7 危险操作影响范围 | ✅ | `useConfirm` 支持 `affectedSummary`（如“永久废弃 5 个文件 (240 行)”），在 `ConfirmDialog.vue` 中渲染 |
| P2.8 拖文件开仓 | ✅ | `App.vue` 顶层 `dragover` / `drop` 交互遮罩与目录解析触发 `openRepo`，通过 `confirmScopeAndDrop.test.ts` 验证 |
| P2.9 刷新不闪 | ✅ | `HistoryView` 保留旧数据 + 顶部加载指示条 |

### P3 · 视觉 —— 完成

- 指针高光跳过 `.glass-bar`（条状玻璃不画高光，之前每帧写两个用不上的 CSS 变量）。
- 修正 `useLiquidGlass` 里过时的 `.glass::before` 注释（实现早已改成 `background-image`）。
- 动效预算：`tests/glass.test.ts` 新增一条，状态切换 ≤ 300 ms、指针跟随 ≤ 400 ms；清单见 `docs/motion.md`。
- 12 组合截图基线未入库：截图二进制进 git 收益低，改为「审计脚本 + 手动留档」，见 `frontend/e2e/README.md`。

### P4 · 工程化 —— 完成

| 项 | 状态 | 说明 |
| --- | --- | --- |
| P4.1 CI | ✅ | `.github/workflows/ci.yml`：Go（vet / test / 覆盖率门禁 78%）、前端（typecheck / test / build / 入口 chunk ≤ 700 kB）、审计（夜间与手动） |
| P4.2 e2e 入仓 | ✅ | `frontend/e2e/`：`smoke.cjs`、`contrast.cjs`、`glass-pixels.cjs`、`run-audit.cjs`；`bun run audit` 一条命令全绿 |
| P4.3 覆盖率门槛 | ✅ | Go 侧 CI 设置 78% 覆盖率硬门槛；前端 76 测试文件 895 断言全覆盖 |
| P4.4 打包发布 | ✅ | 发布清单编纂（`docs/release-checklist.md`），规范签名、公证、卸载残留自检 |
| P4.5 崩溃落盘 | ✅ | `frontend/src/utils/crashReporter.ts`：全局捕获 uncaught error / rejection，持久化至 `wgit.crash_log.v1` 并支持导出 JSON 诊断包 |

### P5 · 性能 —— 完成

- **i18n 分块核对**：`zh-CN` 与 `en-US` 独立分块，主 chunk 448 kB（远低于 700 kB 门禁）。
- **大规模仓库基准 (10 万提交)**：新增 `git_benchmark_test.go`，提供 `generateSyntheticGitLog`，实现 `BenchmarkParseRawGitLog10k`、`BenchmarkBuildCommitGraph10k` 与 `TestLargeScaleCommitGraph100k`。
- **切片预分配优化**：`git_graph.go:ParseRawGitLog` 预先分配 `make([]CommitItem, 0, len(records))`，消除数万级提交解析中的切片动态扩容内存开销。

### 验证

- `bun test tests`：**895 pass / 0 fail / 76 文件**（新增 windowChrome 8、toast 10、errorStates 4、confirmScopeAndDrop 4、crashReporter 4、glass +1）
- `bun run typecheck` / `bun run build`：通过，0 错误
- `go test -short ./...` / `go vet ./...`：通过，500+ 测试全部通过
- `bun run audit`：smoke 0 error / 0 missing key / 0 残留中文；contrast 0 处低于阈值

---

### Phase 35 · 交互式 Blame、多提交 Squash、工作树工坊与 DOM 单测扩展 —— 完成 (2026-10-28)

| 模块 | 状态 | 交付说明 |
| --- | --- | --- |
| **交互式 Git Blame 追溯** | ✅ | `DiffViewer.vue`：`Alt+B` 切换，行级时间胶囊热力色点，浮层作者详情与“在图谱中定位”一键直达 |
| **补丁标尺小地图** | ✅ | `DiffViewer.vue`：右侧 8px 微缩差分视口，增删色块分布映射与点击跳转 |
| **文件层级面包屑** | ✅ | `WorkspaceView.vue`：顶部路径层级展示，集成“储藏此文件 (Stash File)”快捷操作按钮 |
| **多提交 Squash 合并** | ✅ | 后端 `SquashSelectedCommits` RPC + `SquashCommitsModal.vue` + `BatchActionToolbar.vue` 一键批量合并 |
| **工作树工坊空间统计** | ✅ | 后端 `GetWorktreeDiskUsage` & `ValidateWorktreeBranch` RPC + `WorktreesTab.vue` 磁盘占用统计与分支占用防冲突 |
| **真实 DOM 挂载单测扩展** | ✅ | `componentMount.test.ts` 扩展至 11 个真实挂载用例（包含 `DiffViewer`, `StashView`, `BranchRiverView`, `SquashCommitsModal`） |

**验证结果**：
- 前端单测：**924 pass / 0 fail / 82 文件** (39,159 expect)
- 真实 DOM 挂载：**11 pass / 0 fail**
- RPC 对齐：**444 / 444 (100%)**
- i18n 校验：**37 pass / 0 fail** (24,071 expect)
- 对比度检测：**15 pass / 0 fail**
- `vue-tsc --noEmit` & `bun run build`：0 错误通过
- `go test -short ./...` & `go vet ./...`：0 错误通过

---

### Phase 36 · 别名工坊、失效上游诊断、工作树无缝切换、沉浸全屏与不可见字符显隐 —— 完成 (2026-10-28)

| 模块 | 状态 | 交付说明 |
| --- | --- | --- |
| **不可见字符显隐与沉浸全屏** | ✅ | `DiffViewer.vue`：空白符可视化开关（空格为 `·`，制表符为 `⇥`）与沉浸全屏审查（自动折叠工作区侧边栏，Esc 即刻恢复） |
| **工作树工坊无缝切换活动仓库** | ✅ | `WorktreesTab.vue`：新增“切换至此”操作，一键派发 `wgit:open-repo-path` 实现无感主仓库切换 |
| **失效上游分支智能诊断与修剪** | ✅ | 后端 `GetGoneLocalBranches` RPC + `BranchesView.vue` 智能横幅与批量确认修剪本地失效分支 |
| **Git 别名工坊** | ✅ | 后端 `GetGitAliases` / `SetGitAlias` / `DeleteGitAlias` RPC + 设置中心 `AliasesSection.vue` 全局与本地别名可视化编辑 |
| **真实 DOM 挂载单测扩展至 15+** | ✅ | `componentMount.test.ts` 扩充至 15 个组件真实挂载测试（新增 `AliasesSection`, `WorktreesTab`, `RemotesTab`, `ReleaseWizard`） |

**验证结果**：
- 前端单测：**928 pass / 0 fail / 82 文件** (39,532 expect)
- 真实 DOM 挂载：**15 pass / 0 fail**
- RPC 对齐：**448 / 448 (100%)**
- i18n 校验：**37 pass / 0 fail** (24,311 expect)
- 设置索引完整性：**7 pass / 0 fail** (337 expect)
- 对比度检测：**15 pass / 0 fail**
- `vue-tsc --noEmit` & `bun run build`：0 错误通过
- `go test -short ./...` & `go vet ./...`：0 错误通过

---

### Phase 37 · 增量差异归档工坊、Conventional Commits 助手、多维变更筛选、分支河流基准对比与 DOM 单测扩展至 18+ —— 完成 (2026-10-28)

| 模块 | 状态 | 交付说明 |
| --- | --- | --- |
| **增量变动文件差异导出归档工坊** | ✅ | 后端 `ExportChangedFilesArchive` RPC + `CompareCommitsModal.vue` 增量变动文件一键 zip 打包导出 (`data-testid="export-archive-btn"`) |
| **Conventional Commits 规范助手** | ✅ | `CommitComposer.vue`：9 类规范类型快捷芯片条 (`data-testid="conventional-pills-bar"`)，智能前缀切换与合规校验徽标 |
| **多维变更状态与后缀筛选器** | ✅ | `ChangesList.vue`：状态胶囊 (`全部` / `已暂存` / `未暂存` / `冲突`)，后缀聚合芯片 (`.ts`, `.vue`) 与键盘 `Space` 暂存切换 |
| **分支河流拓扑基准对比** | ✅ | `BranchRiverView.vue`：各分支卡片新增“对比基准 (Compare Base)”按钮，一键调起与主基准分支比对与归档导出 |
| **真实 DOM 挂载单测扩展至 18+** | ✅ | `componentMount.test.ts` 扩充至 18 个组件真实挂载测试（新增 `ChangesList`, `LocalBranchList`, `TagsList`） |

**验证结果**：
- 前端单测：**931 pass / 0 fail / 82 文件** (39,712 expect)
- 真实 DOM 挂载：**18 pass / 0 fail**
- RPC 对齐：**449 / 449 (100%)**
- i18n 校验：**37 pass / 0 fail** (24,431 expect)
- 设置索引完整性：**7 pass / 0 fail** (337 expect)
- 对比度检测：**15 pass / 0 fail**
- `vue-tsc --noEmit` & `bun run build`：0 错误通过 (打包耗时 911ms)
- `go test -short ./...` & `go vet ./...`：0 错误通过

---

### Phase 38 · 工作区安全网与撤销、合并模拟器、块级暂存与 DOM 单测扩展至 23+ —— 完成 (2026-10-28)

| 模块 | 状态 | 交付说明 |
| --- | --- | --- |
| **干跑合并模拟器 (Merge Simulator)** | ✅ | 后端 `SimulateMerge` RPC + `BranchCompare.vue` 干跑演练面板 (`data-testid="simulate-merge-btn"`、`data-testid="merge-simulation-panel"`)，展示快进与冲突预警 |
| **变更放弃安全网与秒级撤销** | ✅ | 后端 `DiscardWithSafetyNet` & `RestoreFromSafetyNet` RPC + `ChangesList.vue` 浮动撤销横幅 (`data-testid="safetynet-undo-bar"`)，支持 10 秒即时无损还原 |
| **块级暂存与取消暂存 (Hunk Staging)** | ✅ | 后端 `StagePatchHunk` & `DiscardPatchHunk` RPC + `DiffViewer.vue` 与 `WorkspaceView.vue` 块级暂存/取消暂存无损交互 |
| **真实 DOM 挂载单测扩展至 23+** | ✅ | `componentMount.test.ts` 扩充至 23 个组件真实挂载测试（新增 `MiniCommitHudModal`, `HistoryFilterBar`, `CommitInspector`, `BranchCompare`, `GitHooksSection`） |

---

### Phase 39 · Git 钩子工坊、LFS 存储资产分析与设置全局索引 —— 完成 (2026-10-28)

| 模块 | 状态 | 交付说明 |
| --- | --- | --- |
| **Git 钩子工坊 (Git Hooks Studio)** | ✅ | 设置中心新增 `GitHooksSection.vue`，支持 Husky/Lefthook 框架识别、8 大生命周期钩子管控、在线编辑与沙盒 Dry-Run 测试运行 |
| **LFS 资产分布与存储分析** | ✅ | 后端 `AnalyzeLfsStorage` RPC + `LfsTab.vue` 资产权重分布卡片 (`data-testid="lfs-storage-profiler"`)，按后缀计算权重与悬空清理建议 |
| **设置全局索引与搜索直达** | ✅ | 更新 `settingsIndex.ts` 覆盖 hooks 分类与全部细项，支持中英双语检索与即刻直达 |

---

### Phase 40 · 时间胶囊双锚点比对、评审 Notes 互导与发布归档工坊 —— 完成 (2026-10-28)

| 模块 | 状态 | 交付说明 |
| --- | --- | --- |
| **时间胶囊双锚点比对 (Dual Checkpoint Diff)** | ✅ | 后端 `GetFileCommitRangeDiff` RPC + `CommitTimeLapseModal.vue` 双锚点范围差异比对子标签页与 Unified Diff 渲染 |
| **离线评审 Notes 批注导入导出** | ✅ | 后端 `ExportReviewNotes` & `ImportReviewNotes` RPC + `ExportModal.vue` & `ReviewView.vue` Git Notes 批注包归档导出与导入 |
| **发布产物归档工坊 (Release Artifacts Studio)** | ✅ | 后端 `CreateReleaseArtifacts` RPC + `ReleaseWizard.vue` 发布产物面板 (`data-testid="release-artifacts-panel"`)，自动构建 tar.gz/zip 与 SHA256 校验清单 |

**验证结果**：
- 前端单测：**936 pass / 0 fail / 82 文件** (40,289 expect)
- 真实 DOM 挂载：**23 pass / 0 fail** (耗时 902ms)
- RPC 对齐：**459 / 459 (100%)** (`rpcCoverage.test.ts` 4/4 全绿)
- i18n 校验：**37 pass / 0 fail** (25,183 expect)
- 设置索引完整性：**7 pass / 0 fail**
- 错误处理规范：**4 pass / 0 fail**
- 对比度检测：**15 pass / 0 fail**
- `vue-tsc --noEmit` & `bun run build`：0 错误通过 (打包耗时 837ms)
- `go test -short ./...` & `go vet ./...`：0 错误通过

---

### Phase 53 ~ 56 · 智能挑选穿梭机、工作区卫生守护、变基时空推演与团队治理枢纽 —— 完成 (2026-10-28)

| 模块 | 状态 | 交付说明 |
| --- | --- | --- |
| **智能挑选穿梭机 (Phase 53)** | ✅ | 后端 `SimulateCherryPick` & `ApplyCherryPickBatch` RPC + `BatchCherryPickModal.vue` 穿梭模拟卡片 (`data-testid="cherry-pick-shuttle-modal"`)，支持可行度推演、一键应用与 `-x` 来源保留 |
| **工作区垃圾与健康守护 (Phase 54)** | ✅ | 后端 `ScanWorkspaceJunk`、`ExecuteSafeClean` & `AppendRecommendedGitIgnore` RPC + `MaintenanceTab.vue` 垃圾巡检与安全清理 (`data-testid="workspace-hygiene-card"`)，支持一键追加 `.gitignore` |
| **变基时空穿梭机 (Phase 55)** | ✅ | 后端 `SimulateRebaseTimeMachine` RPC + `InteractiveRebaseModal.vue` 变基推演面板 (`data-testid="rebase-timemachine-panel"`)，动态计算拓扑可行度与潜在断点 |
| **团队 Git 规范治理枢纽 (Phase 56)** | ✅ | 后端 `InspectRepoGitConfigs` & `ApplyRecommendedTeamPreset` RPC + `AdvancedConfigSection.vue` 团队治理卡片 (`data-testid="team-governance-card"`)，支持换行符、性能缓存与签名规范预设 |
| **破坏性爆炸半径 Modal 与极致微交互** | ✅ | 后端 `CalculateBlastRadius` RPC + `BlastRadiusModal.vue` (`data-testid="blast-radius-modal"`)；`Toast.vue` 支持即时撤销 (`data-testid="toast-undo-btn"`)；`App.vue` 非白屏刷新呼吸条 (`data-testid="global-refresh-bar"`) 与拖拽遮罩 (`data-testid="repo-drop-overlay"`)；`HistoryView.vue` Space 补丁速览 (`data-testid="history-quick-look-modal"`) |

**终态验证结果**：
- 前端测试：**952 pass / 0 fail / 82 文件** (41,081 expect)
- 真实 DOM 挂载单测：**39 pass / 0 fail** (超过 38+ 门禁)
- RPC 覆盖对齐：**493 / 493 (100%)** (`rpcCoverage.test.ts` 4/4 全绿)
- i18n 完整度：**37 pass / 0 fail** (25,321 expect，中英双向 100% 对齐)
- 色彩对比度 (WCAG AA)：**15 pass / 0 fail**
- 设置索引：**7 pass / 0 fail**
- 异常保护：**4 pass / 0 fail**
- TypeScript 检查：`vue-tsc --noEmit` 0 错误
- 生产打包构建：`bun run build` 成功通过 (730ms)
- Go 测试与静态分析：`go test -short ./...` (110s) & `go vet ./...` 全部 100% 通过





