<template>
  <nav class="topbar">
    <div class="brand">
      <div class="brand-icon">🧭</div>
      <div class="brand-copy">
        <span class="brand-title">Nomad Compass</span>
        <span class="brand-subtitle">GitOps recon supervisor</span>
      </div>
    </div>
    <div class="topbar-nav">
      <RouterLink to="/" class="nav-link" active-class="active">Dashboard</RouterLink>
      <RouterLink to="/settings" class="nav-link" active-class="active">Settings</RouterLink>
    </div>
    <div class="topbar-actions">
      <span class="status-badge">
        <span class="pulse"></span>
        Active
      </span>
      <button class="ghost" type="button" @click="$emit('refresh')" :disabled="refreshing">
        <span class="button-icon">
          <span v-if="refreshing" class="loader"></span>
          <span v-else class="refresh-icon">⟳</span>
        </span>
        <span>Refresh</span>
      </button>
    </div>
  </nav>
</template>

<script setup lang="ts">
defineProps<{ refreshing: boolean }>();
</script>

<style scoped>
.topbar-nav {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.nav-link {
  font-size: 0.95rem;
  font-weight: 500;
  color: rgba(226, 232, 240, 0.75);
  padding: 0.35rem 0.75rem;
  border-radius: 999px;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.nav-link:hover {
  color: rgba(226, 232, 240, 0.95);
  background: rgba(59, 130, 246, 0.18);
}

.nav-link.active {
  color: #c7d2fe;
  background: rgba(99, 102, 241, 0.25);
  border: 1px solid rgba(129, 140, 248, 0.35);
}
</style>
