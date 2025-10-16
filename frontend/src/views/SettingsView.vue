<template>
  <div class="settings-page">
    <div class="settings-grid">
      <section class="settings-section">
        <header class="section-header">
          <div>
            <h2>App settings</h2>
            <p>Configure how Compass behaves across the workspace.</p>
          </div>
        </header>
        <div class="card">
          <label class="field">
            <span>Refresh interval</span>
            <select v-model.number="selectedInterval">
              <option
                v-for="option in refreshIntervalOptions"
                :key="option.value"
                :value="option.value"
              >
                {{ option.label }}
              </option>
            </select>
          </label>
          <p class="card-helper">Applies to repository lists, details, and job summaries.</p>
        </div>
      </section>

      <section class="settings-section">
        <header class="section-header">
          <div>
            <h2>Credential vault</h2>
            <p>HTTPS tokens and SSH keys are sealed with your container key.</p>
          </div>
          <button class="primary small" type="button" @click="openCredentialModal">
            New credential
          </button>
        </header>
        <CredentialList
          :credentials="credentials"
          :deleting-credential-id="deletingCredentialId"
          @delete="handleDelete"
        />
      </section>
    </div>

    <ModalDialog
      :open="showCredentialModal"
      title="Add credential"
      description="Store HTTPS tokens or SSH keys securely to reuse across repositories."
      @close="handleModalClose"
    >
      <CredentialForm ref="formRef" @submit="handleSubmit" />
      <template #footer>
        <button class="ghost" type="button" @click="handleModalClose" :disabled="savingCredential">
          Cancel
        </button>
        <button
          class="primary"
          type="button"
          @click="submitCredentialForm"
          :disabled="savingCredential"
        >
          <span v-if="savingCredential" class="loader"></span>
          <span v-else>Save credential</span>
        </button>
      </template>
    </ModalDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import CredentialForm from '@/components/CredentialForm.vue';
import CredentialList from '@/components/CredentialList.vue';
import ModalDialog from '@/components/ModalDialog.vue';
import type { Credential } from '@/types';
import { useCompassStore } from '@/composables/useCompassStore';

const formRef = ref<InstanceType<typeof CredentialForm> | null>(null);
const showCredentialModal = ref(false);

const {
  credentials,
  savingCredential,
  deletingCredentialId,
  createCredential,
  deleteCredential,
  refreshIntervalMs,
  setRefreshInterval,
} = useCompassStore();

const refreshIntervalOptions = [
  { label: '30 seconds', value: 30000 },
  { label: '60 seconds', value: 60000 },
  { label: '90 seconds', value: 90000 },
];

const selectedInterval = computed({
  get: () => refreshIntervalMs.value,
  set: (value: number) => {
    setRefreshInterval(value);
  },
});

function openCredentialModal() {
  showCredentialModal.value = true;
}

function handleModalClose() {
  if (savingCredential.value) {
    return;
  }
  formRef.value?.reset();
  showCredentialModal.value = false;
}

function submitCredentialForm() {
  formRef.value?.requestSubmit();
}

async function handleSubmit(payload: Record<string, string>) {
  try {
    await createCredential(payload);
    handleModalClose();
  } catch (err) {
    // error bubbled globally
  }
}

async function handleDelete(credential: Credential) {
  if (!window.confirm(`Delete credential "${credential.name}"?`)) {
    return;
  }
  const deleteRepos = window.confirm('Delete repositories that use this credential?');
  let unschedule = false;
  if (deleteRepos) {
    unschedule = window.confirm('Unschedule associated Nomad jobs as well?');
  }
  try {
    await deleteCredential(credential.id, { deleteRepos, unschedule });
  } catch (err) {
    // handled globally
  }
}
</script>

<style scoped>
.settings-page {
  display: flex;
  flex-direction: column;
  gap: 2.5rem;
  width: 100%;
  max-width: 1120px;
  margin: 0 auto;
  padding: 0 clamp(1rem, 4vw, 2rem);
}

.settings-grid {
  display: grid;
  gap: 2rem;
  align-items: start;
}

@media (min-width: 980px) {
  .settings-grid {
    grid-template-columns: minmax(340px, 400px) minmax(520px, 1fr);
    gap: 2.5rem;
  }
}

@media (min-width: 1280px) {
  .settings-grid {
    grid-template-columns: minmax(360px, 420px) minmax(560px, 1fr);
  }
}

.settings-section {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1.25rem;
}

.section-header > div {
  flex: 1 1 auto;
}

.section-header button {
  flex-shrink: 0;
}

.section-header h2 {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 600;
  color: var(--color-text-primary);
}

.section-header p {
  margin: 0.35rem 0 0;
  color: var(--color-text-secondary);
  font-size: 0.92rem;
  line-height: 1.5;
}

@media (min-width: 980px) {
  .section-header {
    align-items: center;
  }
}

.card {
  padding: 1.5rem;
  border-radius: var(--radius-lg);
  background: var(--color-surface);
  border: 1px solid var(--color-border-soft);
  box-shadow: var(--shadow-soft);
  display: flex;
  flex-direction: column;
  gap: 1.15rem;
}

.card select {
  background: var(--color-surface-muted);
}

.card-helper {
  margin: 0;
  font-size: 0.82rem;
  color: var(--color-text-tertiary);
}

@media (max-width: 640px) {
  .section-header {
    flex-direction: column;
    align-items: stretch;
  }

  .section-header button {
    width: 100%;
    justify-content: center;
  }
}
</style>
