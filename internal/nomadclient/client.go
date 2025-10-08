package nomadclient

import (
	"context"
	"errors"
	"net/http"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/config"
)

// Client defines the operations Nomad Compass uses.
type Client interface {
	RegisterJob(ctx context.Context, job *api.Job) error
	DeregisterJob(ctx context.Context, jobID string, purge bool) error
	Ping(ctx context.Context) error
	JobStatus(ctx context.Context, jobID string) (*JobStatus, error)
}

// API wraps the Nomad API client.
type API struct {
	client *api.Client
}

// JobStatus captures a subset of Nomad job health details.
type JobStatus struct {
	ID                string
	Name              string
	Status            string
	StatusDescription string
	Exists            bool
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

// Ping verifies connectivity with the Nomad control plane.
func (a *API) Ping(ctx context.Context) error {
	// The Nomad client does not expose context-aware calls for status checks.
	// The request is best-effort; we ignore the returned leader string.
	_, err := a.client.Status().Leader()
	return err
}

// JobStatus fetches the current status for a Nomad job by ID.
func (a *API) JobStatus(ctx context.Context, jobID string) (*JobStatus, error) {
	if jobID == "" {
		return nil, nil
	}

	job, _, err := a.client.Jobs().Info(jobID, nil)
	if err != nil {
		var unexpected api.UnexpectedResponseError
		if errors.As(err, &unexpected) && unexpected.StatusCode() == http.StatusNotFound {
			return &JobStatus{ID: jobID, Exists: false}, nil
		}
		return nil, err
	}
	if job == nil {
		return &JobStatus{ID: jobID, Exists: false}, nil
	}

	return &JobStatus{
		ID:                derefString(job.ID, job.Name),
		Name:              derefString(job.Name, job.ID),
		Status:            derefString(job.Status, nil),
		StatusDescription: derefString(job.StatusDescription, nil),
		Exists:            true,
	}, nil
}

func derefString(primary *string, fallback *string) string {
	if primary != nil && *primary != "" {
		return *primary
	}
	if fallback != nil && *fallback != "" {
		return *fallback
	}
	return ""
}
