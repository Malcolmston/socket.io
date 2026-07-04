import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

// The socket.io repo is served as a GitHub *project* page at
// https://malcolmston.github.io/socket.io/, so assets must be based under
// /socket.io/.
export default defineConfig({
  base: '/socket.io/',
  plugins: [react()],
  resolve: {
    alias: {
      // Import the vendored shared library (git submodule) from source.
      'go-ui': fileURLToPath(new URL('./vendor/go/ui/src/index.ts', import.meta.url)),
    },
  },
  build: { outDir: 'dist', emptyOutDir: true },
});
