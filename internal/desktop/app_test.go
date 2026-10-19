package desktop

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	svc "BYR-Client/internal/app"
	"BYR-Client/internal/domain"
	"BYR-Client/internal/ports"
	"BYR-Client/internal/source/demo"
	"BYR-Client/internal/store/memory"
)

func newTestUI(t *testing.T) (*UI, *svc.App) {
	t.Helper()
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	application := svc.New(demo.New(now), memory.New(), ports.SystemClock{}, 10*time.Minute)
	if err := application.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(application.Close)
	return New(application), application
}

func TestViewsBuildInTester(t *testing.T) {
	u, application := newTestUI(t)
	tester := ui.NewTester(u.view, 1280, 860)
	if !tester.HasText("浏览论坛") {
		t.Fatalf("browse view missing: %v", tester.Texts())
	}

	if err := application.OpenBoard(context.Background(), "b-go"); err != nil {
		t.Fatal(err)
	}
	board, _ := application.Board("b-go")
	u.browseNode = "b-go"
	u.browseBoard = "b-go"
	tester.Frame()
	if !tester.HasText(board.Articles[0].Title) {
		t.Fatalf("board article missing: %v", tester.Texts())
	}

	key := board.Articles[0].Key
	if err := application.OpenThread(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	thread, _ := application.Thread(key)
	u.browseThread = key
	tester.Frame()
	if !tester.HasText(thread.Article.Title) {
		t.Fatalf("thread title missing: %v", tester.Texts())
	}

	u.region = regionSearch
	u.keywordTokens = []string{"Go"}
	tester.Frame()
	if !tester.HasText("关键词检索") {
		t.Fatalf("keyword search view missing: %v", tester.Texts())
	}

	id, err := application.StartSearch(domain.SearchQuery{
		Keywords:    []string{"Go"},
		Time:        domain.TimeRange{Kind: domain.TimeAll},
		PerBoard:    2,
		Total:       3,
		ThreadPages: 2,
		Concurrency: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		run, ok := application.Run(id)
		if ok && run.Status != svc.StatusQueued && run.Status != svc.StatusRunning {
			u.activeRun = id
			u.activeHistory = id
			u.run = run
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("search did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	tester.Frame()
	if len(u.run.Boards) == 0 {
		t.Fatal("search produced no board result")
	}

	u.region = regionSettings
	tester.Frame()
	if !tester.HasText("外观") || !tester.HasText("关于应用") {
		t.Fatalf("settings view missing: %v", tester.Texts())
	}
}

func TestKeywordDraftSplitsOnSpacesAndDeduplicates(t *testing.T) {
	u, _ := newTestUI(t)
	u.keywordDraft = "Go 实习 "
	if !u.splitKeywordDraft() {
		t.Fatal("draft was not split")
	}
	if len(u.keywordTokens) != 2 || u.keywordTokens[0] != "Go" || u.keywordTokens[1] != "实习" {
		t.Fatalf("tokens = %#v", u.keywordTokens)
	}
	if u.keywordDraft != "" {
		t.Fatalf("draft = %q, want empty", u.keywordDraft)
	}
	u.keywordDraft = "go 实"
	u.splitKeywordDraft()
	if len(u.keywordTokens) != 2 {
		t.Fatalf("duplicate token added: %#v", u.keywordTokens)
	}
	if u.keywordDraft != "实" {
		t.Fatalf("partial word = %q, want 实", u.keywordDraft)
	}
	u.addKeywordTokens([]string{" GO ", "秋招", "秋招"})
	if len(u.keywordTokens) != 3 {
		t.Fatalf("tokens after add = %#v", u.keywordTokens)
	}
}

func TestChooserLoginViewAndDemoMode(t *testing.T) {
	u := NewChooser()
	t.Cleanup(u.closeApp)
	tester := ui.NewTester(u.view, 1280, 860)
	if !tester.HasText("登录北邮人论坛") || !tester.HasText("先使用演示数据") {
		t.Fatalf("login view missing: %v", tester.Texts())
	}
	u.startDemo()
	tester.Frame()
	if u.app == nil {
		t.Fatal("demo mode did not create an application")
	}
	if !tester.HasText("浏览论坛") {
		t.Fatalf("demo workspace missing: %v", tester.Texts())
	}
}

func TestSettingsPersistThemeSplitAndRemember(t *testing.T) {
	t.Setenv("BYR_CLIENT_HOME", t.TempDir())
	u := NewChooser()
	u.themeMode = 2
	u.accentMode = 1
	u.fontScaleMode = 2
	u.fontScale = fontScaleFor(2)
	u.splitWidth = 420
	u.splitHeight = 360
	u.rememberLogin = false
	u.rememberPassword = false
	u.settings.LastUser = "alice"
	u.saveSettings()

	loaded := NewChooser()
	if loaded.themeMode != 2 || loaded.accentMode != 1 || loaded.fontScaleMode != 2 || loaded.splitWidth != 420 || loaded.splitHeight != 360 {
		t.Fatalf("loaded layout = theme %d accent %d font %d split %v/%v", loaded.themeMode, loaded.accentMode, loaded.fontScaleMode, loaded.splitWidth, loaded.splitHeight)
	}
	if loaded.rememberLogin || loaded.rememberPassword {
		t.Fatal("remember settings were not persisted")
	}
	if loaded.loginUser != "alice" {
		t.Fatalf("last user = %q", loaded.loginUser)
	}
}

func TestTrafficStatsRenderForDirectoryNode(t *testing.T) {
	u, application := newTestUI(t)
	if err := application.LoadTraffic(context.Background(), "f-tech"); err != nil {
		t.Fatal(err)
	}
	u.resetWorkspace()
	u.browseNode = "f-tech"
	u.region = regionBrowse
	tester := ui.NewTester(u.view, 1320, 860)
	if !tester.HasText("今日发帖") || !tester.HasText("在线") {
		t.Fatalf("traffic labels missing: %v", tester.Texts())
	}
	if !tester.HasText("编程") || !tester.HasText("人工智能") {
		t.Fatalf("child rows missing: %v", tester.Texts())
	}
}

func TestArticleShareLinks(t *testing.T) {
	t.Setenv("BYR_BASE_URL", "https://example.com/byr/")
	key := domain.ArticleKey{SiteID: "byr", BoardID: "Go", ArticleID: "1002"}
	if got := articleLink(key); got != "https://example.com/byr/article/Go/1002" {
		t.Fatalf("article link = %q", got)
	}
	if got := articleFloorLink(key, 3); got != "https://example.com/byr/article/Go/1002#a2" {
		t.Fatalf("floor link = %q", got)
	}
}

func TestDirectoryClickLeavesUtilityPage(t *testing.T) {
	u, _ := newTestUI(t)
	u.region = regionSettings
	u.returnValid = true
	u.returnRegion = regionBrowse
	u.selectNode("f-tech")
	if u.region != regionBrowse || u.browseNode != "f-tech" {
		t.Fatalf("region=%v node=%q", u.region, u.browseNode)
	}
	if u.returnValid {
		t.Fatal("utility return state survived directory navigation")
	}
}

func TestAIConfigNormalizesProviderDefaults(t *testing.T) {
	u, _ := newTestUI(t)
	u.aiProvider = "Kimi"
	u.aiModel = ""
	u.aiBaseURL = ""
	u.aiKeyInput = "sk-test"
	config, ok := u.aiConfig()
	if !ok {
		t.Fatal("ai config not available")
	}
	if string(config.Provider) != "kimi" || config.Model != "moonshot-v1-8k" || config.BaseURL != "https://api.moonshot.cn/v1" {
		t.Fatalf("config = %+v", config)
	}
}

func TestAIResultViewShowsToolsAndSources(t *testing.T) {
	u, _ := newTestUI(t)
	u.region = regionSearch
	u.searchMode = 1
	u.aiProvider = "DeepSeek"
	u.aiModel = "deepseek-chat"
	u.aiKeySaved = true
	u.chat = []chatTurn{
		{ID: 1, Role: "user", Text: "有哪些 Go 帖子？"},
		{ID: 2, Role: "assistant", Text: "根据搜索，找到 **相关帖子**。",
			Tools:     []toolTrace{{ID: "call_1", Name: "search_forum", Arguments: `{"keywords":["Go"]}`, Summary: "搜索到 2 个主题"}},
			Citations: []svc.Citation{{Board: "Go", Title: "Go 实习内推", URL: "https://bbs.byr.cn/article/Go/1002"}},
		},
	}
	u.chatSeq = 2
	u.activeChat = 2
	tester := ui.NewTester(u.view, 1320, 860)
	if !tester.HasText("工具调用") || !tester.HasText("引用主题") {
		t.Fatalf("AI result view missing tool sections: %v", tester.Texts())
	}
}

func TestAISettingsShowsManagedKeys(t *testing.T) {
	u, _ := newTestUI(t)
	u.region = regionSettings
	u.aiKeys = []aiKeyEntry{
		{ID: "k1", Provider: "deepseek", Label: "主账号", Key: "test-deepseek-key"},
		{ID: "k2", Provider: "kimi", Label: "备用", Key: "test-kimi-key"},
	}
	u.aiKeyID = "k1"
	u.aiKeySaved = true
	tester := ui.NewTester(u.view, 1320, 900)
	if !tester.HasText("主账号") || !tester.HasText("备用") || !tester.HasText("添加 Key") {
		t.Fatalf("AI key management missing: %v", tester.Texts())
	}
}

func TestBuildReplyTreeFromQuotes(t *testing.T) {
	replies := []domain.Reply{
		{Key: domain.ReplyKey{ReplyID: "r1"}, Floor: 1, Author: "demo-author", Content: "主帖正文\n第二行"},
		{Key: domain.ReplyKey{ReplyID: "r2"}, Floor: 2, Author: "bob", Content: "回复正文", QuoteAuthor: "demo-author", QuoteContent: "主帖正文\n第二行"},
		{Key: domain.ReplyKey{ReplyID: "r3"}, Floor: 3, Author: "alice", Content: "继续回复", QuoteAuthor: "bob", QuoteContent: "回复正文"},
		{Key: domain.ReplyKey{ReplyID: "r4"}, Floor: 4, Author: "carol", Content: "独立回复"},
	}
	roots := buildReplyTree(replies)
	if len(roots) != 2 {
		t.Fatalf("roots = %d", len(roots))
	}
	if roots[0].Reply.Floor != 1 || len(roots[0].Children) != 1 {
		t.Fatalf("root 1 = %+v", roots[0])
	}
	child := roots[0].Children[0]
	if child.Reply.Floor != 2 || child.ParentFloor != 1 || len(child.Children) != 1 {
		t.Fatalf("child = %+v", child)
	}
	grandchild := child.Children[0]
	if grandchild.Reply.Floor != 3 || grandchild.ParentFloor != 2 {
		t.Fatalf("grandchild = %+v", grandchild)
	}
	if roots[1].Reply.Floor != 4 || len(roots[1].Children) != 0 {
		t.Fatalf("root 2 = %+v", roots[1])
	}
	open := make(map[string]bool)
	if !expandReplyPath(roots, 3, open) {
		t.Fatal("nested floor was not found")
	}
	if !open["r1"] || !open["r2"] {
		t.Fatalf("nested reply path was not expanded: %+v", open)
	}
}

func TestThreadExportRendersAndCrops(t *testing.T) {
	u, application := newTestUI(t)
	if err := application.OpenBoard(context.Background(), "b-go"); err != nil {
		t.Fatal(err)
	}
	board, _ := application.Board("b-go")
	key := board.Articles[0].Key
	if err := application.OpenThread(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	thread, _ := application.Thread(key)
	data := &threadExportData{
		title: thread.Article.Title, author: thread.Article.Author, createdAt: thread.Article.CreatedAt,
		replyCount: thread.Article.ReplyCount, content: thread.Article.Content,
		images: thread.Article.Images, bitmaps: map[string]*ui.Bitmap{}, replies: thread.Replies,
		tree: buildReplyTree(thread.Replies), theme: u.exportTheme(), fontScale: 1,
	}
	img := ui.Render(data.view, 1100, data.estimatedHeight(), 1.5)
	cropped := cropExport(img, data.theme.Background, 1.5)
	if cropped.Bounds().Dx() != 1650 || cropped.Bounds().Dy() <= 0 {
		t.Fatalf("export size = %v", cropped.Bounds())
	}
	if cropped.Bounds().Dy() >= img.Bounds().Dy() {
		t.Fatalf("export was not cropped: %d >= %d", cropped.Bounds().Dy(), img.Bounds().Dy())
	}
}
