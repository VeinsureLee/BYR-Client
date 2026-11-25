// Package core owns the shared application's composition root.
package core

import (
	"context"
	"net/http"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/internal/byrhttp"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

type Options struct {
	BaseURL     string
	HTTPClient  *http.Client
	State       local.Backend
	Credentials application.CredentialStore
}

// Open assembles the shared use cases, local adapter, and private protocol
// adapter without restoring a session or making a network request.
func Open(ctx context.Context, options Options) (*application.App, error) {
	store, err := local.Open(ctx, options.State)
	if err != nil {
		return nil, err
	}
	factory := func(token application.TokenSource, ipv6 func() bool) (application.ForumClient, error) {
		return byrhttp.New(byrhttp.Options{
			BaseURL: options.BaseURL, HTTPClient: options.HTTPClient,
			Token: func(ctx context.Context) (string, error) { return token(ctx) }, IPv6: ipv6,
		})
	}
	return application.New(application.Options{Local: store, Credentials: options.Credentials, ForumFactory: factory})
}
