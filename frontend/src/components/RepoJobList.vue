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
      <RepoJob
        v-for="job in jobs"
        :key="job.path"
        :job="job"
        :compact="isCompact"
      />
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
import type { RepoJob as RepoJobType } from '../composables/useCompassStore';

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
  margin-top: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  position: relative;
}

.repo-jobs.compact {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.75rem;
}

.repo-jobs.compact .job-empty {
  grid-column: 1 / -1;
}

.repo-jobs.collapsed {
  max-height: 360px;
  overflow: hidden;
}

.repo-jobs.compact.collapsed {
  max-height: 340px;
}

.repo-jobs.collapsed::after {
  content: "";
  position: absolute;
  inset: auto 0 0;
  height: 72px;
  pointer-events: none;
  background: linear-gradient(180deg, rgba(15, 23, 42, 0) 0%, rgba(15, 23, 42, 0.9) 100%);
}

.job-empty {
  margin-top: 1rem;
  font-size: 0.9rem;
  color: rgba(148, 163, 184, 0.85);
}

.jobs-toggle {
  margin-top: 0.75rem;
  align-self: flex-start;
  background: transparent;
  border: 1px solid rgba(96, 165, 250, 0.35);
  color: rgba(191, 219, 254, 0.9);
  padding: 0.35rem 0.85rem;
  border-radius: 999px;
  font-size: 0.82rem;
  letter-spacing: 0.02em;
  cursor: pointer;
  transition: border-color 0.15s ease, color 0.15s ease, background 0.15s ease;
}

.jobs-toggle:hover,
.jobs-toggle:focus-visible {
  border-color: rgba(96, 165, 250, 0.6);
  color: rgba(224, 242, 254, 0.95);
  background: rgba(30, 64, 175, 0.25);
}
</style>
