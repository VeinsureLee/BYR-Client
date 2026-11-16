package ui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func TestAvatarLoadsForumFaceImage(t *testing.T) {
	face := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	face.Set(0, 0, color.NRGBA{R: 0x0A, G: 0x5C, B: 0xBF, A: 0xFF})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, face); err != nil {
		t.Fatal(err)
	}
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/face/alice.png" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("oauth_token"); got != "token" {
			t.Errorf("avatar token = %q, want token", got)
		}
		select {
		case requested <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(encoded.Bytes())
	}))
	defer server.Close()

	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	credentials := &testCredentials{session: application.Credential{
		Username:  "alice",
		Token:     "token",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}}
	core, err := application.New(application.Options{
		BaseURL:     server.URL,
		HTTPClient:  server.Client(),
		Local:       store,
		Credentials: credentials,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.core = core
	entry := a.imageEntry("/face/alice.png")
	deadline := time.Now().Add(3 * time.Second)
	for entry.loading && time.Now().Before(deadline) {
		a.drain()
		time.Sleep(5 * time.Millisecond)
	}
	a.drain()
	if entry.loading || entry.err != nil {
		t.Fatalf("avatar did not load: loading=%v err=%v", entry.loading, entry.err)
	}
	if entry.img == nil || entry.w != 2 || entry.h != 2 {
		t.Fatalf("decoded avatar = img:%v size:%dx%d", entry.img != nil, entry.w, entry.h)
	}
	select {
	case <-requested:
	default:
		t.Fatal("avatar endpoint was not requested")
	}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(100, 100)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	dims := a.avatar(gtx, forum.User{ID: "alice", UserName: "Alice", FaceURL: "/face/alice.png"}, 40)
	if dims.Size != image.Pt(40, 40) {
		t.Fatalf("avatar dimensions = %v, want 40x40", dims.Size)
	}
}

func TestAvatarFallsBackToInitial(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(100, 100)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	dims := a.avatar(gtx, forum.User{ID: "alice", UserName: "Alice"}, 40)
	if dims.Size != image.Pt(40, 40) {
		t.Fatalf("fallback dimensions = %v, want 40x40", dims.Size)
	}
}
