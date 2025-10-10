<template>
  <div class="repo-jobs">
    <template v-if="jobs.length">
      <RepoJob
        v-for="job in jobs"
        :key="job.path"
        :job="job"
      />
    </template>
    <p v-else class="job-empty">No Nomad jobs registered yet.</p>
  </div>
</template>

<script setup lang="ts">
import RepoJob from './RepoJob.vue';
import type { RepoJob as RepoJobType } from '../composables/useCompassStore';

defineProps<{
  jobs: RepoJobType[];
}>();
</script>

<style scoped>
.repo-jobs {
  margin-top: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.job-empty {
  margin-top: 1rem;
  font-size: 0.9rem;
  color: rgba(148, 163, 184, 0.85);
}
</style>
