import { defineConfig } from 'vite'
import { resolve } from 'node:path'

// Production Wails embed uses frontend/dist from index.html only.
// demo.html + mock bindings are deliberately excluded from the shipped bundle.
export default defineConfig({
  root: '.',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
      },
    },
  },
  server: {
    strictPort: true,
  },
})
