<template>
  <span
    class="status-badge"
    :class="[{ offline: !connected }, variantClass]"
    :title="computedTooltip"
  >
    <span class="pulse" :class="{ offline: !connected }"></span>
    {{ computedLabel }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  connected: boolean;
  message?: string | null;
  variant?: 'default' | 'footer';
}>();

const computedLabel = computed(() => (props.connected ? 'Nomad Connected' : 'Nomad Offline'));

const computedTooltip = computed(() => {
  if (props.message) {
    return props.message;
  }
  return props.connected ? 'Connected to Nomad' : 'Unable to reach Nomad';
});

const variantClass = computed(() => (props.variant === 'footer' ? 'status-badge--footer' : null));
</script>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.32rem;
  font-size: 0.76rem;
  padding: 0.24rem 0.5rem;
  border-radius: var(--radius-pill);
  background: rgba(255, 255, 255, 0.16);
  border: 1px solid rgba(255, 255, 255, 0.22);
  color: rgba(255, 255, 255, 0.95);
}

.status-badge.offline {
  background: rgba(217, 45, 32, 0.18);
  border-color: rgba(217, 45, 32, 0.4);
}

.status-badge--footer {
  background: var(--color-surface);
  border-color: var(--color-border);
  color: var(--color-text-secondary);
}

.status-badge--footer.offline {
  background: var(--color-danger-bg);
  border-color: var(--color-danger-border);
  color: var(--color-danger);
}
</style>
