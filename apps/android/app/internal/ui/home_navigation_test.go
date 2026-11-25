package ui

import (
	"context"
	"fmt"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func TestHomeTabSwitchRestoresNestedSectionBoardPageAndTopicAnchor(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()

	articles := make(forum.List[forum.Article], 20)
	for i := range articles {
		id := fmt.Sprintf("topic-%02d", i)
		if i == 5 {
			id = "42"
		}
		articles[i] = forum.Article{
			ID: forum.Scalar(id), Position: forum.Number(i + 1),
			Title: fmt.Sprintf("Topic %02d", i), Content: "body", Author: forum.User{ID: "alice"},
		}
	}
	board := forum.Board{
		Name: "Go", Description: "Go 讨论", Articles: articles,
		Pagination: forum.Pagination{CurrentPage: 2, TotalPages: 4, TotalItems: 120},
	}
	a.sections = []forum.Section{{Name: "0", Description: "校园", Subsections: []forum.Section{{Name: "1", Description: "院系"}}}}
	a.section = forum.Section{Name: "1", Description: "院系", Parent: "0", Boards: []forum.Board{board}}
	a.boards = []forum.Board{board}
	a.board = board
	a.sectionTab = "all"
	a.stack = []Route{
		{Page: PageHome},
		{Page: PageSection, Name: "1"},
		{Page: PageBoard, Name: "Go", Number: 2},
	}
	renderAppAt(t, a, 420, 700, 1)
	a.homeList.Position.First = 2
	a.homeList.Position.Offset = 11
	a.sectionList.Position.First = 2
	a.sectionList.Position.Offset = 9
	key := listKey{Page: PageBoard, Name: "Go", Number: 2}
	state := a.mobileList(key)
	state.Position.First = 5
	state.Position.Offset = 17
	renderAppAt(t, a, 420, 700, 1)
	if got := string(a.board.Articles[state.Position.First].ID); got != "42" {
		t.Fatalf("board anchor fixture = %q, want topic ID 42", got)
	}

	a.setTab(PageSearch)
	renderAppAt(t, a, 420, 700, 1)
	a.openThread("Go", "42")
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	a.setTab(PageMessages)
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	a.stack[len(a.stack)-1].Sub = "reply"
	a.openThread("Go", "42")
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	a.setTab(PageSelf)
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	a.setTab(PageHome)
	renderAppAt(t, a, 420, 700, 1)

	if len(a.stack) != 3 || a.stack[1] != (Route{Page: PageSection, Name: "1"}) ||
		a.stack[2] != (Route{Page: PageBoard, Name: "Go", Number: 2}) {
		t.Fatalf("restored Home route stack = %+v", a.stack)
	}
	if module, rooted := a.homeModuleForStack(); !rooted || module != "all" || a.sectionTab != "all" {
		t.Fatalf("restored module = %q rooted=%t selected=%q", module, rooted, a.sectionTab)
	}
	if a.current().Number != 2 || a.homeList.Position.First != 2 || a.homeList.Position.Offset != 11 ||
		a.sectionList.Position.First != 2 || a.sectionList.Position.Offset != 9 {
		t.Fatalf("restored route/list state = %+v home=%+v section=%+v", a.current(), a.homeList.Position, a.sectionList.Position)
	}
	state = a.mobileList(key)
	if state.Position.First != 5 || state.Position.Offset != 17 ||
		string(a.board.Articles[state.Position.First].ID) != "42" {
		t.Fatalf("restored board anchor = row %d offset %d article %q",
			state.Position.First, state.Position.Offset, a.board.Articles[state.Position.First].ID)
	}
	if a.renderPanic != "" {
		t.Fatalf("restored Home rendered panic: %s", a.renderPanic)
	}
}

func TestHomeSnapshotIsRejectedAfterSessionGenerationChanges(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.sectionTab = "all"
	a.saveHomeSnapshot("all", true)
	if a.homeSnapshots["all"] == nil {
		t.Fatal("Home navigation snapshot was not stored")
	}
	if err := a.core.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a.restoreHomeSnapshot("all") {
		t.Fatal("snapshot from the previous session generation was restored")
	}
	if _, ok := a.homeSnapshots["all"]; ok {
		t.Fatal("stale account snapshot was not discarded")
	}
}

func TestHomeTabSwitchRestoresRecommendTimelineAndFavoritesModules(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()

	a.articles = make([]forum.Article, 20)
	for i := range a.articles {
		a.articles[i] = forum.Article{ID: forum.Scalar(fmt.Sprintf("rec-%d", i)), Title: fmt.Sprintf("推荐主题 %d", i)}
	}
	a.stack = []Route{{Page: PageTop, Name: "recommend", Number: 1}}
	a.topTab = "recommend"
	renderAppAt(t, a, 420, 700, 1)
	a.recommendList.Position.First = 10
	a.recommendList.Position.Offset = 6
	renderAppAt(t, a, 420, 700, 1)

	a.setTab(PageSearch)
	renderAppAt(t, a, 420, 700, 1)
	a.setTab(PageHome)
	renderAppAt(t, a, 420, 700, 1)
	if a.current().Page != PageTop || a.current().Name != "recommend" || a.topTab != "recommend" ||
		a.recommendList.Position.First != 10 || a.recommendList.Position.Offset != 6 {
		t.Fatalf("recommend restore = %+v topTab=%q pos=%+v", a.current(), a.topTab, a.recommendList.Position)
	}

	a.navigateSectionModule("timeline")
	waitForIdle(t, a)
	a.articles = make([]forum.Article, 20)
	for i := range a.articles {
		a.articles[i] = forum.Article{ID: forum.Scalar(fmt.Sprintf("tl-%d", i)), Title: fmt.Sprintf("时间线主题 %d", i)}
	}
	a.pagination = forum.Pagination{CurrentPage: 1, TotalPages: 3, TotalItems: 30}
	a.timelineList.Position.First = 10
	a.timelineList.Position.Offset = 4
	renderAppAt(t, a, 420, 700, 1)
	a.setTab(PageMessages)
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	a.setTab(PageHome)
	renderAppAt(t, a, 420, 700, 1)
	if a.current().Page != PageTimeline || a.timelineList.Position.First != 10 || a.timelineList.Position.Offset != 4 {
		t.Fatalf("timeline restore = %+v pos=%+v", a.current(), a.timelineList.Position)
	}

	a.navigateSectionModule("favorites")
	waitForIdle(t, a)
	a.favoriteBoards = make([]forum.Board, 20)
	for i := range a.favoriteBoards {
		a.favoriteBoards[i] = forum.Board{Name: fmt.Sprintf("B%02d", i), Description: fmt.Sprintf("版面 %d", i)}
	}
	a.favorite = forum.Favorite{Boards: a.favoriteBoards}
	a.favoriteList.Position.First = 10
	a.favoriteList.Position.Offset = 9
	renderAppAt(t, a, 420, 700, 1)
	a.setTab(PageSearch)
	renderAppAt(t, a, 420, 700, 1)
	a.setTab(PageHome)
	renderAppAt(t, a, 420, 700, 1)
	if a.current().Page != PageHome || a.sectionTab != "favorites" ||
		len(a.favoriteBoards) != 20 || a.favoriteBoards[0].Name != "B00" || a.favoriteList.Position.First != 10 {
		t.Fatalf("favorites restore = %+v tab=%q boards=%+v", a.current(), a.sectionTab, a.favoriteBoards)
	}
}
