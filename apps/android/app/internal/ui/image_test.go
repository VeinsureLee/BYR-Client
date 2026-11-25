package ui

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestAndroidInlineImageFitsAvailableWidth(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.images["thumb"] = &mobileImage{
		img: image.NewRGBA(image.Rect(0, 0, 100, 50)),
		w:   100,
		h:   50,
	}

	var router input.Router
	var ops op.Ops
	dims := a.inlineImage(layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(300, 500)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}, "thumb")
	if dims.Size != image.Pt(300, 150) {
		t.Fatalf("inline image dimensions = %v, want 300x150", dims.Size)
	}
}

func TestAndroidInlineImageUsesAvailableWidthForTallImages(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.images["tall"] = &mobileImage{
		img: image.NewRGBA(image.Rect(0, 0, 100, 1000)),
		w:   100,
		h:   1000,
	}

	var router input.Router
	var ops op.Ops
	dims := a.inlineImage(layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(300, 120)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}, "tall")
	if dims.Size.Y != 480 || dims.Size.X >= 300 {
		t.Fatalf("tall inline image dimensions = %v, want height-capped 480", dims.Size)
	}
}

func TestAndroidGIFFrameChanges(t *testing.T) {
	data := testGIF(t, []color.NRGBA{
		{R: 0xE0, A: 0xFF},
		{B: 0xE0, A: 0xFF},
	}, []int{5, 5})
	entry, err := decodeMobileImage(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.frames) != 2 || len(entry.delays) != 2 {
		t.Fatalf("decoded GIF = %d frames/%d delays, want 2/2", len(entry.frames), len(entry.delays))
	}
	start := entry.started
	first := entry.frameAt(start)
	second := entry.frameAt(start.Add(entry.delays[0]))
	if colorAt(first) == colorAt(second) {
		t.Fatalf("GIF frame did not change: first=%v second=%v", colorAt(first), colorAt(second))
	}
}

func TestAndroidGIFRejectsUnboundedFrameCount(t *testing.T) {
	colors := make([]color.NRGBA, 61)
	for i := range colors {
		colors[i] = color.NRGBA{R: uint8(i), A: 0xFF}
	}
	_, err := decodeMobileImage(testGIF(t, colors, nil))
	if err == nil {
		t.Fatal("GIF with more than 60 frames was accepted")
	}
}

func TestAndroidImageViewerUsesFullURL(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	const thumbnail = "https://cdn.example.test/thumb.png"
	const original = "https://bbs.example.test/original.png"
	a.images[thumbnail] = &mobileImage{
		img: image.NewRGBA(image.Rect(0, 0, 100, 50)),
		w:   100,
		h:   50,
	}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(300, 500)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	dims := a.inlineImage(gtx, thumbnail, original)
	if dims.Size.X == 0 || dims.Size.Y == 0 {
		t.Fatalf("thumbnail has no click target: %v", dims.Size)
	}
	router.Frame(&ops)
	a.click("image:" + thumbnail).Click()
	ops.Reset()
	gtx.Ops = &ops
	a.inlineImage(gtx, thumbnail, original)
	if a.imageViewer != original {
		t.Fatalf("viewer URL = %q, want %q", a.imageViewer, original)
	}
	if a.imageViewerRaw != thumbnail {
		t.Fatalf("viewer thumbnail = %q, want %q", a.imageViewerRaw, thumbnail)
	}
}

func TestAndroidImageViewerStateCanZoomAndClose(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	a.openImage("thumb", "original")
	if a.imageViewer != "original" || a.imageScale != 1 {
		t.Fatalf("opened viewer state = url:%q scale:%v", a.imageViewer, a.imageScale)
	}
	a.zoomImage(2)
	if a.imageScale != 2 {
		t.Fatalf("zoom scale = %v, want 2", a.imageScale)
	}
	a.zoomImage(0.01)
	if a.imageScale != 0.25 {
		t.Fatalf("minimum zoom scale = %v, want 0.25", a.imageScale)
	}
	a.closeImageViewer()
	if a.imageViewer != "" || a.imageViewerRaw != "" || a.imageScale != 1 {
		t.Fatalf("closed viewer state = url:%q raw:%q scale:%v", a.imageViewer, a.imageViewerRaw, a.imageScale)
	}
}

func TestAndroidFailedImageStillRendersFallback(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.images["broken"] = &mobileImage{err: errors.New("broken image")}
	var router input.Router
	var ops op.Ops
	dims := a.inlineImage(layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(300, 500)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}, "broken")
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("failed image rendered no fallback: %v", dims.Size)
	}
}

func TestClearImageCacheCancelsRequestsStopsGIFAndRejectsLateResult(t *testing.T) {
	started := make(chan struct{})
	requestCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(requestCanceled)
	}))
	defer server.Close()

	a := interactionApp(t, server.URL)
	defer a.Close()
	entry := a.imageEntry(server.URL + "/late.png")
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("image request did not start")
	}
	if entry.cancel == nil {
		t.Fatal("pending image request has no cancellation handle")
	}
	gifEntry := &mobileImage{
		raw: []byte("cached GIF"), img: image.NewRGBA(image.Rect(0, 0, 1, 1)),
		frames: []image.Image{image.NewRGBA(image.Rect(0, 0, 1, 1))},
	}
	a.images["cached-gif"] = gifEntry
	a.visibleAnimations = []*mobileImage{gifEntry}
	a.animationFrame = []*mobileImage{gifEntry}
	a.animationTimer = time.NewTimer(time.Hour)
	mediaGeneration := a.mediaGeneration
	cacheGeneration := a.imageCacheGeneration

	a.clearImageCache()
	if a.mediaGeneration != mediaGeneration+1 || a.imageCacheGeneration != cacheGeneration+1 || len(a.images) != 0 {
		t.Fatalf("clear generations/cache = media:%d cache:%d entries:%d", a.mediaGeneration, a.imageCacheGeneration, len(a.images))
	}
	if entry.cancel != nil || gifEntry.raw != nil || gifEntry.img != nil || len(gifEntry.frames) != 0 ||
		len(a.visibleAnimations) != 0 || len(a.animationFrame) != 0 || a.animationTimer != nil {
		t.Fatalf("media resources survived clear: pending=%+v gif=%+v visible=%d frame=%d timer=%v",
			entry, gifEntry, len(a.visibleAnimations), len(a.animationFrame), a.animationTimer)
	}
	select {
	case <-requestCanceled:
	case <-time.After(3 * time.Second):
		t.Fatal("clearing the image cache did not cancel the HTTP request")
	}
	select {
	case update := <-a.updates:
		update()
	case <-time.After(3 * time.Second):
		t.Fatal("late image result did not return to the UI queue")
	}
	if len(a.images) != 0 || !entry.loading || entry.err != nil || entry.raw != nil || entry.img != nil {
		t.Fatalf("late result repopulated cleared cache: entries=%d old-entry=%+v", len(a.images), entry)
	}
}

func testGIF(t *testing.T, colors []color.NRGBA, delays []int) []byte {
	t.Helper()
	frames := make([]*image.Paletted, len(colors))
	for i, c := range colors {
		frames[i] = image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{c, color.NRGBA{A: 0xFF}})
		for p := range frames[i].Pix {
			frames[i].Pix[p] = 0
		}
	}
	if delays == nil {
		delays = make([]int, len(frames))
		for i := range delays {
			delays[i] = 1
		}
	}
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, &gif.GIF{Image: frames, Delay: delays, LoopCount: 0}); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func colorAt(img image.Image) color.Color {
	if img == nil {
		return nil
	}
	return img.At(img.Bounds().Min.X, img.Bounds().Min.Y)
}
