<template>
  <div class="settings-layout">
    <CredentialForm ref="formRef" :saving="savingCredential" @submit="handleSubmit" />
    <CredentialList
      :credentials="credentials"
      :deleting-credential-id="deletingCredentialId"
      @delete="handleDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
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
} = useCompassStore();

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
  gap: 1.75rem;
}
</style>
