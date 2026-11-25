package bootstrap

import (
	"context"
	"net/http"

	"github.com/VeinsureLee/BYR-Client/core"
	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

// Config supplies real platform adapters at the application composition root.
type Config struct {
	BaseURL     string
	HTTPClient  *http.Client
	Storage     local.Backend
	Credentials application.CredentialStore
}

type Services struct {
	App *application.App
}

// New opens local state and wires dependencies. Session restoration and network
// calls start explicitly after window creation, so tests can construct it offline.
func New(config Config) (Services, error) {
	app, err := core.Open(context.Background(), core.Options{
		BaseURL: config.BaseURL, HTTPClient: config.HTTPClient, State: config.Storage, Credentials: config.Credentials,
	})
	if err != nil {
		return Services{}, err
	}
	return Services{App: app}, nil
}
