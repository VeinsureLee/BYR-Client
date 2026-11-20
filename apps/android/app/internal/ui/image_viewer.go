package ui

import (
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	minImageScale float32 = 0.25
	maxImageScale float32 = 4
)

// openImage keeps the thumbnail and the requested original separate. The
// viewer always downloads through imageEntry, which in turn uses
// forum.FetchAsset for URL validation and credential scoping.
func (a *App) openImage(raw, fullURL string) {
	if raw == "" && fullURL == "" {
		return
	}
	if fullURL == "" {
		fullURL = raw
	}
	a.imageViewerRaw = raw
	a.imageViewer = fullURL
	a.imageScale = 1
	a.imagePan = image.Point{}
	a.imageGesture.reset()
	if a.window != nil {
		a.window.Invalidate()
	}
}

func (a *App) closeImageViewer() {
	wasOpen := a.imageViewer != "" || a.imageViewerRaw != ""
	a.imageViewer = ""
	a.imageViewerRaw = ""
	a.imageScale = 1
	a.imagePan = image.Point{}
	a.imageGesture.reset()
	// The viewer replaces the page for a frame, so no image should keep an
	// animation timer alive merely because it was visible before opening it.
	a.stopImageAnimations()
	if wasOpen && a.window != nil {
		a.window.Invalidate()
	}
}

func (a *App) zoomImage(factor float32) {
	if factor <= 0 {
		return
	}
	if a.imageScale == 0 {
		a.imageScale = 1
	}
	a.imageScale *= factor
	a.imageScale = clampImageScale(a.imageScale)
	if a.imageScale <= 1 {
		a.imagePan = image.Point{}
	}
	if a.window != nil {
		a.window.Invalidate()
	}
}

func (a *App) imageViewerPage(gtx layout.Context) layout.Dimensions {
	return a.imageViewerLayout(gtx)
}

func (a *App) imageViewerLayout(gtx layout.Context) layout.Dimensions {
	if a.imageViewer == "" {
		return layout.Dimensions{}
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
			paint.Fill(gtx.Ops, color.NRGBA{R: 0x08, G: 0x0C, B: 0x14, A: 0xF5})
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(a.imageViewerToolbar),
				layout.Flexed(1, a.imageViewerCanvas),
				layout.Rigid(a.imageViewerFooter),
			)
		}),
	)
}

func (a *App) imageViewerToolbar(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				dims, clicked := a.iconButton(gtx, "image-viewer-close", "close")
				if clicked {
					a.closeImageViewer()
				}
				return dims
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.theme, unit.Sp(16), "图片")
				l.Color = color.NRGBA{R: 0xF2, G: 0xF5, B: 0xFA, A: 0xFF}
				l.Alignment = text.Middle
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := a.click("image-viewer-save")
						clicked := btn.Clicked(gtx)
						dims := material.Button(a.theme, btn, "保存").Layout(gtx)
						if clicked {
							a.saveImageViewer()
						}
						return dims
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.imageViewerAction(gtx, "image-viewer-zoom-out", "−", 0.8)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.imageViewerAction(gtx, "image-viewer-zoom-in", "+", 1.25)
			}),
		)
	})
}

func (a *App) imageViewerAction(gtx layout.Context, key, label string, factor float32) layout.Dimensions {
	btn := a.click(key)
	clicked := btn.Clicked(gtx)
	dims := material.Button(a.theme, btn, label).Layout(gtx)
	if clicked {
		a.zoomImage(factor)
	}
	return dims
}

func (a *App) imageViewerFooter(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Caption(a.theme, "缩放 "+formatImageScale(a.imageScale))
		l.Color = color.NRGBA{R: 0xB9, G: 0xC3, B: 0xD4, A: 0xFF}
		return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(12)}.Layout(gtx, l.Layout)
	})
}

func formatImageScale(scale float32) string {
	if scale == 0 {
		scale = 1
	}
	return imageScaleText(scale)
}

func imageScaleText(scale float32) string {
	// Keep the viewer dependency-free; integer percentages are easier to scan
	// than a floating-point value on a narrow phone footer.
	return itoa(int(scale*100)) + "%"
}

func (a *App) imageViewerCanvas(gtx layout.Context) layout.Dimensions {
	if a.imageViewer == "" {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	entry := a.imageEntry(a.imageViewer)
	if entry.loading {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(a.theme, "图片加载中…")
			l.Color = color.NRGBA{R: 0xB9, G: 0xC3, B: 0xD4, A: 0xFF}
			return l.Layout(gtx)
		})
	}
	if entry.err != nil || entry.img == nil {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(a.theme, "[图片加载失败]")
			l.Color = color.NRGBA{R: 0xF0, G: 0x8A, B: 0x82, A: 0xFF}
			return l.Layout(gtx)
		})
	}
	img := entry.frameAt(imageNow(gtx))
	if img == nil {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	a.markImageVisible(entry)
	available := gtx.Constraints.Max
	a.imageViewerPointers(gtx, available)
	size := viewerImageSize(entry, available, a.imageScale)
	a.imagePan = clampImagePan(a.imagePan, size, available)
	if size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: available}
	}
	viewport := clip.Rect(image.Rectangle{Max: available}).Push(gtx.Ops)
	defer viewport.Pop()
	offset := op.Offset(image.Pt((available.X-size.X)/2+a.imagePan.X, (available.Y-size.Y)/2+a.imagePan.Y)).Push(gtx.Ops)
	defer offset.Pop()
	imageContext := gtx
	imageContext.Constraints = layout.Exact(size)
	widget.Image{Src: paint.NewImageOp(img), Fit: widget.Contain, Position: layout.Center}.Layout(imageContext)
	return layout.Dimensions{Size: available}
}

func (a *App) imageViewerPointers(gtx layout.Context, viewport image.Point) {
	area := clip.Rect(image.Rectangle{Max: viewport}).Push(gtx.Ops)
	event.Op(gtx.Ops, &a.imageViewerTag)
	area.Pop()
	for {
		e, ok := gtx.Source.Event(pointer.Filter{
			Target: &a.imageViewerTag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Move | pointer.Release | pointer.Cancel,
		})
		if !ok {
			return
		}
		pe, ok := e.(pointer.Event)
		if !ok {
			continue
		}
		a.imageScale, a.imagePan = a.imageGesture.apply(pe, a.imageScale, a.imagePan)
	}
}

func viewerImageSize(entry *mobileImage, available image.Point, scale float32) image.Point {
	if entry == nil || entry.w <= 0 || entry.h <= 0 || available.X <= 0 || available.Y <= 0 {
		return image.Point{}
	}
	if scale <= 0 {
		scale = 1
	}
	fit := minFloat(float64(available.X)/float64(entry.w), float64(available.Y)/float64(entry.h))
	width := int(float64(entry.w) * fit * float64(scale))
	height := int(float64(entry.h) * fit * float64(scale))
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	return image.Pt(width, height)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
