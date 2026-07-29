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
          <IconExternalLink aria-hidden="true" />
          <span class="sr-only">(opens in a new tab)</span>
        </a>
        <span v-else class="job-name">{{ jobName }}</span>
      </div>
      <div class="job-path">{{ job.path }}</div>
    </td>
    <td class="job-cell job-cell-status" :data-label="compact ? 'Status' : null">
      <span class="badge badge-sm job-status-badge" :class="statusBadgeClass" :title="statusTooltip">
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
import { IconExternalLink } from '@tabler/icons-vue';
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
const statusBadgeClass = computed(() => {
  if (statusClass.value === 'healthy') return 'badge-success';
  if (statusClass.value === 'pending') return 'badge-ghost';
  if (statusClass.value === 'warning') return 'badge-warning';
  if (statusClass.value === 'danger') return 'badge-error';
  return 'badge-neutral';
});
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
  if (!hasAllocationProgress.value) return 'empty';
  if (failedAllocations.value > 0) return 'failed';
  if (pendingAllocations.value > 0) return 'pending';
  if (desiredAllocations.value > 0 && runningAllocations.value >= desiredAllocations.value) return 'complete';
  if (runningAllocations.value > 0) return 'partial';
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
  min-width: 190px;
}

.job-name-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.job-name {
  display: inline-flex;
  align-items: center;
  gap: .3rem;
  color: var(--color-text-primary);
  font-size: 0.82rem;
  font-weight: 620;
  text-decoration: none;
}

a.job-name:hover,
a.job-name:focus-visible {
  color: var(--color-accent-hover);
  text-decoration: none;
}

.job-name svg {
  width: .68rem;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.2;
  opacity: 0;
  transition: opacity var(--transition-fast), transform var(--transition-fast);
}

a.job-name:hover svg,
a.job-name:focus-visible svg {
  opacity: 1;
  transform: translate(1px, -1px);
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.job-path {
  margin-top: 0.2rem;
  font-size: 0.7rem;
  color: var(--color-text-subtle);
  font-family: var(--font-mono);
  word-break: break-word;
}

.job-cell-status {
  min-width: 105px;
}

.job-cell-type {
  min-width: 95px;
}

.job-cell-namespace {
  min-width: 95px;
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
  min-width: 180px;
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
  height: 0.3rem;
  border-radius: 2px;
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

.allocation-progress.is-pending .allocation-progress__fill {
  background: var(--jobs-bar-warning);
}

.allocation-progress.is-failed .allocation-progress__fill {
  background: var(--jobs-bar-danger);
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
  width: 5.25rem;
  min-height: 1.35rem;
  justify-content: center;
  padding-inline: .4rem;
  border-width: 1px;
  font-size: .7rem;
  font-weight: 600;
  white-space: nowrap;
}

@media (max-width: 720px) {
  .allocation-progress-row {
    justify-content: flex-start;
  }
}
</style>
