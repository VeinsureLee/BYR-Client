package ui

import (
	"bytes"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/resources"
)

const (
	maxGIFFrames      = 60
	maxImageDimension = 12000
	maxImagePixels    = int64(24000000)
	// A timer tick must not make a phone render faster than 30fps. GIF frame
	// timing is still evaluated from the wall clock, so a long frame simply
	// renders the same image on any intermediate tick.
	imageAnimationInterval = 34 * time.Millisecond
)

type mobileImage struct {
	loading bool
	err     error
	raw     []byte
	img     image.Image
	frames  []image.Image
	delays  []time.Duration
	started time.Time
	w, h    int

	animationMu         sync.Mutex
	animationVisible    bool
	animationTimer      *time.Timer
	animationGeneration uint64
}

// decodeMobileImage decodes one image entry and keeps every bounded GIF frame
// available to both inline images and avatars. The first frame is retained as
// img so all callers still have a static fallback when animation is unavailable.
func decodeMobileImage(b []byte) (*mobileImage, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if cfg.Width > maxImageDimension || cfg.Height > maxImageDimension || int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return nil, contentError("图片尺寸过大")
	}

	isGIF := format == "gif" || hasGIFHeader(b)
	if !isGIF {
		img, _, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		return &mobileImage{
			img:     img,
			started: time.Now(),
			w:       img.Bounds().Dx(),
			h:       img.Bounds().Dy(),
		}, nil
	}

	decoded, err := gif.DecodeAll(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if len(decoded.Image) == 0 {
		return nil, contentError("GIF 没有可显示的帧")
	}
	if len(decoded.Image) > maxGIFFrames {
		return nil, contentError("GIF 帧数过多")
	}

	frames := make([]image.Image, len(decoded.Image))
	delays := make([]time.Duration, len(decoded.Image))
	var totalPixels int64
	for i, frame := range decoded.Image {
		if frame == nil {
			return nil, contentError("GIF 帧无效")
		}
		bounds := frame.Bounds()
		width, height := bounds.Dx(), bounds.Dy()
		if width > maxImageDimension || height > maxImageDimension {
			return nil, contentError("GIF 帧尺寸过大")
		}
		totalPixels += int64(width) * int64(height)
		if totalPixels > maxImagePixels {
			return nil, contentError("GIF 总像素过多")
		}
		frames[i] = frame
		delay := 0
		if i < len(decoded.Delay) {
			delay = decoded.Delay[i]
		}
		delays[i] = gifDelay(delay)
	}

	first := frames[0]
	return &mobileImage{
		img:     first,
		frames:  frames,
		delays:  delays,
		started: time.Now(),
		w:       first.Bounds().Dx(),
		h:       first.Bounds().Dy(),
	}, nil
}

func hasGIFHeader(b []byte) bool {
	return len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a")
}

func gifDelay(ticks int) time.Duration {
	if ticks <= 0 {
		// A zero GIF delay is commonly used for a browser-default short pause.
		// Keep it positive so frameAt always has a well-defined interval.
		return 100 * time.Millisecond
	}
	return time.Duration(ticks) * 10 * time.Millisecond
}

// frameAt returns the frame active at now. It deliberately uses elapsed time
// rather than advancing an index, so a delayed Android frame cannot make the
// animation drift or accumulate timer callbacks.
func (m *mobileImage) frameAt(now time.Time) image.Image {
	if m == nil {
		return nil
	}
	if len(m.frames) == 0 {
		return m.img
	}
	if now.IsZero() {
		now = time.Now()
	}
	m.animationMu.Lock()
	if m.started.IsZero() {
		m.started = now
	}
	started := m.started
	m.animationMu.Unlock()
	elapsed := now.Sub(started)
	if elapsed < 0 {
		elapsed = 0
	}

	var total time.Duration
	for i := range m.frames {
		total += m.frameDelay(i)
	}
	if total <= 0 {
		return m.frames[0]
	}
	position := elapsed % total
	for i := range m.frames {
		delay := m.frameDelay(i)
		if position < delay {
			if m.frames[i] != nil {
				return m.frames[i]
			}
			return m.img
		}
		position -= delay
	}
	return m.frames[len(m.frames)-1]
}

func (m *mobileImage) frameDelay(index int) time.Duration {
	if index >= 0 && index < len(m.delays) && m.delays[index] > 0 {
		return m.delays[index]
	}
	return 100 * time.Millisecond
}

func (a *App) imageEntry(raw string) *mobileImage {
	if a.images == nil {
		a.images = map[string]*mobileImage{}
	}
	if e := a.images[raw]; e != nil {
		return e
	}
	e := &mobileImage{loading: true}
	a.images[raw] = e
	go func() {
		var err error
		var b []byte
		switch {
		case strings.HasPrefix(raw, "emoji:"):
			b, err = resources.Emoji.ReadFile("em/" + strings.TrimPrefix(raw, "emoji:"))
		case a.core == nil:
			err = contentError("客户端尚未启动")
		default:
			// FetchAsset remains the only network path for images. It resolves
			// trusted origins and injects tokens only where forum.Client allows it.
			b, err = a.core.Forum.FetchAsset(a.ctx, raw)
		}

		var decoded *mobileImage
		if err == nil {
			decoded, err = decodeMobileImage(b)
			if decoded != nil {
				decoded.raw = append([]byte(nil), b...)
			}
		}
		a.post(func() {
			e.loading = false
			e.err = err
			if decoded != nil {
				e.raw = decoded.raw
				e.img = decoded.img
				e.frames = decoded.frames
				e.delays = decoded.delays
				e.started = decoded.started
				e.w = decoded.w
				e.h = decoded.h
			}
		})
	}()
	return e
}

type contentError string

func (e contentError) Error() string { return string(e) }

func imageNow(gtx layout.Context) time.Time {
	if gtx.Now.IsZero() {
		return time.Now()
	}
	return gtx.Now
}

func (a *App) avatar(gtx layout.Context, user forum.User, size unit.Dp) layout.Dimensions {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	if user.FaceURL == "" {
		return a.avatarCircle(gtx, user.DisplayName(), size, colorAccentSoft, colorAccent)
	}
	entry := a.imageEntry(user.FaceURL)
	if entry.loading || entry.err != nil || entry.img == nil {
		return a.avatarCircle(gtx, user.DisplayName(), size, colorAccentSoft, colorAccent)
	}
	img := entry.frameAt(imageNow(gtx))
	if img == nil {
		return a.avatarCircle(gtx, user.DisplayName(), size, colorAccentSoft, colorAccent)
	}
	a.markImageVisible(entry)
	stack := clip.Ellipse(image.Rectangle{Max: image.Pt(px, px)}).Push(gtx.Ops)
	defer stack.Pop()
	imageWidget := widget.Image{
		Src:      paint.NewImageOp(img),
		Fit:      widget.Cover,
		Position: layout.Center,
	}
	return imageWidget.Layout(gtx)
}

func (a *App) postBody(gtx layout.Context, body string, attachments []forum.Attachment) layout.Dimensions {
	parts := content.Parse(body, attachments)
	if len(parts) == 0 {
		l := material.Body1(a.theme, "(无内容)")
		return l.Layout(gtx)
	}
	widgets := make([]layout.Widget, 0, len(parts))
	for _, part := range parts {
		part := part
		switch part.Kind {
		case content.Text:
			widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
				l := material.Body1(a.theme, part.Text)
				l.TextSize = unit.Sp(15)
				if part.Style.Bold {
					l.Font.Weight = 700
				}
				if part.Style.Italic {
					l.Font.Style = font.Italic
				}
				if part.Style.Quote {
					l.Color = colorHint
				}
				return l.Layout(gtx)
			})
		case content.Image:
			raw := part.URL
			full := part.FullURL
			widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
				return a.inlineImage(gtx, raw, full)
			})
		case content.File:
			name := part.Text
			widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
				return material.Caption(a.theme, "[附件] "+name).Layout(gtx)
			})
		case content.Emoji:
			fallback := part.Text
			raw := "emoji:" + part.URL
			widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
				return a.inlineEmoji(gtx, raw, fallback)
			})
		case content.Link:
			text := part.Text
			if text == "" {
				text = part.URL
			}
			widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
				return material.Caption(a.theme, text).Layout(gtx)
			})
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rigidWidgets(widgets)...)
}

func rigidWidgets(widgets []layout.Widget) []layout.FlexChild {
	out := make([]layout.FlexChild, 0, len(widgets))
	for _, w := range widgets {
		out = append(out, layout.Rigid(w))
	}
	return out
}

func (a *App) inlineEmoji(gtx layout.Context, raw, fallback string) layout.Dimensions {
	entry := a.imageEntry(raw)
	if entry.loading {
		return layout.Dimensions{Size: image.Pt(gtx.Dp(unit.Dp(22)), gtx.Dp(unit.Dp(22)))}
	}
	if entry.err != nil || entry.img == nil {
		return material.Caption(a.theme, fallback).Layout(gtx)
	}
	size := gtx.Dp(unit.Dp(24))
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	img := entry.frameAt(imageNow(gtx))
	if img == nil {
		return material.Caption(a.theme, fallback).Layout(gtx)
	}
	a.markImageVisible(entry)
	return widget.Image{Src: paint.NewImageOp(img), Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
}

// inlineImage accepts an optional original URL so callers can display a
// thumbnail while opening the full attachment in the viewer. The variadic form
// keeps the small helper convenient for emoji/tests that only have one URL.
func (a *App) inlineImage(gtx layout.Context, raw string, fullURLs ...string) layout.Dimensions {
	e := a.imageEntry(raw)
	if e.loading {
		return material.Caption(a.theme, "图片加载中…").Layout(gtx)
	}
	if e.err != nil {
		return material.Caption(a.theme, "[图片加载失败]").Layout(gtx)
	}
	if e.img == nil || e.w == 0 || e.h == 0 {
		return layout.Dimensions{}
	}
	maxW := gtx.Constraints.Max.X
	if maxW <= 0 {
		maxW = gtx.Dp(unit.Dp(320))
	}
	h := int(float64(maxW) * float64(e.h) / float64(e.w))
	if h < 1 {
		h = 1
	}
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max = image.Pt(maxW, h)
	img := e.frameAt(imageNow(gtx))
	if img == nil {
		return material.Caption(a.theme, "[图片加载失败]").Layout(gtx)
	}
	a.markImageVisible(e)
	btn := a.click("image:" + raw)
	clicked := btn.Clicked(gtx)
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widget.Image{Src: paint.NewImageOp(img), Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
	})
	if clicked {
		viewerURL := raw
		if len(fullURLs) > 0 && fullURLs[0] != "" {
			viewerURL = fullURLs[0]
		}
		a.openImage(raw, viewerURL)
	}
	return dims
}

func (a *App) beginImageLayoutFrame() {
	for _, entry := range a.images {
		a.stopImageAnimation(entry)
	}
}

func (a *App) markImageVisible(entry *mobileImage) {
	if entry == nil || len(entry.frames) < 2 || a.window == nil || a.closed.Load() {
		return
	}
	entry.animationMu.Lock()
	entry.animationVisible = true
	needsTimer := entry.animationTimer == nil
	entry.animationMu.Unlock()
	if needsTimer {
		a.scheduleImageAnimation(entry)
	}
}

func (a *App) scheduleImageAnimation(entry *mobileImage) {
	if entry == nil || len(entry.frames) < 2 || a.window == nil || a.closed.Load() {
		return
	}
	entry.animationMu.Lock()
	if !entry.animationVisible || entry.animationTimer != nil {
		entry.animationMu.Unlock()
		return
	}
	entry.animationGeneration++
	generation := entry.animationGeneration
	entry.animationTimer = time.AfterFunc(imageAnimationInterval, func() {
		entry.animationMu.Lock()
		if entry.animationGeneration != generation {
			entry.animationMu.Unlock()
			return
		}
		entry.animationTimer = nil
		visible := entry.animationVisible
		entry.animationMu.Unlock()
		if visible && !a.closed.Load() && a.window != nil {
			a.window.Invalidate()
		}
	})
	entry.animationMu.Unlock()
}

func (a *App) stopImageAnimation(entry *mobileImage) {
	if entry == nil {
		return
	}
	entry.animationMu.Lock()
	entry.animationVisible = false
	entry.animationGeneration++
	timer := entry.animationTimer
	entry.animationTimer = nil
	entry.animationMu.Unlock()
	if timer != nil {
		timer.Stop()
	}
}

func (a *App) stopImageAnimations() {
	for _, entry := range a.images {
		a.stopImageAnimation(entry)
	}
}
