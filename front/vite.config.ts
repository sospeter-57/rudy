import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath, URL } from 'node:url'

// The Go API origin and dev port come from the environment, never from code.
// Vite proxies /api to the configured target so the browser stays same-origin
// in development and no CORS headers are needed.
export default defineConfig(({ command, mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const port = Number(env.VITE_DEV_PORT) || 5173
  const proxyTarget = env.VITE_PROXY_TARGET?.trim()

  if (!proxyTarget && command === 'serve') {
    throw new Error(
      'VITE_PROXY_TARGET is required for the dev server. Copy front/.env.example to front/.env and set it to the Go API origin.',
    )
  }

  return {
    plugins: [react()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      port,
      ...(proxyTarget
        ? {
            proxy: {
              '/api': {
                target: proxyTarget,
                changeOrigin: true,
                rewrite: (path) => path.replace(/^\/api/, ''),
              },
            },
          }
        : {}),
    },
  }
})
