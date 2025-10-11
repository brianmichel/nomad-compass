package server

import (
	"context"

	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

// listRepositoryResponses gathers repository information and augments it with
// Nomad job details. The aggregation logic lives outside of the HTTP handler so
// it can be reused by both tests and any future callers (e.g. scheduled cache
// warmers) without going through net/http primitives.
func (s *Server) listRepositoryResponses(ctx context.Context) ([]repositoryResponse, error) {
	repos, err := s.repos.List(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]repositoryResponse, 0, len(repos))
	for _, repo := range repos {
		files, err := s.files.ListByRepo(ctx, repo.ID)
		if err != nil {
			return nil, err
		}

		jobResponses := make([]repositoryJobResponse, 0, len(files))
		for _, file := range files {
			jobResponses = append(jobResponses, s.buildJobResponse(ctx, repo, file))
		}

		repoResp := newRepositoryResponse(repo)
		repoResp.Jobs = jobResponses
		responses = append(responses, repoResp)
	}

	return responses, nil
}

func (s *Server) buildJobResponse(ctx context.Context, repo storage.Repository, file storage.RepoFile) repositoryJobResponse {
	jobResp := newRepositoryJobResponse(file)
	if !file.JobID.Valid || file.JobID.String == "" {
		return jobResp
	}

	jobResp.JobID = file.JobID.String

	status, err := s.nomad.JobStatus(ctx, file.JobID.String)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("fetch job status failed", "repo_id", repo.ID, "repo", repo.Name, "job_id", file.JobID.String, "error", err)
		}
		jobResp.StatusError = err.Error()
		return jobResp
	}
	if status == nil {
		return jobResp
	}

	if status.Exists {
		applyNomadStatus(&jobResp, status, s.nomadAddr)
	} else {
		jobResp.Status = "missing"
		jobResp.StatusDescription = "Job not found in Nomad"
	}
	return jobResp
}

// applyNomadStatus copies job health details from the Nomad API response onto
// the JSON response model so the enrichment logic lives in one place.
func applyNomadStatus(jobResp *repositoryJobResponse, status *nomadclient.JobStatus, nomadAddr string) {
	jobResp.JobName = status.Name
	jobResp.Status = status.DerivedStatus
	if status.DerivedStatusReason != "" {
		jobResp.StatusDescription = status.DerivedStatusReason
	} else {
		jobResp.StatusDescription = status.StatusDescription
	}
	jobResp.NomadStatus = status.Status
	jobResp.Namespace = status.Namespace
	jobResp.JobType = status.Type
	jobResp.RunningAllocs = status.RunningAllocs
	jobResp.DesiredAllocs = status.DesiredAllocs
	jobResp.StartingAllocs = status.StartingAllocs
	jobResp.QueuedAllocs = status.QueuedAllocs
	jobResp.FailedAllocs = status.FailedAllocs
	jobResp.LostAllocs = status.LostAllocs
	jobResp.UnknownAllocs = status.UnknownAllocs
	jobResp.LatestDeploymentID = status.LatestDeploymentID
	jobResp.LatestAllocationID = status.LatestAllocationID
	jobResp.LatestAllocationName = status.LatestAllocationName
	jobResp.Allocations = status.Allocations
	jobResp.JobURL = jobURL(nomadAddr, status.Namespace, status.ID)
}
