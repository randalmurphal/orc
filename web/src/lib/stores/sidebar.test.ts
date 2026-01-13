import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';

// Mock localStorage
const localStorageMock = (() => {
	let store: Record<string, string> = {};
	return {
		getItem: vi.fn((key: string) => store[key] || null),
		setItem: vi.fn((key: string, value: string) => {
			store[key] = value;
		}),
		removeItem: vi.fn((key: string) => {
			delete store[key];
		}),
		clear: vi.fn(() => {
			store = {};
		}),
		_setStore: (newStore: Record<string, string>) => {
			store = newStore;
		}
	};
})();

Object.defineProperty(globalThis, 'localStorage', {
	value: localStorageMock,
	writable: true
});

describe('sidebar store', () => {
	beforeEach(() => {
		localStorageMock.clear();
		vi.clearAllMocks();
		// Reset module cache to get fresh store
		vi.resetModules();
	});

	afterEach(() => {
		vi.resetModules();
	});

	it('defaults to expanded when no localStorage value', async () => {
		const { sidebarExpanded } = await import('./sidebar');
		expect(get(sidebarExpanded)).toBe(true);
	});

	it('reads collapsed value from localStorage', async () => {
		// Set the value BEFORE importing the module
		localStorageMock._setStore({ 'orc-sidebar-expanded': 'false' });
		const { sidebarExpanded } = await import('./sidebar');
		expect(get(sidebarExpanded)).toBe(false);
	});

	it('toggle() switches from expanded to collapsed', async () => {
		const { sidebarExpanded } = await import('./sidebar');
		expect(get(sidebarExpanded)).toBe(true);

		sidebarExpanded.toggle();

		expect(get(sidebarExpanded)).toBe(false);
		expect(localStorageMock.setItem).toHaveBeenCalledWith('orc-sidebar-expanded', 'false');
	});

	it('toggle() switches from collapsed to expanded', async () => {
		localStorageMock._setStore({ 'orc-sidebar-expanded': 'false' });
		const { sidebarExpanded } = await import('./sidebar');
		expect(get(sidebarExpanded)).toBe(false);

		sidebarExpanded.toggle();

		expect(get(sidebarExpanded)).toBe(true);
		expect(localStorageMock.setItem).toHaveBeenCalledWith('orc-sidebar-expanded', 'true');
	});

	it('set() updates value and persists to localStorage', async () => {
		const { sidebarExpanded } = await import('./sidebar');

		sidebarExpanded.set(false);
		expect(get(sidebarExpanded)).toBe(false);
		expect(localStorageMock.setItem).toHaveBeenCalledWith('orc-sidebar-expanded', 'false');

		sidebarExpanded.set(true);
		expect(get(sidebarExpanded)).toBe(true);
		expect(localStorageMock.setItem).toHaveBeenCalledWith('orc-sidebar-expanded', 'true');
	});

	it('maintains backwards compatibility with sidebarPinned export', async () => {
		const { sidebarExpanded, sidebarPinned } = await import('./sidebar');
		// They should be the same store
		expect(sidebarExpanded).toBe(sidebarPinned);
	});
});
