package application

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/local"
)

func TestFetchAssetUsesSharedTrustedTransportAndSession(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/open/attachment/Go/99" {
			t.Errorf("asset path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("oauth_token"); got != "fixture-token" {
			t.Errorf("asset token = %q", got)
		}
		io.WriteString(w, "asset bytes")
	}))
	defer server.Close()
	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(Options{Local: store, Credentials: &credentials{value: Credential{Username: "alice", Token: "fixture-token"}}, ForumFactory: testForumFactory(server.URL, server.Client())})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := a.FetchAsset(context.Background(), "/api/attachment/Go/99")
	if err != nil || string(data) != "asset bytes" || calls != 1 {
		t.Fatalf("asset result=%q calls=%d error=%v", data, calls, err)
	}
	if _, err := a.FetchAsset(context.Background(), " "); err == nil {
		t.Fatal("blank asset URL was accepted")
	}
	if calls != 1 {
		t.Fatalf("blank asset URL issued a request, calls=%d", calls)
	}
}
