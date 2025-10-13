<template>
  <section class="panel">
    <header class="panel-header">
      <div>
        <h2>Credential vault</h2>
        <p>HTTPS tokens and SSH keys are sealed with your container key.</p>
      </div>
    </header>

    <form class="form-grid" @submit.prevent="handleSubmit">
      <label class="field">
        <span>Display name</span>
        <input v-model="form.name" placeholder="production-github" required />
      </label>

      <label class="field">
        <span>Type</span>
        <select v-model="form.type">
          <option value="https-token">HTTPS token</option>
          <option value="ssh-key">SSH key</option>
        </select>
      </label>

      <template v-if="form.type === 'https-token'">
        <label class="field">
          <span>Username <small>(optional)</small></span>
          <input v-model="form.username" placeholder="git" />
        </label>
        <label class="field full">
          <span>Token</span>
          <input v-model="form.token" placeholder="ghp_xxx" required />
        </label>
      </template>

      <template v-else>
        <label class="field full">
          <span>Private key</span>
          <textarea
            v-model="form.private_key"
            placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
            required
          ></textarea>
        </label>
        <label class="field">
          <span>Passphrase <small>(optional)</small></span>
          <input v-model="form.passphrase" type="password" />
        </label>
      </template>

      <button class="primary" type="submit" :disabled="saving">
        <span v-if="saving" class="loader"></span>
        <span v-else>Add credential</span>
      </button>
    </form>
  </section>
</template>

<script setup lang="ts">
import { reactive } from 'vue';

const props = defineProps<{ saving: boolean }>();
const emit = defineEmits<{
  (e: 'submit', payload: Record<string, string>): void;
}>();

const form = reactive({
  name: '',
  type: 'https-token',
  token: '',
  username: '',
  private_key: '',
  passphrase: '',
});

function handleSubmit() {
  emit('submit', { ...form });
}

function reset() {
  form.name = '';
  form.token = '';
  form.username = '';
  form.private_key = '';
  form.passphrase = '';
  form.type = 'https-token';
}

defineExpose({ reset, form });
</script>
