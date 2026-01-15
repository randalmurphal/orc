/**
 * Tests for dependency status filter store
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { get } from 'svelte/store';
import {
	currentDependencyStatus,
	selectDependencyStatus,
	handleDependencyPopState,
	initDependencyStatusFromUrl,
	getUrlDependencyStatus,
	DEPENDENCY_OPTIONS,
	type DependencyStatusFilter
} from './dependency';

describe('dependency store', () => {
	let originalLocation: Location;
	let originalHistory: History;
	let mockUrl: URL;

	beforeEach(() => {
		// Store original globals
		originalLocation = window.location;
		originalHistory = window.history;

		// Create mock URL
		mockUrl = new URL('http://localhost/');

		// Mock location
		Object.defineProperty(window, 'location', {
			value: {
				get href() {
					return mockUrl.href;
				},
				get search() {
					return mockUrl.search;
				}
			},
			writable: true
		});

		// Mock localStorage
		const storage: Record<string, string> = {};
		vi.stubGlobal('localStorage', {
			getItem: vi.fn((key: string) => storage[key] ?? null),
			setItem: vi.fn((key: string, value: string) => {
				storage[key] = value;
			}),
			removeItem: vi.fn((key: string) => {
				delete storage[key];
			})
		});

		// Mock history
		vi.stubGlobal('history', {
			state: {},
			pushState: vi.fn(),
			replaceState: vi.fn()
		});

		// Reset store to default
		currentDependencyStatus.set('all');
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		Object.defineProperty(window, 'location', {
			value: originalLocation,
			writable: true
		});
	});

	describe('DEPENDENCY_OPTIONS', () => {
		it('should have all filter options', () => {
			expect(DEPENDENCY_OPTIONS).toHaveLength(4);
			expect(DEPENDENCY_OPTIONS.map((o) => o.value)).toEqual(['all', 'ready', 'blocked', 'none']);
		});

		it('should have labels for all options', () => {
			expect(DEPENDENCY_OPTIONS.every((o) => o.label.length > 0)).toBe(true);
		});
	});

	describe('getUrlDependencyStatus', () => {
		it('returns null when no dependency_status param in URL', () => {
			mockUrl = new URL('http://localhost/');
			Object.defineProperty(window, 'location', {
				value: { href: mockUrl.href, search: mockUrl.search },
				writable: true
			});

			expect(getUrlDependencyStatus()).toBe(null);
		});

		it('returns the dependency_status param when present', () => {
			mockUrl = new URL('http://localhost/?dependency_status=blocked');
			Object.defineProperty(window, 'location', {
				value: { href: mockUrl.href, search: mockUrl.search },
				writable: true
			});

			expect(getUrlDependencyStatus()).toBe('blocked');
		});

		it('returns null for invalid dependency_status values', () => {
			mockUrl = new URL('http://localhost/?dependency_status=invalid');
			Object.defineProperty(window, 'location', {
				value: { href: mockUrl.href, search: mockUrl.search },
				writable: true
			});

			expect(getUrlDependencyStatus()).toBe(null);
		});

		it('returns null for "all" value (default, not stored in URL)', () => {
			// 'all' should not be stored in URL - it represents no filter
			mockUrl = new URL('http://localhost/?dependency_status=all');
			Object.defineProperty(window, 'location', {
				value: { href: mockUrl.href, search: mockUrl.search },
				writable: true
			});

			// getUrlDependencyStatus should not return 'all' since it's not a valid URL param
			expect(getUrlDependencyStatus()).toBe(null);
		});
	});

	describe('selectDependencyStatus', () => {
		it('updates the store with the selected status', () => {
			selectDependencyStatus('blocked');
			expect(get(currentDependencyStatus)).toBe('blocked');
		});

		it('clears the filter when null is passed', () => {
			selectDependencyStatus('blocked');
			selectDependencyStatus(null);
			expect(get(currentDependencyStatus)).toBe('all');
		});

		it('saves to localStorage for non-all values', () => {
			selectDependencyStatus('ready');
			expect(localStorage.setItem).toHaveBeenCalledWith('orc_dependency_status_filter', 'ready');
		});

		it('removes from localStorage for all value', () => {
			selectDependencyStatus('all');
			expect(localStorage.removeItem).toHaveBeenCalledWith('orc_dependency_status_filter');
		});
	});

	describe('handleDependencyPopState', () => {
		it('updates store from popstate event state', () => {
			mockUrl = new URL('http://localhost/?dependency_status=blocked');
			Object.defineProperty(window, 'location', {
				value: { href: mockUrl.href, search: mockUrl.search },
				writable: true
			});

			const event = new PopStateEvent('popstate', {
				state: { dependencyStatus: 'blocked' }
			});

			handleDependencyPopState(event);
			expect(get(currentDependencyStatus)).toBe('blocked');
		});

		it('falls back to URL param if no state', () => {
			mockUrl = new URL('http://localhost/?dependency_status=ready');
			Object.defineProperty(window, 'location', {
				value: { href: mockUrl.href, search: mockUrl.search },
				writable: true
			});

			const event = new PopStateEvent('popstate', { state: null });

			handleDependencyPopState(event);
			expect(get(currentDependencyStatus)).toBe('ready');
		});

		it('resets to all if no state or URL param', () => {
			selectDependencyStatus('blocked'); // Set initial value
			mockUrl = new URL('http://localhost/');
			Object.defineProperty(window, 'location', {
				value: { href: mockUrl.href, search: mockUrl.search },
				writable: true
			});

			const event = new PopStateEvent('popstate', { state: null });

			handleDependencyPopState(event);
			expect(get(currentDependencyStatus)).toBe('all');
		});
	});

	describe('initDependencyStatusFromUrl', () => {
		it('initializes from URL param', () => {
			mockUrl = new URL('http://localhost/?dependency_status=none');
			Object.defineProperty(window, 'location', {
				value: { href: mockUrl.href, search: mockUrl.search },
				writable: true
			});

			initDependencyStatusFromUrl();
			expect(get(currentDependencyStatus)).toBe('none');
		});

		it('does nothing if no URL param', () => {
			currentDependencyStatus.set('blocked'); // Set initial value
			mockUrl = new URL('http://localhost/');
			Object.defineProperty(window, 'location', {
				value: { href: mockUrl.href, search: mockUrl.search },
				writable: true
			});

			initDependencyStatusFromUrl();
			// Should keep existing value
			expect(get(currentDependencyStatus)).toBe('blocked');
		});
	});
});
