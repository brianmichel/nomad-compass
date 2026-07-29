import type { IncomingMessage, ServerResponse } from 'node:http';
import type { Plugin } from 'vite';
import type { CompassStatus, Credential, Repo, RepoJob } from './src/types';

const now = new Date();
const minutesAgo = (minutes: number) => new Date(now.getTime() - minutes * 60_000).toISOString();

const job = (overrides: Partial<RepoJob> & Pick<RepoJob, 'path' | 'job_id'>): RepoJob => ({
  updated_at: minutesAgo(3),
  namespace: 'default',
  job_type: 'service',
  desired_allocations: 1,
  running_allocations: 1,
  starting_allocations: 0,
  queued_allocations: 0,
  failed_allocations: 0,
  lost_allocations: 0,
  unknown_allocations: 0,
  status: 'healthy',
  status_description: 'All allocations are running',
  job_url: `http://127.0.0.1:4646/ui/jobs/${overrides.job_id}@default`,
  ...overrides,
});

const initialRepos: Repo[] = [
  {
    id: 1,
    name: 'homelab',
    repo_url: 'https://github.com/brianmichel/homelab.git',
    branch: 'main',
    job_path: 'nomad',
    credential_id: 1,
    last_commit: '84eb0eb9ad1370670d3612bc70ee0fe5e0f140e1',
    last_commit_author: 'Brian Michel <brian.michel@gmail.com>',
    last_commit_title: 'Update observability stack',
    last_polled_at: minutesAgo(1),
    jobs: [
      job({ path: 'nomad/grafana.nomad.hcl', job_id: 'grafana', job_name: 'grafana' }),
      job({ path: 'nomad/prometheus.nomad.hcl', job_id: 'prometheus', job_name: 'prometheus', desired_allocations: 2, running_allocations: 2 }),
      job({ path: 'nomad/loki.nomad.hcl', job_id: 'loki', job_name: 'loki' }),
      job({ path: 'nomad/traefik.nomad.hcl', job_id: 'traefik', job_name: 'traefik', job_type: 'system', desired_allocations: 3, running_allocations: 3 }),
      job({ path: 'nomad/github-actions-runner.nomad.hcl', job_id: 'github-actions-runner', job_name: 'github-actions-runner', status: 'failed', status_description: 'Allocations failed to start', desired_allocations: 1, running_allocations: 0, failed_allocations: 1 }),
      job({ path: 'nomad/immich.nomad.hcl', job_id: 'immich', job_name: 'immich', status: 'pending', status_description: 'Waiting for placement', desired_allocations: 2, running_allocations: 1, queued_allocations: 1 }),
    ],
  },
  {
    id: 2,
    name: 'platform-jobs',
    repo_url: 'https://github.com/acme/platform-jobs.git',
    branch: 'production',
    job_path: '.nomad',
    credential_id: 2,
    last_commit: 'bf139a47744100eb56b0b4e799afa496ce50bea2',
    last_commit_author: 'Deploy Bot <deploy@example.com>',
    last_commit_title: 'Promote API release 2026.07.29',
    last_polled_at: minutesAgo(4),
    jobs: [
      job({ path: '.nomad/api.nomad.hcl', job_id: 'api', job_name: 'api', desired_allocations: 4, running_allocations: 4 }),
      job({ path: '.nomad/worker.nomad.hcl', job_id: 'worker', job_name: 'worker', desired_allocations: 6, running_allocations: 5, status: 'warning', status_description: 'One allocation is unhealthy', failed_allocations: 1 }),
      job({ path: '.nomad/cleanup.nomad.hcl', job_id: 'cleanup', job_name: 'cleanup', job_type: 'batch', desired_allocations: 1, running_allocations: 0, status: 'pending', status_description: 'Scheduled for next evaluation', queued_allocations: 1 }),
    ],
  },
  {
    id: 3,
    name: 'edge-services',
    repo_url: 'ssh://git@git.example.net/infra/edge-services.git',
    branch: 'main',
    job_path: 'jobs',
    credential_id: 2,
    last_commit: '04e989b397aa339f8653f661a65133147f91c20c',
    last_commit_author: 'Alex Chen <alex@example.com>',
    last_commit_title: 'Add regional ingress health checks',
    last_polled_at: minutesAgo(12),
    jobs: [
      job({ path: 'jobs/ingress.nomad.hcl', job_id: 'edge-ingress', job_name: 'edge-ingress', namespace: 'edge', desired_allocations: 3, running_allocations: 3 }),
      job({ path: 'jobs/dns.nomad.hcl', job_id: 'edge-dns', job_name: 'edge-dns', namespace: 'edge', job_type: 'system', desired_allocations: 5, running_allocations: 5 }),
    ],
  },
];

const initialCredentials: Credential[] = [
  { id: 1, name: 'GitHub read token', type: 'https-token', created_at: minutesAgo(10_080), updated_at: minutesAgo(10_080) },
  { id: 2, name: 'Infrastructure deploy key', type: 'ssh-key', created_at: minutesAgo(43_200), updated_at: minutesAgo(43_200) },
];

const status: CompassStatus = { nomad_connected: true, nomad_message: 'Connected to Nomad demo cluster' };

/** Provides an in-memory Compass API for local visual development. */
export function demoApiPlugin(): Plugin {
  let repos = structuredClone(initialRepos);
  let credentials = structuredClone(initialCredentials);

  return {
    name: 'compass-demo-api',
    configureServer(server) {
      server.middlewares.use('/api', async (request, response, next) => {
        const path = request.url?.split('?')[0] ?? '/';

        if (request.method === 'GET' && path === '/status') return sendJson(response, 200, status);
        if (request.method === 'GET' && path === '/repos') return sendJson(response, 200, repos);
        if (request.method === 'GET' && path === '/credentials') return sendJson(response, 200, credentials);

        if (request.method === 'POST' && path === '/repos') {
          const payload = await readJson(request);
          const created: Repo = {
            id: Math.max(0, ...repos.map(({ id }) => id)) + 1,
            name: stringField(payload, 'name', 'New repository'),
            repo_url: stringField(payload, 'repo_url', 'https://example.com/repository.git'),
            branch: stringField(payload, 'branch', 'main'),
            job_path: stringField(payload, 'job_path', '.nomad'),
            credential_id: numberField(payload, 'credential_id'),
            last_polled_at: new Date().toISOString(),
            jobs: [],
          };
          repos = [...repos, created];
          return sendJson(response, 200, created);
        }

        if (request.method === 'POST' && path === '/credentials') {
          const payload = await readJson(request);
          credentials = [...credentials, {
            id: Math.max(0, ...credentials.map(({ id }) => id)) + 1,
            name: stringField(payload, 'name', 'Demo credential'),
            type: stringField(payload, 'type', 'https-token'),
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          }];
          return sendJson(response, 200, { status: 'ok' });
        }

        const repoMatch = path.match(/^\/repos\/(\d+)(?:\/reconcile)?$/);
        if (repoMatch && request.method === 'POST') {
          const id = Number(repoMatch[1]);
          repos = repos.map((repo) => repo.id === id ? { ...repo, last_polled_at: new Date().toISOString() } : repo);
          return sendJson(response, 202, { status: 'ok' });
        }
        if (repoMatch && request.method === 'DELETE') {
          repos = repos.filter(({ id }) => id !== Number(repoMatch[1]));
          return sendJson(response, 200, { status: 'ok' });
        }

        const credentialMatch = path.match(/^\/credentials\/(\d+)$/);
        if (credentialMatch && request.method === 'DELETE') {
          credentials = credentials.filter(({ id }) => id !== Number(credentialMatch[1]));
          return sendJson(response, 200, { status: 'ok' });
        }

        next();
      });
    },
  };
}

function sendJson(response: ServerResponse, statusCode: number, value: unknown): void {
  response.statusCode = statusCode;
  response.setHeader('Content-Type', 'application/json');
  response.end(JSON.stringify(value));
}

async function readJson(request: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];
  for await (const chunk of request) chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
  if (chunks.length === 0) return {};
  try { return JSON.parse(Buffer.concat(chunks).toString('utf8')) as unknown; } catch { return {}; }
}

function stringField(value: unknown, key: string, fallback: string): string {
  if (typeof value !== 'object' || value === null) return fallback;
  const field = Reflect.get(value, key);
  return typeof field === 'string' && field.trim() ? field : fallback;
}

function numberField(value: unknown, key: string): number | undefined {
  if (typeof value !== 'object' || value === null) return undefined;
  const field = Reflect.get(value, key);
  return typeof field === 'number' && Number.isFinite(field) ? field : undefined;
}
