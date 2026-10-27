# e2e —— 需要真实渲染器的检查

单元测试跑在 Bun 里，没有布局、没有合成、没有字体度量。下面三件事只能在浏览器里量，所以单独放在这里。

## 运行

```bash
cd frontend
bun run dev --port 9245        # 另开一个终端
bun run audit                  # smoke + contrast
```

默认打 `http://127.0.0.1:9245/`，用 `BASE_URL` 指向别处：

```bash
BASE_URL=http://127.0.0.1:4173/ bun run audit

bun run shots          # 更新基线（e2e/baselines/*.png）
bun run shots:check    # 与基线比对，有差异则退出码 1
```

退出码非 0 表示**发现了问题**，可以直接当 CI 门禁。

## 三个脚本

| 脚本 | 查什么 | 为什么不能进单元测试 |
| --- | --- | --- |
| `smoke.cjs` | 两种语言走遍 7 个主视图：未捕获异常、console error、i18n 缺 key、英文界面残留中文 | 要真实挂载、真实切换 locale |
| `contrast.cjs` | 12 组合（2 主题 × 6 强调色）× 6 视图，逐元素合成半透明背景后算对比度 | 依赖计算样式与合成结果 |
| `glass-pixels.cjs` | 玻璃表面按**渲染像素**取最亮/最暗值，再和墨色 token 比 | `backdrop-filter` 让背景无法从 CSS 推算 |
| `shots.cjs` | 12 组合（2 主题 × 6 强调色）的整屏截图，`--check` 时与 `baselines/` 逐字节比对 | 布局、材质、调色板这类变化只有像素能证明 |

`glass-pixels.cjs` 需要 `pngjs`（`bun add -d pngjs`）与 `playwright` 的 chromium。

## 两个测量陷阱（都踩过）

1. **切主题后立刻测 = 读到插值中间色。** 颜色有 transition，120ms 后取到的是「白 → 深青」的 92% 中间值，会凭空报出 4.39。脚本先注入 `transition: none !important`。
2. **导航失败会伪装成「全部通过」。** 点错按钮时后面每个组合都在测同一页，0 失败是假的。深度脚本因此记录每步 `textNodes`，用 `distinctLayouts` 证明真的走了 26 种布局。

## 产物

`e2e/out/` 存放失败截图，已加入 `.gitignore`。
