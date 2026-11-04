package fyneui

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/fyneproto"
)

type browseLayout struct{ ui *UI }

func (l *browseLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(280, 280) }

func (l *browseLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 2 {
		return
	}
	left, right := objects[0], objects[1]
	readerWidth := size.Width
	if size.Width >= 820 {
		left.Show()
		right.Show()
		width := min(float32(350), size.Width*0.31)
		left.Move(fyne.NewPos(0, 0))
		left.Resize(fyne.NewSize(width, size.Height))
		right.Move(fyne.NewPos(width+10, 0))
		readerWidth = size.Width - width - 10
		right.Resize(fyne.NewSize(readerWidth, size.Height))
	} else if l.ui.reading {
		left.Hide()
		right.Show()
		right.Move(fyne.NewPos(0, 0))
		right.Resize(size)
	} else {
		right.Hide()
		left.Show()
		left.Move(fyne.NewPos(0, 0))
		left.Resize(size)
	}
	if math.Abs(float64(readerWidth-l.ui.readerWidth)) > 8 {
		l.ui.readerWidth = readerWidth
		// Schedule after layout to avoid re-entering List.UpdateItem while
		// virtual row heights are being recomputed for a new viewport.
		fyne.Do(func() {
			if !l.ui.closed.Load() {
				l.ui.sizeReaderRows()
			}
		})
	}
}

func (u *UI) buildBrowse() fyne.CanvasObject {
	u.boardSelect = widget.NewSelect(nil, func(label string) {
		if id, ok := u.boardOptions[label]; ok && id != u.state.SelectedBoard {
			u.reading = false
			u.controller.SelectBoard(id)
			u.browse.Refresh()
		}
	})
	u.boardSelect.PlaceHolder = "选择版面"
	stress := widget.NewButtonWithIcon("压力帖", theme.ListIcon(), func() {
		u.reading = true
		u.controller.OpenThread(fyneproto.StressKey())
		u.browse.Refresh()
	})
	toolbar := container.NewBorder(nil, nil, nil, stress,
		constrained(u.boardSelect, fyne.NewSize(110, 40)))
	u.boardTitle = heading("主题列表")
	u.boardCount = subtle("按需读取")
	u.boardList = widget.NewList(func() int { return len(u.state.Board.Articles) }, func() fyne.CanvasObject {
		return container.NewVBox(heading("主题"), subtle("作者 · 回复"), subtle("摘要"))
	}, func(index widget.ListItemID, object fyne.CanvasObject) {
		if index >= len(u.state.Board.Articles) {
			return
		}
		article := u.state.Board.Articles[index]
		box := object.(*fyne.Container)
		box.Objects[0].(*widget.Label).SetText(article.Title)
		box.Objects[1].(*widget.Label).SetText(fmt.Sprintf("%s  ·  %d 回复", article.Author, article.ReplyCount))
		box.Objects[2].(*widget.Label).SetText(article.Summary)
	})
	u.boardList.OnSelected = func(index widget.ListItemID) {
		if index >= len(u.state.Board.Articles) {
			return
		}
		u.reading = true
		u.controller.OpenThread(u.state.Board.Articles[index].Key)
		u.browse.Refresh()
	}
	u.boardMore = widget.NewButton("加载更多主题", u.controller.LoadMoreBoard)
	listPanel := panel(container.NewBorder(container.NewVBox(u.boardTitle, u.boardCount), u.boardMore, nil, nil, u.boardList))

	u.readerTitle = heading("请选择一个主题")
	u.readerCount = subtle("正文和回复按需加载")
	back := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		u.reading = false
		u.boardList.UnselectAll()
		u.browse.Refresh()
	})
	readerHeader := container.NewBorder(nil, nil, back, nil, container.NewVBox(u.readerTitle, u.readerCount))
	u.reader = widget.NewList(func() int { return len(u.rows) }, func() fyne.CanvasObject {
		return newPostRow(u)
	}, func(index widget.ListItemID, object fyne.CanvasObject) {
		if index < len(u.rows) {
			object.(*postRow).bind(u.rows[index])
		}
	})
	u.reader.HideSeparators = true
	u.replyMore = widget.NewButton("更多回复", u.controller.LoadMoreReplies)
	u.replyAll = widget.NewButton("加载全部", u.controller.LoadAllReplies)
	readerPanel := panel(container.NewBorder(readerHeader,
		container.NewGridWithColumns(2, u.replyMore, u.replyAll), nil, nil, u.reader))
	u.browseLayout = &browseLayout{ui: u}
	u.browse = container.New(u.browseLayout, listPanel, readerPanel)
	return container.NewBorder(padded(toolbar, 0, 8, 0, 0), nil, nil, nil, u.browse)
}

func (u *UI) syncBrowse() {
	if len(u.boardSelect.Options) != len(u.state.Boards) {
		labels := make([]string, 0, len(u.state.Boards))
		for _, board := range u.state.Boards {
			label := board.Name
			if utf8.RuneCountInString(label) > 10 {
				label = string([]rune(label)[:9]) + "…"
			}
			if _, exists := u.boardOptions[label]; exists {
				label += " · " + string(board.ID)
			}
			u.boardOptions[label] = board.ID
			labels = append(labels, label)
		}
		u.boardSelect.Options = labels
		u.boardSelect.Refresh()
	}
	for label, id := range u.boardOptions {
		if id == u.state.SelectedBoard && u.boardSelect.Selected != label {
			// Do not invoke the selection command while applying a snapshot.
			u.boardSelect.Selected = label
			u.boardSelect.Refresh()
		}
	}
	u.boardTitle.SetText(u.state.Board.Name)
	boardSummary := fmt.Sprintf("已加载 %d 个主题", len(u.state.Board.Articles))
	if u.state.Board.Loading {
		boardSummary += " · 加载中"
	}
	if u.state.Board.Error != "" {
		boardSummary = u.state.Board.Error
	}
	u.boardCount.SetText(boardSummary)
	stamp := fmt.Sprintf("%s/%d/%s", u.state.SelectedBoard, len(u.state.Board.Articles), u.state.Board.ObservedAt)
	if stamp != u.boardStamp {
		u.boardStamp = stamp
		u.boardList.Refresh()
	}
	setEnabled(u.boardMore, !u.state.Board.Loading && !u.state.Board.End && u.state.Ready)
	u.readerTitle.SetText(u.state.Thread.Article.Title)
	count := fmt.Sprintf("%s · %d / %d 条回复", u.state.Thread.Article.Author,
		len(u.state.Thread.Replies), u.state.Thread.Article.ReplyCount)
	if u.state.Thread.Loading {
		count += " · 加载中"
	}
	if u.state.Thread.Error != "" {
		count = u.state.Thread.Error
	}
	u.readerCount.SetText(count)
	setEnabled(u.replyMore, !u.state.Thread.Loading && !u.state.Thread.End && u.state.Thread.Article.BodyLoaded)
	setEnabled(u.replyAll, !u.state.Thread.Loading && !u.state.Thread.End && u.state.Thread.Article.BodyLoaded)
	stamp = fmt.Sprintf("%v/%d/%s", u.state.SelectedThread, len(u.state.Thread.Replies), u.state.Thread.ObservedAt)
	if stamp != u.readerStamp {
		changedThread := len(u.rows) == 0 || u.rows[0].article.Key != u.state.Thread.Article.Key
		u.readerStamp = stamp
		u.rebuildReader()
		if changedThread {
			u.reader.ScrollToTop()
		}
	}
}

type replyRow struct {
	article     domain.Article
	reply       domain.Reply
	root        bool
	depth       int
	children    int
	parentFloor int
}

func (r replyRow) key() string {
	if r.root {
		return fmt.Sprint(r.article.Key)
	}
	return string(r.reply.Key.ReplyID)
}

type quoteNode struct {
	reply       domain.Reply
	children    []*quoteNode
	parentFloor int
}

// Quote relationships are inferred, not authoritative parent IDs. Indexing by
// author limits comparisons and the tree is rebuilt only when content changes.
func replyRows(article domain.Article, replies []domain.Reply, open map[string]bool) []replyRow {
	if article.Title == "" {
		return nil
	}
	rows := []replyRow{{article: article, root: true}}
	byAuthor := make(map[string][]*quoteNode)
	var roots []*quoteNode
	for _, reply := range replies {
		node := &quoteNode{reply: reply}
		var parent *quoteNode
		if reply.QuoteAuthor != "" && reply.QuoteContent != "" {
			candidates := byAuthor[strings.ToLower(strings.TrimSpace(reply.QuoteAuthor))]
			for i := len(candidates) - 1; i >= 0; i-- {
				if strings.Contains(candidates[i].reply.Content, reply.QuoteContent) {
					parent = candidates[i]
					break
				}
			}
		}
		if parent == nil {
			roots = append(roots, node)
		} else {
			parent.children = append(parent.children, node)
			node.parentFloor = parent.reply.Floor
		}
		name := strings.ToLower(strings.TrimSpace(reply.Author))
		byAuthor[name] = append(byAuthor[name], node)
	}
	var flatten func([]*quoteNode, int)
	flatten = func(nodes []*quoteNode, depth int) {
		for _, node := range nodes {
			rows = append(rows, replyRow{reply: node.reply, depth: depth,
				children: len(node.children), parentFloor: node.parentFloor})
			if isOpen, exists := open[string(node.reply.Key.ReplyID)]; !exists || isOpen {
				flatten(node.children, depth+1)
			}
		}
	}
	flatten(roots, 0)
	return rows
}

func (u *UI) rebuildReader() {
	u.rows = replyRows(u.state.Thread.Article, u.state.Thread.Replies, u.replyOpen)
	u.sizeReaderRows()
	u.reader.Refresh()
}

func (u *UI) sizeReaderRows() {
	if u.reader == nil {
		return
	}
	width := max(float32(240), u.readerWidth-60)
	for i, row := range u.rows {
		text, images := row.reply.Content, row.reply.Images
		if row.root {
			text, images = row.article.Content, row.article.Images
		} else if !u.expanded[row.key()] {
			text = previewText(text, 150)
		}
		available := width - float32(min(row.depth, 3))*12
		lineLength := max(12, int(available/15))
		lines := 0
		for _, paragraph := range strings.Split(text, "\n") {
			lines += max(1, (utf8.RuneCountInString(paragraph)+lineLength-1)/lineLength)
		}
		// Include header/actions, label inner padding and image captions. The
		// conservative reserve prevents an action row overlapping wrapped text.
		height := float32(lines)*25 + 150
		if row.reply.QuoteAuthor != "" {
			height += 70
		}
		for _, image := range images {
			if image.Kind == domain.ImageEmoji {
				height += 42
			} else {
				height += 245
			}
		}
		u.reader.SetItemHeight(i, height)
	}
}

type postRow struct {
	widget.BaseWidget
	ui       *UI
	bound    string
	row      replyRow
	header   *widget.Label
	quote    *widget.Label
	body     *fyne.Container
	disclose *widget.Button
	details  *widget.Button
	box      *fyne.Container
}

func newPostRow(u *UI) *postRow {
	r := &postRow{ui: u}
	r.header = heading("")
	r.quote = subtle("")
	r.quote.Wrapping = fyne.TextWrapWord
	r.body = container.NewStack()
	r.disclose = widget.NewButton("折叠引用", func() {
		key := r.row.key()
		open, exists := u.replyOpen[key]
		if !exists {
			open = true
		}
		u.replyOpen[key] = !open
		u.rebuildReader()
	})
	r.details = widget.NewButton("展开全文", func() {
		u.expanded[r.row.key()] = !u.expanded[r.row.key()]
		r.bound = ""
		u.sizeReaderRows()
		u.reader.Refresh()
	})
	copy := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		text := r.row.reply.Content
		if r.row.root {
			text = r.row.article.Content
		}
		u.window.Clipboard().SetContent(text)
	})
	header := container.NewBorder(nil, nil, nil, copy, r.header)
	buttons := container.NewHBox(r.disclose, r.details, layout.NewSpacer())
	r.box = container.NewBorder(container.NewVBox(header, r.quote), buttons, nil, nil, r.body)
	r.ExtendBaseWidget(r)
	return r
}

func (r *postRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(padded(r.box, 10, 14, 4, 4))
}

func (r *postRow) bind(row replyRow) {
	u := r.ui
	isOpen, exists := u.replyOpen[row.key()]
	if !exists {
		isOpen = true
	}
	bound := fmt.Sprintf("%s/%t/%t/%d", row.key(), u.expanded[row.key()], isOpen, row.children)
	if r.bound == bound {
		return
	}
	r.bound, r.row = bound, row
	if row.root {
		r.header.SetText("主帖 · " + row.article.Author)
		r.quote.Hide()
		r.disclose.Hide()
		r.details.Hide()
		r.body.Objects = []fyne.CanvasObject{u.content.Post(row.article)}
	} else {
		prefix := strings.Repeat("› ", min(row.depth, 3))
		r.header.SetText(fmt.Sprintf("%s#%d  %s", prefix, row.reply.Floor, row.reply.Author))
		if row.reply.QuoteAuthor != "" {
			quote := "引用 " + row.reply.QuoteAuthor
			if row.parentFloor > 0 {
				quote += fmt.Sprintf(" · #%d（推断）", row.parentFloor)
			}
			r.quote.SetText(quote + "\n" + previewText(row.reply.QuoteContent, 40))
			r.quote.Show()
		} else {
			r.quote.Hide()
		}
		if row.children > 0 {
			r.disclose.SetText(fmt.Sprintf("折叠 %d 条引用", row.children))
			if !isOpen {
				r.disclose.SetText(fmt.Sprintf("展开 %d 条引用", row.children))
			}
			r.disclose.Show()
		} else {
			r.disclose.Hide()
		}
		reply := row.reply
		// The compact quote summary is already rendered in the row header.
		// Avoid duplicating the full (possibly very long) quote in the body.
		reply.QuoteAuthor, reply.QuoteContent = "", ""
		if utf8.RuneCountInString(reply.Content) > 150 {
			r.details.Show()
			if u.expanded[row.key()] {
				r.details.SetText("收起全文")
			} else {
				r.details.SetText("展开全文")
				reply.Content = previewText(reply.Content, 150)
			}
		} else {
			r.details.Hide()
		}
		r.body.Objects = []fyne.CanvasObject{u.content.Reply(reply)}
	}
	r.body.Refresh()
	r.Refresh()
}

func previewText(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit]) + "…"
}

func setEnabled(button *widget.Button, enabled bool) {
	if enabled {
		if button.Disabled() {
			button.Enable()
		}
	} else if !button.Disabled() {
		button.Disable()
	}
}
