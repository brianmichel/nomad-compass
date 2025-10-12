<template>
  <nav class="topbar">
    <div class="topbar__inner">
      <div class="brand">
        <div class="brand-icon">
          <span class="brand-glyph">🧭</span>
        </div>
        <div class="brand-copy">
          <span class="brand-title">Compass</span>
          <span class="brand-subtitle">Nomad GitOps</span>
        </div>
      </div>
      <div class="topbar-nav">
        <RouterLink to="/" class="nav-link" active-class="active">Dashboard</RouterLink>
        <RouterLink to="/settings" class="nav-link" active-class="active">Settings</RouterLink>
      </div>
      <div class="topbar-actions">
        <span
          class="status-badge"
          :class="{ offline: !isConnected }"
          :title="statusTooltip"
        >
          <span class="pulse" :class="{ offline: !isConnected }"></span>
          {{ statusLabel }}
        </span>
        <button class="primary add-repo-btn" type="button" @click="$emit('add-repo')">
          <span class="button-icon">＋</span>
          <span>Add repo</span>
        </button>
      </div>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { CompassStatus } from '@/types';

const props = defineProps<{
  status: CompassStatus | null;
}>();

defineEmits<{
  (e: 'add-repo'): void;
}>();

const isConnected = computed(() => props.status?.nomad_connected ?? false);

const statusLabel = computed(() => (isConnected.value ? 'Nomad Connected' : 'Nomad Offline'));

const statusTooltip = computed(() => {
  if (props.status?.nomad_message) {
    return props.status.nomad_message;
  }
  return isConnected.value ? 'Connected to Nomad' : 'Unable to reach Nomad';
});
</script>

<style scoped>
.topbar {
  width: 100%;
  background: var(--color-brand);
  color: #ffffff;
  border-bottom: 1px solid rgba(0, 0, 0, 0.08);
}

.topbar__inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0.75rem clamp(1rem, 3vw, 2.25rem);
  display: flex;
  align-items: center;
  gap: 1.25rem;
}

.brand {
  display: flex;
  align-items: center;
  gap: 0.9rem;
  flex-shrink: 0;
}

.brand-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.12);
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.brand-glyph {
  font-size: 1.35rem;
}

.brand-copy {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
}

.brand-title {
  font-weight: 600;
  font-size: 1.08rem;
  letter-spacing: 0.01em;
}

.brand-subtitle {
  font-size: 0.82rem;
  color: rgba(255, 255, 255, 0.7);
  text-transform: uppercase;
  letter-spacing: 0.12em;
}

.topbar-nav {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-left: auto;
}

.nav-link {
  font-size: 0.88rem;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.8);
  padding: 0.35rem 0.6rem;
  border-radius: var(--radius-sm);
  transition: color var(--transition-base);
}

.nav-link:hover {
  color: #ffffff;
}

.nav-link.active {
  color: #ffffff;
  box-shadow: inset 0 -2px 0 rgba(255, 255, 255, 0.85);
}

.add-repo-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.topbar-actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-left: 1.25rem;
  flex-shrink: 0;
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.8rem;
  padding: 0.3rem 0.6rem;
  border-radius: var(--radius-pill);
  background: rgba(255, 255, 255, 0.18);
  border: 1px solid rgba(255, 255, 255, 0.24);
  color: rgba(255, 255, 255, 0.95);
}

.status-badge.offline {
  background: rgba(217, 45, 32, 0.2);
  border-color: rgba(217, 45, 32, 0.45);
}

@media (max-width: 960px) {
  .topbar__inner {
    flex-wrap: wrap;
    justify-content: space-between;
  }

  .topbar-nav {
    order: 3;
    width: 100%;
    justify-content: flex-start;
  }

  .topbar-actions {
    margin-left: 0;
  }
}

@media (max-width: 640px) {
  .topbar__inner {
    flex-direction: column;
    align-items: stretch;
  }

  .topbar-nav {
    justify-content: flex-start;
  }

  .topbar-actions {
    justify-content: space-between;
  }
}
</style>
