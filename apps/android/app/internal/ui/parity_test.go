package ui

import (
	"context"
	"image"
	"image/color"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

type recordingSystemBarWindow struct{ colors []color.NRGBA }

func (*recordingSystemBarWindow) Invalidate() {}
func (w *recordingSystemBarWindow) SetSystemBarColor(c color.NRGBA) { w.colors = append(w.colors, c) }

func TestSystemBarsFollowLightAndDarkTheme(t *testing.T) {
	a := previewAppWithCore(t)
	defer a.Close()
	defer func() {
		setPalette(false)
		a.dark, a.themed = false, false
	}()
	window := &recordingSystemBarWindow{}
	a.window = window
	prefs := a.core.Local.Preferences()
	prefs.Theme = 2
	if err := a.core.Local.SetPreferences(context.Background(), prefs); err != nil {
		t.Fatal(err)
	}
	a.applyTheme()
	if len(window.colors) != 1 || window.colors[0] != colorBg {
		t.Fatalf("dark system bar colors = %+v, want dark surface %v", window.colors, colorBg)
	}
	prefs.Theme = 1
	if err := a.core.Local.SetPreferences(context.Background(), prefs); err != nil {
		t.Fatal(err)
	}
	a.applyTheme()
	if len(window.colors) != 2 || window.colors[1] != colorBg {
		t.Fatalf("light system bar colors = %+v, want light surface %v", window.colors, colorBg)
	}
}

func parityServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case path == "/open/section.json":
			_, _ = w.Write([]byte(`{"section":[{"name":"0","description":"校园"},{"name":"2","description":"生活"}]}`))
		case path == "/open/section/0.json":
			_, _ = w.Write([]byte(`{"name":"0","description":"校园","sub_section":[{"name":"1","description":"院系"}],"board":[{"name":"Go","description":"Go 讨论","threads_today_count":3,"post_today_count":9,"user_online_count":12}]}`))
		case path == "/open/favorite/0.json":
			_, _ = w.Write([]byte(`{"board":[{"name":"Go","description":"Go 讨论"}]}`))
		case path == "/open/board/Go.json":
			_, _ = w.Write([]byte(`{"name":"Go","description":"Go 讨论","is_favorite":true,"threads_today_count":3,"post_today_count":9,"user_online_count":12}`))
		case path == "/open/section/1.json":
			_, _ = w.Write([]byte(`{"name":"1","description":"院系","board":[{"name":"Study","description":"学习","threads_today_count":2,"post_today_count":4,"user_online_count":5}]}`))
		case path == "/open/user/query/bob.json":
			_, _ = w.Write([]byte(`{"id":"bob","user_name":"Bob","face_url":"/face/bob.gif","face_width":96,"face_height":96,"gender":"男","astro":"双子座","life":"学生","qq":"12345","msn":"bob@example","home_page":"https://example.test/bob","level":"用户","is_online":1,"post_count":"12","last_login_time":"1700000000","last_login_ip":"127.0.0.1","is_hide":0,"is_register":1,"score":"88","follow_num":3,"fans_num":4,"is_follow":1,"is_fan":0}`))
		case path == "/open/blacklist/list.json":
			_, _ = w.Write([]byte(`{"user":[{"id":"bob","user_name":"Bob"}]}`))
		case path == "/open/mail/inbox.json":
			_, _ = w.Write([]byte(`{"description":"收件箱","new_num":1,"mail":[{"index":1,"title":"你好","user":{"id":"bob","user_name":"Bob"},"post_time":1700000000}]}`))
		case path == "/open/mail/outbox.json":
			_, _ = w.Write([]byte(`{"description":"发件箱","mail":[{"index":3,"title":"已读","user":{"id":"bob","user_name":"Bob"},"post_time":1700000100}]}`))
		case path == "/open/mail/inbox/1.json":
			_, _ = w.Write([]byte(`{"index":1,"title":"你好","user":{"id":"bob","user_name":"Bob"},"post_time":1700000000,"content":"来信正文"}`))
		case path == "/open/mail/outbox/3.json":
			_, _ = w.Write([]byte(`{"index":3,"title":"已读","user":{"id":"bob","user_name":"Bob"},"post_time":1700000100,"content":"去信正文"}`))
		case path == "/open/refer/reply.json":
			_, _ = w.Write([]byte(`{"article":[{"index":1,"id":42,"group_id":42,"board_name":"Go","title":"回复提醒","user":{"id":"bob","user_name":"Bob"},"pos":1}]}`))
		case path == "/open/vote/42.json":
			if r.Method == "POST" {
				io.Copy(io.Discard, r.Body)
				_, _ = w.Write([]byte(`{"vid":42,"title":"投票","options":[{"viid":1,"label":"A","num":3}],"user_count":3,"vote_status":true}`))
				return
			}
			_, _ = w.Write([]byte(`{"vid":42,"title":"投票","options":[{"viid":1,"label":"A","num":2}],"user_count":2}`))
		case path == "/open/bet/42.json":
			_, _ = w.Write([]byte(`{"bid":42,"title":"竞猜","bitems":[{"biid":1,"label":"A","percent":"50%"}],"score":10}`))
		case strings.HasPrefix(path, "/open/mail/"):
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		case strings.HasPrefix(path, "/open/blacklist/"):
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		default:
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func parityApp(t *testing.T, baseURL string) *App {
	t.Helper()
	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	credentials := &testCredentials{session: application.Credential{
		Username: "alice", Token: "token", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}}
	core, err := application.New(application.Options{
		BaseURL: baseURL, HTTPClient: http.DefaultClient, Local: store, Credentials: credentials,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.ready = true
	a.core = core
	a.stack = []Route{{Page: PageSelf}}
	return a
}

func renderParityPage(t *testing.T, a *App, render func(layout.Context) layout.Dimensions) {
	t.Helper()
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	dims := render(gtx)
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("page rendered blank dimensions: %+v", dims.Size)
	}
}

func TestHomeAndSectionHierarchy(t *testing.T) {
	server := parityServer(t)
	a := parityApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.loadHome()
	waitForIdle(t, a)
	if len(a.sections) != 2 {
		t.Fatalf("home sections = %+v", a.sections)
	}
	if a.section.Name != "" || len(a.boards) != 0 {
		t.Fatalf("home should show only sections: section=%+v boards=%+v", a.section, a.boards)
	}
	renderParityPage(t, a, a.homePage)

	a.openSection("0", false)
	waitForIdle(t, a)
	if got := a.current().Page; got != PageSection {
		t.Fatalf("page = %v, want section", got)
	}
	if a.section.Description != "校园" || len(a.boards) != 1 {
		t.Fatalf("section detail = %+v boards=%+v", a.section, a.boards)
	}
	threads, posts, online := sectionTotals(a.section)
	if threads != 5 || posts != 13 || online != 17 {
		t.Fatalf("section totals = %d/%d/%d, want 5/13/17", threads, posts, online)
	}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	a.click("section-board:Go:false").Click()
	ops.Reset()
	a.Layout(gtx)
	if got := a.current().Page; got != PageBoard {
		t.Fatalf("board tap page = %v, want board", got)
	}
}

func TestFavoriteBoardsShowLiveStats(t *testing.T) {
	server := parityServer(t)
	a := parityApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageFavorites, Number: 1}}
	a.favoriteTab = "boards"
	a.loadFavorites()
	waitForIdle(t, a)
	if len(a.favoriteBoards) != 1 {
		t.Fatalf("favorite boards = %+v", a.favoriteBoards)
	}
	board := a.favoriteBoards[0]
	if board.ThreadsTodayCount != 3 || board.PostTodayCount != 9 || board.UserOnlineCount != 12 || !bool(board.IsFavorite) {
		t.Fatalf("favorite board stats = %+v", board)
	}
	renderParityPage(t, a, a.favoritesPage)
}

func TestUserProfileShowsAllProfileFields(t *testing.T) {
	server := parityServer(t)
	a := parityApp(t, server.URL)
	defer a.Close()

	a.openUser("bob")
	waitForIdle(t, a)
	if got := a.user.DisplayName(); got != "Bob" {
		t.Fatalf("user display name = %q, want Bob", got)
	}

	fields := userProfileFields(a.user)
	labels := make([]string, 0, len(fields))
	for _, field := range fields {
		labels = append(labels, field.label)
	}
	want := []string{"在线状态", "注册状态", "生命力", "性别", "星座", "主页", "QQ", "MSN", "上次登录时间", "最后登录 IP", "关注数", "粉丝数", "是否已关注", "是否为粉丝", "发帖数", "积分", "等级"}
	if got := strings.Join(labels, "|"); got != strings.Join(want, "|") {
		t.Fatalf("profile labels = %q, want %q", got, strings.Join(want, "|"))
	}
	for _, value := range []string{"学生", "12345", "bob@example", "https://example.test/bob"} {
		found := false
		for _, field := range fields {
			if field.value == value {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("profile value %q was not rendered by the field contract: %+v", value, fields)
		}
	}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(420, 700)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	if got := a.profileRow(gtx, "空字段", ""); got.Size != (image.Point{}) {
		t.Fatalf("empty profile row = %+v, want zero dimensions", got.Size)
	}
	if got := a.profileRow(gtx, "生命力", "学生"); got.Size.X <= 0 || got.Size.Y <= 0 {
		t.Fatalf("non-empty profile row has no dimensions: %+v", got.Size)
	}

	a.click("user-home").Click()
	ops.Reset()
	gtx.Ops = &ops
	a.userPage(gtx)
	if a.lastClip != a.user.HomePage {
		t.Fatalf("homepage action copied %q, want %q", a.lastClip, a.user.HomePage)
	}

	hidden := a.user
	hidden.IsHide = true
	for _, field := range userProfileFields(hidden) {
		if field.label == "上次登录时间" || field.label == "最后登录 IP" {
			t.Fatalf("private login field leaked when IsHide=true: %+v", field)
		}
	}

	a.clicks = map[string]*widget.Clickable{}
	a.user = forum.User{ID: "IWhisper#93", UserName: "匿名"}
	ops.Reset()
	gtx.Ops = &ops
	a.userPage(gtx)
	if _, ok := a.clicks["user-mail"]; ok {
		t.Fatal("anonymous user exposed private mail action")
	}
	if _, ok := a.clicks["user-block"]; ok {
		t.Fatal("anonymous user exposed private blacklist action")
	}
}

func TestAccountParityPages(t *testing.T) {
	server := parityServer(t)
	a := parityApp(t, server.URL)
	defer a.Close()

	a.openUser("bob")
	waitForIdle(t, a)
	if a.user.ID != "bob" {
		t.Fatalf("user = %+v", a.user)
	}
	renderParityPage(t, a, a.userPage)

	a.back()
	a.openHistory()
	renderParityPage(t, a, a.historyPage)

	a.back()
	a.openBlacklist()
	waitForIdle(t, a)
	if len(a.blacklist.Users) != 1 {
		t.Fatalf("blacklist = %+v", a.blacklist)
	}
	renderParityPage(t, a, a.blacklistPage)

	a.back()
	a.openSettings()
	renderParityPage(t, a, a.settingsPage)
}

func TestMessageParityPages(t *testing.T) {
	server := parityServer(t)
	a := parityApp(t, server.URL)
	defer a.Close()

	a.openMessages("inbox")
	waitForIdle(t, a)
	if len(a.mailbox.Mails) != 1 {
		t.Fatalf("mailbox = %+v", a.mailbox)
	}
	renderParityPage(t, a, a.messagesPage)

	a.openMail("inbox", "1")
	waitForIdle(t, a)
	if a.mail.Title != "你好" || a.mail.Content != "来信正文" {
		t.Fatalf("mail = %+v", a.mail)
	}
	renderParityPage(t, a, a.mailPage)

	a.back()
	a.openChat("bob")
	waitForIdle(t, a)
	if len(a.chat) != 2 {
		t.Fatalf("chat = %+v", a.chat)
	}
	renderParityPage(t, a, a.chatPage)

	a.back()
	a.selectMessageTab("refers")
	waitForIdle(t, a)
	if len(a.refers.Articles) != 1 {
		t.Fatalf("refers = %+v", a.refers)
	}
	renderParityPage(t, a, a.messagesPage)
}

func TestMailComposeReplyUsesReplyEndpoint(t *testing.T) {
	server := parityServer(t)
	a := parityApp(t, server.URL)
	defer a.Close()
	a.mail = forum.Mail{Title: "你好", Index: "1", Author: forum.User{ID: "bob", UserName: "Bob"}}
	a.openMailCompose("bob", "inbox", "1", "reply")
	a.mailEditor.contentEditor.SetText("回复正文")
	a.sendMail()
	waitForIdle(t, a)
	if got := a.current().Page; got != PageChat {
		t.Fatalf("page after send = %v, want chat", got)
	}
	if got := a.current().Name; got != "bob" {
		t.Fatalf("chat target = %q", got)
	}
}

func TestDarkThemeAppliesPalette(t *testing.T) {
	server := parityServer(t)
	a := parityApp(t, server.URL)
	defer a.Close()
	defer func() {
		setPalette(false)
		colorHint = colorMuted
		a.dark, a.themed = false, false
		a.theme.Bg = colorBg
		a.theme.Fg = colorFg
	}()
	prefs := a.core.Local.Preferences()
	prefs.Theme = 2
	if err := a.core.Local.SetPreferences(context.Background(), prefs); err != nil {
		t.Fatal(err)
	}
	a.stack = []Route{{Page: PageSelf}}
	var router input.Router
	var ops op.Ops
	a.Layout(layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	})
	if !a.dark {
		t.Fatal("dark preference did not reach the palette")
	}
	if colorSurface.R > 0x40 || a.theme.Bg.R > 0x20 {
		t.Fatalf("palette is still light: surface=%v bg=%v", colorSurface, a.theme.Bg)
	}
}

func TestVoteAndBetSubmit(t *testing.T) {
	server := parityServer(t)
	a := parityApp(t, server.URL)
	defer a.Close()

	a.openVote("42")
	waitForIdle(t, a)
	if len(a.vote.Options) != 1 {
		t.Fatalf("vote = %+v", a.vote)
	}
	a.choice = "1"
	renderParityPage(t, a, a.votePage)

	a.back()
	a.openBet("42")
	waitForIdle(t, a)
	if len(a.bet.Items) != 1 {
		t.Fatalf("bet = %+v", a.bet)
	}
	renderParityPage(t, a, a.betPage)
}

func TestModuleTabsFitNarrowScreens(t *testing.T) {
	groups := []struct {
		name  string
		items []moduleTab
	}{
		{name: "section", items: []moduleTab{
			{Key: "all", Label: "全部分区"},
			{Key: "favorites", Label: "收藏版面"},
			{Key: "timeline", Label: "时间线"},
			{Key: "top", Label: "今日十大"},
		}},
		{name: "search", items: []moduleTab{
			{Key: "ai", Label: "AI 搜索"},
			{Key: "all", Label: "全站搜索"},
			{Key: "user", Label: "用户"},
		}},
		{name: "messages", items: []moduleTab{
			{Key: "contacts", Label: "私信"},
			{Key: "inbox", Label: "收件箱"},
			{Key: "outbox", Label: "发件箱"},
			{Key: "refers", Label: "回复/@"},
		}},
		{name: "refers", items: []moduleTab{
			{Key: "reply", Label: "回复"},
			{Key: "at", Label: "@ 我的"},
			{Key: "read-all", Label: "一键已读"},
		}},
	}
	for _, group := range groups {
		for _, sp := range []float32{1, 1.15} {
			t.Run(group.name+"/scale-"+itoa(int(sp*100)), func(t *testing.T) {
				a := previewApp(t)
				defer a.Close()
				var router input.Router
				var ops op.Ops
				gtx := layout.Context{
					Ops:         &ops,
					Source:      router.Source(),
					Constraints: layout.Exact(image.Pt(320, 760)),
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: sp},
				}
				var dims layout.Dimensions
				var panicValue any
				func() {
					defer func() { panicValue = recover() }()
					dims = a.moduleTabs(gtx, group.items, group.items[len(group.items)-1].Key, func(string) {})
				}()
				if panicValue != nil {
					t.Fatalf("module strip panicked: %v", panicValue)
				}
				if dims.Size.X > gtx.Constraints.Max.X {
					t.Fatalf("module strip overflow: got %v, max %v", dims.Size, gtx.Constraints.Max)
				}
				if dims.Size.Y <= 0 || dims.Size.Y > gtx.Constraints.Max.Y {
					t.Fatalf("module strip height=%d, constraints=%v", dims.Size.Y, gtx.Constraints.Max)
				}
			})
		}
	}
}

func TestDeletedVoteSubmitIsDisabled(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"vid":7,"aid":42,"title":"已删除投票","is_deleted":true,"options":[{"viid":1,"label":"选项"}]}`))
	}))
	defer server.Close()

	a := parityApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}, {Page: PageVote, ID: "42"}}
	a.vote = forum.Vote{
		ID: "7", ArticleID: "42", Title: "已删除投票", IsDeleted: true,
		Options: []forum.VoteOption{{ID: "1", Label: "选项"}},
	}
	a.choice = "1"
	if a.voteSubmitEnabled() {
		t.Fatal("deleted vote submit was reported as enabled")
	}
	renderParityPage(t, a, a.votePage)
	a.click("vote-submit").Click()
	renderParityPage(t, a, a.votePage)
	if requests != 0 {
		t.Fatalf("deleted vote submitted %d requests", requests)
	}
}

func TestBoardPinnedStateIsolatedByRoute(t *testing.T) {
	a := parityApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.board = forum.Board{Name: "Go", Articles: []forum.Article{{ID: "1", Title: "置顶", IsTop: true}}}
	a.stack = []Route{{Page: PageHome}, {Page: PageBoard, Name: "Go", Number: 1}}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 760)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.boardPage(gtx)
	a.click("board-pinned-toggle").Click()
	ops.Reset()
	a.boardPage(gtx)
	if !a.boardPinnedExpanded(a.current()) {
		t.Fatal("board pinned topics did not expand")
	}

	a.stack[len(a.stack)-1] = Route{Page: PageBoard, Name: "Other", Number: 1}
	a.board = forum.Board{Name: "Other", Articles: []forum.Article{{ID: "1", Title: "置顶", IsTop: true}}}
	ops.Reset()
	a.boardPage(gtx)
	if a.boardPinnedExpanded(a.current()) {
		t.Fatal("board pinned state leaked to another board")
	}
}
