<template>
  <form ref="formEl" class="credential-form" @submit.prevent="handleSubmit">
    <div class="form-grid">
      <label class="fieldset">
        <span class="fieldset-legend">Display name</span>
        <input v-model="form.name" class="input input-bordered w-full" placeholder="production-github" required />
      </label>

      <label class="fieldset">
        <span class="fieldset-legend">Type</span>
        <select v-model="form.type" class="select select-bordered w-full">
          <option value="https-token">HTTPS token</option>
          <option value="ssh-key">SSH key</option>
        </select>
      </label>

      <template v-if="form.type === 'https-token'">
        <label class="fieldset">
          <span class="fieldset-legend">Username <small>(optional)</small></span>
          <input v-model="form.username" class="input input-bordered w-full" placeholder="git" autocomplete="username" />
        </label>
        <label class="fieldset full">
          <span class="fieldset-legend">Token</span>
          <input v-model="form.token" class="input input-bordered w-full" type="password" placeholder="ghp_xxx" autocomplete="new-password" required />
        </label>
      </template>

      <template v-else>
        <label class="fieldset full">
          <span class="fieldset-legend">Private key</span>
          <textarea
            v-model="form.private_key"
            class="textarea textarea-bordered w-full"
            placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
            autocomplete="new-password"
            required
          ></textarea>
        </label>
        <label class="fieldset">
          <span class="fieldset-legend">Passphrase <small>(optional)</small></span>
          <input v-model="form.passphrase" class="input input-bordered w-full" type="password" autocomplete="new-password" />
        </label>
      </template>
    </div>

    <p class="helper-text">
      Credentials are encrypted with your container key before leaving the browser.
    </p>

  </form>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';

const emit = defineEmits<{
  (e: 'submit', payload: Record<string, string>): void;
}>();

const formEl = ref<HTMLFormElement | null>(null);

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
  formEl.value?.requestSubmit();
}

defineExpose({ reset, form, requestSubmit });
</script>

<style scoped>
.credential-form {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.credential-form :deep(textarea) { min-height: 140px; }
.fieldset { min-width: 0; padding: 0; }
.fieldset-legend { padding-bottom: .35rem; color: var(--color-text-secondary); font-size: .75rem; font-weight: 600; }
.fieldset-legend small { color: var(--color-text-tertiary); font-weight: 400; }

.helper-text {
  margin: 0;
  font-size: 0.8rem;
  color: var(--color-text-tertiary);
  line-height: 1.5;
}

</style>
