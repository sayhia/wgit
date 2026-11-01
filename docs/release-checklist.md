# 发布清单（macOS 首发）

> 目标：产物可被 Gatekeeper 接受、可被用户卸载干净、失败可定位。当前状态见每节末尾的「现状」。

## 1. 构建

```bash
bash scripts/package-macos.sh        # 与 CI 完全同一条命令
ls -lh dist/*.dmg
```

- [x] 版本号唯一来源：tag（`v1.2.3` → `1.2.3`），无 tag 时回落 `build/config.yml` 的 `info.version` + 短 SHA；同一个字符串同时进 `-X main.AppVersion`、`CFBundleShortVersionString`、`CFBundleVersion`
- [x] 发布包用 `-tags production`（否则是 dev 壳，前端资源不在二进制里）；`go test ./...` 在 CI 通过
- [x] 前端 `bun run build` 通过，入口 chunk < 700 kB（CI 有预算门禁）
- [ ] `ARCHS` 默认 `arm64 amd64` → `lipo` 通用包；main 推送只冒烟 arm64

现状：本地已产出 `dist/wgit-1.0.0-dev.<sha>-darwin-universal.dmg`（16 MB，`lipo -info` 为 x86_64 + arm64），挂载后含 `wgit.app` 与 `Applications` 别名，`hdiutil verify` 通过，二进制内确认嵌入 `assets/index-*.js`，`sha256sums.txt` 一并生成。

## 2. 签名与公证

```bash
# 由 package-macos.sh 按顺序调用；也可单独对已有产物执行
bash scripts/sign-and-notarize.sh bin/wgit.app
bash scripts/sign-and-notarize.sh dist/wgit-<version>-darwin-universal.dmg
```

- [x] `build/darwin/entitlements.plist` 只声明 `network.client` + `network.server`（Wails 官方 production 预设），**没有** `com.apple.security.cs.disable-library-validation`、`get-task-allow` 与 JIT 豁免
- [x] 顺序固定：签 app → 公证 → staple app → 用已 staple 的 app 生成 DMG → 签 DMG → 公证 → staple DMG → `spctl` 闸门。旧脚本先做 DMG 再签名的写法会让分发出去的 DMG 永远是未签名的
- [x] 不再使用废弃的 `codesign --deep`，改为先签 `Contents/MacOS/*` 再签 bundle
- [ ] 在一台**没有开发者证书**的机器上首次启动验证
- [ ] 配置 Apple 证书后跑一次真实公证（当前只在 `APPLE_SIGNING_IDENTITY` 为空的 ad-hoc 分支上验证过）

CI secrets：`APPLE_SIGNING_IDENTITY`、`APPLE_CERT_P12_BASE64`、`APPLE_CERT_PASSWORD`、`APPLE_API_KEY_P8`、`APPLE_API_KEY_ID`、`APPLE_API_ISSUER`（或工作站上的 `APPLE_NOTARY_PROFILE`）。缺任一项时脚本走 ad-hoc 分支并在产物里打印 `[UNSIGNED]`，`dist/signature-status.txt` 会随产物一起上传。

现状：未配置证书 → 产物 ad-hoc 签名、`codesign --verify --strict` 通过、无公证票（`stapler validate` 明确报缺），Gatekeeper 仍会拦。


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

自动化部分已经全部进 CI（见第 6 节），发布前只需要人工确认这些机器测不到的项：

```bash
cd frontend && AI_VISUAL=off bun run audit:ai   # 真实 server + 视觉基线（基线是本机字体光栅化产物，必须在本地比）
cd frontend && bun run shots:check              # 状态截图基线
bash scripts/package-macos.sh && open dist/*.dmg
```

- [ ] 深浅两套主题 + 一个非默认强调色，各看一个主视图
- [ ] 顶栏可拖动，双击可缩放（窗口拖拽是手工实现，见 `useWindowChrome`）
- [ ] 中文 / 英文各启动一次，启动屏文案正确
- [ ] 打开一个 10 万提交的仓库，提交列表滚动无卡顿

现状：视觉基线与浏览器审计在每次迭代都会本地跑；最后一项缺一个稳定的基准仓库，见 `docs/roadmap-next-2026-10.md` 的 P5.1。

## 6. CI 与打包链路

| 工作流 | 触发 | 内容 |
| --- | --- | --- |
| `.github/workflows/ci.yml` | PR、main push、每日 03:20 UTC、手动 | shellcheck/actionlint 自检、Go 服务层（vet/test/覆盖率 ≥77）、前端（typecheck → build → 预算门禁 → 测试/覆盖率 ≥77）、macOS 上的 AI E2E、夜间浏览器审计 |
| `.github/workflows/release.yml` | main push、`v*` tag、手动 | macOS DMG（tag 出通用包，main 只冒烟 arm64）、Linux server 二进制 amd64/arm64 tarball、tag 时创建 GitHub Release + `sha256sums.txt` |

要点：

- 版本一致性：`v*` tag 的版本号必须等于 `build/config.yml` 的 `info.version`，否则 release.yml 的 version 作业直接失败——`About` 读的是 config 里的值，两者不一致就是在骗用户。
- 产物门禁：上传前跑 `shasum -a 256 -c sha256sums.txt` 自检清单，并限制 DMG ≤ 40 MB（实测通用包 16.7 MB）。超预算通常是资源被重复塞进 bundle，而不是应用真的长大了。
- 作业自检：`shellcheck -S info scripts/*.sh` 与 `actionlint` 都是阻断门禁，且都在本地跑过同一个版本（actionlint v1.7.7 走 `go run`，0 findings；30 个 `run:` 块按 actionlint 的替换规则抽出来单独喂给 shellcheck `-S style`，0 findings）。不用容器镜像跑 lint：runner 的匿名 Docker Hub 拉取会限流，ghcr 路径这里还直接 denied，一个纯检查步骤因此变红只会让人学会忽略红灯。
- 本地入口与 CI 完全同一条命令：`wails3 task package:macos`（等价于 `bash scripts/package-macos.sh`，可用 `ARCHS=arm64` / `SKIP_FRONTEND=1` 缩短冒烟时间）。
- 发版顺序：先在 main 上把 `build/config.yml` 的 `info.version` 改到目标版本 → 打 `vX.Y.Z`  annotated tag → 推 tag，release.yml 出通用 DMG + server tarball + GitHub Release。版本对不上时 version 作业会先失败，不会产出一个「关于」页说谎的包。
- Go 任务跑在 `-tags server` + `CGO_ENABLED=0` 上：`main.go` 的 `//go:embed all:frontend/dist` 在干净检出下必须有一个 dist 目录（作业先 `touch` 占位）；而 Wails v3 beta 的 Linux 分支即使带 `-tags server` 仍会去 pkg-config 找 `gtk4/webkitgtk-6.0/libsoup-3.0`，只有关掉 CGO 才和 `Dockerfile.server` 一样退化成纯 Go。desktop 壳的编译由 release.yml 的 macOS 打包作业覆盖，两边合起来仍是全量编译。
- 模态滚动锁以 DOM 为准自我纠正（`useModalA11y`）：bun 把所有测试文件跑在同一个进程里，模块级计数会被前一个文件遗弃的打开态模态抬高；Linux runner 的文件顺序与本机不同，锁就会永久跳过。现在计数 >0 而 body 不是 `hidden` 时判为陈旧并归零，`tests/modalScrollLock.test.ts` 里留了一条对应的回归测试。
- 前端作业先 `bun run build` 再 `bun test`：`tests/finalDeliveryVerification.test.ts` 断言生产包已生成，预算门禁也要读 `dist/assets/index-*.js` 的体积。
- AI E2E 在 CI 里带 `AI_VISUAL=off`：像素基线是在某一台机器的字体光栅化上拍的，流水线只跑功能断言并上传截图，比对留在本地。
- 覆盖率下限来自实测：Go 78.0% statements、前端 78.38% lines（此前 CI 里写的 78%/80% 是从未真正跑绿过的估计值）。
- 历史背景：CI 自建起 30+ 次运行全红，原因是 embed 缺 dist、finalDelivery 早于 build、`componentMount.test.ts`/`aiChatFileContext.test.ts` 泄漏 `useModalA11y` 的滚动锁计数（bun 单进程共享模块状态，文件顺序随文件系统变，Linux runner 上恰好排在 `modalScrollLock.test.ts` 之前）、bundle budget 步骤用了 root 目录 + GNU `stat -c%s`、以及 `audit` job 判断了一个不存在的 `schedule` 触发器。
- 首跑（2026-11-01）：Release artifacts 全绿并落三个产物（macOS DMG 9.7 MB、server linux/amd64 7.6 MB、linux/arm64 7.0 MB），GitHub Release 如预期 skip。CI 暴露两处：Linux 上即使 `-tags server` 仍会去找 GTK/WebKit 的 pkg-config（加 `CGO_ENABLED=0` 解决，与 `Dockerfile.server` 一致）；以及第三个文件在 Linux 的文件顺序下同样泄漏滚动锁——因此把锁改成以 DOM 为准自我纠正，不再靠「找出谁泄漏」。
