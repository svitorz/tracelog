import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// O config roda no Node; esta declaração evita depender de @types/node só por process.env.
declare const process: { env: Record<string, string | undefined> }

// API para o proxy de desenvolvimento (spec 0006, seção 5.3). Em produção a UI é servida
// pelo próprio binário, na mesma origem, e não há proxy nem CORS.
const apiTarget = process.env.TRACELOG_API_URL ?? 'http://127.0.0.1:8080'

export default defineConfig({
  base: '/ui/',
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      // Respostas de referência da API, compartilhadas com os testes Go (spec 0005, seção 5.4).
      '@fixtures': new URL('../internal/adapters/http/testdata/api', import.meta.url).pathname,
    },
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': apiTarget,
      '/ready': apiTarget,
    },
  },
  build: {
    // Dentro do módulo Go raiz, para o go:embed da fase 6 (spec 0006, seção 5.1).
    outDir: '../internal/adapters/http/webui/dist',
    emptyOutDir: true,
  },
  test: {
    environment: 'happy-dom',
    include: ['src/**/*.test.{ts,tsx}'],
    restoreMocks: true,
  },
})
