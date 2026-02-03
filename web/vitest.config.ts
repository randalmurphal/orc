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
    // Use forks pool for better isolation - threads pool can share global state
    // which causes issues with accumulated mocks and timers across test files
    pool: 'forks',
    poolOptions: {
      forks: {
        maxForks: DEFAULT_WORKERS,
        minForks: 1,
      },
    },
    // Prevent hanging tests
    testTimeout: 30000,
    hookTimeout: 10000,
    teardownTimeout: 5000,
  },
});
