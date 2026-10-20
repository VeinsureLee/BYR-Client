package desktop

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"BYR-Client/internal/domain"
)

func articleLink(key domain.ArticleKey) string {
	base := strings.TrimRight(byrBaseURL(), "/")
	return base + "/article/" + url.PathEscape(string(key.BoardID)) + "/" + url.PathEscape(string(key.ArticleID))
}

func articleFloorLink(key domain.ArticleKey, floor int) string {
	link := articleLink(key)
	if floor > 1 {
		link += "#a" + itoa(floor-1)
	}
	return link
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}

func (u *UI) shareArticleButtons(c *ui.Context, key domain.ArticleKey) {
	link := articleLink(key)
	ui.Button(c, "复制链接").FontSize(u.fs(12)).OnClick(func() {
		c.WriteClipboard(link)
		c.Toast("帖子链接已复制")
	})
	ui.Button(c, "更多").FontSize(u.fs(12)).Menu(func(m *ui.Menu) {
		if m.Item("应用内打开").Chosen() {
			if err := u.openLoggedInBrowserWindow(link); err != nil {
				c.Toast(err.Error())
			}
		}
		if m.Item("系统浏览器").Chosen() {
			c.OpenURL(link)
		}
		m.Separator()
		if m.Item("导出 PNG").Chosen() {
			u.exportThreadPNG(key)
		}
	})
}

func (u *UI) openLoggedInBrowser(c *ui.Context, link string) {
	if err := u.openLoggedInBrowserWindow(link); err != nil {
		c.Toast(err.Error())
	}
}

func (u *UI) openLoggedInBrowserWindow(link string) error {
	if u.app == nil {
		return fmt.Errorf("应用尚未就绪")
	}
	var web *mygo.WebView
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:     "BYR Client · 帖子",
		Width:     1100,
		Height:    820,
		MinWidth:  720,
		MinHeight: 520,
		Content: ui.View(func(inner *ui.Context) {
			if web == nil {
				ui.Column(inner).Fill().Center().Children(func() {
					ui.Spinner(inner)
					ui.Text(inner, "正在打开内置浏览器…").FontSize(u.fs(12)).TextColor(inner.Theme().TextMuted)
				})
				return
			}
			ui.WebView(inner, web).Fill()
		}),
	})
	created, err := win.NewWebView(mygo.WebViewOptions{URL: "about:blank", BackgroundColor: "#FFFFFF"})
	if err != nil {
		win.Close()
		return fmt.Errorf("打开内置浏览器失败：%v", err)
	}
	web = created
	win.Invalidate()

	failed := false
	for _, cookie := range u.app.BrowserCookies() {
		if err := mygo.Cookies.Set(mygo.Cookie{
			Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain,
			Path: cookie.Path, Secure: cookie.Secure, HTTPOnly: cookie.HTTPOnly,
		}); err != nil {
			failed = true
		}
	}
	if err := created.Page().LoadURL(link); err != nil {
		win.Close()
		return fmt.Errorf("打开页面失败：%v", err)
	}

	u.browserWindows = append(u.browserWindows, win)
	win.OnClosed(func() {
		for index, item := range u.browserWindows {
			if item == win {
				u.browserWindows = append(u.browserWindows[:index], u.browserWindows[index+1:]...)
				break
			}
		}
	})
	if failed {
		return fmt.Errorf("部分登录 Cookie 写入失败，内置浏览器可能需要重新登录")
	}
	return nil
}

func (u *UI) shareReplyButton(c *ui.Context, key domain.ArticleKey, floor int) {
	link := articleFloorLink(key, floor)
	ui.Button(c, "").Label("复制本楼链接").Tooltip("复制本楼链接").Padding(2, 2).Radius(5).
		Children(func() { ui.Icon(c, iconShare).FontSize(u.fs(12)) }).OnClick(func() {
		c.WriteClipboard(link)
		c.Toast("本楼链接已复制")
	})
}
