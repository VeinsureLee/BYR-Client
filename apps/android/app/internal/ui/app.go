package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/font/opentype"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/VeinsureLee/BYR-Client/apps/android/app/internal/platform"
	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
	"github.com/VeinsureLee/BYR-Client/core/resources/fonts"
)

type Page uint8

const (
	PageLogin Page = iota
	PageHome
	PageSection
	PageTop
	PageBoard
	PageThread
	PageTimeline
	PageFavorites
	PageSearch
	PageMessages
	PageChat
	PageMailbox
	PageMail
	PageMailCompose
	PageRefers
	PageSelf
	PageUser
	PageHistory
	PageBlacklist
	PageSettings
	PageVote
	PageBet
	PageCompose
)

type Route struct {
	Page   Page
	Name   string
	ID     string
	Sub    string
	Number int
}

type invalidator interface{ Invalidate() }

type App struct {
	window  invalidator
	version string
	theme   *material.Theme
	icons   map[string]*widget.Icon

	updates chan func()
	ctx     context.Context
	close   context.CancelFunc

	started bool
	ready   bool
	initErr string
	busy    bool
	message string
	failed  bool

	core *application.App
	dir  string

	stack []Route

	sections   []forum.Section
	section    forum.Section
	boards     []forum.Board
	board      forum.Board
	thread     forum.Thread
	articles   []forum.Article
	favorite   forum.Favorite
	collect    forum.Collection
	pagination forum.Pagination
	user       forum.User
	history    []local.History
	blacklist  forum.Blacklist
	vote       forum.Vote
	bet        forum.Bet

	contacts    []application.Contact
	chat        []application.ChatMessage
	mailbox     forum.Mailbox
	mail        forum.Mail
	refers      forum.Refers
	mailEditor  mailEditor
	blacklistIn widget.Editor
	scoreIn     widget.Editor
	forwardIn   widget.Editor
	ipv6        widget.Bool
	choice      string

	username   widget.Editor
	password   widget.Editor
	remember   widget.Bool
	searchEdit widget.Editor
	searchMode string

	homeList, sectionList, boardList, threadList, timelineList, favoriteList, selfList, composeList, topList, auxList, searchList widget.List

	clicks map[string]*widget.Clickable

	favoriteTab string
	topTab      string
	compose     composeState
	lastClip    string
	images      map[string]*mobileImage
	closed      atomic.Bool
	themed      bool
	dark        bool
	pageKey     string
	renderPanic string
	startAt     time.Time
	busySince   time.Time
}

type composeState struct {
	board, title, content, replyID, editID string
	titleEditor, contentEditor             widget.Editor
	anonymous, signature, email, outgo     widget.Bool
	boardLocked, ready, loading            bool
	publishing, submitted                  bool
	publishedID                            string
}

func New(window invalidator) (*App, error) {
	faces, err := opentype.ParseCollection(fonts.Regular)
	if err != nil {
		return nil, err
	}
	theme := material.NewTheme()
	theme.Shaper = text.NewShaper(text.WithCollection(faces), text.NoSystemFonts())
	theme.Face = faces[0].Font.Typeface
	theme.TextSize = 15
	theme.Bg = colorBg
	theme.Fg = colorFg
	theme.ContrastBg = colorAccent
	theme.ContrastFg = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		window: window, version: "dev", theme: theme, ctx: ctx, close: cancel,
		updates: make(chan func(), 128), clicks: map[string]*widget.Clickable{}, images: map[string]*mobileImage{},
		stack: []Route{{Page: PageLogin}}, favoriteTab: "boards", topTab: "top", searchMode: "ai",
		homeList:     widget.List{List: layout.List{Axis: layout.Vertical}},
		sectionList:  widget.List{List: layout.List{Axis: layout.Vertical}},
		boardList:    widget.List{List: layout.List{Axis: layout.Vertical}},
		threadList:   widget.List{List: layout.List{Axis: layout.Vertical}},
		timelineList: widget.List{List: layout.List{Axis: layout.Vertical}},
		favoriteList: widget.List{List: layout.List{Axis: layout.Vertical}},
		selfList:     widget.List{List: layout.List{Axis: layout.Vertical}},
		composeList:  widget.List{List: layout.List{Axis: layout.Vertical}},
		topList:      widget.List{List: layout.List{Axis: layout.Vertical}},
		auxList:      widget.List{List: layout.List{Axis: layout.Vertical}},
		searchList:   widget.List{List: layout.List{Axis: layout.Vertical}},
	}
	a.searchEdit.SingleLine = true
	a.username.SingleLine = true
	// Email-type IME keeps English usernames free of sentence capitalization
	// and autocorrect churn that some keyboards apply to plain text fields.
	a.username.InputHint = key.HintEmail
	a.password.SingleLine = true
	a.password.Mask = '•'
	a.password.InputHint = key.HintPassword
	a.password.Submit = true
	iconData := map[string][]byte{
		"home": icons.ActionHome, "timeline": icons.ActionDateRange,
		"favorite": icons.ActionFavoriteBorder, "favorite_on": icons.ActionFavorite,
		"star": icons.ToggleStarBorder, "star_on": icons.ToggleStar,
		"mail": icons.CommunicationEmail, "self": icons.SocialPerson,
		"search": icons.ActionSearch, "back": icons.NavigationArrowBack,
		"refresh": icons.NavigationRefresh, "like": icons.ActionThumbUp,
		"down": icons.ActionThumbDown, "reply": icons.ContentReply,
		"hash": icons.ActionList, "board": icons.ActionList,
		"link": icons.ContentLink, "copy": icons.ContentContentCopy,
		"trash": icons.ActionDelete, "clock": icons.ActionSchedule,
		"plus": icons.ContentAdd, "hot": icons.ActionTrendingUp,
		"edit": icons.EditorModeEdit, "send": icons.ContentSend,
		"next": icons.NavigationArrowForward, "check": icons.ActionCheckCircle,
		"alert": icons.ActionInfoOutline, "close": icons.NavigationClose,
	}
	a.icons = make(map[string]*widget.Icon, len(iconData))
	for name, data := range iconData {
		icon, err := widget.NewIcon(data)
		if err != nil {
			return nil, err
		}
		a.icons[name] = icon
	}
	return a, nil
}

var colorHint = color.NRGBA{R: 0x6C, G: 0x7C, B: 0x93, A: 0xFF}

func (a *App) SetVersion(v string) { a.version = v }
func (a *App) Close() {
	if a.closed.Swap(true) {
		return
	}
	a.saveComposeDraft()
	a.close()
}

// Back handles the Android system back gesture. Inner pages pop; tab roots
// return to Home so a swipe never drops the user out of the app by accident.
// Home at the root is left to the system, which closes the activity.
func (a *App) Back() bool {
	if !a.ready || !a.handlesBack() {
		return false
	}
	if a.busy {
		return true
	}
	a.navigateBack()
	return true
}

func (a *App) handlesBack() bool {
	if !a.ready {
		return false
	}
	if a.canBack() {
		return true
	}
	switch a.current().Page {
	case PageLogin, PageSearch:
		// The search tab is the launcher root; there is no in-app parent.
		return false
	}
	return true
}

func (a *App) navigateBack() {
	if a.canBack() {
		a.back()
		return
	}
	if a.current().Page != PageSearch {
		a.setTab(PageSearch)
	}
}

func (a *App) handleSystemBack(gtx layout.Context) {
	if !a.handlesBack() {
		return
	}
	for {
		event, ok := gtx.Event(key.Filter{Name: key.NameBack})
		if !ok {
			return
		}
		ke, ok := event.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		if a.current().Page == PageCompose && (gtx.Focused(&a.compose.titleEditor) || gtx.Focused(&a.compose.contentEditor)) {
			gtx.Execute(key.FocusCmd{})
			continue
		}
		if a.busy {
			continue
		}
		a.navigateBack()
	}
}

func (a *App) post(fn func()) {
	if a.closed.Load() {
		return
	}
	select {
	case a.updates <- fn:
	case <-a.ctx.Done():
		return
	}
	a.window.Invalidate()
}

func (a *App) drain() {
	for {
		select {
		case fn := <-a.updates:
			func() {
				defer func() {
					if r := recover(); r != nil {
						a.busy = false
						a.fail(fmt.Errorf("界面更新失败：%v", r))
					}
				}()
				fn()
			}()
		default:
			return
		}
	}
}

func (a *App) ensureStarted() {
	if a.started {
		return
	}
	a.started = true
	a.startAt = time.Now()
	go a.start()
}

func (a *App) start() {
	defer func() {
		if r := recover(); r != nil {
			a.post(func() { a.initErr = fmt.Sprintf("客户端启动失败：%v", r) })
		}
	}()
	dir, err := platform.DataDir()
	if err != nil {
		a.post(func() { a.initErr = "无法打开应用数据目录：" + err.Error() })
		return
	}
	store, err := local.Open(a.ctx, platform.FileState{Path: filepath.Join(dir, "state.json")})
	if err != nil {
		a.post(func() { a.initErr = err.Error() })
		return
	}
	creds := platform.FileCredentials{Path: filepath.Join(dir, "session.json")}
	core, err := application.New(application.Options{BaseURL: os.Getenv("BYR_BASE_URL"), Local: store, Credentials: creds})
	if err != nil {
		a.post(func() { a.initErr = err.Error() })
		return
	}
	if err = core.Restore(a.ctx); err != nil {
		a.post(func() { a.initErr = "恢复登录失败：" + err.Error() })
		return
	}
	a.post(func() {
		a.dir = dir
		a.core = core
		a.initErr = ""
		a.ready = true
		if core.Session().Token != "" {
			a.stack = []Route{{Page: PageSearch}}
		} else {
			a.resetSessionUI()
		}
	})
}

func (a *App) current() Route {
	if len(a.stack) == 0 {
		a.stack = []Route{{Page: PageLogin}}
	}
	return a.stack[len(a.stack)-1]
}

func (a *App) resetSessionUI() {
	a.ready = true
	a.busy = false
	a.message = ""
	a.failed = false
	a.stack = []Route{{Page: PageLogin}}
	a.sections = nil
	a.section = forum.Section{}
	a.boards = nil
	a.board = forum.Board{}
	a.thread = forum.Thread{}
	a.articles = nil
	a.favorite = forum.Favorite{}
	a.collect = forum.Collection{}
	a.pagination = forum.Pagination{}
	a.user = forum.User{}
	a.history = nil
	a.favoriteTab = "boards"
	a.compose = composeState{}
	a.lastClip = ""
	a.images = map[string]*mobileImage{}
	a.clicks = map[string]*widget.Clickable{}
	a.homeList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.sectionList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.boardList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.threadList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.timelineList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.favoriteList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.selfList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.composeList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.topList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.auxList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.searchList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.username.SetText("")
	a.password.SetText("")
	a.searchEdit.SetText("")
	a.searchMode = "ai"
}

func (a *App) canBack() bool { return len(a.stack) > 1 }

func (a *App) push(r Route) bool {
	if a.busy {
		return false
	}
	a.stack = append(a.stack, r)
	a.window.Invalidate()
	return true
}

func (a *App) back() {
	if a.busy || !a.canBack() {
		return
	}
	if a.current().Page == PageCompose {
		if err := a.saveComposeDraft(); err != nil {
			a.fail(err)
			return
		}
	}
	a.stack = a.stack[:len(a.stack)-1]
	a.window.Invalidate()
}

func (a *App) setTab(page Page) {
	if a.busy {
		return
	}
	if a.current().Page == PageCompose {
		if err := a.saveComposeDraft(); err != nil {
			a.fail(err)
			return
		}
	}
	switch page {
	case PageSearch:
		a.stack = []Route{{Page: PageSearch}}
	case PageHome:
		a.stack = []Route{{Page: PageHome}}
		a.loadHome()
	case PageTop:
		a.stack = []Route{{Page: PageTop}}
		a.loadTop()
	case PageTimeline:
		a.stack = []Route{{Page: PageTimeline}}
		a.loadTimeline()
	case PageFavorites:
		a.stack = []Route{{Page: PageFavorites}}
		a.favoriteTab = "boards"
		a.loadFavorites()
	case PageMessages:
		a.stack = []Route{{Page: PageMessages, Sub: "contacts", Number: 1}}
		a.loadMessages()
	case PageSelf:
		a.stack = []Route{{Page: PageSelf}}
		a.loadSelf()
	default:
		a.stack = []Route{{Page: page}}
	}
	a.window.Invalidate()
}

func (a *App) run(label string, work func(context.Context) (func(), error)) {
	if a.busy {
		return
	}
	a.busy = true
	a.busySince = time.Now()
	a.message = label
	a.failed = false
	a.window.Invalidate()
	// Let an in-flight request stay invisible for a moment. A second frame at
	// the threshold reveals the status bar and page spinner only if it is still
	// loading, avoiding a "loading" flash when switching tabs.
	time.AfterFunc(500*time.Millisecond, func() {
		if !a.closed.Load() {
			a.window.Invalidate()
		}
	})
	go func() {
		var done func()
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("操作失败：%v", r)
					done = nil
				}
			}()
			done, err = work(a.ctx)
		}()
		a.post(func() {
			a.busy = false
			if err != nil {
				a.compose.loading = false
				a.compose.publishing = false
				a.fail(err)
				return
			}
			if a.message == label {
				a.message = ""
			}
			if done != nil {
				done()
			}
		})
	}()
}

func (a *App) fail(err error) {
	a.failed = true
	a.message = err.Error()
	defer a.window.Invalidate()
	var api *forum.APIError
	if errors.As(err, &api) && api.Message != "" {
		a.message = api.Message
	}
	var decode *forum.DecodeError
	if errors.As(err, &decode) {
		a.message = "论坛返回的数据格式不兼容。请稍后重试。"
	}
	if errors.Is(err, forum.ErrUnauthenticated) {
		a.message = "登录已失效，请重新登录"
		a.stack = []Route{{Page: PageLogin}}
	}
}

func (a *App) click(key string) *widget.Clickable {
	if a.clicks[key] == nil {
		a.clicks[key] = new(widget.Clickable)
	}
	return a.clicks[key]
}

func (a *App) applyTheme() {
	if a.ready && a.core != nil {
		dark := a.core.Local.Preferences().Theme == 2
		if !a.themed || a.dark != dark {
			a.themed = true
			a.dark = dark
			setPalette(dark)
			colorHint = colorMuted
			a.theme.Bg = colorBg
			a.theme.Fg = colorFg
			a.theme.ContrastBg = colorAccent
		}
	}
}

func (a *App) syncPageState() {
	r := a.current()
	key := fmt.Sprintf("%d|%s|%s|%s|%d", r.Page, r.Name, r.ID, r.Sub, r.Number)
	if key != a.pageKey {
		a.pageKey = key
		// Rebuild list state between pages. Resetting a list from inside its own
		// item callback corrupts layout, so this runs only at frame start.
		a.auxList = widget.List{List: layout.List{Axis: layout.Vertical}}
	}
}

func (a *App) Layout(gtx layout.Context) (dims layout.Dimensions) {
	defer func() {
		if r := recover(); r != nil {
			// Draw the recovery screen on a clean frame: the ops of the
			// panicking frame can be partially clipped or unbalanced.
			a.renderPanic = fmt.Sprintf("%v", r)
			a.window.Invalidate()
			dims = layout.Dimensions{}
		}
	}()
	a.drain()
	a.ensureStarted()
	a.applyTheme()
	if a.core != nil {
		if scale := a.core.Local.Preferences().FontScale; scale > 0 && scale != 1 {
			gtx.Metric.PxPerSp *= scale
		}
	}
	if !a.ready && a.initErr == "" && !a.startAt.IsZero() && time.Since(a.startAt) > 12*time.Second {
		a.initErr = "客户端启动超时。请检查存储权限和网络后重试。"
	}
	if len(a.stack) > 0 {
		a.syncPageState()
	}
	if a.renderPanic != "" {
		return a.recoveryPage(gtx)
	}
	a.handleSystemBack(gtx)
	// The window supplies exact constraints; children must not inherit the
	// window minimum as their own minimum, or inner widgets stretch to the
	// whole screen.
	gtx.Constraints.Min = image.Point{}
	paint.Fill(gtx.Ops, a.theme.Bg)
	if a.initErr != "" {
		return a.errorPage(gtx)
	}
	if !a.ready {
		return a.loadingPage(gtx, "正在启动客户端…")
	}
	if len(a.stack) == 0 {
		a.stack = []Route{{Page: PageLogin}}
	}
	body := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.topBar),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.message == "" || a.busy && !a.showLoading() {
					return layout.Dimensions{}
				}
				return a.statusBar(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, a.page)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				page := a.current()
				if page.Page == PageLogin || page.Page == PageCompose || page.Page == PageSection && page.Sub == "pick" {
					return layout.Dimensions{}
				}
				return a.bottomNav(gtx)
			}),
		)
	}
	// On wide screens (foldables unfolded, tablets) keep a phone-like column
	// centered instead of stretching desktop-style across the whole width.
	var bodyDims layout.Dimensions
	if maxW := gtx.Dp(unit.Dp(480)); gtx.Constraints.Max.X > maxW {
		bodyDims = layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(image.Pt(maxW, gtx.Constraints.Max.Y))
				return body(gtx)
			}),
		)
	} else {
		bodyDims = body(gtx)
	}
	// A page push during this frame must arm the back filter before the frame
	// ends; otherwise a swipe during the first loading moment reaches Android
	// with no handler and finishes the activity.
	a.handleSystemBack(gtx)
	return bodyDims
}

// showLoading hides spinners and transient status text for the first 500ms of
// a request so quick page switches do not flash a loading state.
func (a *App) showLoading() bool {
	if !a.busy {
		return false
	}
	return a.busySince.IsZero() || time.Since(a.busySince) >= 500*time.Millisecond
}

func (a *App) pendingPage(gtx layout.Context, text string) layout.Dimensions {
	if a.showLoading() {
		return a.loadingPage(gtx, text)
	}
	return a.skeleton(gtx)
}

func (a *App) emptyOrPending(gtx layout.Context, text string) layout.Dimensions {
	if a.busy {
		if a.showLoading() {
			return a.loadingPage(gtx, "正在加载…")
		}
		return a.skeleton(gtx)
	}
	return a.empty(gtx, text)
}

func (a *App) skeleton(gtx layout.Context) layout.Dimensions {
	row := func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		h := gtx.Dp(unit.Dp(72))
		size := image.Pt(w, h)
		paint.FillShape(gtx.Ops, colorSurface,
			clip.RRect{Rect: image.Rectangle{Max: size}, NE: 12, NW: 12, SE: 12, SW: 12}.Op(gtx.Ops))
		bar := func(top int, fraction float64) {
			width := int(float64(w-32) * fraction)
			if width < 24 {
				width = 24
			}
			off := op.Offset(image.Pt(16, top)).Push(gtx.Ops)
			paint.FillShape(gtx.Ops, colorBorder,
				clip.RRect{Rect: image.Rectangle{Max: image.Pt(width, 12)}, NE: 6, NW: 6, SE: 6, SW: 6}.Op(gtx.Ops))
			off.Pop()
		}
		bar(h/2-16, 0.45)
		bar(h/2+8, 0.7)
		return layout.Dimensions{Size: size}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, row)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, row)
		}),
		layout.Rigid(row),
	)
}

func (a *App) loadingPage(gtx layout.Context, text string) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				loader := material.Loader(a.theme)
				loader.Color = a.theme.ContrastBg
				return loader.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(a.theme, text)
				l.Color = color.NRGBA{R: 0x6C, G: 0x7C, B: 0x93, A: 0xFF}
				return l.Layout(gtx)
			}),
		)
	})
}

func (a *App) errorPage(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(24), Right: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.H6(a.theme, "客户端启动失败")
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(a.theme, a.initErr)
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := a.click("retry-init")
					clicked := btn.Clicked(gtx)
					dims := material.Button(a.theme, btn, "重试").Layout(gtx)
					if clicked {
						a.started = false
						a.initErr = ""
						a.ensureStarted()
					}
					return dims
				}),
			)
		})
	})
}

func (a *App) recoveryPage(gtx layout.Context) layout.Dimensions {
	paint.Fill(gtx.Ops, a.theme.Bg)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(24), Right: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.H6(a.theme, "界面遇到问题").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(a.theme, a.renderPanic)
					l.Color = colorMuted
					l.MaxLines = 4
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Dp(unit.Dp(220))
					dims, clicked := a.primaryButton(gtx, "recover-login", "返回登录页")
					if clicked {
						a.renderPanic = ""
						a.post(func() { a.resetSessionUI() })
					}
					return dims
				}),
			)
		})
	})
}

func (a *App) topBar(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(unit.Dp(64))
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, height)
			paint.FillShape(gtx.Ops, colorSurface, clip.Rect(image.Rectangle{Max: size}).Op())
			paint.FillShape(gtx.Ops, colorBorder, clip.Rect(image.Rect(0, height-1, size.X, height)).Op())
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !a.canBack() {
							return layout.Dimensions{}
						}
						dims, clicked := a.iconButton(gtx, "back", "back")
						if clicked {
							a.back()
						}
						return dims
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(18), a.title())
									l.Font.Weight = 700
									l.MaxLines = 1
									l.Truncator = "…"
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(11), a.subtitle())
									l.Color = colorMuted
									l.MaxLines = 1
									return l.Layout(gtx)
								}),
							)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						page := a.current().Page
						if page == PageLogin || page == PageSearch || page == PageCompose && a.compose.editID == "" {
							return layout.Dimensions{}
						}
						dims, clicked := a.iconButton(gtx, "top-refresh", "refresh")
						if clicked {
							a.refreshCurrent()
						}
						return dims
					}),
				)
			})
		}),
	)
}

func (a *App) subtitle() string {
	switch a.current().Page {
	case PageLogin:
		return "北邮人论坛"
	case PageHome:
		return "发现分区"
	case PageSection:
		if a.current().Sub == "pick" {
			return "选择发帖版面"
		}
		return "子分区与版面"
	case PageTop:
		if a.topTab == "recommend" {
			return "版面推荐"
		}
		return "全站热门讨论"
	case PageBoard:
		return a.current().Name + " · 版面"
	case PageThread:
		return "主题阅读"
	case PageTimeline:
		return "全部帖子 · 按时间"
	case PageFavorites:
		return "我的收藏"
	case PageSearch:
		return "AI 搜索 · 全站搜索"
	case PageMessages:
		switch a.current().Sub {
		case "inbox":
			return "收件箱"
		case "outbox":
			return "发件箱"
		case "refers":
			return "回复与 @ 提醒"
		}
		return "私信与站内信"
	case PageChat:
		return "与 " + a.current().Name + " 的私信"
	case PageMailbox:
		return "站内信"
	case PageMail:
		if a.mail.Title != "" {
			return a.mail.Title
		}
		return "站内信"
	case PageMailCompose:
		return "写站内信"
	case PageRefers:
		return "回复与 @ 提醒"
	case PageUser:
		if name := a.user.DisplayName(); name != "" {
			return name
		}
		return a.current().Name
	case PageHistory:
		return "阅读历史"
	case PageBlacklist:
		return "黑名单"
	case PageSettings:
		return "设置"
	case PageVote:
		return "投票"
	case PageBet:
		return "竞猜"
	case PageSelf:
		return "个人中心"
	case PageCompose:
		return "写点什么"
	}
	return ""
}

func (a *App) refreshCurrent() {
	switch a.current().Page {
	case PageHome:
		a.loadHome()
	case PageSection:
		a.loadSectionPage()
	case PageTop:
		a.loadTop()
	case PageBoard:
		a.loadBoard()
	case PageThread:
		a.loadThread()
	case PageTimeline:
		a.loadTimeline()
	case PageFavorites:
		a.loadFavorites()
	case PageSelf:
		a.loadSelf()
	case PageMessages:
		a.loadMessages()
	case PageChat:
		a.loadChat()
	case PageMail:
		a.loadMail()
	case PageUser:
		a.loadUser(a.current().Name)
	case PageBlacklist:
		a.loadBlacklist()
	case PageVote:
		a.loadVote()
	case PageBet:
		a.loadBet()
	case PageCompose:
		a.loadCompose()
	}
}

func (a *App) statusBar(gtx layout.Context) layout.Dimensions {
	bg := colorAccentSoft
	fg := a.theme.ContrastBg
	if a.failed {
		bg = colorDangerSoft
		fg = colorDanger
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(34)))
			paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rectangle{Max: size}).Op())
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
				var children []layout.FlexChild
				if a.busy {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						loader := material.Loader(a.theme)
						loader.Color = fg
						return loader.Layout(gtx)
					}))
					children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
				}
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					l := material.Caption(a.theme, a.message)
					l.Color = fg
					l.MaxLines = 2
					return l.Layout(gtx)
				}))
				if !a.busy {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := a.click("dismiss-status")
						clicked := btn.Clicked(gtx)
						dims := material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
							l := material.Caption(a.theme, "关闭")
							l.Color = fg
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, l.Layout)
						})
						if clicked {
							a.message = ""
							a.failed = false
							a.window.Invalidate()
						}
						return dims
					}))
				}
				return row.Layout(gtx, children...)
			})
		}),
	)
}

func (a *App) bottomNav(gtx layout.Context) layout.Dimensions {
	tabs := []struct {
		page    Page
		key     string
		icon    string
		label   string
		compose bool
	}{
		{page: PageSearch, key: "nav-search", icon: "search", label: "搜索"},
		{page: PageHome, key: "nav-home", icon: "hash", label: "分区"},
		{page: PageCompose, key: "nav-compose", icon: "plus", label: "发帖", compose: true},
		{page: PageMessages, key: "nav-messages", icon: "mail", label: "消息"},
		{page: PageSelf, key: "nav-self", icon: "self", label: "我的"},
	}
	height := gtx.Dp(unit.Dp(72))
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, height)
			paint.FillShape(gtx.Ops, colorSurface, clip.Rect(image.Rectangle{Max: size}).Op())
			paint.FillShape(gtx.Ops, colorBorder, clip.Rect(image.Rect(0, 0, size.X, 1)).Op())
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(tabs))
			for _, tab := range tabs {
				tab := tab
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					btn := a.click(tab.key)
					clicked := btn.Clicked(gtx)
					dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						if tab.compose {
							return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.composeNavCircle(gtx)
									}),
									layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											l := material.Label(a.theme, unit.Sp(10), tab.label)
											l.Color = colorAccent
											l.Font.Weight = 600
											return l.Layout(gtx)
										})
									}),
								)
							})
						}
						active := a.navPage(a.current().Page) == tab.page
						return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(6), Left: unit.Dp(3), Right: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							m := op.Record(gtx.Ops)
							inner := layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								fg := colorMuted
								if active {
									fg = colorAccent
								}
								return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return a.drawIcon(gtx, tab.icon, 21, fg)
										})
									}),
									layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											l := material.Label(a.theme, unit.Sp(10), tab.label)
											l.Color = fg
											return l.Layout(gtx)
										})
									}),
								)
							})
							call := m.Stop()
							if active {
								paint.FillShape(gtx.Ops, colorAccentSoft, clip.RRect{Rect: image.Rectangle{Max: inner.Size}, NE: 14, NW: 14, SE: 14, SW: 14}.Op(gtx.Ops))
							}
							call.Add(gtx.Ops)
							return inner
						})
					})
					if clicked {
						if tab.compose {
							a.openCompose("", "", "new")
						} else {
							a.setTab(tab.page)
						}
					}
					return dims
				}))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		}),
	)
}

func (a *App) composeNavCircle(gtx layout.Context) layout.Dimensions {
	size := gtx.Dp(unit.Dp(42))
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	m := op.Record(gtx.Ops)
	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.drawIcon(gtx, "plus", 22, a.theme.ContrastFg)
	})
	call := m.Stop()
	paint.FillShape(gtx.Ops, colorAccent, clip.Ellipse(image.Rectangle{Max: image.Pt(size, size)}).Op(gtx.Ops))
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(size, size)}
}

func (a *App) navPage(page Page) Page {
	switch page {
	case PageSection, PageTop, PageBoard, PageThread, PageTimeline, PageCompose, PageVote, PageBet:
		return PageHome
	case PageChat, PageMail, PageMailbox, PageMailCompose, PageRefers:
		return PageMessages
	case PageUser, PageHistory, PageBlacklist, PageSettings, PageFavorites:
		return PageSelf
	case PageSearch:
		return PageSearch
	}
	return page
}

func (a *App) title() string {
	switch a.current().Page {
	case PageLogin:
		return "登录 · " + a.version
	case PageHome:
		return "分区"
	case PageSection:
		if a.section.Description != "" {
			return a.section.Description
		}
		if a.current().Name == "" {
			return "全部分区"
		}
		return a.current().Name
	case PageTop:
		if a.topTab == "recommend" {
			return "版面推荐"
		}
		return "今日十大"
	case PageBoard:
		if a.board.Description != "" {
			return a.board.Description
		}
		return a.current().Name
	case PageThread:
		if a.thread.Title != "" {
			return a.thread.Title
		}
		return "帖子"
	case PageTimeline:
		return "时间线"
	case PageFavorites:
		return "收藏"
	case PageSearch:
		return "搜索"
	case PageMessages:
		return "消息"
	case PageChat:
		return a.current().Name
	case PageMailbox, PageMail, PageMailCompose, PageRefers:
		return "站内信"
	case PageUser:
		if name := a.user.DisplayName(); name != "" {
			return name
		}
		return a.current().Name
	case PageHistory:
		return "阅读历史"
	case PageBlacklist:
		return "黑名单"
	case PageSettings:
		return "设置"
	case PageVote:
		return "投票"
	case PageBet:
		return "竞猜"
	case PageSelf:
		return "我"
	case PageCompose:
		return "发帖"
	}
	return "BYR"
}

func (a *App) page(gtx layout.Context) layout.Dimensions {
	switch a.current().Page {
	case PageLogin:
		return a.loginPage(gtx)
	case PageHome:
		return a.homePage(gtx)
	case PageSection:
		return a.sectionPage(gtx)
	case PageTop:
		return a.topPage(gtx)
	case PageBoard:
		return a.boardPage(gtx)
	case PageThread:
		return a.threadPage(gtx)
	case PageTimeline:
		return a.timelinePage(gtx)
	case PageFavorites:
		return a.favoritesPage(gtx)
	case PageSearch:
		return a.searchPage(gtx)
	case PageMessages:
		return a.messagesPage(gtx)
	case PageChat:
		return a.chatPage(gtx)
	case PageMailbox:
		return a.messagesPage(gtx)
	case PageMail:
		return a.mailPage(gtx)
	case PageMailCompose:
		return a.mailComposePage(gtx)
	case PageRefers:
		return a.messagesPage(gtx)
	case PageUser:
		return a.userPage(gtx)
	case PageHistory:
		return a.historyPage(gtx)
	case PageBlacklist:
		return a.blacklistPage(gtx)
	case PageSettings:
		return a.settingsPage(gtx)
	case PageVote:
		return a.votePage(gtx)
	case PageBet:
		return a.betPage(gtx)
	case PageSelf:
		return a.selfPage(gtx)
	case PageCompose:
		return a.composePage(gtx)
	}
	return layout.Dimensions{}
}

// card paints a rounded surface behind content and returns its dimensions.
func (a *App) card(gtx layout.Context, content layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	inner := layout.UniformInset(unit.Dp(14)).Layout(gtx, content)
	call := m.Stop()
	paint.FillShape(gtx.Ops, colorSurface,
		clip.RRect{Rect: image.Rectangle{Max: inner.Size}, NE: 12, NW: 12, SE: 12, SW: 12}.Op(gtx.Ops))
	call.Add(gtx.Ops)
	return inner
}

// inputBox draws a bordered, rounded field around an editor so the input area
// is visible and easy to tap on a phone.
func (a *App) inputBox(gtx layout.Context, editor *widget.Editor, hint string) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	inner := layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return material.Editor(a.theme, editor, hint).Layout(gtx)
	})
	call := m.Stop()
	size := inner.Size
	if min := gtx.Dp(unit.Dp(48)); size.Y < min {
		size.Y = min
	}
	borderColor := colorBorder
	if gtx.Focused(editor) {
		borderColor = a.theme.ContrastBg
	}
	paint.FillShape(gtx.Ops, borderColor, clip.RRect{Rect: image.Rectangle{Max: size}, NE: 12, NW: 12, SE: 12, SW: 12}.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, colorSurface,
		clip.RRect{Rect: image.Rect(1, 1, size.X-1, size.Y-1), NE: 11, NW: 11, SE: 11, SW: 11}.Op(gtx.Ops))
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

// primaryButton is a full-width button whose painted area is exactly its tap
// area, so it never renders in one place and accepts touches in another.
func (a *App) primaryButton(gtx layout.Context, key, label string) (layout.Dimensions, bool) {
	btn := a.click(key)
	clicked := btn.Clicked(gtx)
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(48)))
	bg := a.theme.ContrastBg
	if btn.Pressed() {
		bg = color.NRGBA{R: 0x06, G: 0x3A, B: 0x78, A: 0xFF}
	}
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(size)
		paint.FillShape(gtx.Ops, bg, clip.RRect{Rect: image.Rectangle{Max: size}, NE: 12, NW: 12, SE: 12, SW: 12}.Op(gtx.Ops))
		labelMacro := op.Record(gtx.Ops)
		l := material.Label(a.theme, unit.Sp(15), label)
		l.Color = a.theme.ContrastFg
		l.Alignment = text.Middle
		labelDims := l.Layout(gtx)
		labelCall := labelMacro.Stop()
		op.Offset(image.Pt((size.X-labelDims.Size.X)/2, (size.Y-labelDims.Size.Y)/2)).Add(gtx.Ops)
		labelCall.Add(gtx.Ops)
		return layout.Dimensions{Size: size}
	})
	return dims, clicked
}

func (a *App) sectionLabel(gtx layout.Context, text string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Caption(a.theme, text)
		l.Color = color.NRGBA{R: 0x6C, G: 0x7C, B: 0x93, A: 0xFF}
		return l.Layout(gtx)
	})
}

func (a *App) copyText(gtx layout.Context, text string) {
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: stringsReader(text)})
}

func stringsReader(s string) *stringReader { return &stringReader{Reader: strings.NewReader(s)} }

type stringReader struct{ *strings.Reader }

func (r *stringReader) Close() error { return nil }

func formatTime(s forum.Scalar) string {
	v, err := timeParse(string(s))
	if err != nil || v <= 0 {
		return string(s)
	}
	return time.Unix(v, 0).Format("01-02 15:04")
}

func timeParse(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
