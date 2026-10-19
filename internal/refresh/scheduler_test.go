package refresh

import (
	"testing"
	"time"

	"BYR-Client/internal/domain"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func TestIdleRefreshAndInteractionReset(t *testing.T) {
	start := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	clock := &fakeClock{now: start}
	s := New(clock, 10*time.Minute)
	s.Activate(Target{Kind: BoardTarget, BoardID: "b-go"})

	if _, ok := s.Due(start.Add(9*time.Minute + 59*time.Second)); ok {
		t.Fatal("triggered before the idle period")
	}
	trigger, ok := s.Due(start.Add(10 * time.Minute))
	if !ok {
		t.Fatal("did not trigger at ten minutes")
	}
	if _, ok := s.Due(start.Add(11 * time.Minute)); ok {
		t.Fatal("stacked a second trigger while one was running")
	}
	if !s.Finish(trigger, true, start.Add(10*time.Minute+30*time.Second)) {
		t.Fatal("finish rejected")
	}
	if _, ok := s.Due(start.Add(20 * time.Minute)); ok {
		t.Fatal("triggered again before another idle period after the attempt")
	}
	if _, ok := s.Due(start.Add(20*time.Minute + 30*time.Second)); !ok {
		t.Fatal("did not trigger after a new idle period")
	}
}

func TestInteractionAndSwitchInvalidateTrigger(t *testing.T) {
	start := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	clock := &fakeClock{now: start}
	s := New(clock, 10*time.Minute)
	first := s.Activate(Target{Kind: BoardTarget, BoardID: "b-go"})

	clock.now = start.Add(9 * time.Minute)
	s.Touch()
	if _, ok := s.Due(start.Add(10 * time.Minute)); ok {
		t.Fatal("interaction did not reset the idle deadline")
	}
	oldTrigger, ok := s.Due(start.Add(19 * time.Minute))
	if !ok {
		t.Fatal("did not trigger ten minutes after the interaction")
	}
	clock.now = start.Add(20 * time.Minute)
	second := s.Activate(Target{Kind: ThreadTarget, BoardID: "b-go", Article: domain.ArticleKey{SiteID: "byr", BoardID: "b-go", ArticleID: "a1"}})
	if second.Generation == first.Generation {
		t.Fatal("new activation reused the generation")
	}
	if _, ok := s.Due(start.Add(20 * time.Minute)); ok {
		t.Fatal("new activation reused an old deadline")
	}
	if s.Finish(oldTrigger, true, clock.now) {
		t.Fatal("an old generation finished the new lease")
	}
}

func TestBackgroundPausesRefresh(t *testing.T) {
	start := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	clock := &fakeClock{now: start}
	s := New(clock, 10*time.Minute)
	s.Activate(Target{Kind: BoardTarget, BoardID: "b-go"})
	s.SetApplicationActive(false)
	if _, ok := s.Due(start.Add(time.Hour)); ok {
		t.Fatal("refreshed while the application was in the background")
	}
	clock.now = start.Add(time.Hour)
	s.SetApplicationActive(true)
	if _, ok := s.Due(start.Add(time.Hour + 10*time.Minute)); !ok {
		t.Fatal("did not resume the idle timer after returning to the foreground")
	}
}
