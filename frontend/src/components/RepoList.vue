<template>
  <section class="repo-table">
    <header class="repo-table__header">
      <div>
        <h1>Repositories</h1>
        <p>Git sources monitored for Nomad job specifications.</p>
      </div>
      <button class="primary add-repo-header add-button" type="button" @click="$emit('add-repo')">
        <span aria-hidden="true">＋</span>
        <span>Add repository</span>
      </button>
    </header>

    <div v-if="hasRepos" class="repo-table__surface">
      <table>
        <thead>
          <tr>
            <th scope="col">Name</th>
            <th scope="col">Branch</th>
            <th scope="col">Last Checked</th>
            <th scope="col">Status</th>
            <th scope="col" class="actions-col">Actions</th>
          </tr>
        </thead>
        <tbody>
          <RepoCard
            v-for="repo in repos"
            :key="repo.id"
            :repo="repo"
            :syncing-repo-id="syncingRepoId"
            :deleting-repo-id="deletingRepoId"
            @reconcile="emit('reconcile', $event)"
            @delete="emit('delete', $event)"
          />
        </tbody>
      </table>
    </div>
    <p v-else class="repo-empty">No repositories registered yet.</p>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import RepoCard from './RepoCard.vue';
import type { Repo } from '@/types';

const props = defineProps<{
  repos: Repo[];
  syncingRepoId: number | null;
  deletingRepoId: number | null;
}>();

const emit = defineEmits<{
  (e: 'reconcile', repo: Repo): void;
  (e: 'delete', repo: Repo): void;
  (e: 'add-repo'): void;
}>();

const hasRepos = computed(() => props.repos.length > 0);
</script>

<style scoped>
.repo-table {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.repo-table__header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
  padding-bottom: 0.1rem;
}

.add-repo-header { flex-shrink: 0; box-shadow: none; }
.add-button { line-height: 1; }

.repo-table__header h1 {
  margin: 0;
  font-size: clamp(1.35rem, 3vw, 1.65rem);
  line-height: 1.2;
  font-weight: 650;
  letter-spacing: -0.02em;
  color: var(--color-text-primary);
}

.repo-table__header p {
  margin: 0.25rem 0 0;
  color: var(--color-text-tertiary);
  font-size: 0.8125rem;
}

.repo-table__surface {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow-x: auto;
  background: var(--color-surface);
  box-shadow: var(--shadow-soft);
}

.repo-table__surface table {
  width: 100%;
}

thead th { text-align: left; }

thead th:last-child {
  border-right: none;
}

.actions-col {
  text-align: right;
}

.actions-col,
.repo-table__surface tbody tr:last-child td {
  border-bottom: none;
}

@media (max-width: 640px) {
  .repo-table__header { align-items: flex-start; flex-direction: column; }
}

.repo-empty {
  margin: 0;
  padding: 1.5rem;
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface-muted);
  color: var(--color-text-tertiary);
  text-align: center;
  font-size: 0.95rem;
}
</style>
