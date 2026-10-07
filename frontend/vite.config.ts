import { fileURLToPath, URL } from 'node:url';
import vue from '@vitejs/plugin-vue';
import vuetify from 'vite-plugin-vuetify';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [vue(), vuetify({ autoImport: true })],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5173,
    strictPort: true,
    // Тот же origin для страницы и API: cookie сессии работает без CORS.
    proxy: { '/api': 'http://localhost:8080' },
  },
  test: {
    environment: 'happy-dom',
    // Vuetify поставляет компоненты вместе с .css — без inline Node не сможет их импортировать.
    server: { deps: { inline: ['vuetify'] } },
  },
});
