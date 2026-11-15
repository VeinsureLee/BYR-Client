# 平台细节与软件形式

## 平台能力分类

| 平台 | Dart 界面层 | 原生入口与配置 | 本机构建依赖 | 可用软件 |
| --- | --- | --- | --- | --- |
| Android | `platforms/mobile/`、`platforms/android/` + 共用功能 | `android/app/src/main/`、Gradle、Manifest、浅/深色样式 | Flutter、Android SDK 36、Build Tools 36.0.0、兼容 JDK | APK，安装到设备后运行 |
| Windows x64 | `platforms/desktop/` + 共用功能 | `windows/runner/`、CMake、资源和窗口定义 | Windows、Visual Studio C++ 桌面工具链 | 完整 `app/` 目录及 ZIP，运行 `app/byr_client.exe` |
| macOS | `platforms/desktop/` + 共用功能 | `macos/Runner/`、Xcode 工程、entitlements | macOS、Xcode | `.app` 应用包及 ZIP |
| Linux | `platforms/desktop/` + 共用功能 | `linux/runner/`、CMake、GTK 入口 | Linux、Clang、CMake、Ninja、pkg-config、GTK 开发库 | 完整 `app/` 目录及 tar.gz，运行 `app/byr_client` |

四端共用网络 API、模型、账号会话、收藏、历史、草稿、正文解析和功能页面。
文件选择、本地偏好、剪贴板和外部打开由 Flutter/插件的各端原生实现负责；
应用中的适配器只做数据与接口转换。

Android 系统栏需要单独处理边到边显示和深浅色图标。桌面窗口尺寸和原生图标分别
由对应宿主工程配置。macOS 的出站网络与用户选择文件权限由 entitlements 声明。
这些差异不进入论坛业务 API。

## 构建与验证范围

Windows 和 Android 可在当前 Windows 开发机上构建。macOS 和 Linux 的原生工程、
布局策略和打包入口已经提供，必须在各自系统构建；通过界面策略测试不代表完成
这两个系统的原生构建、安装或真实账号验收。当前没有 iOS、Web 工程。

Android APK 当前使用 debug 签名，适合本地安装测试；分发正式版本前配置自己的
签名。Windows ZIP 不是安装向导；macOS 应用包没有在此工程中配置签名或公证；
Linux 产物是可运行目录，没有额外生成 AppImage、deb 或 rpm。

## 新增平台细节时放在哪里

1. 纯业务规则或数据格式差异：优先放共用功能/API，并根据真实规则建模。
2. 窗口、导航、系统栏差异：放 `lib/platforms/` 对应位置，在 `bootstrap.dart` 组装。
3. 系统能力插件：在 `platforms/adapters/` 转换为共享契约，不让页面直接使用插件。
4. 包名、权限、图标、签名、原生编译选项：放原生宿主目录。
5. 编译、归档、分发差异：放 `tool/release/`，输出放对应的 `dist/<平台>/`。
