<template>
  <div class="settings-page">
    <header class="page-header">
      <h1 data-dialog-fallback tabindex="-1">Settings</h1>
      <p>Manage refresh behavior and repository credentials.</p>
    </header>

    <div class="settings-grid">
      <section class="settings-section" aria-labelledby="app-settings-heading">
        <header class="section-header">
          <div>
            <h2 id="app-settings-heading">App settings</h2>
            <p>Configure how Compass behaves across the workspace.</p>
          </div>
        </header>
        <div class="card card-border settings-card">
          <div class="card-body">
            <label class="fieldset">
              <span class="fieldset-legend">Refresh interval</span>
              <select v-model.number="selectedInterval" class="select select-bordered w-full">
                <option v-for="option in refreshIntervalOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
              <span class="fieldset-label">Applies to repository lists, details, and job summaries.</span>
            </label>
          </div>
        </div>
      </section>

      <section class="settings-section" aria-labelledby="credentials-heading">
        <header class="section-header">
          <div>
            <h2 id="credentials-heading">Credential vault</h2>
            <p>HTTPS tokens and SSH keys are sealed with your container key.</p>
          </div>
          <button class="btn btn-primary btn-sm" type="button" @click="openCredentialModal">
            <IconPlus aria-hidden="true" /> New credential
          </button>
        </header>
        <CredentialList :credentials="credentials" :deleting-credential-id="deletingCredentialId" @delete="openDeleteDialog" />
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
        <button class="btn btn-ghost" type="button" @click="handleModalClose" :disabled="savingCredential">Cancel</button>
        <button class="btn btn-primary" type="button" @click="submitCredentialForm" :disabled="savingCredential">
          <span v-if="savingCredential" class="loading loading-spinner loading-xs"></span>
          {{ savingCredential ? 'Saving credential' : 'Save credential' }}
        </button>
      </template>
    </ModalDialog>

    <ModalDialog
      :open="Boolean(credentialToDelete)"
      title="Delete credential"
      :description="`Remove ${credentialToDelete?.name || 'this credential'} from Compass.`"
      @close="closeDeleteDialog"
    >
      <div class="alert alert-warning alert-soft" role="alert">
        <IconAlertTriangle aria-hidden="true" />
        <span>Repositories using this credential may no longer be accessible.</span>
      </div>
      <label class="delete-option">
        <input v-model="deleteDependentRepos" type="checkbox" class="checkbox checkbox-sm" />
        <span><strong>Delete repositories using this credential</strong><small>Remove dependent repositories from Compass too.</small></span>
      </label>
      <label v-if="deleteDependentRepos" class="delete-option">
        <input v-model="unscheduleJobs" type="checkbox" class="checkbox checkbox-sm" />
        <span><strong>Unschedule associated Nomad jobs</strong><small>Stop jobs tracked by the dependent repositories.</small></span>
      </label>
      <template #footer>
        <button class="btn btn-ghost" type="button" data-autofocus @click="closeDeleteDialog" :disabled="isDeletingSelected">Cancel</button>
        <button class="btn btn-error" type="button" @click="confirmDelete" :disabled="isDeletingSelected">
          <span v-if="isDeletingSelected" class="loading loading-spinner loading-xs"></span>
          {{ isDeletingSelected ? 'Deleting' : 'Delete credential' }}
        </button>
      </template>
    </ModalDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { IconAlertTriangle, IconPlus } from '@tabler/icons-vue';
import CredentialForm from '@/components/CredentialForm.vue';
import CredentialList from '@/components/CredentialList.vue';
import ModalDialog from '@/components/ModalDialog.vue';
import type { Credential } from '@/types';
import { useCompassStore } from '@/composables/useCompassStore';

const formRef = ref<InstanceType<typeof CredentialForm> | null>(null);
const showCredentialModal = ref(false);
const credentialToDelete = ref<Credential | null>(null);
const deleteDependentRepos = ref(false);
const unscheduleJobs = ref(false);
const { credentials, savingCredential, deletingCredentialId, createCredential, deleteCredential, refreshIntervalMs, setRefreshInterval } = useCompassStore();
const isDeletingSelected = computed(() => deletingCredentialId.value === credentialToDelete.value?.id);
const refreshIntervalOptions = [
  { label: '30 seconds', value: 30000 },
  { label: '60 seconds', value: 60000 },
  { label: '90 seconds', value: 90000 },
];
const selectedInterval = computed({
  get: () => refreshIntervalMs.value,
  set: (value: number) => { setRefreshInterval(value); },
});

function openCredentialModal(): void { showCredentialModal.value = true; }
function handleModalClose(): void {
  if (savingCredential.value) return;
  formRef.value?.reset();
  showCredentialModal.value = false;
}
function submitCredentialForm(): void { formRef.value?.requestSubmit(); }
async function handleSubmit(payload: Record<string, string>): Promise<void> {
  try { await createCredential(payload); handleModalClose(); } catch { /* surfaced globally */ }
}
function openDeleteDialog(credential: Credential): void { credentialToDelete.value = credential; }
function closeDeleteDialog(): void {
  if (isDeletingSelected.value) return;
  credentialToDelete.value = null;
  deleteDependentRepos.value = false;
  unscheduleJobs.value = false;
}
async function confirmDelete(): Promise<void> {
  if (!credentialToDelete.value) return;
  try {
    await deleteCredential(credentialToDelete.value.id, { deleteRepos: deleteDependentRepos.value, unschedule: deleteDependentRepos.value && unscheduleJobs.value });
    closeDeleteDialog();
  } catch { /* surfaced globally */ }
}
</script>

<style scoped>
.settings-page { display: flex; flex-direction: column; gap: 1.75rem; width: 100%; }
.page-header h1 { margin: 0; color: var(--color-text-primary); font-size: clamp(1.35rem, 3vw, 1.65rem); font-weight: 680; letter-spacing: -.02em; }
.page-header p { margin: .25rem 0 0; color: var(--color-text-tertiary); font-size: .8125rem; }
.settings-grid { display: grid; grid-template-columns: minmax(17rem, .7fr) minmax(28rem, 1.3fr); align-items: start; gap: 1.5rem; }
.settings-section { display: flex; flex-direction: column; gap: .75rem; min-width: 0; }
.section-header { display: flex; align-items: flex-end; justify-content: space-between; gap: 1rem; min-height: 3.1rem; }
.section-header h2 { margin: 0; color: var(--color-text-primary); font-size: .98rem; font-weight: 650; }
.section-header p { margin: .2rem 0 0; color: var(--color-text-tertiary); font-size: .75rem; }
.section-header .btn { flex: 0 0 auto; }
.section-header svg { width: .9rem; }
.settings-card { border-color: var(--color-border); border-radius: var(--radius-box); box-shadow: none; }
.settings-card .card-body { padding: 1rem; }
.fieldset { padding: 0; }
.fieldset-legend { padding-bottom: .35rem; color: var(--color-text-secondary); font-size: .75rem; font-weight: 600; }
.fieldset-label { padding-top: .35rem; color: var(--color-text-tertiary); font-size: .7rem; }
.alert svg { width: 1.1rem; }
.alert span { font-size: .8rem; }
.delete-option { display: flex; align-items: flex-start; gap: .7rem; padding: .2rem; cursor: pointer; }
.delete-option span { display: flex; flex-direction: column; gap: .12rem; }
.delete-option strong { color: var(--color-text-primary); font-size: .82rem; }
.delete-option small { color: var(--color-text-tertiary); font-size: .75rem; }
@media (max-width: 900px) { .settings-grid { grid-template-columns: 1fr; } }
@media (max-width: 540px) { .section-header { align-items: stretch; flex-direction: column; } .section-header .btn { width: 100%; } }
</style>
