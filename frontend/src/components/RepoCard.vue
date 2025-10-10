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
          <span class="info-value">
            {{ repo.credential_id ? 'Managed secret' : 'Public' }}
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
import type { Repo } from '../composables/useCompassStore';

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
</script>

<style scoped>
.repo-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.75rem;
  padding-bottom: 0.6rem;
  margin-bottom: 0.4rem;
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
  color: #e2e8f0;
}

.repo-branch-chip {
  display: inline-flex;
  align-items: center;
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  background: rgba(236, 72, 153, 0.12);
  border: 1px solid rgba(236, 72, 153, 0.25);
  color: #fbcfe8;
  font-size: 0.75rem;
  letter-spacing: 0.04em;
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
  margin: 0 0 0.75rem;
  border: 1px solid rgba(71, 85, 105, 0.35);
  border-radius: 12px;
  overflow: hidden;
  background: rgba(15, 23, 42, 0.45);
}

.repo-info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  min-width: 0;
  border-bottom: 1px solid rgba(71, 85, 105, 0.35);
}

.info-cell {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  padding: 0.75rem 0.85rem;
  min-width: 0;
  border-right: 1px solid rgba(71, 85, 105, 0.35);
}

.info-cell:last-child {
  border-right: none;
}

.info-label {
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  font-family: monospace;
  color: rgba(148, 163, 184, 0.7);
}

.info-value {
  font-size: 0.85rem;
  color: rgba(226, 232, 240, 0.88);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.info-value.link {
  color: rgba(96, 165, 250, 0.9);
  text-decoration: none;
  transition: color 0.15s ease;
}

.info-value.link:hover,
.info-value.link:focus-visible {
  color: rgba(96, 165, 250, 1);
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

  .info-cell {
    border-right: none;
    border-bottom: 1px solid rgba(71, 85, 105, 0.35);
  }

  .info-cell:last-child {
    border-bottom: none;
  }
}
</style>
