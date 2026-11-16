package ui

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

type noopInvalidator struct{}

func (noopInvalidator) Invalidate() {}

func previewApp(t *testing.T) *App {
	t.Helper()
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.ready = true
	return a
}

func previewAppWithCore(t *testing.T) *App {
	t.Helper()
	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	core, err := application.New(application.Options{
		Local: store,
		Credentials: &testCredentials{session: application.Credential{
			Username: "alice", Token: "token", ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := previewApp(t)
	a.core = core
	return a
}

func sampleThread() forum.Thread {
	return forum.Thread{
		ID: "42", GroupID: "42", Title: "测试主题：Go 语言", Board: "Go", BoardDescription: "Go 讨论",
		LikeSum: 2, Collect: false, ReplyCount: 1,
		Articles: []forum.Article{
			{ID: "42", Position: 1, Author: forum.User{ID: "alice", UserName: "Alice"}, Content: "你好 [b]北邮[/b]！\n这是一段测试正文。", PostTime: forum.Scalar("1700000000"), LikeSum: 2},
			{ID: "44", Position: 2, ReplyID: "42", Author: forum.User{ID: "bob", UserName: "Bob"}, Content: "回复正文：支持共享核心。", PostTime: forum.Scalar("1700000100"), VotedownSum: 5},
		},
		Pagination: forum.Pagination{CurrentPage: 1, TotalPages: 1},
	}
}

func TestPreviewScreens(t *testing.T) {
	dir := os.Getenv("BYR_ANDROID_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set BYR_ANDROID_PREVIEW_DIR to render offline screenshots")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	const width, height = 420, 820
	w, err := headless.NewWindow(width, height)
	if err != nil {
		t.Skipf("headless renderer unavailable: %v", err)
	}
	defer w.Release()
	var router input.Router
	captureSize := func(name string, a *App, size image.Point) {
		t.Helper()
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Constraints: layout.Exact(size),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Now:         time.Now(),
		}
		a.Layout(gtx)
		if err := w.Frame(&ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
		if err := w.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
	capture := func(name string, a *App) { captureSize(name, a, image.Pt(width, height)) }

	login := previewApp(t)
	login.stack = []Route{{Page: PageLogin}}
	capture("login", login)
	land := previewApp(t)
	land.stack = []Route{{Page: PageLogin}}
	captureSize("login-landscape", land, image.Pt(820, 420))

	home := previewApp(t)
	home.sections = []forum.Section{{Name: "0", Description: "校园"}, {Name: "2", Description: "生活"}}
	home.sectionTab = "favorites"
	home.favoriteBoards = []forum.Board{
		{Name: "Go", Description: "Go 讨论", IsFavorite: true, ThreadsTodayCount: 3, UserOnlineCount: 12},
		{Name: "WorkLife", Description: "工作生活", IsFavorite: true, PostTodayCount: 6, UserOnlineCount: 4},
	}
	home.stack = []Route{{Page: PageHome}}
	capture("home", home)

	allSections := previewApp(t)
	allSections.sections = home.sections
	allSections.sectionTab = "all"
	allSections.stack = []Route{{Page: PageHome}}
	capture("sections-all", allSections)

	section := previewApp(t)
	section.sections = home.sections
	section.section = forum.Section{
		Name: "0", Description: "校园",
		Boards:      []forum.Board{{Name: "Go", Description: "Go 讨论", ThreadsTodayCount: 3, UserOnlineCount: 12}},
		Subsections: []forum.Section{{Name: "1", Description: "院系"}},
	}
	section.boards = []forum.Board{{Name: "Go", Description: "Go 讨论", ThreadsTodayCount: 3, UserOnlineCount: 12}}
	section.stack = []Route{{Page: PageHome}, {Page: PageSection, Name: "0"}}
	capture("section", section)

	picker := previewApp(t)
	picker.sections = home.sections
	picker.stack = []Route{{Page: PageCompose, Name: "", Sub: "new"}, {Page: PageSection, Sub: "pick"}}
	capture("section-picker", picker)

	board := previewApp(t)
	board.board = forum.Board{Name: "Go", Description: "Go 讨论", Articles: []forum.Article{
		{ID: "42", GroupID: "42", Title: "测试主题：Go 语言", Author: forum.User{ID: "alice", UserName: "Alice"}, PostTime: forum.Scalar("1700000000"), ReplyCount: 2},
		{ID: "44", Title: "另一个主题", Author: forum.User{ID: "bob", UserName: "Bob"}, PostTime: forum.Scalar("1700000100"), ReplyCount: 1},
	}, Pagination: forum.Pagination{CurrentPage: 1, TotalPages: 1}}
	board.stack = []Route{{Page: PageBoard, Name: "Go", Number: 1}}
	capture("board", board)

	thread := previewApp(t)
	thread.thread = sampleThread()
	thread.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	capture("thread", thread)

	pinned := previewApp(t)
	pinnedThread := sampleThread()
	pinnedThread.Articles[0].IsTop = true
	pinned.thread = pinnedThread
	pinned.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	capture("thread-pinned", pinned)

	timeline := previewApp(t)
	timeline.articles = board.board.Articles
	timeline.stack = []Route{{Page: PageTimeline}}
	capture("timeline", timeline)

	favorites := previewApp(t)
	favorites.favorite = forum.Favorite{Boards: []forum.Board{{Name: "Go", Description: "Go 讨论"}}}
	favorites.favoriteTab = "boards"
	favorites.stack = []Route{{Page: PageFavorites}}
	capture("favorites", favorites)

	search := previewApp(t)
	search.stack = []Route{{Page: PageSearch}}
	capture("search", search)

	searchAll := previewApp(t)
	searchAll.searchMode = "all"
	searchAll.stack = []Route{{Page: PageSearch}}
	capture("search-all", searchAll)

	user := previewApp(t)
	user.user = forum.User{ID: "bob", UserName: "Bob", PostCount: 8, Score: 42, HomePage: "https://example.com"}
	user.stack = []Route{{Page: PageSelf}, {Page: PageUser, Name: "bob"}}
	capture("user", user)

	messages := previewApp(t)
	messages.mailbox = forum.Mailbox{NewNum: 1, Mails: []forum.Mail{
		{Index: "1", Title: "你好", Author: forum.User{ID: "bob", UserName: "Bob"}, PostTime: forum.Scalar("1700000000")},
		{Index: "2", Title: "已读信件", IsRead: true, Author: forum.User{ID: "carol", UserName: "Carol"}, PostTime: forum.Scalar("1700000100")},
	}}
	messages.stack = []Route{{Page: PageSelf}, {Page: PageMessages, Sub: "inbox"}}
	capture("messages", messages)

	vote := previewAppWithCore(t)
	vote.vote = forum.Vote{ID: "42", Title: "周末去哪里", UserCount: 12, Options: []forum.VoteOption{
		{ID: "1", Label: "爬山", Num: 7},
		{ID: "2", Label: "看电影", Num: 5},
	}}
	vote.choice = "1"
	vote.stack = []Route{{Page: PageSelf}, {Page: PageVote, ID: "42"}}
	capture("vote", vote)

	setPalette(true)
	colorHint = colorMuted
	darkHome := previewApp(t)
	darkHome.sections = home.sections
	darkHome.sectionTab = "all"
	darkHome.stack = []Route{{Page: PageHome}}
	capture("home-dark", darkHome)
	setPalette(false)
	colorHint = colorMuted

	top := previewApp(t)
	top.topTab = "top"
	top.articles = board.board.Articles
	top.stack = []Route{{Page: PageTop}}
	capture("top", top)

	self := previewAppWithCore(t)
	self.user = forum.User{ID: "alice", UserName: "Alice", PostCount: 12, Score: 100}
	self.history = []local.History{{Board: "Go", ID: "42", Title: "测试主题"}}
	self.stack = []Route{{Page: PageSelf}}
	capture("self", self)

	settings := previewAppWithCore(t)
	settings.stack = []Route{{Page: PageSelf}, {Page: PageSettings}}
	capture("settings", settings)

	compose := previewApp(t)
	compose.compose = composeState{board: "Go", boardLocked: true, ready: true}
	compose.compose.titleEditor.SingleLine = true
	compose.compose.titleEditor.SetText("Re: 测试主题")
	compose.compose.contentEditor.SetText("这是一段回复正文。")
	compose.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}, {Page: PageCompose, Name: "Go", ID: "42", Sub: "reply"}}
	capture("compose", compose)

	newPost := previewApp(t)
	newPost.compose = composeState{ready: true}
	newPost.compose.titleEditor.SingleLine = true
	newPost.stack = []Route{{Page: PageHome}, {Page: PageCompose, Sub: "new"}}
	capture("compose-new", newPost)
}
