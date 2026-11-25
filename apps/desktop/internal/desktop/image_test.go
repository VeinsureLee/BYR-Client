package desktop

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core"
	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/egoist/mygo/ui"
)

func TestImageViewerURLPrefersOriginal(t *testing.T) {
	const thumbnail = "https://cdn.example.test/thumb.jpg"
	const original = "https://bbs.example.test/original.jpg"
	if got := imageViewerURL(thumbnail, original); got != original {
		t.Fatalf("viewer URL = %q, want original %q", got, original)
	}
	if got := imageViewerURL(thumbnail, ""); got != thumbnail {
		t.Fatalf("viewer URL without full URL = %q, want thumbnail %q", got, thumbnail)
	}
	if got := imageViewerURL(thumbnail); got != thumbnail {
		t.Fatalf("viewer URL without full URL = %q, want thumbnail %q", got, thumbnail)
	}
}

func TestClearImagesCancelsRequestAndRejectsLateResult(t *testing.T) {
	started := make(chan struct{})
	requestCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(requestCanceled)
	}))
	defer server.Close()

	credentials := &testCredentials{session: application.Credential{Username: "alice", Token: "fixture-token"}}
	coreApp, err := core.Open(context.Background(), core.Options{
		BaseURL: server.URL, HTTPClient: server.Client(), Credentials: credentials,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := coreApp.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	updates := make(chan func(), 8)
	app := New(coreApp, &testDevice{}, func(fn func()) { updates <- fn })
	defer app.Close()

	const raw = "/assets/late.png"
	view := ui.NewTester(func(c *ui.Context) { app.image(c, raw, 64, 64, false) }, 120, 120)
	view.Frame()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("image request did not start")
	}
	entry := app.images[raw]
	if entry == nil || entry.cancel == nil {
		t.Fatalf("pending image entry = %+v, want a cancellable request", entry)
	}
	generation := app.mediaGeneration
	app.clearImages()
	if app.mediaGeneration != generation+1 || len(app.images) != 0 || entry.cancel != nil {
		t.Fatalf("cache clear state: generation=%d entries=%d cancel=%v", app.mediaGeneration, len(app.images), entry.cancel != nil)
	}
	select {
	case <-requestCanceled:
	case <-time.After(3 * time.Second):
		t.Fatal("clearing the image cache did not cancel the HTTP request")
	}

	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	select {
	case update := <-updates:
		update()
	case <-deadline.C:
		t.Fatal("late image result did not return to the UI queue")
	}
	if len(app.images) != 0 || entry.loading || entry.err != nil || entry.source != nil {
		t.Fatalf("late result repopulated cleared cache: entries=%d old-entry=%+v", len(app.images), entry)
	}
}
