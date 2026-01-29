import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { visualizer } from 'rollup-plugin-visualizer';
import path from 'path';

export default defineConfig({
  plugins: [
    react(),
    // Bundle analysis: ANALYZE=true bun run build → opens build/stats.html
    process.env.ANALYZE === 'true' && visualizer({
      filename: 'build/stats.html',
      gzipSize: true,
      open: true,
    }),
  ].filter(Boolean),
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
    rollupOptions: {
      output: {
        manualChunks: {
          'react-vendor': ['react', 'react-dom', 'react-router-dom'],
          'ui-vendor': [
            '@radix-ui/react-accordion',
            '@radix-ui/react-dialog',
            '@radix-ui/react-dropdown-menu',
            '@radix-ui/react-popover',
            '@radix-ui/react-select',
            '@radix-ui/react-slot',
            '@radix-ui/react-tabs',
            '@radix-ui/react-toast',
            '@radix-ui/react-tooltip',
          ],
          'data-vendor': [
            'zustand',
            '@connectrpc/connect',
            '@connectrpc/connect-web',
            '@bufbuild/protobuf',
          ],
        },
      },
    },
  },
});
