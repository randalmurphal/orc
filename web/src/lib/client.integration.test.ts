/**
 * Integration test: client.ts → threadClient wiring
 *
 * Verifies that the production client module exports threadClient,
 * created from ThreadService via createClient. Without this export,
 * threadStore cannot make API calls and thread features are dead code.
 *
 * This test does NOT mock @/lib/client — it imports the real module
 * to verify the actual export exists.
 *
 * Production path: client.ts → createClient(ThreadService, transport) → threadClient export
 */

import { describe, it, expect } from 'vitest';

describe('client.ts thread service wiring', () => {
	it('should export threadClient from @/lib/client', async () => {
		// Dynamic import to get the actual module (not mocked)
		const clientModule = await import('@/lib/client');

		expect(clientModule.threadClient).toBeDefined();
	});

	it('should have listThreads method on threadClient', async () => {
		const clientModule = await import('@/lib/client');

		// threadClient must have the methods that threadStore calls
		expect(typeof clientModule.threadClient.listThreads).toBe('function');
	});

	it('should have createThread method on threadClient', async () => {
		const clientModule = await import('@/lib/client');

		expect(typeof clientModule.threadClient.createThread).toBe('function');
	});

	it('should have sendMessage method on threadClient', async () => {
		const clientModule = await import('@/lib/client');

		expect(typeof clientModule.threadClient.sendMessage).toBe('function');
	});

	it('should have getThread method on threadClient', async () => {
		const clientModule = await import('@/lib/client');

		expect(typeof clientModule.threadClient.getThread).toBe('function');
	});
});
