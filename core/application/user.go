package application

import (
	"context"
	"errors"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

// LookupUser queries one exact forum username or numeric ID as a shared
// application use case, with the account captured when lookup starts.
func (a *App) LookupUser(ctx context.Context, identity string) (forum.User, error) {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return forum.User{}, errors.New("请输入用户名或论坛 ID")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.User, error) {
		return a.forum.User(requestCtx, identity)
	})
}
