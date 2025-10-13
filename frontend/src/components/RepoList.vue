<template>
  <section class="repo-table">
    <header class="repo-table__header">
      <div>
        <h2>Repositories</h2>
        <p>Git sources monitored for Nomad job definitions.</p>
      </div>
      <button class="primary add-repo-header" type="button" @click="$emit('add-repo')">
        <span>Add</span>
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

defineEmits<{
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
  gap: 1rem;
}

.repo-table__header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
}

.add-repo-header {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-weight: 600;
  font-size: 0.9rem;
  padding: 0.45rem 1rem;
  border-radius: var(--radius-md);
  box-shadow: none;
}

.repo-table__header h2 {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--color-text-primary);
}

.repo-table__header p {
  margin: 0.2rem 0 0;
  color: var(--color-text-tertiary);
  font-size: 0.92rem;
}

.repo-table__surface {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md) var(--radius-md);
  overflow: hidden;
  background: var(--color-surface);
}

.repo-table__surface table {
  width: 100%;
}

thead th {
  text-align: left;
  color: var(--color-text-secondary);
  background-color: #f1f2f3;
  border-right: 1px solid var(--color-border);
}

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
