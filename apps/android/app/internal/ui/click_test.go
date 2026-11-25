package ui

import (
	"context"
	"image"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
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
	core := newUIFixtureCore(t, server.URL, http.DefaultClient, nil, &testCredentials{})
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
		if a.stack[0].Page == PageTop && a.topTab == "top" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("login did not reach today's top: page=%v tab=%q message=%q", a.stack[0].Page, a.topTab, a.message)
}

func TestPillRegistersPhysicalTap(t *testing.T) {
	testPhysicalControlTap(t, func(a *App, gtx layout.Context) (layout.Dimensions, bool) {
		return a.pill(gtx, "physical-pill", "收藏文章", "star", false, false)
	})
}

func TestFilledPillRegistersPhysicalTap(t *testing.T) {
	testPhysicalControlTap(t, func(a *App, gtx layout.Context) (layout.Dimensions, bool) {
		return a.filledPill(gtx, "physical-filled-pill", "回复主题", "reply")
	})
}

func testPhysicalControlTap(t *testing.T, control func(*App, layout.Context) (layout.Dimensions, bool)) {
	t.Helper()
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(420, 120)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	dims, clicked := control(a, gtx)
	if clicked {
		t.Fatal("control reported a click before pointer input")
	}
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("control has no tap area: %+v", dims.Size)
	}
	router.Frame(&ops)
	router.Queue(
		pointer.Event{Source: pointer.Touch, Kind: pointer.Press, PointerID: 1, Position: f32.Pt(float32(dims.Size.X)/2, float32(dims.Size.Y)/2)},
		pointer.Event{Source: pointer.Touch, Kind: pointer.Release, PointerID: 1, Position: f32.Pt(float32(dims.Size.X)/2, float32(dims.Size.Y)/2)},
	)
	ops.Reset()
	gtx.Ops = &ops
	_, clicked = control(a, gtx)
	if !clicked {
		t.Fatal("physical press/release did not click control")
	}
}

func TestLogoutReturnsToLoginState(t *testing.T) {
	core := newUIFixtureCore(t, "", http.DefaultClient, nil, &testCredentials{})
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
		a.drain()
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
