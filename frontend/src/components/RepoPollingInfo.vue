<template>
  <section class="repo-status">
    <div class="commit-row">
      <div class="commit-text">
        <p class="commit-message">
          {{ repo.last_commit_title || 'No commits reconciled yet.' }}
        </p>
        <div class="commit-meta">
          <span class="author-name">{{ repo.last_commit_author || 'Unknown author' }}</span>
          <span class="commit-action">committed</span>
          <template v-if="repo.last_polled_at">
            <time
              class="commit-polled"
              :datetime="repo.last_polled_at"
              :title="formatTimestamp(repo.last_polled_at)"
            >
              {{ formatRelativeTime(repo.last_polled_at) }}
            </time>
          </template>
          <span v-else class="commit-polled pending">Awaiting first poll</span>
        </div>
      </div>

      <code
        v-if="repo.last_commit"
        class="commit-hash"
        :title="repo.last_commit"
      >
        {{ commitDisplay(repo.last_commit) }}
      </code>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { Repo } from '../composables/useCompassStore';

defineProps<{
  repo: Repo;
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

function formatRelativeTime(value?: string | null) {
  if (!value) return 'Awaiting first poll';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  const now = new Date();
  const diff = date.getTime() - now.getTime();
  const absDiff = Math.abs(diff);
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;
  const week = 7 * day;
  const month = 30 * day;
  const year = 365 * day;

  const thresholds = [
    { limit: year, unit: 'year', value: year },
    { limit: month, unit: 'month', value: month },
    { limit: week, unit: 'week', value: week },
    { limit: day, unit: 'day', value: day },
    { limit: hour, unit: 'hour', value: hour },
    { limit: minute, unit: 'minute', value: minute },
  ] as const;

  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });

  for (const threshold of thresholds) {
    if (absDiff >= threshold.limit) {
      const amount = Math.round(diff / threshold.value);
      return rtf.format(amount, threshold.unit);
    }
  }

  const seconds = Math.round(diff / 1000);
  if (Math.abs(seconds) < 1) {
    return 'just now';
  }
  return rtf.format(seconds, 'second');
}
</script>

<style scoped>
.repo-status {
  margin-top: 1rem;
  padding: 0.8rem 0.9rem;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.45);
  border: 1px solid rgba(71, 85, 105, 0.35);
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
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
