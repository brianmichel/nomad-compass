<template>
  <form ref="formEl" :class="['repo-form', { 'repo-form--embedded': props.embedded }]" @submit.prevent="handleSubmit">
    <header v-if="!props.hideHeader" class="repo-form__header">
      <div>
        <h2>Add repository</h2>
        <p>Repositories will have their files monitored for Nomad job specifications.</p>
      </div>
    </header>
    <div class="repo-form__grid">
      <label class="fieldset">
        <span class="fieldset-legend">Display name</span>
        <input v-model="form.name" class="input input-bordered w-full" placeholder="payments" required />
      </label>
      <label class="fieldset span-2">
        <span class="fieldset-legend">Repository URL</span>
        <input v-model="form.repo_url" class="input input-bordered w-full" placeholder="git@github.com:acme/payments.git" required />
      </label>
      <label class="fieldset">
        <span class="fieldset-legend">Branch</span>
        <input v-model="form.branch" class="input input-bordered w-full" placeholder="main" required />
      </label>
      <label class="fieldset">
        <span class="fieldset-legend">Credential</span>
        <select v-model.number="form.credential_id" class="select select-bordered w-full">
          <option :value="0">None (public)</option>
          <option v-for="cred in credentials" :value="cred.id" :key="cred.id">
            {{ cred.name }}
          </option>
        </select>
      </label>
      <label class="fieldset span-2">
        <span class="fieldset-legend">Job path</span>
        <input v-model="form.job_path" class="input input-bordered w-full" placeholder=".nomad" required />
        <span class="fieldset-label">Relative to the repository root. All <code>*.nomad</code> and <code>*.nomad.hcl</code> files inside will be tracked.</span>
      </label>
    </div>
    <div v-if="!props.embedded" class="repo-form__actions">
      <button class="btn btn-primary" type="submit" :disabled="saving">
        <span v-if="saving" class="loading loading-spinner loading-xs"></span>
        {{ saving ? 'Adding repository' : 'Add repository' }}
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import type { Credential, RepoPayload } from '@/types';

const props = withDefaults(
  defineProps<{
    credentials: Credential[];
    saving: boolean;
    embedded?: boolean;
    hideHeader?: boolean;
  }>(),
  {
    embedded: false,
    hideHeader: false,
  }
);
const formEl = ref<HTMLFormElement | null>(null);
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

function requestSubmit(): void {
  formEl.value?.requestSubmit();
}

function reset() {
  form.name = '';
  form.repo_url = '';
  form.branch = 'main';
  form.job_path = '.nomad';
  form.credential_id = 0;
}

defineExpose({ reset, form, requestSubmit });
</script>

<style scoped>
.repo-form {
  width: 100%;
  padding: clamp(1.6rem, 3vw, 2rem);
  background: var(--color-surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-border);
  box-shadow: none;
  display: flex;
  flex-direction: column;
  gap: 1.4rem;
}

.repo-form--embedded {
  padding: 0;
  background: transparent;
  border: none;
  border-radius: 0;
}

.repo-form__header h2 {
  margin: 0;
  font-size: 1.18rem;
  font-weight: 600;
  color: var(--color-text-primary);
}

.repo-form__header p {
  margin: 0.4rem 0 0;
  color: var(--color-text-secondary);
  font-size: 0.92rem;
  max-width: 32rem;
}

.repo-form__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.9rem 1.1rem;
}

.fieldset { min-width: 0; padding: 0; }
.fieldset-legend { padding-bottom: .35rem; color: var(--color-text-secondary); font-size: .75rem; font-weight: 600; }
.fieldset-label { align-items: flex-start; padding-top: .35rem; color: var(--color-text-tertiary); font-size: .7rem; }
.fieldset-label code { font-family: var(--font-mono); }
.span-2 { grid-column: span 2; }

@media (max-width: 640px) {
  .repo-form {
    padding: 1.5rem;
  }

  .repo-form--embedded {
    padding: 0;
  }

  .span-2 {
    grid-column: 1 / -1;
  }
}

.repo-form__actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 0.5rem;
}
</style>
