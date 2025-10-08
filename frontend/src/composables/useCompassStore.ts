import { reactive, toRefs } from 'vue';

type Credential = {
  id: number;
  name: string;
  type: string;
  created_at?: string;
  updated_at?: string;
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
  jobs: RepoJob[];
};

type RepoJob = {
  path: string;
  job_id?: string;
  job_name?: string;
  last_commit?: string | null;
  updated_at: string;
  status?: string;
  status_description?: string;
  status_error?: string;
};

type CredentialPayload = {
  name: string;
  type: string;
  token?: string;
  username?: string;
  private_key?: string;
  passphrase?: string;
};

type RepoPayload = {
  name: string;
  repo_url: string;
  branch: string;
  credential_id?: number;
};

type DeleteCredentialOptions = {
  deleteRepos: boolean;
  unschedule: boolean;
};

type DeleteRepoOptions = {
  unschedule: boolean;
};

type CompassStatus = {
  nomad_connected: boolean;
  nomad_message?: string;
};

const state = reactive({
  credentials: [] as Credential[],
  repos: [] as Repo[],
  status: null as CompassStatus | null,
  error: null as string | null,
  refreshing: false,
  savingCredential: false,
  savingRepo: false,
  syncingRepoId: null as number | null,
  deletingRepoId: null as number | null,
  deletingCredentialId: null as number | null,
});

async function refreshAll() {
  try {
    state.refreshing = true;
    await Promise.all([loadCredentials(), loadRepos(), loadStatus()]);
    state.error = null;
  } catch (err) {
    setError(err);
  } finally {
    state.refreshing = false;
  }
}

async function loadCredentials() {
  const res = await fetch('/api/credentials');
  if (!res.ok) {
    throw new Error('Unable to load credentials');
  }
  state.credentials = await res.json();
}

async function loadRepos() {
  const res = await fetch('/api/repos');
  if (!res.ok) {
    throw new Error('Unable to load repositories');
  }
  const repos: Repo[] = await res.json();
  state.repos = repos.map((repo) => ({
    ...repo,
    jobs: repo.jobs ?? [],
  }));
}

async function loadStatus() {
  const res = await fetch('/api/status');
  if (!res.ok) {
    throw new Error('Unable to load Nomad status');
  }
  state.status = await res.json();
}

async function createCredential(payload: CredentialPayload) {
  try {
    state.savingCredential = true;
    const res = await fetch('/api/credentials', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!res.ok) {
      const body = await safeJson(res);
      throw new Error(body?.error || 'Failed to create credential');
    }
    await loadCredentials();
  } catch (err) {
    setError(err);
    throw err;
  } finally {
    state.savingCredential = false;
  }
}

async function deleteCredential(id: number, options: DeleteCredentialOptions) {
  try {
    state.deletingCredentialId = id;
    const res = await fetch(`/api/credentials/${id}`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        delete_repos: options.deleteRepos,
        unschedule: options.unschedule,
      }),
    });
    if (!res.ok) {
      const body = await safeJson(res);
      throw new Error(body?.error || 'Failed to delete credential');
    }
    await refreshAll();
  } catch (err) {
    setError(err);
    throw err;
  } finally {
    state.deletingCredentialId = null;
  }
}

async function createRepo(payload: RepoPayload) {
  try {
    state.savingRepo = true;
    const res = await fetch('/api/repos', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!res.ok) {
      const body = await safeJson(res);
      throw new Error(body?.error || 'Failed to create repository');
    }
    await loadRepos();
  } catch (err) {
    setError(err);
    throw err;
  } finally {
    state.savingRepo = false;
  }
}

async function triggerReconcile(id: number) {
  try {
    state.syncingRepoId = id;
    const res = await fetch(`/api/repos/${id}/reconcile`, { method: 'POST' });
    if (!res.ok) {
      const body = await safeJson(res);
      throw new Error(body?.error || 'Failed to trigger reconcile');
    }
    await loadRepos();
  } catch (err) {
    setError(err);
    throw err;
  } finally {
    state.syncingRepoId = null;
  }
}

async function deleteRepo(id: number, options: DeleteRepoOptions) {
  try {
    state.deletingRepoId = id;
    const res = await fetch(`/api/repos/${id}`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ unschedule: options.unschedule }),
    });
    if (!res.ok) {
      const body = await safeJson(res);
      throw new Error(body?.error || 'Failed to delete repository');
    }
    await loadRepos();
  } catch (err) {
    setError(err);
    throw err;
  } finally {
    state.deletingRepoId = null;
  }
}

function clearError() {
  state.error = null;
}

function setError(err: unknown) {
  if (err instanceof Error) {
    state.error = err.message;
  } else if (typeof err === 'string') {
    state.error = err;
  } else {
    state.error = 'Unexpected error';
  }
}

async function safeJson(res: Response) {
  try {
    return await res.json();
  } catch {
    return null;
  }
}

export function useCompassStore() {
  return {
    ...toRefs(state),
    refreshAll,
    loadCredentials,
    loadRepos,
    loadStatus,
    createCredential,
    deleteCredential,
    createRepo,
    deleteRepo,
    triggerReconcile,
    clearError,
    setError,
  };
}

export type { Credential, Repo, RepoJob, CompassStatus };
