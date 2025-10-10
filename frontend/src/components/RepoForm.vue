<template>
  <section class="panel glass">
    <header class="panel-header">
      <div>
        <h2>Onboard repository</h2>
        <p>Clone and watch Nomad job specs from Git.</p>
      </div>
    </header>
    <form class="form-grid" @submit.prevent="handleSubmit">
      <label class="field">
        <span>Display name</span>
        <input v-model="form.name" placeholder="payments" required />
      </label>
      <label class="field full">
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
      <label class="field">
        <span>Job path</span>
        <input v-model="form.job_path" placeholder=".nomad" required />
        <small>Relative to the repository root. All <code>*.nomad</code> and <code>*.nomad.hcl</code> files inside will be tracked.</small>
      </label>
      <button class="primary" type="submit" :disabled="saving">
        <span v-if="saving" class="loader"></span>
        <span v-else>Onboard repository</span>
      </button>
    </form>
  </section>
</template>

<script setup lang="ts">
import { reactive } from 'vue';
import type { Credential } from '../composables/useCompassStore';

const props = defineProps<{ credentials: Credential[]; saving: boolean }>();
const emit = defineEmits<{
  (e: 'submit', payload: { name: string; repo_url: string; branch: string; job_path: string; credential_id?: number }): void;
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
