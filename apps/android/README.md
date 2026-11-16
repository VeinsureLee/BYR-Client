# Android 客户端

**选型已确定：Gio。** 生产模块位于 `app/`，复用仓库共享的 `core/`；`probes/gio/`
保留为独立技术探针，不参与正式构建。

## 生产模块（app/）

Gio v0.10.3，包名 `cn.byr.bbs.byrclient`，ARM64，min SDK 24 / target SDK 36。
当前功能：

- 登录、会话恢复与退出；退出或重启后稳定回到登录页，令牌保存在应用私有目录（0600）。
- 首页：根分区、子分区卡片、版面列表；今日十大与版面推荐；版面主题列表与分页。
- 主题阅读：真实论坛头像、楼层正文、点赞/点踩、文章收藏、回复主题/楼层、复制正文、
  查看投票/竞猜、系统返回手势。
- 发帖/回复/编辑：编辑器始终可用；可直接在顶部「+」新建帖子或从版面/主题进入；
  支持匿名、签名、邮件提醒、允许转出；返回、切换页签或关闭时保存本地草稿。
- 时间线（全部帖子按发帖时间倒序）与分页。
- 收藏：版面收藏；文章收藏整卡可点、可取消，支持分页。
- 搜索：底部导航固定入口，全站搜索 / AI 搜索均为「敬请期待」，不发起搜索请求。
- 消息：私信联系人与会话、收件箱/发件箱、写信/回复/转发/删除、回复与 @ 提醒。
- 个人：资料与头像、发私信、加入黑名单、完整阅读历史、黑名单列表、浅色/深色主题、
  字号与 IPv6 设置、投票与竞猜读取/提交。

尚未接入：附件选择/上传、图片查看器与表情面板、编辑器 UBB 工具栏、分享 Intent、
真机生命周期与旋转验收。当前 APK 为 debug 签名预览。

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
