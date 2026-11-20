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
}

func imageViewerURL(raw string, fullURLs ...string) string {
	if len(fullURLs) > 0 && fullURLs[0] != "" {
		return fullURLs[0]
	}
	return raw
}

func (a *App) clearImages() {
	for _, e := range a.images {
		if e.player != nil {
			e.player.Close()
		}
	}
	a.images = map[string]*imageEntry{}
}
func (a *App) image(c *ui.Context, raw string, w, h float32, click bool, fullURLs ...string) {
	e := a.images[raw]
	if e == nil {
		if len(a.images) > 160 {
			a.clearImages()
		}
		e = &imageEntry{loading: true}
		a.images[raw] = e
		account := a.Core.Session()
		go func() {
			var b []byte
			var err error
			if strings.HasPrefix(raw, "emoji:") {
				b, err = resources.Emoji.ReadFile("em/" + strings.TrimPrefix(raw, "emoji:"))
			} else {
				b, err = a.Core.Forum.FetchAsset(a.ctx, raw)
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
				if a.closed || a.Core.Session() != account || a.images[raw] != e {
					return
				}
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
		element = ui.Video(c, e.player).Size(w, h)
	} else {
		element = ui.Image(c, e.source).Size(w, h)
	}
	if click && element.Clicked() {
		a.imageView = imageViewerURL(raw, fullURLs...)
		a.imageScale = 1
	}
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
		b, err := a.Core.Forum.FetchAsset(ctx, raw)
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
