// Package bootstrap assembles the Android client after the app backend is ready.
// Window lifecycle and user-visible startup state remain owned by the UI package.
package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/VeinsureLee/BYR-Client/apps/android/app/internal/platform"
	"github.com/VeinsureLee/BYR-Client/core"
	"github.com/VeinsureLee/BYR-Client/core/application"
)

type Config struct {
	BaseURL     string
	HTTPClient  *http.Client
	DataDir     string
	Credentials application.CredentialStore
}

type Services struct {
	App     *application.App
	DataDir string
}

func New(ctx context.Context, config Config) (*Services, error) {
	dir := config.DataDir
	if dir == "" {
		var err error
		dir, err = platform.DataDir()
		if err != nil {
			return nil, fmt.Errorf("无法打开应用数据目录：%w", err)
		}
	}
	credentials := config.Credentials
	if credentials == nil {
		credentials = platform.FileCredentials{Path: filepath.Join(dir, "session.json")}
	}
	shared, err := core.Open(ctx, core.Options{
		BaseURL: config.BaseURL, HTTPClient: config.HTTPClient,
		State: platform.FileState{Path: filepath.Join(dir, "state.json")}, Credentials: credentials,
	})
	if err != nil {
		return nil, fmt.Errorf("组装共享客户端失败：%w", err)
	}
	if err := shared.Restore(ctx); err != nil {
		return nil, fmt.Errorf("恢复登录失败：%w", err)
	}
	return &Services{App: shared, DataDir: dir}, nil
}
