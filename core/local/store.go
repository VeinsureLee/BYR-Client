// Package local owns serialization and account-scoped local business state.
// Platforms adapt Backend to their private application directory.
package local

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

type Backend interface {
	Load(context.Context) ([]byte, error)
	Save(context.Context, []byte) error
}
type Preferences struct {
	Theme     int     `json:"theme"`
	FontScale float32 `json:"fontScale"`
	IPv6      bool    `json:"ipv6"`
}

func (p Preferences) Normalized() Preferences {
	if p.Theme < 0 || p.Theme > 2 {
		p.Theme = 0
	}
	if p.FontScale == 0 {
		p.FontScale = 1
	}
	if p.FontScale < .8 {
		p.FontScale = .8
	}
	if p.FontScale > 1.4 {
		p.FontScale = 1.4
	}
	return p
}

type Draft struct {
	Title, Content string
	Attachments    []forum.Attachment `json:"attachments,omitempty"`
}

func (d Draft) Empty() bool { return d.Title == "" && d.Content == "" && len(d.Attachments) == 0 }
func DraftKey(board, replyID, editID string) string {
	suffix := editID
	if suffix == "" {
		suffix = replyID
	}
	if suffix == "" {
		suffix = "new"
	}
	return "composeDraft." + board + "." + suffix
}

type History struct {
	Board, ID, Title string
	VisitedAt        int64
}
type Account struct {
	Drafts  map[string]Draft `json:"drafts"`
	History []History        `json:"history"`
}
type document struct {
	Version     int                `json:"version"`
	Preferences Preferences        `json:"preferences"`
	Accounts    map[string]Account `json:"accounts"`
}
type Store struct {
	mu      sync.RWMutex
	backend Backend
	data    document
}

func Open(ctx context.Context, backend Backend) (*Store, error) {
	s := &Store{backend: backend, data: document{Version: 1, Preferences: Preferences{FontScale: 1}, Accounts: map[string]Account{}}}
	if backend != nil {
		b, err := backend.Load(ctx)
		if err != nil {
			return nil, err
		}
		if len(b) > 0 {
			if err = json.Unmarshal(b, &s.data); err != nil {
				return nil, errors.New("本地状态文件损坏，请备份后重试")
			}
			if s.data.Version != 1 {
				return nil, errors.New("不支持的本地数据版本")
			}
		}
	}
	s.data.Preferences = s.data.Preferences.Normalized()
	if s.data.Accounts == nil {
		s.data.Accounts = map[string]Account{}
	}
	return s, nil
}
func (s *Store) Preferences() Preferences {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Preferences
}
func (s *Store) SetPreferences(ctx context.Context, p Preferences) error {
	return s.change(ctx, func(d *document) { d.Preferences = p.Normalized() })
}
func accountKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
func (s *Store) Draft(account, key string) (Draft, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Accounts[accountKey(account)].Drafts[key]
	return cloneDraft(v), ok
}
func cloneDraft(d Draft) Draft {
	b, _ := json.Marshal(d)
	var out Draft
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *Store) SaveDraft(ctx context.Context, account, key string, draft Draft) error {
	if accountKey(account) == "" {
		return errors.New("草稿需要登录账号")
	}
	return s.change(ctx, func(d *document) {
		k := accountKey(account)
		a := d.Accounts[k]
		if a.Drafts == nil {
			a.Drafts = map[string]Draft{}
		}
		if draft.Empty() {
			delete(a.Drafts, key)
		} else {
			a.Drafts[key] = cloneDraft(draft)
		}
		d.Accounts[k] = a
	})
}
func (s *Store) History(account string) []History {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]History(nil), s.data.Accounts[accountKey(account)].History...)
}
func (s *Store) Visit(ctx context.Context, account string, entry History) error {
	if accountKey(account) == "" || entry.Board == "" || entry.ID == "" {
		return errors.New("历史记录缺少账号、版面或主题")
	}
	if entry.VisitedAt == 0 {
		entry.VisitedAt = time.Now().UnixMilli()
	}
	return s.change(ctx, func(d *document) {
		k := accountKey(account)
		a := d.Accounts[k]
		next := []History{entry}
		for _, e := range a.History {
			if e.Board != entry.Board || e.ID != entry.ID {
				next = append(next, e)
			}
			if len(next) >= 100 {
				break
			}
		}
		a.History = next
		d.Accounts[k] = a
	})
}
func (s *Store) ClearHistory(ctx context.Context, account string) error {
	return s.change(ctx, func(d *document) { k := accountKey(account); a := d.Accounts[k]; a.History = nil; d.Accounts[k] = a })
}

// Persist a copy before publishing it: a failed disk write cannot make memory
// pretend the change was saved. The mutex serializes concurrent draft/history writes.
func (s *Store) change(ctx context.Context, fn func(*document)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	var next document
	if err = json.Unmarshal(b, &next); err != nil {
		return err
	}
	fn(&next)
	b, err = json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if s.backend != nil {
		if err = s.backend.Save(ctx, b); err != nil {
			return err
		}
	}
	s.data = next
	return nil
}
