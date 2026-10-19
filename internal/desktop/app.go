// Package desktop binds the application services to a MyGo native window.
// It owns presentation state only; network reads, search, storage and refresh
// scheduling stay in internal/app.
package desktop

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	svc "BYR-Client/internal/app"
	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
	"BYR-Client/internal/platform/appdata"
)

type region int

const (
	regionBrowse region = iota
	regionSearch
	regionMe
	regionInbox
	regionSettings
)

type scopeOption struct {
	Label string
	Node  forum.NodeID
}

type UI struct {
	app *svc.App

	windowMu sync.RWMutex
	win      *mygo.Window

	region        region
	returnRegion  region
	returnValid   bool
	themeMode     int
	accentMode    int
	fontScaleMode int
	fontScale     float32
	baseTheme     *ui.Theme
	themePicked   bool

	browseNode         forum.NodeID
	browseBoard        domain.BoardID
	browseThread       domain.ArticleKey
	treeOpen           map[forum.NodeID]bool
	replyOpen          map[string]bool
	lastHighlightFloor int
	splitWidth         float32
	splitHeight        float32

	keywordTokens   []string
	keywordDraft    string
	author          string
	timeKind        string
	from, to        time.Time
	scopeLabel      string
	scopeExtraLabel string
	scopeExtraNode  forum.NodeID
	showDetails     bool
	perBoard        float64
	total           float64
	threadPages     float64
	concurrency     float64
	searchError     string
	undoHint        bool

	activeHistory string
	activeRun     string
	run           svc.RunSnapshot
	collapsed     map[string]bool

	searchMode   int
	aiQuestion   string
	aiTimeKind   string
	aiScope      string
	aiDetails    bool
	aiTotal      float64
	aiPerBoard   float64
	aiFrom       time.Time
	aiTo         time.Time
	aiAnswer     string
	aiError      string
	aiStatus     string
	aiTrace      []string
	aiSources    []string
	aiRunning    bool
	chat         []chatTurn
	chatSeq      int
	activeChat   int
	md           markdown.Options
	aiRunToken   int
	aiHistoryID  string
	aiCancel     context.CancelFunc
	aiProvider   string
	aiModel      string
	aiBaseURL    string
	aiKey        string
	aiKeyInput   string
	aiKeyLabel   string
	aiKeySaved   bool
	aiKeys       []aiKeyEntry
	aiKeyID      string
	aiReveal     map[string]bool
	aiTesting    bool
	aiTestStatus string

	sourceKey   *domain.ArticleKey
	sourceFloor int

	loginUser        string
	loginPassword    string
	loginError       string
	loginStatus      string
	loginBusy        bool
	rememberLogin    bool
	rememberPassword bool
	closed           bool
	persist          bool
	settings         appdata.Settings

	images         map[string]*imageState
	browserWindows []*mygo.Window
	previewImage   domain.Image
	previewZoom    float32
	previewOpen    bool

	boardList    ui.ListState
	resultList   ui.ListState
	resultScroll ui.ScrollState
	threadScroll ui.ScrollState
	sourceScroll ui.ScrollState
	aiScroll     ui.ScrollState
}

func New(application *svc.App) *UI {
	u := newUI(false)
	u.app = application
	return u
}

func NewChooser() *UI {
	return newUI(true)
}

func newUI(persist bool) *UI {
	now := time.Now()
	settings := appdata.DefaultSettings()
	if persist {
		settings = appdata.LoadSettings()
	}
	u := &UI{
		persist:          persist,
		settings:         settings,
		themeMode:        settings.ThemeMode,
		accentMode:       settings.Accent,
		fontScaleMode:    settings.FontScale,
		fontScale:        fontScaleFor(settings.FontScale),
		splitWidth:       settings.SplitWidth,
		splitHeight:      settings.SplitHeight,
		rememberLogin:    settings.RememberLogin,
		rememberPassword: settings.RememberPassword,
		loginUser:        settings.LastUser,
		images:           make(map[string]*imageState),
		timeKind:         "不限",
		scopeLabel:       "全站",
		from:             now.AddDate(0, 0, -7),
		to:               now,
		perBoard:         domain.DefaultPerBoardLimit,
		total:            domain.DefaultTotalLimit,
		threadPages:      domain.DefaultThreadPages,
		concurrency:      domain.DefaultConcurrency,
		collapsed:        make(map[string]bool),
		treeOpen:         make(map[forum.NodeID]bool),
		replyOpen:        make(map[string]bool),
		aiTimeKind:       "近一个月",
		aiScope:          "全站",
		aiTotal:          50,
		aiPerBoard:       10,
		aiFrom:           now.AddDate(0, -1, 0),
		aiTo:             now,
		aiProvider:       aiProviderLabel(settings.AIProvider),
		aiModel:          settings.AIModel,
		aiBaseURL:        settings.AIBaseURL,
		aiReveal:         make(map[string]bool),
	}
	u.md.OnLink = func(url string) {
		_ = u.openLoggedInBrowserWindow(url)
	}
	return u
}

func Run(icon []byte) error {
	mygo.App.SetName("BYR Client")
	u := NewChooser()
	if os.Getenv("BYR_MODE") != "demo" {
		u.loadSavedCredentials()
	}
	u.loadAIConfig()
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleWindowMenu},
	}))
	mygo.App.OnDidBecomeActive(func() { u.setApplicationActive(true) })
	mygo.App.OnDidResignActive(func() { u.setApplicationActive(false) })
	mygo.App.OnQuit(func() { u.closeApp() })
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     "BYR Client",
			Width:     1320,
			Height:    860,
			MinWidth:  980,
			MinHeight: 640,
			StateKey:  "main",
			Content:   ui.View(u.view),
		})
		if len(icon) > 0 {
			_ = win.SetIcon(icon)
			_ = mygo.App.Dock.SetIcon(icon)
		}
		u.setWindow(win)
		if os.Getenv("BYR_MODE") == "demo" {
			u.startDemo()
		} else {
			u.tryRestoreSession()
		}
	})
	if err := mygo.App.Run(); err != nil {
		return err
	}
	return nil
}

func (u *UI) setWindow(win *mygo.Window) {
	u.windowMu.Lock()
	u.win = win
	u.windowMu.Unlock()
}

func (u *UI) window() *mygo.Window {
	u.windowMu.RLock()
	defer u.windowMu.RUnlock()
	return u.win
}

func (u *UI) invalidate() {
	if win := u.window(); win != nil {
		win.Invalidate()
	}
}

func (u *UI) saveSettings() {
	if !u.persist {
		return
	}
	u.settings.ThemeMode = u.themeMode
	u.settings.Accent = u.accentMode
	u.settings.FontScale = u.fontScaleMode
	u.settings.SplitWidth = u.splitWidth
	u.settings.SplitHeight = u.splitHeight
	u.settings.RememberLogin = u.rememberLogin
	u.settings.RememberPassword = u.rememberPassword
	_ = appdata.SaveSettings(u.settings)
}

func (u *UI) view(c *ui.Context) {
	u.captureTheme(c)
	u.applyTheme(c)
	if u.closed {
		return
	}
	if u.app == nil {
		u.loginView(c)
		return
	}
	u.poll()
	t := c.Theme()

	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		u.sidebar(c)
		ui.Column(c).Grow(1).FillHeight().Background(t.Background).Children(func() {
			u.header(c)
			ui.Divider(c)
			key := fmt.Sprintf("region-%d-%t", u.region, u.sourceKey != nil)
			ui.Column(c).Key(key).Grow(1).FillWidth().
				Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Y: 8}, Exit: &ui.Motion{Y: -6}}).
				Children(func() {
					switch u.region {
					case regionBrowse:
						u.browseContent(c)
					case regionSearch:
						u.searchContent(c)
					case regionMe:
						u.meContent(c)
					case regionInbox:
						u.inboxContent(c)
					case regionSettings:
						u.settingsContent(c)
					}
				})
		})
	})
	if u.previewOpen && u.previewImage.URL != "" {
		u.previewModal(c)
	}
}

func (u *UI) previewModal(c *ui.Context) {
	t := c.Theme()
	open := u.previewOpen
	image := u.previewImage
	maxWidth, maxHeight := float32(860), float32(580)
	title := "图片预览"
	if image.Kind == domain.ImageEmoji {
		maxWidth, maxHeight = 120, 120
		title = "表情预览"
	}
	state := u.images[image.URL]
	if state == nil {
		state = &imageState{}
		u.images[image.URL] = state
	}
	if state.bitmap == nil && !state.loading && state.err == "" {
		u.startImageLoad(image.URL, state)
	}
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(920).MaxWidthPercent(92).MaxHeightPercent(88).Padding(16).Gap(10).
			Background(t.Background).Radius(12).Border(1, t.Border).
			Shadow(0, 18, 50, 0, ui.RGBA(0, 0, 0, 0.28)).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).Bold().FontSize(u.fs(15))
				ui.Spacer(c)
				ui.Button(c, "关闭").OnClick(func() { u.previewOpen = false })
			})
			if state.bitmap == nil {
				ui.Box(c).Height(320).Center().Children(func() {
					u.imageView(c, image, maxWidth, maxHeight, false)
				})
			} else {
				width, height := state.bitmap.Size()
				fit := min(maxWidth/float32(width), maxHeight/float32(height), 1)
				zoom := u.previewZoom
				if zoom <= 0 {
					zoom = fit
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Button(c, "−").Padding(4, 10).OnClick(func() {
						u.previewZoom = max(0.1, zoom-0.25)
					})
					ui.Textf(c, "%d%%", int(zoom*100)).FontSize(u.fs(11)).Width(52)
					ui.Button(c, "+").Padding(4, 10).OnClick(func() {
						u.previewZoom = min(8, zoom+0.25)
					})
					ui.Button(c, "适应").FontSize(u.fs(11)).OnClick(func() { u.previewZoom = 0 })
					ui.Button(c, "100%").FontSize(u.fs(11)).OnClick(func() { u.previewZoom = 1 })
					ui.Spacer(c)
					ui.Textf(c, "%d × %d", width, height).FontSize(u.fs(10)).TextColor(t.TextMuted)
				})
				ui.ScrollBoth(c).MaxWidth(maxWidth).MaxHeight(maxHeight).Children(func() {
					ui.Column(c).Padding(8).Center().Children(func() {
						ui.Image(c, state.bitmap).Width(float32(width) * zoom).Height(float32(height) * zoom).Radius(8)
					})
				})
			}
			ui.Text(c, truncate(image.URL, 120)).FontSize(u.fs(10)).TextColor(t.TextMuted)
		})
	})
	if !open {
		u.previewOpen = false
	}
}

func (u *UI) captureTheme(c *ui.Context) {
	if u.themePicked {
		return
	}
	base := *c.Theme()
	u.baseTheme = &base
	u.themePicked = true
}

type accentPreset struct {
	Name                            string
	Light, LightHover, LightPressed ui.Color
	Dark, DarkHover, DarkPressed    ui.Color
	OnLight, OnDark                 ui.Color
}

var accentPresets = []accentPreset{
	{
		Name:  "蓝色",
		Light: ui.Hex("#355FC0"), LightHover: ui.Hex("#2B4FA6"), LightPressed: ui.Hex("#233F85"),
		Dark: ui.Hex("#ABC2FF"), DarkHover: ui.Hex("#C4D5FF"), DarkPressed: ui.Hex("#93ADEF"),
		OnLight: ui.Hex("#FFFFFF"), OnDark: ui.Hex("#172033"),
	},
	{
		Name:  "紫色",
		Light: ui.Hex("#7C3AED"), LightHover: ui.Hex("#6D28D9"), LightPressed: ui.Hex("#5B21B6"),
		Dark: ui.Hex("#C4B5FD"), DarkHover: ui.Hex("#DDD6FE"), DarkPressed: ui.Hex("#A78BFA"),
		OnLight: ui.Hex("#FFFFFF"), OnDark: ui.Hex("#201436"),
	},
	{
		Name:  "绿色",
		Light: ui.Hex("#15803D"), LightHover: ui.Hex("#166534"), LightPressed: ui.Hex("#14532D"),
		Dark: ui.Hex("#86EFAC"), DarkHover: ui.Hex("#BBF7D0"), DarkPressed: ui.Hex("#4ADE80"),
		OnLight: ui.Hex("#FFFFFF"), OnDark: ui.Hex("#0E2917"),
	},
	{
		Name:  "橙色",
		Light: ui.Hex("#C2410C"), LightHover: ui.Hex("#9A3412"), LightPressed: ui.Hex("#7C2D12"),
		Dark: ui.Hex("#FDBA74"), DarkHover: ui.Hex("#FED7AA"), DarkPressed: ui.Hex("#FB923C"),
		OnLight: ui.Hex("#FFFFFF"), OnDark: ui.Hex("#2B1608"),
	},
}

func (u *UI) applyTheme(c *ui.Context) {
	theme := *c.Theme()
	switch u.themeMode {
	case 1:
		theme = *ui.LightTheme()
	case 2:
		theme = *ui.DarkTheme()
	default:
		if u.baseTheme != nil {
			theme = *u.baseTheme
		}
	}
	theme = styledTheme(theme, u.accentMode, u.fontScale)
	c.SetTheme(&theme)
}

func styledTheme(theme ui.Theme, accentMode int, fontScale float32) ui.Theme {
	accent := accentPresets[0]
	if accentMode >= 0 && accentMode < len(accentPresets) {
		accent = accentPresets[accentMode]
	}
	if theme.Dark {
		theme.Accent = accent.Dark
		theme.AccentHover = accent.DarkHover
		theme.AccentPressed = accent.DarkPressed
		theme.AccentText = accent.OnDark
		theme.Selection = accent.Dark.Alpha(0.18)
		theme.Focus = accent.Dark
	} else {
		theme.Accent = accent.Light
		theme.AccentHover = accent.LightHover
		theme.AccentPressed = accent.LightPressed
		theme.AccentText = accent.OnLight
		theme.Selection = accent.Light.Alpha(0.12)
		theme.Focus = accent.Light
	}
	if fontScale <= 0 {
		fontScale = 1
	}
	theme.Radius = 8
	theme.Spacing = 5
	theme.FontSize = 13 * fontScale
	return theme
}

func fontScaleFor(mode int) float32 {
	switch mode {
	case 0:
		return 0.9
	case 2:
		return 1.15
	case 3:
		return 1.3
	default:
		return 1
	}
}

func (u *UI) fs(size float32) float32 {
	if u.fontScale <= 0 {
		return size
	}
	return size * u.fontScale
}

func (u *UI) poll() {
	if u.app == nil || u.activeRun == "" {
		return
	}
	if run, ok := u.app.Run(u.activeRun); ok {
		u.run = run
	}
}

func (u *UI) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(268).FillHeight().Background(t.Surface).
		BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).Gap(10).Padding(16, 16, 12, 16).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(30, 30).Radius(9).Background(t.Accent).Center().Children(func() {
				ui.Text(c, "B").Bold().FontSize(u.fs(15)).TextColor(t.AccentText)
			})
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "BYR Client").Bold().FontSize(u.fs(14))
				ui.Text(c, "北邮人论坛").FontSize(u.fs(11)).TextColor(t.TextMuted)
			})
		})
		ui.Column(c).PaddingX(10).Gap(2).Children(func() {
			u.navItem(c, iconBrowse, "浏览论坛", u.region == regionBrowse, func() {
				u.region = regionBrowse
			})
			u.navItem(c, iconSearch, "搜索", u.region == regionSearch, func() {
				u.region = regionSearch
			})
		})
		ui.Box(c).Padding(10, 12, 6, 12).Children(func() { ui.Divider(c) })
		if u.region == regionSearch {
			u.searchSidebar(c)
		} else {
			u.browseSidebar(c)
		}
		ui.Box(c).Padding(10, 12, 6, 12).Children(func() { ui.Divider(c) })
		ui.Column(c).Padding(10, 10).Gap(2).Children(func() {
			u.navItem(c, iconUser, "我的", u.region == regionMe, func() { u.openUtility(regionMe) })
			u.navItem(c, iconInbox, "私信", u.region == regionInbox, func() { u.openUtility(regionInbox) })
			u.navItem(c, iconSettings, "设置", u.region == regionSettings, func() { u.openUtility(regionSettings) })
		})
	})
}

func (u *UI) browseSidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).PaddingX(10).Gap(6).Children(func() {
		var roots []forum.Node
		if nav := u.app.Navigation(); nav != nil {
			roots, _ = nav.Children("")
		}
		ui.Row(c).Padding(4, 6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "论坛目录").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Textf(c, "%d 个分区", len(roots)).FontSize(u.fs(11)).TextColor(t.TextMuted)
		})
		ui.Scroll(c).Grow(1).Gap(4).ScrollbarInsets(2, 0, 2).Children(func() {
			selected := u.browseNode == ""
			row := ui.Row(c).Gap(8).Padding(6, 8).Radius(6).AlignItems(ui.Center).
				Cursor(ui.CursorPointer).OnClick(func() { u.selectNode("") })
			row = u.interactive(c, row, selected, ui.Transparent)
			row.Children(func() {
				ui.Icon(c, iconBrowse).FontSize(u.fs(14)).TextColor(t.Accent)
				ui.Text(c, "全部论坛").FontSize(u.fs(13))
			})
			ui.Tree(c, func() {
				for _, node := range roots {
					u.treeItem(c, node)
				}
			})
		})
	})
}

func (u *UI) treeItem(c *ui.Context, node forum.Node) {
	if node.Kind != forum.KindBoard {
		open := u.treeOpen[node.ID]
		item := ui.TreeItem(c.Key("tree-"+string(node.ID)), node.Name, &open, func() {
			children, err := u.app.Children(node.ID)
			if err != nil {
				return
			}
			for _, child := range children {
				u.treeItem(c, child)
			}
		})
		u.treeOpen[node.ID] = open
		if item.Selected(u.browseNode == node.ID).Clicked() {
			u.selectNode(node.ID)
		}
		return
	}
	item := ui.TreeItem(c.Key("tree-"+string(node.ID)), node.Name, nil, nil)
	if item.Selected(u.browseNode == node.ID).Clicked() {
		u.selectNode(node.ID)
	}
}

func (u *UI) searchSidebar(c *ui.Context) {
	t := c.Theme()
	pinned, recent := u.app.History()
	ui.Column(c).Grow(1).PaddingX(10).Gap(6).Children(func() {
		ui.Button(c, "新搜索").OnClick(func() { u.newSearch() })
		ui.Scroll(c).Grow(1).Gap(2).Children(func() {
			if len(pinned) > 0 {
				u.sidebarSection(c, "置顶搜索")
				for _, entry := range pinned {
					u.historyRow(c, entry)
				}
			}
			if len(recent) > 0 {
				u.sidebarSection(c, "最近搜索")
				for _, entry := range recent {
					u.historyRow(c, entry)
				}
			}
			if len(pinned) == 0 && len(recent) == 0 {
				ui.Text(c, "还没有搜索记录").FontSize(u.fs(12)).TextColor(t.TextMuted).Padding(6, 6)
			}
			if u.showUndoHint() {
				ui.Row(c).Gap(6).Padding(4, 6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "已删除一条记录").FontSize(u.fs(11)).TextColor(t.TextMuted)
					ui.Button(c, "撤销").OnClick(func() {
						u.app.UndoDelete()
						u.undoHint = false
					})
				})
			}
		})
	})
}

func (u *UI) sidebarSection(c *ui.Context, label string) {
	t := c.Theme()
	ui.Text(c, label).FontSize(u.fs(11)).Bold().TextColor(t.TextMuted).Padding(8, 6, 2, 6)
}

func (u *UI) historyRow(c *ui.Context, entry svc.HistorySnapshot) {
	t := c.Theme()
	row := ui.Row(c).Gap(8).Padding(5, 8).Radius(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	row.OnClick(func() { u.restoreHistory(entry) })
	row = u.interactive(c, row, entry.ID == u.activeHistory, ui.Transparent)
	row.Children(func() {
		icon := iconSearch
		if entry.Mode == svc.ModeAI {
			icon = iconSparkles
		}
		ui.Icon(c, icon).FontSize(u.fs(14)).TextColor(t.TextMuted)
		ui.Text(c, truncate(entry.Title, 18)).FontSize(u.fs(12)).Grow(1).SingleLine()
		if row.Hovered() || entry.ID == u.activeHistory {
			pinLabel := "置顶"
			if entry.Pinned {
				pinLabel = "取消置顶"
			}
			ui.Button(c, "").Label(pinLabel).Tooltip(pinLabel).Padding(2, 2).Radius(5).
				Children(func() { ui.Icon(c, iconPin).FontSize(u.fs(12)) }).
				OnClick(func() { u.app.PinHistory(entry.ID, !entry.Pinned) })
			ui.Button(c, "").Label("删除").Tooltip("删除").Padding(2, 2).Radius(5).
				Children(func() { ui.Icon(c, iconTrash).FontSize(u.fs(12)) }).
				OnClick(func() {
					if u.app.DeleteHistory(entry.ID) {
						u.undoHint = true
						if entry.ID == u.aiHistoryID {
							u.stopAI()
							u.aiHistoryID = ""
							u.chat = nil
							u.chatSeq = 0
							u.activeChat = 0
						}
					}
				})
		}
	})
}

func (u *UI) header(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).MinHeight(58).Padding(8, 18).Gap(10).AlignItems(ui.Center).Children(func() {
		if u.region == regionSearch && u.sourceKey != nil {
			u.iconButton(c, iconArrowLeft, "返回搜索", func() {
				u.sourceKey = nil
				u.sourceFloor = 0
			})
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "来源原文").Bold().FontSize(u.fs(16))
				ui.Text(c, "返回搜索可保留结果、折叠和输入草稿").FontSize(u.fs(11)).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			return
		}
		switch u.region {
		case regionBrowse:
			u.browseHeader(c)
		case regionSearch:
			u.searchHeader(c)
		case regionMe:
			u.utilityHeader(c, "我的")
		case regionInbox:
			u.utilityHeader(c, "私信")
		case regionSettings:
			u.utilityHeader(c, "设置")
		}
	})
}

func (u *UI) browseHeader(c *ui.Context) {
	t := c.Theme()
	if u.browseNode != "" {
		u.iconButton(c, iconArrowLeft, "返回上级", func() {
			path := u.app.Breadcrumb(u.browseNode)
			if len(path) > 1 {
				u.selectNode(path[len(path)-2].ID)
			} else {
				u.selectNode("")
			}
		})
	}
	path := u.app.Breadcrumb(u.browseNode)
	if len(path) == 0 {
		ui.Text(c, "浏览论坛").Bold().FontSize(u.fs(18)).Grow(1)
	} else {
		ui.Text(c, "全部论坛 › "+joinPath(path)).FontSize(u.fs(13)).TextColor(t.TextMuted).
			SingleLine().Grow(1)
	}
	if u.browseBoard != "" {
		ui.Button(c, "").Label("刷新版面").Tooltip("刷新当前版面").Padding(4, 6).Radius(6).
			Children(func() { ui.Icon(c, iconRefresh).FontSize(u.fs(14)) }).
			OnClick(func() {
				u.openBoard(u.browseBoard)
				u.loadTraffic(u.browseNode, true)
			})
	}
	ui.Text(c, u.refreshHint()).FontSize(u.fs(11)).TextColor(t.TextMuted)
}

func (u *UI) breadcrumb(c *ui.Context, path []forum.Node) {
	t := c.Theme()
	ui.Row(c).Gap(2).AlignItems(ui.Center).Grow(1).Wrap().Children(func() {
		if ui.Button(c, "全部论坛").FontSize(u.fs(12)).Padding(2, 6).Shrink(0).OnClick(func() { u.selectNode("") }).Clicked() {
			return
		}
		for _, node := range path {
			ui.Text(c, "›").FontSize(u.fs(12)).TextColor(t.TextMuted).Shrink(0)
			n := node
			style := ui.Button(c, n.Name).FontSize(u.fs(12)).Padding(2, 6).Shrink(0)
			if n.ID == u.browseNode {
				style.Bold()
			}
			if style.Clicked() {
				u.selectNode(n.ID)
			}
		}
	})
}

func (u *UI) searchHeader(c *ui.Context) {
	t := c.Theme()
	title := "搜索"
	if entry, ok := u.app.HistoryEntry(u.activeHistory); ok {
		title = entry.Title
		ui.Text(c, truncate(title, 42)).Bold().FontSize(u.fs(16)).SingleLine()
		ui.Spacer(c)
		pinLabel := "置顶"
		if entry.Pinned {
			pinLabel = "取消置顶"
		}
		ui.Button(c, "").Label(pinLabel).Tooltip(pinLabel).Padding(4, 6).Radius(6).
			Children(func() { ui.Icon(c, iconPin).FontSize(u.fs(14)) }).
			OnClick(func() { u.app.PinHistory(entry.ID, !entry.Pinned) })
	} else {
		ui.Text(c, title).Bold().FontSize(u.fs(18))
		ui.Spacer(c)
		ui.Text(c, "关键词与 AI 问答按各自条件独立保存").FontSize(u.fs(11)).TextColor(t.TextMuted)
	}
}

func (u *UI) interactive(c *ui.Context, e ui.Element, selected bool, base ui.Color) ui.Element {
	t := c.Theme()
	background := base
	switch {
	case selected:
		background = t.Selection
	case e.Pressed():
		background = t.SurfacePressed
	case e.Hovered():
		background = t.SurfaceHover
	}
	e.Background(background)
	e.Transition(ui.ElementTransition{Colors: true, Duration: 130 * time.Millisecond})
	pose := ui.Pose{}
	if e.Pressed() {
		pose = ui.Pose{Y: 1, Scale: 0.987}
	}
	e.AnimatePose(pose, ui.Snappy)
	return e
}

func (u *UI) openUtility(target region) {
	if u.region != regionMe && u.region != regionInbox && u.region != regionSettings {
		u.returnRegion = u.region
		u.returnValid = true
	}
	u.region = target
}

func (u *UI) utilityHeader(c *ui.Context, title string) {
	u.iconButton(c, iconArrowLeft, "返回", func() {
		if u.returnValid {
			u.region = u.returnRegion
			u.returnValid = false
			return
		}
		u.region = regionBrowse
	})
	ui.Text(c, title).Bold().FontSize(u.fs(18))
}

func (u *UI) navItem(c *ui.Context, icon *ui.SVG, label string, selected bool, action func()) {
	t := c.Theme()
	row := ui.Row(c).Gap(10).Padding(7, 9).Radius(7).AlignItems(ui.Center).
		Cursor(ui.CursorPointer).OnClick(action)
	row = u.interactive(c, row, selected, ui.Transparent)
	row.Children(func() {
		color := t.TextMuted
		if selected {
			color = t.Accent
		}
		ui.Icon(c, icon).FontSize(u.fs(15)).TextColor(color)
		text := ui.Text(c, label).FontSize(u.fs(13))
		if selected {
			text.TextColor(t.Accent).Bold()
		}
	})
}

func (u *UI) iconButton(c *ui.Context, icon *ui.SVG, label string, action func()) {
	ui.Button(c, "").Label(label).Tooltip(label).Padding(4, 6).Radius(6).
		Children(func() { ui.Icon(c, icon).FontSize(u.fs(15)) }).OnClick(action)
}

func (u *UI) selectNode(id forum.NodeID) {
	u.region = regionBrowse
	u.returnValid = false
	u.browseNode = id
	u.browseBoard = ""
	u.browseThread = domain.ArticleKey{}
	if id != "" {
		for _, node := range u.app.Breadcrumb(id) {
			u.treeOpen[node.ID] = true
		}
		nav := u.app.Navigation()
		if nav != nil {
			node, ok := nav.Node(id)
			if ok && node.Kind == forum.KindBoard {
				u.openBoard(node.BoardID)
				return
			}
		}
		u.requestTraffic(id)
	}
}

func (u *UI) openBoard(board domain.BoardID) {
	application := u.app
	if application == nil {
		return
	}
	u.browseBoard = board
	u.browseThread = domain.ArticleKey{}
	if u.browseNode != "" {
		u.requestTraffic(u.browseNode)
	}
	application.ActivateBoard(board)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := application.OpenBoard(ctx, board); err != nil {
			log.Printf("open board %s: %v", board, err)
		}
	}()
}

func (u *UI) openThread(key domain.ArticleKey) {
	application := u.app
	if application == nil {
		return
	}
	u.browseThread = key
	application.ActivateThread(key)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := application.OpenThread(ctx, key); err != nil {
			log.Printf("open thread %s: %v", key.ArticleID, err)
		}
	}()
}

func (u *UI) openSource(key domain.ArticleKey, floor int) {
	application := u.app
	if application == nil {
		return
	}
	copy := key
	u.sourceKey = &copy
	u.sourceFloor = floor
	u.sourceScroll.Y = 0
	u.lastHighlightFloor = 0
	application.ActivateThread(key)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := application.OpenThread(ctx, key); err != nil {
			log.Printf("open source %s: %v", key.ArticleID, err)
		}
	}()
}

func (u *UI) restoreHistory(entry svc.HistorySnapshot) {
	u.activeHistory = entry.ID
	u.sourceKey = nil
	u.sourceFloor = 0
	u.resultScroll.Y = 0
	u.scopeExtraNode = ""
	u.scopeExtraLabel = ""
	if entry.Mode == svc.ModeAI {
		u.stopAIContext()
		u.activeRun = ""
		u.searchMode = 1
		u.aiQuestion = ""
		u.aiAnswer = entry.AIAnswer
		u.aiError = ""
		u.aiStatus = ""
		u.aiTrace = nil
		u.aiSources = nil
		u.chat = []chatTurn{
			{ID: 1, Role: "user", Text: entry.Title},
			{ID: 2, Role: "assistant", Provider: u.aiProvider, Model: u.aiModel, Text: entry.AIAnswer},
		}
		u.chatSeq = 2
		u.activeChat = 2
		u.aiHistoryID = entry.ID
		u.aiRunning = false
		u.aiRunToken++
		u.aiTimeKind = timeKindLabel(entry.Query.Time.Kind)
		u.aiScope = u.scopeLabelFor(entry.Query.ScopeNode)
		u.aiTotal = float64(entry.Query.Total)
		u.aiPerBoard = float64(entry.Query.PerBoard)
		if entry.Query.Time.Kind == domain.TimeCustom {
			u.aiFrom = entry.Query.Time.From
			u.aiTo = entry.Query.Time.To
		}
		return
	}
	u.searchMode = 0
	u.activeRun = entry.ID
	u.keywordTokens = append([]string(nil), entry.Query.Keywords...)
	u.keywordDraft = ""
	u.author = entry.Query.Author
	u.perBoard = float64(entry.Query.PerBoard)
	u.total = float64(entry.Query.Total)
	u.threadPages = float64(entry.Query.ThreadPages)
	u.concurrency = float64(entry.Query.Concurrency)
	u.timeKind = timeKindLabel(entry.Query.Time.Kind)
	if entry.Query.Time.Kind == domain.TimeCustom {
		u.from = entry.Query.Time.From
		u.to = entry.Query.Time.To
	}
	if entry.Query.ScopeNode != "" {
		path := u.app.Breadcrumb(entry.Query.ScopeNode)
		if len(path) > 0 {
			u.scopeExtraNode = entry.Query.ScopeNode
			u.scopeExtraLabel = joinPath(path)
		}
	}
	u.scopeLabel = u.scopeLabelFor(entry.Query.ScopeNode)
	u.showDetails = entry.Query.PerBoard != domain.DefaultPerBoardLimit ||
		entry.Query.Total != domain.DefaultTotalLimit ||
		entry.Query.ThreadPages != domain.DefaultThreadPages ||
		entry.Query.Concurrency != domain.DefaultConcurrency ||
		entry.Query.ScopeNode != ""
	if run, ok := u.app.Run(entry.ID); ok {
		u.run = run
	}
	u.region = regionSearch
}

func (u *UI) newSearch() {
	u.region = regionSearch
	u.activeHistory = ""
	u.activeRun = ""
	u.run = svc.RunSnapshot{}
	u.keywordTokens = nil
	u.keywordDraft = ""
	u.author = ""
	u.timeKind = "不限"
	u.scopeLabel = "全站"
	u.scopeExtraNode = ""
	u.scopeExtraLabel = ""
	u.showDetails = false
	u.perBoard = domain.DefaultPerBoardLimit
	u.total = domain.DefaultTotalLimit
	u.threadPages = domain.DefaultThreadPages
	u.concurrency = domain.DefaultConcurrency
	u.searchError = ""
	u.sourceKey = nil
	u.sourceFloor = 0
	u.resultScroll.Y = 0
	if u.searchMode == 1 {
		u.stopAIContext()
		u.aiQuestion = ""
		u.aiAnswer = ""
		u.aiError = ""
		u.aiStatus = ""
		u.aiTrace = nil
		u.aiSources = nil
		u.chat = nil
		u.chatSeq = 0
		u.activeChat = 0
		u.aiHistoryID = ""
		u.aiRunning = false
		u.aiRunToken++
	}
}

func (u *UI) scopeOptions() []scopeOption {
	options := []scopeOption{{Label: "全站", Node: ""}}
	if u.browseNode != "" {
		options = append(options, scopeOption{Label: "当前浏览节点", Node: u.browseNode})
	}
	nav := u.app.Navigation()
	if nav != nil {
		roots, err := nav.Children("")
		if err == nil {
			for _, node := range roots {
				options = append(options, scopeOption{Label: node.Name, Node: node.ID})
			}
		}
	}
	if u.scopeExtraNode != "" {
		exists := false
		for _, option := range options {
			if option.Node == u.scopeExtraNode {
				exists = true
				break
			}
		}
		if !exists {
			label := u.scopeExtraLabel
			if label == "" {
				label = "提交时的范围"
			}
			options = append(options, scopeOption{Label: label, Node: u.scopeExtraNode})
		}
	}
	return options
}

func (u *UI) scopeLabels() []string {
	options := u.scopeOptions()
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = option.Label
	}
	return labels
}

func (u *UI) scopeNodeFor(label string) forum.NodeID {
	for _, option := range u.scopeOptions() {
		if option.Label == label {
			return option.Node
		}
	}
	return ""
}

func (u *UI) scopeLabelFor(node forum.NodeID) string {
	for _, option := range u.scopeOptions() {
		if option.Node == node {
			return option.Label
		}
	}
	return "全站"
}

func (u *UI) refreshHint() string {
	if u.browseBoard == "" {
		return "点击节点查看内容"
	}
	if lease := u.app.RefreshState(); lease.ID != 0 {
		return "10 分钟无操作自动刷新"
	}
	return ""
}

func (u *UI) showUndoHint() bool {
	return u.undoHint
}

func timeKindLabel(kind domain.TimeRangeKind) string {
	switch kind {
	case domain.TimeWeek:
		return "近一周"
	case domain.TimeMonth:
		return "近一个月"
	case domain.TimeCustom:
		return "自定义"
	default:
		return "不限"
	}
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 1 {
		return string(runes[:n])
	}
	return string(runes[:n-1]) + "…"
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "时间未知"
	}
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + " 分钟前"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + " 小时前"
	case d < 7*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/24)) + " 天前"
	default:
		return t.Format("2006-01-02")
	}
}

func joinPath(path []forum.Node) string {
	names := make([]string, 0, len(path))
	for _, node := range path {
		names = append(names, node.Name)
	}
	return strings.Join(names, " › ")
}

func statusText(run svc.RunSnapshot) string {
	switch run.Status {
	case svc.StatusQueued:
		return "排队中…"
	case svc.StatusRunning:
		return fmt.Sprintf("正在获取… 已收到 %d 个主题、%d 条回复", run.ArticleCount, run.ReplyCount)
	case svc.StatusStopped:
		return fmt.Sprintf("已停止，保留 %d 个主题、%d 条回复", run.ArticleCount, run.ReplyCount)
	case svc.StatusFailed:
		return "搜索失败"
	case svc.StatusPartial:
		return fmt.Sprintf("部分完成：%d 个主题、%d 条回复", run.ArticleCount, run.ReplyCount)
	}
	switch run.StopReason {
	case svc.StopResultLimit:
		return fmt.Sprintf("已达到总共 %d 条主题上限", run.Query.Total)
	case svc.StopPerBoard:
		return fmt.Sprintf("所有版面均达到每版 %d 条上限", run.Query.PerBoard)
	case svc.StopUserCancel:
		return "已停止"
	case svc.StopError:
		return "部分版面读取失败"
	default:
		return fmt.Sprintf("本轮范围已读完，共 %d 个主题、%d 条回复", run.ArticleCount, run.ReplyCount)
	}
}
