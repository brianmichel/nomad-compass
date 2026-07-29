<template>
  <div v-if="total > 0" class="jobs-summary">
    <div
      class="tooltip bar-track"
      tabindex="0"
      :aria-label="tooltipText"
      :data-tip="tooltipText"
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
type SummaryStatus = (typeof statusOrder)[number];

const statusLabels: Record<SummaryStatus, string> = {
  danger: 'Failing',
  warning: 'Degraded',
  pending: 'Pending',
  healthy: 'Healthy',
  unknown: 'Unknown',
};

const counts = computed(() =>
  props.jobs.reduce<Record<SummaryStatus, number>>((acc, job) => {
    const status = getJobStatusClass(job);
    const key = isSummaryStatus(status) ? status : 'unknown';
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

function isSummaryStatus(value: string): value is SummaryStatus {
  return statusOrder.some((status) => status === value);
}

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
  font-size: 0.75rem;
  color: var(--color-text-subtle);
  text-align: center;
}

.jobs-summary__count {
  font-size: 0.75rem;
  color: var(--color-text-secondary);
  margin-left: 0.4rem;
  text-align: right;
}

.bar-track {
  width: 100%;
  height: 5px;
  border-radius: 2px;
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
