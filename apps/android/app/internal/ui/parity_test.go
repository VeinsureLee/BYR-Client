package ui

import (
	"context"
	"image"
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

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func parityServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case path == "/open/section.json":
			_, _ = w.Write([]byte(`{"section":[{"name":"0","description":"校园"},{"name":"2","description":"生活"}]}`))
		case path == "/open/section/0.json":
			_, _ = w.Write([]byte(`{"name":"0","description":"校园","sub_section":[{"name":"1","description":"院系"}],"board":[{"name":"Go","description":"Go 讨论","threads_today_count":3}]}`))
		case path == "/open/section/1.json":
			_, _ = w.Write([]byte(`{"name":"1","description":"院系","board":[{"name":"Go","description":"Go 讨论"}]}`))
		case path == "/open/user/query/bob.json":
			_, _ = w.Write([]byte(`{"id":"bob","user_name":"Bob","post_count":3,"score":10,"face_url":""}`))
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
