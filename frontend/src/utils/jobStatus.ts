import type { RepoJob } from '@/types';

function capitalize(value: string) {
  if (!value.length) return value;
  return value[0].toUpperCase() + value.slice(1);
}

export function getJobStatusLabel(job: RepoJob): string {
  if (job.status_error) {
    return 'Error';
  }
  const status = (job.status || job.nomad_status || '').toLowerCase();
  switch (status) {
    case 'healthy':
      return 'Healthy';
    case 'deploying':
      return 'Deploying';
    case 'degraded':
      return 'Degraded';
    case 'failed':
      return 'Failed';
    case 'lost':
      return 'Lost';
    case 'pending':
      return 'Pending';
    case 'dead':
      return 'Stopped';
    case 'missing':
      return 'Missing';
    default:
      if (!job.job_id) {
        return 'Pending';
      }
      if (!status) {
        return 'Unknown';
      }
      return capitalize(status);
  }
}

export function getJobStatusClass(job: RepoJob): string {
  if (job.status_error) {
    return 'danger';
  }
  const normalized = (job.status || job.nomad_status || '').toLowerCase();
  if (['healthy', 'running', 'successful', 'complete'].includes(normalized)) {
    return 'healthy';
  }
  if ([
    'deploying',
    'pending',
    'queued',
    'evaluating',
    'starting',
    'recovering',
    'restarting',
    'initializing',
    'rolling',
    'updating',
    'allocating',
  ].includes(normalized)) {
    return 'pending';
  }
  if (['degraded'].includes(normalized)) {
    return 'warning';
  }
  if (['failed', 'dead', 'lost', 'missing', 'cancelled'].includes(normalized)) {
    return 'danger';
  }
  return 'unknown';
}

export function getJobStatusTooltip(job: RepoJob): string {
  if (job.status_error) {
    return job.status_error;
  }
  if (job.status_description) {
    return job.status_description;
  }
  if (job.nomad_status) {
    return `Nomad status: ${capitalize(job.nomad_status)}`;
  }
  if (!job.job_id) {
    return 'Job has not been registered with Nomad yet.';
  }
  return 'Status is unavailable.';
}
