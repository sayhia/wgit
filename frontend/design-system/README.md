# wgit Design System · Workbench

当前视觉基调是中性的专业工作台：框架与阅读面使用实体底色；浮层保留轻量透明度。颜色、圆角和阴影由共享令牌定义。

## 文件与来源

| 文件 | 职责 |
|---|---|
| `tokens.css` | 185 个唯一 `--ug-*` 令牌名；包含深浅表面、文本、语义色、结构尺度与阴影 |
| `../src/styles/glass.css` | 当前材质：实体框架、96% / 98% 中性浮层、12px 模糊及可访问性降级 |
| `../src/style.css` | Tailwind 语义映射、统一圆角/阴影、源码与差异样式、设置表单原语 |
| `../src/accent-presets.css` | 六个强调色预设，只改变强调色族 |
| `DESIGN_SYSTEM.md` | 当前规范、组件用法、QA 与明确标注的历史迁移记录 |
| `index.html` | 本地交互式 Workbench 样式手册，复用当前令牌与浮层材质，包含主题切换、颜色复制和交互示例 |

## 当前材质

- `canvas` / `code` / `surface` / `inset`：实体内容面，不使用装饰渐变或模糊。
- `chrome`：顶栏、仓库栏和活动栏，实体底色，无阴影、光晕、指针高光或背景滤镜。
- 浮层：96% 中性 `overlay`（弹窗）或 98%（菜单），统一 12px 模糊；圆角和阴影取对应令牌。
- 旧 `glass-*` 类与变量是兼容接口，名称不代表继续使用 Liquid Glass 的反光、极光或指针效果。
- 降低透明度、高对比度与不支持模糊时使用实体表面；forced-colors 使用系统色。

默认画布深色为 `#181a1e`、浅色为 `#f4f5f7`。设置 → 外观可选择深色、浅色或跟随系统；偏好保存至 `localStorage["wgit-theme"]`，启动前同步 `data-theme` 与 `light` / `dark` 类。

## 本地查看与验证

从项目根目录启动样式手册：

```bash
cd frontend
python3 -m http.server 8080
# 打开 http://127.0.0.1:8080/design-system/index.html
```

验证实际应用时，在 `frontend/` 启动开发服务器，再运行对应检查：

```bash
bun run dev
# 另一个终端，在 frontend/ 中运行：
bun test tests/contrast.test.ts tests/glass.test.ts
bun run audit
```

对比度表中的静态值按当前默认蓝色令牌计算。全部强调色、悬停态和浮层仍需用测试与浏览器验证；旧版验收数字不作为当前结果。

## 约束

- 新颜色必须同时提供深浅值；组件使用语义令牌，不写硬编码面板颜色。
- 实心操作配对应 `text-*-contrast`；正文与普通控件保持中性。
- 圆角使用统一尺度，`rounded-2xl` 为 12px；`rounded-full` 保留状态点与圆形语义。
- 内容与框架不加玻璃；浮层只声明一层材质，不叠加 blur、渐变或彩色阴影。
- 修改令牌后检查深浅主题、六个强调色、焦点、减少透明度和强制配色。

## 历史验收

v1.2 的样式手册曾记录 17 个章节、132 项色阶组合检查，最低对比度 4.52:1。这些数字属于旧调色板和旧样式；本轮 Workbench 的验收以当前命令输出为准。
