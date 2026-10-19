// Package refresh schedules a refresh of the currently active business
// resource after a period without user interaction. Only the active resource
// participates; there is no periodic scan of every board.
package refresh

import (
	"sync"
	"time"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/ports"
)

type TargetKind string

const (
	BoardTarget  TargetKind = "board_window"
	ThreadTarget TargetKind = "thread_window"
)

type Target struct {
	Kind    TargetKind
	BoardID domain.BoardID
	Article domain.ArticleKey
}

func (t Target) Equal(other Target) bool {
	if t.Kind != other.Kind || t.BoardID != other.BoardID {
		return false
	}
	if t.Kind == ThreadTarget {
		return t.Article == other.Article
	}
	return true
}

type Trigger struct {
	Target     Target
	LeaseID    uint64
	Generation uint64
	Epoch      uint64
}

type Lease struct {
	ID                  uint64
	Generation          uint64
	Target              Target
	AppActive           bool
	ActivatedAt         time.Time
	LastInteractionAt   time.Time
	LastAttemptFinished time.Time
	LastSuccessAt       time.Time
	Running             bool
	Deadline            time.Time
}

type Scheduler struct {
	mu    sync.Mutex
	clock ports.Clock
	idle  time.Duration

	nextID  uint64
	nextGen uint64
	epoch   uint64
	current *leaseState
}

type leaseState struct {
	id                  uint64
	generation          uint64
	target              Target
	appActive           bool
	activatedAt         time.Time
	lastInteractionAt   time.Time
	lastAttemptFinished time.Time
	lastSuccessAt       time.Time
	running             bool
}

func New(clock ports.Clock, idle time.Duration) *Scheduler {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	if idle <= 0 {
		idle = 10 * time.Minute
	}
	return &Scheduler{clock: clock, idle: idle}
}

func (s *Scheduler) Activate(target Target) Lease {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	s.nextGen++
	now := s.clock.Now()
	s.current = &leaseState{
		id:                s.nextID,
		generation:        s.nextGen,
		target:            target,
		appActive:         true,
		activatedAt:       now,
		lastInteractionAt: now,
	}
	s.epoch++
	return s.leaseLocked()
}

func (s *Scheduler) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return
	}
	s.current.lastInteractionAt = s.clock.Now()
	s.epoch++
}

func (s *Scheduler) Deactivate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = nil
	s.epoch++
}

func (s *Scheduler) SetApplicationActive(active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return
	}
	s.current.appActive = active
	if active {
		s.current.lastInteractionAt = s.clock.Now()
	}
	s.epoch++
}

func (s *Scheduler) Lease() Lease {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.leaseLocked()
}

func (s *Scheduler) leaseLocked() Lease {
	if s.current == nil {
		return Lease{Deadline: time.Time{}}
	}
	current := *s.current
	return Lease{
		ID:                  current.id,
		Generation:          current.generation,
		Target:              current.target,
		AppActive:           current.appActive,
		ActivatedAt:         current.activatedAt,
		LastInteractionAt:   current.lastInteractionAt,
		LastAttemptFinished: current.lastAttemptFinished,
		LastSuccessAt:       current.lastSuccessAt,
		Running:             current.running,
		Deadline:            deadline(current, s.idle),
	}
}

// Due returns one trigger for the active resource when its idle period has
// elapsed. The lease is marked running, so repeated ticks cannot stack work.
func (s *Scheduler) Due(now time.Time) (Trigger, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil || s.current.running || !s.current.appActive {
		return Trigger{}, false
	}
	if now.Before(deadline(*s.current, s.idle)) {
		return Trigger{}, false
	}
	s.current.running = true
	s.epoch++
	return Trigger{
		Target:     s.current.target,
		LeaseID:    s.current.id,
		Generation: s.current.generation,
		Epoch:      s.epoch,
	}, true
}

// Finish closes a trigger. A stale generation from a previous resource cannot
// update the current lease.
func (s *Scheduler) Finish(trigger Trigger, success bool, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil || trigger.Generation != s.current.generation || !s.current.running {
		return false
	}
	s.current.running = false
	s.current.lastAttemptFinished = now
	if success {
		s.current.lastSuccessAt = now
	}
	s.epoch++
	return true
}

func deadline(state leaseState, idle time.Duration) time.Time {
	last := state.activatedAt
	if state.lastInteractionAt.After(last) {
		last = state.lastInteractionAt
	}
	if state.lastAttemptFinished.After(last) {
		last = state.lastAttemptFinished
	}
	return last.Add(idle)
}
