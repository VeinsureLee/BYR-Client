package ui

import (
	"context"
	"image"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

type testCredentials struct{ session application.Credential }

func (c *testCredentials) Load(context.Context) (application.Credential, error) {
	return c.session, nil
}
func (c *testCredentials) Save(_ context.Context, v application.Credential) error {
	c.session = v
	return nil
}
func (c *testCredentials) Clear(context.Context) error {
	c.session = application.Credential{}
	return nil
}

func TestLoginButtonClickStartsLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth2/official":
			w.Write([]byte(`{"access_token":"token","expires_in":3600}`))
		default:
			w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	defer server.Close()
	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	core, err := application.New(application.Options{BaseURL: server.URL, Local: store, Credentials: &testCredentials{}})
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.ready = true
	a.core = core
	a.stack = []Route{{Page: PageLogin}}
	a.username.SetText("alice")
	a.password.SetText("pw")
	a.click("login").Click()

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
	}
	a.Layout(gtx)
	if !a.busy || a.message != "正在登录" {
		t.Fatalf("login click did not start: busy=%v message=%q", a.busy, a.message)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for {
			select {
			case fn := <-a.updates:
				fn()
				continue
			default:
			}
			break
		}
		if a.stack[0].Page == PageHome {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("login did not reach home: page=%v message=%q", a.stack[0].Page, a.message)
}

func TestLogoutReturnsToLoginState(t *testing.T) {
	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	core, err := application.New(application.Options{Local: store, Credentials: &testCredentials{}})
	if err != nil {
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
	a.user = forum.User{ID: "alice", UserName: "Alice"}
	a.username.SetText("alice")
	a.logout()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for {
			select {
			case fn := <-a.updates:
				fn()
				continue
			default:
			}
			break
		}
		if !a.busy {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(a.stack) != 1 || a.stack[0].Page != PageLogin {
		t.Fatalf("logout did not return to login: %+v", a.stack)
	}
	if !a.ready || a.message != "" || a.username.Text() != "" {
		t.Fatalf("logout left stale UI state: ready=%v message=%q username=%q", a.ready, a.message, a.username.Text())
	}
	var router input.Router
	var ops op.Ops
	a.Layout(layout.Context{Ops: &ops, Source: router.Source(), Constraints: layout.Exact(image.Pt(420, 820))})
}
