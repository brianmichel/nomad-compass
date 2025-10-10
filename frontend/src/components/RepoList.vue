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
                <span class="job-name">{{ jobLabel(job) }}</span>
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
import type { Repo, RepoJob } from '../composables/useCompassStore';

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
  if (!job.job_id) {
    return 'Pending';
  }
  if (!job.status) {
    return 'Unknown';
  }
  return capitalize(job.status);
}

function jobStatusClass(job: RepoJob) {
  if (job.status_error) {
    return 'danger';
  }
  if (!job.job_id) {
    return 'pending';
  }
  if (!job.status) {
    return 'unknown';
  }

  const normalized = job.status.toLowerCase();
  if (['running', 'complete', 'successful'].includes(normalized)) {
    return 'healthy';
  }
  if (['pending', 'queued', 'evaluating'].includes(normalized)) {
    return 'pending';
  }
  if (['failed', 'dead', 'lost', 'missing'].includes(normalized)) {
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
  if (!job.job_id) {
    return 'Job has not been registered with Nomad yet.';
  }
  if (!job.status) {
    return 'Status is unavailable.';
  }
  return capitalize(job.status);
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

.job-name {
  font-weight: 600;
  color: #e2e8f0;
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
