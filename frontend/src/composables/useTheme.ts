import { readonly, ref } from 'vue';

/** User-selectable application color modes. */
export type ThemeMode = 'light' | 'dark';

const STORAGE_KEY = 'compass-theme';
const THEME_NAMES: Readonly<Record<ThemeMode, string>> = {
  light: 'nomad-mint',
  dark: 'nomad-mint-dark',
};

const activeTheme = ref<ThemeMode>('light');
let initialized = false;

/** Initializes the application theme from a saved preference or the operating-system preference. */
export function initializeTheme(): void {
  if (initialized || typeof window === 'undefined' || typeof document === 'undefined') return;
  initialized = true;

  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
  activeTheme.value = readStoredTheme() ?? (mediaQuery.matches ? 'dark' : 'light');
  applyTheme(activeTheme.value);

  mediaQuery.addEventListener('change', (event) => {
    if (readStoredTheme() !== null) return;
    activeTheme.value = event.matches ? 'dark' : 'light';
    applyTheme(activeTheme.value);
  });
}

/** Exposes the active theme and an explicit persisted light/dark toggle. */
export function useTheme() {
  function toggleTheme(): void {
    const nextTheme: ThemeMode = activeTheme.value === 'light' ? 'dark' : 'light';
    activeTheme.value = nextTheme;
    applyTheme(nextTheme);
    try {
      window.localStorage.setItem(STORAGE_KEY, nextTheme);
    } catch {
      // Theme switching still works when storage is unavailable.
    }
  }

  return { activeTheme: readonly(activeTheme), toggleTheme };
}

function applyTheme(theme: ThemeMode): void {
  document.documentElement.dataset.theme = THEME_NAMES[theme];
  document.documentElement.style.colorScheme = theme;
}

function readStoredTheme(): ThemeMode | null {
  try {
    const value = window.localStorage.getItem(STORAGE_KEY);
    return value === 'light' || value === 'dark' ? value : null;
  } catch {
    return null;
  }
}
