<template>
  <div class="settings-layout">
    <section class="settings-card">
      <header class="settings-card__header">
        <h2>Auto refresh</h2>
        <p>Choose how often repository data refreshes across the app.</p>
      </header>
      <label class="settings-card__field">
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
    </section>
    <CredentialForm ref="formRef" :saving="savingCredential" @submit="handleSubmit" />
    <CredentialList
      :credentials="credentials"
      :deleting-credential-id="deletingCredentialId"
      @delete="handleDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import CredentialForm from '@/components/CredentialForm.vue';
import CredentialList from '@/components/CredentialList.vue';
import type { Credential } from '@/types';
import { useCompassStore } from '@/composables/useCompassStore';

const formRef = ref<InstanceType<typeof CredentialForm> | null>(null);

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

async function handleSubmit(payload: Record<string, string>) {
  try {
    await createCredential(payload);
    formRef.value?.reset();
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
.settings-layout {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 1.25rem;
  align-items: start;
}

.settings-card {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  padding: 1rem 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.settings-card__header h2 {
  margin: 0;
  font-size: 1rem;
  color: var(--color-text-primary);
}

.settings-card__header p {
  margin: 0.35rem 0 0;
  color: var(--color-text-secondary);
  font-size: 0.85rem;
  line-height: 1.4;
}

.settings-card__field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  font-size: 0.85rem;
  color: var(--color-text-secondary);
}

.settings-card__field select {
  padding: 0.45rem 0.6rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border);
  background: var(--color-surface-muted);
  color: var(--color-text-primary);
  font-size: 0.9rem;
}

.settings-card__field select:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}
</style>
