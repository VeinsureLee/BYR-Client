package desktop

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/egoist/mygo/ui"
)

// Device is the consumer's small port. Tests provide an in-memory adapter;
// platform.Device supplies real MyGo dialogs, clipboard and browser actions.
type Device interface {
	Copy(string) error
	Open(string) error
	Pick(context.Context) (string, []byte, error)
	Save(string, []byte) error
}

// Page identifies a desktop route. Keeping route names out of arbitrary
// strings makes navigation APIs self-documenting without introducing a page
// hierarchy or inheritance tree.
type Page string

// Route is the navigation state for a desktop view.
type Route struct {
	Page          Page
	Name, ID, Sub string
	Number        int
}
type App struct {
	Core                    *application.App
	Version                 string
	device                  Device
	update                  func(func())
	ctx                     context.Context
	close                   context.CancelFunc
	cancel                  context.CancelFunc
	serial                  uint64
	closed, busy, restoring bool
	refreshing              bool
	threadRefreshPending    bool
	threadForceTop          bool
	threadRefreshClearSub   bool
	route                   Route
	back                    []Route
	listStates              map[listKey]*ui.ListState
	fontScale               float32
	windowWidth             float32
	windowHeight            float32
	threadReq               threadRequest
	threadReqSerial         uint64
	threadModels            map[string]*postModel
	threadLoadedRoute       Route
	threadLoadedRequest     Route
	threadRequestPage       int
	threadAnchorID          string
	threadViewport          viewportKey
	frameSerial             uint64
	visiblePlayers          int
	reduceMotion            bool
	message                 string
	historyNotice           string
	failed                  bool
	noticeSerial            uint64
	pageLoading             bool
	pageError, diagnostic   string
	retryLoad               func()
	username, password      string
	remember                bool
	sections                []forum.Section
	section                 forum.Section
	board                   forum.Board
	thread                  forum.Thread
	threadScrollPending     bool
	articles                []forum.Article
	boards                  []forum.Board
	favorites               forum.Favorite
	collection              forum.Collection
	pagination              forum.Pagination
	boardFilter             string
	searchQuery             string
	searchMode              string
	searchResults           []forum.Article
	searchPagination        forum.Pagination
	searchHasRun            bool
	advancedSearch          bool
	pageJump                string
	expandedPosts           map[listKey]map[string]bool
	pinnedExpanded          map[listKey]bool
	threadVoteAvailable     bool
	threadBetAvailable      bool
	threadResourcesBusy     bool
	threadResourcesDone     bool
	threadResourceBoard     string
	threadResourceID        string
	threadResourcesSerial   uint64
	threadResourcesCancel   context.CancelFunc
	user                    forum.User
	savedAccounts           []application.SavedAccount
	blacklist               forum.Blacklist
	refers                  forum.Refers
	mailbox                 forum.Mailbox
	mail                    forum.Mail
	contacts                []application.Contact
	chat                    []application.ChatMessage
	expandedMessages        map[string]bool
	mailEditor              mailEditor
	editor                  editor
	vote                    forum.Vote
	bet                     forum.Bet
	choice, score           string
	forward                 string
	confirm                 *confirmation
	images                  map[string]*imageEntry
	imageView               string
	infoText                string
	imageScale              float32
	emojiPage               int
	SystemDark              func() bool
}
type confirmation struct {
	text string
	run  func()
}

func New(core *application.App, device Device, update func(func())) *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{Core: core, Version: "开发版", device: device, update: update, ctx: ctx, close: cancel, route: Route{Page: "login"}, remember: true, searchMode: "all", images: map[string]*imageEntry{}, expandedPosts: map[listKey]map[string]bool{}, pinnedExpanded: map[listKey]bool{}, expandedMessages: map[string]bool{}, listStates: make(map[listKey]*ui.ListState), imageScale: 1, score: "10"}
}

// Start is explicit so constructing and testing a view never logs in or sends
// a request. The native composition root calls it after window creation.
func (a *App) Start() {
	a.restoring = true
	a.run("正在恢复登录", func(ctx context.Context) (func(), error) {
		err := a.Core.Restore(ctx)
		return func() {
			a.restoring = false
			if a.Core.Session().Token != "" {
				a.navigate(Route{Page: "home"}, false)
			}
		}, err
	})
}
func (a *App) Close() {
	if a.closed {
		return
	}
	a.closed = true
	a.busy = false
	a.serial++
	a.close()
	if a.cancel != nil {
		a.cancel()
	}
	a.cancelThreadPreflight()
	a.saveDraft()
	a.clearImages()
}
func (a *App) run(label string, work func(context.Context) (func(), error)) {
	if a.closed {
		return
	}
	if a.cancel != nil {
		a.cancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.serial++
	serial := a.serial
	a.busy = true
	a.message = label
	a.noticeSerial++
	a.failed = false
	a.pageLoading = false
	go func() {
		done, err := work(ctx)
		a.update(func() {
			if a.closed || serial != a.serial {
				return
			}
			a.busy = false
			a.restoring = false
			wasPageLoad := a.pageLoading
			a.pageLoading = false
			if err != nil {
				if a.refreshing {
					a.refreshing = false
				}
				if a.route.Page == "compose" {
					a.editor.Publishing = false
				}
				if !errors.Is(err, context.Canceled) {
					a.fail(err)
					if wasPageLoad {
						a.pageError = a.message
					}
				}
				return
			}
			a.message = ""
			if done != nil {
				done()
			}
			if a.refreshing {
				a.refreshing = false
				a.notice("刷新成功")
			}
		})
	}()
}

func (a *App) loadData(label string, work func(context.Context) (func(), error)) {
	a.pageError = ""
	a.retryLoad = func() { a.loadData(label, work) }
	a.run(label, work)
	a.pageLoading = true
}
func (a *App) fail(err error) {
	a.failed = true
	a.noticeSerial++
	a.diagnostic = err.Error()
	a.message = err.Error()
	var decode *forum.DecodeError
	if errors.As(err, &decode) {
		a.message = "论坛返回的数据格式不兼容，内容加载失败。请重试或复制诊断信息。"
	}
	var api *forum.APIError
	if errors.As(err, &api) && api.Message != "" {
		a.message = api.Message
	}
	if errors.As(err, &api) && api.Code == "65" {
		a.message = "用户名或密码错误（code 65），请使用论坛用户名登录"
	}
	if errors.Is(err, forum.ErrUnauthenticated) {
		a.message = "登录已失效，请退出后重新登录"
	}
}
func (a *App) notice(s string) {
	a.failed = false
	a.message = s
	a.noticeSerial++
	serial := a.noticeSerial
	if a.closed || a.update == nil {
		return
	}
	a.scheduleNoticeExpiry(s, serial)
}

func (a *App) scheduleNoticeExpiry(message string, serial uint64) {
	time.AfterFunc(3*time.Second, func() {
		if a.closed || a.update == nil {
			return
		}
		a.update(func() {
			if a.closed || a.noticeSerial != serial || a.failed || a.message != message {
				return
			}
			if a.busy {
				a.scheduleNoticeExpiry(message, serial)
				return
			}
			a.message = ""
			a.noticeSerial++
		})
	})
}
func (a *App) navigate(r Route, push bool) {
	if a.busy && !a.cancelPageRead() {
		return
	}
	if a.threadRefreshClearSub {
		a.threadRefreshClearSub = false
		a.route.Sub = ""
	}
	a.cancelThreadPreflight()
	if !a.saveDraft() {
		return
	}
	if push {
		a.back = append(a.back, a.route)
	} else {
		a.back = nil
	}
	a.route = r
	a.pageJump = ""
	if a.route.Number < 1 {
		a.route.Number = 1
	}
	a.confirm = nil
	a.message = ""
	a.historyNotice = ""
	a.pageError = ""
	a.retryLoad = nil
	a.imageView = ""
	a.load()
}
func (a *App) showLoadedUser(name string, user forum.User) {
	if a.busy || !a.saveDraft() {
		return
	}
	a.back = append(a.back, a.route)
	a.route = Route{Page: "user", Name: name, Number: 1}
	a.pageJump = ""
	a.confirm = nil
	a.message = ""
	a.pageError = ""
	a.retryLoad = nil
	a.imageView = ""
	a.pageLoading = false
	a.user = user
}

func (a *App) goBack() {
	if a.busy && !a.cancelPageRead() {
		return
	}
	if len(a.back) == 0 {
		return
	}
	if a.threadRefreshClearSub {
		a.threadRefreshClearSub = false
		a.route.Sub = ""
	}
	a.cancelThreadPreflight()
	if !a.saveDraft() {
		return
	}
	r := a.back[len(a.back)-1]
	a.back = a.back[:len(a.back)-1]
	a.route = r
	a.message = ""
	a.historyNotice = ""
	a.pageError = ""
	a.retryLoad = nil
	a.load()
}
func (a *App) View(c *ui.Context) {
	p := a.Core.Local.Preferences()
	dark := p.Theme == 2 || (p.Theme == 0 && a.SystemDark != nil && a.SystemDark())
	systemScale := c.Preferences().TextScale
	if systemScale <= 0 {
		systemScale = 1
	}
	a.fontScale = p.FontScale * systemScale
	if a.fontScale <= 0 {
		a.fontScale = 1
	}
	a.windowWidth, a.windowHeight = c.Size()
	a.frameSerial++
	a.visiblePlayers = 0
	a.reduceMotion = c.Preferences().ReduceMotion
	defer a.commitImages()
	c.SetTheme(forumTheme(dark, p.FontScale).WithPreferences(c.Preferences()))
	a.shell(c)
	if a.confirm != nil {
		open := true
		ui.Modal(c, &open, func() {
			ui.Text(c, a.confirm.text).MaxWidth(500)
			ui.Row(c).Gap(12).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.confirm = nil
				}
				if ui.PrimaryButton(c, "确认").Clicked() {
					action := a.confirm.run
					a.confirm = nil
					action()
				}
			})
		})
		if !open {
			a.confirm = nil
		}
	}
	if a.imageView != "" {
		a.imageModal(c)
	}
	if a.route.Page == "compose" && a.editor.LinkOpen {
		a.linkModal(c)
	}
	if a.infoText != "" {
		open := true
		ui.Modal(c, &open, func() {
			ui.Column(c).Width(680).Height(500).Gap(12).Children(func() {
				ui.Scroll(c).FillWidth().Grow(1).Children(func() { ui.Text(c, a.infoText).FillWidth().Selectable() })
				if ui.Button(c, "关闭说明").Clicked() {
					a.infoText = ""
				}
			})
		})
		if !open {
			a.infoText = ""
		}
	}
}

func (a *App) pageErrorView(c *ui.Context) {
	ui.Column(c).Fill().Center().Gap(12).Children(func() {
		ui.Column(c).MaxWidth(640).Gap(10).Padding(24).
			Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["alert"]).Size(22, 22).TextColor(c.Theme().Danger).Shrink(0)
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "内容加载失败").Bold().FontSize(17)
					ui.Text(c, a.pageError).FontSize(13).TextColor(c.Theme().TextMuted)
				})
			})
			if a.diagnostic != "" {
				ui.Column(c).FillWidth().Gap(4).Padding(10).
					Background(c.Theme().Background).Radius(8).Children(func() {
					ui.Text(c, "诊断信息").FontSize(11).Bold().TextColor(c.Theme().TextMuted)
					ui.Text(c, a.diagnostic).FontSize(11).TextColor(c.Theme().TextMuted).FillWidth().Selectable()
				})
			}
			ui.Row(c).Wrap().Gap(8).Children(func() {
				a.action(c, "重新加载", "refresh", filled, func() {
					if a.retryLoad != nil {
						a.retryLoad()
					} else {
						a.load()
					}
				})
				if a.device != nil {
					a.action(c, "复制加载诊断", "copy", quiet, func() {
						detail := a.diagnostic
						a.run("复制诊断", func(ctx context.Context) (func(), error) {
							return func() { a.notice("诊断信息已复制") }, a.device.Copy(detail)
						})
					})
				}
			})
		})
	})
}
func (a *App) title() string {
	switch a.route.Page {
	case "login":
		return "添加账号"
	case "home":
		return "分区与版面"
	case "board":
		return a.route.Name + " · " + a.board.Description
	case "thread":
		return "主题阅读"
	case "top":
		if a.route.Sub == "recommend" {
			return "版面推荐"
		}
		return "今日十大"
	case "timeline":
		return "时间线"
	case "favorites":
		return "我的收藏"
	case "compose":
		return "发帖 / 回复 / 编辑"
	case "search":
		return "搜索"
	case "contacts":
		return "私信"
	case "chat":
		return "与 " + a.route.Name + " 的私信"
	case "mailbox":
		return "站内信"
	case "mail":
		return a.mail.Title
	case "mail-compose":
		return "写信"
	case "refers":
		return "回复与 @ 提醒"
	case "self":
		return "个人中心"
	case "user":
		return a.user.DisplayName()
	case "history":
		return "阅读历史"
	case "blacklist":
		return "黑名单"
	case "settings":
		return "设置"
	case "accounts":
		return "账号管理"
	case "vote":
		return "投票"
	case "bet":
		return "竞猜"
	}
	return "BYR Client"
}
func (a *App) page(c *ui.Context) {
	switch a.route.Page {
	case "login":
		a.loginView(c)
	case "home":
		a.homeView(c)
	case "board":
		a.boardView(c)
	case "thread":
		a.threadView(c)
	case "top":
		a.articleList(c, a.articles)
	case "timeline":
		a.timelineView(c)
	case "favorites":
		a.favoritesView(c)
	case "compose":
		a.composeView(c)
	case "search":
		a.searchView(c)
	case "contacts":
		a.contactsView(c)
	case "chat":
		a.chatView(c)
	case "mailbox":
		a.mailboxView(c)
	case "mail":
		a.mailView(c)
	case "mail-compose":
		a.mailComposeView(c)
	case "refers":
		a.refersView(c)
	case "self", "user":
		a.userView(c)
	case "history":
		a.historyView(c)
	case "blacklist":
		a.blacklistView(c)
	case "settings":
		a.settingsView(c)
	case "accounts":
		a.accountsView(c)
	case "vote":
		a.voteView(c)
	case "bet":
		a.betView(c)
	}
}

// resolveSubsections fills human-readable names for subsection references
// (the API returns ["1","2"] without descriptions on the parent response).
// A failed lookup leaves the reference as-is rather than breaking the page.
func (a *App) resolveSubsections(ctx context.Context, section *forum.Section) {
	for i := range section.Subsections {
		sub := &section.Subsections[i]
		if sub.Description != "" || sub.Name == "" {
			continue
		}
		full, err := a.Core.Forum.Section(ctx, string(sub.Name))
		if err == nil {
			*sub = full
		}
	}
}

func (a *App) refreshCurrent() {
	if a.busy || a.closed {
		return
	}
	// A thread refresh keeps the displayed route (and therefore any back-stack
	// entry for it) until page 1 commits. The request page is tracked
	// separately so a failure can keep showing page 2 while retry asks page 1.
	if a.route.Page == "thread" {
		a.pageJump = ""
		a.threadRefreshPending = true
		a.loadThreadPage(1)
		return
	}
	a.route.Number = 1
	a.pageJump = ""
	a.route.Sub = ""
	switch a.route.Page {
	case "search":
		a.desktopListState(a.articleListKey()).ScrollTo(0, ui.Start)
		a.refreshing = true
		a.loadSearch()
		return
	default:
		if a.route.Page == "board" || a.route.Page == "timeline" {
			a.desktopListState(a.articleListKey()).ScrollTo(0, ui.Start)
		}
	}
	a.refreshing = true
	a.load()
}

func (a *App) load() {
	r := a.route
	p := forum.Page{Number: r.Number}
	f := a.Core.Forum
	switch r.Page {
	case "home":
		selected := string(a.section.Name)
		if r.Name != "" {
			selected = r.Name
		}
		a.loadData("加载分区与版面", func(ctx context.Context) (func(), error) {
			v, err := f.Sections(ctx)
			if err != nil {
				return nil, err
			}
			if selected == "" && len(v.Sections) > 0 {
				selected = string(v.Sections[0].Name)
			}
			var section forum.Section
			if selected != "" {
				section, err = f.Section(ctx, selected)
				if err != nil {
					return nil, err
				}
				a.resolveSubsections(ctx, &section)
			}
			return func() { a.sections = v.Sections; a.section = section; a.boards = section.Boards }, nil
		})
	case "board":
		a.board = forum.Board{Name: r.Name}
		a.loadData("加载版面", func(ctx context.Context) (func(), error) {
			v, err := f.Board(ctx, r.Name, p)
			return func() { a.board = v }, err
		})
	case "thread":
		a.loadThread()
	case "top":
		a.loadData("加载十大", func(ctx context.Context) (func(), error) {
			if r.Sub == "recommend" {
				v, err := f.Recommendations(ctx)
				return func() { a.articles = v.Articles }, err
			}
			v, err := f.TopTen(ctx)
			return func() { a.articles = v.Articles }, err
		})
	case "timeline":
		a.loadData("加载时间线", func(ctx context.Context) (func(), error) {
			v, err := f.FavorPosts(ctx, p)
			sort.SliceStable(v.Articles, func(i, j int) bool {
				return articleTime(v.Articles[i]) > articleTime(v.Articles[j])
			})
			return func() { a.articles = v.Articles; a.pagination = v.Pagination }, err
		})
	case "favorites":
		a.loadFavorites()
	case "compose":
		a.openEditor()
	case "self", "user":
		a.user = forum.User{}
		a.loadData("加载用户", func(ctx context.Context) (func(), error) {
			var v forum.User
			var err error
			if r.Page == "self" {
				v, err = f.Self(ctx)
			} else {
				v, err = f.User(ctx, r.Name)
			}
			return func() { a.user = v }, err
		})
	case "accounts":
		a.loadData("加载账号列表", func(ctx context.Context) (func(), error) {
			accounts, err := a.Core.SavedAccounts(ctx)
			return func() { a.savedAccounts = accounts }, err
		})
	case "blacklist":
		a.loadData("加载黑名单", func(ctx context.Context) (func(), error) {
			v, err := f.Blacklist(ctx, r.Number)
			return func() { a.blacklist = v }, err
		})
	case "contacts":
		a.loadData("加载私信", func(ctx context.Context) (func(), error) {
			v, err := a.Core.Contacts(ctx)
			return func() { a.contacts = v }, err
		})
	case "chat":
		a.loadData("加载会话正文", func(ctx context.Context) (func(), error) {
			v, err := a.Core.Conversation(ctx, r.Name)
			return func() { a.chat = v }, err
		})
	case "mailbox":
		if r.Name == "" {
			r.Name = "inbox"
			a.route.Name = r.Name
		}
		a.loadData("加载信箱", func(ctx context.Context) (func(), error) {
			v, err := f.Mailbox(ctx, r.Name, p)
			return func() { a.mailbox = v }, err
		})
	case "mail":
		a.mail = forum.Mail{}
		a.loadData("加载信件", func(ctx context.Context) (func(), error) {
			v, err := f.Mail(ctx, r.Name, r.ID)
			return func() { a.mail = v; a.mail.Box = r.Name }, err
		})
	case "mail-compose":
		a.mailEditor = mailEditor{To: r.Name, Box: r.Sub, ID: r.ID, Mode: "send"}
		if r.Sub != "" && r.ID != "" {
			a.mailEditor.Mode = "reply"
			a.mailEditor.Title = "Re: " + a.mail.Title
		}
	case "refers":
		if r.Name == "" {
			r.Name = "reply"
			a.route.Name = r.Name
		}
		a.loadData("加载提醒", func(ctx context.Context) (func(), error) {
			v, err := f.Refers(ctx, r.Name, p)
			return func() { a.refers = v }, err
		})
	case "vote":
		a.vote = forum.Vote{}
		a.choice = ""
		a.loadData("加载投票", func(ctx context.Context) (func(), error) {
			v, err := f.Vote(ctx, r.ID)
			return func() { a.vote = v }, err
		})
	case "bet":
		a.bet = forum.Bet{}
		a.choice = ""
		a.loadData("加载竞猜", func(ctx context.Context) (func(), error) {
			v, err := f.Bet(ctx, r.ID)
			return func() { a.bet = v }, err
		})
	}
}
func (a *App) loginView(c *ui.Context) {
	ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
		ui.Column(c).Width(440).Padding(30).Gap(14).
			Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(18).Children(func() {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Size(52, 52).Center().Background(accentSurface(c)).Radius(14).Children(func() {
					ui.Text(c, "BYR").FontSize(17).Bold().TextColor(c.Theme().Accent)
				})
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "登录北邮人论坛").FontSize(22).Bold()
					ui.Text(c, "使用论坛用户名和密码继续").FontSize(12).TextColor(c.Theme().TextMuted)
				})
			})
			ui.Divider(c)
			ui.TextInput(c, &a.username).Label("用户名").Placeholder("论坛用户名").FillWidth()
			pass := ui.TextInput(c, &a.password).Label("密码").Placeholder("密码").Password().FillWidth()
			ui.Checkbox(c, &a.remember, "记住登录（系统凭据库）")
			if ui.PrimaryButton(c, "登录").FillWidth().Disabled(a.busy).Clicked() || pass.Submitted() && !a.busy {
				u, p, remember := a.username, a.password, a.remember
				a.run("正在登录", func(ctx context.Context) (func(), error) {
					err := a.Core.Login(ctx, u, p, remember)
					return func() { a.password = ""; a.images = map[string]*imageEntry{}; a.navigate(Route{Page: "home"}, false) }, err
				})
			}
			if ui.Button(c, "网页登录诊断").Disabled(a.busy).Clicked() {
				u, p := a.username, a.password
				a.run("正在诊断", func(ctx context.Context) (func(), error) {
					v, err := a.Core.Forum.CheckWebLogin(ctx, u, p)
					return func() {
						a.notice(fmt.Sprintf("网页登录：成功=%t，ajax_code=%s。/open 接口仍需要 OAuth 登录。", v.OK(), v.Code))
					}, err
				})
			}
			if ui.Button(c, "管理已保存账号").Disabled(a.busy).Clicked() {
				a.navigate(Route{Page: "accounts"}, true)
			}
			if a.message != "" && a.device != nil {
				if ui.Button(c, "复制诊断信息").Clicked() {
					message := a.message
					a.run("复制", func(ctx context.Context) (func(), error) { return nil, a.device.Copy(message) })
				}
			}
		})
	})
}
func (a *App) pager(c *ui.Context, p forum.Pagination, change func(int)) {
	current := a.route.Number
	if a.route.Page == "thread" {
		current = a.threadDisplayPage()
	}
	if current < 1 {
		current = 1
	}
	ui.Row(c).Gap(12).Padding(4).Children(func() {
		if ui.Button(c, "上一页").Disabled(a.busy || current <= 1).Clicked() {
			change(current - 1)
		}
		ui.Textf(c, "第 %d / %d 页", current, max(1, int(p.TotalPages)))
		if ui.Button(c, "下一页").Disabled(a.busy || p.TotalPages == 0 || current >= int(p.TotalPages)).Clicked() {
			change(current + 1)
		}
	})
}
func (a *App) pageNumber(n int) {
	if a.route.Page == "thread" {
		a.loadThreadPage(n)
		return
	}
	a.route.Number = n
	a.load()
}
func formatTime(s forum.Scalar) string {
	v, err := strconv.ParseInt(string(s), 10, 64)
	if err != nil {
		return string(s)
	}
	if v <= 0 {
		return ""
	}
	return time.Unix(v, 0).Format("2006-01-02 15:04")
}

func articleTime(v forum.Article) int64 {
	n, err := strconv.ParseInt(string(v.PostTime), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
func empty(c *ui.Context, n int) {
	if n == 0 {
		ui.Text(c, "暂无内容").Padding(12).TextColor(c.Theme().TextMuted)
	}
}
func boardOf(v forum.Article, fallback string) string {
	if v.Board != "" {
		return v.Board
	}
	return fallback
}
func threadID(v forum.Article) string {
	if v.GroupID != "" {
		return string(v.GroupID)
	}
	return string(v.ID)
}
func (a *App) openArticle(v forum.Article) {
	a.navigate(Route{Page: "thread", Name: boardOf(v, a.route.Name), ID: threadID(v)}, true)
}
func (a *App) button(c *ui.Context, label string, action func()) {
	if ui.Button(c, label).Disabled(a.busy).Clicked() {
		action()
	}
}
func has(s, query string) bool { return strings.Contains(strings.ToLower(s), strings.ToLower(query)) }
