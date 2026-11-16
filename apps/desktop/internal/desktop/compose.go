package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
	"github.com/VeinsureLee/BYR-Client/core/resources"
	"github.com/egoist/mygo/ui"
)

type editor struct {
	Board, Title, Content, ReplyID, EditID string
	Meta                                   forum.Board
	Attachments                            []forum.Attachment
	Anonymous, Signature, Email, Outgo     bool
	Preview, Emoji, Ready, Submitted       bool
	Publishing                             bool
	PublishedID                            string
	SelectionStart, SelectionEnd           int
	SetSelection                           bool
	LinkOpen                               bool
	LinkURL, LinkText                      string
	ReplyFloor                             int
	ReplyAuthor, ReplyExcerpt              string
}

func (a *App) openEditor() {
	r := a.route
	a.editor = editor{Board: r.Name}
	a.boards = nil
	if r.Name == "" {
		a.loadData("加载常用版面", func(ctx context.Context) (func(), error) {
			v, err := a.Core.Forum.Favorites(ctx, "0")
			return func() { a.boards = v.Boards }, err
		})
		return
	}
	account := a.Core.Session().Username
	initialTitle := a.thread.Title
	a.loadData("准备编辑器", func(ctx context.Context) (func(), error) {
		meta, err := a.Core.Forum.Board(ctx, r.Name, forum.Page{Size: 1})
		if err != nil {
			return nil, err
		}
		e := editor{Board: r.Name, Meta: meta, Ready: true}
		if r.Sub == "edit" {
			e.EditID = r.ID
			v, err := a.Core.Forum.Article(ctx, r.Name, r.ID)
			if err != nil {
				return nil, err
			}
			e.Title, e.Content, e.Attachments = v.Title, v.Content, v.Attachments
		}
		if r.Sub == "reply" {
			e.ReplyID = r.ID
			e.Title = initialTitle
			if !strings.HasPrefix(strings.ToLower(e.Title), "re:") {
				e.Title = "Re: " + e.Title
			}
			// The target is optional context: a failed lookup must not prevent
			// replying, so this one request never blocks the editor.
			if target, lookupErr := a.Core.Forum.Article(ctx, r.Name, r.ID); lookupErr == nil {
				e.ReplyFloor = int(target.Position)
				e.ReplyAuthor = target.Author.DisplayName()
				e.ReplyExcerpt = excerpt(content.Plain(target.Content), 180)
			}
		}
		draft, exists := a.Core.Local.Draft(account, local.DraftKey(e.Board, e.ReplyID, e.EditID))
		if exists {
			e.Title, e.Content, e.Attachments = draft.Title, draft.Content, draft.Attachments
		}
		return func() {
			a.editor = e
			if exists {
				a.notice("已恢复本地草稿")
			}
		}, nil
	})
}
func (a *App) saveDraft() bool {
	e := a.editor
	if a.route.Page != "compose" || !e.Ready || e.Submitted || e.Publishing || e.Board == "" {
		return true
	}
	err := a.Core.Local.SaveDraft(context.Background(), a.Core.Session().Username, local.DraftKey(e.Board, e.ReplyID, e.EditID), local.Draft{Title: e.Title, Content: e.Content, Attachments: e.Attachments})
	if err != nil {
		a.fail(err)
		return false
	}
	return true
}
func (a *App) composeView(c *ui.Context) {
	if a.route.Name == "" {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.TextInput(c, &a.editor.Board).Label("发帖版面").Placeholder("输入版面名或搜索").Grow(1)
			a.button(c, "载入版面", func() {
				name := strings.TrimSpace(a.editor.Board)
				if name == "" {
					return
				}
				a.route.Name = name
				a.openEditor()
			})
			a.button(c, "查找版面", func() {
				keyword := a.editor.Board
				a.loadData("查找版面", func(ctx context.Context) (func(), error) {
					v, err := a.Core.Forum.SearchBoards(ctx, keyword)
					return func() { a.boards = v.Boards }, err
				})
			})
		})
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				for _, b := range a.boards {
					a.button(c, b.Description+" · "+b.Name, func() { a.route.Name = b.Name; a.openEditor() })
				}
				empty(c, len(a.boards))
			})
		})
		return
	}
	e := &a.editor
	ui.Textf(c, "版面：%s · %s", e.Board, e.Meta.Description)
	if !e.Ready {
		ui.Text(c, "正在读取版面权限与草稿…")
		return
	}
	a.replyContextView(c)
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		if ui.TextInput(c, &e.Title).Label("帖子标题").Placeholder("标题").FillWidth().Disabled(a.busy).Changed() {
			a.saveDraft()
		}
		ui.Row(c).Wrap().Gap(8).Children(func() {
			for _, tag := range []string{"b", "i", "u"} {
				tag := tag
				a.button(c, strings.ToUpper(tag), func() {
					e.Content, e.SelectionStart, e.SelectionEnd = content.WrapSelection(e.Content, e.SelectionStart, e.SelectionEnd, tag)
					e.SetSelection = true
					a.saveDraft()
				})
			}
			a.button(c, "插入链接", func() {
				e.LinkOpen = true
				e.LinkURL = ""
				r := []rune(e.Content)
				s, n := max(0, min(e.SelectionStart, len(r))), max(0, min(e.SelectionEnd, len(r)))
				if s > n {
					s, n = n, s
				}
				e.LinkText = string(r[s:n])
			})
			a.button(c, "表情", func() { e.Emoji = !e.Emoji })
			a.button(c, "添加附件", a.upload)
			a.button(c, "预览 / 编辑", func() { e.Preview = !e.Preview })
		})
		if e.Emoji {
			a.emojiPicker(c)
		}
	})
	if e.Preview {
		ui.Scroll(c).Grow(1).FillWidth().Children(func() { ui.Column(c).FillWidth().Gap(6).Children(func() { a.bodyView(c, e.Content, e.Attachments) }) })
	} else {
		area := ui.TextArea(c, &e.Content).Label("帖子正文").Placeholder("正文，支持 UBB 与图片附件").FillWidth().Grow(1).MinHeight(140).Disabled(a.busy)
		if e.SetSelection {
			area.SetTextSelection(e.SelectionStart, e.SelectionEnd)
			e.SetSelection = false
		}
		e.SelectionStart, e.SelectionEnd = area.TextSelection()
		if area.Changed() {
			a.saveDraft()
		}
	}
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		files := flatten(e.Attachments)
		for i, f := range files {
			ui.Row(c.Key("attachment-" + strconv.Itoa(i))).FillWidth().Gap(8).Children(func() {
				ui.Textf(c, "附件 %d：%s", i+1, f.Name)
				a.button(c, "删除附件", func() { a.removeAttachment(i, f) })
			})
		}
		ui.Row(c).Wrap().Gap(10).Children(func() {
			if e.Meta.AllowAnonymous {
				ui.Checkbox(c, &e.Anonymous, "匿名").Disabled(a.busy)
			}
			ui.Checkbox(c, &e.Signature, "签名").Disabled(a.busy)
			ui.Checkbox(c, &e.Email, "邮件提醒").Disabled(a.busy)
			if e.Meta.AllowOutgo {
				ui.Checkbox(c, &e.Outgo, "允许转出").Disabled(a.busy)
			}
		})
		ui.Row(c).Gap(10).Children(func() {
			if ui.PrimaryButton(c, "提交帖子").Disabled(a.busy || !e.Ready || e.Submitted).Clicked() {
				a.publish()
			}
			if e.Submitted {
				a.button(c, "查看已发布帖子", func() { a.navigate(Route{Page: "thread", Name: e.Board, ID: e.PublishedID}, true) })
			}
			a.button(c, "保存草稿", func() {
				a.saveDraft()
				if !a.failed {
					a.notice("草稿已保存")
				}
			})
			a.button(c, "清空草稿", func() {
				a.confirm = &confirmation{text: "清空当前标题、正文和本地草稿？", run: func() { e.Title, e.Content = "", ""; e.Attachments = nil; a.saveDraft() }}
			})
		})
	})
}
func flatten(attachments []forum.Attachment) []forum.File {
	var out []forum.File
	for _, att := range attachments {
		out = append(out, att.Files...)
	}
	return out
}
func (a *App) emojiPicker(c *ui.Context) {
	names := resources.EmojiNames()
	start := a.emojiPage * 24
	if start >= len(names) {
		start = 0
		a.emojiPage = 0
	}
	end := min(start+24, len(names))
	ui.Row(c).FillWidth().Wrap().Gap(4).Children(func() {
		for _, name := range names[start:end] {
			marker := "[" + strings.TrimSuffix(name, ".gif") + "]"
			ui.Row(c.Key(name)).Gap(2).AlignItems(ui.Center).Children(func() {
				a.image(c, "emoji:"+name, 22, 22, false)
				a.button(c, marker, func() { a.insertText(marker); a.saveDraft() })
			})
		}
	})
	ui.Row(c).Gap(8).Children(func() {
		a.button(c, "上一组表情", func() { a.emojiPage = max(0, a.emojiPage-1) })
		a.button(c, "下一组表情", func() {
			if end < len(names) {
				a.emojiPage++
			}
		})
	})
}
func (a *App) insertText(text string) {
	e := &a.editor
	next, caret := content.Insert(e.Content, e.SelectionStart, e.SelectionEnd, text)
	e.Content = next
	e.SelectionStart, e.SelectionEnd = caret, caret
	e.SetSelection = true
}
func (a *App) linkModal(c *ui.Context) {
	e := &a.editor
	ui.Modal(c, &e.LinkOpen, func() {
		ui.Column(c).Width(420).Gap(10).Children(func() {
			ui.Text(c, "插入链接").Bold()
			ui.TextInput(c, &e.LinkURL).Label("链接地址").Placeholder("https://...").FillWidth()
			ui.TextInput(c, &e.LinkText).Label("链接文字").Placeholder("显示文字").FillWidth()
			ui.Row(c).Gap(10).Children(func() {
				if ui.Button(c, "取消链接").Clicked() {
					e.LinkOpen = false
				}
				if ui.PrimaryButton(c, "插入").Clicked() {
					u, err := url.Parse(e.LinkURL)
					if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.Contains(e.LinkURL, "]") {
						a.fail(errors.New("请输入有效的 HTTP/HTTPS 地址"))
						return
					}
					text := e.LinkText
					if text == "" {
						text = e.LinkURL
					}
					a.insertText("[url=" + e.LinkURL + "]" + text + "[/url]")
					e.LinkOpen = false
					a.saveDraft()
				}
			})
		})
	})
}
func (a *App) upload() {
	e := a.editor
	a.saveDraft()
	id := e.EditID
	a.run("选择并上传附件", func(ctx context.Context) (func(), error) {
		name, b, err := a.device.Pick(ctx)
		if err != nil || name == "" {
			return nil, err
		}
		v, err := a.Core.Forum.Upload(ctx, e.Board, id, name, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		return func() {
			files := flatten(a.editor.Attachments)
			seen := map[string]bool{}
			for _, f := range files {
				seen[f.URL] = true
			}
			for _, f := range v.Files {
				if seen[f.URL] {
					continue
				}
				files = append(files, f)
				a.editor.Content += fmt.Sprintf("\n[upload=%d][/upload]", len(files))
				seen[f.URL] = true
			}
			a.editor.Attachments = []forum.Attachment{{Files: files, RemainCount: v.RemainCount, RemainSpace: v.RemainSpace}}
			a.saveDraft()
		}, nil
	})
}
func (a *App) removeAttachment(index int, f forum.File) {
	e := a.editor
	u, err := url.Parse(f.URL)
	if err != nil {
		a.fail(err)
		return
	}
	id := path.Base(u.Path)
	if id == "" || id == "." || id == "/" {
		a.fail(errors.New("附件地址缺少文件 ID"))
		return
	}
	a.confirm = &confirmation{text: "从服务器删除这个附件？", run: func() {
		a.run("删除附件", func(ctx context.Context) (func(), error) {
			_, err := a.Core.Forum.DeleteAttachment(ctx, e.Board, id)
			return func() {
				files := flatten(a.editor.Attachments)
				if index < len(files) {
					files = append(files[:index], files[index+1:]...)
				}
				a.editor.Attachments = []forum.Attachment{{Files: files}}
				a.editor.Content = renumberUploads(a.editor.Content, index+1)
				a.saveDraft()
			}, err
		})
	}}
}
func renumberUploads(body string, removed int) string {
	for n := removed; n <= 1000; n++ {
		old := fmt.Sprintf("[upload=%d][/upload]", n)
		if n == removed {
			body = strings.ReplaceAll(body, old, "")
		} else {
			body = strings.ReplaceAll(body, old, fmt.Sprintf("[upload=%d][/upload]", n-1))
		}
	}
	return body
}
func (a *App) replyContextView(c *ui.Context) {
	e := &a.editor
	if e.ReplyID == "" {
		return
	}
	label := "回复主题"
	if e.ReplyFloor > 0 {
		label = fmt.Sprintf("回复 #%d · %s", e.ReplyFloor, e.ReplyAuthor)
	}
	ui.Column(c).FillWidth().Padding(12, 14).
		Background(c.Theme().Surface.Mix(c.Theme().Accent, .06)).
		Border(1, c.Theme().Border.Mix(c.Theme().Accent, .22)).Radius(10).Gap(5).Children(func() {
		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(12).Bold().TextColor(c.Theme().Accent)
			if e.ReplyExcerpt != "" {
				a.action(c, "引用到正文", "quote", quiet, func() { a.insertQuote(e.ReplyExcerpt) })
			}
		})
		if e.ReplyExcerpt != "" {
			ui.Text(c, e.ReplyExcerpt).FontSize(12).TextColor(c.Theme().TextMuted).MaxLines(3)
		}
	})
}

func (a *App) insertQuote(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	e := &a.editor
	if strings.Contains(e.Content, text) {
		a.notice("正文中已经包含这段引用")
		return
	}
	a.insertText("[quote]" + text + "[/quote]\n\n")
	a.saveDraft()
	a.notice("已引用回复内容")
}

func excerpt(body string, limit int) string {
	body = strings.Join(strings.Fields(body), " ")
	runes := []rune(body)
	if len(runes) <= limit {
		return body
	}
	return string(runes[:limit]) + "…"
}

func (a *App) publish() {
	e := a.editor
	if !a.saveDraft() {
		return
	}
	p := forum.Post{Board: e.Board, Title: e.Title, Content: e.Content, ReplyID: e.ReplyID, Anonymous: e.Anonymous, Signature: e.Signature, Email: e.Email, Outgo: e.Outgo}
	if err := application.ValidatePost(e.Meta, p, e.EditID); err != nil {
		a.fail(err)
		return
	}
	a.editor.Publishing = true
	a.run("提交帖子", func(ctx context.Context) (func(), error) {
		result, err := a.Core.Publish(ctx, p, e.EditID)
		return func() {
			a.editor.Submitted = true
			a.editor.Publishing = false
			id := threadID(result.Article)
			if id == "" && len(a.back) > 0 && a.back[len(a.back)-1].Page == "thread" {
				id = a.back[len(a.back)-1].ID
			}
			if (e.ReplyID != "" || e.EditID != "") && len(a.back) > 0 {
				last := a.back[len(a.back)-1]
				if last.Page == "thread" && last.Name == e.Board {
					id = last.ID
				}
			}
			a.editor.PublishedID = id
			if result.LocalError != nil {
				a.fail(fmt.Errorf("帖子已发布，请勿重复提交；草稿清理失败：%w", result.LocalError))
				return
			}
			if len(a.back) > 0 {
				last := a.back[len(a.back)-1]
				if last.Page == "thread" && last.Name == e.Board && last.ID == id {
					a.back = a.back[:len(a.back)-1]
				}
			}
			if id == "" {
				a.route = Route{Page: "board", Name: e.Board, Number: 1}
			} else {
				a.route = Route{Page: "thread", Name: e.Board, ID: id, Number: 1}
			}
			a.load()
		}, err
	})
}
