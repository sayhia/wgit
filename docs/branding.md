# wgit 品牌资源

图形由 W、两条分支路径和三个提交节点组成。沿用设计系统颜色：主蓝 `#3D63DD`、浅蓝 `#93B4FF`、墨黑 `#111113`。

## 入口

- `frontend/public/brand/`：应用图标、独立标识、字标及深色版本；SVG 字标已转路径。
- `ActivityDock.vue`、`RepoOnboarding.vue` 和 `frontend/index.html`：侧栏、欢迎页、启动画面与 favicon 使用同一个 `wgit-icon.svg`。
- `build/appicon.png`：1024px 桌面图标，供 Windows、Linux 及通用打包任务使用。
- `resources/icon.png`：与 `build/appicon.png` 相同，通过 Go embed 传给 Wails 应用图标选项。
- `build/appicon.icon/`：macOS Icon Composer 源；蓝色背景与 `Assets/wgit-mark.svg` 前景分别保存，由系统应用平台遮罩。
- `build/darwin/Assets.car`、`build/darwin/icons.icns`、`build/windows/icon.ico`：生成的原生图标。
- `build/darwin/dmg-background.svg` / `.png`、`dmg-file-icon.png` / `.icns`：安装盘背景与图标。

## 更新

调整图形时同步上述矢量与 PNG 源，再从项目根目录运行：

```sh
wails3 task common:generate:icons
```

macOS 的 Icon Composer 编译需要支持它的 Xcode 工具链。更新 macOS 图标时同时提交新的 `Assets.car` 和 `icons.icns`，以免应用包继续使用旧资源。

前端构建会自动将 `frontend/public/brand/` 复制到 `frontend/dist/brand/`，随后嵌入桌面程序。
