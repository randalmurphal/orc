/**
 * Integration test: stores/index.ts → threadStore barrel export wiring
 *
 * Verifies that threadStore hooks are re-exported from the barrel file
 * (stores/index.ts). Components import from '@/stores', not from
 * '@/stores/threadStore' directly. If the barrel doesn't re-export
 * threadStore, components silently get undefined imports.
 *
 * This test imports from '@/stores' (the barrel) — not from
 * '@/stores/threadStore' directly — to verify the production import path.
 *
 * Production path: Component → import { useThreadStore } from '@/stores' → stores/index.ts → threadStore.ts
 */

import { describe, it, expect } from 'vitest';

describe('stores barrel export: threadStore wiring', () => {
	it('should export useThreadStore from barrel', async () => {
		const stores = await import('@/stores');

		expect(stores.useThreadStore).toBeDefined();
		expect(typeof stores.useThreadStore).toBe('function');
	});

	it('should export useThreads selector from barrel', async () => {
		const stores = await import('@/stores');

		expect(stores.useThreads).toBeDefined();
		expect(typeof stores.useThreads).toBe('function');
	});

	it('should export useSelectedThread selector from barrel', async () => {
		const stores = await import('@/stores');

		expect(stores.useSelectedThread).toBeDefined();
		expect(typeof stores.useSelectedThread).toBe('function');
	});

	it('should export useThreadLoading selector from barrel', async () => {
		const stores = await import('@/stores');

		expect(stores.useThreadLoading).toBeDefined();
		expect(typeof stores.useThreadLoading).toBe('function');
	});

	it('should export useThreadError selector from barrel', async () => {
		const stores = await import('@/stores');

		expect(stores.useThreadError).toBeDefined();
		expect(typeof stores.useThreadError).toBe('function');
	});
});
