package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func renderAppRGBA(t *testing.T, a *App, width, height int) *image.RGBA {
	t.Helper()
	window, err := headless.NewWindow(width, height)
	if err != nil {
		t.Skipf("headless renderer unavailable: %v", err)
	}
	defer window.Release()
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(width, height)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Now:         time.Now(),
	}
	a.Layout(gtx)
	if err := window.Frame(&ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if err := window.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return img
}

func emptyContentBounds(t *testing.T, img *image.RGBA, bg color.NRGBA, top, bottom int) image.Rectangle {
	t.Helper()
	bounds := image.Rectangle{Min: image.Pt(img.Bounds().Dx(), img.Bounds().Dy()), Max: image.Pt(0, 0)}
	for y := top; y < bottom && y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == bg.R && uint8(g>>8) == bg.G && uint8(b>>8) == bg.B {
				continue
			}
			if x < bounds.Min.X {
				bounds.Min.X = x
			}
			if y < bounds.Min.Y {
				bounds.Min.Y = y
			}
			if x > bounds.Max.X {
				bounds.Max.X = x
			}
			if y > bounds.Max.Y {
				bounds.Max.Y = y
			}
		}
	}
	if bounds.Max.X <= bounds.Min.X {
		t.Fatalf("empty state painted nothing between y=%d and y=%d", top, bottom)
	}
	return bounds
}

func TestEmptyStatesCenterInTheActualContentViewport(t *testing.T) {
	for _, width := range []int{320, 360, 420} {
		height := 700
		t.Run(itoa(width), func(t *testing.T) {
			cases := []struct {
				name string
				app  func(t *testing.T) *App
			}{
				{
					name: "search initial",
					app: func(t *testing.T) *App {
						a := previewApp(t)
						a.stack = []Route{{Page: PageSearch}}
						a.searchMode = "all"
						return a
					},
				},
				{
					name: "search empty result",
					app: func(t *testing.T) *App {
						a := previewApp(t)
						a.stack = []Route{{Page: PageSearch}}
						a.searchMode = "all"
						a.searchEdit.SetText("needle")
						a.searchListQuery = "needle"
						a.searchHasRun = true
						return a
					},
				},
				{
					name: "history empty",
					app: func(t *testing.T) *App {
						a := previewAppWithCore(t)
						a.stack = []Route{{Page: PageHistory}}
						return a
					},
				},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					a := tc.app(t)
					defer a.Close()
					img := renderAppRGBA(t, a, width, height)
					// The band excludes the search form, top bar, and bottom
					// navigation so only the empty block is measured.
					bounds := emptyContentBounds(t, img, a.theme.Bg, 300, height-80)
					iconBounds := emptyContentBounds(t, img, a.theme.Bg, bounds.Min.Y, bounds.Min.Y+34)
					// The title's last glyph advances past its ink (especially
					// full-width punctuation in the detail line), so measure the
					// icon and the title's own line rather than the union.
					titleBounds := emptyContentBounds(t, img, a.theme.Bg, bounds.Min.Y+40, bounds.Min.Y+66)
					want := float32(width) / 2
					for _, part := range []struct {
						name   string
						bounds image.Rectangle
					}{{"icon", iconBounds}, {"title", titleBounds}} {
						center := float32(part.bounds.Min.X+part.bounds.Max.X) / 2
						if diff := center - want; diff < -2 || diff > 2 {
							t.Fatalf("%s x-center = %.1f, want %.1f (bounds %v)", part.name, center, want, part.bounds)
						}
					}
				})
			}
		})
	}
}
