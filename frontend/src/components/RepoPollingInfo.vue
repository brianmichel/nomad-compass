<template>
  <section class="repo-status">
    <div class="repo-status__header">
      <span class="repo-status__label">Last commit</span>
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
        class="commit-hash commit-hash--static"
        :title="repo.last_commit || 'Awaiting first run'"
      >
        {{ commitHash }}
      </code>
      <span v-else class="commit-empty">—</span>
    </div>
    <div class="repo-status__details">
      <span class="commit-message">
        {{ repo.last_commit_title || 'No commits reconciled yet.' }}
      </span>
      <span class="commit-meta">
        {{ repo.last_commit_author || 'Unknown author' }}
      </span>
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
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  padding: 0.5rem 0.75rem;
  display: grid;
  row-gap: 0.4rem;
}

.repo-status__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.repo-status__label {
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
  font-weight: 600;
}

.repo-status__details {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.commit-message {
  display: inline-flex;
  align-items: center;
  font-weight: 600;
  color: var(--color-text-primary);
  font-size: 0.85rem;
}

.commit-meta {
  color: var(--color-text-subtle);
  font-size: 0.8rem;
}

.commit-hash {
  font-family: var(--font-mono);
  padding: 0.14rem 0.4rem;
  border: 1px solid var(--color-border);
  border-radius: 4px;
  letter-spacing: 0.05em;
  color: var(--color-text-secondary);
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  background: var(--color-surface-muted);
  transition: background-color var(--transition-fast), border-color var(--transition-fast), color var(--transition-fast);
  flex-shrink: 0;
}

.commit-hash:hover,
.commit-hash:focus-visible {
  border-color: var(--color-accent);
  color: var(--color-accent);
  background: rgba(59, 130, 246, 0.08);
}

.commit-hash--static {
  pointer-events: none;
  background: var(--color-surface);
}

.commit-empty {
  color: var(--color-text-tertiary);
  font-size: 0.8rem;
  flex-shrink: 0;
}
</style>
