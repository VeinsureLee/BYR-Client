package forum

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Expected paths/forms are independent fixtures transcribed from the Flutter
// contracts. In particular do not derive expected paths from resource().
func TestMigratedEndpointContracts(t *testing.T) {
	ctx := context.Background()
	type contract struct {
		name, method, path string
		form, urlQuery     url.Values
		call               func(*Client) error
		public             bool
	}
	cases := []contract{
		{name: "article", method: "GET", path: "/open/article/Go/42.json", call: func(c *Client) error { _, e := c.Article(ctx, "Go", "42"); return e }},
		{name: "new post", method: "POST", path: "/open/article/Go/post.json", form: url.Values{"title": {"标题"}, "content": {"正文"}, "anonymous": {"0"}, "signature": {"0"}, "email": {"0"}, "outgo": {"0"}}, call: func(c *Client) error {
			_, e := c.PostArticle(ctx, Post{Board: "Go", Title: "标题", Content: "正文"})
			return e
		}},
		{name: "reply flags", method: "POST", path: "/open/article/Go/post.json", form: url.Values{"title": {"Re"}, "content": {"reply"}, "reid": {"42"}, "anonymous": {"1"}, "signature": {"1"}, "email": {"1"}, "outgo": {"1"}}, call: func(c *Client) error {
			_, e := c.PostArticle(ctx, Post{Board: "Go", Title: "Re", Content: "reply", ReplyID: "42", Anonymous: true, Signature: true, Email: true, Outgo: true})
			return e
		}},
		{name: "edit", method: "POST", path: "/open/article/Go/update/42.json", form: url.Values{"title": {"新标题"}, "content": {"新正文"}}, call: func(c *Client) error { _, e := c.UpdateArticle(ctx, "Go", "42", "新标题", "新正文"); return e }},
		{name: "delete", method: "POST", path: "/open/article/Go/delete/42.json", call: func(c *Client) error { _, e := c.DeleteArticle(ctx, "Go", "42"); return e }},
		{name: "like", method: "GET", path: "/open/article/Go/like/42.json", call: func(c *Client) error { _, e := c.Like(ctx, "Go", "42"); return e }},
		{name: "downvote", method: "GET", path: "/open/article/Go/votedown/42.json", call: func(c *Client) error { _, e := c.VoteDown(ctx, "Go", "42"); return e }},
		{name: "attachments", method: "GET", path: "/open/attachment/Go/42.json", call: func(c *Client) error { _, e := c.Attachments(ctx, "Go", "42"); return e }},
		{name: "delete attachment", method: "POST", path: "/open/attachment/Go/delete/100.json", call: func(c *Client) error { _, e := c.DeleteAttachment(ctx, "Go", "100"); return e }},
		{name: "favorite", method: "GET", path: "/open/favorite/0.json", call: func(c *Client) error { _, e := c.Favorites(ctx, "0"); return e }},
		{name: "favorite add", method: "POST", path: "/open/favorite/add/0.json", form: url.Values{"name": {"Go"}, "dir": {"0"}}, call: func(c *Client) error { _, e := c.AddFavorite(ctx, "Go", "0"); return e }},
		{name: "favorite delete", method: "POST", path: "/open/favorite/delete/0.json", form: url.Values{"name": {"Go"}, "dir": {"0"}}, call: func(c *Client) error { _, e := c.RemoveFavorite(ctx, "Go", "0"); return e }},
		{name: "collection", method: "GET", path: "/open/collection.json", urlQuery: url.Values{"page": {"2"}, "count": {"20"}}, call: func(c *Client) error { _, e := c.Collection(ctx, "", Page{Number: 2, Size: 20}); return e }},
		{name: "collect", method: "POST", path: "/open/collection/add.json", form: url.Values{"board": {"Go"}, "id": {"42"}}, call: func(c *Client) error { _, e := c.AddCollection(ctx, "Go", "42"); return e }},
		{name: "uncollect", method: "POST", path: "/open/collection/delete.json", form: url.Values{"board": {"Go"}, "id": {"42"}}, call: func(c *Client) error { _, e := c.RemoveCollection(ctx, "Go", "42"); return e }},
		{name: "favorposts", method: "GET", path: "/open/favorpost.json", urlQuery: url.Values{"page": {"1"}, "count": {"50"}}, call: func(c *Client) error { _, e := c.FavorPosts(ctx, Page{}); return e }},
		{name: "topten", method: "GET", path: "/open/widget/topten.json", call: func(c *Client) error { _, e := c.TopTen(ctx); return e }},
		{name: "recommend", method: "GET", path: "/open/widget/recommend.json", call: func(c *Client) error { _, e := c.Recommendations(ctx); return e }},
		{name: "article search", method: "GET", path: "/open/search/article.json", urlQuery: url.Values{
			"board": {"Go"}, "title1": {"北邮"}, "title2": {"校园"}, "titlen": {"广告"},
			"author": {"alice"}, "day": {"7"}, "m": {"1"}, "a": {"2"}, "o": {"2"},
			"page": {"2"}, "count": {"20"},
		}, call: func(c *Client) error {
			_, e := c.SearchArticles(ctx, Search{Board: "Go", Title: "北邮", Title2: "校园", ExcludeTitle: "广告", Author: "alice", Day: "7", Marked: "1", Attachment: "2", Order: "2", Page: Page{Number: 2, Size: 20}})
			return e
		}},
		{
			name:   "article full-site search",
			method: "GET",
			path:   "/open/search/article.json",
			urlQuery: url.Values{
				"board": {""}, "title1": {"北邮"}, "title2": {""}, "titlen": {""},
				"author": {""}, "day": {""}, "m": {""}, "a": {""}, "o": {"1"},
				"page": {"2"}, "count": {"30"},
			},
			call: func(c *Client) error {
				_, err := c.SearchArticles(ctx, Search{
					Title: "北邮",
					Page:  Page{Number: 2, Size: 30},
				})
				return err
			},
		},
		{name: "board search", method: "GET", path: "/open/search/board.json", urlQuery: url.Values{"board": {"北邮"}}, call: func(c *Client) error { _, e := c.SearchBoards(ctx, "北邮"); return e }},
		{name: "self", method: "GET", path: "/open/user/getinfo.json", call: func(c *Client) error { _, e := c.Self(ctx); return e }},
		{name: "user", method: "GET", path: "/open/user/query/alice.json", call: func(c *Client) error { _, e := c.User(ctx, "alice"); return e }},
		{name: "blacklist", method: "GET", path: "/open/blacklist/list.json", urlQuery: url.Values{"page": {"3"}}, call: func(c *Client) error { _, e := c.Blacklist(ctx, 3); return e }},
		{name: "block", method: "POST", path: "/open/blacklist/add.json", form: url.Values{"id": {"alice"}}, call: func(c *Client) error { _, e := c.AddBlacklist(ctx, "alice"); return e }},
		{name: "unblock", method: "POST", path: "/open/blacklist/delete.json", form: url.Values{"id": {"alice"}}, call: func(c *Client) error { _, e := c.RemoveBlacklist(ctx, "alice"); return e }},
		{name: "mailbox", method: "GET", path: "/open/mail/inbox.json", urlQuery: url.Values{"page": {"1"}, "count": {"50"}}, call: func(c *Client) error { _, e := c.Mailbox(ctx, "inbox", Page{}); return e }},
		{name: "mail", method: "GET", path: "/open/mail/inbox/2.json", call: func(c *Client) error { _, e := c.Mail(ctx, "inbox", "2"); return e }},
		{name: "send", method: "POST", path: "/open/mail/send.json", form: url.Values{"id": {"bob"}, "title": {"hi"}, "content": {"你好"}, "signature": {"0"}, "backup": {"1"}}, call: func(c *Client) error { _, e := c.SendMail(ctx, "bob", "hi", "你好"); return e }},
		{name: "reply mail", method: "POST", path: "/open/mail/inbox/reply/2.json", form: url.Values{"title": {"Re"}, "content": {"好"}, "signature": {"0"}, "backup": {"1"}}, call: func(c *Client) error { _, e := c.ReplyMail(ctx, "inbox", "2", "Re", "好"); return e }},
		{name: "forward", method: "POST", path: "/open/mail/inbox/forward/2.json", form: url.Values{"target": {"bob"}, "noansi": {"0"}, "big5": {"0"}}, call: func(c *Client) error { _, e := c.ForwardMail(ctx, "inbox", "2", "bob"); return e }},
		{name: "delete mail", method: "POST", path: "/open/mail/inbox/delete/2.json", call: func(c *Client) error { _, e := c.DeleteMail(ctx, "inbox", "2"); return e }},
		{name: "refers", method: "GET", path: "/open/refer/at.json", urlQuery: url.Values{"page": {"1"}, "count": {"50"}}, call: func(c *Client) error { _, e := c.Refers(ctx, "at", Page{}); return e }},
		{name: "refer info", method: "GET", path: "/open/refer/reply/info.json", call: func(c *Client) error { _, e := c.ReferInfo(ctx, "reply"); return e }},
		{name: "read one", method: "POST", path: "/open/refer/reply/setRead/4.json", call: func(c *Client) error { _, e := c.ReadRefer(ctx, "reply", "4"); return e }},
		{name: "read all", method: "POST", path: "/open/refer/at/setRead.json", call: func(c *Client) error { _, e := c.ReadAllRefers(ctx, "at"); return e }},
		{name: "vote", method: "GET", path: "/open/vote/42.json", call: func(c *Client) error { _, e := c.Vote(ctx, "42"); return e }},
		{name: "submit vote", method: "POST", path: "/open/vote/42.json", form: url.Values{"vote": {"2"}}, call: func(c *Client) error { _, e := c.SubmitVote(ctx, "42", "2"); return e }},
		{name: "multiple vote", method: "POST", path: "/open/vote/42.json", form: url.Values{"vid": {"8"}, "options[]": {"2", "3"}}, call: func(c *Client) error {
			_, e := c.SubmitMultipleVote(ctx, "42", url.Values{"vid": {"8"}, "options[]": {"2", "3"}})
			return e
		}},
		{name: "bet", method: "GET", path: "/open/bet/42.json", call: func(c *Client) error { _, e := c.Bet(ctx, "42"); return e }},
		{name: "submit bet", method: "POST", path: "/open/bet/42.json", form: url.Values{"biid": {"2"}, "score": {"10"}}, call: func(c *Client) error { _, e := c.SubmitBet(ctx, "42", "2", 10); return e }},
		{name: "banners", method: "GET", path: "/open/banner.json", call: func(c *Client) error { _, e := c.Banners(ctx); return e }},
		{name: "welcome public", method: "GET", path: "/open/welcomeimg.json", public: true, call: func(c *Client) error { _, e := c.Welcome(ctx); return e }},
		{name: "release public", method: "GET", path: "/open/app/release/version.json", public: true, call: func(c *Client) error { _, e := c.Version(ctx, false); return e }},
		{name: "beta public", method: "GET", path: "/open/app/beta/version.json", public: true, call: func(c *Client) error { _, e := c.Version(ctx, true); return e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called++
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Errorf("got %s %s", r.Method, r.URL.Path)
				}
				q := r.URL.Query()
				token := q.Get("oauth_token")
				if tc.public && token != "" || !tc.public && token != "test-token" {
					t.Errorf("unexpected auth %q", token)
				}
				q.Del("oauth_token")
				if len(q) != 0 || len(tc.urlQuery) != 0 {
					if !reflect.DeepEqual(q, tc.urlQuery) {
						t.Errorf("query %v, want %v", q, tc.urlQuery)
					}
				}
				if r.Method == "POST" {
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					want := tc.form
					if want == nil {
						want = url.Values{}
					}
					if !reflect.DeepEqual(r.PostForm, want) {
						t.Errorf("form %v, want %v", r.PostForm, want)
					}
				}
				io.WriteString(w, `{"code":"0000"}`)
			}))
			defer s.Close()
			c, err := New(Options{BaseURL: s.URL, Token: func(context.Context) (string, error) {
				if tc.public {
					t.Error("public endpoint read token")
				}
				return "test-token", nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			if err = tc.call(c); err != nil {
				t.Fatal(err)
			}
			if called != 1 {
				t.Fatalf("requests %d", called)
			}
		})
	}
}
func TestLoginSignatureAndWire(t *testing.T) {
	f, err := LoginForm("user", "p@ss", time.UnixMilli(1700000000600))
	if err != nil {
		t.Fatal(err)
	}
	if f.Get("source") != "1700000001-1" || f.Get("appkey") != "552c142c40b072aee5fb48b7d5f37026" {
		t.Fatal("official login signature changed")
	}
	if f.Get("username") != base64.StdEncoding.EncodeToString([]byte("user")) {
		t.Fatal("username encoding")
	}
	if _, err = LoginForm("user", "密码", time.Now()); err == nil {
		t.Fatal("non-byte code units must not be silently changed to UTF-8")
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/oauth2/official" || r.URL.Query().Has("oauth_token") {
			t.Errorf("invalid auth request %s", r.URL)
		}
		r.ParseForm()
		if r.PostForm.Get("password") != "cEBzcw==" {
			t.Error("password encoding")
		}
		io.WriteString(w, `{"access_token":"secret","expires_in":"3600"}`)
	}))
	defer s.Close()
	c, _ := New(Options{BaseURL: s.URL})
	info, err := c.Login(context.Background(), "user", "p@ss")
	if err != nil || info.AccessToken != "secret" || info.ExpiresIn != 3600 {
		t.Fatalf("%+v %v", info, err)
	}
}
func TestUploadRetainsEmptyArticleIDAndMultipart(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/attachment/Go/add/.json" {
			t.Errorf("path %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatal(err)
		}
		f, h, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		if h.Filename != "图.png" || string(b) != "image-bytes" {
			t.Errorf("multipart %s %s", h.Filename, b)
		}
		io.WriteString(w, `{"file":{"name":"图.png","url":"/api/attachment/Go/123","size":7},"remain_count":"9"}`)
	}))
	defer s.Close()
	c, _ := New(Options{BaseURL: s.URL, Token: func(context.Context) (string, error) { return "x", nil }})
	v, err := c.Upload(context.Background(), "Go", "", "图.png", strings.NewReader("image-bytes"))
	if err != nil || len(v.Files) != 1 || v.RemainCount != 9 {
		t.Fatalf("%+v %v", v, err)
	}
}
func TestRealResponseVariants(t *testing.T) {
	var a Article
	if err := json.Unmarshal([]byte(`{"id":9007199254740993,"user":{"id":"IWhisper#93","user_name":null},"is_liked":"1","like_sum":"12","attachment":{"file":{"name":"x","url":"/api/attachment/Go/2"}}}`), &a); err != nil {
		t.Fatal(err)
	}
	if a.ID != "9007199254740993" || a.Author.DisplayName() != "IWhisper#93" || !a.Author.Anonymous() || !bool(a.IsLiked) || len(a.Attachments) != 1 || len(a.Attachments[0].Files) != 1 {
		t.Fatalf("%+v", a)
	}
	var collection Collection
	if err := json.Unmarshal([]byte(`{"article":[{"gid":1241765,"bname":"WorkLife","title":"收藏","num":"21","user":{"id":"u"}}]}`), &collection); err != nil {
		t.Fatal(err)
	}
	if collection.Entries[0].ID != "1241765" || collection.Entries[0].ReplyCount != 21 {
		t.Fatal(collection)
	}
	var b Board
	if err := json.Unmarshal([]byte(`{"manager":["a",{"id":"b"}],"section":{"name":"0"},"allow_anonymous":1}`), &b); err != nil || len(b.Managers) != 2 {
		t.Fatalf("%+v %v", b, err)
	}
	// The live reaction endpoints answer with a boolean status. Loose scalar
	// typing must absorb it so a successful like is not reported as a failure.
	var r Reaction
	if err := json.Unmarshal([]byte(`{"status":true,"like_sum":3,"votedown_sum":"0"}`), &r); err != nil {
		t.Fatal(err)
	}
	if r.Status != "true" || r.LikeSum == nil || *r.LikeSum != 3 || r.VotedownSum == nil || *r.VotedownSum != 0 {
		t.Fatalf("reaction with boolean status: %+v", r)
	}
}
func TestAssetCredentialOrigin(t *testing.T) {
	c, _ := New(Options{Token: func(context.Context) (string, error) { return "private", nil }})
	for _, tc := range []struct {
		raw  string
		auth bool
	}{{"/api/attachment/Go/2", true}, {"https://bbs6.byr.cn/api/attachment/Go/2", true}, {"https://evilbyr.cn/x", false}, {"https://bbs.byr.cn.evil.example/x", false}, {"http://bbs.byr.cn/x", false}, {"https://example.org/x?oauth_token=old", false}} {
		s, err := c.AssetURL(context.Background(), tc.raw)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(s)
		if u.Query().Has("oauth_token") != tc.auth {
			t.Errorf("credential origin failed for %s", tc.raw)
		}
		if tc.auth && strings.HasPrefix(u.Path, "/api/") {
			t.Error("legacy path not adapted")
		}
	}
	for _, raw := range []string{"file:///etc/passwd", "javascript:alert(1)", "https://u:p@bbs.byr.cn/x"} {
		if _, err := c.AssetURL(context.Background(), raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}
