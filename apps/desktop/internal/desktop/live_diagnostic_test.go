package desktop

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/VeinsureLee/BYR-Client/apps/desktop/internal/bootstrap"
	"github.com/VeinsureLee/BYR-Client/core/application"
)

// diagnoseFixture is an opt-in offline diagnostic. It never reads saved system
// credentials or contacts the public forum; every response comes from httptest.
func diagnoseFixture() error {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.URL.Query().Get("oauth_token"); got != "fixture-token" {
			http.Error(w, `{"code":"missing fixture token"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/section.json":
			_, _ = io.WriteString(w, `{"section":[{"name":"0","description":"fixture"}]}`)
		case "/open/section/0.json":
			_, _ = io.WriteString(w, `{"name":"0","description":"fixture","board":[{"name":"Go"}]}`)
		case "/open/search/article.json":
			if r.URL.Query().Get("board") != "Go" || r.URL.Query().Get("title1") != "fixture" {
				http.Error(w, `{"code":"invalid fixture query"}`, http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"article":[{"id":42,"group_id":42,"board_name":"Go","title":"fixture result"}],"pagination":{"page_all_count":1,"item_all_count":1}}`)
		default:
			_, _ = io.WriteString(w, `{}`)
		}
	}))
	defer server.Close()

	services, err := bootstrap.New(bootstrap.Config{
		BaseURL:     server.URL,
		HTTPClient:  server.Client(),
		Credentials: &testCredentials{session: application.Credential{Username: "fixture-user", Token: "fixture-token"}},
	})
	if err != nil {
		return err
	}
	if err := services.App.Restore(context.Background()); err != nil {
		return err
	}
	result, err := services.App.Search(context.Background(), application.SearchRequest{
		Keyword: "fixture", Board: "Go", Page: 1, Size: 20,
	})
	if err != nil {
		return err
	}
	if len(result.Articles) != 1 || result.Articles[0].Title != "fixture result" {
		return fmt.Errorf("offline fixture search result = %+v", result.Articles)
	}
	fmt.Printf("FIXTURE requests=%d results=%d\n", requests, len(result.Articles))
	return nil
}
