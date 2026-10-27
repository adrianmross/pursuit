import path from 'node:path'
import { fileURLToPath } from 'node:url'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(dirname, './src'),
    },
  },
  server: {
    host: process.env.HOST || 'localhost',
    port: Number(process.env.PORT) || 5173,
    proxy: {
      '/v1': process.env.PURSUIT_API || 'http://127.0.0.1:8787',
    },
  },
})
