<template>
  <div class="app-shell">
    <Topbar />
    <main class="content-frame">
      <div class="content-container">
        <router-view v-slot="{ Component }">
          <component :is="Component" @add-repo="openRepoModal" />
        </router-view>
      </div>
    </main>
    <ToastMessage v-if="error" :message="error" @dismiss="clearError" />
    <ModalDialog
      :open="showRepoModal"
      title="Add repository"
      description="Monitor a repository for Nomad job specifications."
      @close="closeRepoModal"
    >
      <RepoForm
        ref="repoFormRef"
        :credentials="credentials"
        :saving="savingRepo"
        embedded
        hide-header
        @submit="handleRepoSubmit"
      />
      <template #footer>
        <button class="btn btn-ghost" type="button" @click="closeRepoModal" :disabled="savingRepo">Cancel</button>
        <button class="btn btn-primary" type="button" @click="repoFormRef?.requestSubmit()" :disabled="savingRepo">
          <span v-if="savingRepo" class="loading loading-spinner loading-xs"></span>
          {{ savingRepo ? 'Adding repository' : 'Add repository' }}
        </button>
      </template>
    </ModalDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import ModalDialog from '@/components/ModalDialog.vue';
import Topbar from '@/components/Topbar.vue';
import ToastMessage from '@/components/ToastMessage.vue';
import RepoForm from '@/components/RepoForm.vue';
import { useCompassStore } from '@/composables/useCompassStore';
import type { RepoPayload } from '@/types';

const {
  refreshAll,
  error,
  clearError,
  credentials,
  loadRepos,
  savingRepo,
  createRepo,
  refreshIntervalMs,
} = useCompassStore();

const showRepoModal = ref(false);
const repoFormRef = ref<InstanceType<typeof RepoForm> | null>(null);

const repoPolling = ref(false);
let repoPollIntervalId: number | null = null;

const pollIntervalMs = computed(() => Math.max(1000, refreshIntervalMs.value));

onMounted(() => {
  void (async () => {
    await refreshAll();
    startRepoPolling();
  })();
});

watch(
  pollIntervalMs,
  () => {
    startRepoPolling();
  },
);

function openRepoModal() {
  showRepoModal.value = true;
}

async function handleRepoSubmit(payload: RepoPayload) {
  try {
    await createRepo(payload);
    repoFormRef.value?.reset();
    showRepoModal.value = false;
  } catch (err) {
    // errors shown via toast
  }
}

function closeRepoModal() {
  if (savingRepo.value) return;
  showRepoModal.value = false;
  repoFormRef.value?.reset();
}

async function refreshRepos() {
  if (repoPolling.value) {
    return;
  }
  repoPolling.value = true;
  try {
    await loadRepos();
  } catch (err) {
    // errors surfaced elsewhere
  } finally {
    repoPolling.value = false;
  }
}

function stopRepoPolling() {
  if (repoPollIntervalId !== null) {
    window.clearInterval(repoPollIntervalId);
    repoPollIntervalId = null;
  }
}

function startRepoPolling() {
  stopRepoPolling();
  repoPollIntervalId = window.setInterval(() => {
    void refreshRepos();
  }, pollIntervalMs.value);
  void refreshRepos();
}

onBeforeUnmount(() => {
  stopRepoPolling();
});
</script>

<style scoped>
.app-shell { min-height: 100vh; background: var(--color-bg); display: flex; flex-direction: column; }
.content-frame { flex: 1; padding: clamp(1.5rem, 3vw, 2.25rem) clamp(1rem, 3vw, 2rem) 2.25rem; }
.content-container { max-width: 1200px; width: 100%; margin: 0 auto; display: flex; flex-direction: column; gap: 1.25rem; }
@media (max-width: 640px) { .content-frame { padding: 1.5rem 1rem 2rem; } }
</style>
