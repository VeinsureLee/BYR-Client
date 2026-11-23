package desktop

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDesktopThreadRefreshErrorDoesNotCoverLastFloorLike(t *testing.T) {
	for _, size := range [][2]int{{720, 480}, {1100, 760}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			h := newHarness(t)
			h.login()
			const endpoint = "/open/threads/Go/fixture.json"
			const likeRequest = "GET /open/article/Go/like/last-floor.json"
			const likeLabel = "赞 3141"

			thread := desktopLongThread(30)
			thread.Articles[29].ID = "last-floor"
			thread.Articles[29].LikeSum = 3141
			data, err := json.Marshal(thread)
			if err != nil {
				t.Fatal(err)
			}
			h.mu.Lock()
			h.overrides = map[string]string{endpoint: string(data)}
			h.mu.Unlock()
			h.goTo(Route{Page: "thread", Name: "Go", ID: "fixture", Number: 1})
			h.view.SetSize(size[0], size[1])
			state := h.app.desktopListState(h.app.threadListKey())
			state.ScrollToEnd()
			h.view.Frame()
			if _, ok := h.view.Find(likeLabel); !ok || !state.AtEnd() {
				t.Fatalf("invalid end-of-thread fixture: action=%t atEnd=%t", ok, state.AtEnd())
			}

			h.mu.Lock()
			h.overrides[endpoint] = "{invalid"
			h.mu.Unlock()
			h.click("刷新")
			if !h.app.failed || h.app.busy || !h.view.HasText("关闭提示") {
				t.Fatalf("refresh error not visible: failed=%t busy=%t", h.app.failed, h.app.busy)
			}
			state.ScrollToEnd()
			h.view.Frame()
			if _, ok := h.view.Find(likeLabel); !ok || !state.AtEnd() || !h.view.HasText("关闭提示") {
				t.Fatalf("last floor or refresh error disappeared: action=%t atEnd=%t", ok, state.AtEnd())
			}

			before := h.count(likeRequest)
			if err := h.view.Click(likeLabel); err != nil {
				t.Fatalf("last-floor like was blocked while refresh error was shown: %v", err)
			}
			h.idle()
			if got := h.count(likeRequest); got != before+1 {
				t.Fatalf("last-floor like requests = %d, want exactly one additional request from %d", got, before)
			}
			t.Logf("%dx%d last-floor like requests: %d -> %d", size[0], size[1], before, h.count(likeRequest))
		})
	}
}
