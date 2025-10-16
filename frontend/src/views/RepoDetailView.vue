<template>
  <div class="repo-detail" v-if="repo">
    <header class="repo-detail__header">
      <button class="ghost small" type="button" @click="goBack">
        ← Back
      </button>
      <div class="repo-detail__actions">
        <button
          class="ghost small"
          type="button"
          @click="handleReconcile"
          :disabled="isSyncing"
        >
          <span v-if="isSyncing" class="loader"></span>
          <span v-else>Sync</span>
        </button>
        <button
          class="ghost danger small"
          type="button"
          @click="handleDelete"
          :disabled="isDeleting"
        >
          <span v-if="isDeleting" class="loader"></span>
          <span v-else>Delete</span>
        </button>
      </div>
    </header>

    <section class="repo-detail__summary">
      <div class="repo-detail__identity">
        <h1>{{ repo.name }}</h1>
        <div class="repo-detail__location">
          <a
            v-if="repo.repo_url"
            :href="repo.repo_url"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ repo.repo_url }}
          </a>
          <span v-else>—</span>
        </div>
      </div>
      <RepoPollingInfo class="repo-detail__commit" :repo="repo" />
    </section>

    <section class="repo-detail__meta">
      <dl>
        <div>
          <dt>Credential</dt>
          <dd>{{ credentialLabel }}</dd>
        </div>
        <div>
          <dt>Namespace</dt>
          <dd>{{ repo.nomad_namespace || '—' }}</dd>
        </div>
        <div>
          <dt>Job path</dt>
          <dd>{{ repo.job_path || '—' }}</dd>
        </div>
      </dl>
    </section>

    <section class="repo-detail__jobs">
      <RepoJobList :jobs="repo.jobs || []" :enable-collapse="false" :show-header="false" />
    </section>
  </div>
  <div v-else class="repo-detail__empty">
    <p>Repository not found.</p>
    <button class="primary small" type="button" @click="goBack">
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
.repo-detail {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.repo-detail__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.repo-detail__actions {
  display: flex;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.repo-detail__summary {
  display: flex;
  justify-content: space-between;
  gap: 1.5rem;
  align-items: center;
  flex-wrap: wrap;
}

.repo-detail__identity {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  min-width: 220px;
}

.repo-detail__identity h1 {
  margin: 0;
  font-size: 1.45rem;
  font-weight: 600;
  color: var(--color-text-primary);
}

.repo-detail__location a {
  color: var(--color-accent);
}

.repo-detail__location a:hover,
.repo-detail__location a:focus-visible {
  color: var(--color-accent-hover);
}

.repo-detail__meta {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  padding: 0.75rem 1rem;
  margin: 0;
}

.repo-detail__meta dl {
  margin: 0;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 0.75rem 1.5rem;
}

.repo-detail__meta div {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.repo-detail__meta dt {
  margin: 0;
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
}

.repo-detail__meta dd {
  margin: 0;
  font-size: 0.86rem;
  color: var(--color-text-secondary);
}

.repo-detail__commit {
  min-width: 260px;
}

.repo-detail__jobs {
  margin-top: 1rem;
}

.repo-detail__jobs .repo-jobs {
  width: 100%;
}

.repo-detail__empty {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.8rem;
  padding: 2rem 1.5rem;
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface-muted);
  color: var(--color-text-secondary);
}

@media (max-width: 840px) {
  .repo-detail__summary {
    flex-direction: column;
    align-items: stretch;
  }

  .repo-detail__commit {
    width: 100%;
  }
}

@media (max-width: 640px) {
  .repo-detail__header {
    flex-direction: column;
    align-items: stretch;
  }

  .repo-detail__actions {
    justify-content: flex-start;
  }
}
</style>
