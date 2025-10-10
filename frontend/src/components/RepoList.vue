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
      <RepoCard
        v-for="repo in repos"
        :key="repo.id"
        :repo="repo"
        :syncing-repo-id="syncingRepoId"
        :deleting-repo-id="deletingRepoId"
        @reconcile="emit('reconcile', $event)"
        @delete="emit('delete', $event)"
      />
    </ul>
  </section>
</template>

<script setup lang="ts">
import RepoCard from './RepoCard.vue';
import type { Repo } from '../composables/useCompassStore';

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
