import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

export default defineConfig({
	plugins: [svelte({ hot: !process.env.VITEST })],
	resolve: {
		alias: {
			$lib: path.resolve(__dirname, './src/lib'),
			'$app/environment': path.resolve(__dirname, './src/test-mocks/environment.ts'),
			'$app/navigation': path.resolve(__dirname, './src/test-mocks/navigation.ts'),
			'$app/stores': path.resolve(__dirname, './src/test-mocks/stores.ts')
		},
		// Force browser conditions for Svelte
		conditions: ['browser']
	},
	test: {
		include: ['src/**/*.test.ts'],
		environment: 'jsdom',
		globals: true,
		setupFiles: ['./src/test-setup.ts'],
		// Ensure DOM environment is used
		deps: {
			optimizer: {
				web: {
					include: ['svelte']
				}
			}
		}
	}
});
