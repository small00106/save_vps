import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  experimental: {
    bundledDev: false,
  },
  server: {
    port: 3000,
    hmr: false,
    proxy: {
      '/api': {
        target: 'http://localhost:8800',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
