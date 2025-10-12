<template>
  <div class="job-row" :class="{ compact }">
    <div class="job-info">
      <div class="job-heading">
        <span class="job-name">{{ jobLabel(job) }}</span>
        <a
          v-if="(isService(job) || isSystem(job)) && job.job_url"
          class="job-allocations"
          :href="job.job_url"
          target="_blank"
          rel="noopener noreferrer"
        >
          <span
            v-for="allocation in visibleAllocations(job)"
            :key="allocation.id"
            class="allocation-square"
            :class="allocationStatusClass(allocation)"
            :title="allocationTooltipForAllocation(allocation)"
          ></span>
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

<script setup lang="ts">
import type { RepoJob, AllocationStatus } from '@/types';

defineProps<{
  job: RepoJob;
  compact?: boolean;
}>();

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

function isBatch(job: RepoJob) {
  return (job.job_type || '').toLowerCase() === 'batch';
}

function isService(job: RepoJob) {
  return (job.job_type || '').toLowerCase() === 'service';
}

function isSystem(job: RepoJob) {
  return (job.job_type || '').toLowerCase() === 'system';
}

function visibleAllocations(job: RepoJob) {
  if (!Array.isArray(job.allocations)) {
    return [];
  }
  return job.allocations.filter(
    (allocation) => (allocation.status || '').toLowerCase() === 'running',
  );
}

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
  padding: 0.75rem 0.85rem;
  border-radius: var(--radius-md);
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid var(--color-border);
  gap: 1rem;
  transition: border-color var(--transition-base), box-shadow var(--transition-base), transform var(--transition-fast);
}

.job-row:hover {
  border-color: var(--color-border-strong);
  box-shadow: var(--shadow-soft);
  transform: translateY(-1px);
}

.job-info {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.job-heading {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.job-name {
  font-weight: 600;
  color: var(--color-text-primary);
}

.job-allocations {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.15rem 0.3rem;
  text-decoration: none;
}

.allocation-square {
  width: 0.85rem;
  height: 0.85rem;
  border-radius: 0.22rem;
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

.job-batch {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--status-warning-text);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  text-decoration: none;
  padding: 0.12rem 0.5rem;
  border-radius: var(--radius-pill);
  background: var(--status-warning-bg);
  border: 1px solid var(--status-warning-border);
}

.job-batch:hover {
  background: rgba(246, 186, 108, 0.25);
  color: var(--status-warning-text);
}

.job-path {
  font-size: 0.8rem;
  color: var(--color-text-subtle);
}

.job-status-badge {
  font-size: 0.78rem;
  padding: 0.28rem 0.7rem;
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
  padding: 0.6rem 0.65rem;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.6rem;
}

.job-row.compact .job-info {
  width: 100%;
  gap: 0.3rem;
}

.job-row.compact .job-heading {
  width: 100%;
  justify-content: space-between;
  align-items: flex-start;
}

.job-row.compact .job-name {
  font-size: 0.9rem;
}

.job-row.compact .job-path {
  font-size: 0.75rem;
}

.job-row.compact .job-status-badge {
  align-self: flex-start;
}
</style>
