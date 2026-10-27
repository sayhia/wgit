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
