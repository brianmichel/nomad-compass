<template>
  <div class="dashboard-layout">
    <RepoList
      :repos="repos"
      :syncing-repo-id="syncingRepoId"
      :deleting-repo-id="deletingRepoId"
      @reconcile="handleReconcile"
      @delete="handleDeleteRepo"
    />
  </div>
</template>

<script setup lang="ts">
import RepoList from '../components/RepoList.vue';
import type { Repo } from '../composables/useCompassStore';
import { useCompassStore } from '../composables/useCompassStore';

const {
  repos,
  syncingRepoId,
  deletingRepoId,
  triggerReconcile,
  deleteRepo,
} = useCompassStore();

async function handleReconcile(repo: Repo) {
  try {
    await triggerReconcile(repo.id);
  } catch (err) {
    // handled globally
  }
}

async function handleDeleteRepo(repo: Repo) {
  if (!window.confirm(`Delete repository "${repo.name}"?`)) {
    return;
  }
  const unschedule = window.confirm('Unschedule associated Nomad jobs?');
  try {
    await deleteRepo(repo.id, { unschedule });
  } catch (err) {
    // handled globally
  }
}
</script>

<style scoped>
.dashboard-layout {
  display: flex;
  flex-direction: column;
  gap: 1.75rem;
}
</style>
