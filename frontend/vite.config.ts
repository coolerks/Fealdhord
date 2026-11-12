import { fileURLToPath, URL } from 'node:url'
import { searchForWorkspaceRoot } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import wails from '@wailsio/runtime/plugins/vite'

export default defineConfig({
  server: {
    host: '127.0.0.1', port: Number(process.env.WAILS_VITE_PORT) || 9245, strictPort: true,
    fs: { allow: [searchForWorkspaceRoot(fileURLToPath(new URL('.', import.meta.url))), fileURLToPath(new URL('../build/appicon.png', import.meta.url))] },
  },
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  plugins: [tailwindcss(), react(), wails('./bindings')],
  test: { environment: 'jsdom', setupFiles: ['./src/test-setup.ts'], clearMocks: true, css: { include: [/highlight\.js\/styles\//] } },
})
