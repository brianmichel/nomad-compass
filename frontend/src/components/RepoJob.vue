<template>
  <tr
    class="repo-job-row"
    :class="{ 'repo-job-row--clickable': job.job_url }"
    @click="handleRowClick"
    @keydown.enter.prevent="handleRowActivate"
    @keydown.space.prevent="handleRowActivate"
    :tabindex="job.job_url ? 0 : undefined"
    :role="job.job_url ? 'link' : undefined"
  >
    <td class="job-cell job-cell-name" :data-label="compact ? 'Job' : null">
      <div class="job-name-row">
        <span class="job-name">
          {{ jobName }}
        </span>
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
      <div v-if="hasAllocationProgress" class="allocation-details">
        <div class="allocation-progress-row">
          <div
            class="allocation-progress"
            :class="`is-${allocationProgressState}`"
            role="progressbar"
            :aria-valuenow="allocationProgressPercent"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-valuetext="allocationProgressLabel"
          >
            <span
              class="allocation-progress__fill"
              :style="{ width: allocationProgressPercent + '%' }"
            ></span>
          </div>
          <span class="allocation-progress__value">{{ allocationProgressDisplay }}</span>
        </div>
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
const runningAllocations = computed(() => props.job.running_allocations ?? 0);
const pendingAllocations = computed(
  () => (props.job.starting_allocations ?? 0) + (props.job.queued_allocations ?? 0),
);
const failedAllocations = computed(
  () => (props.job.failed_allocations ?? 0) + (props.job.lost_allocations ?? 0),
);
const unknownAllocations = computed(() => props.job.unknown_allocations ?? 0);
const compact = computed(() => props.compact ?? false);

type RepoJobWithNomadNamespace = RepoJob & { nomad_namespace?: string | null };

function resolveNamespace(job: RepoJob): string | null {
  const extended = job as RepoJobWithNomadNamespace;
  return extended.namespace || extended.nomad_namespace || null;
}

const allocationTotals = computed(() => {
  const running = runningAllocations.value;
  const pending = pendingAllocations.value;
  const failed = failedAllocations.value;
  const unknown = unknownAllocations.value;
  const total = running + pending + failed + unknown;
  const desired = desiredAllocations.value;
  const denominator = desired > 0 ? desired : total;

  return { running, pending, failed, unknown, total, denominator };
});

const hasAllocationProgress = computed(() => allocationTotals.value.denominator > 0);

const allocationProgressPercent = computed(() => {
  const totals = allocationTotals.value;
  if (!totals.denominator) {
    return 0;
  }
  const percent = (totals.running / totals.denominator) * 100;
  return Math.max(0, Math.min(100, Math.round(percent)));
});

const allocationProgressState = computed(() => {
  if (!hasAllocationProgress.value) {
    return 'empty';
  }
  if (desiredAllocations.value > 0 && runningAllocations.value >= desiredAllocations.value) {
    return 'complete';
  }
  if (runningAllocations.value > 0) {
    return 'partial';
  }
  return 'empty';
});

const allocationProgressDisplay = computed(() => {
  if (!hasAllocationProgress.value) {
    return '';
  }
  if (desiredAllocations.value > 0) {
    return `${runningAllocations.value}/${desiredAllocations.value}`;
  }
  return `${runningAllocations.value}`;
});

const allocationProgressLabel = computed(() => {
  const totals = allocationTotals.value;
  if (!totals.denominator) {
    return '';
  }

  const parts: string[] = [];
  const desired = desiredAllocations.value;
  if (desired > 0) {
    parts.push(`Running ${totals.running}/${desired}`);
  } else {
    parts.push(`Running ${totals.running}`);
  }

  if (totals.pending > 0) {
    parts.push(`${totals.pending} pending`);
  }
  if (totals.failed > 0) {
    parts.push(`${totals.failed} failed`);
  }
  if (totals.unknown > 0) {
    parts.push(`${totals.unknown} unknown`);
  }

  return parts.join(', ');
});

function capitalize(value: string) {
  if (!value.length) return value;
  return value[0].toUpperCase() + value.slice(1);
}

function handleRowClick(event: MouseEvent) {
  if (!props.job.job_url) {
    return;
  }

  const target = event.target as HTMLElement | null;
  if (target && (target.tagName === 'A' || target.closest('a') || target.closest('button'))) {
    return;
  }

  window.open(props.job.job_url, '_blank', 'noopener,noreferrer');
}

function handleRowActivate() {
  if (!props.job.job_url) {
    return;
  }
  window.open(props.job.job_url, '_blank', 'noopener,noreferrer');
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

.repo-job-row--clickable {
  cursor: pointer;
}

.repo-job-row--clickable td {
  cursor: pointer;
}

.repo-job-row--clickable .job-name {
  color: var(--color-text-primary);
  transition: color var(--transition-fast);
}

.repo-job-row--clickable:hover .job-name,
.repo-job-row--clickable:focus-visible .job-name {
  color: var(--color-accent-hover);
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
  text-align: left;
}

.allocation-details {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 0.35rem;
}

.allocation-progress-row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  width: 100%;
}

.allocation-progress {
  position: relative;
  flex: 1 1 auto;
  width: 100%;
  height: 0.35rem;
  border-radius: 999px;
  background: var(--color-surface-muted);
  overflow: hidden;
}

.allocation-progress::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: inherit;
  border: 1px solid rgba(148, 163, 184, 0.3);
  pointer-events: none;
}

.allocation-progress__fill {
  display: block;
  height: 100%;
  transition: width var(--transition-fast);
  background: var(--jobs-bar-healthy);
}

.allocation-progress.is-complete .allocation-progress__fill {
  background: var(--jobs-bar-healthy);
}

.allocation-progress.is-partial .allocation-progress__fill {
  background: var(--jobs-bar-healthy);
}

.allocation-progress.is-empty .allocation-progress__fill {
  background: transparent;
}

.allocation-progress__value {
  flex: 0 0 auto;
  font-size: 0.78rem;
  color: var(--color-text-secondary);
  font-variant-numeric: tabular-nums;
  text-align: right;
  min-width: 2.5rem;
}

.allocation-empty {
  font-size: 0.75rem;
  color: var(--color-text-tertiary);
}

.job-status-badge {
  font-size: 0.84rem;
  padding: 0.26rem 0.4rem;
  border-radius: 4px;
  border: 1px solid var(--status-unknown-border);
  background: var(--status-unknown-bg);
  color: var(--status-unknown-text);
  font-weight: 600;
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
  .allocation-progress-row {
    justify-content: flex-start;
  }
}
</style>
