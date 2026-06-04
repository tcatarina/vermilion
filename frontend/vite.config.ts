import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

const backendUrl = process.env.BACKEND_URL || 'http://backend:8080'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    watch: { usePolling: true, interval: 400 },
    proxy: {
      '/api': { target: backendUrl, changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    exclude: ['e2e/**', 'node_modules/**'],
  },
})
