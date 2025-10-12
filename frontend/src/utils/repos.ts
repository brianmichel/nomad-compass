import type { Repo } from '@/types';

export function buildCommitUrl(repo: Pick<Repo, 'repo_url' | 'last_commit'>) {
  if (!repo.last_commit) {
    return null;
  }

  const rawUrl = (repo.repo_url || '').trim();
  if (!rawUrl) {
    return null;
  }

  const normalized = rawUrl.replace(/\.git$/i, '');

  if (normalized.startsWith('http://') || normalized.startsWith('https://')) {
    return `${normalized}/commit/${repo.last_commit}`;
  }

  if (normalized.startsWith('git@')) {
    const match = normalized.match(/^git@([^:]+):(.+)$/);
    if (match) {
      const [, host, path] = match;
      return `https://${host}/${path}/commit/${repo.last_commit}`;
    }
  }

  if (normalized.startsWith('ssh://')) {
    try {
      const url = new URL(normalized);
      const host = url.hostname;
      const path = url.pathname.replace(/\.git$/i, '').replace(/^\/+/, '');
      return `https://${host}/${path}/commit/${repo.last_commit}`;
    } catch {
      return null;
    }
  }

  return null;
}

export function formatCommitHash(hash?: string | null, length = 7) {
  if (!hash) {
    return 'pending';
  }
  return hash.slice(0, length);
}
