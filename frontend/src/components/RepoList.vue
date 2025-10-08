<template>
  <section class="panel glass repo-panel">
    <header class="panel-header">
      <div>
        <h2>Tracked repositories</h2>
        <p v-if="repos.length">Showing {{ repos.length }} sources.</p>
        <p v-else>Onboard your first repository to start reconciling jobs.</p>
      </div>
    </header>

    <ul v-if="repos.length" class="repo-list">
      <li
        v-for="repo in repos"
        :key="repo.id"
        class="repo-card"
        :class="{ active: syncingRepoId === repo.id }"
      >
        <div class="repo-header">
          <div>
            <span class="repo-name">{{ repo.name }}</span>
            <span class="repo-branch">{{ repo.branch }}</span>
          </div>
          <div class="repo-actions">
            <button class="ghost small" type="button" @click="$emit('reconcile', repo)" :disabled="syncingRepoId === repo.id">
              <span v-if="syncingRepoId === repo.id" class="loader"></span>
              <span v-else>Sync now</span>
            </button>
            <button
              class="ghost danger small"
              type="button"
              @click="$emit('delete', repo)"
              :disabled="deletingRepoId === repo.id"
            >
              <span v-if="deletingRepoId === repo.id" class="loader"></span>
              <span v-else>Delete</span>
            </button>
          </div>
        </div>

        <div class="repo-meta">
          <span class="meta-pill" :title="repo.repo_url">
            <span class="label">Source</span>
            {{ repo.repo_url }}
          </span>
          <span class="meta-pill">
            <span class="label">Credential</span>
            {{ repo.credential_id ? 'Managed secret' : 'Public' }}
          </span>
        </div>

        <div class="repo-commit">
          <span class="commit-hash" :title="repo.last_commit || 'Awaiting first run'">
            #{{ commitDisplay(repo.last_commit) }}
          </span>
          <span v-if="repo.last_commit_title" class="commit-title">{{ repo.last_commit_title }}</span>
          <span v-else class="commit-title muted">No commits reconciled yet.</span>
        </div>

        <div class="repo-footer">
          <span>
            <strong>Author:</strong>
            <span>{{ repo.last_commit_author || '—' }}</span>
          </span>
          <span>
            <strong>Last polled:</strong>
            <span>{{ formatTimestamp(repo.last_polled_at) }}</span>
          </span>
        </div>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import type { Repo } from '../composables/useCompassStore';

defineProps<{
  repos: Repo[];
  syncingRepoId: number | null;
  deletingRepoId: number | null;
}>();

defineEmits<{
  (e: 'reconcile', repo: Repo): void;
  (e: 'delete', repo: Repo): void;
}>();

function commitDisplay(commit?: string | null) {
  if (!commit) return 'pending';
  return commit.slice(0, 7);
}

function formatTimestamp(value?: string | null) {
  if (!value) return 'Awaiting first poll';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}
</script>
