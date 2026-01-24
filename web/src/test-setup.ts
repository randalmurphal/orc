import '@testing-library/jest-dom';

// Mock ResizeObserver for Recharts and other components that use it
class ResizeObserverMock {
	observe() {}
	unobserve() {}
	disconnect() {}
}
globalThis.ResizeObserver = ResizeObserverMock;

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
