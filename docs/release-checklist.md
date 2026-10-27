# 发布清单（macOS 首发）

> 目标：产物可被 Gatekeeper 接受、可被用户卸载干净、失败可定位。当前状态见每节末尾的「现状」。

## 1. 构建

```bash
wails3 build            # Vite → go build -tags production -trimpath -ldflags="-w -s"
ls -la bin/wgit         # 单二进制；bin/wgit.app 是打包后的壳
```

- [ ] 版本号唯一来源：`build/config.yml` 的 `info.version`，构建脚本注入 `-X main.version=`
- [ ] `go build` 无 `-race`（发布包），`go test ./...` 在 CI 通过
- [ ] 前端 `bun run build` 通过，入口 chunk < 700 kB（CI 有预算门禁）

现状：`wails3 build` 可用，产物约 15.6 MB；版本号尚未注入，`About` 显示的是开发值。

## 2. 签名与公证

```bash
codesign --deep --force --options runtime \
  --sign "Developer ID Application: <Team>" bin/wgit.app
xcrun notarytool submit bin/wgit.zip \
  --keychain-profile "wgit-notary" --wait
xcrun stapler staple bin/wgit.app
spctl -a -vvv -t install bin/wgit.app     # 期望: accepted, source=Notarized Developer ID
```

- [ ] `entitlements.plist` 只声明必需项（网络、文件选择、通知），**不要** `com.apple.security.cs.disable-library-validation`
- [ ] 公证后 `stapler validate` 通过
- [ ] 在一台**没有开发者证书**的机器上首次启动验证

现状：未签名。分发前必须完成，否则用户看到「无法验证开发者」。

## 3. 卸载与残留

应用写入的位置只有两处：

| 位置 | 内容 | 卸载 |
| --- | --- | --- |
| `~/Library/Application Support/wgit/` | 会话、预置、缓存 | 删除目录 |
| `NSUserDefaults` / `localStorage`（WKWebView） | 主题、强调色、窗口几何、语言 | 删除应用即随容器清理 |

- [ ] 首次启动不写任何仓库外的文件
- [ ] 不安装后台服务、不注册 launchd
- [ ] 卸载文档说明上述两处

现状：符合（`wgit.window.v1` / `wgit.locale` / `wgit.active-tab` 等键都在 WebView 容器内）。

## 4. 升级

- [ ] 升级不覆盖用户主题、窗口位置、快捷键自定义
- [ ] 会话恢复失败时降级到欢迎页，而不是白屏
- [ ] `wgit://` deep link 在升级后仍然注册（`Info.plist` 的 `CFBundleURLTypes`）

现状：无自动更新；手动替换 .app 后设置保留（都在 WebView 容器内）。

## 5. 发布前 15 分钟自检

```bash
cd frontend && bun test tests && bun run typecheck && bun run build
cd .. && go vet ./... && go test ./...
cd frontend && bun run audit            # 需要 dev server + chromium
```

- [ ] 深浅两套主题 + 一个非默认强调色，各看一个主视图
- [ ] 顶栏可拖动，双击可缩放（窗口拖拽是手工实现，见 `useWindowChrome`）
- [ ] 中文 / 英文各启动一次，启动屏文案正确
- [ ] 打开一个 10 万提交的仓库，提交列表滚动无卡顿

现状：前四项在每次迭代都会跑；最后一项缺一个稳定的基准仓库，见 `docs/roadmap-next-2026-10.md` 的 P5.1。
