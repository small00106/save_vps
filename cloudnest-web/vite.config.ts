import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const apiProxy = {
  '/api': {
    target: 'http://localhost:8800',
    changeOrigin: true,
    ws: true,
  },
}

export default defineConfig({
  plugins: [react(), tailwindcss()],
  experimental: {
    bundledDev: false,
  },
  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    pool: "threads",
    maxWorkers: 1,
  },
  server: {
    port: 3000,
    hmr: false,
    proxy: apiProxy,
  },
  preview: {
    proxy: apiProxy,
  },
})

