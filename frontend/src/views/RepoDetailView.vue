<template>
  <div class="repo-detail" v-if="repo">
    <header class="repo-detail__toolbar">
      <button class="btn btn-ghost btn-sm detail-back" type="button" @click="goBack">
        <span aria-hidden="true">←</span> Repositories
      </button>
      <div class="repo-detail__actions">
        <button class="btn btn-outline btn-sm" type="button" @click="handleReconcile" :disabled="isSyncing">
          <span v-if="isSyncing" class="loading loading-spinner loading-xs"></span>
          <span v-else>Sync</span>
        </button>
        <button class="btn btn-error btn-outline btn-sm" type="button" @click="handleDelete" :disabled="isDeleting">
          <span v-if="isDeleting" class="loading loading-spinner loading-xs"></span>
          <span v-else>Delete</span>
        </button>
      </div>
    </header>

    <section class="repo-detail__hero">
      <div class="repo-detail__identity">
        <span class="detail-kicker">Repository</span>
        <h1>{{ repo.name }}</h1>
        <a v-if="repo.repo_url" class="repo-detail__location" :href="repo.repo_url" target="_blank" rel="noopener noreferrer">
          {{ repo.repo_url }} <span aria-hidden="true">↗</span>
        </a>
        <span v-else class="repo-detail__location">—</span>
      </div>
      <RepoPollingInfo class="repo-detail__commit" :repo="repo" />
    </section>

    <div class="tabs tabs-border detail-tabs" role="tablist" aria-label="Repository details">
      <a class="tab tab-active" role="tab" aria-selected="true">Overview</a>
      <a class="tab" role="tab">Jobs <span class="badge badge-ghost badge-sm">{{ repo.jobs?.length || 0 }}</span></a>
      <a class="tab" role="tab">Configuration</a>
    </div>

    <section class="repo-detail__facts" aria-label="Repository configuration">
      <div class="detail-fact">
        <span>Credential</span>
        <strong>{{ credentialLabel }}</strong>
      </div>
      <div class="detail-fact">
        <span>Namespace</span>
        <strong>{{ repo.nomad_namespace || '—' }}</strong>
      </div>
      <div class="detail-fact">
        <span>Job path</span>
        <strong class="font-mono">{{ repo.job_path || '—' }}</strong>
      </div>
      <div class="detail-fact">
        <span>Branch</span>
        <strong>{{ repo.branch || '—' }}</strong>
      </div>
    </section>

    <section class="repo-detail__jobs" aria-labelledby="jobs-heading">
      <div class="jobs-section-heading">
        <div>
          <h2 id="jobs-heading">Jobs</h2>
          <p>Nomad jobs discovered from this repository.</p>
        </div>
        <span class="badge badge-ghost badge-sm">{{ repo.jobs?.length || 0 }} tracked</span>
      </div>
      <RepoJobList :jobs="repo.jobs || []" :enable-collapse="false" :show-header="false" />
    </section>
  </div>
  <div v-else class="repo-detail__empty">
    <p>Repository not found.</p>
    <button class="btn btn-primary btn-sm" type="button" @click="goBack">
      Return to repositories
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import RepoJobList from '@/components/RepoJobList.vue';
import RepoPollingInfo from '@/components/RepoPollingInfo.vue';
import type { Repo } from '@/types';
import { useCompassStore } from '@/composables/useCompassStore';

const route = useRoute();
const router = useRouter();

const {
  repos,
  loadRepos,
  triggerReconcile,
  deleteRepo,
  syncingRepoId,
  deletingRepoId,
} = useCompassStore();

const repoId = computed(() => {
  const raw = Number(route.params.id);
  return Number.isFinite(raw) ? raw : NaN;
});

const repo = computed<Repo | undefined>(() =>
  repos.value.find((candidate) => candidate.id === repoId.value),
);

const credentialLabel = computed(() =>
  repo.value?.credential_id ? 'Managed secret' : 'Public',
);

const isSyncing = computed(() => syncingRepoId.value === repoId.value);
const isDeleting = computed(() => deletingRepoId.value === repoId.value);

async function ensureRepoLoaded() {
  if (Number.isNaN(repoId.value)) {
    return;
  }
  if (!repos.value.length) {
    await loadRepos();
  }
  if (!repo.value) {
    await loadRepos();
  }
}

onMounted(() => {
  void ensureRepoLoaded();
});

watch(
  () => route.params.id,
  () => {
    void ensureRepoLoaded();
  },
);

function goBack() {
  router.push({ name: 'dashboard' });
}

async function handleReconcile() {
  if (!repo.value) return;
  try {
    await triggerReconcile(repo.value.id);
  } catch (err) {
    // handled globally
  }
}

async function handleDelete() {
  if (!repo.value) return;
  if (!window.confirm(`Delete repository "${repo.value.name}"?`)) {
    return;
  }
  const unschedule = window.confirm('Unschedule associated Nomad jobs?');
  try {
    await deleteRepo(repo.value.id, { unschedule });
    goBack();
  } catch (err) {
    // handled globally
  }
}
</script>

<style scoped>
.repo-detail { display: flex; flex-direction: column; gap: 1rem; }
.repo-detail__toolbar, .repo-detail__actions, .repo-detail__hero, .jobs-section-heading { display: flex; align-items: center; justify-content: space-between; gap: .75rem; }
.repo-detail__toolbar { min-height: 2rem; }
.repo-detail__actions { justify-content: flex-end; }
.detail-back { padding-inline: .25rem; color: var(--color-text-tertiary); }
.detail-back:hover { color: var(--color-accent); }
.repo-detail__hero { align-items: flex-end; gap: 2rem; padding: .5rem 0 .25rem; }
.repo-detail__identity { min-width: 0; display: flex; flex-direction: column; align-items: flex-start; gap: .25rem; }
.detail-kicker { color: var(--color-text-tertiary); font-size: .7rem; font-weight: 600; letter-spacing: .06em; text-transform: uppercase; }
.repo-detail__identity h1 { margin: 0; color: var(--color-text-primary); font-size: 1.6rem; line-height: 1.15; font-weight: 650; letter-spacing: -.02em; }
.repo-detail__location { max-width: min(58rem, 100%); overflow: hidden; color: var(--color-accent); font-family: var(--font-mono); font-size: .75rem; text-overflow: ellipsis; white-space: nowrap; }
a.repo-detail__location:hover { color: var(--color-accent-hover); }
.repo-detail__commit { min-width: min(24rem, 100%); }
.detail-tabs { min-height: 2.35rem; margin-top: .25rem; border-bottom: 1px solid var(--color-border); }
.detail-tabs .tab { min-height: 2.35rem; height: 2.35rem; padding: 0 .75rem; gap: .4rem; color: var(--color-text-tertiary); font-size: .75rem; }
.detail-tabs .tab-active { color: var(--color-accent); }
.detail-tabs .badge { font-size: .65rem; }
.repo-detail__facts { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border: 1px solid var(--color-border); background: var(--color-surface); }
.detail-fact { min-width: 0; padding: .65rem .8rem; border-right: 1px solid var(--color-border); }
.detail-fact:last-child { border-right: 0; }
.detail-fact span { display: block; margin-bottom: .2rem; color: var(--color-text-tertiary); font-size: .68rem; font-weight: 600; text-transform: uppercase; letter-spacing: .05em; }
.detail-fact strong { display: block; overflow: hidden; color: var(--color-text-secondary); font-size: .8rem; font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
.jobs-section-heading { align-items: flex-end; padding-top: .5rem; }
.jobs-section-heading h2 { margin: 0; color: var(--color-text-primary); font-size: 1rem; font-weight: 650; }
.jobs-section-heading p { margin: .15rem 0 0; color: var(--color-text-tertiary); font-size: .75rem; }
.repo-detail__jobs .repo-jobs { width: 100%; margin-top: .15rem; }
.repo-detail__empty { display: flex; flex-direction: column; align-items: flex-start; gap: .65rem; padding: 1.5rem; border: 1px dashed var(--color-border); background: var(--color-surface-muted); color: var(--color-text-secondary); }

@media (max-width: 840px) {
  .repo-detail__hero { align-items: stretch; flex-direction: column; gap: 1rem; }
  .repo-detail__commit { width: 100%; }
  .repo-detail__facts { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .detail-fact:nth-child(2) { border-right: 0; }
  .detail-fact:nth-child(-n + 2) { border-bottom: 1px solid var(--color-border); }
}
@media (max-width: 640px) {
  .repo-detail__toolbar { align-items: flex-start; flex-direction: column; }
  .repo-detail__actions { width: 100%; justify-content: flex-start; }
  .repo-detail__facts { grid-template-columns: 1fr; }
  .detail-fact, .detail-fact:nth-child(2) { border-right: 0; border-bottom: 1px solid var(--color-border); }
  .detail-fact:last-child { border-bottom: 0; }
  .detail-tabs { overflow-x: auto; }
}
</style>
