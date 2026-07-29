import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import tailwindcss from '@tailwindcss/vite';
import { demoApiPlugin } from './demoApi';

const demoDataEnabled = process.env.COMPASS_DEMO_DATA === '1';

export default defineConfig({
  plugins: [tailwindcss(), vue(), ...(demoDataEnabled ? [demoApiPlugin()] : [])],
  resolve: {
    alias: {
      '@': new URL('./src', import.meta.url).pathname,
    },
  },
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './vitest.setup.ts',
  },
});
