package desktop

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDesktopGIFPlayersFollowVisibilityAndMotionPreference(t *testing.T) {
	frames := []*image.Paletted{
		image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.NRGBA{R: 0xE0, A: 0xFF}}),
		image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.NRGBA{B: 0xE0, A: 0xFF}}),
	}
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, &gif.GIF{Image: frames, Delay: []int{5, 5}, LoopCount: 0}); err != nil {
		t.Fatal(err)
	}
	decoder, err := ui.NewGIFDecoder(encoded.Bytes())
	if err != nil {
		t.Skipf("GIF decoder unavailable: %v", err)
	}
	player, err := ui.NewVideoPlayer(decoder)
	if err != nil || player == nil {
		t.Skipf("video player unavailable: %v", err)
	}
	defer player.Close()
	a := &App{images: map[string]*imageEntry{"gif": {player: player}}, frameSerial: 1}
	entry := a.images["gif"]
	a.showPlayer(entry)
	if !player.Playing() {
		t.Fatal("visible GIF did not start")
	}
	a.frameSerial = 2
	a.commitImages()
	if player.Playing() {
		t.Fatal("offscreen GIF kept playing after the next frame")
	}
	a.frameSerial = 3
	entry.seen = 0
	a.showPlayer(entry)
	if !player.Playing() {
		t.Fatal("returning GIF did not resume")
	}
	a.reduceMotion = true
	a.visiblePlayers = 0
	a.showPlayer(entry)
	if player.Playing() {
		t.Fatal("reduced motion did not pause the GIF")
	}
}
