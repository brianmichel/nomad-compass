export interface Credential {
  id: number;
  name: string;
  type: string;
  created_at?: string;
  updated_at?: string;
}

export interface AllocationStatus {
  id: string;
  name?: string;
  client?: string;
  status?: string;
  desired?: string;
  group?: string;
  healthy?: boolean | null;
}

export interface RepoJob {
  path: string;
  job_id?: string;
  job_name?: string;
  namespace?: string;
  job_type?: string;
  last_commit?: string | null;
  updated_at: string;
  status?: string;
  status_description?: string;
  status_error?: string;
  nomad_status?: string;
  desired_allocations?: number;
  running_allocations?: number;
  starting_allocations?: number;
  queued_allocations?: number;
  failed_allocations?: number;
  lost_allocations?: number;
  unknown_allocations?: number;
  latest_deployment_id?: string;
  latest_allocation_id?: string;
  latest_allocation_name?: string;
  job_url?: string;
  allocations?: AllocationStatus[];
}

export interface Repo {
  id: number;
  name: string;
  repo_url: string;
  branch: string;
  job_path: string;
  credential_id?: number | null;
  last_commit?: string | null;
  last_commit_author?: string | null;
  last_commit_title?: string | null;
  last_polled_at?: string | null;
  jobs: RepoJob[];
}

export interface CompassStatus {
  nomad_connected: boolean;
  nomad_message?: string;
}

export interface CredentialPayload {
  name: string;
  type: string;
  token?: string;
  username?: string;
  private_key?: string;
  passphrase?: string;
}

export interface RepoPayload {
  name: string;
  repo_url: string;
  branch: string;
  job_path: string;
  credential_id?: number;
}

export interface DeleteCredentialOptions {
  deleteRepos: boolean;
  unschedule: boolean;
}

export interface DeleteRepoOptions {
  unschedule: boolean;
}
