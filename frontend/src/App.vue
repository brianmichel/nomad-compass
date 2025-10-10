<template>
  <div class="app-shell">
    <Topbar :status="status" @add-repo="openRepoModal" />
    <div class="content-frame">
      <router-view />
    </div>
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
import { onMounted, ref, watch, onBeforeUnmount } from 'vue';
import Topbar from './components/Topbar.vue';
import ToastMessage from './components/ToastMessage.vue';
import RepoForm from './components/RepoForm.vue';
import { useCompassStore } from './composables/useCompassStore';
import type { RepoPayload } from './composables/useCompassStore';

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

onMounted(() => {
  refreshAll();
});

watch(showRepoModal, (value) => {
  if (typeof document === 'undefined') return;
  document.body.style.overflow = value ? 'hidden' : '';
});

onBeforeUnmount(() => {
  if (typeof document !== 'undefined') {
    document.body.style.overflow = '';
  }
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
  padding: 2.5rem clamp(1rem, 4vw, 4rem);
  display: flex;
  flex-direction: column;
  gap: 2rem;
  max-width: 1200px;
  margin: 0 auto;
}

.content-frame {
  flex: 1;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.65);
  backdrop-filter: blur(6px);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: clamp(2rem, 4vw, 4rem);
  overflow-y: auto;
  z-index: 20;
  animation: fadeIn 0.2s ease;
}

.modal-card {
  position: relative;
  width: min(720px, 100%);
  display: flex;
  justify-content: center;
}

.close-button {
  font-size: 0.85rem;
  padding: 0.35rem 0.95rem;
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  z-index: 1;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.35);
  background: rgba(15, 23, 42, 0.75);
  color: rgba(226, 232, 240, 0.85);
  transition: border-color 0.15s ease, background 0.15s ease, color 0.15s ease;
}

.close-button:hover,
.close-button:focus-visible {
  border-color: rgba(148, 163, 184, 0.55);
  background: rgba(30, 41, 59, 0.85);
  color: rgba(226, 232, 240, 0.95);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
