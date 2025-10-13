<template>
  <div
    ref="root"
    class="repo-jobs"
    :class="{
      compact: isCompact,
      collapsed: !expanded && hasOverflow,
    }"
  >
    <template v-if="jobs.length">
      <header class="jobs-header">
        <div class="jobs-heading">
          <span class="jobs-title">Jobs</span>
          <span class="jobs-subtitle">Tracked Nomad allocations</span>
        </div>
        <span class="jobs-count">{{ jobs.length }} total</span>
      </header>
      <div class="jobs-stack">
        <RepoJob
          v-for="job in jobs"
          :key="job.path"
          :job="job"
          :compact="isCompact"
        />
      </div>
    </template>
    <p v-else class="job-empty">No Nomad jobs registered yet.</p>
  </div>
  <button
    v-if="hasOverflow"
    type="button"
    class="jobs-toggle"
    @click="toggleExpanded"
  >
    <span v-if="expanded">Show fewer jobs</span>
    <span v-else>Show all jobs ({{ jobs.length }})</span>
  </button>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import RepoJob from './RepoJob.vue';
import type { RepoJob as RepoJobType } from '@/types';

const props = defineProps<{
  jobs: RepoJobType[];
}>();

const root = ref<HTMLElement | null>(null);
const isCompact = ref(false);
const expanded = ref(false);
const hasOverflow = ref(false);
let resizeObserver: ResizeObserver | null = null;
let mutationObserver: MutationObserver | null = null;
const COMPACT_WIDTH = 620;
const COLLAPSED_HEIGHT = 360;

function updateOverflow() {
  if (!root.value) return;
  if (expanded.value) {
    hasOverflow.value = false;
    return;
  }
  hasOverflow.value = root.value.scrollHeight - 1 > COLLAPSED_HEIGHT;
}

onMounted(() => {
  const el = root.value;
  if (!el) return;

  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver((entries) => {
      if (!entries.length) return;
      const entry = entries[0];
      isCompact.value = entry.contentRect.width <= COMPACT_WIDTH;
      updateOverflow();
    });
    resizeObserver.observe(el);
  } else {
    // Fallback: check on window resize
    const onResize = () => {
      if (!root.value) return;
      isCompact.value = root.value.clientWidth <= COMPACT_WIDTH;
      updateOverflow();
    };
    window.addEventListener('resize', onResize);
    onBeforeUnmount(() => window.removeEventListener('resize', onResize));
    onResize();
  }

  if (typeof MutationObserver !== 'undefined') {
    mutationObserver = new MutationObserver(() => updateOverflow());
    mutationObserver.observe(el, { childList: true, subtree: true });
  }

  updateOverflow();
});

onBeforeUnmount(() => {
  if (resizeObserver) {
    resizeObserver.disconnect();
    resizeObserver = null;
  }
  if (mutationObserver) {
    mutationObserver.disconnect();
    mutationObserver = null;
  }
});

function toggleExpanded() {
  expanded.value = !expanded.value;
  if (!expanded.value && root.value) {
    root.value.scrollTop = 0;
  }
  nextTick(() => updateOverflow());
}

watch(
  () => props.jobs.length,
  () => nextTick(() => updateOverflow()),
);
</script>

<style scoped>

.repo-jobs {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  padding: 0.75rem 0.85rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}

.jobs-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 0.55rem;
  border-bottom: 1px solid var(--color-border-soft);
  gap: 1rem;
}

.jobs-heading {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.jobs-title {
  font-size: 0.85rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
}

.jobs-subtitle {
  font-size: 0.82rem;
  color: var(--color-text-subtle);
}

.jobs-count {
  font-size: 0.8rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--color-text-subtle);
}

.jobs-stack {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.repo-jobs.collapsed {
  max-height: 340px;
  overflow: hidden;
}

.repo-jobs.collapsed::after {
  content: "";
  position: absolute;
  inset: auto 0 0;
  height: 64px;
  pointer-events: none;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0) 0%, rgba(242, 245, 250, 0.95) 100%);
}


.job-empty {
  margin: 0.25rem 0;
  font-size: 0.88rem;
  color: var(--color-text-subtle);
  text-align: center;
}

.jobs-toggle {
  margin-top: 0.35rem;
  align-self: flex-start;
  background: transparent;
  border: none;
  color: var(--color-accent);
  font-size: 0.8rem;
  letter-spacing: 0.03em;
}

.jobs-toggle:hover,
.jobs-toggle:focus-visible {
  color: var(--color-accent-hover);
  text-decoration: underline;
}
</style>
