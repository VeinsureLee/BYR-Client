package desktop

import (
	"bytes"
	"context"
	"errors"
	"image"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/resources"
	"github.com/egoist/mygo/ui"
)

type imageEntry struct {
	source  ui.ImageSource
	player  *ui.VideoPlayer
	err     error
	loading bool
	w, h    float32
	seen    uint64
	playing bool
	cancel  context.CancelFunc
}

const maxVisiblePlayers = 4

func imageViewerURL(raw string, fullURLs ...string) string {
	if len(fullURLs) > 0 && fullURLs[0] != "" {
		return fullURLs[0]
	}
	return raw
}

func (a *App) clearImages() {
	a.mediaGeneration++
	for _, e := range a.images {
		if e.cancel != nil {
			e.cancel()
			e.cancel = nil
		}
		if e.player != nil {
			e.playing = false
			e.player.Close()
			e.player = nil
		}
		e.source = nil
		e.err = nil
		e.loading = false
		e.w, e.h = 0, 0
	}
	a.images = map[string]*imageEntry{}
}
func (a *App) image(c *ui.Context, raw string, w, h float32, click bool, fullURLs ...string) {
	e := a.images[raw]
	if e == nil {
		if len(a.images) > 160 {
			a.clearImages()
		}
		account := a.Core.SessionInfo()
		generation := a.mediaGeneration
		imageCtx, cancel := context.WithCancel(a.ctx)
		e = &imageEntry{loading: true, cancel: cancel}
		a.images[raw] = e
		go func() {
			var b []byte
			var err error
			if strings.HasPrefix(raw, "emoji:") {
				b, err = resources.Emoji.ReadFile("em/" + strings.TrimPrefix(raw, "emoji:"))
			} else {
				b, err = a.Core.FetchAsset(imageCtx, raw)
			}
			var source ui.ImageSource
			var decoder ui.VideoDecoder
			if err == nil {
				if cfg, _, ce := image.DecodeConfig(bytes.NewReader(b)); ce == nil && (cfg.Width > 12000 || cfg.Height > 12000 || int64(cfg.Width)*int64(cfg.Height) > 24000000) {
					err = errors.New("图片尺寸过大")
				}
			}
			if err == nil {
				source, err = ui.DecodeImage(b)
				if len(b) > 6 && (string(b[:6]) == "GIF89a" || string(b[:6]) == "GIF87a") {
					// A GIF can still be shown as a static image when animation
					// support is unavailable, so keep the decoder optional.
					if candidate, decodeErr := ui.NewGIFDecoder(b); decodeErr == nil {
						decoder = candidate
					}
				}
			}
			a.update(func() {
				if a.closed || generation != a.mediaGeneration ||
					a.Core.SessionInfo().Generation != account.Generation || a.images[raw] != e {
					cancel()
					return
				}
				e.cancel = nil
				cancel()
				e.source, e.err, e.loading = source, err, false
				e.w, e.h = ui.ImageSize(source)
				if decoder != nil {
					// Falling back to the decoded still image is preferable to
					// failing the whole attachment if a player cannot be created.
					if player, playerErr := ui.NewVideoPlayer(decoder); playerErr == nil && player != nil {
						e.player = player
						e.player.SetLoop(true)
					}
				}
			})
		}()
	}
	if e.loading {
		ui.Text(c, "图片加载中…").TextColor(c.Theme().TextMuted)
		return
	}
	if e.err != nil {
		ui.Text(c, "[图片加载失败]").TextColor(c.Theme().TextMuted)
		if ui.Button(c, "重载图片").Clicked() {
			delete(a.images, raw)
		}
		return
	}
	if h == 0 {
		h = w * e.h / max(1, e.w)
		if h > 420 {
			w *= 420 / h
			h = 420
		}
	}
	var element ui.Element
	if e.player != nil {
		a.showPlayer(e)
		element = ui.Video(c, e.player).Size(w, h)
	} else {
		element = ui.Image(c, e.source).Size(w, h)
	}
	if click && element.Clicked() {
		a.imageView = imageViewerURL(raw, fullURLs...)
		a.imageScale = 1
	}
}

// showPlayer keeps animation work bounded: only players drawn this frame keep
// running, and the reduced-motion preference or the visible-player budget pauses
// the rest. commitImages pauses players that were drawn last frame but not this
// one, so leaving a thread stops its GIF redraws.
func (a *App) showPlayer(e *imageEntry) {
	if e == nil || e.player == nil {
		return
	}
	e.seen = a.frameSerial
	if a.reduceMotion || a.visiblePlayers >= maxVisiblePlayers {
		if e.playing {
			e.player.Pause()
			e.playing = false
		}
		return
	}
	a.visiblePlayers++
	if !e.playing {
		e.player.Play()
		e.playing = true
	}
}

func (a *App) commitImages() {
	for _, e := range a.images {
		if e.player == nil || !e.playing || e.seen == a.frameSerial {
			continue
		}
		e.player.Pause()
		e.playing = false
	}
}

func (a *App) bodyPartsView(c *ui.Context, parts []content.Part) {
	var spans []ui.Span
	flush := func() {
		if len(spans) > 0 {
			ui.RichText(c, spans...).FillWidth().Selectable().
				FontSize(a.textSize(c, 16)).FixedLineHeight(a.textSize(c, 26))
			spans = nil
		}
	}
	for _, p := range parts {
		switch p.Kind {
		case content.Text:
			s := ui.Span{Text: p.Text, Italic: p.Style.Italic, Underline: p.Style.Underline, Size: a.textSize(c, 16)}
			if p.Style.Bold {
				s.Weight = 600
			}
			if p.Style.Quote {
				s.Color = c.Theme().TextMuted
			}
			spans = append(spans, s)
		case content.Emoji:
			flush()
			a.image(c, "emoji:"+p.URL, 24, 24, false)
		case content.Image:
			flush()
			a.inlineImage(c, p.URL, p.FullURL)
		case content.File:
			flush()
			full := p.FullURL
			if full == "" {
				full = p.URL
			}
			a.button(c, "下载附件："+p.Text, func() { a.download(p.Text, full) })
		case content.Link:
			flush()
			raw := p.URL
			a.button(c, p.Text, func() {
				a.run("打开链接", func(ctx context.Context) (func(), error) { return nil, a.device.Open(raw) })
			})
		}
	}
	flush()
	if len(parts) == 0 {
		ui.Text(c, "(无内容)")
	}
}

// inlineImage reserves a fixed box before decoding and keeps that box for the
// final image, so loading a picture never changes the height of its floor.
func (a *App) inlineImage(c *ui.Context, raw, full string) {
	const boxWidth, boxHeight = float32(520), float32(300)
	ui.Column(c).FillWidth().Height(boxHeight).Center().Gap(6).
		Background(c.Theme().Background).Radius(12).Children(func() {
		e := a.images[raw]
		if e == nil || e.loading || e.err != nil || e.source == nil {
			a.image(c, raw, boxWidth, boxHeight, true, full)
			return
		}
		scale := min(boxWidth/e.w, boxHeight/e.h)
		width, height := e.w*scale, e.h*scale
		var element ui.Element
		if e.player != nil {
			a.showPlayer(e)
			element = ui.Video(c, e.player).Size(width, height)
		} else {
			element = ui.Image(c, e.source).Size(width, height)
		}
		if element.Clicked() {
			a.imageView = imageViewerURL(raw, full)
			a.imageScale = 1
		}
	})
}
func (a *App) bodyView(c *ui.Context, body string, attachments []forum.Attachment) {
	parts := content.Parse(body, attachments)
	var spans []ui.Span
	flush := func() {
		if len(spans) > 0 {
			ui.RichText(c, spans...).FillWidth().Selectable()
			spans = nil
		}
	}
	for _, p := range parts {
		switch p.Kind {
		case content.Text:
			s := ui.Span{Text: p.Text, Italic: p.Style.Italic, Underline: p.Style.Underline}
			if p.Style.Bold {
				s.Weight = 700
			}
			if p.Style.Quote {
				s.Color = c.Theme().TextMuted
			}
			spans = append(spans, s)
		case content.Emoji:
			flush()
			a.image(c, "emoji:"+p.URL, 24, 24, false)
		case content.Image:
			flush()
			a.image(c, p.URL, 520, 0, true, p.FullURL)

		case content.File:
			flush()
			full := p.FullURL
			if full == "" {
				full = p.URL
			}
			a.button(c, "下载附件："+p.Text, func() { a.download(p.Text, full) })
		case content.Link:
			flush()
			raw := p.URL
			a.button(c, p.Text, func() {
				a.run("打开链接", func(ctx context.Context) (func(), error) { return nil, a.device.Open(raw) })
			})
		}
	}
	flush()
	if len(parts) == 0 {
		ui.Text(c, "(无内容)")
	}
}
func (a *App) download(name, raw string) {
	a.run("下载附件", func(ctx context.Context) (func(), error) {
		b, err := a.Core.FetchAsset(ctx, raw)
		if err != nil {
			return nil, err
		}
		err = a.device.Save(name, b)
		return func() { a.notice("附件下载完成") }, err
	})
}
func (a *App) imageModal(c *ui.Context) {
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(850).Height(620).Gap(8).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "缩小").Clicked() {
					a.imageScale = max(.25, a.imageScale/1.25)
				}
				if ui.Button(c, "放大").Clicked() {
					a.imageScale = min(4, a.imageScale*1.25)
				}
				a.button(c, "保存原图", func() { a.download("image", a.imageView) })
				if ui.Button(c, "关闭图片").Clicked() {
					a.imageView = ""
				}
			})
			ui.ScrollBoth(c).FillWidth().Grow(1).Children(func() { a.image(c, a.imageView, 780*a.imageScale, 580*a.imageScale, false) })
		})
	})
	if !open {
		a.imageView = ""
	}
}
