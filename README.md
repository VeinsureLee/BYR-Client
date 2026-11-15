# BYR Client

北邮人论坛跨端客户端，使用 Flutter / Dart，面向 Android、Windows、macOS 和 Linux。
桌面端采用侧边导航与分栏工作区，移动端采用底部导航，两者共用论坛协议、功能和数据。

## 启动

使用 Flutter **3.50.0** / Dart **3.14.0**，SDK 版本记录在 `.flutter-version`。

```sh
flutter pub get --enforce-lockfile
flutter run -d windows
```

Android 通过 `flutter devices` 查看设备 ID，再执行 `flutter run -d <设备ID>`。
macOS 和 Linux 分别使用 `-d macos`、`-d linux`，需在对应系统运行。

## 目录分类

| 位置 | 职责 |
| --- | --- |
| `lib/app/` | 启动、依赖组装、主题和应用入口 |
| `lib/shared/core/` | 共用常量、结果和时间格式 |
| `lib/shared/data/` | 四端共用的论坛协议、API、数据模型 |
| `lib/shared/storage/` | 会话、历史、草稿及存储契约 |
| `lib/shared/services/` | 剪贴板、外部打开、文件选择契约 |
| `lib/shared/widgets/` | 共用正文、图片、头像和交互组件 |
| `lib/features/` | 跨平台功能页面 |
| `lib/platforms/desktop/` | Windows / macOS / Linux 的导航与分栏布局 |
| `lib/platforms/mobile/` | 移动导航与宽屏布局适配 |
| `lib/platforms/android/` | Android 系统栏和边到边显示 |
| `lib/platforms/adapters/` | 系统能力与本地存储的插件适配 |
| `android/`、`windows/`、`macos/`、`linux/` | Flutter 标准原生宿主、权限、图标与构建配置 |
| `assets/em/` | 201 个 GIF 表情 |
| `test/` | 共享逻辑、平台策略、功能、打包和架构约束测试 |
| `tool/release/` | 按平台组织的构建归档策略 |
| `tool/build/`、`tool/diagnostics/` | 构建快捷入口与手动诊断 |
| `build/`、`.dart_tool/` | 工具链生成的中间文件，不提交 |
| `dist/<平台>/` | 完整可运行软件、分发包和发布清单，不提交 |

详细说明：[代码分类与设计模式](docs/architecture.md) · [平台细节与软件形式](docs/platforms.md) · [构建与发布](docs/releases.md)。

## 设计模式

- **Strategy**：注入界面构建函数，启动和登录共用同一个桌面/移动策略；各端发布使用独立的归档策略。
- **Adapter**：存储和系统能力通过应用契约调用，具体 Flutter 插件集中到适配层。
- **Facade**：`ByrApi` 统一组装论坛各业务 API。
- **Observer**：主题和字体使用 `ValueNotifier` / `ValueListenableBuilder` 通知界面。

依赖集中在 `app/bootstrap.dart` 组装，业务页面不直接检测操作系统或导入原生插件。

## 功能

- 账号登录、会话恢复、IPv6 镜像切换。
- 分区、全部版面、十大、帖子、翻页与刷新。
- 发帖、回复、编辑、删除、草稿、匿名选项、表情和附件。
- 图片预览、点赞/踩、收藏、复制链接、搜索和浏览历史。
- 用户主页、黑名单、信箱、私信、回复与 @ 提醒。
- 投票、竞猜、浅色/深色/跟随系统主题与字体缩放。

真实账号操作和各平台的完整业务行为仍需实际联调。

## 验证与软件构建

```sh
flutter analyze --no-fatal-infos
flutter test

dart run tool/build_release.dart --platform windows
dart run tool/build_release.dart --platform android
```

在对应系统可使用 `--platform macos` / `--platform linux`。已有 Release 构建时，
追加 `--package-only` 只验证、整理发布软件。Android Bash 快捷入口为
`bash tool/build/android.sh`。

| 平台 | 可分发软件 | 当前验证范围 |
| --- | --- | --- |
| Windows x64 | `dist/windows/app/`、`dist/windows/BYR-Client-windows-x64.zip` | 本机 Release 构建；完整运行依赖归档检查 |
| Android | `dist/android/BYR-Client-android.apk` | 本机 Release 构建；APK 归档检查 |
| macOS | `dist/macos/app/byr_client.app/`、ZIP | 已提供原生工程和发布策略，需 macOS 构建验证 |
| Linux | `dist/linux/app/`、tar.gz | 已提供原生工程和发布策略，需 Linux 构建验证 |

Windows 运行目录必须整体分发，包含 exe、DLL 和 `data/`。Android APK 当前为
**debug 签名**，适合本地安装测试；正式发布前需配置自己的签名。当前没有 iOS、Web 工程。
静态分析会报告弃用提示（info），错误和警告使检查失败。

CI 在 push / pull request 时分析并运行测试；tag 发布调用统一工具生成 Windows ZIP，
上传 GitHub Release。软件版本来自 `pubspec.yaml`，可用 `--build-name` 覆盖本次构建版本。

手动诊断：`dart run tool/diagnostics/login_probe.dart`，使用 `test/test` 假凭据访问真实论坛，
不会在自动测试中执行。账号、会话、本机 SDK 路径和签名密钥均不属于源码。
