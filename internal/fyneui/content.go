package fyneui

import (
	"bytes"
	"container/list"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"BYR-Client/internal/domain"
)

const (
	imageWorkerCount = 4
	imageQueueLength = 16
	imageCacheCount  = 16
	photoHeight      = float32(220)
)

// contentRenderer owns a bounded image queue for the current window. Views
// contain no network goroutines: even a long reply list can enqueue only a
// fixed amount of image work. Close prevents queued callbacks touching the UI.
type contentRenderer struct {
	fetch  func(string) ([]byte, error)
	window fyne.Window
	ctx    context.Context
	cancel context.CancelFunc
	jobs   chan imageJob
	mu     sync.Mutex
	cache  map[string]*list.Element
	lru    *list.List
}

type imageJob struct {
	url  string
	done func(image.Image, error)
}

type cachedImage struct {
	url   string
	image image.Image
}

func newContentRenderer(fetch func(string) ([]byte, error), window fyne.Window) *contentRenderer {
	ctx, cancel := context.WithCancel(context.Background())
	r := &contentRenderer{
		fetch: fetch, window: window, ctx: ctx, cancel: cancel,
		jobs:  make(chan imageJob, imageQueueLength),
		cache: make(map[string]*list.Element), lru: list.New(),
	}
	for i := 0; i < imageWorkerCount; i++ {
		go r.imageWorker()
	}
	return r
}

func (r *contentRenderer) Close() { r.cancel() }

func (r *contentRenderer) Post(article domain.Article) fyne.CanvasObject {
	return r.body(article.Content, article.Images)
}

func (r *contentRenderer) Reply(reply domain.Reply) fyne.CanvasObject {
	body := r.body(reply.Content, reply.Images)
	if reply.QuoteAuthor == "" && reply.QuoteContent == "" {
		return body
	}
	quote := newSelectableBody("引用 @" + reply.QuoteAuthor + "\n" + reply.QuoteContent)
	quote.TextStyle.Italic = true
	return container.NewVBox(widget.NewCard("", "", quote), body)
}

var bodyLinkPattern = regexp.MustCompile(`https?://[^\s<>]+`)

func (r *contentRenderer) body(text string, images []domain.Image) fyne.CanvasObject {
	items := make([]fyne.CanvasObject, 0, len(images)+3)
	if strings.TrimSpace(text) != "" {
		items = append(items, newSelectableBody(text))
	}
	// Domain content currently has no inline span model. Preserve the selectable
	// source text, then expose its web links as separately tappable controls.
	seen := make(map[string]bool)
	for _, candidate := range bodyLinkPattern.FindAllString(text, -1) {
		raw := strings.TrimRight(candidate, ".,;!?，。；！？）)]}》】\"'")
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Host == "" || seen[raw] {
			continue
		}
		seen[raw] = true
		link := widget.NewHyperlink(raw, parsed)
		link.Wrapping = fyne.TextWrapBreak
		items = append(items, link)
	}
	for _, picture := range images {
		items = append(items, r.imageView(picture))
	}
	if len(items) == 0 {
		items = append(items, newSelectableBody("（此楼没有正文）"))
	}
	return container.NewVBox(items...)
}

func newSelectableBody(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapBreak
	label.Selectable = true
	return label
}

func (r *contentRenderer) imageView(picture domain.Image) fyne.CanvasObject {
	height := photoHeight
	if picture.Kind == domain.ImageEmoji {
		height = 28
	}
	img := canvas.NewImageFromImage(nil)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(1, height))
	img.CornerRadius = 8
	tile := &tappablePicture{picture: img}
	tile.ExtendBaseWidget(tile)
	status := widget.NewLabel("正在加载图片…")
	status.Wrapping = fyne.TextWrapWord
	status.Importance = widget.LowImportance
	if picture.Kind == domain.ImageEmoji {
		status.SetText("表情")
	}
	retry := widget.NewButton("重试图片", nil)
	retry.Importance = widget.LowImportance
	retry.Hide()
	box := container.NewVBox(tile, container.NewBorder(nil, nil, nil, retry, status))
	load := func() {
		if r.ctx.Err() != nil {
			return
		}
		retry.Hide()
		status.SetText("正在加载图片…")
		job := imageJob{url: picture.URL, done: func(decoded image.Image, err error) {
			if err != nil {
				status.SetText("图片加载失败：" + shortImageError(err))
				retry.Show()
				return
			}
			img.Image = decoded
			img.Refresh()
			if picture.Kind == domain.ImageEmoji {
				status.Hide()
			} else {
				status.SetText("点击图片放大")
			}
			tile.onTapped = func() { r.preview(decoded) }
			box.Refresh()
		}}
		select {
		case r.jobs <- job:
		default:
			status.SetText("图片队列已满，点击重试加载")
			retry.Show()
		}
	}
	retry.OnTapped = load
	load()
	return box
}

func shortImageError(err error) string {
	runes := []rune(err.Error())
	if len(runes) > 72 {
		return string(runes[:72]) + "…"
	}
	return string(runes)
}

func (r *contentRenderer) imageWorker() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case job := <-r.jobs:
			if r.ctx.Err() != nil {
				return
			}
			decoded, err := r.readImage(job.url)
			if r.ctx.Err() != nil {
				return
			}
			fyne.Do(func() {
				if r.ctx.Err() == nil {
					job.done(decoded, err)
				}
			})
		}
	}
}

func (r *contentRenderer) readImage(rawURL string) (image.Image, error) {
	r.mu.Lock()
	if element := r.cache[rawURL]; element != nil {
		r.lru.MoveToFront(element)
		img := element.Value.(cachedImage).image
		r.mu.Unlock()
		return img, nil
	}
	r.mu.Unlock()
	if r.fetch == nil {
		return nil, fmt.Errorf("没有配置图片数据源")
	}
	data, err := r.fetch(rawURL)
	if err != nil {
		return nil, err
	}
	if len(data) > 20<<20 {
		return nil, fmt.Errorf("图片超过原型的 20 MiB 大小限制")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 24_000_000 {
		return nil, fmt.Errorf("图片像素尺寸超过原型限制")
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	// Keep decoded cache memory bounded as well as its item count. Full-size
	// original export is outside this prototype; previews use this thumbnail.
	decoded = imageThumbnail(decoded, 1280, 960)
	r.mu.Lock()
	if element := r.cache[rawURL]; element != nil {
		element.Value = cachedImage{url: rawURL, image: decoded}
		r.lru.MoveToFront(element)
	} else {
		r.cache[rawURL] = r.lru.PushFront(cachedImage{url: rawURL, image: decoded})
	}
	for r.lru.Len() > imageCacheCount {
		oldest := r.lru.Back()
		delete(r.cache, oldest.Value.(cachedImage).url)
		r.lru.Remove(oldest)
	}
	r.mu.Unlock()
	return decoded, nil
}

func imageThumbnail(src image.Image, maxWidth, maxHeight int) image.Image {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= maxWidth && height <= maxHeight {
		return src
	}
	if int64(width)*int64(maxHeight) > int64(height)*int64(maxWidth) {
		height = max(1, height*maxWidth/width)
		width = maxWidth
	} else {
		width = max(1, width*maxHeight/height)
		height = maxHeight
	}
	thumb := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(thumb, thumb.Bounds(), src, bounds, draw.Src, nil)
	return thumb
}

func (r *contentRenderer) preview(decoded image.Image) {
	if r.window == nil || r.ctx.Err() != nil {
		return
	}
	image := canvas.NewImageFromImage(decoded)
	image.FillMode = canvas.ImageFillContain
	image.SetMinSize(fyne.NewSize(1, 1))
	preview := dialog.NewCustom("图片预览", "关闭", image, r.window)
	size := r.window.Canvas().Size()
	preview.Resize(fyne.NewSize(max(240, size.Width-40), max(240, size.Height-100)))
	preview.Show()
}

type tappablePicture struct {
	widget.BaseWidget
	picture  *canvas.Image
	onTapped func()
}

func (p *tappablePicture) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.picture)
}

func (p *tappablePicture) Tapped(*fyne.PointEvent) {
	if p.onTapped != nil {
		p.onTapped()
	}
}
