package nomadclient

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

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
	ID                   string
	Name                 string
	Namespace            string
	Type                 string
	Status               string
	StatusDescription    string
	DerivedStatus        string
	DerivedStatusReason  string
	Exists               bool
	DesiredAllocs        int
	RunningAllocs        int
	StartingAllocs       int
	QueuedAllocs         int
	FailedAllocs         int
	LostAllocs           int
	UnknownAllocs        int
	LatestDeploymentID   string
	LatestAllocationID   string
	LatestAllocationName string
	Allocations          []AllocationStatus
}

// AllocationStatus captures summary information for an allocation.
type AllocationStatus struct {
	ID      string
	Name    string
	Client  string
	Status  string
	Desired string
	Group   string
	Healthy *bool
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

	status := &JobStatus{
		ID:                derefString(job.ID, job.Name),
		Name:              derefString(job.Name, job.ID),
		Namespace:         derefString(job.Namespace, nil),
		Type:              strings.ToLower(derefString(job.Type, nil)),
		Status:            strings.ToLower(derefString(job.Status, nil)),
		StatusDescription: derefString(job.StatusDescription, nil),
		Exists:            true,
		DerivedStatus:     strings.ToLower(derefString(job.Status, nil)),
	}

	if statusSummaries, _, err := a.client.Jobs().Summary(status.ID, nil); err == nil && statusSummaries != nil && statusSummaries.Summary != nil {
		var desired, running, starting, queued, failed, lost, unknown int
		for _, grp := range statusSummaries.Summary {
			running += grp.Running
			starting += grp.Starting
			queued += grp.Queued
			failed += grp.Failed
			lost += grp.Lost
			unknown += grp.Unknown
		}
		desired = running + starting + queued + failed + lost + unknown
		status.DesiredAllocs = desired
		status.RunningAllocs = running
		status.StartingAllocs = starting
		status.QueuedAllocs = queued
		status.FailedAllocs = failed
		status.LostAllocs = lost
		status.UnknownAllocs = unknown
		status.DerivedStatus, status.DerivedStatusReason = deriveStatus(status)
	}

	if deployment, _, err := a.client.Jobs().LatestDeployment(status.ID, nil); err == nil && deployment != nil {
		status.LatestDeploymentID = deployment.ID
		if status.DerivedStatus == "" {
			status.DerivedStatus, status.DerivedStatusReason = deriveStatusFromDeployment(status, deployment.Status)
		}
	}

	if allocs, _, err := a.client.Jobs().Allocations(status.ID, true, nil); err == nil && len(allocs) > 0 {
		status.Allocations = make([]AllocationStatus, 0, len(allocs))
		for _, alloc := range allocs {
			if alloc == nil {
				continue
			}
			var healthy *bool
			if alloc.DeploymentStatus != nil {
				healthy = alloc.DeploymentStatus.Healthy
			}
			status.Allocations = append(status.Allocations, AllocationStatus{
				ID:      alloc.ID,
				Name:    alloc.Name,
				Client:  alloc.NodeName,
				Status:  strings.ToLower(alloc.ClientStatus),
				Desired: strings.ToLower(alloc.DesiredStatus),
				Group:   alloc.TaskGroup,
				Healthy: healthy,
			})
		}
		status.LatestAllocationID = allocs[0].ID
		status.LatestAllocationName = allocs[0].Name
	}

	if status.DerivedStatus == "" {
		status.DerivedStatus = status.Status
	}

	return status, nil
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

func deriveStatus(status *JobStatus) (string, string) {
	if status == nil {
		return "", ""
	}

	if status.FailedAllocs > 0 {
		return "failed", formatAllocationSummary(status)
	}
	if status.LostAllocs > 0 {
		return "lost", formatAllocationSummary(status)
	}
	if status.RunningAllocs > 0 && status.DesiredAllocs > 0 {
		if status.RunningAllocs == status.DesiredAllocs {
			return "healthy", formatAllocationSummary(status)
		}
		return "degraded", formatAllocationSummary(status)
	}
	if status.StartingAllocs > 0 || status.QueuedAllocs > 0 {
		return "deploying", formatAllocationSummary(status)
	}
	if status.Status == "pending" {
		return "pending", formatAllocationSummary(status)
	}
	if status.Status == "dead" {
		return "dead", formatAllocationSummary(status)
	}
	return status.Status, formatAllocationSummary(status)
}

func formatAllocationSummary(status *JobStatus) string {
	if status == nil {
		return ""
	}
	var desired string
	if status.DesiredAllocs > 0 {
		desired = strconv.Itoa(status.DesiredAllocs)
	} else {
		desired = "0"
	}
	return strconv.Itoa(status.RunningAllocs) + "/" + desired + " allocations running"
}

func deriveStatusFromDeployment(status *JobStatus, deploymentStatus string) (string, string) {
	switch strings.ToLower(deploymentStatus) {
	case api.DeploymentStatusSuccessful:
		return "healthy", formatAllocationSummary(status)
	case api.DeploymentStatusRunning, api.DeploymentStatusPending, api.DeploymentStatusUnblocking:
		return "deploying", formatAllocationSummary(status)
	case api.DeploymentStatusFailed, api.DeploymentStatusCancelled, api.DeploymentStatusBlocked:
		return "failed", formatAllocationSummary(status)
	}
	return "", ""
}
