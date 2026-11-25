package byrhttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/internal/byrhttp"
)

func token(context.Context) (string, error) { return "test-token", nil }

func TestReadBoardProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/open/board/Joke.json" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("oauth_token") != "test-token" || r.URL.Query().Get("page") != "2" || r.URL.Query().Get("count") != "30" {
			t.Error("missing token or pagination")
		}
		fmt.Fprint(w, `{"name":"Joke","description":"笑话","article":[{"id":123,"title":"主题","post_time":1794497375,"user":{"id":"alice"}}],"pagination":{"page_all_count":4,"page_current_count":2}}`)
	}))
	defer server.Close()
	client, err := byrhttp.New(byrhttp.Options{BaseURL: server.URL, Token: token})
	if err != nil {
		t.Fatal(err)
	}
	board, err := client.Board(context.Background(), "Joke", forum.Page{Number: 2, Size: 30})
	if err != nil {
		t.Fatal(err)
	}
	if board.Description != "笑话" || board.Pagination.TotalPages != 4 || len(board.Articles) != 1 || board.Articles[0].ID != "123" || board.Articles[0].PostTime != "1794497375" {
		t.Fatalf("unexpected board: %+v", board)
	}
}

func TestReadResources(t *testing.T) {
	responses := map[string]string{
		"/prefix/open/section.json":         `{"section":[{"name":0,"description":"本站","board":[]}]}`,
		"/prefix/open/section/0.json":       `{"name":"0","board":[{"name":"Joke"}]}`,
		"/prefix/open/threads/Joke/12.json": `{"id":"12","title":"主题","article":[{"id":"13","content":"回复"}]}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, body)
	}))
	defer server.Close()
	client, err := byrhttp.New(byrhttp.Options{BaseURL: server.URL + "/prefix", Token: token})
	if err != nil {
		t.Fatal(err)
	}
	sections, err := client.Sections(context.Background())
	if err != nil || len(sections.Sections) != 1 || sections.Sections[0].Name != "0" {
		t.Fatalf("sections: %+v, %v", sections, err)
	}
	section, err := client.Section(context.Background(), "0")
	if err != nil || len(section.Boards) != 1 {
		t.Fatalf("section: %+v, %v", section, err)
	}
	thread, err := client.Thread(context.Background(), "Joke", "12", forum.Page{})
	if err != nil || len(thread.Articles) != 1 || thread.Articles[0].Content != "回复" {
		t.Fatalf("thread: %+v, %v", thread, err)
	}
}

func TestNoRequestsDuringConstructionOrWithoutToken(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer server.Close()
	client, err := byrhttp.New(byrhttp.Options{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Fatal("constructor performed a network request")
	}
	if _, err := client.Sections(context.Background()); !errors.Is(err, forum.ErrUnauthenticated) {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Fatal("unauthenticated request reached the server")
	}
}

func TestRejectInvalidInput(t *testing.T) {
	for _, base := range []string{"file:///tmp/data", "https://user:password@example.com", "https://example.com?token=secret", ":bad"} {
		if _, err := byrhttp.New(byrhttp.Options{BaseURL: base}); err == nil {
			t.Errorf("accepted base URL %q", base)
		}
	}
	client, _ := byrhttp.New(byrhttp.Options{Token: token})
	for _, board := range []string{"", "../other", "..", "Joke?query", "Joke%2Fother"} {
		if _, err := client.Board(context.Background(), board, forum.Page{}); err == nil {
			t.Errorf("accepted board %q", board)
		}
	}
	if _, err := client.Board(context.Background(), "Joke", forum.Page{Number: -1}); err == nil {
		t.Fatal("accepted negative page")
	}
	if _, err := client.Board(context.Background(), "Joke", forum.Page{Size: 101}); err == nil {
		t.Fatal("accepted invalid count")
	}
}

func TestProtocolFailures(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
		check      func(error) bool
	}{
		{"API string code", `{"code":"0010","msg":"未登录"}`, 200, func(err error) bool { var api *forum.APIError; return errors.As(err, &api) && api.Code == "0010" }},
		{"API numeric code", `{"code":65,"error":"invalid credentials"}`, 200, func(err error) bool { var api *forum.APIError; return errors.As(err, &api) && api.Code == "65" }},
		{"HTTP with API envelope", `{"code":"1603","msg":"已经赞过"}`, 400, func(err error) bool {
			var api *forum.APIError
			return errors.As(err, &api) && api.Code == "1603" && api.Message == "已经赞过"
		}},
		{"HTTP", "private server details", 503, func(err error) bool {
			var status *forum.HTTPError
			return errors.As(err, &status) && status.StatusCode == 503 && !strings.Contains(err.Error(), "private")
		}},
		{"empty", " ", 200, func(err error) bool { return errors.Is(err, forum.ErrEmptyResponse) }},
		{"null", "null", 200, func(err error) bool { return errors.Is(err, forum.ErrEmptyResponse) }},
		{"malformed", "<html>not JSON</html>", 200, func(err error) bool { return err != nil }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(test.status); fmt.Fprint(w, test.body) }))
			defer server.Close()
			client, _ := byrhttp.New(byrhttp.Options{BaseURL: server.URL, Token: token})
			_, err := client.Sections(context.Background())
			if !test.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCancelAndRedactTransportErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("cancelled request reached server") }))
	client, _ := byrhttp.New(byrhttp.Options{BaseURL: server.URL, Token: token})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Sections(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	server.Close()
	if _, err := client.Sections(context.Background()); err == nil || strings.Contains(err.Error(), "test-token") || strings.Contains(err.Error(), "oauth_token") {
		t.Fatalf("invalid transport error: %v", err)
	}
}

func TestRedirectCannotSendTokenToAnotherOrigin(t *testing.T) {
	var leaked atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1) }))
	defer other.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/?oauth_token=test-token", http.StatusFound)
	}))
	defer source.Close()
	client, _ := byrhttp.New(byrhttp.Options{BaseURL: source.URL, Token: token})
	_, err := client.Sections(context.Background())
	if !errors.Is(err, forum.ErrRedirectOrigin) || strings.Contains(err.Error(), "test-token") {
		t.Fatalf("unexpected redirect error: %v", err)
	}
	if leaked.Load() != 0 {
		t.Fatal("redirect reached another origin")
	}
}

func TestClientsKeepCredentialsSeparate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"section":[{"name":%q}]}`, r.URL.Query().Get("oauth_token"))
	}))
	defer server.Close()
	for _, credential := range []string{"first", "second"} {
		client, _ := byrhttp.New(byrhttp.Options{BaseURL: server.URL, Token: func(context.Context) (string, error) { return credential, nil }})
		data, err := client.Sections(context.Background())
		if err != nil || data.Sections[0].Name.String() != credential {
			t.Fatalf("client credential leaked: %+v, %v", data, err)
		}
	}
}

func TestScalarTypes(t *testing.T) {
	for _, test := range []struct{ raw, want string }{{`"alice"`, "alice"}, {"123", "123"}, {"null", ""}, {"true", "true"}, {"false", "false"}} {
		var value forum.Scalar
		if err := json.Unmarshal([]byte(test.raw), &value); err != nil || value.String() != test.want {
			t.Fatalf("%s -> %q, %v", test.raw, value, err)
		}
	}
}
