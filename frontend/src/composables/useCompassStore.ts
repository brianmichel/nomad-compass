import { reactive, toRefs } from 'vue';
import {
  createCredential as requestCreateCredential,
  createRepo as requestCreateRepo,
  fetchCredentials,
  fetchRepos,
  fetchStatus,
  removeCredential,
  removeRepo,
  triggerRepoReconcile,
} from '@/services/compassApi';
import type {
  CompassStatus,
  Credential,
  CredentialPayload,
  DeleteCredentialOptions,
  DeleteRepoOptions,
  Repo,
  RepoPayload,
} from '@/types';
import { ApiError } from '@/services/http';

interface CompassState {
  credentials: Credential[];
  repos: Repo[];
  status: CompassStatus | null;
  error: string | null;
  refreshing: boolean;
  savingCredential: boolean;
  savingRepo: boolean;
  syncingRepoId: number | null;
  deletingRepoId: number | null;
  deletingCredentialId: number | null;
}

const state = reactive<CompassState>({
  credentials: [],
  repos: [],
  status: null,
  error: null,
  refreshing: false,
  savingCredential: false,
  savingRepo: false,
  syncingRepoId: null,
  deletingRepoId: null,
  deletingCredentialId: null,
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
  state.credentials = await fetchCredentials();
}

async function loadRepos() {
  const repos = await fetchRepos();
  state.repos = repos.map((repo) => ({
    ...repo,
    jobs: repo.jobs ?? [],
  }));
}

async function loadStatus() {
  state.status = await fetchStatus();
}

async function createCredential(payload: CredentialPayload) {
  try {
    state.savingCredential = true;
    await requestCreateCredential(payload);
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
    await removeCredential(id, options);
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
    const createdRepo = await requestCreateRepo(payload);
    await loadRepos();
    if (createdRepo?.id) {
      await waitForRepoJobs(createdRepo.id);
    }
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
    await triggerRepoReconcile(id);
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
    await removeRepo(id, options);
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
  if (err instanceof ApiError) {
    state.error = err.message;
  } else if (err instanceof Error) {
    state.error = err.message;
  } else if (typeof err === 'string') {
    state.error = err;
  } else {
    state.error = 'Unexpected error';
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

async function waitForRepoJobs(repoId: number) {
  const maxAttempts = 6;
  const delayMs = 2000;
  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    const repo = state.repos.find((r) => r.id === repoId);
    if (repo && repo.jobs && repo.jobs.length > 0) {
      return;
    }
    await sleep(delayMs);
    await loadRepos();
  }
}

function sleep(duration: number) {
  return new Promise((resolve) => setTimeout(resolve, duration));
}
