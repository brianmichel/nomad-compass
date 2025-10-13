<template>
  <div class="job-row" :class="{ compact }">
    <div class="job-overview">
      <div class="job-title-group">
        <span class="status-dot" :class="statusClass"></span>
        <span class="job-name">{{ jobName }}</span>
        <span v-if="jobTypeLabel" class="job-type-pill">{{ jobTypeLabel }}</span>
        <span v-if="jobNamespace" class="job-namespace-pill">{{ jobNamespace }}</span>
      </div>
      <span
        class="job-status-badge"
        :class="statusClass"
        :title="statusTooltip"
      >
        {{ statusLabel }}
      </span>
    </div>

    <div class="job-meta">
      <div class="job-meta-left">
        <span class="job-path">{{ job.path }}</span>
        <div v-if="hasAllocations" class="allocation-summary">
          <span class="allocation-pill healthy" v-if="runningCount">{{ runningCount }} Running</span>
          <span class="allocation-pill pending" v-if="pendingCount">{{ pendingCount }} Pending</span>
          <span class="allocation-pill danger" v-if="failedCount">{{ failedCount }} Failed</span>
          <span class="allocation-pill muted" v-if="remainingCount">{{ remainingCount }} Other</span>
        </div>
      </div>
      <div class="job-meta-right">
        <a
          v-if="job.job_url"
          class="job-link"
          :href="job.job_url"
          target="_blank"
          rel="noopener noreferrer"
        >
          Open in Nomad →
        </a>
        <div v-else-if="isBatchJob" class="job-inline-badge">Batch job</div>
      </div>
    </div>

    <div v-if="showAllocationLink || isBatchJob" class="job-allocations-bar">
      <a
        v-if="showAllocationLink"
        class="job-allocations"
        :href="job.job_url"
        target="_blank"
        rel="noopener noreferrer"
        title="View allocations in Nomad"
      >
        <span
          v-for="allocation in runningAllocations"
          :key="allocation.id"
          class="allocation-square"
          :class="allocationStatusClass(allocation)"
          :title="allocationTooltipForAllocation(allocation)"
        ></span>
      </a>
      <span
        v-else-if="isBatchJob"
        class="job-inline-note"
      >
        Batch jobs run on demand; allocations appear after submission.
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { RepoJob, AllocationStatus } from '@/types';
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
const jobTypeLabel = computed(() => {
  if (!jobType.value) return null;
  return capitalize(jobType.value);
});
const jobNamespace = computed(() => props.job.nomad_namespace || null);
const isServiceJob = computed(() => jobType.value === 'service');
const isSystemJob = computed(() => jobType.value === 'system');
const isBatchJob = computed(() => jobType.value === 'batch');

const showAllocationLink = computed(
  () => (isServiceJob.value || isSystemJob.value) && Boolean(props.job.job_url),
);

const runningAllocations = computed(() => {
  if (!Array.isArray(props.job.allocations)) {
    return [] as AllocationStatus[];
  }
  return props.job.allocations.filter(
    (allocation) => (allocation.status || '').toLowerCase() === 'running',
  );
});

const allAllocations = computed(() => (Array.isArray(props.job.allocations) ? props.job.allocations : []));

const runningCount = computed(() => runningAllocations.value.length);
const pendingCount = computed(() =>
  allAllocations.value.filter((allocation) =>
    ['pending', 'starting', 'queued', 'evaluating'].includes((allocation.status || '').toLowerCase()),
  ).length,
);
const failedCount = computed(() =>
  allAllocations.value.filter((allocation) =>
    ['failed', 'lost', 'dead', 'missing', 'cancelled'].includes((allocation.status || '').toLowerCase()),
  ).length,
);
const remainingCount = computed(() => {
  const other = allAllocations.value.length - runningCount.value - pendingCount.value - failedCount.value;
  return other > 0 ? other : 0;
});

const hasAllocations = computed(() => allAllocations.value.length > 0);

function allocationStatusClass(allocation: AllocationStatus) {
  const status = (allocation.status || '').toLowerCase();
  if (['complete'].includes(status)) {
    return 'completed';
  }
  if (['running', 'successful'].includes(status)) {
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

function allocationTooltipForAllocation(allocation: AllocationStatus) {
  const parts: string[] = [];
  if (allocation.name) {
    parts.push(allocation.name);
  }
  if (allocation.id) {
    parts.push(allocation.id);
  }
  if (allocation.status) {
    parts.push(`Status: ${capitalize(allocation.status)}`);
  }
  return parts.join(' • ') || 'View in Nomad';
}

function capitalize(value: string) {
  if (!value.length) return value;
  return value[0].toUpperCase() + value.slice(1);
}
</script>

<style scoped>
.job-row {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  padding: 0.65rem 0.8rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface-muted);
}

.job-overview {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.job-title-group {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
}

.status-dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  background: var(--status-unknown-border);
  flex-shrink: 0;
}

.status-dot.healthy {
  background: var(--color-success);
}

.status-dot.pending {
  background: var(--status-pending-border);
}

.status-dot.warning {
  background: var(--status-warning-border);
}

.status-dot.danger {
  background: var(--color-danger);
}

.job-name {
  font-weight: 600;
  color: var(--color-text-primary);
  font-size: 0.92rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.job-type-pill,
.job-namespace-pill {
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  border-radius: var(--radius-pill);
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-text-tertiary);
  padding: 0.12rem 0.4rem;
}

.job-namespace-pill {
  letter-spacing: 0.06em;
}


.job-meta {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.job-meta-left {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  min-width: 0;
}

.job-path {
  font-size: 0.78rem;
  color: var(--color-text-subtle);
  font-family: var(--font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
}

.allocation-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.allocation-pill {
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  padding: 0.14rem 0.4rem;
  border-radius: var(--radius-pill);
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

.job-meta-right {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.job-link {
  font-size: 0.78rem;
  color: var(--color-accent);
  text-decoration: none;
}

.job-link:hover,
.job-link:focus-visible {
  color: var(--color-accent-hover);
}

.job-inline-badge {
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  border-radius: var(--radius-pill);
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-text-tertiary);
  padding: 0.12rem 0.4rem;
}

.job-allocations-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.job-allocations {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  text-decoration: none;
}

.allocation-square {
  width: 0.65rem;
  height: 0.65rem;
  border-radius: 0.18rem;
  background: var(--color-border);
  border: 1px solid rgba(148, 163, 184, 0.45);
  transition: opacity var(--transition-fast);
}

.allocation-square:hover {
  opacity: 0.8;
}

.allocation-square.healthy {
  background: var(--color-success);
  border-color: var(--color-success);
}

.allocation-square.completed {
  background: var(--status-healthy-bg);
  border-color: var(--color-success-border);
}

.allocation-square.pending {
  background: var(--status-pending-border);
  border-color: var(--color-accent);
}

.allocation-square.danger {
  background: var(--color-danger);
  border-color: var(--color-danger);
}


.job-inline-note {
  font-size: 0.75rem;
  color: var(--color-text-subtle);
}


.job-status-badge {
  font-size: 0.74rem;
  padding: 0.18rem 0.55rem;
  border-radius: var(--radius-pill);
  text-transform: none;
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


.job-row.compact {
  padding: 0.55rem 0.7rem;
}

.job-row.compact .job-overview {
  flex-direction: column;
  align-items: flex-start;
  gap: 0.4rem;
}

.job-row.compact .job-meta {
  flex-direction: column;
  align-items: flex-start;
}

.job-row.compact .job-meta-right {
  align-self: flex-start;
}
</style>
