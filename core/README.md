# 共享 Go 业务核心

模块 github.com/VeinsureLee/BYR-Client/core，最低 Go 1.25.6，仅依赖标准库。
架构测试禁止导入 MyGo、Fyne、Gio 和 apps/。

| 包 | 内容 |
| --- | --- |
| forum | 完整迁移的论坛协议、DTO、OAuth、附件、邮件、搜索、收藏、通知、投票/竞猜、公开信息、实验加密接口 |
| application | 会话恢复/退出、发帖权限与发布、草稿清理、单向评价规则、私信聚合和正文 |
| local | 按账号保存草稿与最近历史、偏好、版本化序列化 |
| content | UBB/图片/附件/表情解析，按 Unicode rune 处理编辑选区 |
| resources | 原有 GIF；fonts 子包包含共享中文字体和 OFL 许可证 |

接口由消费者声明：local.Backend 只负责读写状态文档，application.CredentialStore
只负责凭据。MyGo 系统凭据、文件位置和 Gio 控件留在应用模块。
HTTP 客户端、TokenSource 及平台适配器通过构造参数注入，没有全局账户。

独立检查：

```sh
# core 目录，先设置 GOWORK=off
go test ./...
go vet ./...
```

根目录的 go run ./tools/dev check --scope core 自动执行隔离检查。
测试使用本地 HTTP 服务；接口合约通过不代表真实账号的服务端写入已验收。
