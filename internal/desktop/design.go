package desktop

import (
	"time"

	"github.com/egoist/mygo/ui"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
)

const sidebarWidth float32 = 248

func (u *UI) readingBreakpoint() float32 {
	return sidebarWidth + 320 + 560*max(u.fontScale, 1)
}

func sidebarColor(t *ui.Theme, darkBackground int) ui.Color {
	if t.Dark {
		if darkBackground == 1 {
			return ui.Hex("#1C222C")
		}
		return ui.Hex("#171717")
	}
	return ui.Hex("#EEF1F5")
}

func (u *UI) appLogo(c *ui.Context, size float32) {
	if u.logo != nil {
		ui.Image(c, u.logo).Size(size, size).Shrink(0).Label("BYR Client 标志")
		return
	}
	ui.Icon(c, iconBrowse).Size(size, size).TextColor(c.Theme().Accent)
}

// Only the changing pane enters; stable chrome and streaming content stay put.
func pageTransition() ui.ElementTransition {
	return ui.ElementTransition{
		Colors: true, Duration: 180 * time.Millisecond,
		Enter: &ui.Motion{Y: 4}, Exit: &ui.Motion{},
	}
}

func (u *UI) readingPlaceholder(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillWidth().Background(t.Surface).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(820).Margin(0, ui.Auto).Padding(28, 32).Gap(20).Children(func() {
			ui.Box(c).WidthPercent(78).Height(28).Radius(6).Background(t.SurfaceHover)
			ui.Box(c).WidthPercent(32).Height(14).Radius(4).Background(t.SurfaceHover)
			ui.Row(c).Gap(8).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在读取主题正文…").FontSize(u.fs(12)).TextColor(t.TextMuted)
			})
			for _, width := range []float32{100, 94, 98, 70} {
				ui.Box(c).WidthPercent(width).Height(14).Radius(4).Background(t.SurfaceHover)
			}
		})
	})
}

type boardViewState struct {
	list   ui.ListState
	thread domain.ArticleKey
}

type readingKey struct {
	article domain.ArticleKey
	source  bool
}

type searchViewState struct {
	resultX, resultY float32
	aiX, aiY         float32
	collapsed        map[string]bool
}

func (u *UI) boardView(board domain.BoardID) *boardViewState {
	if u.boardViews == nil {
		u.boardViews = make(map[domain.BoardID]*boardViewState)
	}
	if u.boardViews[board] == nil {
		u.boardViews[board] = &boardViewState{}
	}
	return u.boardViews[board]
}

func (u *UI) readingView(article domain.ArticleKey, source bool) *ui.ScrollState {
	if u.readingViews == nil {
		u.readingViews = make(map[readingKey]*ui.ScrollState)
	}
	key := readingKey{article, source}
	if u.readingViews[key] == nil {
		u.readingViews[key] = &ui.ScrollState{}
	}
	return u.readingViews[key]
}

func (u *UI) directoryView(node forum.NodeID) *ui.ScrollState {
	if u.directoryViews == nil {
		u.directoryViews = make(map[forum.NodeID]*ui.ScrollState)
	}
	if u.directoryViews[node] == nil {
		u.directoryViews[node] = &ui.ScrollState{}
	}
	return u.directoryViews[node]
}

func (u *UI) saveSearchView() {
	if u.activeHistory == "" {
		return
	}
	if u.searchViews == nil {
		u.searchViews = make(map[string]searchViewState)
	}
	u.searchViews[u.activeHistory] = searchViewState{
		resultX: u.resultScroll.X, resultY: u.resultScroll.Y,
		aiX: u.aiScroll.X, aiY: u.aiScroll.Y, collapsed: u.collapsed,
	}
}

func (u *UI) restoreSearchView(id string) {
	state := u.searchViews[id]
	if u.searchMode == 1 {
		u.aiScroll = ui.ScrollState{X: state.aiX, Y: state.aiY}
		return
	}
	u.resultScroll = ui.ScrollState{X: state.resultX, Y: state.resultY}
	u.collapsed = state.collapsed
	if u.collapsed == nil {
		u.collapsed = make(map[string]bool)
	}
}

func (u *UI) switchSearchMode(previous int) {
	u.saveSearchView()
	if previous == 0 {
		u.keywordHistory = u.activeHistory
	}
	if u.searchMode == 0 {
		u.activeHistory = u.keywordHistory
	} else {
		u.activeHistory = u.aiHistoryID
	}
	if u.activeHistory != "" {
		u.restoreSearchView(u.activeHistory)
	}
}
