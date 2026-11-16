package bootstrap

import (
	"context"
	"net/http"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
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
	Forum *forum.Client
	App   *application.App
}

// New opens local state and wires dependencies. Session restoration and network
// calls start explicitly after window creation, so tests can construct it offline.
func New(config Config) (Services, error) {
	store, err := local.Open(context.Background(), config.Storage)
	if err != nil {
		return Services{}, err
	}
	app, err := application.New(application.Options{BaseURL: config.BaseURL, HTTPClient: config.HTTPClient, Local: store, Credentials: config.Credentials})
	if err != nil {
		return Services{}, err
	}
	return Services{Forum: app.Forum, App: app}, nil
}
