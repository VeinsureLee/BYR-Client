package application

import (
	"context"
	"errors"
	"sort"
	"strconv"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

type Contact struct {
	User    forum.User
	Time    int64
	Preview string
}
type ChatMessage struct {
	Mail forum.Mail
	Mine bool
}

func timestamp(s forum.Scalar) int64 { v, _ := strconv.ParseInt(string(s), 10, 64); return v }
func (a *App) Contacts(ctx context.Context) ([]Contact, error) {
	byID := map[forum.Scalar]Contact{}
	var failures []error
	for _, box := range []string{"inbox", "outbox"} {
		m, err := a.Forum.Mailbox(ctx, box, forum.Page{Size: 100})
		if err != nil {
			failures = append(failures, err)
			continue
		}
		for _, v := range m.Mails {
			if v.Author.ID == "" {
				continue
			}
			old, ok := byID[v.Author.ID]
			t := timestamp(v.PostTime)
			if !ok || t > old.Time {
				byID[v.Author.ID] = Contact{User: v.Author, Time: t, Preview: v.Title}
			}
		}
	}
	if len(failures) == 2 {
		return nil, errors.Join(failures...)
	}
	out := make([]Contact, 0, len(byID))
	for _, v := range byID {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Time == out[j].Time {
			return out[i].User.ID < out[j].User.ID
		}
		return out[i].Time > out[j].Time
	})
	return out, nil
}
func (a *App) Conversation(ctx context.Context, contact string) ([]ChatMessage, error) {
	var out []ChatMessage
	for _, box := range []string{"inbox", "outbox"} {
		m, err := a.Forum.Mailbox(ctx, box, forum.Page{Size: 100})
		if err != nil {
			return nil, err
		}
		for _, v := range m.Mails {
			if string(v.Author.ID) != contact {
				continue
			}
			detail, err := a.Forum.Mail(ctx, box, string(v.Index))
			if err != nil {
				return nil, err
			}
			detail.Box = box
			out = append(out, ChatMessage{Mail: detail, Mine: box == "outbox"})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return timestamp(out[i].Mail.PostTime) < timestamp(out[j].Mail.PostTime) })
	return out, nil
}
