# 代码分类与设计模式

## 按职责放置代码

| 目录 | 内容 | 放置规则 |
| --- | --- | --- |
| `lib/app/` | 启动、依赖组装、应用主题、应用级入口 | `bootstrap.dart` 是具体实现的组装位置 |
| `lib/shared/core/` | 常量、结果类型、时间格式 | 四端共用，不判断操作系统 |
| `lib/shared/data/` | 论坛模型、HTTP 协议、各业务 API | 四端使用同一套协议；不复制到平台目录 |
| `lib/shared/storage/` | 会话、浏览历史、草稿、存储契约 | 负责数据规则和序列化，通过 `PreferenceStore` 持久化 |
| `lib/shared/services/` | 剪贴板、外部链接、附件选择的契约 | 使用应用自己的类型，不暴露插件类型 |
| `lib/shared/widgets/` | 正文、图片、头像、互动与状态视图 | 多个功能/平台复用的 Flutter 组件 |
| `lib/features/` | 登录、版面、帖子、搜索、信箱、设置等功能页面 | 描述业务交互，通过 `AppScope` 读取依赖 |
| `lib/platforms/desktop/` | 桌面导航与多栏论坛工作区 | Windows、macOS、Linux 共用桌面布局 |
| `lib/platforms/mobile/` | 移动导航与宽屏布局适配 | Android 共用功能页面；宽屏使用导航栏，不切换桌面业务实现 |
| `lib/platforms/android/` | Android 系统栏、边到边显示 | Android/HarmonyOS 深浅色系统栏细节 |
| `lib/platforms/adapters/` | 本地存储与系统能力插件适配器 | 唯一直接依赖 `shared_preferences`、`file_picker`、`url_launcher` 的应用层位置 |
| `android/`、`windows/`、`macos/`、`linux/` | Flutter 原生宿主与平台构建配置 | 保持 Flutter 标准位置，由工具链识别 |

`dart:io` 中的 HTTP Cookie 类型属于共用网络协议，不能仅凭 `dart:io` 导入把整个
模块归为某个平台。实际的操作系统检测集中在 `platform_detector.dart`。

## 采用的模式

| 模式 | 实现 | 解决的问题 |
| --- | --- | --- |
| Strategy（策略） | `AppServices.homeBuilder` + `AppHome` | 启动、恢复会话、登录成功使用同一个可注入界面策略，页面不再自行判断操作系统 |
| Adapter（适配器） | `SharedPreferencesStore`、`FlutterDeviceServices` | 把插件的各类返回值和调用方式转换为应用契约，测试可以替换为内存存储和假系统能力 |
| Facade（外观） | `ByrApi` | 统一组装认证、版面、文章、信箱等 API，共享 HTTP 客户端与会话 |
| Observer（观察者） | `Session` 的 `ValueNotifier`，应用中的 `ValueListenableBuilder` | 主题和字体修改后通知订阅的界面，不建立额外事件总线 |
| Strategy（发布策略） | `ReleaseTarget` 的配置与 `assemble` 函数 | Android 复制 APK，Windows 打包完整运行目录，macOS 保留应用包结构，Linux 保留权限和链接 |

策略使用 Dart 函数表达；跨系统差异已由 Flutter 插件实现的部分共用一个适配器，
不再为每个操作系统创建空壳类。依赖由应用实例持有，不使用全局服务定位器。

## 依赖组装

`main.dart` 调用 `bootstrapApplication()`，集中检测平台、创建存储和系统适配器、
恢复会话，再将 `AppServices` 传入 `ByrApp`。`AppScope` 向各功能暴露这些依赖。

功能页面使用 `AppScope.of(context).api` 调用论坛接口，使用 `.device` 调用系统能力，
使用 `.history` / `.drafts` 处理本地数据。共享数据和存储模块不导入具体平台适配器。
移动端、桌面端都使用同一个分区页、十大页、模型、编辑器与正文渲染组件。

## 测试归类

| 位置 | 验证内容 |
| --- | --- |
| `test/architecture_test.dart` | 共享模块和功能页面不重新引入具体插件或操作系统判断 |
| `test/shared/` | 存储适配、会话恢复、观察者通知、历史限制、草稿序列化 |
| `test/platforms/` | 四个平台的布局选择、登录策略、系统能力替换、Android 系统栏 |
| `test/tool/` | 构建宿主约束、产物分类、失败保留已有软件、完整 Windows 打包 |
| `test/feature_navigation_test.dart` | 版面、十大、发帖入口和邮件等功能流程 |
| `test/widget_test.dart` | 协议模型、正文、主题及共用组件 |
| `test/support/` | 测试专用内存存储、假系统能力、假 HTTP 服务，不进入应用产物 |

修改布局策略、适配器或发布流程时运行相关测试，再执行完整的静态分析和测试。
