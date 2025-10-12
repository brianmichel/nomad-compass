<template>
  <div class="app-shell">
    <Topbar :status="status" @add-repo="openRepoModal" />
    <main class="content-frame">
      <div class="content-container">
        <router-view />
      </div>
    </main>
    <ToastMessage v-if="error" :message="error" @dismiss="clearError" />
    <transition name="fade">
      <div v-if="showRepoModal" class="modal-backdrop" @click.self="closeRepoModal">
        <div class="modal-card">
          <button
            class="ghost close-button"
            type="button"
            @click="closeRepoModal"
            aria-label="Close add repo form"
          >
            Close
          </button>
          <RepoForm
            ref="repoFormRef"
            :credentials="credentials"
            :saving="savingRepo"
            @submit="handleRepoSubmit"
          />
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import Topbar from '@/components/Topbar.vue';
import ToastMessage from '@/components/ToastMessage.vue';
import RepoForm from '@/components/RepoForm.vue';
import { useCompassStore } from '@/composables/useCompassStore';
import { useBodyScrollLock } from '@/composables/useBodyScrollLock';
import type { RepoPayload } from '@/types';

const {
  refreshAll,
  error,
  clearError,
  status,
  credentials,
  savingRepo,
  createRepo,
} = useCompassStore();

const showRepoModal = ref(false);
const repoFormRef = ref<InstanceType<typeof RepoForm> | null>(null);

useBodyScrollLock(showRepoModal);

onMounted(() => {
  refreshAll();
});

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
  showRepoModal.value = false;
  repoFormRef.value?.reset();
}
</script>

<style scoped>
.app-shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--color-bg);
}

.content-frame {
  flex: 1;
  padding: clamp(1.5rem, 4vw, 3rem);
}

.content-container {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: clamp(1.5rem, 3vw, 2.5rem);
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.35);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: clamp(2rem, 4vw, 4rem);
  overflow-y: auto;
  z-index: 20;
}

.modal-card {
  position: relative;
  width: min(720px, 100%);
  display: flex;
  justify-content: center;
  padding-top: 2.75rem;
}

.close-button {
  font-size: 0.82rem;
  padding: 0.4rem 0.95rem;
  position: absolute;
  top: 1rem;
  right: 1rem;
  z-index: 1;
  border-radius: var(--radius-pill);
  border: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.92);
  color: var(--color-text-secondary);
  transition: border-color var(--transition-fast), background-color var(--transition-fast), color var(--transition-fast);
}

.close-button:hover,
.close-button:focus-visible {
  border-color: var(--color-border-strong);
  background: var(--color-surface-muted);
  color: var(--color-text-primary);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
