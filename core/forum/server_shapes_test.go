package forum

import (
	"encoding/json"
	"testing"
)

// Anonymized fixtures preserve types observed in the real /open responses.
// No account data, access token or actual message body is recorded here.
func TestRealUserLevelLabelsDoNotDiscardArticles(t *testing.T) {
	for _, level := range []string{`"用户"`, `"版主"`, `3`, `null`} {
		t.Run(level, func(t *testing.T) {
			var widget Widget
			body := `{"article":[{"id":42,"board_name":"Go","title":"主题","user":{"id":"fixture","level":` + level + `}}]}`
			if err := json.Unmarshal([]byte(body), &widget); err != nil {
				t.Fatal(err)
			}
			if len(widget.Articles) != 1 || widget.Articles[0].ID != "42" {
				t.Fatal("valid article was discarded")
			}
			if level == `"用户"` && widget.Articles[0].Author.Level != "用户" {
				t.Fatal("level label lost")
			}
		})
	}
}
func TestRealSectionReferencesAndBoardList(t *testing.T) {
	var s Section
	if err := json.Unmarshal([]byte(`{"name":"0","description":"本站","sub_section":["1",2,{"name":"3","description":"子分区"}],"board":[{"name":"Go","section":"0","manager":["fixture"],"allow_post":true}]}`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Boards) != 1 || len(s.Subsections) != 3 || s.Subsections[0].Name != "1" || s.Subsections[1].Name != "2" || s.Subsections[2].Description != "子分区" {
		t.Fatalf("%+v", s)
	}
}
func TestRealFavoritesAndSearchAcceptSectionArraysOrObjects(t *testing.T) {
	for _, section := range []string{`[]`, `null`, `[{"name":"0"}]`, `{"name":"0"}`} {
		body := []byte(`{"section":` + section + `,"sub_favorite":[],"board":[{"name":"Go","section":"0","manager":["fixture"]}]}`)
		var favorite Favorite
		if err := json.Unmarshal(body, &favorite); err != nil {
			t.Fatal(err)
		}
		if len(favorite.Boards) != 1 {
			t.Fatal("favorite boards lost")
		}
		var search BoardList
		if err := json.Unmarshal(body, &search); err != nil {
			t.Fatal(err)
		}
		if len(search.Boards) != 1 {
			t.Fatal("board search results lost")
		}
	}
}
