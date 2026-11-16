# 构建流程与产物分类

## 工具与模块隔离

| 模块 | 最低 Go 版本 | UI / 构建工具依赖 |
| --- | --- | --- |
| `core/` | 1.25.6 | 标准库，无 UI 依赖 |
| `tools/` | 1.25.6 | 标准库，通过子进程调用各模块工具 |
| `apps/desktop/` | 1.27.4 | MyGo v0.9.1，库与 CLI 同版本 |
| `apps/android/app/` | 1.25.6 | Gio v0.10.3、gogio v0.10.3；Android 需要 SDK / NDK / JDK |
| `apps/android/probes/gio/` | 1.25.6 | Gio v0.10.3、gogio v0.10.3；独立技术探针 |

根目录推荐 Go 1.27.4。`go.work` 聚合核心、桌面和工具；Android 生产模块与探针都在工作区之外。
工具对子模块执行命令时设置 `GOWORK=off`，确保各模块的 `go.mod`、`go.sum` 和相对
`replace` 独立有效，不通过桌面的依赖版本间接“修好”Android 构建。

根目录直接 `go test ./...` 不是本仓库的全模块入口，使用以下命令：

```sh
go run ./tools/dev check
go run ./tools/dev check --scope core
go run ./tools/dev check --scope android
go run ./tools/dev check --scope gio-probe
go run ./tools/dev check android-core
```

主检查分别执行核心、工具、桌面、Android 生产模块的 `go test ./...` 与 `go vet ./...`。探针单独检查。
Android 核心检查只编译 `GOOS=android GOARCH=arm64 CGO_ENABLED=0` 的共享库包，
报告放 `build/checks/android-arm64.json`，不会创建 APK 或分发目录。

## 桌面构建

```sh
go run ./tools/dev build desktop --target windows/amd64
```

目标还支持 Windows ARM64、Linux amd64/arm64、macOS amd64/arm64/universal。
MyGo 实际执行在桌面模块中，CLI 版本来自 `go.mod` 的 tool 声明，不使用全局安装的
未知版本。默认生成可运行目录和 ZIP，不下载/调用 Windows 安装程序工具；加
`--installer` 时交由 MyGo 生成 Windows 安装包。

构建步骤：

1. 校验目标和桌面元数据。
2. 在 `build/` 下创建临时目录，由 MyGo 编译及组织运行文件。
3. 确认目标可执行文件非空，归档完整输出，计算 SHA-256，写入构建清单。
4. 先准备完整发布目录，再替换 `dist/desktop/<平台>-<架构>/`；失败时保留已有软件。

Windows 若资源管理器占用目录但未锁定 exe，构建会原地替换内容；若 exe 正在运行，
命令会明确提示关闭程序后重试。不会创建时间戳副本，也不会关闭用户程序。

清理和移动只允许在项目生成目录内部执行，并检查解析后的路径，避免跟随目录链接
误操作其他位置。不同目标不共享一个输出目录。ZIP 保留相对链接，并明确为目标入口、
Shell 脚本及目录记录执行权限，避免从 Windows 交叉编译时丢失 Unix 执行位；不只取一个 exe。

当前桌面版本号来自 `apps/desktop/mygo.json`，阶段为 `feature-migration`。
macOS 签名、公证和磁盘镜像有宿主/签名环境要求，Windows 本机构建通过不表示其他系统
的运行体验也已验收。迁移范围与真实账号验收边界见 [功能对照](features.md)。

中文字体在 `core/resources/fonts/` 统一维护，桌面和 Gio 使用同一份资源。
构建工具将 OFL 许可证放入桌面运行目录及归档、Android 评估产物目录；不重复维护字体副本。

## Android 核心、评估与正式应用

| 命令 | 作用 | 产物 |
| --- | --- | --- |
| `check android-core` | 共享 Go 库交叉编译验证 | `build/checks/` 报告，不是 APK |
| `build android-probe --framework gio` | 独立 Gio 技术验证 | `dist/evaluation/android/gio/` 的评估 APK |
| `build android` | Gio 生产预览构建 | `dist/android/` 的 ARM64 APK |

生产构建读取 `apps/android/decision.json`，使用 `apps/android/app`、包名
`cn.byr.bbs.byrclient`、Gio 与 ARM64；当前版本 `0.4.2`（versionCode 13），使用
debug 签名，清单标记 `stage: android-preview`、`production: true`、`deviceTested: false`。
提供发布签名密钥之前不得作为正式商店版本发布。构建需要 `ANDROID_HOME`、NDK 和 JDK；
这些本机路径不写入源码或清单。

## 生成内容分类

```text
build/
  checks/android-arm64.json                  # 共享核心的编译报告
  desktop-*/                                # MyGo 临时输出，完成后清理
dist/
  desktop/windows-amd64/
    app/                                    # 完整可运行内容
    BYR-Client-windows-amd64.zip
    manifest.json                           # 阶段、版本、目标、入口、校验值
  android/
    BYR-Client-android-arm64.apk
    FONT-LICENSE.txt
    manifest.json                           # 生产预览、debug 签名、未实机验收
  evaluation/android/gio/
    BYR-Gio-Probe.apk
    FONT-LICENSE.txt
    manifest.json                           # 非生产技术探针
```

上述目录不提交 Git。源码、模块锁定文件、选型状态和文档提交 Git；账号、令牌、SDK
本机路径、签名密钥不提交。原生工具的缓存由对应工具管理，不移动进共享核心。

## CI

- `ci.yml`：独立核心/工具检查，桌面、Android 生产模块及 Gio 探针的无窗口测试。
- `release.yml`：手动选择桌面目标，在匹配系统构建并上传 ZIP 和清单。
- `android-evaluation.yml`：手动构建 Gio ARM64 验证 APK，并上传评估产物与许可证；生产 Android 预览目前由本地构建命令生成。

这些工作流不会自动发布生产 GitHub Release，也不改变产品页面或 Android 选型。
本地构建已验证；工作流文件的语法检查不等于远端 GitHub Actions 已执行成功。
