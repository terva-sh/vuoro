import { defineConfig } from 'vitest/config'

export default defineConfig({
  esbuild: { jsx: 'automatic', jsxImportSource: 'preact' },
  test: { environment: 'node', include: ['web/src/**/*.test.{ts,tsx}'] },
})
