<template>
  <section class="card card-border card-sm revision-card" aria-labelledby="revision-heading">
    <div class="card-body revision-body">
      <IconGitCommit class="revision-symbol" aria-hidden="true" />
      <div class="revision-main">
        <span id="revision-heading" class="revision-label">Latest revision</span>
        <span class="commit-message">{{ repo.last_commit_title || 'No commits reconciled yet' }}</span>
      </div>
      <div class="revision-meta">
        <a
          v-if="repo.last_commit && commitLink"
          class="badge badge-outline commit-hash"
          :href="commitLink"
          target="_blank"
          rel="noopener noreferrer"
          :title="`View commit ${repo.last_commit}`"
        >
          {{ commitHash }}
          <IconExternalLink aria-hidden="true" />
        </a>
        <code v-else-if="repo.last_commit" class="badge badge-outline commit-hash commit-hash--static" :title="repo.last_commit">
          {{ commitHash }}
        </code>
        <span class="commit-author" :title="repo.last_commit_author || undefined">{{ authorName }}</span>
        <span class="poll-freshness" :title="polledAtTitle">
          {{ hasPolled ? `Checked ${polledAtRelative}` : 'Awaiting first check' }}
        </span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { IconExternalLink, IconGitCommit } from '@tabler/icons-vue';
import type { Repo } from '@/types';
import { formatRelativeTime, formatTimestamp } from '@/utils/date';
import { buildCommitUrl, formatCommitHash } from '@/utils/repos';

const props = defineProps<{ repo: Repo }>();
const commitLink = computed(() => buildCommitUrl(props.repo));
const commitHash = computed(() => formatCommitHash(props.repo.last_commit));
const authorName = computed(() => {
  const author = props.repo.last_commit_author?.trim();
  if (!author) return 'Unknown author';
  return author.replace(/\s*<[^>]+>\s*$/, '');
});
const hasPolled = computed(() => Boolean(props.repo.last_polled_at));
const polledAtRelative = computed(() => formatRelativeTime(props.repo.last_polled_at));
const polledAtTitle = computed(() => hasPolled.value
  ? `Last checked ${formatTimestamp(props.repo.last_polled_at)}`
  : 'Repository has not been checked yet');
</script>

<style scoped>
.revision-card { border-color: var(--color-border); border-radius: var(--radius-box); background: var(--color-surface); box-shadow: none; }
.revision-body { display: grid; grid-template-columns: auto minmax(12rem, 1fr) auto; align-items: center; gap: .75rem; min-width: 0; padding: .72rem .85rem; }
.revision-symbol { width: 1.15rem; color: var(--color-text-tertiary); }
.revision-main { display: flex; flex-direction: column; min-width: 0; gap: .08rem; }
.revision-label { color: var(--color-text-tertiary); font-size: .61rem; font-weight: 650; letter-spacing: .08em; text-transform: uppercase; }
.commit-message { overflow: hidden; color: var(--color-text-primary); font-size: .8rem; font-weight: 620; text-overflow: ellipsis; white-space: nowrap; }
.revision-meta { display: flex; align-items: center; gap: .65rem; min-width: 0; }
.commit-hash { gap: .25rem; min-height: 1.45rem; border-color: var(--color-border); color: var(--color-accent); font-family: var(--font-mono); font-size: .67rem; letter-spacing: .025em; text-decoration: none; }
.commit-hash:hover { border-color: var(--color-accent); text-decoration: none; }
.commit-hash svg { width: .66rem; }
.commit-hash--static { color: var(--color-text-secondary); }
.commit-author { max-width: 10rem; overflow: hidden; color: var(--color-text-tertiary); font-size: .7rem; text-overflow: ellipsis; white-space: nowrap; }
.poll-freshness { padding-left: .75rem; border-left: 1px solid var(--color-border-soft); color: var(--color-text-tertiary); font-size: .68rem; white-space: nowrap; }
@media (max-width: 760px) { .revision-body { grid-template-columns: auto minmax(0, 1fr); } .revision-meta { grid-column: 2; flex-wrap: wrap; gap: .35rem .6rem; } }
@media (max-width: 480px) { .poll-freshness { width: 100%; padding: .2rem 0 0; border: 0; } }
</style>
