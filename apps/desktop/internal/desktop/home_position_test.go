package desktop

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func TestDesktopHomeReturnRestoresSelectedSectionAndBoardScroll(t *testing.T) {
	for _, size := range [][2]int{{720, 480}, {1100, 760}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			h := newHarness(t)
			h.login()
			boards := make(forum.List[forum.Board], 24)
			for i := range boards {
				boards[i] = forum.Board{
					Name: fmt.Sprintf("B%02d", i), Description: fmt.Sprintf("Board %02d", i),
				}
			}
			section := forum.Section{Name: "1", Description: "院系", Parent: "0", Boards: boards}
			data, err := json.Marshal(section)
			if err != nil {
				t.Fatal(err)
			}
			h.mu.Lock()
			h.overrides = map[string]string{"/open/section/1.json": string(data)}
			h.mu.Unlock()
			h.goTo(Route{Page: "home", Name: "1"})
			h.view.SetSize(size[0], size[1])
			h.view.Scroll(float32(size[0])*0.72, float32(size[1])*0.68, 0, 900)
			h.view.Frame()
			before, found := h.view.Find("Board 15 · B15")
			if !found {
				t.Fatalf("scroll fixture did not render target board; section=%q visible=%q", h.app.section.Name, h.view.Texts())
			}
			if h.app.section.Name != "1" {
				t.Fatalf("selected section before leaving = %q, want 1", h.app.section.Name)
			}

			h.app.navigate(Route{Page: "thread", Name: "Go", ID: "42"}, true)
			h.idle()
			h.click("首页")
			h.view.Frame()
			if h.app.route.Page != "home" || h.app.section.Name != "1" {
				t.Fatalf("home return page/section = %q/%q, want home/1", h.app.route.Page, h.app.section.Name)
			}
			after, found := h.view.Find("Board 15 · B15")
			if !found {
				t.Fatal("target board disappeared after returning Home")
			}
			if math.Abs(float64(after.Y-before.Y)) > .5 {
				t.Fatalf("board scroll position changed: before y=%.1f after y=%.1f", before.Y, after.Y)
			}
		})
	}
}
