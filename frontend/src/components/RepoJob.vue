<template>
  <div class="job-row" :class="{ compact }">
    <div class="job-info">
      <div class="job-heading">
        <span class="job-name">{{ jobName }}</span>
        <a
          v-if="showAllocationLink"
          class="job-allocations"
          :href="job.job_url"
          target="_blank"
          rel="noopener noreferrer"
        >
          <span
            v-for="allocation in runningAllocations"
            :key="allocation.id"
            class="allocation-square"
            :class="allocationStatusClass(allocation)"
            :title="allocationTooltipForAllocation(allocation)"
          ></span>
        </a>
        <a
          v-else-if="isBatchJob && job.job_url"
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
      :class="statusClass"
      :title="statusTooltip"
    >
      {{ statusLabel }}
    </span>
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
  align-items: center;
  justify-content: space-between;
  gap: 1.2rem;
  padding: 0.55rem 0;
  border-bottom: 1px solid var(--color-border-soft);
}

.job-row:last-child {
  border-bottom: none;
}

.job-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 0;
}

.job-heading {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.job-name {
  font-weight: 600;
  color: var(--color-text-primary);
  font-size: 0.92rem;
}

.job-allocations {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  text-decoration: none;
}

.allocation-square {
  width: 0.7rem;
  height: 0.7rem;
  border-radius: 0.2rem;
  background: var(--color-border);
  border: 1px solid rgba(148, 163, 184, 0.5);
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

.job-batch {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--status-warning-text);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  text-decoration: none;
  padding: 0.1rem 0.4rem;
  border-radius: var(--radius-pill);
  background: var(--status-warning-bg);
  border: 1px solid var(--status-warning-border);
}

.job-batch:hover {
  background: rgba(246, 186, 108, 0.25);
  color: var(--status-warning-text);
}

.job-path {
  font-size: 0.78rem;
  color: var(--color-text-subtle);
  font-family: var(--font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
}

.job-status-badge {
  font-size: 0.76rem;
  padding: 0.22rem 0.55rem;
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
  padding: 0.5rem 0;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.5rem;
}

.job-row.compact .job-status-badge {
  align-self: flex-start;
}

.job-row.compact .job-info {
  width: 100%;
}
</style>
