package reconcile

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec2"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	bundleplan "github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/repo"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

const (
	compassMetaRepoURL      = "nomad-compass/repo-url"
	compassMetaRepoName     = "nomad-compass/repo-name"
	compassMetaJobFile      = "nomad-compass/job-file"
	compassMetaCommit       = "nomad-compass/commit"
	compassMetaCommitAuthor = "nomad-compass/commit-author"
	compassMetaCommitTitle  = "nomad-compass/commit-title"
)

func (m *Manager) observeBundleJob(ctx context.Context, resource manifest.Resource, tracked storage.ManagedResource) (bundleplan.Observation, error) {
	source, err := manifest.NativeJobSource(resource)
	if err != nil {
		return bundleplan.Observation{}, err
	}
	job, _, err := parseJob(resource.SourcePath, source)
	if err != nil {
		return bundleplan.Observation{}, err
	}
	status, err := m.nomad.JobStatus(ctx, tracked.NomadID.String, desiredJobNamespace(job))
	if err != nil || status == nil || !status.Exists {
		return bundleplan.Observation{Present: status != nil && status.Exists}, err
	}
	job.ID = &tracked.NomadID.String
	jobPlan, err := m.nomad.PlanJob(ctx, job)
	if err != nil {
		return bundleplan.Observation{}, err
	}
	return bundleplan.Observation{Present: true, Matches: !jobPlanHasChanges(jobPlan)}, nil
}

func (m *Manager) adoptBundleJob(ctx context.Context, resource manifest.Resource) (managedResourceResult, error) {
	source, err := manifest.NativeJobSource(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	job, _, err := parseJob(resource.SourcePath, source)
	if err != nil {
		return managedResourceResult{}, err
	}
	status, err := m.nomad.JobStatus(ctx, resource.Name, desiredJobNamespace(job))
	if err != nil {
		return managedResourceResult{}, err
	}
	if status == nil || !status.Exists {
		return managedResourceResult{}, fmt.Errorf("Nomad job %q does not exist", resource.Name)
	}
	job.ID = &resource.Name
	jobPlan, err := m.nomad.PlanJob(ctx, job)
	if err != nil {
		return managedResourceResult{}, err
	}
	if jobPlanHasChanges(jobPlan) {
		return managedResourceResult{}, fmt.Errorf("Nomad job %q does not match desired bundle resource", resource.Name)
	}
	return managedResourceResult{NomadID: resource.Name, Namespace: desiredJobNamespace(job)}, nil
}

func desiredJobNamespace(job *api.Job) string {
	if job != nil && job.Namespace != nil {
		return *job.Namespace
	}
	return ""
}

func (m *Manager) applyJob(ctx context.Context, repoRecord *storage.Repository, jobFile repo.JobFile, snapshot *repo.Snapshot, job *api.Job, submission *api.JobSubmission) (string, error) {
	if job == nil || submission == nil {
		return "", errors.New("job and submission are required")
	}

	annotateJob(job, repoRecord, jobFile, snapshot, true)

	if err := m.nomad.RegisterJob(ctx, job, submission); err != nil {
		return "", err
	}
	return jobID(job), nil
}

func (m *Manager) ensureJobs(ctx context.Context, repoRecord *storage.Repository, snapshot *repo.Snapshot, commitChanged bool) error {
	jobFiles, err := snapshotJobFiles(snapshot)
	if err != nil {
		return err
	}

	repoFiles, err := m.files.ListByRepo(ctx, repoRecord.ID)
	if err != nil {
		return err
	}

	fileIndex := make(map[string]storage.RepoFile, len(repoFiles))
	for _, file := range repoFiles {
		fileIndex[file.Path] = file
	}

	seen := make(map[string]struct{}, len(jobFiles))

	for _, jobFile := range jobFiles {
		seen[jobFile.Path] = struct{}{}
		existing, tracked := fileIndex[jobFile.Path]
		job, submission, err := parseJob(jobFile.Path, jobFile.Content)
		if err != nil {
			m.logger.Error("job parse failed", "repo", repoRecord.Name, "file", jobFile.Path, "error", err)
			continue
		}
		namespace := desiredJobNamespace(job)

		if tracked && (existing.Status == "adoption_required" || existing.Status == "conflict") {
			status, statusErr := m.nomad.JobStatus(ctx, existing.JobID.String, namespace)
			if statusErr != nil {
				m.logger.Warn("unmanaged job status check failed", "repo", repoRecord.Name, "job_id", existing.JobID.String, "file", jobFile.Path, "error", statusErr)
				continue
			}
			if status != nil && status.Exists {
				if existing.Status == "conflict" {
					job.ID = &existing.JobID.String
					plan, planErr := m.nomad.PlanJob(ctx, job)
					if planErr == nil && !jobPlanHasChanges(plan) {
						if err := m.files.UpsertWithNamespaceAndState(ctx, repoRecord.ID, jobFile.Path, snapshot.CommitHash, existing.JobID.String, namespace, "adoption_required", "job exists in Nomad; explicit adoption is required", jobFile.DeleteMode); err != nil {
							return err
						}
					}
				}
				continue
			}
			tracked = false
		}

		if !tracked {
			candidateID := jobID(job)
			status, statusErr := m.nomad.JobStatus(ctx, candidateID, namespace)
			if statusErr != nil {
				m.logger.Warn("unmanaged job status check failed", "repo", repoRecord.Name, "job_id", candidateID, "file", jobFile.Path, "error", statusErr)
				continue
			}
			if status != nil && status.Exists {
				job.ID = &candidateID
				plan, planErr := m.nomad.PlanJob(ctx, job)
				ownershipStatus := "conflict"
				message := "job exists in Nomad but does not match the desired job"
				if planErr == nil && !jobPlanHasChanges(plan) {
					ownershipStatus = "adoption_required"
					message = "job exists in Nomad; explicit adoption is required"
				}
				if err := m.files.UpsertWithNamespaceAndState(ctx, repoRecord.ID, jobFile.Path, snapshot.CommitHash, candidateID, namespace, ownershipStatus, message, jobFile.DeleteMode); err != nil {
					return err
				}
				continue
			}
		}

		needApply := !tracked
		var trackedJobID string
		if tracked && existing.JobID.Valid {
			trackedJobID = existing.JobID.String
		}

		if tracked && !needApply {
			if trackedJobID == "" {
				needApply = true
			} else {
				status, err := m.nomad.JobStatus(ctx, trackedJobID, namespace)
				if err != nil {
					m.logger.Warn("job status check failed", "repo", repoRecord.Name, "job_id", trackedJobID, "file", jobFile.Path, "error", err)
					if commitChanged {
						needApply = true
					} else {
						continue
					}
				}
				if status == nil || !status.Exists {
					needApply = true
				}
			}
		}

		if tracked && !needApply {
			job.ID = &trackedJobID
			annotateJob(job, repoRecord, jobFile, snapshot, false)
			plan, err := m.nomad.PlanJob(ctx, job)
			if err != nil {
				m.logger.Warn("job plan failed", "repo", repoRecord.Name, "job_id", trackedJobID, "file", jobFile.Path, "error", err)
				needApply = true
			} else if !jobPlanHasChanges(plan) {
				if commitChanged || existing.Namespace.String != namespace || existing.Status != "applied" {
					if err := m.files.UpsertWithNamespaceAndState(ctx, repoRecord.ID, jobFile.Path, snapshot.CommitHash, trackedJobID, namespace, "applied", "", jobFile.DeleteMode); err != nil {
						return err
					}
				}
				continue
			} else {
				needApply = true
			}
		}

		if !needApply {
			continue
		}

		jobID, err := m.applyJob(ctx, repoRecord, jobFile, snapshot, job, submission)
		if err != nil {
			m.logger.Error("job apply failed", "repo", repoRecord.Name, "file", jobFile.Path, "error", err)
			continue
		}
		if err := m.files.UpsertWithNamespaceAndState(ctx, repoRecord.ID, jobFile.Path, snapshot.CommitHash, jobID, namespace, "applied", "", jobFile.DeleteMode); err != nil {
			return err
		}
	}

	for path, file := range fileIndex {
		if _, ok := seen[path]; ok {
			continue
		}
		if file.Status == "adoption_required" || file.Status == "conflict" {
			if err := m.files.Delete(ctx, repoRecord.ID, path); err != nil {
				return err
			}
			continue
		}
		if file.DeleteMode.Valid && file.DeleteMode.String == string(manifest.DeleteModeProtect) {
			if m.logger != nil {
				m.logger.Warn("job removal protected", "repo", repoRecord.Name, "file", path, "job_id", file.JobID.String)
			}
			continue
		}
		if file.JobID.Valid && file.JobID.String != "" {
			if err := m.nomad.DeregisterJob(ctx, file.JobID.String, file.Namespace.String, true); err != nil {
				if m.logger != nil {
					m.logger.Error("job deregister failed", "repo", repoRecord.Name, "job_id", file.JobID.String, "file", path, "error", err)
				}
				continue
			}
		}
		if err := m.files.Delete(ctx, repoRecord.ID, path); err != nil {
			return err
		}
		if m.logger != nil {
			m.logger.Info("job removed", "repo", repoRecord.Name, "file", path, "job_id", file.JobID.String)
		}
	}

	return nil
}

// AdoptJob records ownership of an existing legacy job after verifying that
// its live Nomad specification matches the job file.
func (m *Manager) AdoptJob(ctx context.Context, repoID int64, path string) error {
	m.repoMu.Lock()
	defer m.repoMu.Unlock()

	repoRecord, err := m.repos.Get(ctx, repoID)
	if err != nil {
		return err
	}
	if repoRecord == nil {
		return errors.New("repository not found")
	}
	snapshot, err := m.syncRepo(ctx, repoRecord)
	if err != nil {
		return err
	}
	if snapshot.Bundle != nil {
		return errors.New("repository contains a Compass bundle; adopt the bundle resource by address")
	}
	var jobFile *repo.JobFile
	for index := range snapshot.JobFiles {
		if snapshot.JobFiles[index].Path == path {
			jobFile = &snapshot.JobFiles[index]
			break
		}
	}
	if jobFile == nil {
		return fmt.Errorf("job file %q not found", path)
	}
	job, _, err := parseJob(jobFile.Path, jobFile.Content)
	if err != nil {
		return err
	}
	candidateID := jobID(job)
	namespace := desiredJobNamespace(job)
	status, err := m.nomad.JobStatus(ctx, candidateID, namespace)
	if err != nil {
		return err
	}
	if status == nil || !status.Exists {
		return fmt.Errorf("Nomad job %q does not exist", candidateID)
	}
	job.ID = &candidateID
	plan, err := m.nomad.PlanJob(ctx, job)
	if err != nil {
		return err
	}
	if jobPlanHasChanges(plan) {
		return fmt.Errorf("Nomad job %q does not match desired job", candidateID)
	}
	return m.files.UpsertWithNamespaceAndState(ctx, repoID, jobFile.Path, snapshot.CommitHash, candidateID, namespace, "applied", "", jobFile.DeleteMode)
}

func (m *Manager) ensureBundleJobs(ctx context.Context, repoRecord *storage.Repository, snapshot *repo.Snapshot, ordered []manifest.Resource, tracked map[string]storage.ManagedResource) error {
	for _, resource := range ordered {
		if resource.Kind != "job" {
			continue
		}

		source, err := manifest.NativeJobSource(resource)
		if err != nil {
			return err
		}
		job, submission, err := parseJob(resource.Address, source)
		if err != nil {
			return fmt.Errorf("parse bundle job %q: %w", resource.Address, err)
		}

		hash := manifest.SpecHash(resource)
		manifestHash := manifest.ManifestHash(resource)
		existing := tracked[resource.Address]
		jobFile := repo.JobFile{Path: resource.Address, Content: source, DeleteMode: string(resource.DeleteMode)}
		trackedJobID := existing.NomadID.String
		namespace := desiredJobNamespace(job)

		if existing.Address == "" {
			candidateID := jobID(job)
			status, statusErr := m.nomad.JobStatus(ctx, candidateID, namespace)
			if statusErr != nil {
				return fmt.Errorf("check ownership for bundle job %q: %w", resource.Address, statusErr)
			}
			if status != nil && status.Exists {
				return fmt.Errorf("bundle job %q collides with unmanaged Nomad job %q", resource.Address, candidateID)
			}
		}

		if existing.Address != "" && trackedJobID != "" {
			status, statusErr := m.nomad.JobStatus(ctx, trackedJobID, namespace)
			if statusErr != nil {
				if m.logger != nil {
					m.logger.Warn("bundle job status check failed", "repo", repoRecord.Name, "job", resource.Address, "error", statusErr)
				}
			} else if status != nil && status.Exists {
				job.ID = &trackedJobID
				annotateJob(job, repoRecord, jobFile, snapshot, false)
				plan, planErr := m.nomad.PlanJob(ctx, job)
				if planErr == nil && !jobPlanHasChanges(plan) {
					if err := m.upsertManagedResource(ctx, repoRecord.ID, snapshot, resource, trackedJobID, namespace, hash, manifestHash, "applied", "", ""); err != nil {
						return err
					}
					continue
				}
			}
		}

		appliedID, applyErr := m.applyJob(ctx, repoRecord, jobFile, snapshot, job, submission)
		if applyErr != nil {
			_ = m.upsertManagedResource(ctx, repoRecord.ID, snapshot, resource, trackedJobID, namespace, hash, manifestHash, "failed", applyErr.Error(), "")
			return fmt.Errorf("apply bundle job %q: %w", resource.Address, applyErr)
		}
		if err := m.upsertManagedResource(ctx, repoRecord.ID, snapshot, resource, appliedID, namespace, hash, manifestHash, "applied", "", ""); err != nil {
			return err
		}
	}
	return nil
}

func snapshotJobFiles(snapshot *repo.Snapshot) ([]repo.JobFile, error) {
	if snapshot == nil {
		return nil, errors.New("snapshot is required")
	}
	if snapshot.Bundle != nil {
		return nil, errors.New("bundle jobs use managed-resource tracking")
	}
	return snapshot.JobFiles, nil
}

func jobID(job *api.Job) string {
	if job == nil {
		return ""
	}
	if job.ID != nil && *job.ID != "" {
		return *job.ID
	}
	if job.Name != nil {
		return *job.Name
	}
	return ""
}

func parseJob(path string, contents []byte) (*api.Job, *api.JobSubmission, error) {
	cfg := &jobspec2.ParseConfig{Body: contents, Path: path, Strict: true}
	job, err := jobspec2.ParseWithConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	if job.Meta == nil {
		job.Meta = map[string]string{}
	}
	submission := &api.JobSubmission{Source: string(contents), Format: "hcl2"}
	return job, submission, nil
}

func jobPlanHasChanges(resp *api.JobPlanResponse) bool {
	if resp == nil {
		return true
	}
	return jobDiffHasChanges(resp.Diff)
}

func jobDiffHasChanges(diff *api.JobDiff) bool {
	if diff == nil {
		return false
	}
	if fieldDiffsHaveChanges(diff.Fields) || len(diff.Objects) > 0 {
		return true
	}
	for _, tg := range diff.TaskGroups {
		if taskGroupDiffHasChanges(tg) {
			return true
		}
	}
	return false
}

func taskGroupDiffHasChanges(diff *api.TaskGroupDiff) bool {
	if diff == nil {
		return false
	}
	if fieldDiffsHaveChanges(diff.Fields) || len(diff.Objects) > 0 {
		return true
	}
	for _, task := range diff.Tasks {
		if taskDiffHasChanges(task) {
			return true
		}
	}
	return false
}

func taskDiffHasChanges(diff *api.TaskDiff) bool {
	if diff == nil {
		return false
	}
	return fieldDiffsHaveChanges(diff.Fields) || len(diff.Objects) > 0
}

func fieldDiffsHaveChanges(fields []*api.FieldDiff) bool {
	for _, field := range fields {
		if field == nil || isCompassCommitMetadataField(field.Name) {
			continue
		}
		return true
	}
	return false
}

func isCompassCommitMetadataField(name string) bool {
	switch name {
	case nomadMetaFieldName(compassMetaCommit), nomadMetaFieldName(compassMetaCommitAuthor), nomadMetaFieldName(compassMetaCommitTitle):
		return true
	default:
		return false
	}
}

func nomadMetaFieldName(key string) string { return "Meta[" + key + "]" }

func annotateJob(job *api.Job, repoRecord *storage.Repository, jobFile repo.JobFile, snapshot *repo.Snapshot, includeCommitMetadata bool) {
	if job == nil {
		return
	}
	if job.Meta == nil {
		job.Meta = map[string]string{}
	}
	job.Meta[compassMetaRepoURL] = repoRecord.RepoURL
	job.Meta[compassMetaRepoName] = repoRecord.Name
	job.Meta[compassMetaJobFile] = jobFile.Path
	if includeCommitMetadata && snapshot != nil {
		job.Meta[compassMetaCommit] = snapshot.CommitHash
		job.Meta[compassMetaCommitAuthor] = snapshot.CommitAuthor
		job.Meta[compassMetaCommitTitle] = snapshot.CommitTitle
	}
}
