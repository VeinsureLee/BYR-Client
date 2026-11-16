# Android 客户端

**选型已确定：Gio。** 生产模块位于 `app/`，复用仓库共享的 `core/`；`probes/gio/`
保留为独立技术探针，不参与正式构建。

## 生产模块（app/）

Gio v0.10.3，包名 `cn.byr.bbs.byrclient`，ARM64，min SDK 24 / target SDK 36。
当前功能：

- 登录、会话恢复与退出；令牌保存在应用私有目录（0600），不写入普通状态文件。
- 首页：根分区、子分区卡片、版面列表；版面主题列表与分页。
- 主题阅读：楼层正文、点赞/点踩、文章收藏、回复主题/楼层、复制正文、系统返回手势。
- 发帖/回复/编辑（基础编辑器与本地草稿）；附件和图片上传暂未接入。
- 时间线（全部帖子按发帖时间倒序）与分页。
- 收藏：版面收藏、文章收藏/取消。
- 搜索页：全站搜索 / AI 搜索均为「敬请期待」，不发起搜索请求。
- 个人页：资料、最近阅读与退出登录。

尚未接入：站内信/私信、投票、竞猜、网络图片与附件显示、文件选择、分享 Intent、
深色主题切换、真机生命周期与旋转验收。当前 APK 为 debug 签名预览。

```sh
go run ./tools/dev build android
```

输出 `dist/android/`：`BYR-Client-android-arm64.apk`、字体许可证与 `manifest.json`。
构建需要 `ANDROID_HOME`（SDK）、NDK 和 JDK；正式签名密钥尚未配置。

## 技术探针（probes/gio/）

离线验证中文输入、文本选择、长列表和自定义绘制，不登录、不访问论坛；继续作为
Gio 能力对照，不随生产 Android 模块发布。

```sh
go run ./tools/dev check --scope gio-probe
go run ./tools/dev build android-probe --framework gio
```

技术探针输出在 `dist/evaluation/android/gio/`，包名与正式客户端分开。

参见 [选型比较](../../docs/android-options.md)、[验证与实机检查表](../../docs/android-evaluation.md)。
