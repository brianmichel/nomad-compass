package nomadclient

import (
	"context"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/config"
)

// Client defines the operations Nomad Compass uses.
type Client interface {
	RegisterJob(ctx context.Context, job *api.Job) error
	DeregisterJob(ctx context.Context, jobID string, purge bool) error
}

// API wraps the Nomad API client.
type API struct {
	client *api.Client
}

// New constructs a Nomad API wrapper from config.
func New(cfg config.NomadConfig) (*API, error) {
	apiCfg := api.DefaultConfig()
	apiCfg.Address = cfg.Address
	if cfg.Token != "" {
		apiCfg.SecretID = cfg.Token
	}
	client, err := api.NewClient(apiCfg)
	if err != nil {
		return nil, err
	}
	if cfg.Namespace != "" {
		client.SetNamespace(cfg.Namespace)
	}
	if cfg.Region != "" {
		client.SetRegion(cfg.Region)
	}
	return &API{client: client}, nil
}

// RegisterJob submits a Nomad job specification.
func (a *API) RegisterJob(ctx context.Context, job *api.Job) error {
	// The Nomad client does not expose context-aware calls for Register, so we rely on API client internals.
	_, _, err := a.client.Jobs().Register(job, nil)
	return err
}

// DeregisterJob removes a Nomad job by ID.
func (a *API) DeregisterJob(ctx context.Context, jobID string, purge bool) error {
	_, _, err := a.client.Jobs().Deregister(jobID, purge, nil)
	return err
}
