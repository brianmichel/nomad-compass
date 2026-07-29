<template>
  <tr class="repo-row" :class="{ syncing: isSyncing }">
    <td class="cell-name">
      <RouterLink class="repo-name" :to="{ name: 'repo-detail', params: { id: repo.id } }">
        {{ repo.name }}
      </RouterLink>
    </td>
    <td class="cell-branch">
      <span class="badge badge-ghost badge-sm branch-chip">
        <IconGitBranch aria-hidden="true" />
        {{ repo.branch || '—' }}
      </span>
    </td>
    <td class="cell-timestamp">
      <time v-if="repo.last_polled_at" class="polled-time" :datetime="lastPolledDatetime" :title="lastPolledAbsolute">
        {{ lastPolledRelative }}
      </time>
      <span v-else class="polled-pending">Awaiting poll</span>
    </td>
    <td class="cell-status">
      <RepoJobsSummary v-if="repo.jobs?.length" :jobs="repo.jobs" />
      <span v-else class="job-status-empty">No jobs</span>
    </td>
    <td class="cell-actions">
      <div class="join row-actions" aria-label="Repository actions">
        <button class="btn btn-ghost btn-xs join-item" type="button" @click="emit('reconcile', repo)" :disabled="isSyncing">
          <span v-if="isSyncing" class="loading loading-spinner loading-xs"></span>
          <IconRefresh v-else aria-hidden="true" />
          <span class="sr-only">Sync {{ repo.name }}</span>
        </button>
        <button class="btn btn-ghost btn-xs join-item delete-button" type="button" @click="emit('delete', repo)" :disabled="isDeleting">
          <span v-if="isDeleting" class="loading loading-spinner loading-xs"></span>
          <IconTrash v-else aria-hidden="true" />
          <span class="sr-only">Delete {{ repo.name }}</span>
        </button>
      </div>
    </td>
  </tr>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { IconGitBranch, IconRefresh, IconTrash } from '@tabler/icons-vue';
import RepoJobsSummary from './RepoJobsSummary.vue';
import type { Repo } from '@/types';
import { formatRelativeTime, formatTimestamp } from '@/utils/date';

const props = defineProps<{ repo: Repo; syncingRepoId: number | null; deletingRepoId: number | null }>();
const emit = defineEmits<{ (e: 'reconcile', repo: Repo): void; (e: 'delete', repo: Repo): void }>();
const isSyncing = computed(() => props.syncingRepoId === props.repo.id);
const isDeleting = computed(() => props.deletingRepoId === props.repo.id);
const lastPolledRelative = computed(() => formatRelativeTime(props.repo.last_polled_at));
const lastPolledAbsolute = computed(() => formatTimestamp(props.repo.last_polled_at));
const lastPolledDatetime = computed(() => props.repo.last_polled_at ?? undefined);
</script>

<style scoped>
.repo-row td { vertical-align: middle; padding: .6rem .75rem; }
.repo-row.syncing td { background: var(--color-accent-muted); }
.cell-name { max-width: 260px; }
.repo-name { display: inline-block; max-width: 100%; overflow: hidden; color: var(--color-text-primary); font-size: .86rem; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.repo-name:hover { color: var(--color-accent); }
.cell-branch { min-width: 110px; }
.branch-chip { gap: .25rem; max-width: 9rem; font-family: var(--font-mono); font-size: .66rem; text-transform: none; }
.branch-chip svg { width: .72rem; }
.cell-timestamp { color: var(--color-text-secondary); font-size: .8rem; white-space: nowrap; }
.polled-pending, .job-status-empty { color: var(--color-text-tertiary); font-size: .78rem; }
.cell-status { min-width: 190px; }
:deep(.jobs-summary) { min-width: 0; }
.cell-actions { width: 1%; text-align: right; white-space: nowrap; }
.row-actions { border: 1px solid var(--color-border); border-radius: var(--radius-field); }
.row-actions .btn { border: 0; }
.row-actions svg { width: .82rem; }
.delete-button:hover { background: var(--color-danger-bg); color: var(--color-danger); }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
</style>
