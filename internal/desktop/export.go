package desktop

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"BYR-Client/internal/domain"
)

type threadExportData struct {
	title      string
	author     string
	createdAt  time.Time
	replyCount int
	content    string
	images     []domain.Image
	bitmaps    map[string]*ui.Bitmap
	replies    []domain.Reply
	tree       []*replyNode
	theme      ui.Theme
	fontScale  float32
}

func (u *UI) exportThreadPNG(key domain.ArticleKey) {
	if u.app == nil {
		return
	}
	thread, ok := u.app.Thread(key)
	if !ok || thread.Article.Title == "" {
		return
	}
	data := &threadExportData{
		title:      thread.Article.Title,
		author:     thread.Article.Author,
		createdAt:  thread.Article.CreatedAt,
		replyCount: thread.Article.ReplyCount,
		content:    thread.Article.Content,
		images:     thread.Article.Images,
		bitmaps:    make(map[string]*ui.Bitmap),
		replies:    thread.Replies,
		tree:       buildReplyTree(thread.Replies),
		theme:      u.exportTheme(),
		fontScale:  u.fontScale,
	}
	if data.fontScale <= 0 {
		data.fontScale = 1
	}
	for _, image := range thread.Article.Images {
		if state := u.images[image.URL]; state != nil && state.bitmap != nil {
			data.bitmaps[image.URL] = state.bitmap
		}
	}
	for _, reply := range thread.Replies {
		for _, image := range reply.Images {
			if state := u.images[image.URL]; state != nil && state.bitmap != nil {
				data.bitmaps[image.URL] = state.bitmap
			}
		}
	}
	win := u.window()
	defaultName := safeExportName(thread.Article.Title)
	if defaultName == "" {
		defaultName = "thread-" + string(key.ArticleID)
	}
	defaultName += ".png"

	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      win,
			Title:       "导出帖子 PNG",
			DefaultPath: defaultName,
			ButtonLabel: "导出",
			Filters:     []mygo.FileFilter{{Name: "PNG 图片", Extensions: []string{"png"}}},
		})
		if err != nil {
			mygo.Dialog.Error("导出失败", err.Error())
			return
		}
		if strings.TrimSpace(path) == "" {
			return
		}
		if !strings.EqualFold(filepath.Ext(path), ".png") {
			path += ".png"
		}
		height := data.estimatedHeight()
		img := ui.Render(data.view, 1100, height, 1.5)
		img = cropExport(img, data.theme.Background, 1.5)
		file, err := os.Create(path)
		if err != nil {
			mygo.Dialog.Error("导出失败", err.Error())
			return
		}
		defer file.Close()
		if err := png.Encode(file, img); err != nil {
			mygo.Dialog.Error("导出失败", err.Error())
			return
		}
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  win,
			Type:    mygo.MessageInfo,
			Title:   "导出完成",
			Message: "帖子已导出为 PNG",
			Detail:  path,
			Buttons: []string{"好"},
		})
	}()
}

func (u *UI) exportTheme() ui.Theme {
	var theme ui.Theme
	switch u.themeMode {
	case 1:
		theme = *ui.LightTheme()
	case 2:
		theme = *ui.DarkTheme()
	default:
		if u.baseTheme != nil {
			theme = *u.baseTheme
		} else {
			theme = *ui.LightTheme()
		}
	}
	return styledTheme(theme, u.accentMode, u.fontScale)
}

func (d *threadExportData) fs(size float32) float32 {
	if d.fontScale <= 0 {
		return size
	}
	return size * d.fontScale
}

func (d *threadExportData) view(c *ui.Context) {
	c.SetTheme(&d.theme)
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Padding(40, 44).Gap(14).Children(func() {
		ui.Text(c, d.title).FontSize(d.fs(26)).Bold().LineHeight(1.3)
		ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
			ui.Text(c, d.author).FontSize(d.fs(13)).Bold()
			ui.Text(c, relativeTime(d.createdAt)).FontSize(d.fs(12)).TextColor(t.TextMuted)
			ui.Textf(c, "%d 回复", d.replyCount).FontSize(d.fs(12)).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		if d.content != "" {
			ui.Text(c, d.content).FontSize(d.fs(14)).LineHeight(1.7)
		}
		d.writeImages(c, d.images, 560, 420)
		if len(d.replies) > 0 {
			ui.Divider(c)
			ui.Textf(c, "回复 · %d", len(d.replies)).FontSize(d.fs(15)).Bold()
			for _, node := range d.tree {
				d.writeReply(c, node, 0)
			}
		} else {
			ui.Textf(c, "回复 · %d", d.replyCount).FontSize(d.fs(13)).TextColor(t.TextMuted)
		}
		ui.Box(c).Height(4)
		ui.Text(c, "BYR Client · 导出结束").FontSize(d.fs(9)).TextColor(t.TextMuted)
	})
}

func (d *threadExportData) writeReply(c *ui.Context, node *replyNode, depth int) {
	t := c.Theme()
	reply := node.Reply
	indent := float32(depth)
	if indent > 6 {
		indent = 6
	}
	card := ui.Column(c).MarginStart(indent*18).Gap(6).Padding(11, 13).Radius(8).
		Background(t.Surface).Border(1, t.Border)
	if depth > 0 {
		card.BorderWidth(1, 1, 1, 3)
		card.PaddingStart(10)
	}
	card.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Badge(c, "#"+itoa(reply.Floor))
			ui.Text(c, reply.Author).FontSize(d.fs(12)).Bold()
			ui.Text(c, relativeTime(reply.CreatedAt)).FontSize(d.fs(10)).TextColor(t.TextMuted)
			if node.ParentFloor > 0 {
				ui.Textf(c, "回复 #%d", node.ParentFloor).FontSize(d.fs(9)).TextColor(t.TextMuted)
			}
		})
		if reply.QuoteAuthor != "" {
			ui.Column(c).Padding(5, 8).Radius(6).Background(t.Background).
				BorderWidth(0, 0, 0, 2).BorderColor(t.Accent).Gap(2).Children(func() {
				label := "引用 @" + reply.QuoteAuthor
				if node.ParentFloor > 0 {
					label += " · #" + itoa(node.ParentFloor)
				}
				ui.Text(c, label).FontSize(d.fs(9)).Bold().TextColor(t.Accent)
				if reply.QuoteContent != "" {
					ui.Text(c, truncate(reply.QuoteContent, 240)).FontSize(d.fs(10)).
						TextColor(t.TextMuted).MaxLines(4)
				}
			})
		}
		if reply.Content != "" {
			ui.Text(c, reply.Content).FontSize(d.fs(13)).LineHeight(1.6)
		}
		d.writeImages(c, reply.Images, 480, 340)
	})
	if len(node.Children) > 0 {
		ui.Column(c).MarginStart(indent * 18).Gap(4).Children(func() {
			ui.Textf(c, "楼中楼 · %d 条回复", len(node.Children)).FontSize(d.fs(10)).Bold().TextColor(t.Accent)
			for _, child := range node.Children {
				d.writeReply(c, child, depth+1)
			}
		})
	}
}

func (d *threadExportData) writeImages(c *ui.Context, images []domain.Image, maxWidth, maxHeight float32) {
	emojis := make([]domain.Image, 0)
	photos := make([]domain.Image, 0)
	for _, image := range images {
		if d.bitmaps[image.URL] == nil {
			continue
		}
		if image.Kind == domain.ImageEmoji {
			emojis = append(emojis, image)
		} else {
			photos = append(photos, image)
		}
	}
	if len(emojis) > 0 {
		ui.Row(c).Gap(4).Wrap().Children(func() {
			for _, image := range emojis {
				ui.Image(c, d.bitmaps[image.URL]).MaxWidth(d.fs(22)).MaxHeight(d.fs(22)).Radius(4)
			}
		})
	}
	for _, image := range photos {
		ui.Image(c, d.bitmaps[image.URL]).MaxWidth(maxWidth).MaxHeight(maxHeight).Radius(8)
	}
}

func (d *threadExportData) estimatedHeight() int {
	height := 300
	height += textLineCount(d.content, 62) * int(d.fs(24))
	for _, image := range d.images {
		if d.bitmaps[image.URL] != nil && image.Kind != domain.ImageEmoji {
			height += int(maxHeightForExport(image, 420))
		}
	}
	for _, reply := range d.replies {
		height += 90 + textLineCount(reply.Content, 64)*int(d.fs(22))
		if reply.QuoteAuthor != "" {
			height += 60 + textLineCount(reply.QuoteContent, 64)*int(d.fs(18))
		}
		for _, image := range reply.Images {
			if d.bitmaps[image.URL] != nil && image.Kind != domain.ImageEmoji {
				height += int(maxHeightForExport(image, 340))
			}
		}
	}
	if height < 720 {
		height = 720
	}
	if height > 9000 {
		height = 9000
	}
	return height
}

func maxHeightForExport(image domain.Image, limit float32) float32 {
	if image.Height <= 0 {
		return 220 * (limit / 420)
	}
	height := float32(image.Height)
	if height > limit {
		return limit
	}
	return height
}

func textLineCount(value string, perLine int) int {
	if value == "" {
		return 0
	}
	total := 0
	for _, line := range strings.Split(value, "\n") {
		runes := []rune(line)
		lines := len(runes) / perLine
		if len(runes)%perLine != 0 {
			lines++
		}
		if lines == 0 {
			lines = 1
		}
		total += lines
	}
	return total
}

func cropExport(img *image.RGBA, background ui.Color, scale float32) *image.RGBA {
	bg := color.RGBA{R: background.R, G: background.G, B: background.B, A: 255}
	bounds := img.Bounds()
	lastContent := bounds.Min.Y
	for y := bounds.Max.Y - 1; y >= bounds.Min.Y; y-- {
		rowContent := false
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := img.RGBAAt(x, y)
			if !sameRGB(pixel, bg) {
				rowContent = true
				break
			}
		}
		if rowContent {
			lastContent = y
			break
		}
	}
	margin := int(24 * scale)
	end := lastContent + margin
	if end < bounds.Max.Y {
		end = min(end, bounds.Max.Y)
	} else {
		end = bounds.Max.Y
	}
	height := end - bounds.Min.Y
	if height < 1 {
		return img
	}
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), height))
	draw.Draw(out, out.Bounds(), img, image.Point{X: bounds.Min.X, Y: bounds.Min.Y}, draw.Src)
	return out
}

func sameRGB(a, b color.RGBA) bool {
	const tolerance = 3
	difference := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return difference(a.R, b.R) <= tolerance && difference(a.G, b.G) <= tolerance && difference(a.B, b.B) <= tolerance
}

func safeExportName(title string) string {
	replacer := strings.NewReplacer(`\\`, "_", "/", "_", ":", "_", "*", "_", "?", "_", `"`, "_", "<", "_", ">", "_", "|", "_")
	name := strings.TrimSpace(replacer.Replace(title))
	runes := []rune(name)
	if len(runes) > 60 {
		name = string(runes[:60])
	}
	return strings.Trim(name, " .")
}
