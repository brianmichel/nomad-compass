<template>
  <section class="panel glass repo-panel">
    <header class="panel-header">
      <div>
        <h2>Tracked repositories</h2>
        <p v-if="repos.length">Showing {{ repos.length }} sources.</p>
        <p v-else>Onboard your first repository to start reconciling jobs.</p>
      </div>
    </header>

    <ul v-if="repos.length" class="repo-list">
      <li
        v-for="repo in repos"
        :key="repo.id"
        class="repo-card"
        :class="{ active: syncingRepoId === repo.id }"
      >
        <div class="repo-header">
          <div>
            <span class="repo-name">{{ repo.name }}</span>
            <span class="repo-branch">{{ repo.branch }}</span>
          </div>
          <div class="repo-actions">
            <button class="ghost small" type="button" @click="$emit('reconcile', repo)" :disabled="syncingRepoId === repo.id">
              <span v-if="syncingRepoId === repo.id" class="loader"></span>
              <span v-else>Sync now</span>
            </button>
            <button
              class="ghost danger small"
              type="button"
              @click="$emit('delete', repo)"
              :disabled="deletingRepoId === repo.id"
            >
              <span v-if="deletingRepoId === repo.id" class="loader"></span>
              <span v-else>Delete</span>
            </button>
          </div>
        </div>

        <div class="repo-meta">
          <span class="meta-pill" :title="repo.repo_url">
            <span class="label">Source</span>
            {{ repo.repo_url }}
          </span>
          <span class="meta-pill" :title="repo.job_path">
            <span class="label">Job path</span>
            {{ repo.job_path }}
          </span>
          <span class="meta-pill">
            <span class="label">Credential</span>
            {{ repo.credential_id ? 'Managed secret' : 'Public' }}
          </span>
        </div>

        <div class="repo-commit">
          <span class="commit-hash" :title="repo.last_commit || 'Awaiting first run'">
            #{{ commitDisplay(repo.last_commit) }}
          </span>
          <span v-if="repo.last_commit_title" class="commit-title">{{ repo.last_commit_title }}</span>
          <span v-else class="commit-title muted">No commits reconciled yet.</span>
        </div>

        <div class="repo-jobs">
          <template v-if="repo.jobs.length">
            <div
              v-for="job in repo.jobs"
              :key="job.path"
              class="job-row"
            >
              <div class="job-info">
                <div class="job-heading">
                  <span class="job-name">{{ jobLabel(job) }}</span>
                  <a
                    v-if="hasMultipleAllocations(job) && job.job_url"
                    class="job-allocations"
                    :href="job.job_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    title="View allocations in Nomad"
                  >
                    <span
                      v-for="allocation in job.allocations"
                      :key="allocation.id"
                      class="allocation-square"
                      :class="allocationStatusClass(allocation)"
                    ></span>
                  </a>
                  <a
                    v-else-if="job.latest_allocation_id && job.job_url"
                    class="job-allocation"
                    :href="job.job_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    :title="allocationTooltip(job)"
                  >
                    {{ shortAllocation(job.latest_allocation_id) }}
                  </a>
                  <a
                    v-else-if="isBatch(job) && job.job_url"
                    class="job-batch"
                    :href="job.job_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    title="View batch job in Nomad"
                  >
                    Batch
                  </a>
                </div>
                <span class="job-path">{{ job.path }}</span>
              </div>
              <span
                class="job-status-badge"
                :class="jobStatusClass(job)"
                :title="jobTooltip(job)"
              >
                {{ jobStatusLabel(job) }}
              </span>
            </div>
          </template>
          <p v-else class="job-empty">No Nomad jobs registered yet.</p>
        </div>

        <div class="repo-footer">
          <span>
            <strong>Author:</strong>
            <span>{{ repo.last_commit_author || '—' }}</span>
          </span>
          <span>
            <strong>Last polled:</strong>
            <span>{{ formatTimestamp(repo.last_polled_at) }}</span>
          </span>
        </div>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import type { Repo, RepoJob, AllocationStatus } from '../composables/useCompassStore';

defineProps<{
  repos: Repo[];
  syncingRepoId: number | null;
  deletingRepoId: number | null;
}>();

defineEmits<{
  (e: 'reconcile', repo: Repo): void;
  (e: 'delete', repo: Repo): void;
}>();

function commitDisplay(commit?: string | null) {
  if (!commit) return 'pending';
  return commit.slice(0, 7);
}

function formatTimestamp(value?: string | null) {
  if (!value) return 'Awaiting first poll';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function jobLabel(job: RepoJob) {
  return job.job_name || job.job_id || job.path;
}

function jobStatusLabel(job: RepoJob) {
  if (job.status_error) {
    return 'Error';
  }
  const status = (job.status || job.nomad_status || '').toLowerCase();
  switch (status) {
    case 'healthy':
      return 'Healthy';
    case 'deploying':
      return 'Deploying';
    case 'degraded':
      return 'Degraded';
    case 'failed':
      return 'Failed';
    case 'lost':
      return 'Lost';
    case 'pending':
      return 'Pending';
    case 'dead':
      return 'Stopped';
    case 'missing':
      return 'Missing';
    default:
      if (!job.job_id) {
        return 'Pending';
      }
      if (!status) {
        return 'Unknown';
      }
      return capitalize(status);
  }
}

function jobStatusClass(job: RepoJob) {
  if (job.status_error) {
    return 'danger';
  }
  const normalized = (job.status || job.nomad_status || '').toLowerCase();
  if (['healthy', 'running', 'successful', 'complete'].includes(normalized)) {
    return 'healthy';
  }
  if (['deploying', 'pending', 'queued', 'evaluating', 'starting'].includes(normalized)) {
    return 'pending';
  }
  if (['degraded'].includes(normalized)) {
    return 'warning';
  }
  if (['failed', 'dead', 'lost', 'missing', 'cancelled'].includes(normalized)) {
    return 'danger';
  }
  return 'unknown';
}

function jobTooltip(job: RepoJob) {
  if (job.status_error) {
    return job.status_error;
  }
  if (job.status_description) {
    return job.status_description;
  }
  if (job.nomad_status) {
    return `Nomad status: ${capitalize(job.nomad_status)}`;
  }
  if (!job.job_id) {
    return 'Job has not been registered with Nomad yet.';
  }
  return 'Status is unavailable.';
}

function shortAllocation(id?: string) {
  if (!id) {
    return '';
  }
  return id.slice(0, 8);
}

function allocationTooltip(job: RepoJob) {
  const parts: string[] = [];
  if (job.latest_allocation_name) {
    parts.push(job.latest_allocation_name);
  }
  if (job.latest_allocation_id) {
    parts.push(job.latest_allocation_id);
  }
  return parts.join(' • ') || 'View in Nomad';
}

function isBatch(job: RepoJob) {
  return (job.job_type || '').toLowerCase() === 'batch';
}

function hasMultipleAllocations(job: RepoJob) {
  return Array.isArray(job.allocations) && job.allocations.length > 1;
}

function allocationStatusClass(allocation: AllocationStatus) {
  const status = (allocation.status || '').toLowerCase();
  if (['running', 'complete', 'successful'].includes(status)) {
    return 'healthy';
  }
  if (['starting', 'pending', 'queued', 'evaluating'].includes(status)) {
    return 'pending';
  }
  if (['failed', 'lost', 'dead', 'missing', 'cancelled'].includes(status)) {
    return 'danger';
  }
  if (allocation.healthy === false) {
    return 'danger';
  }
  if (allocation.healthy === true) {
    return 'healthy';
  }
  return 'healthy';
}

function capitalize(value: string) {
  if (!value.length) return value;
  return value[0].toUpperCase() + value.slice(1);
}
</script>

<style scoped>
.repo-jobs {
  margin-top: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.job-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem;
  border-radius: 0.9rem;
  background: rgba(30, 41, 59, 0.55);
  border: 1px solid rgba(51, 65, 85, 0.5);
  box-shadow: inset 0 0 0 1px rgba(148, 163, 184, 0.08);
  gap: 1rem;
}

.job-info {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.job-heading {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.job-name {
  font-weight: 600;
  color: #e2e8f0;
}

.job-allocation {
  font-size: 0.75rem;
  font-weight: 500;
  color: #38bdf8;
  text-decoration: none;
  padding: 0.1rem 0.4rem;
  border-radius: 0.5rem;
  background: rgba(56, 189, 248, 0.15);
  border: 1px solid rgba(56, 189, 248, 0.35);
}

.job-allocation:hover {
  background: rgba(56, 189, 248, 0.25);
  color: #f0f9ff;
}

.job-allocations {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.15rem 0.3rem;
  border-radius: 0.6rem;
  border: 1px solid rgba(34, 197, 94, 0.35);
  background: rgba(34, 197, 94, 0.12);
  text-decoration: none;
}

.job-allocations:hover {
  border-color: rgba(34, 197, 94, 0.6);
  background: rgba(34, 197, 94, 0.2);
}

.allocation-square {
  width: 0.55rem;
  height: 0.55rem;
  border-radius: 0.2rem;
  background: #22c55e;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
}

.allocation-square.pending {
  background: #fbbf24;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.1);
}

.allocation-square.danger {
  background: #f87171;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.1);
}

.job-batch {
  font-size: 0.75rem;
  font-weight: 600;
  color: #fbbf24;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  text-decoration: none;
  padding: 0.1rem 0.45rem;
  border-radius: 0.5rem;
  background: rgba(251, 191, 36, 0.12);
  border: 1px solid rgba(251, 191, 36, 0.35);
}

.job-batch:hover {
  background: rgba(251, 191, 36, 0.22);
  color: #fef3c7;
}

.job-path {
  font-size: 0.8rem;
  color: rgba(148, 163, 184, 0.85);
}

.job-status-badge {
  font-size: 0.75rem;
  padding: 0.25rem 0.65rem;
  border-radius: 999px;
  text-transform: none;
  border: 1px solid rgba(148, 163, 184, 0.35);
  background: rgba(148, 163, 184, 0.2);
  color: rgba(226, 232, 240, 0.85);
  white-space: nowrap;
}

.job-status-badge.healthy {
  background: rgba(34, 197, 94, 0.2);
  border-color: rgba(34, 197, 94, 0.4);
  color: #bbf7d0;
}

.job-status-badge.pending {
  background: rgba(251, 191, 36, 0.18);
  border-color: rgba(251, 191, 36, 0.4);
  color: #fef3c7;
}

.job-status-badge.warning {
  background: rgba(96, 165, 250, 0.2);
  border-color: rgba(96, 165, 250, 0.45);
  color: #bfdbfe;
}

.job-status-badge.danger {
  background: rgba(248, 113, 113, 0.2);
  border-color: rgba(248, 113, 113, 0.4);
  color: #fecaca;
}

.job-status-badge.unknown {
  background: rgba(100, 116, 139, 0.25);
  border-color: rgba(100, 116, 139, 0.35);
  color: rgba(226, 232, 240, 0.8);
}

.job-empty {
  margin-top: 1rem;
  font-size: 0.9rem;
  color: rgba(148, 163, 184, 0.85);
}
</style>
