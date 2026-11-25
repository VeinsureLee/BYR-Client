package application

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func TestBrowseUseCasesResolveSectionsAndPreserveFavoriteTarget(t *testing.T) {
	var favoriteForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("oauth_token"); got != "fixture-token" {
			t.Errorf("browse request token = %q", got)
		}
		switch r.URL.Path {
		case "/open/section.json":
			io.WriteString(w, `{"section":[{"name":"0","description":"校园"}]}`)
		case "/open/section/0.json":
			io.WriteString(w, `{"name":"0","description":"校园","sub_section":[{"name":"1"}],"board":[{"name":"Go","description":"Go 讨论"}]}`)
		case "/open/section/1.json":
			io.WriteString(w, `{"name":"1","description":"院系","parent":"0","board":[{"name":"CS","description":"计算机学院"}]}`)
		case "/open/board/Go.json":
			io.WriteString(w, `{"name":"Go","description":"Go 讨论","article":[]}`)
		case "/open/search/board.json":
			io.WriteString(w, `{"section":[],"board":[{"name":"Go","description":"Go 讨论"}]}`)
		case "/open/favorite/0.json", "/open/favorite/add/0.json":
			if r.Method == http.MethodPost {
				if err := r.ParseForm(); err != nil {
					t.Errorf("parse favorite form: %v", err)
				}
				favoriteForm = r.PostForm
			}
			io.WriteString(w, `{"section":[],"board":[{"name":"Go"}]}`)
		default:
			t.Errorf("unexpected browse request %s %s", r.Method, r.URL.Path)
			io.WriteString(w, `{}`)
		}
	}))
	defer server.Close()
	a := newLookupTestApp(t, server.URL, &credentials{value: Credential{Username: "alice", Token: "fixture-token"}})

	roots, err := a.LoadSections(context.Background())
	if err != nil || len(roots.Sections) != 1 {
		t.Fatalf("sections = %+v error=%v", roots, err)
	}
	section, err := a.LoadSection(context.Background(), "0")
	if err != nil || len(section.Subsections) != 1 || section.Subsections[0].Description != "院系" || len(section.Subsections[0].Boards) != 1 {
		t.Fatalf("resolved section = %+v error=%v", section, err)
	}
	all, err := a.LoadAllBoards(context.Background())
	if err != nil || len(all) != 2 || all[0].Name != "Go" || all[1].Name != "CS" {
		t.Fatalf("all boards = %+v error=%v", all, err)
	}
	board, err := a.LoadBoard(context.Background(), "Go", forum.Page{Number: 1, Size: 30})
	if err != nil || board.Name != "Go" {
		t.Fatalf("board = %+v error=%v", board, err)
	}
	found, err := a.SearchBoards(context.Background(), "Go")
	if err != nil || len(found.Boards) != 1 || found.Boards[0].Name != "Go" {
		t.Fatalf("board search = %+v error=%v", found, err)
	}
	if _, err := a.LoadFavorites(context.Background(), "0"); err != nil {
		t.Fatal(err)
	}
	favorite, favoriteBoards, err := a.LoadFavoriteBoards(context.Background(), "0")
	if err != nil || len(favorite.Boards) != 1 || len(favoriteBoards) != 1 || !bool(favoriteBoards[0].IsFavorite) {
		t.Fatalf("favorite board snapshot = tree:%+v enriched:%+v error=%v", favorite, favoriteBoards, err)
	}
	if _, err := a.SetFavorite(context.Background(), " Go ", "0", true); err != nil {
		t.Fatal(err)
	}
	if favoriteForm.Get("name") != "Go" || favoriteForm.Get("dir") != "0" {
		t.Fatalf("favorite request form = %v", favoriteForm)
	}
	if _, err := a.SearchBoards(context.Background(), " "); err == nil {
		t.Fatal("blank board search was accepted")
	}
}
