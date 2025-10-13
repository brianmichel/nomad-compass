<template>
  <div v-if="total > 0" class="jobs-summary">
    <div
      class="bar-track"
      role="presentation"
      :aria-label="tooltipText"
      :data-tooltip="tooltipText"
    >
      <div class="bar-track__segments">
        <div
          v-for="segment in segments"
          :key="segment.type"
          class="bar-segment"
          :class="segment.type"
          :style="{ flexGrow: segment.count }"
          :aria-label="`${segment.count} ${statusLabels[segment.type]}`"
        ></div>
      </div>
    </div>
    <div class="jobs-summary__count">{{ jobsSummaryCount }}</div>
  </div>
  <div v-else class="jobs-summary jobs-summary-empty">
    <span>No jobs tracked</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { RepoJob } from '@/types';
import { getJobStatusClass } from '@/utils/jobStatus';

const props = defineProps<{ jobs: RepoJob[] }>();

const statusOrder = ['danger', 'warning', 'pending', 'healthy', 'unknown'] as const;

const statusLabels: Record<(typeof statusOrder)[number], string> = {
  danger: 'Failing',
  warning: 'Degraded',
  pending: 'Pending',
  healthy: 'Healthy',
  unknown: 'Unknown',
};

const counts = computed(() =>
  props.jobs.reduce<Record<(typeof statusOrder)[number], number>>((acc, job) => {
    const status = getJobStatusClass(job);
    const key = (statusOrder.includes(status as any) ? status : 'unknown') as (typeof statusOrder)[number];
    acc[key] += 1;
    return acc;
  }, {
    danger: 0,
    warning: 0,
    pending: 0,
    healthy: 0,
    unknown: 0,
  }),
);

const jobsSummaryCount = computed(() => `${counts.value.healthy}/${props.jobs.length}`);

const total = computed(() => props.jobs.length);

const segments = computed(() =>
  statusOrder
    .map((type) => ({ type, count: counts.value[type] }))
    .filter((segment) => segment.count > 0),
);

const tooltipText = computed(() => {
  if (total.value === 0) {
    return 'No jobs tracked';
  }

  const entries = statusOrder
    .map((type) => ({ type, count: counts.value[type] }))
    .filter((entry) => entry.count > 0)
    .map((entry) => `${entry.count} ${statusLabels[entry.type].toLowerCase()}`);

  return entries.length ? entries.join(' · ') : 'No jobs tracked';
});
</script>

<style scoped>
.jobs-summary {
  display: flex;
  align-items: center;
  width: 100%;
}

.jobs-summary-empty {
  font-size: 0.82rem;
  color: var(--color-text-subtle);
  text-align: center;
}

.jobs-summary__count {
  font-size: 0.82rem;
  color: var(--color-text-secondary);
  margin-left: 0.5rem;
  text-align: right;
}

.bar-track {
  width: 100%;
  height: 6px;
  border-radius: 999px;
  background: var(--color-surface-muted);
  position: relative;
  cursor: default;
}

.bar-track__segments {
  display: flex;
  gap: 1px;
  width: 100%;
  height: 100%;
  overflow: hidden;
  border-radius: inherit;
}

.bar-segment {
  height: 100%;
  min-width: 4px;
}

.bar-track::after {
  content: attr(data-tooltip);
  position: absolute;
  bottom: calc(100% + 8px);
  left: 50%;
  transform: translate(-50%, 6px);
  padding: 0.35rem 0.55rem;
  border-radius: var(--radius-sm);
  background: rgba(15, 23, 42, 0.9);
  color: #fff;
  font-size: 0.72rem;
  letter-spacing: 0.04em;
  white-space: nowrap;
  box-shadow: 0 8px 18px -12px rgba(15, 23, 42, 0.6);
  opacity: 0;
  pointer-events: none;
  transition: opacity var(--transition-fast), transform var(--transition-fast);
  z-index: 1;
}

.bar-track::before {
  content: '';
  position: absolute;
  bottom: calc(100% + 4px);
  left: 50%;
  transform: translateX(-50%);
  border-width: 5px 5px 0 5px;
  border-style: solid;
  border-color: rgba(15, 23, 42, 0.9) transparent transparent transparent;
  opacity: 0;
  transition: opacity var(--transition-fast), transform var(--transition-fast);
  z-index: 1;
}

.bar-track:hover::after,
.bar-track:hover::before {
  opacity: 1;
}

.bar-track:hover::after {
  transform: translate(-50%, 0);
}

.bar-segment.healthy {
  background: var(--jobs-bar-healthy);
}

.bar-segment.pending {
  background: var(--jobs-bar-pending);
}

.bar-segment.warning {
  background: var(--jobs-bar-warning);
}

.bar-segment.danger {
  background: var(--jobs-bar-danger);
}

.bar-segment.unknown {
  background: var(--jobs-bar-unknown);
}
</style>
