<template>
  <div class="dashboard-layout">
    <RepoList
      :repos="repos"
      :syncing-repo-id="syncingRepoId"
      :deleting-repo-id="deletingRepoId"
      @reconcile="handleReconcile"
      @delete="openDeleteDialog"
      @add-repo="emit('add-repo')"
    />
  </div>

  <ModalDialog
    :open="Boolean(repoToDelete)"
    title="Delete repository"
    :description="`Remove ${repoToDelete?.name || 'this repository'} from Compass.`"
    @close="closeDeleteDialog"
  >
    <div class="alert alert-warning alert-soft" role="alert">
      <IconAlertTriangle aria-hidden="true" />
      <span>This stops repository monitoring. The Git repository itself will not be changed.</span>
    </div>
    <label class="delete-option">
      <input v-model="unscheduleJobs" type="checkbox" class="checkbox checkbox-sm" />
      <span><strong>Unschedule associated Nomad jobs</strong><small>Stop all jobs currently tracked from this repository.</small></span>
    </label>
    <template #footer>
      <button class="btn btn-ghost" type="button" data-autofocus @click="closeDeleteDialog" :disabled="isDeletingSelected">Cancel</button>
      <button class="btn btn-error" type="button" @click="confirmDelete" :disabled="isDeletingSelected">
        <span v-if="isDeletingSelected" class="loading loading-spinner loading-xs"></span>
        {{ isDeletingSelected ? 'Deleting' : 'Delete repository' }}
      </button>
    </template>
  </ModalDialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { IconAlertTriangle } from '@tabler/icons-vue';
import ModalDialog from '@/components/ModalDialog.vue';
import RepoList from '@/components/RepoList.vue';
import type { Repo } from '@/types';
import { useCompassStore } from '@/composables/useCompassStore';

const emit = defineEmits<{ (e: 'add-repo'): void }>();
const { repos, syncingRepoId, deletingRepoId, triggerReconcile, deleteRepo } = useCompassStore();
const repoToDelete = ref<Repo | null>(null);
const unscheduleJobs = ref(false);
const isDeletingSelected = computed(() => deletingRepoId.value === repoToDelete.value?.id);

async function handleReconcile(repo: Repo): Promise<void> {
  try { await triggerReconcile(repo.id); } catch { /* surfaced globally */ }
}

function openDeleteDialog(repo: Repo): void {
  repoToDelete.value = repo;
}

function closeDeleteDialog(): void {
  if (isDeletingSelected.value) return;
  repoToDelete.value = null;
  unscheduleJobs.value = false;
}

async function confirmDelete(): Promise<void> {
  if (!repoToDelete.value) return;
  try {
    await deleteRepo(repoToDelete.value.id, { unschedule: unscheduleJobs.value });
    closeDeleteDialog();
  } catch { /* surfaced globally */ }
}
</script>

<style scoped>
.dashboard-layout { display: flex; flex-direction: column; gap: 1.75rem; }
.alert svg { width: 1.1rem; }
.alert span { font-size: .8rem; }
.delete-option { display: flex; align-items: flex-start; gap: .7rem; padding: .2rem; cursor: pointer; }
.delete-option span { display: flex; flex-direction: column; gap: .12rem; }
.delete-option strong { color: var(--color-text-primary); font-size: .82rem; }
.delete-option small { color: var(--color-text-tertiary); font-size: .75rem; }
</style>
