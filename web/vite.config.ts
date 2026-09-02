import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    // Локально фронт ходит в Go-сервис, поэтому в проде CORS не нужен вовсе.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
