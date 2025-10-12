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
  padding: 0.85rem 1rem;
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
  color: var(--color-text-primary);
}

.commit-meta {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  align-items: center;
  font-size: 0.8rem;
  color: var(--color-text-subtle);
}

.commit-hash {
  font-family: 'JetBrains Mono', 'SFMono-Regular', ui-monospace, Menlo, Consolas, monospace;
  background: var(--color-surface);
  border-radius: var(--radius-sm);
  padding: 0.2rem 0.55rem;
  border: 1px solid var(--color-border);
  color: var(--color-text-secondary);
  letter-spacing: 0.05em;
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  text-decoration: none;
  transition: border-color var(--transition-fast), color var(--transition-fast), box-shadow var(--transition-fast);
}

.commit-hash:hover,
.commit-hash:focus-visible {
  border-color: var(--color-accent);
  color: var(--color-accent);
  box-shadow: 0 0 0 3px rgba(29, 111, 228, 0.12);
}

.author-name {
  color: var(--color-text-secondary);
  font-weight: 500;
}

.commit-action {
  color: var(--color-text-subtle);
}

@media (max-width: 768px) {
  .repo-status {
    padding: 0.75rem 0.85rem;
  }
}
</style>
