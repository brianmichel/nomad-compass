<template>
  <div class="dashboard-layout">
    <RepoForm
      ref="repoFormRef"
      :credentials="credentials"
      :saving="savingRepo"
      @submit="handleRepoSubmit"
    />
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
import { ref } from 'vue';
import RepoForm from '../components/RepoForm.vue';
import RepoList from '../components/RepoList.vue';
import type { Repo, RepoPayload } from '../composables/useCompassStore';
import { useCompassStore } from '../composables/useCompassStore';

const repoFormRef = ref<InstanceType<typeof RepoForm> | null>(null);

const {
  credentials,
  repos,
  savingRepo,
  syncingRepoId,
  deletingRepoId,
  createRepo,
  triggerReconcile,
  deleteRepo,
} = useCompassStore();

async function handleRepoSubmit(payload: RepoPayload) {
  try {
    await createRepo(payload);
    repoFormRef.value?.reset();
  } catch (err) {
    // Error surfaced via store toast
  }
}

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
