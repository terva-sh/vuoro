import { defineConfig } from 'vite'
import preact from '@preact/preset-vite'

// `npm run dev` serves the frontend from source and proxies /api and /mcp to
// a running `vuoro -browser`, named by VUORO_API_URL. It must be a loopback
// origin: the guard refuses anything else anyway.
const target = process.env.VUORO_API_URL || 'http://127.0.0.1:7777'
const api = new URL(target)
if (api.protocol !== 'http:' || api.hostname !== '127.0.0.1' || api.username || api.password ||
    api.pathname !== '/' || api.search || api.hash) {
  throw new Error('VUORO_API_URL must be a http://127.0.0.1:PORT origin')
}

export default defineConfig({
  root: 'web',
  base: './',
  plugins: [preact()],
  build: { outDir: 'dist', emptyOutDir: true },
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    // The guard checks Host, so the proxy sends the API's own.
    proxy: { '/api': { target, changeOrigin: true } },
  },
})
