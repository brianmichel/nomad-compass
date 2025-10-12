import { httpRequest } from './http';
import type {
  CompassStatus,
  Credential,
  CredentialPayload,
  DeleteCredentialOptions,
  DeleteRepoOptions,
  Repo,
  RepoPayload,
} from '@/types';

const API_BASE = '/api';

export function fetchCredentials() {
  return httpRequest<Credential[]>(`${API_BASE}/credentials`);
}

export function createCredential(payload: CredentialPayload) {
  return httpRequest<void>(`${API_BASE}/credentials`, {
    method: 'POST',
    json: payload,
  });
}

export function removeCredential(id: number, options: DeleteCredentialOptions) {
  return httpRequest<void>(`${API_BASE}/credentials/${id}`, {
    method: 'DELETE',
    json: {
      delete_repos: options.deleteRepos,
      unschedule: options.unschedule,
    },
  });
}

export function fetchRepos() {
  return httpRequest<Repo[]>(`${API_BASE}/repos`);
}

export function createRepo(payload: RepoPayload) {
  return httpRequest<Repo>(`${API_BASE}/repos`, {
    method: 'POST',
    json: payload,
  });
}

export function removeRepo(id: number, options: DeleteRepoOptions) {
  return httpRequest<void>(`${API_BASE}/repos/${id}`, {
    method: 'DELETE',
    json: { unschedule: options.unschedule },
  });
}

export function triggerRepoReconcile(id: number) {
  return httpRequest<void>(`${API_BASE}/repos/${id}/reconcile`, {
    method: 'POST',
  });
}

export function fetchStatus() {
  return httpRequest<CompassStatus>(`${API_BASE}/status`);
}
