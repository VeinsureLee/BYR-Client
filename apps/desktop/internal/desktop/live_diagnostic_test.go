package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
	"github.com/egoist/mygo"
)

type savedReadOnlyCredential struct{ HideName bool }

func (s savedReadOnlyCredential) Load(context.Context) (application.Credential, error) {
	var c application.Credential
	b, err := mygo.Secrets.Service("cn.byr.bbs.byrclient").Get("forum-session")
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	if s.HideName {
		c.Username = "接口验证"
	}
	return c, err
}
func (savedReadOnlyCredential) Save(context.Context, application.Credential) error {
	return fmt.Errorf("read-only diagnostic")
}
func (savedReadOnlyCredential) Clear(context.Context) error {
	return fmt.Errorf("read-only diagnostic")
}

type diagnosticTransport struct{}

func (diagnosticTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != "GET" || !safeDiagnosticPath(r.URL.Path) {
		return nil, fmt.Errorf("diagnostic blocked non-read request")
	}
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(strings.NewReader(string(b)))
	var object map[string]json.RawMessage
	if json.Unmarshal(b, &object) == nil {
		keys := make([]string, 0, len(object))
		for k := range object {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Printf("LIVE path=%s status=%d bytes=%d fields=%v\n", r.URL.Path, resp.StatusCode, len(b), keys)
		for _, key := range []string{"code", "section_count", "article", "section", "board", "user", "pagination", "data"} {
			v, ok := object[key]
			if !ok {
				continue
			}
			var list []json.RawMessage
			if json.Unmarshal(v, &list) == nil {
				fmt.Printf("  %s: array length=%d\n", key, len(list))
				if len(list) > 0 {
					var item map[string]json.RawMessage
					if json.Unmarshal(list[0], &item) == nil {
						names := []string{}
						for name := range item {
							names = append(names, name)
						}
						sort.Strings(names)
						fmt.Printf("  first item keys=%v\n", names)
					}
				}
			} else if key == "code" {
				fmt.Printf("  code=%s\n", v)
			} else {
				fmt.Printf("  %s: %s\n", key, jsonKind(v))
			}
		}
	}
	return resp, nil
}

func safeDiagnosticPath(path string) bool {
	for _, prefix := range []string{"/open/section", "/open/widget/", "/open/board/", "/open/threads/", "/open/user/", "/open/favorite/0.json", "/open/search/", "/open/mail/", "/open/refer/reply.json", "/open/refer/at.json", "/open/collection.json", "/open/favorpost.json", "/open/blacklist/list.json", "/open/vote/", "/open/bet/", "/open/attachment/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	// Article reads are /open/article/<board>/<id>.json only; like, votedown
	// and delete live under the same prefix and must never be reached.
	if article := regexp.MustCompile(`^/open/article/[^/]+/[^/]+\.json$`); article.MatchString(path) {
		return true
	}
	// Reaction probes use the nonexistent article id 0 so no real article is
	// ever liked or voted down; the server answers with an error envelope.
	if probe := regexp.MustCompile(`^/open/article/[^/]+/(?:like|votedown)/0\.json$`); probe.MatchString(path) {
		return true
	}
	return false
}
func jsonKind(b []byte) string {
	if len(b) == 0 {
		return "empty"
	}
	switch b[0] {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case 'n':
		return "null"
	}
	return "scalar"
}
func diagnoseLiveSession() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := local.Open(ctx, nil)
	if err != nil {
		return err
	}
	a, err := application.New(application.Options{Local: store, Credentials: savedReadOnlyCredential{}, HTTPClient: &http.Client{Timeout: 10 * time.Second, Transport: diagnosticTransport{}}})
	if err != nil {
		return err
	}
	if err = a.Restore(ctx); err != nil {
		return fmt.Errorf("restore saved session: %w", err)
	}
	fmt.Printf("LIVE savedTokenPresent=%t expiresInFuture=%t\n", a.Session().Token != "", a.Session().ExpiresAt == 0 || a.Session().ExpiresAt > time.Now().Unix())
	sections, err := a.Forum.Sections(ctx)
	fmt.Printf("LIVE sections=%d error=%v\n", len(sections.Sections), err)
	if len(sections.Sections) > 0 {
		v, e := a.Forum.Section(ctx, string(sections.Sections[0].Name))
		fmt.Printf("LIVE section boards=%d subsections=%d error=%v\n", len(v.Boards), len(v.Subsections), e)
	}
	top, err := a.Forum.TopTen(ctx)
	fmt.Printf("LIVE topTen=%d error=%v\n", len(top.Articles), err)
	if err == nil && len(top.Articles) > 0 {
		v := top.Articles[0]
		id := string(v.GroupID)
		if id == "" {
			id = string(v.ID)
		}
		board, e := a.Forum.Board(ctx, v.Board, forum.Page{Size: 5})
		fmt.Printf("LIVE board articles=%d error=%v\n", len(board.Articles), e)
		// Thread with the app's default page size, exactly as a real page load.
		thread, e := a.Forum.Thread(ctx, v.Board, id, forum.Page{})
		fmt.Printf("LIVE thread(default page) articles=%d error=%v\n", len(thread.Articles), e)
		if e == nil {
			detail, e := a.Forum.Article(ctx, v.Board, id)
			fmt.Printf("LIVE article detail error=%v\n", e)
			_, e = a.Forum.Vote(ctx, id)
			fmt.Printf("LIVE vote error=%v\n", e)
			_, e = a.Forum.Bet(ctx, id)
			fmt.Printf("LIVE bet error=%v\n", e)
			if len(thread.Articles) > 0 {
				first := thread.Articles[0]
				if first.HasAttachment {
					_, e = a.Forum.Attachments(ctx, v.Board, string(first.ID))
					fmt.Printf("LIVE attachments error=%v\n", e)
				}
			}
			_ = detail
		}
	}
	recommend, e := a.Forum.Recommendations(ctx)
	fmt.Printf("LIVE recommend articles=%d error=%v\n", len(recommend.Articles), e)
	// Probe every top-ten thread and a sample of boards with their first
	// thread, so a shape difference in any board becomes visible.
	for _, article := range top.Articles {
		id := string(article.GroupID)
		if id == "" {
			id = string(article.ID)
		}
		_, e := a.Forum.Thread(ctx, article.Board, id, forum.Page{})
		if e != nil {
			fmt.Printf("LIVE thread %s/%s error=%v\n", article.Board, id, e)
		}
	}
	probedBoards := 0
	if len(sections.Sections) > 0 {
		sample, e := a.Forum.Section(ctx, string(sections.Sections[0].Name))
		if e == nil {
			for _, b := range sample.Boards {
				if probedBoards >= 5 {
					break
				}
				probedBoards++
				page, e := a.Forum.Board(ctx, b.Name, forum.Page{})
				fmt.Printf("LIVE board %s articles=%d error=%v\n", b.Name, len(page.Articles), e)
				if e == nil && len(page.Articles) > 0 {
					id := string(page.Articles[0].GroupID)
					if id == "" {
						id = string(page.Articles[0].ID)
					}
					_, e = a.Forum.Thread(ctx, b.Name, id, forum.Page{})
					if e != nil {
						fmt.Printf("LIVE thread %s/%s error=%v\n", b.Name, id, e)
					}
				}
			}
		}
	}
	if err == nil && len(top.Articles) > 0 {
		// Reaction endpoints with the nonexistent article id 0: the server
		// replies with its error envelope, revealing the response shape
		// without liking or voting down any real article.
		board := top.Articles[0].Board
		_, e = a.Forum.Like(ctx, board, "0")
		fmt.Printf("LIVE like(id 0) error=%v\n", e)
		_, e = a.Forum.VoteDown(ctx, board, "0")
		fmt.Printf("LIVE votedown(id 0) error=%v\n", e)
	}
	self, err := a.Forum.Self(ctx)
	fmt.Printf("LIVE profile error=%v\n", err)
	if err == nil {
		_, e := a.Forum.User(ctx, string(self.ID))
		fmt.Printf("LIVE user query error=%v\n", e)
	}
	favorite, err := a.Forum.Favorites(ctx, "0")
	fmt.Printf("LIVE favorite boards=%d error=%v\n", len(favorite.Boards), err)
	search, e := a.Forum.SearchBoards(ctx, "Go")
	fmt.Printf("LIVE board-search boards=%d sections=%d error=%v\n", len(search.Boards), len(search.Sections), e)
	articleSearch, e := a.Forum.SearchArticles(ctx, forum.Search{Title: "北邮", Page: forum.Page{Number: 1, Size: 20}})
	fmt.Printf("LIVE article-search results=%d error=%v\n", len(articleSearch.Articles), e)
	for _, box := range []string{"inbox", "outbox"} {
		v, e := a.Forum.Mailbox(ctx, box, forum.Page{Size: 5})
		fmt.Printf("LIVE mailbox headers=%d error=%v\n", len(v.Mails), e)
		if e == nil && len(v.Mails) > 0 {
			_, e = a.Forum.Mail(ctx, box, string(v.Mails[0].Index))
			fmt.Printf("LIVE mail detail error=%v\n", e)
		}
	}
	for _, kind := range []string{"reply", "at"} {
		v, e := a.Forum.Refers(ctx, kind, forum.Page{Size: 5})
		fmt.Printf("LIVE notification headers=%d error=%v\n", len(v.Articles), e)
	}
	collection, e := a.Forum.Collection(ctx, "", forum.Page{Size: 5})
	fmt.Printf("LIVE collection entries=%d error=%v\n", len(collection.Entries), e)
	posts, e := a.Forum.FavorPosts(ctx, forum.Page{Size: 5})
	fmt.Printf("LIVE followed posts=%d error=%v\n", len(posts.Articles), e)
	blacklist, e := a.Forum.Blacklist(ctx, 1)
	fmt.Printf("LIVE blacklist count=%d error=%v\n", len(blacklist.Users), e)
	return nil
}
