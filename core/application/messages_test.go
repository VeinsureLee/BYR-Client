package application

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func TestMessageUseCasesPreserveMailSnapshotAndValidateTargets(t *testing.T) {
	var sendForm url.Values
	var mu sync.Mutex
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/open/mail/send.json":
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse send form: %v", err)
			}
			mu.Lock()
			sendForm = make(url.Values, len(r.PostForm))
			for key, values := range r.PostForm {
				sendForm[key] = append([]string(nil), values...)
			}
			mu.Unlock()
			io.WriteString(w, `{"code":"0000"}`)
		case "/open/mail/inbox.json":
			io.WriteString(w, `{"mail":[{"index":7,"title":"Inbox preview","user":{"id":"bob"}}],"pagination":{"page_current_count":1}}`)
		case "/open/mail/inbox/7.json":
			io.WriteString(w, `{"index":7,"title":"Inbox title","content":"Inbox body","user":{"id":"bob"}}`)
		case "/open/refer/reply.json":
			io.WriteString(w, `{"article":[{"index":9,"title":"提醒"}]}`)
		default:
			t.Errorf("unexpected message endpoint %s", r.URL.Path)
			io.WriteString(w, `{}`)
		}
	}))
	defer server.Close()
	a := newLookupTestApp(t, server.URL, &credentials{value: Credential{Username: "alice", Token: "fixture-token"}})

	if _, err := a.SendMail(context.Background(), "  bob  ", "MiXeD 😀 title ", "一行\n第二行  "); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	form := sendForm
	mu.Unlock()
	if form.Get("id") != "bob" || form.Get("title") != "MiXeD 😀 title " || form.Get("content") != "一行\n第二行  " {
		t.Fatalf("send snapshot = %v", form)
	}
	mailbox, err := a.LoadMailbox(context.Background(), " INBOX ", forum.Page{Number: 1, Size: 30})
	if err != nil || len(mailbox.Mails) != 1 || mailbox.Mails[0].Index != forum.Scalar("7") {
		t.Fatalf("mailbox = %+v error=%v", mailbox, err)
	}
	mail, err := a.ReadMail(context.Background(), "inbox", "7")
	if err != nil || mail.Box != "inbox" || mail.Title != "Inbox title" {
		t.Fatalf("read mail = %+v error=%v", mail, err)
	}
	refers, err := a.LoadRefers(context.Background(), "reply", forum.Page{Number: 1, Size: 30})
	if err != nil || len(refers.Articles) != 1 || refers.Articles[0].Index != forum.Scalar("9") {
		t.Fatalf("refers = %+v error=%v", refers, err)
	}
	if _, err := a.SendMail(context.Background(), " ", "title", "body"); err == nil {
		t.Fatal("blank recipient was accepted")
	}
	if _, err := a.LoadMailbox(context.Background(), "sent", forum.Page{Number: 1}); err == nil {
		t.Fatal("unsupported mailbox was accepted")
	}
	if got := calls.Load(); got != 4 {
		t.Fatalf("invalid input made requests: total calls=%d, want 4", got)
	}
}

func TestMailboxDiscardsResponseAfterAccountSwitch(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("oauth_token"); got != "alice-token" {
			t.Errorf("in-flight mailbox token = %q, want original Alice token", got)
		}
		close(started)
		<-release
		io.WriteString(w, `{"mail":[]}`)
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	creds := &accountsCredentialStore{
		accounts: []Credential{{Username: "alice", Token: "alice-token"}, {Username: "bob", Token: "bob-token"}},
		active:   "alice",
	}
	a := newLookupTestApp(t, server.URL, creds)
	type result struct {
		mailbox forum.Mailbox
		err     error
	}
	completed := make(chan result, 1)
	go func() {
		mailbox, err := a.LoadMailbox(context.Background(), "inbox", forum.Page{Number: 1, Size: 30})
		completed <- result{mailbox: mailbox, err: err}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("mailbox request did not start")
	}
	if err := a.SwitchAccount(context.Background(), "bob"); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case got := <-completed:
		if !errors.Is(got.err, ErrStaleSession) || len(got.mailbox.Mails) != 0 {
			t.Fatalf("old-account mailbox result = %+v error=%v", got.mailbox, got.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("mailbox request did not finish after release")
	}
}
