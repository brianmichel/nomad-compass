<template>
  <main class="container">
    <header>
      <h1>Nomad Compass</h1>
      <p class="subtitle">Onboard repositories and keep Nomad jobs in sync.</p>
    </header>

    <section class="panel">
      <h2>Credentials</h2>
      <p class="hint">Create entries for HTTPS tokens or SSH keys supplied to the container.</p>
      <form class="form" @submit.prevent="submitCredential">
        <label>
          Name
          <input v-model="credentialForm.name" required placeholder="production-github" />
        </label>
        <label>
          Type
          <select v-model="credentialForm.type">
            <option value="https-token">HTTPS Token</option>
            <option value="ssh-key">SSH Key</option>
          </select>
        </label>
        <template v-if="credentialForm.type === 'https-token'">
          <label>
            Username (optional)
            <input v-model="credentialForm.username" placeholder="git" />
          </label>
          <label>
            Token
            <input v-model="credentialForm.token" required placeholder="ghp_xxx" />
          </label>
        </template>
        <template v-else>
          <label>
            Private Key
            <textarea v-model="credentialForm.private_key" required placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"></textarea>
          </label>
          <label>
            Passphrase (optional)
            <input v-model="credentialForm.passphrase" type="password" />
          </label>
        </template>
        <button type="submit">Save Credential</button>
      </form>

      <ul v-if="credentials.length" class="list">
        <li v-for="cred in credentials" :key="cred.id">
          <strong>{{ cred.name }}</strong>
          <span class="tag">{{ formatType(cred.type) }}</span>
        </li>
      </ul>
      <p v-else class="hint">No credentials yet.</p>
    </section>

    <section class="panel">
      <h2>Repositories</h2>
      <form class="form" @submit.prevent="submitRepo">
        <label>
          Display Name
          <input v-model="repoForm.name" required placeholder="payments" />
        </label>
        <label>
          Repo URL
          <input v-model="repoForm.repo_url" required placeholder="git@github.com:acme/payments.git" />
        </label>
        <label>
          Branch
          <input v-model="repoForm.branch" required />
        </label>
        <label>
          Credential
          <select v-model.number="repoForm.credential_id">
            <option value="0">None (public)</option>
            <option v-for="cred in credentials" :value="cred.id" :key="cred.id">
              {{ cred.name }}
            </option>
          </select>
        </label>
        <button type="submit">Onboard Repository</button>
      </form>

      <ul v-if="repos.length" class="repo-list">
        <li v-for="repo in repos" :key="repo.id">
          <div>
            <strong>{{ repo.name }}</strong>
            <span class="hint">{{ repo.repo_url }}@{{ repo.branch }}</span>
          </div>
          <div class="meta" v-if="repo.last_commit">
            <span>Last commit: <code>{{ repo.last_commit }}</code></span>
            <span>{{ repo.last_commit_author }}</span>
            <span>{{ repo.last_commit_title }}</span>
          </div>
          <div class="actions">
            <button @click="triggerReconcile(repo.id)">Reconcile</button>
          </div>
        </li>
      </ul>
      <p v-else class="hint">No repositories onboarded yet.</p>
    </section>

    <p v-if="error" class="error">{{ error }}</p>
  </main>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';

type Credential = {
  id: number;
  name: string;
  type: string;
};

type Repo = {
  id: number;
  name: string;
  repo_url: string;
  branch: string;
  last_commit?: string | null;
  last_commit_author?: string | null;
  last_commit_title?: string | null;
};

const credentials = ref<Credential[]>([]);
const repos = ref<Repo[]>([]);
const error = ref<string | null>(null);

const credentialForm = reactive({
  name: '',
  type: 'https-token',
  token: '',
  username: '',
  private_key: '',
  passphrase: '',
});

const repoForm = reactive({
  name: '',
  repo_url: '',
  branch: 'main',
  credential_id: 0,
});

onMounted(async () => {
  await Promise.all([loadCredentials(), loadRepos()]);
});

function formatType(value: string) {
  if (value === 'https-token') return 'HTTPS token';
  if (value === 'ssh-key') return 'SSH key';
  return value;
}

async function loadCredentials() {
  const res = await fetch('/api/credentials');
  if (!res.ok) {
    throw new Error('Unable to load credentials');
  }
  credentials.value = await res.json();
}

async function loadRepos() {
  const res = await fetch('/api/repos');
  if (!res.ok) {
    throw new Error('Unable to load repositories');
  }
  repos.value = await res.json();
}

async function submitCredential() {
  try {
    const res = await fetch('/api/credentials', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(credentialForm),
    });
    if (!res.ok) {
      const body = await res.json();
      throw new Error(body.error || 'Failed to create credential');
    }
    credentialForm.name = '';
    credentialForm.token = '';
    credentialForm.username = '';
    credentialForm.private_key = '';
    credentialForm.passphrase = '';
    await loadCredentials();
  } catch (err) {
    error.value = (err as Error).message;
  }
}

async function submitRepo() {
  try {
    const res = await fetch('/api/repos', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(repoForm),
    });
    if (!res.ok) {
      const body = await res.json();
      throw new Error(body.error || 'Failed to create repository');
    }
    repoForm.name = '';
    repoForm.repo_url = '';
    repoForm.branch = 'main';
    repoForm.credential_id = 0;
    await loadRepos();
  } catch (err) {
    error.value = (err as Error).message;
  }
}

async function triggerReconcile(id: number) {
  try {
    const res = await fetch(`/api/repos/${id}/reconcile`, { method: 'POST' });
    if (!res.ok) {
      const body = await res.json();
      throw new Error(body.error || 'Failed to trigger reconcile');
    }
  } catch (err) {
    error.value = (err as Error).message;
  }
}
</script>

<style scoped>
.container {
  max-width: 960px;
  margin: 0 auto;
  padding: 2rem;
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

header {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.subtitle {
  color: #52616b;
  margin: 0;
}

.panel {
  background: white;
  border-radius: 12px;
  padding: 1.5rem;
  box-shadow: 0 2px 12px rgba(15, 23, 42, 0.08);
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form {
  display: grid;
  gap: 1rem;
}

label {
  display: flex;
  flex-direction: column;
  font-weight: 600;
  gap: 0.3rem;
}

input,
select,
textarea {
  padding: 0.6rem;
  border: 1px solid #cbd2d9;
  border-radius: 8px;
  font-size: 1rem;
}

textarea {
  min-height: 120px;
}

button {
  align-self: flex-start;
  padding: 0.6rem 1rem;
  border-radius: 8px;
  border: none;
  background: #2563eb;
  color: white;
  font-weight: 600;
}

button:hover {
  background: #1d4ed8;
}

.list,
.repo-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.repo-list li {
  border: 1px solid #e4e7eb;
  border-radius: 10px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.meta {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.9rem;
  color: #52616b;
}

.actions {
  display: flex;
  gap: 0.5rem;
}

.tag {
  background: #e0e7ff;
  color: #3730a3;
  border-radius: 999px;
  padding: 0.2rem 0.6rem;
  font-size: 0.8rem;
  margin-left: 0.5rem;
}

.hint {
  color: #7b8794;
  margin: 0;
}

.error {
  color: #b91c1c;
  background: #fee2e2;
  border-radius: 8px;
  padding: 0.75rem;
}
</style>
