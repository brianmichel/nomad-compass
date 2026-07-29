import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

function setSystemTheme(dark: boolean): void {
  vi.stubGlobal('matchMedia', vi.fn(() => ({
    matches: dark,
    media: '(prefers-color-scheme: dark)',
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })));
}

describe('useTheme', () => {
  beforeEach(() => {
    vi.resetModules();
    window.localStorage.clear();
    delete document.documentElement.dataset.theme;
    document.documentElement.style.colorScheme = '';
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('uses a saved preference before the system preference', async () => {
    setSystemTheme(false);
    window.localStorage.setItem('compass-theme', 'dark');
    const { initializeTheme, useTheme } = await import('../useTheme');

    initializeTheme();

    expect(useTheme().activeTheme.value).toBe('dark');
    expect(document.documentElement.dataset.theme).toBe('nomad-mint-dark');
    expect(document.documentElement.style.colorScheme).toBe('dark');
  });

  it('falls back to the system preference when storage is missing or invalid', async () => {
    setSystemTheme(true);
    window.localStorage.setItem('compass-theme', 'unexpected');
    const { initializeTheme, useTheme } = await import('../useTheme');

    initializeTheme();

    expect(useTheme().activeTheme.value).toBe('dark');
    expect(document.documentElement.dataset.theme).toBe('nomad-mint-dark');
  });

  it('toggles and persists an explicit preference', async () => {
    setSystemTheme(false);
    const { initializeTheme, useTheme } = await import('../useTheme');
    initializeTheme();
    const theme = useTheme();

    theme.toggleTheme();

    expect(theme.activeTheme.value).toBe('dark');
    expect(window.localStorage.getItem('compass-theme')).toBe('dark');
    expect(document.documentElement.dataset.theme).toBe('nomad-mint-dark');
  });
});
