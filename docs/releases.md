# 构建文件与发布软件

## 三类目录

| 目录 | 性质 | 是否纳入 Git |
| --- | --- | --- |
| `android/`、`windows/`、`macos/`、`linux/` | 原生源代码与构建输入 | 是，本机配置和临时生成内容除外 |
| `build/`、`.dart_tool/`、各端 `ephemeral/` | Flutter/Gradle/CMake 编译缓存与中间输出 | 否，由工具链管理 |
| `dist/<平台>/` | 完整可运行软件、分发压缩包与发布清单 | 否，可作为发布附件单独分发 |
| `dist/archive/<平台>/` | 已有分发包的归档，与当前可用软件分开 | 否 |

平台宿主目录保持 Flutter 要求的位置。整理分类不需要移动它们，也不要把 DLL、
APK、缓存或 SDK 路径放入 `lib/` 或源码仓库。

## 统一入口

在仓库根目录执行：

```sh
flutter pub get --enforce-lockfile
flutter analyze --no-fatal-infos
flutter test
dart run tool/build_release.dart --platform windows
```

将 `windows` 替换为 `android`、`macos`、`linux` 即可使用对应发布策略。
桌面目标必须使用相同系统构建，Android 需要已配置的 Android SDK。
`--build-name 1.2.3` 可覆盖本次构建的软件版本；默认使用 `pubspec.yaml`。

已有对应 Release 构建时可加 `--package-only`，只验证与整理软件，不调用 Flutter
重新编译。此模式按当前 `pubspec.yaml` 记录版本，不能与 `--build-name` 同时使用；
需要准确记录自定义版本时使用完整构建命令。

Android 的 Bash / Git Bash 快捷入口为 `bash tool/build/android.sh`，会依次安装
锁定依赖、分析、测试、构建并整理 APK。可选 `FLUTTER_BIN` 指定 Flutter bin 目录。

## 分平台输出

```text
dist/
  archive/                        # 按平台归档的已有分发包
  windows/
    app/                          # exe、所有 DLL、data/，整体保留
    BYR-Client-windows-x64.zip
    manifest.json
  android/
    BYR-Client-android.apk
    manifest.json
  macos/
    app/byr_client.app/
    BYR-Client-macos.zip
    manifest.json
  linux/
    app/                          # 可执行文件、lib/、data/
    BYR-Client-linux.tar.gz
    manifest.json
```

仅已在相应宿主上成功打包的目标才会出现在 `dist/`，不会创建空的可用软件目录。
Windows 必须分发整个运行目录或其 ZIP；macOS 使用 `ditto` 保留应用包结构；
Linux 使用 `cp -a` 和 `tar` 保留可执行权限及相对库链接。

`manifest.json` 记录平台、版本、构建号、模式、包名、运行入口和打包时间。
Android 额外记录当前 debug 签名配置，清单不包含账号或令牌。

## 打包流程与发布

`tool/build_release.dart` 负责参数检查和调用 Flutter；`tool/release/release_target.dart`
定义四个平台的归档策略；`package_release.dart` 验证必需文件、写入清单并发布到
对应目录。先在 `dist/` 内创建临时目录并完成归档，再替换目标平台已有输出，
不会因为缺少构建文件而先删除已有软件，也不会修改其他平台的发布目录。

CI 检查共享代码和测试。tag 工作流目前自动构建 Windows x64，调用同一个发布工具，
将 `dist/windows/BYR-Client-windows-x64.zip` 上传 GitHub Release。
其他平台使用各自宿主上的统一命令生成，未配置自动上传。
