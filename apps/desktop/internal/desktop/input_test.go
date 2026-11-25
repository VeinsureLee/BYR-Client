package desktop

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func TestDesktopLoginInputsPreserveCaseAndSuffix(t *testing.T) {
	h := newHarness(t)
	username := "aBcD-URL_9"
	password := "PqRs-URL_9"
	if err := h.view.Click("用户名"); err != nil {
		t.Fatal(err)
	}
	for _, r := range username {
		h.view.Type(string(r))
	}
	h.view.Type("-TailEN")
	if err := h.view.Click("密码"); err != nil {
		t.Fatal(err)
	}
	for _, r := range password {
		h.view.Type(string(r))
	}
	h.view.Type("-TailEN")
	h.idle()
	if h.app.username != username+"-TailEN" || h.app.password != password+"-TailEN" {
		t.Fatal("login editor did not retain the complete mixed-case Unicode values")
	}

	h.click("登录")
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.authForms) != 1 {
		t.Fatalf("OAuth login forms = %d, requests=%v route=%+v failed=%t message=%q",
			len(h.authForms), h.requests, h.app.route, h.app.failed, h.app.message)
	}
	decodedUsername, err := base64.StdEncoding.DecodeString(h.authForms[0].values.Get("username"))
	if err != nil || string(decodedUsername) != username+"-TailEN" {
		t.Fatalf("OAuth username did not preserve the typed value: decode error=%v", err)
	}
	decodedPassword, err := base64.StdEncoding.DecodeString(h.authForms[0].values.Get("password"))
	if err != nil || string(decodedPassword) != password+"-TailEN" {
		t.Fatalf("OAuth password did not preserve the typed value: decode error=%v", err)
	}
}

func TestDesktopLoginRejectsUnsupportedUnicodeAtSubmit(t *testing.T) {
	h := newHarness(t)
	h.typeIn("用户名", "用户😀")
	h.typeIn("密码", "TestPassword")
	h.click("登录")
	if !h.app.failed || !strings.Contains(h.app.message, "单字节") {
		t.Fatalf("unsupported login characters did not produce visible validation: failed=%t message=%q", h.app.failed, h.app.message)
	}
	if got := h.count("POST /oauth2/official"); got != 0 {
		t.Fatalf("invalid Unicode login issued %d OAuth requests", got)
	}
}

func TestDesktopSearchInputPreservesMixedCaseSuffix(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.goTo(Route{Page: "search"})
	query := "aBcD-URL_9中文😀"
	h.typeIn("搜索关键词", query)
	h.view.Type("-TailEN")
	h.idle()
	h.click("开始搜索")

	if count := h.count("GET /open/search/article.json"); count == 0 {
		t.Fatal("search button did not issue a request")
	}
	values, err := url.ParseQuery(h.query("GET /open/search/article.json"))
	if err != nil {
		t.Fatalf("parse search query: %v", err)
	}
	if got, want := values.Get("title1"), query+"-TailEN"; got != want {
		t.Fatalf("server search title = %q, want %q", got, want)
	}
}
