<template>
  <div v-if="isLoading" class="repo-detail-loading" aria-live="polite" aria-busy="true">
    <div class="skeleton loading-line loading-line--short"></div>
    <div class="skeleton loading-line loading-line--title"></div>
    <div class="skeleton loading-line"></div>
    <span class="sr-only">Loading repository</span>
  </div>

  <div v-else-if="repo" class="repo-detail">
    <nav class="breadcrumbs detail-breadcrumbs" aria-label="Breadcrumb">
      <ul>
        <li><button type="button" @click="goBack">Repositories</button></li>
        <li aria-current="page">{{ repo.name }}</li>
      </ul>
    </nav>

    <header class="repo-header">
      <div class="repo-header__copy">
        <div class="repo-title-row">
          <h1>{{ repo.name }}</h1>
          <span class="badge badge-ghost branch-badge">
            <IconGitBranch aria-hidden="true" />
            {{ repo.branch || 'default' }}
          </span>
        </div>
        <a v-if="repo.repo_url" class="repo-location" :href="repo.repo_url" target="_blank" rel="noopener noreferrer">
          <span>{{ repo.repo_url }}</span>
          <IconExternalLink aria-hidden="true" />
          <span class="sr-only">(opens in a new tab)</span>
        </a>
      </div>

      <div class="join repo-header__actions" aria-label="Repository actions">
        <button class="btn btn-primary btn-sm join-item" type="button" @click="handleReconcile" :disabled="isSyncing">
          <span v-if="isSyncing" class="loading loading-spinner loading-xs"></span>
          <IconRefresh v-else aria-hidden="true" />
          {{ isSyncing ? 'Syncing' : 'Sync now' }}
        </button>
        <button class="btn btn-outline btn-error btn-sm join-item" type="button" @click="showDeleteDialog = true" :disabled="isDeleting">
          <IconTrash aria-hidden="true" />
          Delete
        </button>
      </div>
    </header>

    <RepoPollingInfo :repo="repo" />

    <section class="stats stats-horizontal repo-metadata" aria-label="Repository configuration">
      <div class="stat">
        <div class="stat-figure"><IconGitBranch aria-hidden="true" /></div>
        <div class="stat-title">Branch</div>
        <div class="stat-value"><code>{{ repo.branch || '—' }}</code></div>
      </div>
      <div class="stat">
        <div class="stat-figure"><IconFolderCode aria-hidden="true" /></div>
        <div class="stat-title">Job path</div>
        <div class="stat-value"><code>{{ repo.job_path || '—' }}</code></div>
      </div>
      <div class="stat">
        <div class="stat-figure"><IconKey aria-hidden="true" /></div>
        <div class="stat-title">Credential</div>
        <div class="stat-value">{{ credentialName }}</div>
      </div>
    </section>

    <section class="jobs-panel" aria-labelledby="jobs-heading">
      <header class="section-heading">
        <div>
          <div class="section-title-row">
            <h2 id="jobs-heading">Nomad jobs</h2>
            <span class="badge badge-ghost badge-sm">{{ jobs.length }}</span>
          </div>
          <p>Workloads discovered from <code>{{ repo.job_path || '—' }}</code></p>
        </div>
      </header>
      <div v-if="hasAdoptableJobs" class="alert alert-warning alert-soft adoption-alert" role="alert">
        <IconAlertTriangle aria-hidden="true" />
        <span>Existing Nomad jobs are paused until you explicitly adopt them.</span>
      </div>
      <RepoJobList
        :jobs="jobs"
        :enable-collapse="false"
        :show-header="false"
        :adopting-job-path="adoptingJobPath"
        @adopt="handleAdopt"
      />
    </section>
  </div>

  <div v-else class="repo-detail-empty">
    <h1>Repository not found</h1>
    <p>This repository may have been removed or the link is no longer valid.</p>
    <button class="btn btn-primary btn-sm" type="button" @click="goBack">Return to repositories</button>
  </div>

  <ModalDialog
    :open="showDeleteDialog"
    title="Delete repository"
    :description="`Remove ${repo?.name || 'this repository'} from Compass.`"
    @close="closeDeleteDialog"
  >
    <div class="alert alert-warning alert-soft delete-warning" role="alert">
      <IconAlertTriangle aria-hidden="true" />
      <span>This stops repository monitoring. The Git repository itself will not be changed.</span>
    </div>
    <label class="delete-option">
      <input v-model="unscheduleJobs" type="checkbox" class="checkbox checkbox-sm" />
      <span><strong>Unschedule associated Nomad jobs</strong><small>Stop all jobs currently tracked from this repository.</small></span>
    </label>
    <template #footer>
      <button class="btn btn-ghost btn-sm" type="button" data-autofocus @click="closeDeleteDialog" :disabled="isDeleting">Cancel</button>
      <button class="btn btn-error btn-sm" type="button" @click="confirmDelete" :disabled="isDeleting">
        <span v-if="isDeleting" class="loading loading-spinner loading-xs"></span>
        {{ isDeleting ? 'Deleting' : 'Delete repository' }}
      </button>
    </template>
  </ModalDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
  IconAlertTriangle,
  IconExternalLink,
  IconFolderCode,
  IconGitBranch,
  IconKey,
  IconRefresh,
  IconTrash,
} from '@tabler/icons-vue';
import ModalDialog from '@/components/ModalDialog.vue';
import RepoJobList from '@/components/RepoJobList.vue';
import RepoPollingInfo from '@/components/RepoPollingInfo.vue';
import { useCompassStore } from '@/composables/useCompassStore';
import type { Repo } from '@/types';

const route = useRoute();
const router = useRouter();
const { repos, credentials, loadRepos, triggerReconcile, adoptJob, deleteRepo, syncingRepoId, adoptingJobPath, deletingRepoId } = useCompassStore();

const isLoading = ref(true);
const showDeleteDialog = ref(false);
const unscheduleJobs = ref(false);
const repoId = computed(() => {
  const raw = Number(route.params.id);
  return Number.isFinite(raw) ? raw : NaN;
});
const repo = computed<Repo | undefined>(() => repos.value.find((candidate) => candidate.id === repoId.value));
const jobs = computed(() => repo.value?.jobs ?? []);
const hasAdoptableJobs = computed(() => jobs.value.some((job) => job.adoptable));
const credential = computed(() => credentials.value.find((item) => item.id === repo.value?.credential_id));
const credentialName = computed(() => credential.value?.name ?? (repo.value?.credential_id ? 'Managed credential' : 'None required'));
const isSyncing = computed(() => syncingRepoId.value === repoId.value);
const isDeleting = computed(() => deletingRepoId.value === repoId.value);

async function ensureRepoLoaded(): Promise<void> {
  isLoading.value = true;
  try {
    if (!Number.isNaN(repoId.value) && (!repos.value.length || !repo.value)) await loadRepos();
  } finally {
    isLoading.value = false;
  }
}

onMounted(() => { void ensureRepoLoaded(); });
watch(() => route.params.id, () => { void ensureRepoLoaded(); });

function goBack(): void {
  void router.push({ name: 'dashboard' });
}

async function handleReconcile(): Promise<void> {
  if (!repo.value) return;
  try { await triggerReconcile(repo.value.id); } catch { /* surfaced globally */ }
}

async function handleAdopt(job: { path: string }): Promise<void> {
  if (!repo.value) return;
  try { await adoptJob(repo.value.id, job.path); } catch { /* surfaced globally */ }
}

function closeDeleteDialog(): void {
  if (isDeleting.value) return;
  showDeleteDialog.value = false;
  unscheduleJobs.value = false;
}

async function confirmDelete(): Promise<void> {
  if (!repo.value) return;
  try {
    await deleteRepo(repo.value.id, { unschedule: unscheduleJobs.value });
    showDeleteDialog.value = false;
    goBack();
  } catch { /* surfaced globally */ }
}
</script>

<style scoped>
.repo-detail { display: flex; flex-direction: column; gap: 1.25rem; }
.detail-breadcrumbs { max-width: 100%; padding: 0; color: var(--color-text-tertiary); font-size: .76rem; }
.detail-breadcrumbs button { padding: 0; color: var(--color-accent); font-size: inherit; font-weight: 500; }
.detail-breadcrumbs button:hover { text-decoration: underline; }
.repo-header { display: flex; align-items: flex-end; justify-content: space-between; gap: 2rem; padding-bottom: .15rem; }
.repo-header__copy { min-width: 0; }
.repo-title-row { display: flex; align-items: center; gap: .65rem; min-width: 0; }
.repo-title-row h1 { margin: 0; overflow: hidden; color: var(--color-text-primary); font-size: clamp(1.65rem, 3vw, 2.05rem); font-weight: 700; letter-spacing: -.035em; line-height: 1.05; text-overflow: ellipsis; white-space: nowrap; }
.branch-badge { gap: .3rem; min-height: 1.45rem; border-color: var(--color-border); color: var(--color-text-secondary); font-family: var(--font-mono); font-size: .68rem; }
.branch-badge svg { width: .78rem; }
.repo-location { display: flex; align-items: center; gap: .3rem; max-width: min(46rem, 100%); margin-top: .45rem; color: var(--color-text-tertiary); font-family: var(--font-mono); font-size: .73rem; }
.repo-location span:first-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.repo-location svg { flex: 0 0 auto; width: .78rem; }
.repo-header__actions { flex: 0 0 auto; border-radius: var(--radius-field); box-shadow: 0 1px 2px rgba(31, 33, 36, .08); }
.repo-header__actions .btn { min-width: 6.6rem; min-height: 2rem; height: 2rem; transition: background-color var(--transition-fast), border-color var(--transition-fast), color var(--transition-fast), transform var(--transition-fast); }
.repo-header__actions .btn:active:not(:disabled) { transform: translateY(1px); }
.repo-header__actions svg { width: .9rem; }
.repo-metadata { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); width: 100%; overflow: hidden; border: 1px solid var(--color-border); border-radius: var(--radius-box); background: var(--color-surface); box-shadow: none; }
.repo-metadata .stat { align-content: center; min-width: 0; min-height: 4.35rem; padding: .72rem .9rem; }
.repo-metadata .stat:not(:last-child) { border-right: 1px solid var(--color-border-soft); }
.repo-metadata .stat-figure { align-self: center; color: var(--color-text-subtle); }
.repo-metadata .stat-figure svg { width: 1rem; }
.repo-metadata .stat-title { color: var(--color-text-tertiary); font-size: .66rem; font-weight: 600; letter-spacing: .04em; text-transform: uppercase; }
.repo-metadata .stat-value { overflow: hidden; color: var(--color-text-primary); font-size: .78rem; font-weight: 600; line-height: 1.35; text-overflow: ellipsis; white-space: nowrap; }
.repo-metadata .stat-value code { font-family: var(--font-mono); font-size: .75rem; }
.jobs-panel { min-width: 0; }
.adoption-alert { margin-bottom: .7rem; }
.adoption-alert svg { width: 1rem; }
.adoption-alert span { font-size: .78rem; }
.section-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 1rem; margin-bottom: .7rem; }
.section-title-row { display: flex; align-items: center; gap: .5rem; }
.section-heading h2 { margin: 0; color: var(--color-text-primary); font-size: .98rem; font-weight: 680; letter-spacing: -.01em; }
.section-heading p { margin: .22rem 0 0; color: var(--color-text-tertiary); font-size: .75rem; }
.section-heading code { color: var(--color-text-secondary); font-family: var(--font-mono); font-size: .72rem; }
.repo-detail-loading { display: flex; flex-direction: column; gap: .9rem; padding-top: .5rem; }
.loading-line { width: 100%; height: 3.8rem; }
.loading-line--short { width: 8rem; height: 1.1rem; }
.loading-line--title { width: 22rem; max-width: 70%; height: 2.7rem; }
.repo-detail-empty { display: flex; flex-direction: column; align-items: center; padding: 4rem 1rem; text-align: center; }
.repo-detail-empty h1 { margin: 0 0 .3rem; font-size: 1.25rem; }
.repo-detail-empty p { margin: 0 0 1.2rem; color: var(--color-text-tertiary); font-size: .85rem; }
.delete-warning svg { width: 1.1rem; }
.delete-warning span { font-size: .8rem; }
.delete-option { display: flex; align-items: flex-start; gap: .7rem; padding: .2rem; cursor: pointer; }
.delete-option span { display: flex; flex-direction: column; gap: .12rem; }
.delete-option strong { color: var(--color-text-primary); font-size: .82rem; }
.delete-option small { color: var(--color-text-tertiary); font-size: .75rem; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
@media (max-width: 640px) { .repo-detail { gap: 1.1rem; } .repo-header { align-items: stretch; flex-direction: column; gap: 1rem; } .repo-header__actions { display: flex; width: 100%; } .repo-header__actions .btn { flex: 1; } .repo-title-row { flex-wrap: wrap; gap: .45rem; } .repo-title-row h1 { width: 100%; white-space: normal; } .repo-metadata { grid-template-columns: 1fr; grid-auto-flow: row; } .repo-metadata .stat:not(:last-child) { border-right: 0; border-bottom: 1px solid var(--color-border-soft); } .section-heading { align-items: flex-start; flex-direction: column; } }
@media (prefers-reduced-motion: reduce) { * { scroll-behavior: auto !important; transition-duration: .01ms !important; animation-duration: .01ms !important; } }
</style>
