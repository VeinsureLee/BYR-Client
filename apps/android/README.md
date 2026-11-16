# Android 接入边界

正式 UI 框架待定。所有者当前更重视自定义阅读交互与绘制控制，因此 **Gio 优先验证**，
Fyne 保留为对照。`decision.json` 的 `selectedFramework` 与 `productionModule` 保持空值。

`probes/gio/` 是独立的技术实验模块，导入同一份 `core/`，不能被桌面或核心反向依赖。
探针没有账号登录、论坛数据、生产导航或功能模块，也不会代替后续页面设计。

正式选择后，再创建生产 Android 模块、系统能力适配、生命周期接入与对应构建规则。
编辑选型 JSON 不会自动启用生产 APK 构建，避免产生误导性产物。

参见 [选型比较](../../docs/android-options.md)、[验证与实机检查表](../../docs/android-evaluation.md)。
