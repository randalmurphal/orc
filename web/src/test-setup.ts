import '@testing-library/jest-dom';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';
import { resetShortcutManager } from '@/lib/shortcuts';
import { clearToastTimers } from '@/stores/uiStore';

// =============================================================================
// GLOBAL MOCKS - Set up once, shared by all test files
// =============================================================================
// IMPORTANT: These mocks are set up globally to prevent test files from setting
// up their own mocks in beforeAll() without cleanup in afterAll(). Such uncleaned
// mocks accumulate across test files and corrupt the jsdom environment, causing
// vitest to hang after ~33 test files.
// =============================================================================

// Mock ResizeObserver for Recharts, React Flow, and other components that use it.
// React Flow requires non-zero dimensions from ResizeObserver to initialize its
// viewport and render edges. The callback fires immediately on observe() with
// synthetic dimensions so React Flow can calculate edge paths in jsdom.
class ResizeObserverMock {
	private callback: ResizeObserverCallback;
	constructor(callback: ResizeObserverCallback) {
		this.callback = callback;
	}
	observe(target: Element) {
		// Fire callback synchronously with non-zero dimensions so React Flow
		// initializes its viewport within the same act() scope. This is called
		// from useEffect (not during render), so synchronous setState is safe.
		this.callback(
			[
				{
					target,
					contentRect: {
						x: 0,
						y: 0,
						width: 800,
						height: 600,
						top: 0,
						right: 800,
						bottom: 600,
						left: 0,
						toJSON() {
							return this;
						},
					},
					borderBoxSize: [{ blockSize: 600, inlineSize: 800 }],
					contentBoxSize: [{ blockSize: 600, inlineSize: 800 }],
					devicePixelContentBoxSize: [{ blockSize: 600, inlineSize: 800 }],
				} as ResizeObserverEntry,
			],
			this
		);
	}
	unobserve() {}
	disconnect() {}
}
globalThis.ResizeObserver = ResizeObserverMock;

// Mock IntersectionObserver for React Flow and lazy-loading components
class IntersectionObserverMock {
	constructor(_callback: IntersectionObserverCallback) {}
	observe() {}
	unobserve() {}
	disconnect() {}
	takeRecords() {
		return [];
	}
}
globalThis.IntersectionObserver = IntersectionObserverMock as unknown as typeof IntersectionObserver;

// Mock Element.prototype methods used by Radix UI and other libraries
Element.prototype.scrollIntoView = vi.fn();
Element.prototype.hasPointerCapture = vi.fn().mockReturnValue(false);
Element.prototype.setPointerCapture = vi.fn();
Element.prototype.releasePointerCapture = vi.fn();

// Mock window.confirm for delete confirmations in tests
window.confirm = vi.fn().mockReturnValue(true);

// Prevent test files from replacing ResizeObserver via Object.defineProperty.
// Test files that define their own beforeAll() mocks without afterAll() cleanup
// can corrupt the environment. We intercept Object.defineProperty to block this.
// Direct assignment (global.ResizeObserver = ...) is allowed but will be overwritten
// since our mocks are already set up.
const _origDefineProperty = Object.defineProperty;
const protectedGlobalProps = new Set(['ResizeObserver', 'IntersectionObserver']);

Object.defineProperty = ((
	obj: object,
	prop: PropertyKey,
	descriptor: PropertyDescriptor
) => {
	// Block attempts to redefine our protected mocks via defineProperty
	// Use typeof check to avoid ReferenceError when window is undefined during teardown
	const isGlobalObject =
		obj === globalThis ||
		obj === global ||
		(typeof window !== 'undefined' && obj === window);
	if (isGlobalObject && protectedGlobalProps.has(prop as string)) {
		// Silently ignore - our mock is already set up
		return obj;
	}
	return _origDefineProperty.call(Object, obj, prop, descriptor);
}) as typeof Object.defineProperty;

// jsdom returns 0 for all layout measurements. React Flow uses offsetWidth/offsetHeight
// to measure nodes and getBoundingClientRect for handle positions. Without non-zero
// values, nodes aren't "initialized" and edges never render.
_origDefineProperty.call(Object, HTMLElement.prototype, 'offsetWidth', {
	configurable: true,
	get() {
		return 800;
	},
});
_origDefineProperty.call(Object, HTMLElement.prototype, 'offsetHeight', {
	configurable: true,
	get() {
		return 600;
	},
});

// React Flow reads DOMMatrixReadOnly to extract zoom from the viewport's CSS transform.
// jsdom doesn't implement DOMMatrixReadOnly, so provide a minimal mock with zoom=1.
if (!globalThis.DOMMatrixReadOnly) {
	(globalThis as Record<string, unknown>).DOMMatrixReadOnly = class DOMMatrixReadOnly {
		m22 = 1;
		constructor(_init?: string | number[]) {}
	};
}

Element.prototype.getBoundingClientRect = function () {
	return {
		x: 0,
		y: 0,
		width: 800,
		height: 600,
		top: 0,
		right: 800,
		bottom: 600,
		left: 0,
		toJSON() {
			return this;
		},
	} as DOMRect;
};

// Mock localStorage for tests
const localStorageMock = (() => {
	let store: Record<string, string> = {};
	return {
		getItem: (key: string) => store[key] ?? null,
		setItem: (key: string, value: string) => {
			store[key] = value;
		},
		removeItem: (key: string) => {
			delete store[key];
		},
		clear: () => {
			store = {};
		},
	};
})();

Object.defineProperty(globalThis, 'localStorage', {
	value: localStorageMock,
});

// Mock window.location for URL tests
let mockSearch = '';
let mockHref = 'http://localhost:5174/';

const locationMock = {
	get search() {
		return mockSearch;
	},
	get href() {
		return mockHref;
	},
	set href(value: string) {
		mockHref = value;
		const url = new URL(value);
		mockSearch = url.search;
	},
};

Object.defineProperty(globalThis, 'location', {
	value: locationMock,
	writable: true,
});

// Mock history for URL sync tests
const historyMock = {
	pushState: vi.fn((_state: unknown, _title: string, url: string) => {
		mockHref = url;
		const urlObj = new URL(url);
		mockSearch = urlObj.search;
	}),
	replaceState: vi.fn((_state: unknown, _title: string, url: string) => {
		mockHref = url;
		const urlObj = new URL(url);
		mockSearch = urlObj.search;
	}),
};

Object.defineProperty(globalThis, 'history', {
	value: historyMock,
	writable: true,
});

// Helper to reset URL mocks between tests
export function resetUrlMocks() {
	mockSearch = '';
	mockHref = 'http://localhost:5174/';
	historyMock.pushState.mockClear();
	historyMock.replaceState.mockClear();
}

// Helper to set URL search params for tests
export function setMockSearch(search: string) {
	mockSearch = search;
	mockHref = `http://localhost:5174/${search}`;
}

// Mock fetch for API tests
globalThis.fetch = vi.fn();

// Mock requestAnimationFrame/cancelAnimationFrame to prevent hanging
// jsdom's polyfill can leave pending frames that keep the process alive
let rafId = 0;
const rafCallbacks = new Map<number, FrameRequestCallback>();
// Track setTimeout IDs so we can cancel them in afterEach
const rafTimeoutIds = new Map<number, ReturnType<typeof setTimeout>>();

globalThis.requestAnimationFrame = (callback: FrameRequestCallback): number => {
	const id = ++rafId;
	rafCallbacks.set(id, callback);
	// Execute via setTimeout(0) for predictability
	// Track the timeout ID so we can cancel pending ones in afterEach
	const timeoutId = setTimeout(() => {
		rafTimeoutIds.delete(id);
		const cb = rafCallbacks.get(id);
		if (cb) {
			rafCallbacks.delete(id);
			cb(performance.now());
		}
	}, 0);
	rafTimeoutIds.set(id, timeoutId);
	return id;
};

globalThis.cancelAnimationFrame = (id: number): void => {
	rafCallbacks.delete(id);
	const timeoutId = rafTimeoutIds.get(id);
	if (timeoutId) {
		clearTimeout(timeoutId);
		rafTimeoutIds.delete(id);
	}
};

/**
 * Cancel all pending RAF timeouts. Called in afterEach to prevent
 * orphan timers from keeping the test runner alive.
 */
function clearPendingRafTimeouts(): void {
	for (const timeoutId of rafTimeoutIds.values()) {
		clearTimeout(timeoutId);
	}
	rafTimeoutIds.clear();
	rafCallbacks.clear();
}

// Global cleanup after each test to prevent hanging handles
afterEach(() => {
	// CRITICAL: Unmount all rendered components to prevent DOM accumulation
	// Testing-library auto-cleanup can fail with custom setups
	cleanup();

	// Clear pending RAF timeouts - must cancel the actual setTimeout handles
	clearPendingRafTimeouts();

	// Clear toast auto-dismiss timers to prevent orphan setTimeout handles
	clearToastTimers();

	// Reset the ShortcutManager singleton to clear event listeners and timers
	resetShortcutManager();

	// Reset URL mocks
	mockSearch = '';
	mockHref = 'http://localhost:5174/';
	historyMock.pushState.mockClear();
	historyMock.replaceState.mockClear();

	// Clear localStorage mock
	localStorageMock.clear();

	// Clear mock call history to prevent accumulation across tests
	// These mocks are set up once globally but their call history should reset
	vi.mocked(Element.prototype.scrollIntoView).mockClear();
	vi.mocked(Element.prototype.hasPointerCapture).mockClear();
	vi.mocked(Element.prototype.setPointerCapture).mockClear();
	vi.mocked(Element.prototype.releasePointerCapture).mockClear();
	vi.mocked(window.confirm).mockClear();
	vi.mocked(globalThis.fetch).mockClear();

	// Restore global mocks to prevent test file overrides from persisting
	// Test files that set `global.ResizeObserver = noOpMock` would break React Flow
	globalThis.ResizeObserver = ResizeObserverMock;
	globalThis.IntersectionObserver = IntersectionObserverMock as unknown as typeof IntersectionObserver;
});
