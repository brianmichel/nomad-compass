<template>
  <header class="app-header">
    <nav class="navbar topbar" aria-label="Primary navigation">
      <div class="navbar-start">
        <RouterLink to="/" class="brand" aria-label="Compass home">
          <IconCompass aria-hidden="true" />
          <span>Compass</span>
        </RouterLink>
      </div>
      <div class="navbar-end">
        <ul class="menu menu-horizontal topbar-menu">
          <li>
            <RouterLink to="/" exact-active-class="menu-active">
              <IconLayoutDashboard aria-hidden="true" />
              <span>Repositories</span>
            </RouterLink>
          </li>
          <li>
            <RouterLink to="/settings" active-class="menu-active">
              <IconSettings aria-hidden="true" />
              <span>Settings</span>
            </RouterLink>
          </li>
        </ul>
        <button
          class="btn btn-ghost btn-sm btn-square theme-toggle tooltip tooltip-left"
          type="button"
          :aria-label="themeActionLabel"
          :data-tip="themeActionLabel"
          @click="toggleTheme"
        >
          <IconMoon v-if="activeTheme === 'light'" aria-hidden="true" />
          <IconSun v-else aria-hidden="true" />
        </button>
      </div>
    </nav>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { IconCompass, IconLayoutDashboard, IconMoon, IconSettings, IconSun } from '@tabler/icons-vue';
import { useTheme } from '@/composables/useTheme';

const { activeTheme, toggleTheme } = useTheme();
const themeActionLabel = computed(() => activeTheme.value === 'light' ? 'Use dark theme' : 'Use light theme');
</script>

<style scoped>
.app-header { border-bottom: 1px solid var(--color-border); background: var(--color-surface); }
.topbar { width: 100%; max-width: 1200px; min-height: 3.65rem; margin: 0 auto; padding: .45rem clamp(1rem, 3vw, 2rem); }
.brand { display: inline-flex; align-items: center; gap: .55rem; color: var(--color-text-primary); font-size: .96rem; font-weight: 720; letter-spacing: -.015em; text-decoration: none; }
.brand:hover { color: var(--color-text-primary); text-decoration: none; }
.brand svg { width: 1.25rem; color: var(--color-brand-dark); stroke-width: 2; }
.navbar-end { gap: .4rem; }
.topbar-menu { gap: .2rem; padding: 0; }
.topbar-menu a { min-height: 2.25rem; gap: .4rem; border-radius: var(--radius-field); color: var(--color-text-tertiary); font-size: .78rem; font-weight: 600; }
.topbar-menu a:hover { background: var(--color-surface-muted); color: var(--color-text-primary); text-decoration: none; }
.topbar-menu a.menu-active { background: var(--color-brand-light); color: var(--color-brand-darker); }
.topbar-menu svg { width: .9rem; }
.theme-toggle { border: 1px solid var(--color-border-soft); color: var(--color-text-secondary); }
.theme-toggle:hover { border-color: var(--color-border); background: var(--color-surface-muted); color: var(--color-text-primary); }
.theme-toggle svg { width: .95rem; }
@media (max-width: 520px) { .topbar-menu span { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0, 0, 0, 0); } .topbar-menu a { padding-inline: .65rem; } }
</style>
