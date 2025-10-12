<template>
  <section class="repo-table">
    <header class="repo-table__header">
      <div>
        <h2>Repositories</h2>
        <p>Git sources monitored for Nomad job definitions.</p>
      </div>
      <span class="repo-table__count">{{ repos.length }} total</span>
    </header>

    <div v-if="repos.length" class="repo-table__surface">
      <table>
        <thead>
          <tr>
            <th scope="col" />
            <th scope="col">Name</th>
            <th scope="col">Repository</th>
            <th scope="col">Branch</th>
            <th scope="col">Credential</th>
            <th scope="col">Last Checked</th>
            <th scope="col">Jobs</th>
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
import RepoCard from './RepoCard.vue';
import type { Repo } from '@/types';

defineProps<{
  repos: Repo[];
  syncingRepoId: number | null;
  deletingRepoId: number | null;
}>();

const emit = defineEmits<{
  (e: 'reconcile', repo: Repo): void;
  (e: 'delete', repo: Repo): void;
}>();
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

.repo-table__count {
  font-size: 0.82rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--color-text-subtle);
}

.repo-table__surface {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
  background: var(--color-surface);
}

.repo-table__surface table {
  width: 100%;
}

.repo-table__surface thead th:first-child,
.repo-table__surface tbody td:first-child {
  width: 40px;
  padding-left: 0.6rem;
  padding-right: 0.3rem;
}

.repo-table__surface tbody tr:last-child td {
  border-bottom: none;
}

.actions-col {
  text-align: right;
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
