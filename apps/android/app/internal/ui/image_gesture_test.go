package ui

import (
	"context"
	"image"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

type recordingImageSaver struct {
	name string
	data []byte
}

func (*recordingImageSaver) Invalidate() {}
func (s *recordingImageSaver) SaveImage(ctx context.Context, name string, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.name = name
	s.data = append([]byte(nil), data...)
	return filepath.Join("/Pictures/BYR-Client", name), nil
}

func TestImageGesturePinchZoomAndPan(t *testing.T) {
	var gesture imageGestureState
	scale, pan := float32(1), image.Point{}
	scale, pan = gesture.apply(pointer.Event{Kind: pointer.Press, PointerID: 1, Position: f32.Pt(50, 100)}, scale, pan)
	scale, pan = gesture.apply(pointer.Event{Kind: pointer.Press, PointerID: 2, Position: f32.Pt(150, 100)}, scale, pan)
	scale, pan = gesture.apply(pointer.Event{Kind: pointer.Drag, PointerID: 2, Position: f32.Pt(250, 100)}, scale, pan)
	if scale != 2 {
		t.Fatalf("pinch scale = %v, want 2", scale)
	}
	gesture.apply(pointer.Event{Kind: pointer.Release, PointerID: 2, Position: f32.Pt(250, 100)}, scale, pan)
	scale, pan = gesture.apply(pointer.Event{Kind: pointer.Drag, PointerID: 1, Position: f32.Pt(75, 125)}, scale, pan)
	if scale != 2 || pan.X <= 0 || pan.Y <= 0 {
		t.Fatalf("post-pinch pan = scale:%v offset:%v, want zoomed image panned", scale, pan)
	}
}

func TestImageGestureClampsPinchScaleAndPan(t *testing.T) {
	var gesture imageGestureState
	scale, pan := float32(1), image.Point{}
	gesture.apply(pointer.Event{Kind: pointer.Press, PointerID: 1, Position: f32.Pt(0, 0)}, scale, pan)
	gesture.apply(pointer.Event{Kind: pointer.Press, PointerID: 2, Position: f32.Pt(100, 0)}, scale, pan)
	scale, pan = gesture.apply(pointer.Event{Kind: pointer.Drag, PointerID: 2, Position: f32.Pt(2000, 0)}, scale, pan)
	if scale != maxImageScale {
		t.Fatalf("maximum pinch scale = %v, want %v", scale, maxImageScale)
	}
	if got := clampImageScale(0.01); got != minImageScale {
		t.Fatalf("minimum pinch scale = %v, want %v", got, minImageScale)
	}
	if got, want := clampImagePan(image.Pt(500, -500), image.Pt(800, 400), image.Pt(300, 300)), image.Pt(250, -50); got != want {
		t.Fatalf("clamped pan = %v, want %v", got, want)
	}
	if got := clampImagePan(image.Pt(50, 50), image.Pt(300, 300), image.Pt(300, 300)); got != (image.Point{}) {
		t.Fatalf("fit image pan = %v, want zero", got)
	}
}

func TestImageSaveUsesOriginalEncodedBytes(t *testing.T) {
	a := previewApp(t)
	defer a.Close()
	saver := &recordingImageSaver{}
	a.window = saver
	const original = "https://bbs.example.test/images/post.png"
	data := []byte("\x89PNG\r\n\x1a\noriginal bytes")
	a.openImage("thumb", original)
	a.images[original] = &mobileImage{raw: append([]byte(nil), data...)}

	a.saveImageViewer()
	waitForIdle(t, a)
	if saver.name == "" || filepath.Ext(saver.name) != ".png" {
		t.Fatalf("saved name = %q, want PNG filename", saver.name)
	}
	if string(saver.data) != string(data) {
		t.Fatalf("saved bytes = %q, want original %q", saver.data, data)
	}
	if a.message != "图片已保存到："+filepath.Join("/Pictures/BYR-Client", saver.name) {
		t.Fatalf("save feedback = %q", a.message)
	}
}

func TestImageViewerHandlesPhysicalPinchAndDragEvents(t *testing.T) {
	a := previewApp(t)
	defer a.Close()
	a.images["original"] = &mobileImage{img: image.NewRGBA(image.Rect(0, 0, 100, 50)), w: 100, h: 50}
	a.openImage("thumb", "original")

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(320, 640)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.imageViewerLayout(gtx)
	router.Frame(&ops)
	router.Queue(
		pointer.Event{Source: pointer.Touch, Kind: pointer.Press, PointerID: 1, Position: f32.Pt(80, 240)},
		pointer.Event{Source: pointer.Touch, Kind: pointer.Press, PointerID: 2, Position: f32.Pt(160, 240)},
		pointer.Event{Source: pointer.Touch, Kind: pointer.Move, PointerID: 2, Position: f32.Pt(240, 240)},
		pointer.Event{Source: pointer.Touch, Kind: pointer.Release, PointerID: 2, Position: f32.Pt(240, 240)},
	)
	ops.Reset()
	gtx.Ops = &ops
	a.imageViewerLayout(gtx)
	if a.imageScale != 2 {
		t.Fatalf("physical pinch scale = %v, want 2", a.imageScale)
	}
	router.Frame(&ops)
	router.Queue(
		pointer.Event{Source: pointer.Touch, Kind: pointer.Move, PointerID: 1, Position: f32.Pt(120, 240)},
		pointer.Event{Source: pointer.Touch, Kind: pointer.Release, PointerID: 1, Position: f32.Pt(120, 240)},
	)
	ops.Reset()
	gtx.Ops = &ops
	a.imageViewerLayout(gtx)
	if a.imagePan.X <= 0 {
		t.Fatalf("physical drag did not pan zoomed image: %v", a.imagePan)
	}
}

func TestImageSaveFilenameIsStableAndKeepsOriginalFormat(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 34, 56, 123000000, time.UTC)
	got := imageSaveFilename("https://cdn.example.test/image", []byte("GIF89a"), now)
	if got != "BYR-20261120-123456.123.gif" {
		t.Fatalf("image filename = %q", got)
	}
}
