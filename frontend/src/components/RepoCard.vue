<template>
  <tr
    class="repo-row"
    :class="{ syncing: isSyncing }"
    tabindex="0"
    role="button"
    @click="openDetails"
    @keydown.enter.prevent="openDetails"
    @keydown.space.prevent="openDetails"
  >
    <td class="cell-name">
      <span class="repo-name">{{ repo.name }}</span>
    </td>
    <td class="cell-branch">
      <span v-if="repo.branch" class="branch-chip">{{ repo.branch }}</span>
      <span v-else class="branch-chip branch-chip--muted">—</span>
    </td>
    <td class="cell-timestamp">
      <template v-if="repo.last_polled_at">
        <time
          class="polled-time"
          :datetime="lastPolledDatetime"
          :title="lastPolledAbsolute"
        >
          {{ lastPolledRelative }}
        </time>
      </template>
      <span v-else class="polled-pending">Awaiting poll</span>
    </td>
    <td class="cell-status">
      <RepoJobsSummary v-if="repo.jobs && repo.jobs.length" :jobs="repo.jobs" />
      <span v-else class="job-status-empty">No jobs</span>
    </td>
    <td class="cell-actions">
      <button
        class="ghost small"
        type="button"
        @click.stop="emit('reconcile', repo)"
        :disabled="isSyncing"
      >
        <span v-if="isSyncing" class="loader"></span>
        <span v-else>Sync</span>
      </button>
      <button
        class="ghost danger small"
        type="button"
        @click.stop="emit('delete', repo)"
        :disabled="isDeleting"
      >
        <span v-if="isDeleting" class="loader"></span>
        <span v-else>Delete</span>
      </button>
    </td>
  </tr>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';
import RepoJobsSummary from './RepoJobsSummary.vue';
import type { Repo } from '@/types';
import { formatRelativeTime, formatTimestamp } from '@/utils/date';

const props = defineProps<{
  repo: Repo;
  syncingRepoId: number | null;
  deletingRepoId: number | null;
}>();

const emit = defineEmits<{
  (e: 'reconcile', repo: Repo): void;
  (e: 'delete', repo: Repo): void;
}>();

const router = useRouter();

const isSyncing = computed(() => props.syncingRepoId === props.repo.id);
const isDeleting = computed(() => props.deletingRepoId === props.repo.id);
const lastPolledRelative = computed(() => formatRelativeTime(props.repo.last_polled_at));
const lastPolledAbsolute = computed(() => formatTimestamp(props.repo.last_polled_at));
const lastPolledDatetime = computed(() => props.repo.last_polled_at ?? undefined);

function openDetails() {
  router.push({ name: 'repo-detail', params: { id: props.repo.id } });
}
</script>

<style scoped>
.repo-row td {
  vertical-align: middle;
  padding: 0.75rem 0.9rem;
}

.repo-row {
  cursor: pointer;
}

.repo-row.syncing td {
  background: rgba(23, 106, 209, 0.06);
}

.cell-name {
  max-width: 260px;
}

.repo-name {
  display: inline-block;
  font-weight: 600;
  color: var(--color-text-primary);
  font-size: 0.96rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.cell-branch {
  min-width: 120px;
}

.branch-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.2rem 0.75rem;
  border-radius: var(--radius-pill);
  border: 1px solid var(--color-border);
  background: var(--color-surface-muted);
  font-size: 0.75rem;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
}

.branch-chip--muted {
  opacity: 0.6;
}

.cell-timestamp {
  font-size: 0.86rem;
  white-space: nowrap;
  color: var(--color-text-secondary);
}

.polled-time {
  color: inherit;
}

.polled-pending {
  color: var(--color-text-subtle);
  font-style: italic;
}

.cell-status {
  min-width: 220px;
}

:deep(.jobs-summary) {
  min-width: 0;
}

.job-status-empty {
  color: var(--color-text-subtle);
  font-size: 0.85rem;
}

.cell-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 0.45rem;
  flex-wrap: wrap;
}

@media (max-width: 960px) {
  .cell-status {
    min-width: 180px;
  }
}

@media (max-width: 640px) {
  .repo-row td {
    padding: 0.65rem 0.7rem;
  }

  .cell-actions {
    justify-content: flex-start;
  }
}
</style>
