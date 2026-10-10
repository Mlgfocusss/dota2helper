/// <reference types="vitest/config" />
import { fileURLToPath } from 'node:url'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

const rootDir = fileURLToPath(new URL('..', import.meta.url))
const srcDir = fileURLToPath(new URL('./src', import.meta.url))

export default defineConfig(({ mode }) => {
  // Название берём из VITE_APP_NAME (сборка в Docker), иначе из APP_NAME в корневом .env.
  const rootEnv = loadEnv(mode, rootDir, ['VITE_', 'APP_NAME'])
  const appName =
    process.env.VITE_APP_NAME || rootEnv.VITE_APP_NAME || rootEnv.APP_NAME || 'D2Helper'

  return {
    plugins: [
      react(),
      tailwindcss(),
      {
        name: 'app-name-html',
        transformIndexHtml: (html) => html.replaceAll('%APP_NAME%', appName),
      },
    ],
    define: {
      'import.meta.env.VITE_APP_NAME': JSON.stringify(appName),
    },
    resolve: {
      alias: { '@': srcDir },
    },
    server: {
      port: 5173,
      proxy: {
        // Host не меняем: бэкенд строит по нему адрес возврата из Steam и проверяет Origin
        '/api': { target: process.env.VITE_PROXY_TARGET || 'http://localhost:8080' },
      },
    },
    test: {
      include: ['src/**/*.test.ts'],
    },
  }
})
