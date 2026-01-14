import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import type { Initiative } from '$lib/types';

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
const locationMock = {
	href: 'http://localhost:5173/',
	search: '',
	origin: 'http://localhost:5173'
};

Object.defineProperty(globalThis, 'location', {
	value: locationMock,
	writable: true
});

// Mock window.history
const historyMock = {
	state: null as unknown,
	pushState: vi.fn((state, _title, url) => {
		historyMock.state = state;
		if (url) locationMock.href = url as string;
	}),
	replaceState: vi.fn((state, _title, url) => {
		historyMock.state = state;
		if (url) locationMock.href = url as string;
	})
};

Object.defineProperty(globalThis, 'history', {
	value: historyMock,
	writable: true
});

// Mock API
vi.mock('$lib/api', () => ({
	listInitiatives: vi.fn().mockResolvedValue([]),
	createInitiative: vi.fn().mockResolvedValue({
		id: 'INIT-001',
		title: 'Test Initiative',
		status: 'draft',
		version: 1,
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString()
	})
}));

describe('initiative store', () => {
	beforeEach(() => {
		localStorageMock.clear();
		locationMock.search = '';
		locationMock.href = 'http://localhost:5173/';
		historyMock.state = null;
		vi.clearAllMocks();
		vi.resetModules();
	});

	afterEach(() => {
		vi.resetModules();
	});

	it('initializes with null when no stored value', async () => {
		const { currentInitiativeId } = await import('./initiative');
		expect(get(currentInitiativeId)).toBe(null);
	});

	it('reads initiative ID from localStorage', async () => {
		localStorageMock._setStore({ 'orc_current_initiative_id': 'INIT-001' });
		const { currentInitiativeId } = await import('./initiative');
		expect(get(currentInitiativeId)).toBe('INIT-001');
	});

	it('URL param takes precedence over localStorage', async () => {
		localStorageMock._setStore({ 'orc_current_initiative_id': 'INIT-001' });
		locationMock.search = '?initiative=INIT-002';
		const { currentInitiativeId } = await import('./initiative');
		expect(get(currentInitiativeId)).toBe('INIT-002');
	});

	it('selectInitiative updates store and persists to localStorage', async () => {
		const { selectInitiative, currentInitiativeId } = await import('./initiative');

		selectInitiative('INIT-001');

		expect(get(currentInitiativeId)).toBe('INIT-001');
		expect(localStorageMock.setItem).toHaveBeenCalledWith('orc_current_initiative_id', 'INIT-001');
	});

	it('selectInitiative with null clears the filter', async () => {
		localStorageMock._setStore({ 'orc_current_initiative_id': 'INIT-001' });
		const { selectInitiative, currentInitiativeId } = await import('./initiative');

		selectInitiative(null);

		expect(get(currentInitiativeId)).toBe(null);
		expect(localStorageMock.removeItem).toHaveBeenCalledWith('orc_current_initiative_id');
	});

	it('initiatives store starts empty', async () => {
		const { initiatives } = await import('./initiative');
		expect(get(initiatives)).toEqual([]);
	});

	it('currentInitiative derives from initiatives and currentInitiativeId', async () => {
		const { initiatives, currentInitiativeId, currentInitiative } = await import('./initiative');

		const testInitiative: Initiative = {
			id: 'INIT-001',
			title: 'Test',
			status: 'active',
			version: 1,
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString()
		};

		initiatives.set([testInitiative]);
		currentInitiativeId.set('INIT-001');

		expect(get(currentInitiative)).toEqual(testInitiative);
	});

	it('currentInitiative returns null when no match', async () => {
		const { initiatives, currentInitiativeId, currentInitiative } = await import('./initiative');

		initiatives.set([]);
		currentInitiativeId.set('INIT-001');

		expect(get(currentInitiative)).toBe(null);
	});

	it('initiativeProgress computes completed/total correctly', async () => {
		const { initiatives, initiativeProgress } = await import('./initiative');

		const testInitiative: Initiative = {
			id: 'INIT-001',
			title: 'Test',
			status: 'active',
			version: 1,
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString(),
			tasks: [
				{ id: 'TASK-001', title: 'Task 1', status: 'completed' },
				{ id: 'TASK-002', title: 'Task 2', status: 'running' },
				{ id: 'TASK-003', title: 'Task 3', status: 'pending' }
			]
		};

		initiatives.set([testInitiative]);

		const progress = get(initiativeProgress);
		expect(progress.get('INIT-001')).toEqual({
			id: 'INIT-001',
			completed: 1,
			total: 3
		});
	});

	it('addInitiativeToStore adds new initiative', async () => {
		const { initiatives, addInitiativeToStore } = await import('./initiative');

		const testInitiative: Initiative = {
			id: 'INIT-001',
			title: 'Test',
			status: 'draft',
			version: 1,
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString()
		};

		addInitiativeToStore(testInitiative);

		expect(get(initiatives)).toHaveLength(1);
		expect(get(initiatives)[0].id).toBe('INIT-001');
	});

	it('addInitiativeToStore does not add duplicate', async () => {
		const { initiatives, addInitiativeToStore } = await import('./initiative');

		const testInitiative: Initiative = {
			id: 'INIT-001',
			title: 'Test',
			status: 'draft',
			version: 1,
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString()
		};

		addInitiativeToStore(testInitiative);
		addInitiativeToStore(testInitiative);

		expect(get(initiatives)).toHaveLength(1);
	});

	it('removeInitiativeFromStore removes initiative', async () => {
		const { initiatives, removeInitiativeFromStore } = await import('./initiative');

		const testInitiative: Initiative = {
			id: 'INIT-001',
			title: 'Test',
			status: 'draft',
			version: 1,
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString()
		};

		initiatives.set([testInitiative]);
		removeInitiativeFromStore('INIT-001');

		expect(get(initiatives)).toHaveLength(0);
	});

	it('removeInitiativeFromStore clears selection if selected', async () => {
		const { initiatives, currentInitiativeId, removeInitiativeFromStore } = await import('./initiative');

		const testInitiative: Initiative = {
			id: 'INIT-001',
			title: 'Test',
			status: 'draft',
			version: 1,
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString()
		};

		initiatives.set([testInitiative]);
		currentInitiativeId.set('INIT-001');
		removeInitiativeFromStore('INIT-001');

		expect(get(currentInitiativeId)).toBe(null);
	});

	it('updateInitiativeInStore updates existing initiative', async () => {
		const { initiatives, updateInitiativeInStore } = await import('./initiative');

		const testInitiative: Initiative = {
			id: 'INIT-001',
			title: 'Test',
			status: 'draft',
			version: 1,
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString()
		};

		initiatives.set([testInitiative]);
		updateInitiativeInStore('INIT-001', { title: 'Updated Title', status: 'active' });

		const updated = get(initiatives)[0];
		expect(updated.title).toBe('Updated Title');
		expect(updated.status).toBe('active');
	});

	it('exports UNASSIGNED_INITIATIVE constant', async () => {
		const { UNASSIGNED_INITIATIVE } = await import('./initiative');
		expect(UNASSIGNED_INITIATIVE).toBe('__unassigned__');
	});
});
