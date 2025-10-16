<template>
  <form ref="formEl" class="credential-form" @submit.prevent="handleSubmit">
    <div class="form-grid">
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
    </div>

    <p class="helper-text">
      Credentials are encrypted with your container key before leaving the browser.
    </p>

    <button ref="submitButton" type="submit" class="visually-hidden">Submit</button>
  </form>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';

const emit = defineEmits<{
  (e: 'submit', payload: Record<string, string>): void;
}>();

const formEl = ref<HTMLFormElement | null>(null);
const submitButton = ref<HTMLButtonElement | null>(null);

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

function requestSubmit() {
  if (formEl.value) {
    formEl.value.requestSubmit();
    return;
  }
  submitButton.value?.click();
}

defineExpose({ reset, form, requestSubmit });
</script>

<style scoped>
.credential-form {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.credential-form :deep(textarea) {
  min-height: 140px;
}

.helper-text {
  margin: 0;
  font-size: 0.8rem;
  color: var(--color-text-tertiary);
  line-height: 1.5;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
</style>
