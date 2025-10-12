<template>
  <form class="repo-form" @submit.prevent="handleSubmit">
    <header class="repo-form__header">
      <div>
        <h2>Add repository</h2>
        <p>Repositories will have their files monitored for Nomad job specifications.</p>
      </div>
    </header>
    <div class="repo-form__grid">
      <label class="field">
        <span>Display name</span>
        <input v-model="form.name" placeholder="payments" required />
      </label>
      <label class="field span-2">
        <span>Repository URL</span>
        <input v-model="form.repo_url" placeholder="git@github.com:acme/payments.git" required />
      </label>
      <label class="field">
        <span>Branch</span>
        <input v-model="form.branch" placeholder="main" required />
      </label>
      <label class="field">
        <span>Credential</span>
        <select v-model.number="form.credential_id">
          <option :value="0">None (public)</option>
          <option v-for="cred in credentials" :value="cred.id" :key="cred.id">
            {{ cred.name }}
          </option>
        </select>
      </label>
      <label class="field span-2">
        <span>Job path</span>
        <input v-model="form.job_path" placeholder=".nomad" required />
        <small>Relative to the repository root. All <code>*.nomad</code> and <code>*.nomad.hcl</code> files inside will be tracked.</small>
      </label>
    </div>
    <div class="repo-form__actions">
      <button class="primary" type="submit" :disabled="saving">
        <span v-if="saving" class="loader"></span>
        <span v-else>Add repository</span>
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { reactive } from 'vue';
import type { Credential, RepoPayload } from '@/types';

const props = defineProps<{ credentials: Credential[]; saving: boolean }>();
const emit = defineEmits<{
  (e: 'submit', payload: RepoPayload): void;
}>();

const form = reactive({
  name: '',
  repo_url: '',
  branch: 'main',
  job_path: '.nomad',
  credential_id: 0,
});

function handleSubmit() {
  emit('submit', {
    name: form.name,
    repo_url: form.repo_url,
    branch: form.branch,
    job_path: form.job_path,
    credential_id: form.credential_id || undefined,
  });
}

function reset() {
  form.name = '';
  form.repo_url = '';
  form.branch = 'main';
  form.job_path = '.nomad';
  form.credential_id = 0;
}

defineExpose({ reset, form });
</script>

<style scoped>
.repo-form {
  width: 100%;
  padding: 1.85rem 2rem;
  background: rgba(15, 23, 42, 0.88);
  border-radius: 24px;
  border: 1px solid rgba(148, 163, 184, 0.28);
  box-shadow: 0 35px 90px -45px rgba(2, 6, 23, 0.85);
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.repo-form__header h2 {
  margin: 0;
  font-size: 1.35rem;
  font-weight: 600;
  color: #e2e8f0;
}

.repo-form__header p {
  margin: 0.35rem 0 0 0;
  color: rgba(148, 163, 184, 0.85);
  font-size: 0.9rem;
}

.repo-form__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem 1.25rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  font-size: 0.95rem;
  font-weight: 500;
  color: #e2e8f0;
}

.field span {
  font-size: 0.82rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: rgba(148, 163, 184, 0.8);
}

.field small {
  font-size: 0.8rem;
  font-weight: 400;
  color: rgba(148, 163, 184, 0.75);
}

.field input,
.field select {
  width: 100%;
  padding: 0.7rem 0.85rem;
  border-radius: 12px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  background: rgba(15, 23, 42, 0.65);
  color: #e2e8f0;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.field input:focus,
.field select:focus {
  outline: none;
  border-color: rgba(99, 102, 241, 0.6);
  box-shadow: 0 0 0 3px rgba(129, 140, 248, 0.25);
}

.span-2 {
  grid-column: span 2;
}

@media (max-width: 640px) {
  .repo-form {
    padding: 1.5rem;
  }

  .span-2 {
    grid-column: 1 / -1;
  }
}

.repo-form__actions {
  display: flex;
  justify-content: flex-end;
}
</style>
