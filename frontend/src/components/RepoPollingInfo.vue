<template>
  <section class="repo-status">
    <div class="commit-row">
      <div class="commit-text">
        <p class="commit-message">
          {{ repo.last_commit_title || 'No commits reconciled yet.' }}
        </p>
        <div class="commit-meta">
          <span class="author-name">{{ repo.last_commit_author || 'Unknown author' }}</span>
        </div>
      </div>

      <a
        v-if="repo.last_commit && commitLink"
        class="commit-hash"
        :href="commitLink"
        target="_blank"
        rel="noopener noreferrer"
        :title="`View commit ${repo.last_commit}`"
      >
        {{ commitHash }}
      </a>
      <code
        v-else-if="repo.last_commit"
        class="commit-hash"
        :title="repo.last_commit || 'Awaiting first run'"
      >
        {{ commitHash }}
      </code>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { Repo } from '@/types';
import { buildCommitUrl, formatCommitHash } from '@/utils/repos';

const props = defineProps<{ repo: Repo }>();

const commitLink = computed(() => buildCommitUrl(props.repo));
const commitHash = computed(() => formatCommitHash(props.repo.last_commit));
</script>

<style scoped>
.repo-status {
  margin: 0;
  padding: 0.75rem 0.85rem;
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.commit-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
}

.commit-text {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  flex: 1;
  min-width: 200px;
}

.commit-message {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 500;
  color: rgba(226, 232, 240, 0.9);
}

.commit-meta {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  align-items: center;
  font-size: 0.8rem;
  color: rgba(148, 163, 184, 0.8);
}

.commit-hash {
  font-family: 'JetBrains Mono', 'SFMono-Regular', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  background: rgba(30, 41, 59, 0.6);
  border-radius: 6px;
  padding: 0.15rem 0.45rem;
  border: 1px solid rgba(148, 163, 184, 0.3);
  color: rgba(226, 232, 240, 0.85);
  letter-spacing: 0.05em;
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  text-decoration: none;
  transition: border-color 0.15s ease, color 0.15s ease;
}

.commit-hash:hover,
.commit-hash:focus-visible {
  border-color: rgba(148, 163, 184, 0.5);
  color: rgba(226, 232, 240, 0.95);
}

.author-name {
  color: rgba(226, 232, 240, 0.85);
  font-weight: 500;
}

.commit-action {
  color: rgba(148, 163, 184, 0.7);
}

.commit-polled {
  color: rgba(148, 163, 184, 0.7);
}

.commit-polled.pending {
  color: rgba(148, 163, 184, 0.6);
}

@media (max-width: 768px) {
  .repo-status {
    padding: 0.75rem 0.8rem;
  }
}
</style>
