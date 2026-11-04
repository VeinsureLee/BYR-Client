// Package fyneui contains the experimental Fyne presentation adapter. No
// original desktop settings, credentials or session files are read or written.
package fyneui

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"BYR-Client/internal/ai"
	"BYR-Client/internal/domain"
	"BYR-Client/internal/fyneproto"
	"BYR-Client/resources"
)

type page int

const (
	pageBrowse page = iota
	pageSearch
	pageChat
	pageChecks
)

type UI struct {
	application fyne.App
	window      fyne.Window
	controller  *fyneproto.Controller
	content     *contentRenderer
	state       fyneproto.Snapshot
	closed      atomic.Bool
	dirty       atomic.Bool
	queued      atomic.Bool
	done        chan struct{}

	page       page
	pageHost   *fyne.Container
	pages      []fyne.CanvasObject
	navButtons []*widget.Button
	status     *widget.Label

	boardSelect  *widget.Select
	boardOptions map[string]domain.BoardID
	boardTitle   *widget.Label
	boardCount   *widget.Label
	boardList    *widget.List
	boardMore    *widget.Button
	boardStamp   string

	browse       *fyne.Container
	browseLayout *browseLayout
	reading      bool
	reader       *widget.List
	readerTitle  *widget.Label
	readerCount  *widget.Label
	replyMore    *widget.Button
	replyAll     *widget.Button
	readerStamp  string
	readerWidth  float32
	rows         []replyRow
	replyOpen    map[string]bool
	expanded     map[string]bool

	query         *widget.Entry
	searchList    *widget.List
	searchStatus  *widget.Label
	searchResults []domain.Article
	searchStamp   string

	question   *widget.Entry
	answer     *widget.RichText
	answerText string
	chatScroll *container.Scroll
	chatStatus *widget.Label
	chatTrace  *widget.Label
	chatSend   *widget.Button
	chatStop   *widget.Button
	chatConfig *ai.Config
	chatMode   *widget.Label
	autoFollow *widget.Check
	checks     *widget.Label
}

func Run() {
	a := app.NewWithID("com.byrclient.fyneprototype")
	a.Settings().SetTheme(fontTheme(prototypeTheme{}))
	a.SetIcon(fyne.NewStaticResource("icon.png", resources.Icon))
	w := a.NewWindow("BYR · Fyne Prototype")
	u := New(a, w, fyneproto.NewDemo(fyneproto.Options{StressReplies: 1000}))
	w.SetContent(u.View())
	w.Resize(fyne.NewSize(1280, 820))
	w.SetOnClosed(u.Close)
	a.Lifecycle().SetOnExitedForeground(func() {
		// Mobile processes are not permanent background workers. Keep the
		// received answer but stop the stream when the application is suspended.
		if a.Driver().Device().IsMobile() {
			u.controller.StopChat()
		}
	})
	u.Start()
	if destination := os.Getenv("BYR_FYNE_VERIFY_DIR"); destination != "" {
		go u.verify(destination)
	}
	w.ShowAndRun()
	if destination := os.Getenv("BYR_FYNE_VERIFY_DIR"); destination != "" {
		if err := verificationResult(destination); err != nil {
			fmt.Fprintln(os.Stderr, "Fyne verification failed:", err)
			os.Exit(1)
		}
	}
}

func New(application fyne.App, window fyne.Window, controller *fyneproto.Controller) *UI {
	u := &UI{
		application: application, window: window, controller: controller,
		done: make(chan struct{}), boardOptions: make(map[string]domain.BoardID),
		replyOpen: make(map[string]bool), expanded: make(map[string]bool),
	}
	u.content = newContentRenderer(func(url string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return controller.ReadImage(ctx, url)
	}, window)
	u.build()
	controller.SetOnChange(func() { u.dirty.Store(true) })
	u.sync()
	return u
}

func (u *UI) View() fyne.CanvasObject {
	logo := canvas.NewImageFromResource(fyne.NewStaticResource("icon.png", resources.Icon))
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(36, 36))
	brand := canvas.NewText("BYR", ink)
	brand.TextSize = 23
	brand.TextStyle.Bold = true
	prototype := canvas.NewText("Fyne Prototype", muted)
	prototype.TextSize = 12
	u.status = widget.NewLabel("正在准备离线数据…")
	u.status.Truncation = fyne.TextTruncateEllipsis
	header := container.NewBorder(nil, nil,
		container.NewHBox(logo, container.NewVBox(brand, prototype)), nil,
		container.NewVBox(layout.NewSpacer(), u.status, layout.NewSpacer()))
	nav := container.NewGridWithColumns(4)
	labels := []string{"浏览", "搜索", "AI", "验证"}
	icons := []fyne.Resource{theme.HomeIcon(), theme.SearchIcon(), theme.MailComposeIcon(), theme.ConfirmIcon()}
	for i, label := range labels {
		index := page(i)
		button := widget.NewButtonWithIcon(label, icons[i], func() { u.showPage(index) })
		u.navButtons = append(u.navButtons, button)
		nav.Add(button)
	}
	u.showPage(pageBrowse)
	return container.NewBorder(padded(header, 8, 8, 14, 14), padded(nav, 6, 8, 10, 10), nil, nil,
		padded(u.pageHost, 0, 0, 10, 10))
}

func (u *UI) Start() {
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-u.done:
				return
			case <-ticker.C:
				if !u.dirty.Load() || !u.queued.CompareAndSwap(false, true) {
					continue
				}
				u.dirty.Store(false)
				fyne.Do(func() {
					u.queued.Store(false)
					if !u.closed.Load() {
						u.sync()
					}
				})
			}
		}
	}()
	u.controller.Start()
}

func (u *UI) Close() {
	if !u.closed.CompareAndSwap(false, true) {
		return
	}
	close(u.done)
	u.controller.SetOnChange(nil)
	u.content.Close()
	u.controller.Close()
	if u.chatConfig != nil {
		u.chatConfig.APIKey = ""
	}
}

func (u *UI) build() {
	u.pages = []fyne.CanvasObject{u.buildBrowse(), u.buildSearch(), u.buildChat(), u.buildChecks()}
	u.pageHost = container.NewStack(u.pages...)
}

func (u *UI) showPage(next page) {
	u.page = next
	for i, object := range u.pages {
		if page(i) == next {
			object.Show()
		} else {
			object.Hide()
		}
	}
	for i, button := range u.navButtons {
		button.Importance = widget.MediumImportance
		if page(i) == next {
			button.Importance = widget.HighImportance
		}
		button.Refresh()
	}
}

func (u *UI) sync() {
	next := u.controller.Snapshot()
	if next.SelectedBoard != u.state.SelectedBoard {
		u.boardList.UnselectAll()
	}
	u.state = next
	if u.status != nil {
		message := "离线演示 · 共用 Go 核心"
		if u.state.Loading {
			message = "正在加载…"
		}
		if u.state.Error != "" {
			message = u.state.Error
		}
		u.status.SetText(message)
	}
	u.syncBrowse()
	u.syncSearch()
	u.syncChat()
	u.checks.SetText(fmt.Sprintf("此原型复用现有 Go 目录、分页、搜索与 AI 服务。\n\n"+
		"• 数据来源：本地演示，不代表已通过真实论坛登录验收\n"+
		"• 默认 AI：模拟文本流；可在 AI 页面配置本次运行的真实模型\n"+
		"• 当前版面：已加载 %d 个主题\n• 当前帖子：已加载 %d 条回复\n"+
		"• 压力测试：1000 条回复，使用复用行的虚拟列表\n"+
		"• 窄屏：主题列表和正文之间切换；宽屏：分栏阅读\n"+
		"• 中文字体：内嵌开源 Noto Sans SC，可离线使用\n"+
		"• 原型不会读取、保存原客户端的账号、Cookie 或 API Key\n\n"+
		"请重点检查：中文选择复制、输入法、图文预览、回复折叠、连续滚动和 AI 停止。\n"+
		"手机后台会停止 AI 流，保留已经收到的文本。\n\n"+
		"构建成功与设备运行成功分别记录在验证文档中。",
		len(u.state.Board.Articles), len(u.state.Thread.Replies)))
}

func (u *UI) buildChecks() fyne.CanvasObject {
	u.checks = widget.NewLabel("")
	u.checks.Wrapping = fyne.TextWrapWord
	u.checks.Selectable = true
	title := widget.NewLabelWithStyle("跨端验证清单", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	license := widget.NewButton("字体开源许可", func() {
		text := widget.NewLabel(fontLicense)
		text.Wrapping = fyne.TextWrapBreak
		text.Selectable = true
		viewer := dialog.NewCustom("Noto Sans SC · SIL OFL 1.1", "关闭", container.NewVScroll(text), u.window)
		size := u.window.Canvas().Size()
		viewer.Resize(fyne.NewSize(min(float32(640), size.Width-28), min(float32(580), size.Height-90)))
		viewer.Show()
	})
	return panel(container.NewBorder(title, license, nil, nil, container.NewVScroll(u.checks)))
}

func (u *UI) configureAI() {
	provider := widget.NewSelect([]string{"DeepSeek", "Kimi"}, nil)
	provider.SetSelectedIndex(0)
	base := widget.NewEntry()
	model := widget.NewEntry()
	key := widget.NewPasswordEntry()
	base.SetText("https://api.deepseek.com/v1")
	model.SetText("deepseek-chat")
	provider.OnChanged = func(value string) {
		p := ai.DeepSeek
		if value == "Kimi" {
			p = ai.Kimi
		}
		address, name := ai.Defaults(p)
		base.SetText(address)
		model.SetText(name)
	}
	if u.chatConfig != nil {
		if u.chatConfig.Provider == ai.Kimi {
			provider.SetSelected("Kimi")
		}
		base.SetText(u.chatConfig.BaseURL)
		model.SetText(u.chatConfig.Model)
		key.SetText(u.chatConfig.APIKey)
	}
	form := dialog.NewForm("真实 AI · 仅本次运行", "使用", "取消", []*widget.FormItem{
		widget.NewFormItem("服务商", provider), widget.NewFormItem("API 地址", base),
		widget.NewFormItem("模型", model), widget.NewFormItem("API Key", key),
	}, func(ok bool) {
		if !ok {
			return
		}
		if strings.TrimSpace(key.Text) == "" {
			dialog.ShowInformation("尚未填写 API Key", "继续使用模拟流，或填入自己的 API Key。", u.window)
			return
		}
		p := ai.DeepSeek
		if provider.Selected == "Kimi" {
			p = ai.Kimi
		}
		config := ai.Normalize(ai.Config{Provider: p, APIKey: key.Text, BaseURL: base.Text, Model: model.Text})
		u.chatConfig = &config
		u.chatMode.SetText("真实模型 · " + config.Model)
	}, u.window)
	width := min(float32(580), u.window.Canvas().Size().Width-28)
	form.Resize(fyne.NewSize(width, 360))
	form.Show()
}

func subtle(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Importance = widget.LowImportance
	label.Truncation = fyne.TextTruncateEllipsis
	return label
}

func heading(text string) *widget.Label {
	label := widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	label.Truncation = fyne.TextTruncateEllipsis
	return label
}

func lineText(text string, size float32, colour color.Color) *canvas.Text {
	result := canvas.NewText(text, colour)
	result.TextSize = size
	return result
}
