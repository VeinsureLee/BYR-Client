package application

import (
	"context"
	"errors"
	"strings"
)

// FetchAsset routes image and attachment reads through the forum client's
// trusted-origin policy while pinning authentication to the starting account.
func (a *App) FetchAsset(ctx context.Context, raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("图片或附件地址不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) ([]byte, error) {
		return a.forum.FetchAsset(requestCtx, raw)
	})
}
