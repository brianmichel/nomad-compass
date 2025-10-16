<template>
  <div class="credential-table-wrapper">
    <div v-if="credentials.length" class="table-scroll">
      <table>
        <thead>
          <tr>
            <th scope="col">Name</th>
            <th scope="col">Type</th>
            <th scope="col" class="actions-column">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="cred in credentials" :key="cred.id">
            <td class="name-column">
              <div class="name-cell">
                <strong>{{ cred.name }}</strong>
                <span v-if="describeCredential(cred)" class="description">
                  {{ describeCredential(cred) }}
                </span>
              </div>
            </td>
            <td class="type-column">
              <span class="pill">{{ formatType(cred.type) }}</span>
            </td>
            <td class="actions-column">
              <button
                class="ghost danger small"
                type="button"
                @click="$emit('delete', cred)"
                :disabled="deletingCredentialId === cred.id"
              >
                <span v-if="deletingCredentialId === cred.id" class="loader"></span>
                <span v-else>Delete</span>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="credential-empty">
      <h3>No credentials stored</h3>
      <p>Securely save HTTPS tokens or SSH keys to connect repository sources.</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Credential } from '@/types';

defineProps<{
  credentials: Credential[];
  deletingCredentialId: number | null;
}>();

defineEmits<{
  (e: 'delete', credential: Credential): void;
}>();

function formatType(value: string) {
  if (value === 'https-token') return 'HTTPS token';
  if (value === 'ssh-key') return 'SSH key';
  return value;
}

function describeCredential(credential: Credential) {
  if (credential.created_at) {
    const createdAt = new Date(credential.created_at);
    if (!Number.isNaN(createdAt.valueOf())) {
      return `Added ${createdAt.toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      })}`;
    }
  }
  if (credential.updated_at) {
    const updatedAt = new Date(credential.updated_at);
    if (!Number.isNaN(updatedAt.valueOf())) {
      return `Updated ${updatedAt.toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      })}`;
    }
  }
  return '';
}
</script>

<style scoped>
.credential-table-wrapper {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--color-border-soft);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
  box-shadow: var(--shadow-soft);
  overflow: hidden;
}

.table-scroll {
  overflow-x: auto;
}

table {
  width: 100%;
  min-width: 520px;
  border-collapse: collapse;
}

thead th {
  text-align: left;
  vertical-align: middle;
}

thead th.actions-column {
  text-align: right;
}

tbody td {
  vertical-align: middle;
}

.name-cell {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.name-cell strong {
  font-weight: 600;
  color: var(--color-text-primary);
}

.description {
  font-size: 0.82rem;
  color: var(--color-text-tertiary);
}

.type-column,
.actions-column {
  white-space: nowrap;
}

.pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.25rem 0.65rem;
  border-radius: var(--radius-pill);
  background: var(--color-surface-muted);
  border: 1px solid var(--color-border);
  color: var(--color-text-tertiary);
  font-size: 0.72rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.actions-column {
  width: 1%;
  white-space: nowrap;
  text-align: right;
}

tbody tr:hover td {
  background: var(--color-surface-muted);
}

.credential-empty {
  padding: 2.5rem 1.5rem;
  text-align: center;
  color: var(--color-text-tertiary);
}

.credential-empty h3 {
  margin: 0 0 0.35rem;
  font-size: 1rem;
  color: var(--color-text-primary);
}

.credential-empty p {
  margin: 0;
  font-size: 0.9rem;
}
</style>
