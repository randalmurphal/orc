import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';
import {
	initiatives,
	initiativesList,
	initiativesLoading,
	initiativesError,
	getInitiativeFromStore,
	getInitiativeTitle,
	truncateInitiativeTitle,
	getInitiativeBadgeTitle,
	updateInitiative,
	addInitiative,
	removeInitiative,
	resetInitiatives
} from './initiatives';
import type { Initiative } from '$lib/types';

// Mock api module
vi.mock('$lib/api', () => ({
	listInitiatives: vi.fn()
}));

describe('initiatives store', () => {
	const mockInitiative: Initiative = {
		id: 'INIT-001',
		title: 'Frontend Redesign',
		status: 'active',
		version: 1,
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	};

	const mockInitiative2: Initiative = {
		id: 'INIT-002',
		title: 'API Performance Improvements',
		status: 'draft',
		version: 1,
		created_at: '2024-01-02T00:00:00Z',
		updated_at: '2024-01-02T00:00:00Z'
	};

	beforeEach(() => {
		resetInitiatives();
		vi.clearAllMocks();
	});

	describe('truncateInitiativeTitle', () => {
		it('returns full title if within maxLength', () => {
			expect(truncateInitiativeTitle('Short', 12)).toBe('Short');
			expect(truncateInitiativeTitle('ExactLength!', 12)).toBe('ExactLength!');
		});

		it('truncates and adds ellipsis if too long', () => {
			expect(truncateInitiativeTitle('This is a very long title', 12)).toBe('This is a v\u2026');
			expect(truncateInitiativeTitle('Frontend Redesign', 12)).toBe('Frontend Re\u2026');
		});

		it('uses custom maxLength', () => {
			expect(truncateInitiativeTitle('Long title here', 5)).toBe('Long\u2026');
		});
	});

	describe('getInitiativeTitle', () => {
		it('returns initiative title if found', () => {
			addInitiative(mockInitiative);
			expect(getInitiativeTitle('INIT-001')).toBe('Frontend Redesign');
		});

		it('returns ID if initiative not found', () => {
			expect(getInitiativeTitle('INIT-999')).toBe('INIT-999');
		});
	});

	describe('getInitiativeBadgeTitle', () => {
		it('returns display and full title when initiative exists', () => {
			addInitiative(mockInitiative);
			const result = getInitiativeBadgeTitle('INIT-001');
			expect(result.full).toBe('Frontend Redesign');
			expect(result.display).toBe('Frontend Re\u2026');
		});

		it('returns ID for both when initiative not found', () => {
			const result = getInitiativeBadgeTitle('INIT-999');
			expect(result.full).toBe('INIT-999');
			expect(result.display).toBe('INIT-999');
		});

		it('uses custom maxLength', () => {
			addInitiative(mockInitiative2);
			const result = getInitiativeBadgeTitle('INIT-002', 20);
			expect(result.full).toBe('API Performance Improvements');
			expect(result.display).toBe('API Performance Imp\u2026');
		});
	});

	describe('addInitiative', () => {
		it('adds initiative to store', () => {
			addInitiative(mockInitiative);
			expect(get(initiatives).get('INIT-001')).toEqual(mockInitiative);
			expect(get(initiativesList)).toHaveLength(1);
		});

		it('can add multiple initiatives', () => {
			addInitiative(mockInitiative);
			addInitiative(mockInitiative2);
			expect(get(initiatives).size).toBe(2);
			expect(get(initiativesList)).toHaveLength(2);
		});
	});

	describe('updateInitiative', () => {
		it('updates existing initiative', () => {
			addInitiative(mockInitiative);
			updateInitiative('INIT-001', { status: 'completed' });

			const updated = get(initiatives).get('INIT-001');
			expect(updated?.status).toBe('completed');
			expect(updated?.title).toBe('Frontend Redesign');
		});

		it('does nothing if initiative not found', () => {
			addInitiative(mockInitiative);
			updateInitiative('INIT-999', { status: 'completed' });
			expect(get(initiatives).size).toBe(1);
		});
	});

	describe('removeInitiative', () => {
		it('removes initiative from store', () => {
			addInitiative(mockInitiative);
			addInitiative(mockInitiative2);

			removeInitiative('INIT-001');

			expect(get(initiatives).has('INIT-001')).toBe(false);
			expect(get(initiatives).has('INIT-002')).toBe(true);
			expect(get(initiativesList)).toHaveLength(1);
		});

		it('does nothing if initiative not found', () => {
			addInitiative(mockInitiative);
			removeInitiative('INIT-999');
			expect(get(initiatives).size).toBe(1);
		});
	});

	describe('getInitiativeFromStore', () => {
		it('returns initiative if found', () => {
			addInitiative(mockInitiative);
			expect(getInitiativeFromStore('INIT-001')).toEqual(mockInitiative);
		});

		it('returns undefined if not found', () => {
			expect(getInitiativeFromStore('INIT-999')).toBeUndefined();
		});
	});

	describe('resetInitiatives', () => {
		it('clears all initiatives', () => {
			addInitiative(mockInitiative);
			addInitiative(mockInitiative2);

			resetInitiatives();

			expect(get(initiatives).size).toBe(0);
			expect(get(initiativesList)).toHaveLength(0);
			expect(get(initiativesError)).toBeNull();
		});
	});

	describe('initiativesList derived store', () => {
		it('returns initiatives as array', () => {
			addInitiative(mockInitiative);
			addInitiative(mockInitiative2);

			const list = get(initiativesList);
			expect(list).toHaveLength(2);
			expect(list.map(i => i.id).sort()).toEqual(['INIT-001', 'INIT-002']);
		});
	});
});
