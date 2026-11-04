# Fyne Windows / Android 原型

这个入口验证同一套 Go 业务和 Fyne 界面在 Windows 与 Android 上的可行性。原型与仓库根目录的 MyGo 程序是两个独立入口；运行原型使用 `go run ./cmd/byr-fyne`。

默认使用离线论坛样例和本地模拟 AI 文本流，包含中文图文、分页回复、引用折叠和千楼压力数据。AI 页可以手动配置真实模型进行流式调用；界面会明确区分模拟模式与真实模型。真实模型调用的论坛工具仍读取离线样例，API Key 仅保留在内存中。真实 BYR 账号登录不在这个原型的验证范围内，原型不读取原客户端保存的设置、账号或 Cookie。

原型内嵌 Noto Sans SC 中文字体；进入「验证 → 字体开源许可」可以查看随应用携带的完整字体许可证，源文件和许可说明位于 `internal/fyneui/fonts/`。

## 统一构建入口

从仓库目录执行：

```sh
# 检查 Go、C 编译器、Android SDK / NDK / Java 与已连接设备
go run ./tools/fynebuild -target check

# Windows x64 桌面程序
go run ./tools/fynebuild -target windows

# Android ARM64，面向常见真机
go run ./tools/fynebuild -target android

# Android x86_64，面向模拟器
go run ./tools/fynebuild -target android -arch amd64
```

产物：

```text
dist/fyne/windows/BYR-Fyne.exe
dist/fyne/android/BYR-Fyne-arm64.apk
dist/fyne/android/BYR-Fyne-amd64.apk
```

Android APK 是用于原型验证的调试包，不是应用商店发布包；原型包名为 `com.byrclient.fyneprototype`。不要把“APK 构建成功”记为“真机验证通过”。

构建入口锁定 Fyne CLI `fyne.io/tools/cmd/fyne@v1.7.4`，Fyne 界面库的版本以仓库 `go.mod` 为准。首次 Android 构建会下载 CLI 及其依赖。脚本不要求先全局安装 `fyne`，不修改全局 Go 或 Android 环境配置。

如果当前 Go 模块代理返回 403 或超时，可以只对本次构建指定备用代理：

```sh
go run ./tools/fynebuild -target android -goproxy https://goproxy.cn,https://goproxy.io,direct
```

## 构建环境

Windows 需要 Go 和 PATH 中可用的 C 编译器（默认 `gcc`，可通过 `CC` 指定）。Windows 产物在 Windows 构建节点生成。

Android 需要 JDK、Android SDK platforms / build-tools 和 NDK。默认使用 NDK `28.2.13676358`；可使用 SDK Manager 安装 `ndk;28.2.13676358`，也可显式指定已经验证的 NDK 路径。

SDK 查找顺序包括 `ANDROID_HOME`、`ANDROID_SDK_ROOT`、PATH 中 `adb` 所在的 SDK，以及常见用户 SDK 目录。多个 SDK 同时存在时，优先使用包含默认 NDK 的目录。也可以明确传入：

```sh
go run ./tools/fynebuild -target android -sdk D:/Android/Sdk -ndk D:/Android/Sdk/ndk/28.2.13676358
```

构建脚本内部会进入 `cmd/byr-fyne` 再调用 `fyne package`，因为该版本 Fyne 的移动打包不支持 `--source-dir`。打包结束后，APK 会被整理到 `dist/fyne/android`。不要同时执行两个 Android 构建，它们共用 Fyne 的临时输出文件。

## Android 安装与验证

使用已授权的 USB 调试设备或已经启动的 x86_64 Android 模拟器：

```sh
adb devices -l

# 真机
adb install -r dist/fyne/android/BYR-Fyne-arm64.apk

# 模拟器，两条 install 命令按实际设备二选一
adb install -r dist/fyne/android/BYR-Fyne-amd64.apk

adb shell monkey -p com.byrclient.fyneprototype -c android.intent.category.LAUNCHER 1
```

多设备连接时，给每条 `adb` 命令增加 `-s <设备序列号>`。原型不自动覆盖、删除或重置用户已有模拟器。

检查项目：

- 中文标题、长段落、链接、图片、引用与回复展开是否正常。
- 版面切换、主题分页与窄窗口返回操作是否保留正确状态。
- 中文输入与软键盘弹出后，输入区是否仍可见。
- AI 增量展示、停止和停止后的已收到文本是否正常。
- 连续打开多图帖、返回列表、切换前后台后，是否出现崩溃或明显卡顿。

开发期自动化 UI 测试、桌面软件渲染截图和移动模拟尺寸可验证布局；真实 Android APK 启动、触摸、中文输入和后台恢复仍需设备或模拟器验证。

## Windows 自动验证

设置 `BYR_FYNE_VERIFY_DIR` 后启动原型，会在真实 Fyne 窗口中依次检查初始读帖、图片解码、千楼分页、引用折叠、窄布局导航、搜索与模拟 AI 停止，并保存六张截图和 `report.json` 后自动退出。

PowerShell 示例：

```powershell
$env:BYR_FYNE_VERIFY_DIR = Join-Path (Get-Location) 'dist/fyne/verification'
& './dist/fyne/windows/BYR-Fyne.exe'
Remove-Item Env:BYR_FYNE_VERIFY_DIR
```

核对 `report.json` 中的 `checks` 和可选 `error` 字段；不要仅凭进程退出就判为通过。报告里的窄窗口截图属于 Windows 布局验证，不能替代 Android 执行；Go heap 样本也不是整个进程的内存占用或滚动帧率基准。

## 当前机器的构建准备记录

2026-11-04 核查：Go `1.27.1`、Windows GCC、JDK 21 可用；Android SDK 已安装 platform 35 / 36.1、NDK 28.2.13676358 和 build-tools。Android Emulator 的 WHPX 加速检查通过。初始 `adb devices -l` 无连接设备，初始无可用 AVD 或系统镜像。

已尝试准备 Android 35 默认 x86_64 镜像（官方压缩包 782,404,023 字节）；当前网络下载速度不足，已停止下载，未创建或启动模拟器。因此本轮不能将 Android 真机、触摸、软键盘或后台行为标记为通过。

同日完成 Android 两种架构的真实交叉编译：ARM64 APK 约 37.7 MiB，x86_64 APK 约 39.4 MiB。已使用 Android SDK 的 `aapt` 检查包名、启动 Activity 与 ABI，使用 `apksigner verify --verbose` 校验调试签名，均通过。工具生成的调试包 min SDK 为 21、target SDK 为 29，不作为应用商店发布配置。

Windows x64 程序约 32.5 MiB，已在真实 Fyne 窗口中运行，并通过九项自动检查；宽屏与 360 DIP 窄布局共六张截图已人工查看。结果在 `dist/fyne/verification/report.json` 和同目录 PNG 中。这里的输入操作由应用内部验证逻辑驱动，尚未验证操作系统级中文输入法与触控，也未进行 FPS、耗电或进程内存性能基准测试。

`dist/fyne/android/build-report.json` 保存 Android 产物大小、SHA-256、工具版本与明确的 `runtimeVerified: false`。
