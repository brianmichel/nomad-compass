<template>
  <tr class="repo-job-row">
    <td class="job-cell job-cell-name" :data-label="compact ? 'Job' : null">
      <div class="job-name-row">
        <a
          v-if="job.job_url"
          class="job-name"
          :href="job.job_url"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ jobName }}
        </a>
        <span v-else class="job-name">
          {{ jobName }}
        </span>
        <a
          v-if="job.job_url"
          class="job-open-link"
          :href="job.job_url"
          target="_blank"
          rel="noopener noreferrer"
        >
          Open →
        </a>
      </div>
      <div class="job-path">{{ job.path }}</div>
    </td>
    <td class="job-cell job-cell-status" :data-label="compact ? 'Status' : null">
      <span class="job-status-badge" :class="statusClass" :title="statusTooltip">
        {{ statusLabel }}
      </span>
    </td>
    <td class="job-cell job-cell-type" :data-label="compact ? 'Type' : null">
      <span v-if="jobTypeDisplay" class="job-type-chip">{{ jobTypeDisplay }}</span>
      <span v-else class="job-type-chip job-type-chip--muted">—</span>
    </td>
    <td class="job-cell job-cell-namespace" :data-label="compact ? 'Namespace' : null">
      <span class="job-namespace">{{ jobNamespaceDisplay }}</span>
    </td>
    <td class="job-cell job-cell-allocations" :data-label="compact ? 'Allocations' : null">
      <div v-if="hasAllocationSummary" class="allocation-summary">
        <span
          v-for="item in allocationSummary"
          :key="item.key"
          class="allocation-pill"
          :class="item.class"
        >
          {{ item.displayCount }} {{ item.label }}
        </span>
      </div>
      <span v-else class="allocation-empty">No data</span>
    </td>
  </tr>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { RepoJob } from '@/types';
import { getJobStatusClass, getJobStatusLabel, getJobStatusTooltip } from '@/utils/jobStatus';

const props = defineProps<{
  job: RepoJob;
  compact?: boolean;
}>();

const jobName = computed(() => props.job.job_name || props.job.job_id || props.job.path);
const statusClass = computed(() => getJobStatusClass(props.job));
const statusLabel = computed(() => getJobStatusLabel(props.job));
const statusTooltip = computed(() => getJobStatusTooltip(props.job));
const jobType = computed(() => (props.job.job_type || '').toLowerCase());
const jobTypeDisplay = computed(() => (jobType.value ? capitalize(jobType.value) : null));
const jobNamespace = computed(() => resolveNamespace(props.job));
const jobNamespaceDisplay = computed(() => jobNamespace.value || '—');
const desiredAllocations = computed(() => props.job.desired_allocations ?? 0);
const compact = computed(() => props.compact ?? false);

type RepoJobWithNomadNamespace = RepoJob & { nomad_namespace?: string | null };

function resolveNamespace(job: RepoJob): string | null {
  const extended = job as RepoJobWithNomadNamespace;
  return extended.namespace || extended.nomad_namespace || null;
}

const allocationSummary = computed(() => {
  const running = props.job.running_allocations ?? 0;
  const pending =
    (props.job.starting_allocations ?? 0) + (props.job.queued_allocations ?? 0);
  const failed = (props.job.failed_allocations ?? 0) + (props.job.lost_allocations ?? 0);
  const unknown = props.job.unknown_allocations ?? 0;

  const items: Array<{ key: string; label: string; class: string; displayCount: string }> = [];

  if (desiredAllocations.value > 0 || running > 0) {
    const displayCount =
      desiredAllocations.value > 0 ? `${running}/${desiredAllocations.value}` : `${running}`;
    items.push({
      key: 'running',
      label: 'RUNNING',
      class: 'healthy',
      displayCount,
    });
  }

  if (pending > 0) {
    items.push({
      key: 'pending',
      label: 'PENDING',
      class: 'pending',
      displayCount: `${pending}`,
    });
  }

  if (failed > 0) {
    items.push({
      key: 'failed',
      label: 'FAILED',
      class: 'danger',
      displayCount: `${failed}`,
    });
  }

  if (unknown > 0) {
    items.push({
      key: 'unknown',
      label: 'UNKNOWN',
      class: 'muted',
      displayCount: `${unknown}`,
    });
  }

  return items;
});

const hasAllocationSummary = computed(() => allocationSummary.value.length > 0);

function capitalize(value: string) {
  if (!value.length) return value;
  return value[0].toUpperCase() + value.slice(1);
}
</script>

<style scoped>
.repo-job-row td {
  vertical-align: middle;
  background: var(--color-surface);
}

.repo-job-row:hover td {
  background: rgba(148, 163, 184, 0.12);
}

.job-cell-name {
  min-width: 220px;
}

.job-name-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.job-name {
  font-weight: 600;
  color: var(--color-text-primary);
  font-size: 0.92rem;
  text-decoration: none;
}

.job-name:hover,
.job-name:focus-visible {
  color: var(--color-accent);
}

.job-open-link {
  font-size: 0.72rem;
  color: var(--color-accent);
  text-decoration: none;
  letter-spacing: 0.04em;
}

.job-open-link:hover,
.job-open-link:focus-visible {
  color: var(--color-accent-hover);
}

.job-path {
  margin-top: 0.2rem;
  font-size: 0.7rem;
  color: var(--color-text-subtle);
  font-family: var(--font-mono);
  word-break: break-word;
}

.job-cell-status {
  min-width: 120px;
}

.job-cell-type {
  min-width: 110px;
}

.job-cell-namespace {
  min-width: 110px;
}

.job-type-chip {
  display: inline-block;
  font-size: 0.76rem;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.job-type-chip--muted {
  color: var(--color-text-tertiary);
}

.job-namespace {
  font-size: 0.78rem;
  color: var(--color-text-secondary);
}

.job-cell-allocations {
  min-width: 200px;
}

.allocation-summary {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.allocation-pill {
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  padding: 0.12rem 0.38rem;
  border-radius: 4px;
  border: 1px solid var(--status-unknown-border);
  background: var(--status-unknown-bg);
  color: var(--status-unknown-text);
}

.allocation-pill.healthy {
  border-color: var(--status-healthy-border);
  background: var(--status-healthy-bg);
  color: var(--status-healthy-text);
}

.allocation-pill.pending {
  border-color: var(--status-pending-border);
  background: var(--status-pending-bg);
  color: var(--status-pending-text);
}

.allocation-pill.danger {
  border-color: var(--status-danger-border);
  background: var(--status-danger-bg);
  color: var(--status-danger-text);
}

.allocation-pill.muted {
  border-color: var(--color-border);
  background: var(--color-surface);
  color: var(--color-text-tertiary);
}

.allocation-empty {
  font-size: 0.75rem;
  color: var(--color-text-tertiary);
}

.job-status-badge {
  font-size: 0.74rem;
  padding: 0.16rem 0.5rem;
  border-radius: 4px;
  border: 1px solid var(--status-unknown-border);
  background: var(--status-unknown-bg);
  color: var(--status-unknown-text);
  white-space: nowrap;
}

.job-status-badge.healthy {
  background: var(--status-healthy-bg);
  border-color: var(--status-healthy-border);
  color: var(--status-healthy-text);
}

.job-status-badge.pending {
  background: var(--status-pending-bg);
  border-color: var(--status-pending-border);
  color: var(--status-pending-text);
}

.job-status-badge.warning {
  background: var(--status-warning-bg);
  border-color: var(--status-warning-border);
  color: var(--status-warning-text);
}

.job-status-badge.danger {
  background: var(--status-danger-bg);
  border-color: var(--status-danger-border);
  color: var(--status-danger-text);
}

.job-status-badge.unknown {
  background: var(--status-unknown-bg);
  border-color: var(--status-unknown-border);
  color: var(--status-unknown-text);
}

@media (max-width: 720px) {
  .allocation-summary {
    justify-content: flex-start;
  }
}
</style>
