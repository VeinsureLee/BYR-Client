package byr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
	"BYR-Client/internal/ports"
)

type fakeForum struct {
	t        *testing.T
	server   *httptest.Server
	requests []string
	queries  []string
}

func newFakeForum(t *testing.T) *fakeForum {
	t.Helper()
	f := &fakeForum{t: t}
	mux := http.NewServeMux()
	mux.HandleFunc("/index", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "nforum[UTMPUSERID]=guest; Path=/")
		fmt.Fprint(w, `<html><body><form><input name="passwd"></form></body></html>`)
	})
	mux.HandleFunc("/user/ajax_login.json", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("passwd") != "secret" {
			_ = json.NewEncoder(w).Encode(map[string]any{"ajax_st": 0, "ajax_text": "密码错误"})
			return
		}
		if r.Form.Get("id") != "alice" {
			_ = json.NewEncoder(w).Encode(map[string]any{"ajax_st": 0, "ajax_text": "账号不存在"})
			return
		}
		w.Header().Add("Set-Cookie", "nforum[UTMPUSERID]=alice; Path=/")
		w.Header().Add("Set-Cookie", "nforum[PASSWORD]=login-token; Path=/")
		_ = json.NewEncoder(w).Encode(map[string]any{"ajax_st": 1, "user_name": "Alice", "is_login": true})
	})
	mux.HandleFunc("/default", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Cookie"), "nforum[PASSWORD]=login-token") {
			fmt.Fprint(w, `<html><body><form><input name="passwd"></form></body></html>`)
			return
		}
		fmt.Fprint(w, `<html><body>logged in</body></html>`)
	})
	mux.HandleFunc("/section/ajax_list.json", func(w http.ResponseWriter, r *http.Request) {
		f.requests = append(f.requests, r.URL.Path)
		if r.URL.Path == "/section/ajax_list.json" {
			f.queries = append(f.queries, r.URL.RawQuery)
		}
		if !strings.Contains(r.Header.Get("Cookie"), "nforum[PASSWORD]=login-token") {
			fmt.Fprint(w, `<div class="error">您未登录</div>`)
			return
		}
		switch r.URL.Query().Get("root") {
		case "list-section":
			fmt.Fprint(w, `[{"t":"<a href=\"/section/1\">技术</a>","id":"sec-1"},{"t":"<a href=\"/section/2\">校园</a>","id":"sec-2"},{"t":"<a href=\"/board/RootBoard\">根版</a>","id":"b0"}]`)
		case "sec-1":
			fmt.Fprint(w, `[{"t":"<a href=\"/board/Go\">Go</a>","id":11},{"t":"<a href=\"/section/3\">编程</a>","id":3}]`)
		case "sec-2":
			// Exercise the GBK decoder used by the real forum.
			encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(`[{"t":"<a href=\"/board/Campus\">校园生活</a>","id":"b2"}]`))
			if err != nil {
				f.t.Fatal(err)
			}
			w.Header().Set("Content-Type", "application/json; charset=gbk")
			_, _ = w.Write(encoded)
		case "3":
			fmt.Fprint(w, `[{"t":"<a href=\"/board/Front\">前端</a>","id":"b3"}]`)
		default:
			fmt.Fprint(w, `[]`)
		}
	})
	mux.HandleFunc("/board/Go", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Cookie"), "nforum[PASSWORD]=login-token") {
			fmt.Fprint(w, "您未登录")
			return
		}
		fmt.Fprint(w, `<html><body><table class="board-list"><tbody>
<tr class="top"><td class="title_9"><a href="/article/Go/1001">置顶主题</a></td><td class="title_10">2026-10-01</td><td class="title_12"><a href="/user/query/alice">alice</a></td><td class="title_11">3</td><td class="title_10">2026-10-02 12:00:00</td></tr>
<tr><td class="title_9"><a href="/article/Go/1002">普通主题</a></td><td class="title_10">2026-10-05</td><td class="title_12"><a href="/user/query/bob">bob</a></td><td class="title_11">8</td><td class="title_10">2026-10-06 09:30:00</td></tr>
</tbody></table><ul class="pagination"><a title="下一页" href="?p=2">下一页</a></ul></body></html>`)
	})
	mux.HandleFunc("/section/1", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("count") != "1" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `<html><body><table class="board-list"><tbody>
<tr><td class="title_1"><a href="/board/Go">Go</a></td><td class="title_4">5</td><td class="title_5">12</td><td class="title_6">100</td><td class="title_7">2000</td></tr>
</tbody></table></body></html>`)
	})
	mux.HandleFunc("/att/photo.png", func(w http.ResponseWriter, r *http.Request) {
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		img.Set(0, 0, color.RGBA{R: 255, A: 255})
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, img)
	})
	mux.HandleFunc("/article/Go/1002", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><div class="b-head"><span class="n-left">文章主题: 普通主题</span></div>
<div class="a-wrap"><a name="a0"></a><div class="a-pos">楼主</div><div class="a-u-name">alice</div><div class="a-content-wrap">发信人: alice (Alice), 信区: Go
发信站: 北邮人论坛 (Mon Oct 05 10:00:00 2026), 站内

主帖正文
<img src="/att/photo.png">
<img src="/static/image/smiley/face.gif" class="smiley">
--
※ 来源:·网页 bbs.byr.cn·[FROM: 1.2.3.4]</div></div>
<div class="a-wrap"><a name="a1"></a><div class="a-pos">沙发</div><div class="a-u-name">bob</div><div class="a-content-wrap">发信人: bob (Bob), 信区: Go
发信站: 北邮人论坛 (Tue Oct 06 11:00:00 2026), 站内

【 在 alice 的大作中提到: 】
: 主帖正文
: 第二行

回复正文</div></div>
</body></html>`)
	})
	mux.HandleFunc("/s/article", func(w http.ResponseWriter, r *http.Request) {
		f.queries = append(f.queries, r.URL.RawQuery)
		fmt.Fprint(w, `<html><body><table class="board-list tiz"><tbody>
<tr><td class="title_9"><a href="/article/Go/1002">普通主题</a></td><td class="title_10">2026-10-05</td><td class="title_12"><a href="/user/query/bob">bob</a></td><td class="title_11">8</td><td class="title_10">2026-10-06 09:30:00</td></tr>
</tbody></table></body></html>`)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeForum) client() *Client {
	return New(Config{BaseURL: f.server.URL, HTTPClient: f.server.Client()})
}

func TestLoginAndCatalogInitialization(t *testing.T) {
	fake := newFakeForum(t)
	client := fake.client()
	if _, err := client.ReadCatalog(context.Background()); err != ports.ErrAuthRequired {
		t.Fatalf("catalog without login: %v", err)
	}
	if _, err := client.Login(context.Background(), "alice", "wrong"); err == nil || !strings.Contains(err.Error(), "密码错误") {
		t.Fatalf("wrong password error = %v", err)
	}
	session, err := client.Login(context.Background(), "alice", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if !session.LoggedIn || session.DisplayName != "Alice" || session.AccountID != "alice" {
		t.Fatalf("session = %+v", session)
	}
	catalog, err := client.ReadCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Roots) != 3 {
		t.Fatalf("roots = %#v", catalog.Roots)
	}
	if len(catalog.Nodes) != 8 { // 3 forums + 1 sub forum + 4 boards
		t.Fatalf("nodes = %d: %#v", len(catalog.Nodes), catalog.Nodes)
	}
	if _, err := forum.NewNavigation(catalog.Roots, catalog.Nodes); err != nil {
		t.Fatalf("catalog is not a valid navigation tree: %v", err)
	}
	names := make(map[string]string)
	for _, node := range catalog.Nodes {
		names[string(node.ID)] = node.Name
	}
	foundGBK := false
	for _, node := range catalog.Nodes {
		if node.Name == "校园生活" {
			foundGBK = true
		}
	}
	if !foundGBK {
		t.Fatalf("GBK board name was not decoded: %#v", names)
	}
}

func TestBoardThreadAndSearchPages(t *testing.T) {
	fake := newFakeForum(t)
	client := fake.client()
	if _, err := client.Login(context.Background(), "alice", "secret"); err != nil {
		t.Fatal(err)
	}
	page, err := client.ReadBoardPage(context.Background(), domain.Scope{}, "Go", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Articles) != 2 || !page.Articles[0].Pinned {
		t.Fatalf("board page = %+v", page)
	}
	if page.Next != "2" || page.End {
		t.Fatalf("pagination = next %q end %v", page.Next, page.End)
	}
	if page.Articles[1].Author != "bob" || page.Articles[1].ReplyCount != 8 {
		t.Fatalf("article = %+v", page.Articles[1])
	}

	thread, err := client.ReadThreadPage(context.Background(), domain.Scope{}, page.Articles[1].Key, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if thread.Article.Title != "普通主题" || thread.Article.Author != "alice" || !thread.Article.BodyLoaded {
		t.Fatalf("thread article = %+v", thread.Article)
	}
	if strings.TrimSpace(thread.Article.Content) != "主帖正文" {
		t.Fatalf("content = %q", thread.Article.Content)
	}
	if len(thread.Article.Images) != 2 || thread.Article.Images[0].URL != fake.server.URL+"/att/photo.png" {
		t.Fatalf("images = %#v", thread.Article.Images)
	}
	if thread.Article.Images[0].Kind != domain.ImagePhoto || thread.Article.Images[1].Kind != domain.ImageEmoji {
		t.Fatalf("image kinds = %+v", thread.Article.Images)
	}
	imageData, err := client.ReadImage(context.Background(), thread.Article.Images[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(imageData) == 0 || !bytes.HasPrefix(imageData, []byte("\x89PNG")) {
		t.Fatalf("image data = %d bytes", len(imageData))
	}
	if len(thread.Replies) != 1 || thread.Replies[0].Floor != 2 || thread.Replies[0].Author != "bob" {
		t.Fatalf("replies = %+v", thread.Replies)
	}
	if thread.Replies[0].Content != "回复正文" {
		t.Fatalf("reply content = %q", thread.Replies[0].Content)
	}
	if thread.Replies[0].QuoteAuthor != "alice" || thread.Replies[0].QuoteContent != "主帖正文\n第二行" {
		t.Fatalf("reply quote = %q / %q", thread.Replies[0].QuoteAuthor, thread.Replies[0].QuoteContent)
	}

	search, err := client.SearchBoardPage(context.Background(), domain.Scope{}, "Go", domain.BoardQuery{
		Keywords: []string{"Go", "实习"},
		Author:   "alice",
	}, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(search.Articles) != 1 {
		t.Fatalf("search articles = %+v", search.Articles)
	}
	lastQuery := fake.queries[len(fake.queries)-1]
	if !strings.Contains(lastQuery, "t1=Go%20%E5%AE%9E%E4%B9%A0") {
		t.Fatalf("search query did not encode spaces as %%20: %q", lastQuery)
	}
	if !strings.Contains(lastQuery, "au=alice") {
		t.Fatalf("search query missing author: %q", lastQuery)
	}
}

func TestSessionPersistenceAndRestore(t *testing.T) {
	fake := newFakeForum(t)
	path := t.TempDir() + "/session.json"
	client := New(Config{BaseURL: fake.server.URL, HTTPClient: fake.server.Client(), SessionFile: path})
	if _, err := client.Login(context.Background(), "alice", "secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file not written: %v", err)
	}

	restored := New(Config{BaseURL: fake.server.URL, HTTPClient: fake.server.Client(), SessionFile: path})
	session, ok, err := restored.Restore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !session.LoggedIn || session.DisplayName != "Alice" {
		t.Fatalf("restored session = %+v ok=%v", session, ok)
	}
	if err := restored.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file survived logout: %v", err)
	}
}

func TestSectionTrafficParsing(t *testing.T) {
	fake := newFakeForum(t)
	client := fake.client()
	if _, err := client.Login(context.Background(), "alice", "secret"); err != nil {
		t.Fatal(err)
	}
	values, err := client.ReadSectionTraffic(context.Background(), domain.Scope{}, "1", []domain.BoardID{"Go"})
	if err != nil {
		t.Fatal(err)
	}
	traffic, ok := values["Go"]
	if !ok || !traffic.Known {
		t.Fatalf("traffic = %+v", traffic)
	}
	if traffic.OnlineUsers != 5 || traffic.TodayPosts != 12 || traffic.Threads != 100 || traffic.Posts != 2000 {
		t.Fatalf("traffic = %+v", traffic)
	}
}
