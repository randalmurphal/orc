import '@testing-library/jest-dom';

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

// Prevent test files from replacing the ResizeObserver mock with a no-op version.
// React Flow needs the callback-firing mock above to initialize node dimensions and
// render edge components in jsdom. Several test files define their own ResizeObserver
// mock in beforeAll that would override ours, breaking edge rendering.
const _origDefineProperty = Object.defineProperty;
Object.defineProperty = ((
	obj: object,
	prop: PropertyKey,
	descriptor: PropertyDescriptor
) => {
	if ((obj === window || obj === globalThis) && prop === 'ResizeObserver') {
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
