package desktop

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"BYR-Client/internal/ai"
)

func (u *UI) meContent(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).MaxWidth(860).Margin(0, ui.Auto).Padding(30).Gap(16).Children(func() {
			ui.Text(c, "我的").FontSize(u.fs(26)).Bold()
			ui.Text(c, "个人功能预留入口；当前版本不读取真实账号数据，也不执行发帖或回帖。").
				TextColor(t.TextMuted)
			ui.Row(c).Gap(12).Children(func() {
				u.plannedCard(c, "我的发帖", "按主题作者查询本账号发表的主题。")
				u.plannedCard(c, "我的回帖", "按回复楼层作者查询，保留原帖上下文。")
			})
		})
	})
}

func (u *UI) inboxContent(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).Fill().Center().Gap(10).Children(func() {
		ui.Icon(c, iconInbox).FontSize(u.fs(30)).TextColor(t.TextMuted)
		ui.Text(c, "私信").FontSize(u.fs(22)).Bold()
		ui.Badge(c, "planned")
		ui.Text(c, "私信将使用独立的会话与消息模型，并按账号隔离。").
			FontSize(u.fs(12)).TextColor(t.TextMuted)
		ui.Text(c, "当前不显示虚构的未读数或消息内容。").
			FontSize(u.fs(11)).TextColor(t.TextMuted)
	})
}

func (u *UI) plannedCard(c *ui.Context, title, description string) {
	t := c.Theme()
	ui.Column(c).Grow(1).Padding(18).Gap(8).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).Bold().FontSize(u.fs(15))
			ui.Badge(c, "planned")
		})
		ui.Text(c, description).FontSize(u.fs(12)).TextColor(t.TextMuted)
	})
}

func (u *UI) settingsContent(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(900).Margin(0, ui.Auto).Padding(32).Gap(24).Children(func() {
			ui.Text(c, "设置").FontSize(u.fs(26)).Bold()

			ui.Column(c).Padding(18).Gap(10).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, "账号").Bold().FontSize(u.fs(15))
				name, detail := u.sessionLine()
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Text(c, name).FontSize(u.fs(13)).Bold()
						if detail != "" {
							ui.Text(c, detail).FontSize(u.fs(11)).TextColor(t.TextMuted)
						}
					})
					if u.loginBusy {
						ui.Spinner(c)
						ui.Text(c, u.loginStatus).FontSize(u.fs(11)).TextColor(t.TextMuted)
					}
					ui.Button(c, "退出登录").Disabled(u.loginBusy).OnClick(u.logout)
				})
				if u.loginError != "" {
					ui.Text(c, u.loginError).FontSize(u.fs(11)).TextColor(t.Danger)
				}
				ui.Row(c).Gap(10).Wrap().AlignItems(ui.Center).Children(func() {
					if u.rememberPassword && u.hasSavedCredentials() {
						ui.Text(c, "已保存账号密码（系统凭证存储）").FontSize(u.fs(10)).TextColor(t.TextMuted)
						ui.Button(c, "清除已保存密码").FontSize(u.fs(11)).OnClick(func() {
							u.rememberPassword = false
							u.saveSettings()
							u.clearSavedCredentials()
						})
					} else {
						ui.Text(c, "未保存账号密码；可在登录页勾选“保存账号密码”。").FontSize(u.fs(10)).TextColor(t.TextMuted)
					}
				})
				ui.Text(c, "会话 Cookie 可在登录页关闭“保持登录”；退出登录会清除 Cookie、目录、任务与本地内容实体。").
					FontSize(u.fs(10)).TextColor(t.TextMuted)
			})

			ui.Column(c).Padding(18).Gap(8).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, "论坛目录").Bold().FontSize(u.fs(15))
				stats := u.app.CatalogStats()
				if stats.Ready {
					ui.Text(c, fmt.Sprintf("%d 个讨论区 · %d 个子讨论区 · %d 个版面 · 更新于 %s",
						stats.Forums, stats.SubForums, stats.Boards, stats.FetchedAt.Format("2006-01-02 15:04"))).
						FontSize(u.fs(11)).TextColor(t.TextMuted)
				} else {
					ui.Text(c, "目录尚未初始化").FontSize(u.fs(11)).TextColor(t.TextMuted)
				}
				for i, warning := range stats.Warnings {
					if i >= 3 {
						ui.Text(c, fmt.Sprintf("另有 %d 条目录警告", len(stats.Warnings)-3)).FontSize(u.fs(10)).TextColor(t.Warning)
						break
					}
					ui.Text(c, "目录警告："+warning).FontSize(u.fs(10)).TextColor(t.Warning)
				}
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Button(c, "重新初始化目录").Disabled(u.loginBusy).OnClick(u.refreshCatalog)
					if u.loginBusy {
						ui.Spinner(c)
					}
					ui.Text(c, "更新讨论区与版面目录").FontSize(u.fs(12)).TextColor(t.TextMuted)
				})
			})

			ui.Column(c).Padding(18).Gap(10).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, "外观").Bold().FontSize(u.fs(15))
				ui.Text(c, "跟随系统时使用启动时的系统主题；浅色与深色可立即预览。").
					FontSize(u.fs(11)).TextColor(t.TextMuted)
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					before := u.themeMode
					ui.Segmented(c, &u.themeMode, "跟随系统", "浅色", "深色")
					if u.themeMode != before {
						u.saveSettings()
					}
					ui.Text(c, u.themeModeLabel()).FontSize(u.fs(11)).TextColor(t.TextMuted)
				})
				ui.Text(c, "字体大小").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					before := u.fontScaleMode
					ui.Segmented(c, &u.fontScaleMode, "小", "标准", "大", "特大")
					if u.fontScaleMode != before {
						u.fontScale = fontScaleFor(u.fontScaleMode)
						u.saveSettings()
					}
					ui.Text(c, "流量与界面文字会一起缩放").FontSize(u.fs(11)).TextColor(t.TextMuted)
				})
				ui.Text(c, "主题色").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					for index, preset := range accentPresets {
						i := index
						color := preset.Light
						if t.Dark {
							color = preset.Dark
						}
						swatch := ui.Box(c).Size(28, 28).Radius(9).Background(color).
							Border(1, t.Border).Cursor(ui.CursorPointer).Tooltip(preset.Name).
							Label("主题色 " + preset.Name).
							OnClick(func() {
								u.accentMode = i
								u.saveSettings()
							})
						if i == u.accentMode {
							swatch.Border(2, t.Text)
						}
						pose := ui.Pose{}
						if swatch.Pressed() {
							pose = ui.Pose{Scale: 0.9}
						}
						swatch.AnimatePose(pose, ui.Animation{Duration: 100 * time.Millisecond, Ease: ui.EaseOut})
					}
					accentName := accentPresets[0].Name
					if u.accentMode >= 0 && u.accentMode < len(accentPresets) {
						accentName = accentPresets[u.accentMode].Name
					}
					ui.Text(c, accentName).FontSize(u.fs(11)).TextColor(t.TextMuted)
				})
			})

			ui.Column(c).Padding(18).Gap(10).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, "AI 配置").Bold().FontSize(u.fs(15))
				ui.Text(c, "用于“搜索 → AI 问答”。当前支持 DeepSeek 与 Kimi（Moonshot），使用 OpenAI 兼容的 chat/completions 接口。").
					FontSize(u.fs(11)).TextColor(t.TextMuted)
				ui.Row(c).Gap(12).Wrap().Children(func() {
					u.field(c, "服务商", 180, func() {
						before := u.aiProvider
						ui.Select(c, &u.aiProvider, []string{"DeepSeek", "Kimi"})
						if u.aiProvider != before {
							base, model := ai.Defaults(aiProviderKey(u.aiProvider))
							u.aiBaseURL = base
							u.aiModel = model
							u.aiTestStatus = ""
						}
					})
					u.field(c, "模型", 280, func() {
						ui.TextInput(c, &u.aiModel).Placeholder("deepseek-chat / moonshot-v1-8k")
					})
				})
				u.field(c, "API 地址", 520, func() {
					ui.TextInput(c, &u.aiBaseURL).Placeholder("https://api.deepseek.com/v1")
				})

				ui.Text(c, "已保存的 Key").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
				if len(u.aiKeys) == 0 {
					ui.Text(c, "还没有保存的 API Key。").FontSize(u.fs(11)).TextColor(t.TextMuted)
				}
				for _, entry := range u.aiKeys {
					item := entry
					active := item.ID == u.aiKeyID
					row := ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Padding(6, 8).Radius(6).
						Background(t.Surface).Border(1, t.Border)
					if active {
						row.Background(t.Selection)
					}
					row.Children(func() {
						ui.Badge(c, aiProviderLabel(item.Provider))
						ui.Text(c, item.Label).FontSize(u.fs(12)).Bold()
						keyText := maskAIKey(item.Key)
						if u.aiReveal[item.ID] {
							keyText = item.Key
						}
						ui.Text(c, keyText).FontSize(u.fs(10)).TextColor(t.TextMuted).SingleLine()
						if active {
							ui.Badge(c, "当前")
						} else {
							ui.Button(c, "使用").FontSize(u.fs(10)).OnClick(func() { u.selectAIKey(item.ID) })
						}
						reveal := "显示"
						if u.aiReveal[item.ID] {
							reveal = "隐藏"
						}
						ui.Button(c, reveal).FontSize(u.fs(10)).OnClick(func() {
							u.aiReveal[item.ID] = !u.aiReveal[item.ID]
						})
						ui.Button(c, "删除").FontSize(u.fs(10)).OnClick(func() {
							u.deleteAIKey(item.ID)
							c.Toast("已删除 API Key")
						})
					})
				}

				ui.Row(c).Gap(10).Wrap().AlignItems(ui.End).Children(func() {
					u.field(c, "Key 备注", 180, func() {
						ui.TextInput(c, &u.aiKeyLabel).Placeholder("例如：主账号")
					})
					ui.Column(c).Width(360).Gap(4).Children(func() {
						ui.Text(c, "新增 API Key").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
						ui.TextInput(c, &u.aiKeyInput).Password().Placeholder("输入新的 API Key")
					})
					ui.Button(c, "添加 Key").OnClick(func() {
						u.addAIKeyFromInput()
						c.Toast("已添加 API Key")
					})
				})

				ui.Row(c).Gap(10).Wrap().AlignItems(ui.Center).Children(func() {
					ui.PrimaryButton(c, "保存 AI 配置").OnClick(func() {
						u.saveAIConfig()
						c.Toast("AI 配置已保存")
					})
					ui.Button(c, "测试连接").Disabled(u.aiTesting).OnClick(u.testAIConfig)
					if u.aiTesting {
						ui.Spinner(c)
					}
					ui.Spacer(c)
					ui.Text(c, u.aiTestStatus).FontSize(u.fs(11)).TextColor(t.TextMuted)
				})
				ui.Text(c, "API Key 优先保存在系统凭证存储；系统不支持时退回仅当前用户可读的本地文件。密钥不会写入源码或安装目录。").
					FontSize(u.fs(10)).TextColor(t.TextMuted)
			})

			ui.Column(c).Padding(18).Gap(8).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, "关于应用").Bold().FontSize(u.fs(15))
				ui.Text(c, "BYR Client · 初始版本").FontSize(u.fs(13))
				ui.Text(c, "Go 业务层 + MyGo 原生界面；BYR 登录、目录、版面、主题、搜索与 DeepSeek/Kimi AI 接口已接入，SQLite 尚未接入。").
					FontSize(u.fs(11)).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Children(func() {
					ui.Badge(c, "Go")
					ui.Badge(c, "mygo")
					ui.Badge(c, "in-memory")
				})
			})

			ui.Column(c).Padding(18).Gap(8).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, "本地数据").Bold().FontSize(u.fs(15))
				stats := u.app.Stats()
				ui.Text(c, fmt.Sprintf("主题实体 %d · 回复实体 %d · 累计新增主题 %d · 更新主题 %d · 新增回复 %d",
					stats.Articles, stats.Replies, stats.InsertedArticles, stats.UpdatedArticles, stats.InsertedReplies)).
					FontSize(u.fs(11)).TextColor(t.TextMuted)
				ui.Text(c, "本轮版本重启后数据清空；SQLite 适配器将在后续接入。").
					FontSize(u.fs(10)).TextColor(t.TextMuted)
			})
		})
	})
}

func (u *UI) themeModeLabel() string {
	switch u.themeMode {
	case 1:
		return "浅色"
	case 2:
		return "深色"
	default:
		return "跟随系统"
	}
}
