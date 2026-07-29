<template>
  <nav class="topbar" aria-label="Primary navigation">
    <div class="topbar__inner">
      <RouterLink to="/" class="brand" aria-label="Compass home">
        <span class="brand-accent" aria-hidden="true"></span>
        <span class="brand-title">Compass</span>
      </RouterLink>
      <div class="topbar-nav">
        <RouterLink to="/" class="nav-link" active-class="active" exact-active-class="active">Dashboard</RouterLink>
        <RouterLink to="/settings" class="nav-link" active-class="active">Settings</RouterLink>
      </div>
    </div>
  </nav>
</template>

<style scoped>
.topbar {
  width: 100%;
  min-height: 3.25rem;
  background: linear-gradient(90deg, var(--color-brand-darker), var(--color-brand-dark));
  color: #fff;
  border-bottom: 1px solid rgba(22, 112, 77, 0.45);
}

.topbar__inner {
  max-width: 1200px;
  min-height: 3.25rem;
  margin: 0 auto;
  padding: 0 clamp(1rem, 3vw, 1.5rem);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  color: #fff;
  text-decoration: none;
}

.brand:hover,
.brand:focus-visible { color: #fff; text-decoration: none; }

.brand-accent { width: 0.22rem; height: 1.35rem; border-radius: 1px; background: rgba(255, 255, 255, 0.9); }
.brand-title { font-size: 0.95rem; font-weight: 700; letter-spacing: 0.01em; }
.topbar-nav { display: flex; align-items: center; gap: 0.35rem; }
.nav-link {
  position: relative;
  padding: 0.45rem 0.65rem;
  color: rgba(255, 255, 255, 0.78);
  font-size: 0.8125rem;
  font-weight: 600;
  text-decoration: none;
}
.nav-link:hover { color: #fff; background: rgba(22, 112, 77, 0.32); text-decoration: none; }
.nav-link.active { color: #fff; }
.nav-link.active::after { content: ""; position: absolute; right: 0.75rem; bottom: -0.95rem; left: 0.75rem; height: 3px; background: #fff; }

@media (max-width: 640px) {
  .topbar__inner { padding-inline: 1rem; }
  .brand-title { font-size: 1rem; }
  .nav-link { padding-inline: 0.55rem; }
  .nav-link.active::after { right: 0.55rem; left: 0.55rem; }
}
</style>
