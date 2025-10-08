<template>
  <div class="app-shell">
    <nav class="topbar">
      <div class="brand">
        <div class="brand-icon">🧭</div>
        <div class="brand-copy">
          <span class="brand-title">Nomad Compass</span>
          <span class="brand-subtitle">GitOps recon supervisor</span>
        </div>
      </div>
      <div class="topbar-actions">
        <span class="status-badge">
          <span class="pulse"></span>
          Watchdog active
        </span>
        <button class="ghost" type="button" @click="refreshAll" :disabled="isRefreshing">
          <span class="button-icon">
            <span v-if="isRefreshing" class="loader"></span>
            <span v-else class="refresh-icon">⟳</span>
          </span>
          <span>Refresh</span>
        </button>
      </div>
    </nav>

    <div class="layout">
      <aside class="sidebar">
        <section class="panel glass">
          <header class="panel-header">
            <div>
              <h2>Credentials</h2>
              <p>HTTPS tokens and SSH keys are sealed with your container key.</p>
            </div>
            <span class="count-badge">{{ credentials.length }}</span>
          </header>

          <form class="form-grid" @submit.prevent="submitCredential">
            <label class="field">
              <span>Display name</span>
              <input v-model="credentialForm.name" placeholder="production-github" required />
            </label>

            <label class="field">
              <span>Type</span>
              <select v-model="credentialForm.type">
                <option value="https-token">HTTPS token</option>
                <option value="ssh-key">SSH key</option>
              </select>
            </label>

            <template v-if="credentialForm.type === 'https-token'">
              <label class="field">
                <span>Username <small>(optional)</small></span>
                <input v-model="credentialForm.username" placeholder="git" />
              </label>

              <label class="field full">
                <span>Token</span>
                <input v-model="credentialForm.token" placeholder="ghp_xxx" required />
              </label>
            </template>

            <template v-else>
              <label class="field full">
                <span>Private key</span>
                <textarea v-model="credentialForm.private_key" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" required></textarea>
              </label>

              <label class="field">
                <span>Passphrase <small>(optional)</small></span>
                <input v-model="credentialForm.passphrase" type="password" />
              </label>
            </template>

            <button class="primary" type="submit" :disabled="credentialSubmitting">
              <span v-if="credentialSubmitting" class="loader"></span>
              <span v-else>Add credential</span>
            </button>
          </form>

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
                  @click="deleteCredential(cred)"
                  :disabled="deletingCredential === cred.id"
                >
                  <span v-if="deletingCredential === cred.id" class="loader"></span>
                  <span v-else>Delete</span>
                </button>
              </div>
            </article>
          </div>
          <p v-else class="empty-state">No credentials yet.</p>
        </section>
      </aside>

      <main class="content">
        <section class="panel glass">
          <header class="panel-header">
            <div>
              <h2>Onboard repository</h2>
              <p>Clone and watch `.nomad/*.nomad.hcl` job specs from Git.</p>
            </div>
          </header>

          <form class="form-grid" @submit.prevent="submitRepo">
            <label class="field">
              <span>Display name</span>
              <input v-model="repoForm.name" placeholder="payments" required />
            </label>
            <label class="field full">
              <span>Repository URL</span>
              <input v-model="repoForm.repo_url" placeholder="git@github.com:acme/payments.git" required />
            </label>
            <label class="field">
              <span>Branch</span>
              <input v-model="repoForm.branch" placeholder="main" required />
            </label>
            <label class="field">
              <span>Credential</span>
              <select v-model.number="repoForm.credential_id">
                <option value="0">None (public)</option>
                <option v-for="cred in credentials" :value="cred.id" :key="cred.id">
                  {{ cred.name }}
                </option>
              </select>
            </label>
            <button class="primary" type="submit" :disabled="repoSubmitting">
              <span v-if="repoSubmitting" class="loader"></span>
              <span v-else>Onboard repository</span>
            </button>
          </form>
        </section>

        <section class="panel glass repo-panel">
          <header class="panel-header">
            <div>
              <h2>Tracked repositories</h2>
              <p v-if="repos.length">Showing {{ repos.length }} sources.</p>
              <p v-else>Onboard your first repository to start reconciling jobs.</p>
            </div>
          </header>

          <ul v-if="repos.length" class="repo-list">
            <li v-for="repo in repos" :key="repo.id" class="repo-card" :class="{ active: triggeringRepo === repo.id }">
              <div class="repo-header">
                <div>
                  <span class="repo-name">{{ repo.name }}</span>
                  <span class="repo-branch">{{ repo.branch }}</span>
                </div>
                <div class="repo-actions">
                  <button class="ghost small" type="button" @click="triggerReconcile(repo.id)" :disabled="triggeringRepo === repo.id">
                    <span v-if="triggeringRepo === repo.id" class="loader"></span>
                    <span v-else>Sync now</span>
                  </button>
                  <button
                    class="ghost danger small"
                    type="button"
                    @click="deleteRepo(repo)"
                    :disabled="deletingRepo === repo.id"
                  >
                    <span v-if="deletingRepo === repo.id" class="loader"></span>
                    <span v-else>Delete</span>
                  </button>
                </div>
              </div>

              <div class="repo-meta">
                <span class="meta-pill" :title="repo.repo_url">
                  <span class="label">Source</span>
                  {{ repo.repo_url }}
                </span>
                <span class="meta-pill">
                  <span class="label">Credential</span>
                  {{ repo.credential_id ? 'Managed secret' : 'Public' }}
                </span>
              </div>

              <div class="repo-commit">
                <span class="commit-hash" :title="repo.last_commit || 'Awaiting first run'">
                  #{{ commitDisplay(repo.last_commit) }}
                </span>
                <span v-if="repo.last_commit_title" class="commit-title">{{ repo.last_commit_title }}</span>
                <span v-else class="commit-title muted">No commits reconciled yet.</span>
              </div>

              <div class="repo-footer">
                <span>
                  <strong>Author:</strong>
                  <span>{{ repo.last_commit_author || '—' }}</span>
                </span>
                <span>
                  <strong>Last polled:</strong>
                  <span>{{ formatTimestamp(repo.last_polled_at) }}</span>
                </span>
              </div>
            </li>
          </ul>
        </section>
      </main>
    </div>

    <transition name="toast">
      <div v-if="error" class="toast">
        <span>{{ error }}</span>
        <button class="ghost" type="button" @click="error = null">Dismiss</button>
      </div>
    </transition>
  </div>
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
  credential_id?: number | null;
  last_commit?: string | null;
  last_commit_author?: string | null;
  last_commit_title?: string | null;
  last_polled_at?: string | null;
};

const credentials = ref<Credential[]>([]);
const repos = ref<Repo[]>([]);
const error = ref<string | null>(null);

const credentialSubmitting = ref(false);
const repoSubmitting = ref(false);
const triggeringRepo = ref<number | null>(null);
const isRefreshing = ref(false);
const deletingRepo = ref<number | null>(null);
const deletingCredential = ref<number | null>(null);

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

onMounted(() => {
  refreshAll();
});

async function refreshAll() {
  try {
    isRefreshing.value = true;
    await Promise.all([loadCredentials(), loadRepos()]);
    error.value = null;
  } catch (err) {
    error.value = (err as Error).message;
  } finally {
    isRefreshing.value = false;
  }
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
    credentialSubmitting.value = true;
    error.value = null;
    const res = await fetch('/api/credentials', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(credentialForm),
    });
    if (!res.ok) {
      const body = await res.json();
      throw new Error(body.error || 'Failed to create credential');
    }
    resetCredentialForm();
    await loadCredentials();
  } catch (err) {
    error.value = (err as Error).message;
  } finally {
    credentialSubmitting.value = false;
  }
}

async function submitRepo() {
  try {
    repoSubmitting.value = true;
    error.value = null;
    const res = await fetch('/api/repos', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(repoForm),
    });
    if (!res.ok) {
      const body = await res.json();
      throw new Error(body.error || 'Failed to create repository');
    }
    resetRepoForm();
    await loadRepos();
  } catch (err) {
    error.value = (err as Error).message;
  } finally {
    repoSubmitting.value = false;
  }
}

async function triggerReconcile(id: number) {
  try {
    triggeringRepo.value = id;
    error.value = null;
    const res = await fetch(`/api/repos/${id}/reconcile`, { method: 'POST' });
    if (!res.ok) {
      const body = await res.json();
      throw new Error(body.error || 'Failed to trigger reconcile');
    }
    await loadRepos();
  } catch (err) {
    error.value = (err as Error).message;
  } finally {
    triggeringRepo.value = null;
  }
}

async function deleteRepo(repo: Repo) {
  try {
    if (!window.confirm(`Delete repository "${repo.name}"?`)) {
      return;
    }
    const unschedule = window.confirm('Unschedule associated Nomad jobs?');
    deletingRepo.value = repo.id;
    error.value = null;
    const res = await fetch(`/api/repos/${repo.id}`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ unschedule }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || 'Failed to delete repository');
    }
    await loadRepos();
  } catch (err) {
    error.value = (err as Error).message;
  } finally {
    deletingRepo.value = null;
  }
}

async function deleteCredential(cred: Credential) {
  try {
    if (!window.confirm(`Delete credential "${cred.name}"?`)) {
      return;
    }
    const deleteRepos = window.confirm('Delete repositories that use this credential?');
    let unschedule = false;
    if (deleteRepos) {
      unschedule = window.confirm('Unschedule associated Nomad jobs as well?');
    }
    deletingCredential.value = cred.id;
    error.value = null;
    const res = await fetch(`/api/credentials/${cred.id}`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ delete_repos: deleteRepos, unschedule }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || 'Failed to delete credential');
    }
    await refreshAll();
  } catch (err) {
    error.value = (err as Error).message;
  } finally {
    deletingCredential.value = null;
  }
}

function resetCredentialForm() {
  credentialForm.name = '';
  credentialForm.token = '';
  credentialForm.username = '';
  credentialForm.private_key = '';
  credentialForm.passphrase = '';
  credentialForm.type = credentialForm.type || 'https-token';
}

function resetRepoForm() {
  repoForm.name = '';
  repoForm.repo_url = '';
  repoForm.branch = 'main';
  repoForm.credential_id = 0;
}

function formatType(value: string) {
  if (value === 'https-token') return 'HTTPS token';
  if (value === 'ssh-key') return 'SSH key';
  return value;
}

function commitDisplay(commit?: string | null) {
  if (!commit) return 'pending';
  return commit.slice(0, 7);
}

function formatTimestamp(value?: string | null) {
  if (!value) return 'Awaiting first poll';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}
</script>

<style scoped>
.app-shell {
  min-height: 100vh;
  padding: 2.5rem clamp(1rem, 4vw, 4rem);
  display: flex;
  flex-direction: column;
  gap: 2rem;
  max-width: 1200px;
  margin: 0 auto;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.5rem;
  border-radius: 20px;
  border: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(15, 23, 42, 0.72);
  backdrop-filter: blur(18px);
  box-shadow: 0 25px 60px -40px rgba(15, 23, 42, 0.7);
}

.brand {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.brand-icon {
  width: 44px;
  height: 44px;
  display: grid;
  place-items: center;
  font-size: 1.5rem;
  border-radius: 14px;
  background: linear-gradient(145deg, rgba(59, 130, 246, 0.45), rgba(14, 116, 144, 0.4));
  box-shadow: inset 0 0 0 1px rgba(148, 163, 184, 0.2);
}

.brand-copy {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.brand-title {
  font-weight: 600;
  font-size: 1.2rem;
}

.brand-subtitle {
  font-size: 0.85rem;
  color: rgba(148, 163, 184, 0.8);
}

.topbar-actions {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.85rem;
  padding: 0.3rem 0.75rem;
  border-radius: 999px;
  background: rgba(34, 197, 94, 0.15);
  border: 1px solid rgba(34, 197, 94, 0.35);
  color: #bbf7d0;
}

.pulse {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: #34d399;
  box-shadow: 0 0 0 0 rgba(52, 211, 153, 0.7);
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(52, 211, 153, 0.7);
  }
  70% {
    box-shadow: 0 0 0 12px rgba(52, 211, 153, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(52, 211, 153, 0);
  }
}

.layout {
  display: grid;
  grid-template-columns: minmax(260px, 320px) 1fr;
  gap: 1.75rem;
  flex: 1;
}

.sidebar {
  display: flex;
  flex-direction: column;
  gap: 1.75rem;
}

.content {
  display: flex;
  flex-direction: column;
  gap: 1.75rem;
}

.panel {
  padding: 1.75rem;
  border-radius: 22px;
}

.glass {
  background: rgba(15, 23, 42, 0.78);
  border: 1px solid rgba(148, 163, 184, 0.18);
  box-shadow: 0 20px 55px -35px rgba(2, 6, 23, 0.85);
  backdrop-filter: blur(18px);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  margin-bottom: 1.4rem;
}

.panel-header h2 {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 600;
}

.panel-header p {
  margin: 0.2rem 0 0 0;
  font-size: 0.9rem;
  color: rgba(148, 163, 184, 0.9);
}

.count-badge {
  align-self: center;
  padding: 0.35rem 0.75rem;
  border-radius: 999px;
  background: rgba(99, 102, 241, 0.18);
  border: 1px solid rgba(129, 140, 248, 0.35);
  color: #c7d2fe;
  font-size: 0.85rem;
  font-weight: 600;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem 1.25rem;
  align-items: start;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  font-weight: 500;
  color: #e2e8f0;
  font-size: 0.95rem;
}

.field small {
  font-weight: 400;
  color: rgba(148, 163, 184, 0.8);
}

.field.full {
  grid-column: 1 / -1;
}

button {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  border-radius: 12px;
  border: none;
  font-weight: 600;
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease, opacity 0.15s ease;
}

button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
  transform: none !important;
  box-shadow: none !important;
}

.primary {
  justify-self: flex-start;
  background: linear-gradient(135deg, #2563eb, #7c3aed);
  color: white;
  padding: 0.7rem 1.4rem;
  box-shadow: 0 15px 30px -15px rgba(37, 99, 235, 0.65);
}

.primary:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 18px 35px -15px rgba(129, 140, 248, 0.55);
}

.ghost {
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(148, 163, 184, 0.25);
  color: rgba(226, 232, 240, 0.95);
  padding: 0.55rem 1rem;
}

.ghost.danger {
  border-color: rgba(248, 113, 113, 0.35);
  color: #fecaca;
}

.ghost.small {
  padding: 0.45rem 0.9rem;
  font-size: 0.85rem;
}

.ghost:hover:not(:disabled) {
  transform: translateY(-1px);
}

.ghost.danger:hover:not(:disabled) {
  box-shadow: 0 12px 25px -15px rgba(248, 113, 113, 0.45);
}

.button-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
}

.refresh-icon {
  font-size: 0.95rem;
  color: rgba(226, 232, 240, 0.8);
}

.loader {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 2px solid rgba(226, 232, 240, 0.35);
  border-top-color: rgba(226, 232, 240, 0.95);
  animation: spin 0.9s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.credential-list {
  margin-top: 1.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.credential-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.85rem 1rem;
  border-radius: 14px;
  border: 1px solid rgba(59, 130, 246, 0.15);
  background: rgba(30, 41, 59, 0.55);
}

.row-actions {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.credential-row strong {
  display: block;
  font-weight: 600;
}

.credential-row p {
  margin: 0.2rem 0 0 0;
  font-size: 0.85rem;
  color: rgba(148, 163, 184, 0.85);
}

.pill {
  padding: 0.3rem 0.75rem;
  border-radius: 999px;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  background: rgba(59, 130, 246, 0.18);
  border: 1px solid rgba(59, 130, 246, 0.35);
  color: #bfdbfe;
}

.repo-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.repo-card {
  padding: 1.4rem 1.5rem;
  border-radius: 18px;
  border: 1px solid rgba(100, 116, 139, 0.2);
  background: rgba(15, 23, 42, 0.68);
  transition: border 0.2s ease, transform 0.2s ease;
}

.repo-card:hover {
  border-color: rgba(129, 140, 248, 0.35);
  transform: translateY(-1px);
}

.repo-card.active {
  border-color: rgba(59, 130, 246, 0.6);
}

.repo-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
}

.repo-actions {
  display: flex;
  gap: 0.6rem;
}

.repo-name {
  font-size: 1.05rem;
  font-weight: 600;
}

.repo-branch {
  margin-left: 0.75rem;
  font-size: 0.8rem;
  border-radius: 999px;
  padding: 0.25rem 0.75rem;
  background: rgba(236, 72, 153, 0.16);
  border: 1px solid rgba(236, 72, 153, 0.28);
  color: #fbcfe8;
}

.repo-meta {
  margin-top: 1rem;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 0.75rem;
}

.meta-pill {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  padding: 0.75rem 0.9rem;
  border-radius: 14px;
  background: rgba(30, 41, 59, 0.66);
  border: 1px solid rgba(148, 163, 184, 0.2);
  font-size: 0.85rem;
  color: rgba(226, 232, 240, 0.9);
}

.meta-pill .label {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: rgba(148, 163, 184, 0.75);
}

.repo-commit {
  margin-top: 1rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
}

.commit-hash {
  font-family: 'JetBrains Mono', 'SFMono-Regular', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  background: rgba(129, 140, 248, 0.18);
  border: 1px solid rgba(129, 140, 248, 0.4);
  border-radius: 10px;
  padding: 0.35rem 0.65rem;
  font-size: 0.85rem;
  letter-spacing: 0.05em;
  color: #ede9fe;
}

.commit-title {
  font-size: 0.9rem;
  color: rgba(226, 232, 240, 0.9);
}

.commit-title.muted {
  color: rgba(148, 163, 184, 0.65);
}

.repo-footer {
  margin-top: 1.25rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem 1.5rem;
  font-size: 0.85rem;
  color: rgba(148, 163, 184, 0.85);
}

.repo-footer strong {
  color: rgba(226, 232, 240, 0.9);
  margin-right: 0.45rem;
}

.empty-state {
  margin: 1rem 0 0 0;
  font-size: 0.9rem;
  color: rgba(148, 163, 184, 0.75);
}

.toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  display: inline-flex;
  align-items: center;
  gap: 1rem;
  padding: 0.85rem 1.2rem;
  border-radius: 14px;
  background: rgba(248, 113, 113, 0.12);
  border: 1px solid rgba(248, 113, 113, 0.35);
  color: #fecaca;
  box-shadow: 0 18px 45px -25px rgba(248, 113, 113, 0.55);
}

.toast-enter-active,
.toast-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(10px);
}

@media (max-width: 1080px) {
  .layout {
    grid-template-columns: 1fr;
  }

  .sidebar {
    order: 2;
  }

  .topbar {
    flex-direction: column;
    align-items: flex-start;
    gap: 1rem;
  }
}
</style>
