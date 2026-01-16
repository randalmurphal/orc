import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

// Limit workers to prevent OOM when running parallel orc tasks.
// Each orc worker can spawn Vitest which by default uses all CPU cores.
// With unlimited workers on a 16-core machine, 3 parallel orc tasks could
// spawn 48 test workers, exhausting memory.
const DEFAULT_WORKERS = 4;

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    include: ['src/**/*.test.{ts,tsx}'],
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
    // Limit parallelism for memory safety
    pool: 'threads',
    poolOptions: {
      threads: {
        maxThreads: DEFAULT_WORKERS,
        minThreads: 1,
      },
    },
  },
});
