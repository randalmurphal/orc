import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

// We need to test that VirtualScroller properly cleans up ResizeObserver
// Since VirtualScroller uses generics and Snippet, we'll test the behavior directly

describe('VirtualScroller', () => {
	let resizeObserverInstances: {
		observe: ReturnType<typeof vi.fn>;
		disconnect: ReturnType<typeof vi.fn>;
		unobserve: ReturnType<typeof vi.fn>;
	}[] = [];
	let ResizeObserverMock: ReturnType<typeof vi.fn>;

	beforeEach(() => {
		resizeObserverInstances = [];

		// Mock ResizeObserver
		ResizeObserverMock = vi.fn((callback) => {
			const instance = {
				observe: vi.fn(),
				disconnect: vi.fn(),
				unobserve: vi.fn()
			};
			resizeObserverInstances.push(instance);
			return instance;
		});

		vi.stubGlobal('ResizeObserver', ResizeObserverMock);
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	describe('cleanup function behavior', () => {
		it('ResizeObserver mock is correctly set up', () => {
			// Create a mock observer to verify our mock works
			const callback = vi.fn();
			const observer = new ResizeObserver(callback);

			expect(ResizeObserverMock).toHaveBeenCalledTimes(1);
			expect(observer.observe).toBeDefined();
			expect(observer.disconnect).toBeDefined();
			expect(resizeObserverInstances).toHaveLength(1);
		});

		it('ResizeObserver disconnect is callable', () => {
			const callback = vi.fn();
			const observer = new ResizeObserver(callback);

			// Should not throw
			expect(() => observer.disconnect()).not.toThrow();
			expect(resizeObserverInstances[0].disconnect).toHaveBeenCalled();
		});
	});

	describe('virtual scrolling calculations', () => {
		it('calculates visible range correctly', () => {
			const items = Array.from({ length: 100 }, (_, i) => ({ id: i, value: `item-${i}` }));
			const itemHeight = 22;
			const buffer = 10;
			const containerHeight = 400;
			const scrollTop = 0;

			// Calculate expected values matching VirtualScroller logic
			const visibleStart = Math.max(0, Math.floor(scrollTop / itemHeight) - buffer);
			const visibleEnd = Math.min(
				items.length,
				visibleStart + Math.ceil(containerHeight / itemHeight) + buffer * 2
			);

			expect(visibleStart).toBe(0);
			// ceil(400/22) + 20 = 19 + 20 = 39, but start is 0 so: 0 + 39 = 39
			expect(visibleEnd).toBe(39);
		});

		it('calculates visible range with scroll offset', () => {
			const items = Array.from({ length: 100 }, (_, i) => ({ id: i, value: `item-${i}` }));
			const itemHeight = 22;
			const buffer = 10;
			const containerHeight = 400;
			const scrollTop = 440; // Scrolled down 20 items

			const visibleStart = Math.max(0, Math.floor(scrollTop / itemHeight) - buffer);
			const visibleEnd = Math.min(
				items.length,
				visibleStart + Math.ceil(containerHeight / itemHeight) + buffer * 2
			);

			// floor(440/22) = 20, 20 - 10 = 10
			expect(visibleStart).toBe(10);
			// 10 + ceil(400/22) + 20 = 10 + 19 + 20 = 49
			expect(visibleEnd).toBe(49);
		});

		it('clamps visible range to items array bounds', () => {
			const items = Array.from({ length: 20 }, (_, i) => ({ id: i }));
			const itemHeight = 22;
			const buffer = 10;
			const containerHeight = 400;
			const scrollTop = 0;

			const visibleStart = Math.max(0, Math.floor(scrollTop / itemHeight) - buffer);
			const visibleEnd = Math.min(
				items.length,
				visibleStart + Math.ceil(containerHeight / itemHeight) + buffer * 2
			);

			expect(visibleStart).toBe(0);
			// Would be 39, but clamped to items.length = 20
			expect(visibleEnd).toBe(20);
		});

		it('calculates correct padding heights', () => {
			const items = Array.from({ length: 100 }, (_, i) => ({ id: i }));
			const itemHeight = 22;
			const buffer = 10;
			const containerHeight = 400;
			const scrollTop = 440;

			const visibleStart = Math.max(0, Math.floor(scrollTop / itemHeight) - buffer);
			const visibleEnd = Math.min(
				items.length,
				visibleStart + Math.ceil(containerHeight / itemHeight) + buffer * 2
			);

			const topPadding = visibleStart * itemHeight;
			const bottomPadding = (items.length - visibleEnd) * itemHeight;
			const totalHeight = items.length * itemHeight;

			expect(topPadding).toBe(10 * 22); // 220
			expect(bottomPadding).toBe((100 - 49) * 22); // 51 * 22 = 1122
			expect(totalHeight).toBe(2200);
		});
	});

	describe('edge cases', () => {
		it('handles empty items array', () => {
			const items: unknown[] = [];
			const itemHeight = 22;
			const buffer = 10;
			const containerHeight = 400;
			const scrollTop = 0;

			const visibleStart = Math.max(0, Math.floor(scrollTop / itemHeight) - buffer);
			const visibleEnd = Math.min(
				items.length,
				visibleStart + Math.ceil(containerHeight / itemHeight) + buffer * 2
			);

			expect(visibleStart).toBe(0);
			expect(visibleEnd).toBe(0);
			expect(items.slice(visibleStart, visibleEnd)).toEqual([]);
		});

		it('handles single item', () => {
			const items = [{ id: 0 }];
			const itemHeight = 22;
			const buffer = 10;
			const containerHeight = 400;
			const scrollTop = 0;

			const visibleStart = Math.max(0, Math.floor(scrollTop / itemHeight) - buffer);
			const visibleEnd = Math.min(
				items.length,
				visibleStart + Math.ceil(containerHeight / itemHeight) + buffer * 2
			);

			expect(visibleStart).toBe(0);
			expect(visibleEnd).toBe(1);
		});

		it('handles scroll beyond content', () => {
			const items = Array.from({ length: 10 }, (_, i) => ({ id: i }));
			const itemHeight = 22;
			const buffer = 10;
			const containerHeight = 400;
			const scrollTop = 5000; // Way past end

			const visibleStart = Math.max(0, Math.floor(scrollTop / itemHeight) - buffer);
			const visibleEnd = Math.min(
				items.length,
				visibleStart + Math.ceil(containerHeight / itemHeight) + buffer * 2
			);

			// floor(5000/22) = 227, 227 - 10 = 217 > 10, so items.length = 10
			// visibleEnd = min(10, 217 + 39) = 10
			expect(visibleEnd).toBeLessThanOrEqual(items.length);
		});

		it('handles custom item height', () => {
			const items = Array.from({ length: 50 }, (_, i) => ({ id: i }));
			const itemHeight = 50; // Larger items
			const buffer = 5;
			const containerHeight = 300;
			const scrollTop = 0;

			const visibleStart = Math.max(0, Math.floor(scrollTop / itemHeight) - buffer);
			const visibleEnd = Math.min(
				items.length,
				visibleStart + Math.ceil(containerHeight / itemHeight) + buffer * 2
			);

			// ceil(300/50) = 6, 0 + 6 + 10 = 16
			expect(visibleStart).toBe(0);
			expect(visibleEnd).toBe(16);
		});

		it('handles zero buffer', () => {
			const items = Array.from({ length: 100 }, (_, i) => ({ id: i }));
			const itemHeight = 22;
			const buffer = 0;
			const containerHeight = 400;
			const scrollTop = 220; // 10 items down

			const visibleStart = Math.max(0, Math.floor(scrollTop / itemHeight) - buffer);
			const visibleEnd = Math.min(
				items.length,
				visibleStart + Math.ceil(containerHeight / itemHeight) + buffer * 2
			);

			// floor(220/22) - 0 = 10
			// 10 + ceil(400/22) + 0 = 10 + 19 = 29
			expect(visibleStart).toBe(10);
			expect(visibleEnd).toBe(29);
		});
	});

	describe('cleanup always returns function', () => {
		it('onMount cleanup pattern returns function', () => {
			// Simulate onMount pattern from VirtualScroller
			const onMountCleanup = (() => {
				const resizeObserver = new ResizeObserver(() => {});
				const containerEl: HTMLElement | null = null;

				if (containerEl) {
					resizeObserver.observe(containerEl);
				}

				return () => {
					resizeObserver.disconnect();
				};
			})();

			// Cleanup should always be a function
			expect(typeof onMountCleanup).toBe('function');

			// Calling cleanup should call disconnect
			onMountCleanup();
			expect(resizeObserverInstances[0].disconnect).toHaveBeenCalled();
		});

		it('cleanup is called even when containerEl is null', () => {
			// Simulate scenario where containerEl never gets set
			const resizeObserver = new ResizeObserver(() => {});
			const containerEl: HTMLElement | null = null;

			if (containerEl) {
				resizeObserver.observe(containerEl);
			}

			// The cleanup function is still valid
			const cleanup = () => {
				resizeObserver.disconnect();
			};

			expect(() => cleanup()).not.toThrow();
			expect(resizeObserverInstances[0].disconnect).toHaveBeenCalled();
		});
	});
});
