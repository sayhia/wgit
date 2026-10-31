# wgit 视图状态矩阵 (State Matrix)

> 依据 P1.1 规范：所有主要视图与数据面板必须清晰区分 `loading / empty / error` 三态，且 `error` 状态的判断与渲染逻辑必须**排在 `empty` 之前**，杜绝将“后端异常 / Git 错误”静默降级为“空仓库 / 无数据”。

---

## 1. 核心主视图 (Main Views)

| 视图 / 面板 | Loading 态表现 | Error 态表现 (优先于 Empty) | Empty 态表现 | 状态保护机制 |
| :--- | :--- | :--- | :--- | :--- |
| **WorkspaceView**<br/>工作区与改动列表 | 顶部骨架指示条 + 文件树加载转圈 | 失败横幅警告 (`ws.diff.failed`) + 错误原文 Toast | 干净工作区提示 (`workspace.empty.clean`) | 改动与差异解耦，读取失败保留上一次有效快照 |
| **HistoryView**<br/>提交图谱与历史 | 虚拟列表转圈 (`history.graph.loading`) | 错误看板 (`history.graph.error.title`) + 重试按钮 | 空分支/空范围提示 (`history.graph.empty`) | `v-else-if="error"` 严格排在 `empty` 前 |
| **CommitInspector**<br/>提交详情检视器 | 头部骨架微动画 (`isLoadingDetail`) | 错误警告卡片 (`v-else-if="error"`) + 原文展开 | 未选中提交提示 (`history.inspector.empty`) | 错误信息完整穿透，杜绝空白提示 |
| **BranchesView**<br/>分支、标签与河流 | 列表骨架屏 (`loading: true`) | 错误横幅 (`branch.toast.load.failed`) + 局部保留 | 空列表引导卡片 (`branches.local.empty`) | `allSettled` 并行捕获，分支与标签错误独立隔离 |
| **StashView**<br/>储藏与快照栈 | 储藏卡片加载骨架 | 错误 Toast (`branches.stash.toast.failed`) | 暂无储藏引导卡片 (`stashes.empty.title`) | 读写分离，丢弃/应用带二次确认与撤销通道 |
| **PrView**<br/>PR / MR 评审中枢 | 评审列表转圈 (`loading: true`) | Provider 错误看板 (`pr.error.title`) + 引导配置 | 暂无待评审 PR 提示 | 认证失效与网络超时分级提示 |
| **ReviewView**<br/>代码审查与协作 | 审查文件树骨架屏 | 审查差异加载错误提示 + 刷新重试 | 暂无变更可供审查 | 本地与局域网 Review 双路降级 |

---

## 2. 工具面板子页 (Tools Sub-Tabs)

| 子页名称 | Loading 态 | Error 态 (优先于 Empty) | Empty 态 | 状态保护机制 |
| :--- | :--- | :--- | :--- | :--- |
| **TerminalTab** | 终端初始连接指示 | 连接断开警告 (`terminal.disconnected`) | 无活跃进程 | 本地 PTY 退出码捕获 |
| **ReflogTab** | 引用变动日志骨架 | 读取失败提示 (`reflog.error`) + 重试 | 引用日志为空 (`reflog.empty`) | 错误优先于空状态 |
| **CleanTab** | 未跟踪文件扫描转圈 | 扫描失败错误指示条 | 工作区纯净提示 | 破坏性删除前显示精准文件量 |
| **RemotesTab** | 远程源拉取状态中 | 远程源连接失败警告看板 | 暂未配置远程源 | 凭据掩码脱敏防泄漏 |
| **WorktreesTab** | 工作树列表加载中 | 锁死/丢失工作树警告卡片 | 暂无附加工作树 | 路径损坏与断开显式标记 |
| **LfsTab** | LFS 指针与大文件扫描中 | LFS 环境检测失败面板 | 未检测到 LFS 大文件 | 锁状态与孤儿指针分类容错 |
| **SubmodulesTab**| 子模块初始化状态中 | 子模块同步冲突/失联报错 | 暂无配置子模块 | 递归状态独立追踪 |
| **SparseTab** | 稀疏规则加载中 | 锥体规则语法错误警告 | 全量工程（未启用稀疏） | 角色预设与自定义规则隔离 |
| **DiskTab** | 大文件与对象占用分析中 | 对象库解析失败警告卡片 | 磁盘占用极低/暂无分析 | 超时优雅降级 |
| **MaintenanceTab**| 诊断项逐条检测中 | FSCK 损坏/悬空对象警报 | 健康度 100 分完美状态 | 诊断失败不中断其余项 |
| **StudioTab** | 游戏引擎配置扫描中 | 引擎路径未找到/配置损坏 | 暂未检测到项目资产 | 多引擎适配降级 |
| **EngineTab** | 二进制资产与烘焙数据读取中| 资产解析器不支持警告 | 暂无引擎特化资产 | 保护纯文本 diff |
| **BisectPanel** | 二分查找提交状态中 | 二分步进失败/断点冲突报错 | 未启动二分查找 | 保存断点状态以便恢复 |
| **ReleaseWizard**| 发布变更集解析中 | 标签冲突/规范校验未通过 | 暂无待发布版本 | 预检拦截硬性阻塞错误 |

---

## 2.5 AI 面板 (AI Panels)

| 面板 | Loading 态 | Error 态 (优先于 Empty) | Empty 态 | 状态保护机制 |
| :--- | :--- | :--- | :--- | :--- |
| **AiQuickSetup** | 首次加载骨架 (`ai-quick-loading`) + `aria-busy` | 错误横幅 `role=alert` + 重试 (`ai-quick-load-error` / `ai-quick-retry`)；profile/key/CLI 失败降级为 `role=status` 警告 | 未配置时显示“未配置”徽标与供应商选择，不伪装成错误 | catalog/config 独立 `allSettled`，失败保留上次有效配置 |
| **AiControlPlaneSection** | `aria-busy` + 禁用保存按钮 | 加载失败横幅 + 重试 (`ai-control-load-error` / `ai-control-retry`)；诊断卡逐项标出 ok/warn/fail | 档案/密钥为空时显示引导文案 | 诊断只读本地配置，不发送网络请求；配置导出不含密钥，导入拒绝密钥字段并保留本机仓库覆盖 |
| **AiUsageCard** | `settings.ai.control.loading` | `role=alert` 错误文本 + 刷新重试 | `ai.usage.empty` | 按仓库筛选仅影响调用/Token/成本，预算保持全局口径 |
| **AiChatModal** | 流式回答占位与 `ai.summarizing` | 单条消息 `role=alert` 错误，不丢弃历史 | 未发送时为空对话区 | 推理与最终答案分离，取消后保留已生成内容；附加当前文件需显式勾选，限 16 KB，默认关闭，拒绝越界路径/符号链接/二进制 |
| **AiReviewModal** | `ai.summarizing` 流式文本 | 错误块 + 重新生成 | `ai.review.empty` | 推理面板独立折叠，不混入审查结果 |
| **AiReleaseNotesModal** | 流式发布说明占位 | 错误块 + 重新生成 | 空范围提示 | 推理面板独立折叠 |
| **AiRepoOverrideModal** | `settings.ai.control.loading` | `role=alert` 错误文本 + 保存失败 Toast | 未配置覆盖时继承全局值 | 稀疏覆盖，空字段不重置全局配置 |

---

## 3. 防静默失败守卫规则

1. **禁止裸吞异常**：组件内所有 `async function load*()` / `refresh*()` 在 `catch` 块中必须满足以下至少一项：
   - 赋值至响应式 `error` 并在模板中渲染；
   - 调用全局 `reportError(scope, err)` 或 `notify(msg, { tone: "danger" })`；
   - 降级至明确标记的 Fallback 策略（如 `console.debug("fallback to heuristics", err)`）。
2. **三态模板顺序**：
   ```vue
   <div v-if="isLoading">...</div>
   <div v-else-if="error">...</div>
   <div v-else-if="items.length === 0">...</div>
   <div v-else>...</div>
   ```

## 4. 运行时反馈与全局兜底

- **剪贴板**：统一走 `utils/clipboard.ts` 的 `writeClipboardText`；成功返回 `null`，失败返回错误消息，由调用方展示 `common.copy.failed`，禁止裸 `navigator.clipboard.writeText` 后直接报“已复制”。
- **Vue 异常**：`main.ts` 的 `app.config.errorHandler` 除 console 外写入本地崩溃日志（scope=`vue`，附 `info` 与组件名），可从崩溃报告导出。
- **外链与表格**：`target="_blank"` 必须带 `rel="noopener noreferrer"`；`<table>` 需 `caption` 或 `aria-label`，表头 `<th>` 需 `scope`，由 `markupSemantics` 守卫锁定。
- **可点击自定义元素**：真实动作优先用 `<button>`；确需 `div/li/tr` 承载时必须有 `role`、`tabindex`、Enter/Space 处理、焦点环与选中语义，由 `clickableSurfaces` 守卫关键路径。
- **模态焦点与滚动**：`useModalA11y` 统一锁定 body 滚动（嵌套计数，关闭/卸载恢复原 overflow）；无焦点元素时聚焦对话框容器；关闭后焦点回到打开前元素。由 `modalScrollLock` / `modalFocusFallback` 守卫。
- **Toast 计时**：危险态使用 `role=alert`/`assertive`，其余 `role=status`/`polite`；悬停或聚焦暂停倒计时，离开后按剩余时间恢复，满足 WCAG 2.2.1。
- **快捷键输入边界**：`isComposing` 期间不触发任何应用快捷键；`?` 不作用于 input/textarea/select/contenteditable；终端内 Cmd/Ctrl+K 归 xterm（清屏），不再同时打开命令面板。
- **下拉披露状态**：项目/分支/上下文/更多四个顶栏触发器暴露 `aria-expanded`；Escape 关闭菜单并把焦点还给触发按钮。
- **表单可访问名**：可见 `<label>` 必须用 `for/id` 关联或为控件补 `aria-label`；设置页、发布向导、Studio 运维与 Sparse/Reflog/Bisect 的关键输入已覆盖，并由 `formControlNames` / `searchControlNames` 锁定。
- **后台轮询**：RepoDashboard 自动刷新在 `document.visibilityState !== "visible"` 时跳过 tick；组件卸载时清定时器与一次性 timer。
