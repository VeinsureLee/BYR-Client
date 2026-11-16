package ui

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

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

type mobileImage struct {
	loading bool
	err     error
	img     image.Image
	w, h    int
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
		var img image.Image
		var err error
		var b []byte
		switch {
		case strings.HasPrefix(raw, "emoji:"):
			b, err = resources.Emoji.ReadFile("em/" + strings.TrimPrefix(raw, "emoji:"))
		case a.core == nil:
			err = contentError("客户端尚未启动")
		default:
			b, err = a.core.Forum.FetchAsset(a.ctx, raw)
		}
		if err == nil {
			cfg, _, ce := image.DecodeConfig(bytes.NewReader(b))
			if ce != nil {
				err = ce
			} else if cfg.Width > 12000 || cfg.Height > 12000 || cfg.Width*cfg.Height > 24000000 {
				err = contentError("图片尺寸过大")
			} else {
				img, _, err = image.Decode(bytes.NewReader(b))
			}
		}
		a.post(func() {
			e.loading = false
			e.err = err
			if img != nil {
				e.img = img
				e.w = img.Bounds().Dx()
				e.h = img.Bounds().Dy()
			}
		})
	}()
	return e
}

type contentError string

func (e contentError) Error() string { return string(e) }

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
	stack := clip.Ellipse(image.Rectangle{Max: image.Pt(px, px)}).Push(gtx.Ops)
	defer stack.Pop()
	img := widget.Image{
		Src:      paint.NewImageOp(entry.img),
		Fit:      widget.Cover,
		Position: layout.Center,
	}
	return img.Layout(gtx)
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
			widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
				return a.inlineImage(gtx, raw)
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
	return widget.Image{Src: paint.NewImageOp(entry.img), Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
}

func (a *App) inlineImage(gtx layout.Context, raw string) layout.Dimensions {
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
	maxH := gtx.Dp(unit.Dp(360))
	h := maxW * e.h / e.w
	if h > maxH {
		maxW = maxH * e.w / e.h
		h = maxH
	}
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max = image.Pt(maxW, h)
	img := widget.Image{Src: paint.NewImageOp(e.img), Fit: widget.Contain, Position: layout.Center}
	return img.Layout(gtx)
}
