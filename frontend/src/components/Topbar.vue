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
    </div>
  </nav>
</template>

<script setup lang="ts">
</script>

<style scoped>
.topbar {
  width: 100%;
  min-height: 3.25rem;
  background: var(--color-brand);
  color: #ffffff;
  border-bottom: 1px solid rgba(0, 0, 0, 0.08);
  display: flex;
  align-items: center;
}

.topbar__inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0.4rem clamp(1rem, 3vw, 2rem);
  display: flex;
  align-items: center;
  gap: 1.1rem;
  width: 100%;
}

.brand {
  display: flex;
  align-items: center;
  gap: 0.9rem;
  flex-shrink: 0;
}

.brand-icon {
  width: 32px;
  height: 32px;
  border-radius: 10px;
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
  font-size: 0.85rem;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.82);
  padding: 0.24rem 0.5rem;
  border-radius: var(--radius-pill);
  transition: color var(--transition-base), background var(--transition-base);
}

.nav-link:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.12);
}

.nav-link.active {
  color: #ffffff;
  background: rgba(15, 108, 67, 0.48);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.22);
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

}

@media (max-width: 640px) {
  .topbar__inner {
    flex-direction: column;
    align-items: stretch;
  }

  .topbar-nav {
    justify-content: flex-start;
  }
}
</style>
