package desktop

import (
	"context"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func TestAccountManagerFromProfileShowsCurrentAccountAndAddFlow(t *testing.T) {
	h := newHarness(t)
	if err := h.app.Core.Login(context.Background(), "alice", "pw", true); err != nil {
		t.Fatal(err)
	}
	h.app.route = Route{Page: "self"}
	h.app.user = forum.User{ID: "alice", UserName: "Alice"}
	h.view.Frame()
	h.click("账号管理")
	if h.app.route.Page != "accounts" || len(h.app.savedAccounts) != 1 {
		t.Fatalf("account manager route/data = %+v %+v", h.app.route, h.app.savedAccounts)
	}
	if account := h.app.savedAccounts[0]; account.Username != "alice" || !account.Active || !account.Remembered {
		t.Fatalf("current account summary = %+v", account)
	}
	h.click("添加账号")
	if h.app.route.Page != "login" || h.app.route.Name != "add-account" {
		t.Fatalf("add account route = %+v", h.app.route)
	}
	if !h.app.remember {
		t.Fatal("add-account flow did not enable credential remembering")
	}
}

func TestAccountManagerCanOpenFromLogin(t *testing.T) {
	h := newHarness(t)
	h.view.Frame()
	h.click("管理已保存账号")
	if h.app.route.Page != "accounts" {
		t.Fatalf("login account-manager route = %+v", h.app.route)
	}
}
