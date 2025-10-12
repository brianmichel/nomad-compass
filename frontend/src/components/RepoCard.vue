<template>
  <tr class="repo-row" :class="{ syncing: isSyncing }">
    <td class="cell-toggle">
      <button
        class="row-toggle"
        type="button"
        :aria-expanded="expanded.toString()"
        :aria-label="`Toggle details for ${repo.name}`"
        @click="toggleExpanded"
      >
        <span class="caret" :class="{ open: expanded }"></span>
      </button>
    </td>
    <td class="cell-name">
      <div class="name-block">
        <span class="repo-name">{{ repo.name }}</span>
        <span class="job-path" :title="repo.job_path">{{ repo.job_path }}</span>
      </div>
    </td>
    <td class="cell-source">
      <a
        :href="repo.repo_url"
        class="source-link"
        :title="repo.repo_url"
        target="_blank"
        rel="noopener noreferrer"
      >
        {{ repo.repo_url }}
      </a>
    </td>
    <td class="cell-branch">
      <span class="branch-chip" v-if="repo.branch">{{ repo.branch }}</span>
    </td>
    <td class="cell-credential">{{ credentialLabel }}</td>
    <td class="cell-timestamp">
      <template v-if="repo.last_polled_at">
        <time
          class="polled-time"
          :datetime="lastPolledDatetime"
          :title="lastPolledAbsolute"
        >
          {{ lastPolledRelative }}
        </time>
      </template>
      <span v-else class="polled-pending">Awaiting poll</span>
    </td>
    <td class="cell-jobs">
      <div v-if="jobSummaries.length" class="job-statuses">
        <span
          v-for="summary in jobSummaries"
          :key="summary.key"
          class="job-status-chip"
          :class="summary.state"
          :title="summary.tooltip"
        >
          {{ summary.label }}
        </span>
        <span v-if="jobOverflow > 0" class="job-status-chip more">+{{ jobOverflow }}</span>
      </div>
      <span v-else class="job-status-empty">No jobs</span>
    </td>
    <td class="cell-actions">
      <button
        class="ghost small"
        type="button"
        @click="emit('reconcile', repo)"
        :disabled="isSyncing"
      >
        <span v-if="isSyncing" class="loader"></span>
        <span v-else>Sync now</span>
      </button>
      <button
        class="ghost danger small"
        type="button"
        @click="emit('delete', repo)"
        :disabled="isDeleting"
      >
        <span v-if="isDeleting" class="loader"></span>
        <span v-else>Delete</span>
      </button>
    </td>
  </tr>
  <tr v-if="expanded" class="repo-detail-row">
    <td colspan="8">
      <div class="repo-detail">
        <RepoPollingInfo :repo="repo" />
        <RepoJobList :jobs="repo.jobs" />
      </div>
    </td>
  </tr>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import RepoJobList from './RepoJobList.vue';
import RepoPollingInfo from './RepoPollingInfo.vue';
import type { Repo } from '@/types';
import { formatRelativeTime, formatTimestamp } from '@/utils/date';
import { getJobStatusClass, getJobStatusLabel, getJobStatusTooltip } from '@/utils/jobStatus';

const props = defineProps<{
  repo: Repo;
  syncingRepoId: number | null;
  deletingRepoId: number | null;
}>();

const emit = defineEmits<{
  (e: 'reconcile', repo: Repo): void;
  (e: 'delete', repo: Repo): void;
}>();

const expanded = ref(false);

const isSyncing = computed(() => props.syncingRepoId === props.repo.id);
const isDeleting = computed(() => props.deletingRepoId === props.repo.id);
const credentialLabel = computed(() => (props.repo.credential_id ? 'Managed secret' : 'Public'));
const lastPolledRelative = computed(() => formatRelativeTime(props.repo.last_polled_at));
const lastPolledAbsolute = computed(() => formatTimestamp(props.repo.last_polled_at));
const lastPolledDatetime = computed(() => props.repo.last_polled_at ?? undefined);

const jobSummaries = computed(() =>
  (props.repo.jobs ?? []).slice(0, 4).map((job) => ({
    key: job.job_id || job.path,
    label: getJobStatusLabel(job),
    state: getJobStatusClass(job),
    tooltip: getJobStatusTooltip(job),
  })),
);

const jobOverflow = computed(() => Math.max(0, (props.repo.jobs?.length ?? 0) - jobSummaries.value.length));

function toggleExpanded() {
  expanded.value = !expanded.value;
}
</script>

<style scoped>
.repo-row td {
  vertical-align: top;
}

.repo-row.syncing td {
  background: rgba(23, 106, 209, 0.06);
}

.cell-toggle {
  width: 36px;
  padding-right: 0;
}

.row-toggle {
  width: 26px;
  height: 26px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: border-color var(--transition-fast), background var(--transition-fast);
}

.row-toggle:hover,
.row-toggle:focus-visible {
  border-color: var(--color-accent);
  background: var(--color-surface-muted);
}

.caret {
  width: 0;
  height: 0;
  border-top: 5px solid transparent;
  border-bottom: 5px solid transparent;
  border-left: 6px solid var(--color-text-tertiary);
  transition: transform var(--transition-fast);
}

.caret.open {
  transform: rotate(90deg);
}

.cell-name {
  max-width: 220px;
}

.name-block {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.repo-name {
  font-weight: 600;
  color: var(--color-text-primary);
  font-size: 0.96rem;
}

.job-path {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--color-text-subtle);
  overflow: hidden;
  text-overflow: ellipsis;
}

.source-link {
  display: inline-block;
  max-width: 280px;
  font-size: 0.88rem;
  color: var(--color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
}

.source-link:hover,
.source-link:focus-visible {
  color: var(--color-accent);
}

.branch-chip {
  display: inline-flex;
  align-items: center;
  padding: 0.2rem 0.55rem;
  border-radius: var(--radius-pill);
  border: 1px solid var(--color-border);
  background: var(--color-surface-muted);
  font-size: 0.75rem;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
}

.cell-credential {
  white-space: nowrap;
  font-size: 0.88rem;
}

.cell-timestamp {
  font-size: 0.86rem;
  white-space: nowrap;
}

.polled-time {
  color: var(--color-text-secondary);
}

.polled-pending {
  color: var(--color-text-subtle);
  font-style: italic;
}

.cell-jobs {
  min-width: 180px;
}

.job-statuses {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.job-status-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.18rem 0.5rem;
  border-radius: var(--radius-pill);
  border: 1px solid var(--status-unknown-border);
  background: var(--status-unknown-bg);
  color: var(--status-unknown-text);
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.job-status-chip.healthy {
  border-color: var(--status-healthy-border);
  background: var(--status-healthy-bg);
  color: var(--status-healthy-text);
}

.job-status-chip.pending {
  border-color: var(--status-pending-border);
  background: var(--status-pending-bg);
  color: var(--status-pending-text);
}

.job-status-chip.warning {
  border-color: var(--status-warning-border);
  background: var(--status-warning-bg);
  color: var(--status-warning-text);
}

.job-status-chip.danger {
  border-color: var(--status-danger-border);
  background: var(--status-danger-bg);
  color: var(--status-danger-text);
}

.job-status-chip.more {
  border-style: dashed;
}

.job-status-empty {
  color: var(--color-text-subtle);
  font-size: 0.85rem;
}

.cell-actions {
  text-align: right;
  white-space: nowrap;
  display: flex;
  justify-content: flex-end;
  gap: 0.4rem;
}

.repo-detail-row td {
  padding: 0;
  border-top: none;
  background: var(--color-surface-subtle);
}

.repo-detail-row:hover td {
  background: var(--color-surface-subtle);
}

.repo-detail {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.2rem 1.4rem;
}

@media (max-width: 960px) {
  .cell-source,
  .cell-credential {
    max-width: 200px;
  }

  .cell-jobs {
    min-width: 140px;
  }
}

@media (max-width: 768px) {
  .cell-source,
  .cell-branch {
    display: none;
  }

  .repo-detail {
    padding: 1rem;
  }
}
</style>
