# 发布清单（macOS / Windows）

> 目标：产物可被 Gatekeeper / SmartScreen 接受、可被用户卸载干净、失败可定位。当前状态见每节末尾的「现状」。

## 1. 构建

打包只在 GitHub Actions 的 `.github/workflows/release.yml` 中执行：macOS 走 `macos` 作业，Windows 走 `windows` 作业；本地不再提供 `scripts/package-macos.sh`，也不再提供 `wails3 task package:macos` 入口。

- [x] 版本号唯一来源：`build/config.yml` 的 `info.version`；tag 推送时 tag 必须与 config 一致，main 推送在对应 tag 不存在时自动发布并使用 config 版本；同一个字符串同时进 `-X main.AppVersion`、`CFBundleShortVersionString`、`CFBundleVersion`，并在 Windows 作业里写入 Win32 版本资源（`build/windows/info.json` 由 CI 按 config 生成）
- [x] 发布包用 `-tags production`（否则是 dev 壳，前端资源不在二进制里）；`go test ./...` 在 CI 通过
- [x] 前端 `bun run build` 通过，入口 chunk < 700 kB（CI 有预算门禁）
- [x] `release=true`（tag 或 main 版本首次出现）为 `arm64 amd64` → `lipo` 通用包；其余 main 推送只冒烟 arm64
- [x] Windows：每次推送都构建 `amd64` / `arm64` 两个二进制与两个独立 NSIS 安装包，不再产出 `amd64_arm64` 通用包

现状：tag 或 main 上版本号首次出现时产出 universal DMG、Windows amd64/arm64 两个 installer；其余 main 推送产出 arm64 DMG 与同样的两个 Windows installer 冒烟 artifact；构建、`.app` 组装、DMG 创建、Windows NSIS 打包与校验全部由 release workflow 内联步骤完成。

## 2. 签名与公证

签名与公证全部内联在 macOS 作业的 `Sign, notarize, and package DMG` 步骤中；不再保留本地打包或签名脚本。

- [x] `build/darwin/entitlements.plist` 只声明 `network.client` + `network.server`（Wails 官方 production 预设），**没有** `com.apple.security.cs.disable-library-validation`、`get-task-allow` 与 JIT 豁免
- [x] 顺序固定：签 app → 公证 → staple app → 用已 staple 的 app 生成 DMG → 签 DMG → 公证 → staple DMG → `spctl` 闸门。旧脚本先做 DMG 再签名的写法会让分发出去的 DMG 永远是未签名的
- [x] 不再使用废弃的 `codesign --deep`，改为先签 `Contents/MacOS/*` 再签 bundle
- [ ] 在一台**没有开发者证书**的机器上首次启动验证
- [ ] 配置 Apple 证书后跑一次真实公证（当前只在 `APPLE_SIGNING_IDENTITY` 为空的 ad-hoc 分支上验证过）

CI secrets：`APPLE_SIGNING_IDENTITY`、`APPLE_CERT_P12_BASE64`、`APPLE_CERT_PASSWORD`、`APPLE_API_KEY_P8`、`APPLE_API_KEY_ID`、`APPLE_API_ISSUER`（或工作站上的 `APPLE_NOTARY_PROFILE`）。缺任一项时脚本走 ad-hoc 分支并在产物里打印 `[UNSIGNED]`，`dist/signature-status.txt` 会随产物一起上传。

现状：未配置证书 → 产物 ad-hoc 签名、`codesign --verify --strict` 通过、无公证票（`stapler validate` 明确报缺），Gatekeeper 仍会拦。

Windows 安装包当前不做 Authenticode 签名：CI 没有配置代码签名证书，Release 正文会明确标注 “not Authenticode-signed; SmartScreen may warn”。配置证书后应在 `windows` 作业里给 `bin/*.exe` 与 installer 增加 `signtool` 签名步骤，并同样加失败门禁。


## 3. 卸载与残留

应用写入的位置只有两处：

| 位置 | 内容 | 卸载 |
| --- | --- | --- |
| `~/.config/wgit/config.json` / `%USERPROFILE%\.config\wgit\config.json` | 最近仓库、Feature Flag、AI/访问配置、会话 | 删除目录 |
| `~/Library/Application Support/wgit/`（macOS） | 会话、预置、缓存 | 删除目录 |
| `NSUserDefaults` / `localStorage`（WKWebView / WebView2） | 主题、强调色、窗口几何、语言 | 删除应用即随容器清理 |

- [ ] 首次启动不写任何仓库外的文件
- [ ] 不安装后台服务、不注册 launchd / Windows 服务
- [ ] 卸载文档说明上述位置；NSIS 卸载器会删除安装目录，并尝试清理 WebView2 数据目录（首次实测时确认实际路径）

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
open ~/Downloads/wgit-<version>-darwin-universal.dmg  # 从 CI Release 下载
```

- [ ] 深浅两套主题 + 一个非默认强调色，各看一个主视图
- [ ] 顶栏可拖动，双击可缩放（窗口拖拽是手工实现，见 `useWindowChrome`）
- [ ] 中文 / 英文各启动一次，启动屏文案正确
- [ ] 打开一个 10 万提交的仓库，提交列表滚动无卡顿
- [ ] 在未装 WebView2 的 Windows 10/11 上分别跑 `wgit-<version>-windows-amd64-installer.exe`（x64）与 `wgit-<version>-windows-arm64-installer.exe`（ARM64，或用 ARM64 虚拟机），确认引导安装、桌面/开始菜单快捷方式与卸载都正常

现状：视觉基线与浏览器审计在每次迭代都会本地跑；最后一项缺一个稳定的基准仓库，见 `docs/roadmap-next-2026-10.md` 的 P5.1。

## 6. CI 与打包链路

| 工作流 | 触发 | 内容 |
| --- | --- | --- |
| `.github/workflows/ci.yml` | PR、main push、每日 03:20 UTC、手动 | shellcheck/actionlint 自检、Go 服务层（vet/test/覆盖率 ≥77）、前端（typecheck → build → 预算门禁 → 测试/覆盖率 ≥77）、macOS 上的 AI E2E、夜间浏览器审计 |
| `.github/workflows/release.yml` | main push、`v*` tag、手动 | 先跑 `verify`（shellcheck/actionlint、Go vet/test、前端 typecheck/build/test）→ macOS DMG（release=true 出通用包，其余 main 只冒烟 arm64）、Windows NSIS installers（amd64、arm64 两个独立包，release=true 时随 Release 发布）、Linux server 二进制 amd64/arm64 tarball → release=true 时逐产物校验并创建/更新 GitHub Release（`sha256sums.txt` + `build-metadata.txt` + `signature-status.txt`） |

要点：

- 打包前先验证：`release.yml` 的 `verify` 作业在 macOS/Linux/Windows 打包之前执行 shellcheck、actionlint、Go vet/test、前端 typecheck/build/test；测试不绿不会产出 DMG、server tarball 或 Windows installer。`verify` 失败时 `macos`/`server`/`windows` 被跳过，tag 也不会创建 Release。
- 校验和可移植：server 与 release 作业统一用 Ubuntu 自带的 `sha256sum`（不再依赖可选的 Perl `shasum`）；macOS 作业继续用系统自带 `shasum -a 256`，Windows 作业走 Git Bash 自带的 `sha256sum`。下载 artifact 后先逐个校验各作业自带的 `sha256sums.txt`，再合并生成全局清单，避免同名文件在 merge 时互相覆盖。CI 的 DMG 步骤重建时截断旧 `sha256sums.txt`，不会把上一版 DMG 的哈希留在清单里。
- Windows 工具链：`windows-latest` 镜像不带 NSIS，`windows` 作业在缺失时通过 Chocolatey 安装；Wails CLI 用 `go install` 安装到与 `go.mod` 完全相同的版本，避免 syso 生成器与运行时版本漂移；`build/windows/info.json` 在 CI 里按 `build/config.yml` 重新生成，Explorer 里的文件版本不会停在模板默认值 `0.1.0`。两个架构各自调用一次 `makensis`，产物分别是 `wgit-<version>-windows-amd64-installer.exe` 与 `wgit-<version>-windows-arm64-installer.exe`。
- 幂等发布：同一 tag 重跑 Release 时，若 release 已存在则 `gh release upload --clobber` 后 `release edit` 更新说明，不会因 "release already exists" 失败；`GH_REPO` 显式传入，release 作业无需 checkout 也能正确定位仓库。
- 签名门禁：`APPLE_SIGNING_IDENTITY` 已配置时，macOS 作业必须通过 `xcrun stapler validate` 与 `spctl -a -vvv -t install`，否则打包失败；未配置证书时仍走 ad-hoc 分支并在 `signature-status.txt` 标注不可分发。Windows installer 当前未做 Authenticode 签名，Release 正文会注明 SmartScreen 可能告警。
- 发布元数据：Release 附件包含 `build-metadata.txt`（tag/commit/run URL）与 `signature-status.txt`，正文同时内嵌签名状态与构建元数据。
- Bun 依赖缓存：`verify`、`macos`、`windows`、`server` 作业按 `frontend/bun.lock` 缓存 `~/.bun/install/cache`，减少重复安装。
- 自动发布：main 推送时 `version` 作业读取 `build/config.yml`；若 `v<version>` tag 尚不存在，则 `release=true`，打包 universal DMG + Windows amd64/arm64 installers + server tarball，并由 release 作业在本次提交上创建 tag 与 GitHub Release。同一版本后续 main 推送只产冒烟 artifact；手动推 tag 仍然支持。
- 版本一致性：`v*` tag 的版本号必须等于 `build/config.yml` 的 `info.version`，否则 release.yml 的 version 作业直接失败——`About` 读的是 config 里的值，两者不一致就是在骗用户。Windows 作业还要求版本能取出三段数字（`1.2.3`）作为 Win32 文件版本，否则直接失败。
- 产物门禁：上传前跑 `sha256sum -c sha256sums.txt`（macOS 作业用 `shasum -a 256 -c`）自检清单，并限制 DMG ≤ 40 MB（实测通用包 16.7 MB）、Windows installer ≤ 80 MB（实测单架构约 11 MB）。超预算通常是资源被重复塞进 bundle，而不是应用真的长大了。Release 装配要求恰好 1 个 DMG、2 个 server tarball、Windows amd64/arm64 各 1 个 installer。
- 作业自检：`shellcheck -S info scripts/*.sh` 与 `actionlint` 都是阻断门禁，且都在本地跑过同一个版本（actionlint v1.7.7 走 `go run`，0 findings；全部 `run:` 块按 actionlint 的替换规则抽出来单独喂给 shellcheck `-S style`，0 findings）。不用容器镜像跑 lint：runner 的匿名 Docker Hub 拉取会限流，ghcr 路径这里还直接 denied，一个纯检查步骤因此变红只会让人学会忽略红灯。
- 2026-11-01 补强：新增 `verify` 前置作业、分目录 artifact 下载与逐包校验、`sha256sum` 可移植化、Bun 缓存、签名已配置时的公证硬门禁；删除 `scripts/package-macos.sh` 与 `scripts/sign-and-notarize.sh`，macOS 构建/打包/签名全部内联进 release workflow；新增 `windows` 作业（Wails CLI 与 `go.mod` 同版本、Chocolatey 安装 NSIS、CI 生成 `build/windows/info.json`、release 出 amd64/arm64 两个独立 NSIS installer）；本地 `actionlint` 通过，待下一次 tag/main 推送在 GitHub 上实跑验证。
- 打包入口仅 GitHub Actions：`release.yml` 的 macOS 作业内联完成构建、`.app` 组装、签名/公证、DMG 创建与校验；`scripts/package-macos.sh`、`scripts/sign-and-notarize.sh` 均已删除，`Taskfile.yml` 的 `package:macos` 也已移除。
- 发版顺序：在 main 上把 `build/config.yml` 的 `info.version` 改到目标版本并推送 → 若 `vX.Y.Z` 不存在，release.yml 自动打 tag 并创建 GitHub Release；也可手动推 `vX.Y.Z` annotated tag。版本对不上时 version 作业会先失败，不会产出一个「关于」页说谎的包。
- Go 任务跑在 `-tags server` + `CGO_ENABLED=0` 上：`main.go` 的 `//go:embed all:frontend/dist` 在干净检出下必须有一个 dist 目录（作业先 `touch` 占位）；而 Wails v3 beta 的 Linux 分支即使带 `-tags server` 仍会去 pkg-config 找 `gtk4/webkitgtk-6.0/libsoup-3.0`，只有关掉 CGO 才和 `Dockerfile.server` 一样退化成纯 Go。desktop 壳的编译由 release.yml 的 macOS 打包作业覆盖，两边合起来仍是全量编译。
- 模态滚动锁以 DOM 为准自我纠正（`useModalA11y`）：bun 把所有测试文件跑在同一个进程里，模块级计数会被前一个文件遗弃的打开态模态抬高；Linux runner 的文件顺序与本机不同，锁就会永久跳过。现在计数 >0 而 body 不是 `hidden` 时判为陈旧并归零，`tests/modalScrollLock.test.ts` 里留了一条对应的回归测试。
- 前端作业先 `bun run build` 再 `bun test`：`tests/finalDeliveryVerification.test.ts` 断言生产包已生成，预算门禁也要读 `dist/assets/index-*.js` 的体积。
- AI E2E 在 CI 里带 `AI_VISUAL=off`：像素基线是在某一台机器的字体光栅化上拍的，流水线只跑功能断言并上传截图，比对留在本地。
- 覆盖率下限来自实测：Go 78.0% statements、前端 78.38% lines（此前 CI 里写的 78%/80% 是从未真正跑绿过的估计值）。
- 历史背景：CI 自建起 30+ 次运行全红，原因是 embed 缺 dist、finalDelivery 早于 build、`componentMount.test.ts`/`aiChatFileContext.test.ts` 泄漏 `useModalA11y` 的滚动锁计数（bun 单进程共享模块状态，文件顺序随文件系统变，Linux runner 上恰好排在 `modalScrollLock.test.ts` 之前）、bundle budget 步骤用了 root 目录 + GNU `stat -c%s`、以及 `audit` job 判断了一个不存在的 `schedule` 触发器。
- 首跑（2026-11-01）：Release artifacts 全绿并落三个产物（macOS DMG 9.7 MB、server linux/amd64 7.6 MB、linux/arm64 7.0 MB），GitHub Release 如预期 skip。CI 暴露两处：Linux 上即使 `-tags server` 仍会去找 GTK/WebKit 的 pkg-config（加 `CGO_ENABLED=0` 解决，与 `Dockerfile.server` 一致）；以及第三个文件在 Linux 的文件顺序下同样泄漏滚动锁——因此把锁改成以 DOM 为准自我纠正，不再靠「找出谁泄漏」。
