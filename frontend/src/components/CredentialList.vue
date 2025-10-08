<template>
  <section class="panel glass">
    <header class="panel-header">
      <div>
        <h2>Stored credentials</h2>
        <p v-if="credentials.length">{{ credentials.length }} saved secrets.</p>
        <p v-else>No credentials stored yet.</p>
      </div>
    </header>

    <div v-if="credentials.length" class="credential-list">
      <article v-for="cred in credentials" :key="cred.id" class="credential-row">
        <div>
          <strong>{{ cred.name }}</strong>
          <p>{{ formatType(cred.type) }}</p>
        </div>
        <div class="row-actions">
          <span class="pill">{{ cred.type }}</span>
          <button
            class="ghost danger small"
            type="button"
            @click="$emit('delete', cred)"
            :disabled="deletingCredentialId === cred.id"
          >
            <span v-if="deletingCredentialId === cred.id" class="loader"></span>
            <span v-else>Delete</span>
          </button>
        </div>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { Credential } from '../composables/useCompassStore';

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
</script>
