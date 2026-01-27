import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/orc.v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        // Configure proxy to handle server-streaming (Connect RPC)
        // This ensures chunked transfer encoding works properly
        configure: (proxy) => {
          // Disable response buffering for streaming responses
          proxy.on('proxyRes', (proxyRes) => {
            // Check for streaming response indicators
            const contentType = proxyRes.headers['content-type'] || '';
            if (contentType.includes('application/connect+proto') ||
                contentType.includes('application/grpc-web')) {
              // Ensure no buffering for streaming responses
              proxyRes.headers['x-accel-buffering'] = 'no';
            }
          });
        },
      },
      '/files': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'build',
  },
});
