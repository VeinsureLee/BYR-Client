# BYR Client

北邮人论坛客户端：**共享 Go 业务核心 + MyGo PC + 独立 Android UI**。

原 Flutter 客户端的业务能力已迁入 Go，并接入 MyGo 桌面界面。沿用原有功能流程作为迁移基线，后续页面设计与交互调整由项目所有者决定。

| 部分 | 当前状态 |
| --- | --- |
| PC | MyGo v0.9.1；登录、论坛阅读、发帖编辑、附件、收藏、时间线、消息和个人设置；搜索入口为「敬请期待」占位 |
| 共享核心 | 纯 Go 标准库；完整迁移的论坛接口、会话、发帖规则、草稿、历史、私信聚合、UBB 与共享资源 |
| Android | Gio v0.10.3 生产预览；登录、分区/版面/主题阅读、时间线、收藏、发帖回复、点赞踩/收藏与搜索占位；ARM64 debug 预览，待真机验收 |
| 验证范围 | 本地合约与交互、Windows 原生菜单点击、真实只读接口、各模块 test/vet；线上写入与 Android 实机尚待验收 |

## 目录

```text
core/
  forum/                      # HTTP 协议、JSON 模型、认证、附件与实验加密接口
  application/                # 会话、发布、评价与私信业务用例
  local/                      # 按账号隔离的草稿、历史、偏好与序列化
  content/                    # UBB 正文解析、图片/文件标记、Unicode 编辑操作
  resources/                  # 201 个原有 GIF；fonts/ 共享中文字体与许可证
apps/
  desktop/
    internal/bootstrap/       # 显式组装核心与平台适配器
    internal/desktop/         # MyGo 页面、导航、异步任务和 UI 状态
    internal/platform/        # 系统凭据库、文件、对话框、剪贴板与浏览器
  android/
    decision.json             # 已选择 Gio 生产模块
    app/                      # Gio 生产 Android 客户端
    probes/gio/               # 独立技术验证模块（保留）
tools/                        # 模块检查、构建、完整归档与校验
docs/                         # 架构、功能对照、选型及构建说明
build/                        # 中间输出与本地渲染截图，不提交
dist/                         # 按平台分类的可执行软件与评估 APK，不提交
```

各模块独立维护依赖。根工作区聚合核心、桌面和工具；Android 生产模块与 Gio 探针
都使用 `GOWORK=off`，通过相对 `replace` 引用同一份核心源码，不把 Gio 依赖混入桌面工作区。

## 开发

推荐 Go **1.27.4**，最低模块版本及固定依赖见 [构建说明](docs/build.md)。

```sh
go run ./tools/dev check
go run ./tools/dev check --scope android
go run ./tools/dev check --scope gio-probe
go run ./tools/dev check android-core
go run ./tools/dev run desktop
```

启动时恢复已保存的 OAuth 登录；首次运行显示登录页。用户名使用论坛 ID。
令牌保存在 MyGo 适配的 Windows Credential Manager、macOS Keychain 或 Linux Secret Service；
关闭“记住登录”可仅在本次进程使用令牌。普通配置文件不保存令牌或密码。

草稿与最近 100 条阅读历史按账号保存。偏好及本地内容位于系统用户配置目录下的
`BYR-Client/state.json`；可通过 `BYR_DATA_DIR` 指定测试目录。
`BYR_BASE_URL` 仅用于开发或本地接口测试，默认连接北邮人论坛。

## 构建与产物

```sh
go run ./tools/dev build desktop --target windows/amd64
go run ./tools/dev build desktop --target linux/amd64
go run ./tools/dev build desktop --target darwin/arm64
go run ./tools/dev build android-probe --framework gio
go run ./tools/dev build android
```

```text
dist/desktop/windows-amd64/
  app/BYR Client.exe
  app/FONT-LICENSE.txt
  BYR-Client-windows-amd64.zip
  manifest.json
dist/android/
  BYR-Client-android-arm64.apk
  FONT-LICENSE.txt
  manifest.json
dist/evaluation/android/gio/
  BYR-Gio-Probe.apk
  FONT-LICENSE.txt
  manifest.json
```

桌面 ZIP 包含完整运行目录和字体许可证。跨平台编译不代替目标系统运行验收。
Android 生产 APK 使用包名 `cn.byr.bbs.byrclient`、ARM64、debug 签名，构建前需要
`ANDROID_HOME`（以及可用的 NDK/JDK）；在提供发布签名密钥前不会作为正式商店版本。
技术探针仍使用独立包名 `cn.byr.bbs.byr_client.gioprobe`，保留作框架对照。

- [迁移功能对照与验收边界](docs/features.md)
- [架构与设计模式](docs/architecture.md)
- [Android：Fyne / Gio 的选择](docs/android-options.md)
- [Android 客户端与 Gio 验证](docs/android-evaluation.md)
- [构建流程](docs/build.md)

测试使用本地模拟服务，不需要真实论坛账号，不会向线上发布帖子或发送站内信。
本机默认 Go 代理不可用时，可在当前 PowerShell 临时设置
`$env:GOPROXY = 'https://goproxy.cn,direct'`，项目不修改全局 Go 环境。
