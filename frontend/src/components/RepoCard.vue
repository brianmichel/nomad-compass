<template>
  <li
    class="repo-card"
    :class="{ active: isSyncing }"
  >
    <div class="repo-header">
      <div>
        <span class="repo-name">{{ repo.name }}</span>
        <span class="repo-branch">{{ repo.branch }}</span>
      </div>
      <div class="repo-actions">
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
      </div>
    </div>

    <div class="repo-meta">
      <span class="meta-pill" :title="repo.repo_url">
        <span class="label">Source</span>
        {{ repo.repo_url }}
      </span>
      <span class="meta-pill" :title="repo.job_path">
        <span class="label">Job path</span>
        {{ repo.job_path }}
      </span>
      <span class="meta-pill">
        <span class="label">Credential</span>
        {{ repo.credential_id ? 'Managed secret' : 'Public' }}
      </span>
    </div>

    <RepoPollingInfo :repo="repo" />
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
