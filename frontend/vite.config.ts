import { fileURLToPath, URL } from 'node:url'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import wails from '@wailsio/runtime/plugins/vite'

export default defineConfig({
  server: { host: '127.0.0.1', port: 9245, strictPort: true },
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  plugins: [tailwindcss(), react(), wails('./bindings')],
  test: { environment: 'jsdom', setupFiles: ['./src/test-setup.ts'], clearMocks: true },
})
