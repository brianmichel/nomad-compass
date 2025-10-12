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
  background: linear-gradient(118deg, var(--color-brand-dark), var(--color-brand));
  color: #ffffff;
  box-shadow: 0 18px 40px -32px rgba(15, 23, 42, 0.6);
}

.topbar__inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 1rem clamp(1rem, 4vw, 2.5rem);
  display: flex;
  align-items: center;
  gap: 1.5rem;
}

.brand {
  display: flex;
  align-items: center;
  gap: 0.9rem;
  flex-shrink: 0;
}

.brand-icon {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.18);
  border: 1px solid rgba(255, 255, 255, 0.28);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.35);
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
  font-size: 1.18rem;
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
  font-size: 0.9rem;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.78);
  padding: 0.45rem 0.85rem;
  border-radius: var(--radius-pill);
  transition: background-color var(--transition-base), color var(--transition-base), box-shadow var(--transition-base);
}

.nav-link:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.16);
}

.nav-link.active {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.18);
  box-shadow: inset 0 -3px 0 rgba(29, 111, 228, 0.8);
}

.add-repo-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.topbar-actions {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-left: 1.5rem;
  flex-shrink: 0;
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.82rem;
  padding: 0.35rem 0.75rem;
  border-radius: var(--radius-pill);
  background: rgba(255, 255, 255, 0.14);
  border: 1px solid rgba(255, 255, 255, 0.28);
  color: rgba(255, 255, 255, 0.92);
}

.status-badge.offline {
  background: rgba(217, 45, 32, 0.18);
  border-color: rgba(217, 45, 32, 0.4);
  color: #ffffff;
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
