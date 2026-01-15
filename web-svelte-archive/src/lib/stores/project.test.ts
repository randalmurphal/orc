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

// Mock window.location
let mockSearch = '';
const mockLocation = {
	get search() {
		return mockSearch;
	},
	set search(value: string) {
		mockSearch = value;
	},
	href: 'http://localhost:5173/',
	origin: 'http://localhost:5173',
	pathname: '/'
};

Object.defineProperty(globalThis, 'location', {
	value: mockLocation,
	writable: true
});

// Mock window.history
const historyMock = {
	pushState: vi.fn(),
	replaceState: vi.fn()
};

Object.defineProperty(globalThis, 'history', {
	value: historyMock,
	writable: true
});

// Mock URL class if needed
const OriginalURL = globalThis.URL;

describe('project store - URL parameter persistence', () => {
	beforeEach(() => {
		localStorageMock.clear();
		mockSearch = '';
		mockLocation.href = 'http://localhost:5173/';
		vi.clearAllMocks();
		vi.resetModules();
	});

	afterEach(() => {
		vi.resetModules();
	});

	describe('getUrlProjectId', () => {
		it('returns null when no project param in URL', async () => {
			mockSearch = '';
			const { getUrlProjectId } = await import('./project');
			expect(getUrlProjectId()).toBe(null);
		});

		it('returns project ID from URL param', async () => {
			mockSearch = '?project=proj-123';
			const { getUrlProjectId } = await import('./project');
			expect(getUrlProjectId()).toBe('proj-123');
		});

		it('handles URL with other params', async () => {
			mockSearch = '?foo=bar&project=proj-456&baz=qux';
			const { getUrlProjectId } = await import('./project');
			expect(getUrlProjectId()).toBe('proj-456');
		});
	});

	describe('setUrlProjectId', () => {
		it('pushes state with project ID to history', async () => {
			mockLocation.href = 'http://localhost:5173/';
			const { setUrlProjectId } = await import('./project');

			setUrlProjectId('proj-789', false);

			expect(historyMock.pushState).toHaveBeenCalledWith(
				{ projectId: 'proj-789' },
				'',
				'http://localhost:5173/?project=proj-789'
			);
		});

		it('replaces state when replace=true', async () => {
			mockLocation.href = 'http://localhost:5173/';
			const { setUrlProjectId } = await import('./project');

			setUrlProjectId('proj-999', true);

			expect(historyMock.replaceState).toHaveBeenCalledWith(
				{ projectId: 'proj-999' },
				'',
				'http://localhost:5173/?project=proj-999'
			);
			expect(historyMock.pushState).not.toHaveBeenCalled();
		});

		it('removes project param when id is null', async () => {
			mockLocation.href = 'http://localhost:5173/?project=old-proj';
			const { setUrlProjectId } = await import('./project');

			setUrlProjectId(null, false);

			expect(historyMock.pushState).toHaveBeenCalledWith(
				{ projectId: null },
				'',
				'http://localhost:5173/'
			);
		});

		it('preserves other URL params', async () => {
			mockLocation.href = 'http://localhost:5173/?view=board&project=old';
			const { setUrlProjectId } = await import('./project');

			setUrlProjectId('new-proj', false);

			expect(historyMock.pushState).toHaveBeenCalledWith(
				{ projectId: 'new-proj' },
				'',
				expect.stringContaining('project=new-proj')
			);
			expect(historyMock.pushState).toHaveBeenCalledWith(
				{ projectId: 'new-proj' },
				'',
				expect.stringContaining('view=board')
			);
		});
	});

	describe('initial project ID priority', () => {
		it('prefers URL param over localStorage', async () => {
			mockSearch = '?project=url-proj';
			localStorageMock._setStore({ orc_current_project_id: 'local-proj' });

			const { currentProjectId } = await import('./project');
			expect(get(currentProjectId)).toBe('url-proj');
		});

		it('falls back to localStorage when no URL param', async () => {
			mockSearch = '';
			localStorageMock._setStore({ orc_current_project_id: 'local-proj' });

			const { currentProjectId } = await import('./project');
			expect(get(currentProjectId)).toBe('local-proj');
		});

		it('returns null when neither URL nor localStorage has value', async () => {
			mockSearch = '';
			localStorageMock.clear();

			const { currentProjectId } = await import('./project');
			expect(get(currentProjectId)).toBe(null);
		});
	});

	describe('selectProject', () => {
		it('updates store and pushes to URL history', async () => {
			mockLocation.href = 'http://localhost:5173/';
			const { selectProject, currentProjectId } = await import('./project');

			selectProject('new-proj');

			expect(get(currentProjectId)).toBe('new-proj');
			expect(historyMock.pushState).toHaveBeenCalledWith(
				{ projectId: 'new-proj' },
				'',
				'http://localhost:5173/?project=new-proj'
			);
		});

		it('persists to localStorage', async () => {
			mockLocation.href = 'http://localhost:5173/';
			const { selectProject } = await import('./project');

			selectProject('persisted-proj');

			expect(localStorageMock.setItem).toHaveBeenCalledWith(
				'orc_current_project_id',
				'persisted-proj'
			);
		});
	});

	describe('handlePopState', () => {
		it('updates store from popstate event state', async () => {
			mockSearch = '?project=initial';
			const { handlePopState, currentProjectId, selectProject } = await import('./project');

			// First, select a different project
			selectProject('current-proj');
			vi.clearAllMocks();

			// Simulate back button (popstate event)
			const event = new PopStateEvent('popstate', {
				state: { projectId: 'previous-proj' }
			});
			handlePopState(event);

			expect(get(currentProjectId)).toBe('previous-proj');
		});

		it('falls back to URL param when state is null', async () => {
			mockSearch = '?project=url-proj';
			const { handlePopState, currentProjectId, selectProject } = await import('./project');

			// Select a different project
			selectProject('current-proj');
			vi.clearAllMocks();

			// Simulate popstate with null state (e.g., manually typed URL)
			mockSearch = '?project=url-proj';
			const event = new PopStateEvent('popstate', { state: null });
			handlePopState(event);

			expect(get(currentProjectId)).toBe('url-proj');
		});

		it('does not push to history during popstate handling', async () => {
			mockSearch = '?project=initial';
			mockLocation.href = 'http://localhost:5173/?project=initial';
			const { handlePopState, selectProject } = await import('./project');

			selectProject('current-proj');
			vi.clearAllMocks();

			// Simulate back button
			const event = new PopStateEvent('popstate', {
				state: { projectId: 'previous-proj' }
			});
			handlePopState(event);

			// Should NOT push new state (would break browser history)
			expect(historyMock.pushState).not.toHaveBeenCalled();
		});
	});
});
