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
  padding: 0 0 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  border-bottom: 1px solid var(--color-border-soft);
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
  gap: 0.3rem;
  flex: 1;
  min-width: 200px;
}

.commit-message {
  margin: 0;
  font-size: 0.92rem;
  font-weight: 600;
  color: var(--color-text-primary);
}

.commit-meta {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  align-items: center;
  font-size: 0.78rem;
  color: var(--color-text-subtle);
}

.commit-hash {
  font-family: var(--font-mono);
  background: var(--color-surface-muted);
  border-radius: var(--radius-sm);
  padding: 0.18rem 0.5rem;
  border: 1px solid var(--color-border);
  color: var(--color-text-secondary);
  letter-spacing: 0.05em;
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  text-decoration: none;
  transition: border-color var(--transition-fast), color var(--transition-fast);
}

.commit-hash:hover,
.commit-hash:focus-visible {
  border-color: var(--color-accent);
  color: var(--color-accent);
}

.author-name {
  color: var(--color-text-secondary);
  font-weight: 500;
}

@media (max-width: 768px) {
  .repo-status {
    padding-bottom: 0.65rem;
  }
}

.repo-status:last-child {
  border-bottom: none;
}
</style>
