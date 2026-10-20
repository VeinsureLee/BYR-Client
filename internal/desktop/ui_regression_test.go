package desktop

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	svc "BYR-Client/internal/app"
	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
)

func TestChildDirectoriesShowLatestTopics(t *testing.T) {
	for _, id := range []forum.NodeID{"f-tech", "s-prog"} {
		t.Run(string(id), func(t *testing.T) {
			u, application := newTestUI(t)
			children, err := application.Children(id)
			if err != nil {
				t.Fatal(err)
			}
			for _, child := range children {
				if err := application.LoadLatestTopic(context.Background(), child.ID); err != nil {
					t.Fatal(err)
				}
			}
			u.browseNode = id
			tester := ui.NewTester(u.view, 1320, 860)
			for _, child := range children {
				latest, _, ok := application.LatestTopic(child.ID)
				if !ok || !tester.HasText(latest.Title) {
					t.Fatalf("child %s has no latest topic in UI: %v", child.ID, tester.Texts())
				}
			}
			if tester.HasText("主题列表按需") {
				t.Fatal("old placeholder is still shown")
			}
		})
	}
}

func TestHistoryRestoresIndependentReadingPositions(t *testing.T) {
	u, _ := newTestUI(t)
	a := svc.HistorySnapshot{ID: "a", Mode: svc.ModeKeyword}
	b := svc.HistorySnapshot{ID: "b", Mode: svc.ModeAI}
	u.restoreHistory(a)
	u.resultScroll.Y = 320
	u.collapsed["board:Go"] = true
	u.restoreHistory(b)
	if u.aiScroll.Y != 0 || u.resultScroll.Y != 320 || !u.collapsed["board:Go"] {
		t.Fatal("switching to AI lost the keyword reading state")
	}
	u.aiScroll.Y = 180
	u.restoreHistory(a)
	if u.resultScroll.Y != 320 || !u.collapsed["board:Go"] {
		t.Fatalf("keyword reading state not restored: y=%v collapsed=%v", u.resultScroll.Y, u.collapsed)
	}
	u.restoreHistory(b)
	if u.aiScroll.Y != 180 || u.region != regionSearch {
		t.Fatalf("AI history state not restored: y=%v region=%v", u.aiScroll.Y, u.region)
	}
	u.resetWorkspace()
	if len(u.searchViews) != 0 || len(u.readingViews) != 0 || len(u.boardViews) != 0 {
		t.Fatal("reading states survived workspace reset")
	}
}

func TestReplyTreeUsesIconDisclosure(t *testing.T) {
	u, _ := newTestUI(t)
	key := domain.ArticleKey{BoardID: "Go", ArticleID: "100"}
	parent := domain.Reply{Key: domain.ReplyKey{ArticleKey: key, ReplyID: "r2"}, Floor: 2, Author: "alice", Content: "父回复"}
	child := domain.Reply{Key: domain.ReplyKey{ArticleKey: key, ReplyID: "r3"}, Floor: 3, Author: "bob", Content: "子回复"}
	node := &replyNode{Reply: parent, Children: []*replyNode{{Reply: child, ParentFloor: 2}}}
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { u.replyTree(c, node, 0, 0) })
	}, 700, 600)
	if !tester.HasText("父回复") || !tester.HasText("子回复") || tester.HasText("楼中楼") {
		t.Fatalf("unexpected reply presentation: %v", tester.Texts())
	}
	if err := tester.Click("折叠或展开 #2 的回复"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(250 * time.Millisecond)
	if tester.HasText("子回复") || !tester.HasText("父回复") {
		t.Fatal("disclosure did not hide only the child replies")
	}
	if err := tester.Click("折叠或展开 #2 的回复"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(250 * time.Millisecond)
	if !tester.HasText("子回复") {
		t.Fatal("child replies did not reopen")
	}
}

func TestSearchModesKeepRecordsAndNewDraftsIndependent(t *testing.T) {
	u, application := newTestUI(t)
	u.region = regionSearch
	u.activeHistory, u.keywordHistory, u.activeRun = "keyword-record", "keyword-record", "keyword-record"
	u.keywordDraft = "未提交的关键词"
	u.aiHistoryID = application.AddAIHistory("AI 记录", domain.SearchQuery{})
	u.aiQuestion = "未提交的问题"
	u.chat = []chatTurn{{ID: 1, Role: "assistant", Text: "保留的回答"}}
	u.resultScroll.Y, u.aiScroll.Y = 125, 245
	u.collapsed["board:Go"] = true
	tester := ui.NewTester(u.view, 1320, 860)
	if err := tester.Click("AI 问答"); err != nil {
		t.Fatal(err)
	}
	if u.activeHistory != u.aiHistoryID || u.activeRun != "keyword-record" || u.keywordDraft != "未提交的关键词" {
		t.Fatal("mode switch lost the keyword record or draft")
	}
	u.newSearch()
	if u.aiHistoryID != "" || len(u.chat) != 0 || u.keywordHistory != "keyword-record" || u.keywordDraft != "未提交的关键词" {
		t.Fatal("new AI draft cleared keyword state")
	}
	tester.Frame()
	if err := tester.Click("关键词检索"); err != nil {
		t.Fatal(err)
	}
	if u.activeHistory != "keyword-record" || u.activeRun != "keyword-record" || !u.collapsed["board:Go"] {
		t.Fatal("return to keyword mode lost its selected record")
	}
	u.chat = []chatTurn{{ID: 1, Role: "assistant", Text: "另一份回答"}}
	u.aiQuestion = "另一份草稿"
	u.newSearch()
	if u.activeRun != "" || u.keywordDraft != "" || len(u.chat) != 1 || u.aiQuestion != "另一份草稿" {
		t.Fatal("new keyword draft cleared AI state")
	}
}

func TestImagePlaceholderPreservesAspectRatio(t *testing.T) {
	for _, image := range []domain.Image{{Width: 1600, Height: 900}, {Width: 600, Height: 1200}, {Width: 20, Height: 20}} {
		w, h := imagePlaceholderSize(image, 560, 420)
		if w > 560 || h > 420 || w > float32(image.Width) || h > float32(image.Height) {
			t.Fatalf("placeholder exceeds limits: %v x %v for %+v", w, h, image)
		}
		if w*float32(image.Height) != h*float32(image.Width) {
			t.Fatalf("aspect ratio changed: %v x %v for %+v", w, h, image)
		}
	}
}

func TestAIComposerFitsWindowAndLargeFonts(t *testing.T) {
	for _, scale := range []float32{1, 1.15, 1.3} {
		for _, width := range []int{980, 1320} {
			t.Run(fmt.Sprintf("%d-font-%g", width, scale), func(t *testing.T) {
				u, _ := newTestUI(t)
				u.region, u.searchMode, u.fontScale = regionSearch, 1, scale
				u.aiTimeKind = "自定义"
				u.aiDetails = true
				tester := ui.NewTester(u.view, width, 640)
				tester.SetScale(1.5)
				for _, label := range []string{"发送", "AI 问题", "时间范围"} {
					r, ok := tester.Find(label)
					if !ok || r.X < sidebarWidth || r.X+r.W > float32(width)+1 || r.Y < 0 || r.Y+r.H > 641 {
						t.Fatalf("%q outside usable window: %+v, found=%t", label, r, ok)
					}
				}
			})
		}
	}
}

func TestReadingLayoutAndPositions(t *testing.T) {
	u, application := newTestUI(t)
	if err := application.OpenBoard(context.Background(), "b-go"); err != nil {
		t.Fatal(err)
	}
	board, _ := application.Board("b-go")
	first, second := board.Articles[1].Key, board.Articles[2].Key
	for _, key := range []domain.ArticleKey{first, second} {
		if err := application.OpenThread(context.Background(), key); err != nil {
			t.Fatal(err)
		}
	}
	u.browseNode, u.browseBoard, u.browseThread = "b-go", "b-go", first
	u.boardView("b-go").thread = first
	tester := ui.NewTester(u.view, 1320, 860)
	position := u.readingView(first, false)
	position.Y = 140
	tester.Frame()
	y := position.Y
	if y <= 0 {
		t.Fatal("test thread is not scrollable")
	}
	u.browseThread = second
	tester.Frame()
	if u.readingView(second, false).Y != 0 {
		t.Fatal("a new thread inherited the previous thread's scroll")
	}
	u.browseThread = first
	tester.Frame()
	if position.Y != y {
		t.Fatalf("thread scroll lost: got %v want %v", position.Y, y)
	}
	if err := tester.Click("更多"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(tester.Menu(), "|") != "应用内打开|系统浏览器|-|导出 PNG" {
		t.Fatalf("article actions are missing: %v", tester.Menu())
	}
	if err := tester.ChooseMenuItem("系统浏览器"); err != nil {
		t.Fatal(err)
	}
	if urls := tester.OpenedURLs(); len(urls) != 1 || urls[0] != articleLink(first) {
		t.Fatalf("menu opened wrong URL: %v", urls)
	}
	tester.SetSize(980, 640)
	if !tester.HasText("主题列表") {
		t.Fatalf("narrow reading layout has no way back to list: %v", tester.Texts())
	}
	if err := tester.Click("主题列表"); err != nil {
		t.Fatal(err)
	}
	if !u.listOnly || !tester.HasText(board.Articles[0].Title) {
		t.Fatal("narrow reading layout did not return to list")
	}
}

// Optional native-render previews, produced from the same views used by the app.
// BYR_UI_PREVIEW_DIR is only an output directory; no credentials or live API are used.
func TestUIPreviewSnapshots(t *testing.T) {
	dir := os.Getenv("BYR_UI_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set BYR_UI_PREVIEW_DIR to render visual previews")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"login", "overview", "directory", "reading", "ai", "settings"} {
		for _, dark := range []bool{false, true} {
			for _, width := range []int{1320, 980} {
				name := fmt.Sprintf("%s-%d-dark-%t", mode, width, dark)
				t.Run(name, func(t *testing.T) {
					u, application := newTestUI(t)
					u.themeMode = 1
					if dark {
						u.themeMode = 2
					}
					roots, _ := application.Children("")
					for _, node := range roots {
						_ = application.LoadLatestTopic(context.Background(), node.ID)
					}
					_ = application.LoadTraffic(context.Background(), "")
					switch mode {
					case "login":
						u.app = nil
						u.recordLoginFailure(fmt.Errorf("请勿频繁登录"), time.Now())
					case "directory":
						u.browseNode = "s-prog"
						children, _ := application.Children(u.browseNode)
						for _, node := range children {
							_ = application.LoadLatestTopic(context.Background(), node.ID)
						}
					case "reading":
						_ = application.OpenBoard(context.Background(), "b-go")
						board, _ := application.Board("b-go")
						u.browseNode, u.browseBoard, u.browseThread = "b-go", "b-go", board.Articles[1].Key
						_ = application.OpenThread(context.Background(), u.browseThread)
					case "ai":
						u.region, u.searchMode = regionSearch, 1
						u.aiKeySaved = true
						u.aiQuestion = "继续找北京的岗位"
						u.chat = []chatTurn{
							{ID: 1, Role: "user", Text: "最近有哪些 Go 实习帖？"},
							{ID: 2, Role: "assistant", Text: "找到了两个相关主题，可以先看岗位要求，再比较实习地点。\n\n### 值得关注的讨论\n\n1. **Go 后端实习内推**：关注技术栈和到岗时间。\n2. **秋招面经**：了解面试题目与准备方向。", Tools: []toolTrace{{ID: "t1", Name: "search_forum", Summary: "搜索到 2 个主题"}}, Citations: []svc.Citation{{Board: "Go", Title: "Go 后端实习内推", URL: "https://bbs.byr.cn/article/Go/1002"}}},
						}
					case "settings":
						u.region = regionSettings
					}
					tester := ui.NewTester(u.view, width, 860)
					for _, text := range tester.Texts() {
						if strings.Contains(text, "楼中楼") {
							t.Fatalf("old reply terminology: %s", text)
						}
					}
					file, err := os.Create(filepath.Join(dir, name+".png"))
					if err != nil {
						t.Fatal(err)
					}
					defer file.Close()
					if err := png.Encode(file, tester.Image()); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
