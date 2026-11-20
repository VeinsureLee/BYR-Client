package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestAccountManagerShowsCurrentAccountAndStartsAddLogin(t *testing.T) {
	server := parityServer(t)
	a := parityApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageSelf}}
	a.openAccountManager()
	waitForIdle(t, a)
	if a.current().Page != PageAccounts || len(a.savedAccounts) != 1 {
		t.Fatalf("account manager route/data = %+v %+v", a.current(), a.savedAccounts)
	}
	if account := a.savedAccounts[0]; account.Username != "alice" || !account.Active || !account.Remembered {
		t.Fatalf("current account summary = %+v", account)
	}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	if dims := a.accountsPage(gtx); dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("account manager rendered blank: %+v", dims.Size)
	}
	router.Frame(&ops)
	a.click("accounts-add").Click()
	ops.Reset()
	gtx.Ops = &ops
	a.accountsPage(gtx)
	if got := a.current(); got.Page != PageLogin || got.Name != "add-account" {
		t.Fatalf("add-account route = %+v", got)
	}
	if !a.remember.Value {
		t.Fatal("adding an account did not default to remembering its credential")
	}
}

func TestLoggedOutUserCanOpenSavedAccountManager(t *testing.T) {
	server := parityServer(t)
	a := parityApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageLogin}}
	if !a.push(Route{Page: PageAccounts}) {
		t.Fatal("account manager was blocked from login page")
	}
	a.loadAccounts()
	waitForIdle(t, a)
	if a.current().Page != PageAccounts || len(a.savedAccounts) != 1 {
		t.Fatalf("logged-out account manager = route:%+v accounts:%+v", a.current(), a.savedAccounts)
	}
}
