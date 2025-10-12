<template>
  <li
    class="repo-card"
    :class="{ active: isSyncing }"
  >
    <header class="repo-card-header">
      <div class="repo-title">
        <h3 class="repo-name">{{ repo.name }}</h3>
        <span class="repo-branch-chip" v-if="repo.branch">{{ repo.branch }}</span>
      </div>
      <div class="repo-controls">
        <button
          class="ghost small control-button"
          type="button"
          @click="emit('reconcile', repo)"
          :disabled="isSyncing"
        >
          <span v-if="isSyncing" class="loader"></span>
          <span v-else>Sync now</span>
        </button>
        <button
          class="ghost danger small control-button"
          type="button"
          @click="emit('delete', repo)"
          :disabled="isDeleting"
        >
          <span v-if="isDeleting" class="loader"></span>
          <span v-else>Delete</span>
        </button>
      </div>
    </header>

    <section class="repo-info">
      <div class="repo-info-grid">
        <div class="info-cell">
          <span class="info-label">Source</span>
          <a
            :href="repo.repo_url"
            class="info-value link"
            :title="repo.repo_url"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ repo.repo_url }}
          </a>
        </div>
        <div class="info-cell">
          <span class="info-label">Job path</span>
          <span class="info-value" :title="repo.job_path">
            {{ repo.job_path }}
          </span>
        </div>
        <div class="info-cell">
          <span class="info-label">Credential</span>
          <span class="info-value">{{ credentialLabel }}</span>
        </div>
        <div class="info-cell">
          <span class="info-label">Last Checked</span>
          <span class="info-value">
            <template v-if="repo.last_polled_at">
              <time
                class="commit-polled"
                :datetime="lastPolledDatetime"
                :title="lastPolledAbsolute"
              >
                {{ lastPolledRelative }}
              </time>
            </template>
            <span v-else class="commit-polled pending">Awaiting first poll</span>
          </span>
        </div>
      </div>
      <RepoPollingInfo class="repo-info-commit" :repo="repo" />
    </section>
    <RepoJobList :jobs="repo.jobs" />
  </li>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import RepoJobList from './RepoJobList.vue';
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

const isSyncing = computed(() => props.syncingRepoId === props.repo.id);
const isDeleting = computed(() => props.deletingRepoId === props.repo.id);
const credentialLabel = computed(() => (props.repo.credential_id ? 'Managed secret' : 'Public'));
const lastPolledRelative = computed(() => formatRelativeTime(props.repo.last_polled_at));
const lastPolledAbsolute = computed(() => formatTimestamp(props.repo.last_polled_at));
const lastPolledDatetime = computed(() => props.repo.last_polled_at ?? undefined);
</script>

<style scoped>
.repo-card {
  list-style: none;
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
  padding: clamp(1.4rem, 3vw, 1.85rem);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  box-shadow: var(--shadow-soft);
  transition: border-color var(--transition-base), box-shadow var(--transition-base), transform var(--transition-fast);
}

.repo-card:hover {
  border-color: var(--color-border-strong);
  box-shadow: var(--shadow-card);
  transform: translateY(-2px);
}

.repo-card.active {
  border-color: var(--color-accent);
  box-shadow: 0 0 0 2px rgba(29, 111, 228, 0.18), var(--shadow-card);
}

.repo-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.75rem;
  padding-bottom: 0.6rem;
  border-bottom: 1px solid var(--color-border-soft);
}

.repo-title {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.repo-card-header .repo-name {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--color-text-primary);
}

.repo-branch-chip {
  display: inline-flex;
  align-items: center;
  padding: 0.25rem 0.6rem;
  border-radius: var(--radius-pill);
  background: var(--color-accent-muted);
  border: 1px solid rgba(29, 111, 228, 0.35);
  color: var(--color-accent);
  font-size: 0.74rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.repo-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.control-button {
  padding: 0.35rem 0.75rem;
  font-size: 0.8rem;
}

.repo-info {
  margin: 0;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
  background: var(--color-surface-muted);
}

.repo-info-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  grid-template-rows: repeat(2, 1fr);
  min-width: 0;
  border-bottom: 1px solid var(--color-border);
}

.repo-info-grid > * {
  padding: 0.8rem 0.9rem;
  border-right: 1px solid var(--color-border);
  border-bottom: 1px solid var(--color-border);
}

.repo-info-grid > *:nth-child(2n) {
  border-right: none;
}

.repo-info-grid > *:nth-child(n+3) {
  border-bottom: none;
}

.info-cell {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  min-width: 0;
}

.info-cell:last-child {
  border-right: none;
}

.info-label {
  font-size: 0.7rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  color: var(--color-text-subtle);
}

.info-value {
  font-size: 0.85rem;
  color: var(--color-text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.info-value.link {
  color: var(--color-accent);
  text-decoration: none;
  transition: color var(--transition-fast);
}

.info-value.link:hover,
.info-value.link:focus-visible {
  color: var(--color-accent-hover);
}

.commit-polled {
  color: var(--color-text-secondary);
  font-weight: 500;
}

.commit-polled.pending {
  color: var(--status-warning-text);
}

.repo-info-commit {
  border-top: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.7);
}

@media (max-width: 768px) {
  .repo-card-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .repo-controls {
    width: 100%;
    justify-content: flex-start;
  }

  .repo-info-grid {
    grid-template-columns: 1fr;
  }

  .repo-info-grid > * {
    border-right: none;
  }
}
</style>
