<template>
  <tr
    class="repo-row"
    :class="{ syncing: isSyncing }"
    tabindex="0"
    role="button"
    @click="openDetails"
    @keydown.enter.prevent="openDetails"
    @keydown.space.prevent="openDetails"
    ref="rowRef"
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
        @click="emit('reconcile', repo)"
        :disabled="isSyncing"
      >
        <span v-if="isSyncing" class="loader"></span>
        <span v-else>Sync now</span>
      </button>
      <button
        class="ghost danger small"
        type="button"
        @click="emit('delete', repo)"
        :disabled="isDeleting"
      >
        <span v-if="isDeleting" class="loader"></span>
        <span v-else>Delete</span>
      </button>
    </td>
  </tr>
  <Teleport to="body">
    <transition name="repo-modal">
      <div
        v-if="showDetails"
        class="repo-modal__backdrop"
        @click="closeDetails"
      >
        <div
          class="repo-modal"
          role="dialog"
          :aria-modal="true"
          :aria-labelledby="modalTitleId"
          tabindex="-1"
          ref="modalRef"
          @click.stop
        >
          <header class="repo-modal__header">
            <div class="repo-modal__heading">
              <h3 :id="modalTitleId">{{ repo.name }}</h3>
              <p v-if="repo.repo_url" class="repo-modal__subtitle">
                {{ repo.repo_url }}
              </p>
            </div>
            <button class="ghost small" type="button" @click="closeDetails">
              Close
            </button>
          </header>
          <div class="repo-modal__content">
            <section class="repo-modal__meta">
              <dl>
                <div class="repo-modal__meta-item">
                  <dt>Repository</dt>
                  <dd>
                    <a
                      v-if="repo.repo_url"
                      :href="repo.repo_url"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      {{ repo.repo_url }}
                    </a>
                    <span v-else>—</span>
                  </dd>
                </div>
                <div class="repo-modal__meta-item">
                  <dt>Credential</dt>
                  <dd>{{ credentialLabel }}</dd>
                </div>
                <div class="repo-modal__meta-item">
                  <dt>Namespace</dt>
                  <dd>{{ repo.nomad_namespace || '—' }}</dd>
                </div>
                <div class="repo-modal__meta-item">
                  <dt>Job path</dt>
                  <dd>{{ repo.job_path || '—' }}</dd>
                </div>
              </dl>
            </section>
            <RepoPollingInfo :repo="repo" />
            <RepoJobList :jobs="repo.jobs || []" />
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch, nextTick, onBeforeUnmount } from 'vue';
import RepoJobList from './RepoJobList.vue';
import RepoJobsSummary from './RepoJobsSummary.vue';
import RepoPollingInfo from './RepoPollingInfo.vue';
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

const showDetails = ref(false);
const modalRef = ref<HTMLElement | null>(null);
const rowRef = ref<HTMLTableRowElement | null>(null);

const isSyncing = computed(() => props.syncingRepoId === props.repo.id);
const isDeleting = computed(() => props.deletingRepoId === props.repo.id);
const credentialLabel = computed(() => (props.repo.credential_id ? 'Managed secret' : 'Public'));
const modalTitleId = computed(() => `repo-${props.repo.id}-details`);
const lastPolledRelative = computed(() => formatRelativeTime(props.repo.last_polled_at));
const lastPolledAbsolute = computed(() => formatTimestamp(props.repo.last_polled_at));
const lastPolledDatetime = computed(() => props.repo.last_polled_at ?? undefined);

let previousBodyOverflow: string | null = null;

function openDetails() {
  showDetails.value = true;
}

function closeDetails() {
  showDetails.value = false;
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault();
    closeDetails();
  }
}

watch(showDetails, (value) => {
  if (value) {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    document.addEventListener('keydown', handleKeydown);
    nextTick(() => {
      modalRef.value?.focus();
    });
  } else {
    document.body.style.overflow = previousBodyOverflow ?? '';
    previousBodyOverflow = null;
    document.removeEventListener('keydown', handleKeydown);
    nextTick(() => {
      rowRef.value?.focus();
    });
  }
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown);
  if (showDetails.value) {
    document.body.style.overflow = previousBodyOverflow ?? '';
  }
});
</script>

<style scoped>
.repo-row td {
  vertical-align: middle;
  padding: 0.75rem 0.9rem;
  border-bottom: 1px solid var(--color-border-soft);
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

.repo-modal__backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2rem;
  z-index: 1000;
}

.repo-modal {
  width: min(780px, 100%);
  max-height: min(90vh, 820px);
  background: var(--color-surface);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-elevated);
  outline: none;
  display: flex;
  flex-direction: column;
  gap: 1.4rem;
  padding: 1.6rem 1.8rem;
  overflow: hidden;
}

.repo-modal__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.repo-modal__heading h3 {
  margin: 0;
  font-size: 1.12rem;
  font-weight: 600;
  color: var(--color-text-primary);
}

.repo-modal__subtitle {
  margin: 0.4rem 0 0;
  font-size: 0.88rem;
  color: var(--color-text-secondary);
  word-break: break-all;
}

.repo-modal__content {
  display: flex;
  flex-direction: column;
  gap: 1.4rem;
  flex: 1 1 auto;
  position: relative;
  overflow-y: auto;
  padding-right: 0.6rem;
  padding-bottom: 1.2rem;
  margin-right: -0.6rem;
  scrollbar-gutter: stable both-edges;
}

.repo-modal__content::-webkit-scrollbar {
  width: 8px;
}

.repo-modal__content::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(71, 85, 105, 0.35);
}

.repo-modal__content::before,
.repo-modal__content::after {
  content: '';
  position: sticky;
  left: 0;
  right: 0;
  height: 12px;
  pointer-events: none;
  z-index: 1;
}

.repo-modal__content::before {
  top: 0;
  margin-top: -12px;
  background: linear-gradient(180deg, var(--color-surface) 0%, rgba(255, 255, 255, 0.4), rgba(255, 255, 255, 0));
}

.repo-modal__content::after {
  bottom: 0;
  margin-bottom: -18px;
  background: linear-gradient(0deg, var(--color-surface) 0%, rgba(255, 255, 255, 0.55), rgba(255, 255, 255, 0));
}

.repo-modal__meta {
  padding: 1rem 1.2rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface-muted);
}

.repo-modal__meta dl {
  margin: 0;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 1rem 1.5rem;
}

.repo-modal__meta-item dt {
  margin: 0 0 0.35rem;
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
}

.repo-modal__meta-item dd {
  margin: 0;
  font-size: 0.9rem;
  color: var(--color-text-secondary);
  word-break: break-word;
}

.repo-modal__meta-item a {
  color: var(--color-accent);
}

.repo-modal__meta-item a:hover,
.repo-modal__meta-item a:focus-visible {
  color: var(--color-accent-hover);
}

.repo-modal-enter-active,
.repo-modal-leave-active {
  transition: opacity var(--transition-base);
}

.repo-modal-enter-from,
.repo-modal-leave-to {
  opacity: 0;
}

.repo-modal-enter-active .repo-modal,
.repo-modal-leave-active .repo-modal {
  transition: transform var(--transition-base);
}

.repo-modal-enter-from .repo-modal,
.repo-modal-leave-to .repo-modal {
  transform: translateY(12px) scale(0.97);
}

@media (max-width: 960px) {
  .cell-status {
    min-width: 180px;
  }
}

@media (max-width: 720px) {
  .repo-modal {
    padding: 1.3rem 1.1rem;
    gap: 1.1rem;
  }

  .repo-modal__backdrop {
    padding: 1.25rem;
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
