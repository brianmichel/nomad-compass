<template>
  <div class="credential-table-wrapper">
    <div v-if="credentials.length" class="table-scroll">
      <table class="table table-sm">
        <thead>
          <tr><th scope="col">Name</th><th scope="col">Type</th><th scope="col" class="actions-column">Actions</th></tr>
        </thead>
        <tbody>
          <tr v-for="cred in credentials" :key="cred.id">
            <td class="name-column">
              <div class="name-cell">
                <strong>{{ cred.name }}</strong>
                <span v-if="describeCredential(cred)" class="description">{{ describeCredential(cred) }}</span>
              </div>
            </td>
            <td class="type-column"><span class="badge badge-ghost badge-sm">{{ formatType(cred.type) }}</span></td>
            <td class="actions-column">
              <button class="btn btn-ghost btn-xs delete-button" type="button" @click="$emit('delete', cred)" :disabled="deletingCredentialId === cred.id">
                <span v-if="deletingCredentialId === cred.id" class="loading loading-spinner loading-xs"></span>
                <IconTrash v-else aria-hidden="true" />
                <span class="sr-only">Delete {{ cred.name }}</span>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="credential-empty">
      <IconKey aria-hidden="true" />
      <h3>No credentials stored</h3>
      <p>Securely save HTTPS tokens or SSH keys to connect repository sources.</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconKey, IconTrash } from '@tabler/icons-vue';
import type { Credential } from '@/types';
defineProps<{ credentials: Credential[]; deletingCredentialId: number | null }>();
defineEmits<{ (e: 'delete', credential: Credential): void }>();

function formatType(value: string): string {
  if (value === 'https-token') return 'HTTPS token';
  if (value === 'ssh-key') return 'SSH key';
  return value;
}

function describeCredential(credential: Credential): string {
  const value = credential.created_at ?? credential.updated_at;
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return '';
  return `${credential.created_at ? 'Added' : 'Updated'} ${date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })}`;
}
</script>

<style scoped>
.credential-table-wrapper { display: flex; flex-direction: column; overflow: hidden; border: 1px solid var(--color-border); border-radius: var(--radius-box); background: var(--color-surface); }
.table-scroll { overflow-x: auto; }
table { width: 100%; min-width: 32rem; }
thead th, tbody td { vertical-align: middle; }
.name-cell { display: flex; flex-direction: column; gap: .15rem; }
.name-cell strong { color: var(--color-text-primary); font-size: .82rem; font-weight: 650; }
.description { color: var(--color-text-tertiary); font-size: .7rem; }
.type-column, .actions-column { white-space: nowrap; }
.actions-column { width: 1%; text-align: right; }
.actions-column svg { width: .82rem; }
.delete-button:hover { background: var(--color-danger-bg); color: var(--color-danger); }
.credential-empty { display: flex; flex-direction: column; align-items: center; padding: 2.5rem 1.5rem; text-align: center; }
.credential-empty > svg { width: 1.3rem; margin-bottom: .6rem; color: var(--color-text-subtle); }
.credential-empty h3 { margin: 0; color: var(--color-text-primary); font-size: .9rem; }
.credential-empty p { margin: .3rem 0 0; color: var(--color-text-tertiary); font-size: .78rem; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
</style>
