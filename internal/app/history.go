package app

import (
	"slices"
	"sort"
	"time"

	"BYR-Client/internal/domain"
)

type HistoryEntry struct {
	ID        string
	Mode      SearchMode
	Title     string
	Query     domain.SearchQuery
	Pinned    bool
	Deleted   bool
	CreatedAt time.Time
	UpdatedAt time.Time
	AIAnswer  string
}

type HistorySnapshot struct {
	ID        string
	Mode      SearchMode
	Title     string
	Query     domain.SearchQuery
	Pinned    bool
	Deleted   bool
	CreatedAt time.Time
	UpdatedAt time.Time
	AIAnswer  string
}

func (a *App) History() (pinned, recent []HistorySnapshot) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, entry := range a.history {
		if entry.Deleted {
			continue
		}
		snapshot := historySnapshot(entry)
		if entry.Pinned {
			pinned = append(pinned, snapshot)
		} else {
			recent = append(recent, snapshot)
		}
	}
	sort.SliceStable(pinned, func(i, j int) bool { return pinned[i].UpdatedAt.After(pinned[j].UpdatedAt) })
	sort.SliceStable(recent, func(i, j int) bool { return recent[i].UpdatedAt.After(recent[j].UpdatedAt) })
	return pinned, recent
}

func (a *App) HistoryEntry(id string) (HistorySnapshot, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, entry := range a.history {
		if entry.ID == id && !entry.Deleted {
			return historySnapshot(entry), true
		}
	}
	return HistorySnapshot{}, false
}

func (a *App) PinHistory(id string, pinned bool) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, entry := range a.history {
		if entry.ID == id && !entry.Deleted {
			entry.Pinned = pinned
			entry.UpdatedAt = a.clock.Now()
			return true
		}
	}
	return false
}

func (a *App) DeleteHistory(id string) bool {
	a.mu.Lock()
	var cancel func()
	found := false
	for _, entry := range a.history {
		if entry.ID == id && !entry.Deleted {
			entry.Deleted = true
			entry.UpdatedAt = a.clock.Now()
			found = true
			if run := a.runs[id]; run != nil {
				cancel = run.cancel
			}
			break
		}
	}
	if found {
		a.lastDelete = id
	}
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if found {
		a.changed()
	}
	return found
}

func (a *App) UndoDelete() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastDelete == "" {
		return false
	}
	for _, entry := range a.history {
		if entry.ID == a.lastDelete {
			entry.Deleted = false
			entry.UpdatedAt = a.clock.Now()
			a.lastDelete = ""
			return true
		}
	}
	a.lastDelete = ""
	return false
}

func (a *App) RestoreHistory(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, entry := range a.history {
		if entry.ID == id && entry.Deleted {
			entry.Deleted = false
			entry.UpdatedAt = a.clock.Now()
			return true
		}
	}
	return false
}

func (a *App) AddAIHistory(question string, q domain.SearchQuery) string {
	now := a.clock.Now()
	q = q.Normalized()
	a.mu.Lock()
	a.runSeq++
	id := "ai-" + time.Now().Format("20060102150405") + "-" + itoa(a.runSeq)
	a.history = append(a.history, &HistoryEntry{
		ID:        id,
		Mode:      ModeAI,
		Title:     question,
		Query:     q,
		CreatedAt: now,
		UpdatedAt: now,
	})
	a.mu.Unlock()
	a.changed()
	return id
}

func (a *App) SetAIAnswer(id, answer string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, entry := range a.history {
		if entry.ID == id {
			entry.AIAnswer = answer
			entry.UpdatedAt = a.clock.Now()
			return true
		}
	}
	return false
}

func historySnapshot(entry *HistoryEntry) HistorySnapshot {
	return HistorySnapshot{
		ID:        entry.ID,
		Mode:      entry.Mode,
		Title:     entry.Title,
		Query:     entry.Query,
		Pinned:    entry.Pinned,
		Deleted:   entry.Deleted,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
		AIAnswer:  entry.AIAnswer,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	slices.Reverse(digits)
	return string(digits)
}
