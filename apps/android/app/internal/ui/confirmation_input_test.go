package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

type confirmationFixture struct {
	app    *App
	router input.Router
	gtx    layout.Context
}

func newConfirmationFixture(t *testing.T, a *App) *confirmationFixture {
	t.Helper()
	f := &confirmationFixture{app: a}
	f.gtx = layout.Context{
		Ops:         &op.Ops{},
		Source:      f.router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	f.frame()
	return f
}

func (f *confirmationFixture) frame() {
	f.gtx.Ops.Reset()
	f.app.Layout(f.gtx)
	f.router.Frame(f.gtx.Ops)
}

func (f *confirmationFixture) tap(x, y float32) {
	f.router.Queue(
		pointer.Event{Source: pointer.Touch, Kind: pointer.Press, PointerID: 1, Position: f32.Pt(x, y)},
		pointer.Event{Source: pointer.Touch, Kind: pointer.Release, PointerID: 1, Position: f32.Pt(x, y)},
	)
	f.frame()
}

func TestConfirmationBlocksBackgroundInputAndHandlesCancelConfirm(t *testing.T) {
	server := newInteractionServer(t)
	backend := &toggleHistoryBackend{}
	seedMobileHistory(t, backend, "alice", "42")
	seedMobileHistory(t, backend, "bob", "bob-old")
	a := newHistoryPageApp(t, server.server.URL, backend)
	f := newConfirmationFixture(t, a)

	a.confirmClearHistory()
	f.frame()
	f.frame()
	if a.confirmation == nil {
		t.Fatal("confirmation did not open")
	}

	// The bottom navigation's search entry sits here at 420x820. A touch on
	// the dimmed page must not switch routes while the dialog is open.
	f.tap(126, 782)
	if a.current().Page != PageHistory {
		t.Fatalf("tap behind the dialog navigated to %v", a.current().Page)
	}
	if a.confirmation == nil {
		t.Fatal("tap behind the dialog dismissed it")
	}
	if got := a.core.RecentHistory(); len(got) != 1 || got[0].ID != "42" {
		t.Fatalf("tap behind the dialog changed history: %+v", got)
	}

	// System back closes the dialog without clearing the current account.
	f.router.Queue(key.Event{Name: key.NameBack, State: key.Press})
	f.frame()
	if a.confirmation != nil {
		t.Fatal("system back did not close the confirmation")
	}
	if got := a.core.RecentHistory(); len(got) != 1 {
		t.Fatalf("system back cleared history: %+v", got)
	}

	f.gtx.Metric.PxPerSp = 1.5
	f.frame()
	a.confirmClearHistory()
	f.frame()
	if a.confirmation == nil {
		t.Fatal("confirmation did not reopen at a large font size")
	}
	a.click("confirmation-cancel").Click()
	f.frame()
	if a.confirmation != nil {
		t.Fatal("large-font cancel did not close the dialog")
	}
	if got := a.core.RecentHistory(); len(got) != 1 {
		t.Fatalf("large-font cancel cleared history: %+v", got)
	}

	a.confirmClearHistory()
	f.frame()
	a.click("confirmation-accept").Click()
	f.frame()
	waitForIdle(t, a)
	if a.confirmation != nil {
		t.Fatal("large-font confirm left the dialog open")
	}
	if got := a.core.RecentHistory(); len(got) != 0 {
		t.Fatalf("confirmed clear left Alice history: %+v", got)
	}
	if got := storedMobileHistory(t, backend, "bob"); len(got) != 1 || got[0].ID != "bob-old" {
		t.Fatalf("confirmed clear changed Bob history: %+v", got)
	}
}

func TestConfirmationAcceptButtonReceivesTouchThroughTheBlocker(t *testing.T) {
	server := newInteractionServer(t)
	backend := &toggleHistoryBackend{}
	seedMobileHistory(t, backend, "alice", "42")
	a := newHistoryPageApp(t, server.server.URL, backend)
	f := newConfirmationFixture(t, a)
	a.confirmClearHistory()
	f.frame()
	f.frame()

	img := renderAppRGBA(t, a, 420, 820)
	minX, minY, maxX, maxY := 420, 820, -1, -1
	for y := 0; y < 820; y++ {
		for x := 0; x < 420; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == colorAccent.R && uint8(g>>8) == colorAccent.G && uint8(b>>8) == colorAccent.B {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < 0 {
		t.Fatal("confirmation accept button was not painted")
	}
	f.tap(float32(minX+maxX)/2, float32(minY+maxY)/2)
	waitForIdle(t, a)
	if a.confirmation != nil {
		t.Fatal("touch on the accept button did not close the dialog")
	}
	if got := a.core.RecentHistory(); len(got) != 0 {
		t.Fatalf("accept button did not clear history: %+v", got)
	}
}
