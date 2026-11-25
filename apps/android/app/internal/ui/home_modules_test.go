package ui

import (
	"fmt"
	"testing"

	"gioui.org/layout"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func homeNavigationArticles(n int) []forum.Article {
	articles := make([]forum.Article, n)
	for i := range articles {
		articles[i] = forum.Article{
			ID:       forum.Scalar(fmt.Sprintf("home-topic-%02d", i)),
			Position: forum.Number(i + 1), Board: "Go", Title: fmt.Sprintf("Home topic %02d", i),
			Content: "长帖正文", Author: forum.User{ID: "alice"},
		}
	}
	return articles
}

func homeNavigationBoards(n int) []forum.Board {
	boards := make([]forum.Board, n)
	for i := range boards {
		boards[i] = forum.Board{Name: fmt.Sprintf("board-%02d", i), Description: fmt.Sprintf("Board %02d", i)}
	}
	return boards
}

func homeNavigationSections(n int) []forum.Section {
	sections := make([]forum.Section, n)
	for i := range sections {
		sections[i] = forum.Section{Name: forum.Scalar(fmt.Sprintf("section-%02d", i)), Description: fmt.Sprintf("Section %02d", i)}
	}
	return sections
}

func TestHomeModulesKeepIndependentListPositions(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.sectionTab = "all"
	a.sections = homeNavigationSections(24)
	a.section = a.sections[0]
	a.boards = homeNavigationBoards(20)
	renderAppAt(t, a, 420, 700, 1)
	a.homeList.Position.First = 6
	a.homeList.Position.Offset = 11
	renderAppAt(t, a, 420, 700, 1)
	allPosition := a.homeList.Position

	a.navigateSectionModule("favorites")
	waitForIdle(t, a)
	a.favoriteBoards = homeNavigationBoards(24)
	renderAppAt(t, a, 420, 700, 1)
	a.homeList.Position.First = 6
	a.homeList.Position.Offset = 13
	renderAppAt(t, a, 420, 700, 1)
	favoritePosition := a.homeList.Position

	a.navigateSectionModule("top")
	waitForIdle(t, a)
	a.articles = homeNavigationArticles(30)
	renderAppAt(t, a, 420, 700, 1)
	a.topList.Position.First = 6
	a.topList.Position.Offset = 17
	renderAppAt(t, a, 420, 700, 1)
	topPosition := a.topList.Position

	a.navigateSectionModule("recommend")
	waitForIdle(t, a)
	a.articles = homeNavigationArticles(30)
	renderAppAt(t, a, 420, 700, 1)
	a.recommendList.Position.First = 7
	a.recommendList.Position.Offset = 19
	renderAppAt(t, a, 420, 700, 1)
	recommendPosition := a.recommendList.Position

	a.navigateSectionModule("timeline")
	waitForIdle(t, a)
	a.articles = homeNavigationArticles(30)
	renderAppAt(t, a, 420, 700, 1)
	a.timelineList.Position.First = 8
	a.timelineList.Position.Offset = 23
	renderAppAt(t, a, 420, 700, 1)
	timelinePosition := a.timelineList.Position

	for _, module := range []string{"favorites", "top", "recommend", "timeline", "all"} {
		a.navigateSectionModule(module)
		renderAppAt(t, a, 420, 700, 1)
		if got, rooted := a.homeModuleForStack(); !rooted || got != module {
			t.Fatalf("restored module = %q rooted=%t, want %q", got, rooted, module)
		}
		var got, want layout.Position
		switch module {
		case "favorites":
			got, want = a.homeList.Position, favoritePosition
		case "top":
			got, want = a.topList.Position, topPosition
		case "recommend":
			got, want = a.recommendList.Position, recommendPosition
		case "timeline":
			got, want = a.timelineList.Position, timelinePosition
		case "all":
			got, want = a.homeList.Position, allPosition
		}
		if got.First != want.First || got.Offset != want.Offset {
			t.Fatalf("%s position = %+v, want %+v", module, got, want)
		}
	}
}
