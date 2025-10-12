import { describe, expect, it } from 'vitest';
import { buildCommitUrl, formatCommitHash } from '@/utils/repos';

const baseRepo = {
  id: 1,
  name: 'demo',
  repo_url: '',
  branch: 'main',
  job_path: '.nomad',
  jobs: [],
};

describe('buildCommitUrl', () => {
  it('returns null when commit is missing', () => {
    expect(buildCommitUrl({ ...baseRepo, last_commit: null })).toBeNull();
  });

  it('builds URLs for HTTPS repositories', () => {
    const url = buildCommitUrl({ ...baseRepo, repo_url: 'https://github.com/acme/demo', last_commit: 'abcdef1' });
    expect(url).toBe('https://github.com/acme/demo/commit/abcdef1');
  });

  it('builds URLs for SSH repositories', () => {
    const url = buildCommitUrl({ ...baseRepo, repo_url: 'git@github.com:acme/demo.git', last_commit: 'abcdef1' });
    expect(url).toBe('https://github.com/acme/demo/commit/abcdef1');
  });

  it('handles ssh protocol URLs', () => {
    const url = buildCommitUrl({ ...baseRepo, repo_url: 'ssh://git@example.com/acme/demo.git', last_commit: 'abcdef1' });
    expect(url).toBe('https://example.com/acme/demo/commit/abcdef1');
  });
});

describe('formatCommitHash', () => {
  it('falls back to pending when hash is missing', () => {
    expect(formatCommitHash(null)).toBe('pending');
  });

  it('trims hash to the provided length', () => {
    expect(formatCommitHash('abcdef123456', 8)).toBe('abcdef12');
  });
});
