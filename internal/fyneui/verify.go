package fyneui

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/fyne/v2"

	svc "BYR-Client/internal/app"
)

type verificationReport struct {
	Platform       string           `json:"platform"`
	Renderer       string           `json:"renderer"`
	StartedAt      time.Time        `json:"startedAt"`
	ElapsedSeconds float64          `json:"elapsedSeconds"`
	Checks         map[string]bool  `json:"checks"`
	Screenshots    []string         `json:"screenshots"`
	Measurements   map[string]int64 `json:"measurements"`
	Notes          []string         `json:"notes"`
	Error          string           `json:"error,omitempty"`
}

var verificationChecks = []string{
	"shared_core_initial_thread", "offline_images_decoded", "1000_reply_pagination",
	"quote_branch_collapse", "compact_viewport", "compact_reading_navigation",
	"compact_topic_reopen_handler", "shared_core_search", "simulated_stream_stop_retains_text",
}

// verify runs inside the real Fyne application, after its event loop starts.
// A narrow desktop viewport is deliberately not reported as Android execution.
// This opt-in mode writes only screenshots/report files and closes its window.
func (u *UI) verify(destination string) {
	started := time.Now()
	report := verificationReport{
		Platform: runtime.GOOS + "/" + runtime.GOARCH, Renderer: "native Fyne window",
		StartedAt: started, Checks: make(map[string]bool), Measurements: make(map[string]int64),
		Notes: []string{
			"Offline forum fixtures and simulated AI; no real account or model credentials were used.",
			"Compact screenshots verify desktop layouts only; Android runtime/IME/touch remain unverified.",
			"Timing and Go heap samples are smoke-test observations, not FPS or process RSS benchmarks.",
		},
	}
	for _, name := range verificationChecks {
		report.Checks[name] = false
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "Fyne verification:", err)
		fyne.Do(u.window.Close)
		return
	}
	defer func() {
		report.ElapsedSeconds = time.Since(started).Seconds()
		data, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(filepath.Join(destination, "report.json"), data, 0o644)
		fyne.Do(u.window.Close)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	fail := func(err error) { report.Error = err.Error() }
	if err := waitFor(ctx, func() bool {
		s := u.controller.Snapshot()
		return s.Ready && s.Thread.Article.BodyLoaded && !s.Thread.Loading
	}); err != nil {
		fail(err)
		return
	}
	report.Checks["shared_core_initial_thread"] = true
	report.Measurements["ready_milliseconds"] = time.Since(started).Milliseconds()
	capture := func(name string, before func()) bool {
		fyne.DoAndWait(func() { u.sync(); before() })
		time.Sleep(450 * time.Millisecond) // Allow layout, image callbacks and native painting.
		var shot image.Image
		fyne.DoAndWait(func() {
			shot = u.window.Canvas().Capture()
			if name >= "03-compact" {
				width := int64(u.window.Canvas().Size().Width)
				report.Measurements["compact_canvas_width"] = max(report.Measurements["compact_canvas_width"], width)
				report.Checks["compact_viewport"] = report.Measurements["compact_canvas_width"] <= 362
			}
		})
		file, err := os.Create(filepath.Join(destination, name+".png"))
		if err == nil {
			err = png.Encode(file, shot)
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
		}
		if err != nil {
			fail(err)
			return false
		}
		report.Screenshots = append(report.Screenshots, name+".png")
		return true
	}
	if !capture("01-windows-reading", func() {
		u.window.Resize(fyne.NewSize(1280, 820))
		u.reading = true
		u.showPage(pageBrowse)
		u.browse.Refresh()
	}) {
		return
	}
	fyne.DoAndWait(func() {
		u.content.mu.Lock()
		report.Checks["offline_images_decoded"] = len(u.content.cache) > 0
		u.content.mu.Unlock()
	})
	stressStarted := time.Now()
	u.controller.LoadAllReplies()
	if err := waitFor(ctx, func() bool {
		s := u.controller.Snapshot()
		return !s.Thread.Loading && s.Thread.End && len(s.Thread.Replies) >= 1000
	}); err != nil {
		fail(err)
		return
	}
	report.Measurements["load_1000_replies_milliseconds"] = time.Since(stressStarted).Milliseconds()
	report.Measurements["loaded_replies"] = int64(len(u.controller.Snapshot().Thread.Replies))
	report.Checks["1000_reply_pagination"] = true
	if !capture("02-windows-1000-replies", func() {
		u.reader.ScrollToBottom()
	}) {
		return
	}
	// Collapse a quoted branch and verify the flattened virtual list shrinks.
	fyne.DoAndWait(func() {
		before := len(u.rows)
		for _, row := range u.rows {
			if row.children > 0 {
				u.replyOpen[row.key()] = false
				u.rebuildReader()
				report.Checks["quote_branch_collapse"] = len(u.rows) < before
				u.replyOpen[row.key()] = true
				u.rebuildReader()
				break
			}
		}
	})
	if !capture("03-compact-reading", func() {
		u.window.Resize(fyne.NewSize(360, 800))
		u.reading = true
		u.browse.Refresh()
		u.reader.ScrollToTop()
	}) {
		return
	}
	fyne.DoAndWait(func() {
		report.Checks["compact_reading_navigation"] = !u.browse.Objects[0].Visible() && u.browse.Objects[1].Visible()
	})
	if !capture("04-compact-topics", func() {
		u.reading = false
		u.boardList.UnselectAll()
		u.browse.Refresh()
	}) {
		return
	}
	fyne.DoAndWait(func() {
		u.boardList.Select(0)
		opened := u.reading
		u.reading = false
		u.boardList.UnselectAll() // Mirrors the back-button handler.
		u.boardList.Select(0)
		report.Checks["compact_topic_reopen_handler"] = opened && u.reading
	})
	u.controller.Search("Fyne")
	if err := waitFor(ctx, func() bool {
		s := u.controller.Snapshot()
		return s.SearchID != "" && s.Search.Status == svc.StatusCompleted && s.Search.ArticleCount > 0
	}); err != nil {
		fail(err)
		return
	}
	report.Checks["shared_core_search"] = true
	if !capture("05-compact-search", func() {
		u.query.SetText("Fyne")
		u.showPage(pageSearch)
	}) {
		return
	}
	question := "如何验证手机端与桌面端的中文阅读体验？"
	fyne.DoAndWait(func() {
		u.showPage(pageChat)
		u.question.SetText(question)
	})
	u.controller.StartChat(question, nil)
	if err := waitFor(ctx, func() bool {
		s := u.controller.Snapshot().Chat
		return s.Running && len([]rune(s.Answer)) >= 100
	}); err != nil {
		fail(err)
		return
	}
	u.controller.StopChat()
	if err := waitFor(ctx, func() bool { return !u.controller.Snapshot().Chat.Running }); err != nil {
		fail(err)
		return
	}
	saved := u.controller.Snapshot().Chat.Answer
	time.Sleep(180 * time.Millisecond)
	report.Checks["simulated_stream_stop_retains_text"] = saved != "" && saved == u.controller.Snapshot().Chat.Answer
	if !capture("06-compact-ai-stopped", func() {}) {
		return
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	report.Measurements["go_heap_alloc_bytes"] = int64(mem.HeapAlloc)
	report.Measurements["goroutines"] = int64(runtime.NumGoroutine())
	for name, passed := range report.Checks {
		if !passed {
			report.Error = "verification check failed: " + name
			break
		}
	}
}

func verificationResult(destination string) error {
	data, err := os.ReadFile(filepath.Join(destination, "report.json"))
	if err != nil {
		return err
	}
	var report verificationReport
	if err := json.Unmarshal(data, &report); err != nil {
		return err
	}
	if report.Error != "" {
		return fmt.Errorf("%s", report.Error)
	}
	for _, name := range verificationChecks {
		if !report.Checks[name] {
			return fmt.Errorf("missing or failed check: %s", name)
		}
	}
	return nil
}

func waitFor(ctx context.Context, condition func() bool) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if condition() {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("verification timed out: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
