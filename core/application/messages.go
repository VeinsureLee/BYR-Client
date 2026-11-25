package application

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

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

func timestamp(s forum.Scalar) int64 {
	v, err := strconv.ParseInt(string(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// Contacts combines inbox and outbox previews under one pinned account session.
func (a *App) Contacts(ctx context.Context) ([]Contact, error) {
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) ([]Contact, error) {
		byID := map[forum.Scalar]Contact{}
		var failures []error
		for _, box := range []string{"inbox", "outbox"} {
			mailbox, err := a.forum.Mailbox(requestCtx, box, forum.Page{Size: 100})
			if err != nil {
				failures = append(failures, err)
				continue
			}
			for _, mail := range mailbox.Mails {
				if mail.Author.ID == "" {
					continue
				}
				old, ok := byID[mail.Author.ID]
				t := timestamp(mail.PostTime)
				if !ok || t > old.Time {
					byID[mail.Author.ID] = Contact{User: mail.Author, Time: t, Preview: mail.Title}
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
	})
}

// Conversation loads both directions of one contact under the same account
// credential, then orders the result chronologically.
func (a *App) Conversation(ctx context.Context, contact string) ([]ChatMessage, error) {
	contact = strings.TrimSpace(contact)
	if contact == "" {
		return nil, errors.New("联系人不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, credential Credential) ([]ChatMessage, error) {
		out := make([]ChatMessage, 0)
		boxes := []string{"inbox", "outbox"}
		if strings.EqualFold(contact, strings.TrimSpace(credential.Username)) {
			// BYR mirrors a self-addressed mail in both boxes; keep only the
			// canonical sent copy so the conversation does not display it twice.
			boxes = []string{"outbox"}
		}
		for _, box := range boxes {
			mailbox, err := a.forum.Mailbox(requestCtx, box, forum.Page{Size: 100})
			if err != nil {
				return nil, err
			}
			for _, preview := range mailbox.Mails {
				if !strings.EqualFold(string(preview.Author.ID), contact) {
					continue
				}
				detail, err := a.forum.Mail(requestCtx, box, string(preview.Index))
				if err != nil {
					return nil, err
				}
				detail.Box = box
				out = append(out, ChatMessage{Mail: detail, Mine: box == "outbox"})
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return timestamp(out[i].Mail.PostTime) < timestamp(out[j].Mail.PostTime) })
		return out, nil
	})
}

func normalizeMailbox(box string) (string, error) {
	box = strings.ToLower(strings.TrimSpace(box))
	if box != "inbox" && box != "outbox" {
		return "", errors.New("信箱类型无效")
	}
	return box, nil
}

func normalizeReferKind(kind string) (string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "reply" && kind != "at" {
		return "", errors.New("提醒类型无效")
	}
	return kind, nil
}

func (a *App) LoadMailbox(ctx context.Context, box string, page forum.Page) (forum.Mailbox, error) {
	box, err := normalizeMailbox(box)
	if err != nil {
		return forum.Mailbox{}, err
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Mailbox, error) {
		return a.forum.Mailbox(requestCtx, box, page)
	})
}

func (a *App) ReadMail(ctx context.Context, box, id string) (forum.Mail, error) {
	box, err := normalizeMailbox(box)
	if err != nil {
		return forum.Mail{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return forum.Mail{}, errors.New("信件标识不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Mail, error) {
		mail, err := a.forum.Mail(requestCtx, box, id)
		if err == nil {
			mail.Box = box
		}
		return mail, err
	})
}

func (a *App) SendMail(ctx context.Context, to, title, content string) (forum.Status, error) {
	to = strings.TrimSpace(to)
	if to == "" || strings.TrimSpace(content) == "" {
		return forum.Status{}, errors.New("收件人和正文不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Status, error) {
		return a.forum.SendMail(requestCtx, to, title, content)
	})
}

func (a *App) ReplyMail(ctx context.Context, box, id, title, content string) (forum.Mail, error) {
	box, err := normalizeMailbox(box)
	if err != nil {
		return forum.Mail{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" || strings.TrimSpace(content) == "" {
		return forum.Mail{}, errors.New("信件标识和正文不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Mail, error) {
		return a.forum.ReplyMail(requestCtx, box, id, title, content)
	})
}

func (a *App) ForwardMail(ctx context.Context, box, id, target string) (forum.Mail, error) {
	box, err := normalizeMailbox(box)
	if err != nil {
		return forum.Mail{}, err
	}
	id, target = strings.TrimSpace(id), strings.TrimSpace(target)
	if id == "" || target == "" {
		return forum.Mail{}, errors.New("信件标识和转发目标不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Mail, error) {
		return a.forum.ForwardMail(requestCtx, box, id, target)
	})
}

func (a *App) DeleteMail(ctx context.Context, box, id string) (forum.Mail, error) {
	box, err := normalizeMailbox(box)
	if err != nil {
		return forum.Mail{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return forum.Mail{}, errors.New("信件标识不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Mail, error) {
		return a.forum.DeleteMail(requestCtx, box, id)
	})
}

func (a *App) LoadRefers(ctx context.Context, kind string, page forum.Page) (forum.Refers, error) {
	kind, err := normalizeReferKind(kind)
	if err != nil {
		return forum.Refers{}, err
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Refers, error) {
		return a.forum.Refers(requestCtx, kind, page)
	})
}

func (a *App) MarkReferRead(ctx context.Context, kind, index string) (forum.Refers, error) {
	kind, err := normalizeReferKind(kind)
	if err != nil {
		return forum.Refers{}, err
	}
	index = strings.TrimSpace(index)
	if index == "" {
		return forum.Refers{}, errors.New("提醒序号不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Refers, error) {
		return a.forum.ReadRefer(requestCtx, kind, index)
	})
}

func (a *App) MarkAllRefersRead(ctx context.Context, kind string) (forum.Status, error) {
	kind, err := normalizeReferKind(kind)
	if err != nil {
		return forum.Status{}, err
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Status, error) {
		return a.forum.ReadAllRefers(requestCtx, kind)
	})
}
