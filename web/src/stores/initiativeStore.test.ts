import { describe, it, expect, beforeEach } from 'vitest';
import {
	useInitiativeStore,
	UNASSIGNED_INITIATIVE,
	truncateInitiativeTitle,
} from './initiativeStore';
import { resetUrlMocks, setMockSearch } from '../test-setup';
import type { Initiative, Task } from '@/lib/types';

// Factory for creating test initiatives
function createInitiative(overrides: Partial<Initiative> = {}): Initiative {
	return {
		version: 1,
		id: `INIT-${Math.random().toString(36).slice(2, 7)}`,
		title: 'Test Initiative',
		status: 'active',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString(),
		...overrides,
	};
}

// Factory for creating test tasks
function createTask(overrides: Partial<Task> = {}): Task {
	return {
		id: `TASK-${Math.random().toString(36).slice(2, 7)}`,
		title: 'Test Task',
		weight: 'medium',
		status: 'planned',
		branch: 'main',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString(),
		...overrides,
	};
}

describe('InitiativeStore', () => {
	beforeEach(() => {
		// Reset store and mocks before each test
		useInitiativeStore.getState().reset();
		resetUrlMocks();
		localStorage.clear();
	});

	describe('setInitiatives', () => {
		it('should set initiatives as a Map', () => {
			const initiatives = [
				createInitiative({ id: 'INIT-001', title: 'Initiative 1' }),
				createInitiative({ id: 'INIT-002', title: 'Initiative 2' }),
			];

			useInitiativeStore.getState().setInitiatives(initiatives);

			expect(useInitiativeStore.getState().initiatives.size).toBe(2);
			expect(useInitiativeStore.getState().initiatives.get('INIT-001')?.title).toBe(
				'Initiative 1'
			);
		});

		it('should set hasLoaded to true', () => {
			useInitiativeStore.getState().setInitiatives([]);

			expect(useInitiativeStore.getState().hasLoaded).toBe(true);
		});

		it('should clear invalid current selection', () => {
			useInitiativeStore.setState({ currentInitiativeId: 'invalid-id' });
			const initiatives = [createInitiative({ id: 'INIT-001' })];

			useInitiativeStore.getState().setInitiatives(initiatives);

			expect(useInitiativeStore.getState().currentInitiativeId).toBeNull();
		});

		it('should keep UNASSIGNED_INITIATIVE selection (always valid)', () => {
			useInitiativeStore.setState({ currentInitiativeId: UNASSIGNED_INITIATIVE });
			const initiatives = [createInitiative({ id: 'INIT-001' })];

			useInitiativeStore.getState().setInitiatives(initiatives);

			expect(useInitiativeStore.getState().currentInitiativeId).toBe(UNASSIGNED_INITIATIVE);
		});

		it('should keep valid current selection', () => {
			useInitiativeStore.setState({ currentInitiativeId: 'INIT-002' });
			const initiatives = [
				createInitiative({ id: 'INIT-001' }),
				createInitiative({ id: 'INIT-002' }),
			];

			useInitiativeStore.getState().setInitiatives(initiatives);

			expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-002');
		});
	});

	describe('addInitiative', () => {
		it('should add initiative to map', () => {
			const initiative = createInitiative({ id: 'INIT-001' });

			useInitiativeStore.getState().addInitiative(initiative);

			expect(useInitiativeStore.getState().initiatives.has('INIT-001')).toBe(true);
		});
	});

	describe('updateInitiative', () => {
		it('should update initiative properties', () => {
			const initiative = createInitiative({ id: 'INIT-001', title: 'Original' });
			useInitiativeStore.getState().addInitiative(initiative);

			useInitiativeStore.getState().updateInitiative('INIT-001', { title: 'Updated' });

			expect(useInitiativeStore.getState().initiatives.get('INIT-001')?.title).toBe('Updated');
		});

		it('should not modify state if initiative not found', () => {
			const initiative = createInitiative({ id: 'INIT-001' });
			useInitiativeStore.getState().addInitiative(initiative);
			const originalSize = useInitiativeStore.getState().initiatives.size;

			useInitiativeStore.getState().updateInitiative('INIT-999', { title: 'Updated' });

			expect(useInitiativeStore.getState().initiatives.size).toBe(originalSize);
		});
	});

	describe('removeInitiative', () => {
		it('should remove initiative from map', () => {
			const initiatives = [
				createInitiative({ id: 'INIT-001' }),
				createInitiative({ id: 'INIT-002' }),
			];
			useInitiativeStore.getState().setInitiatives(initiatives);

			useInitiativeStore.getState().removeInitiative('INIT-001');

			expect(useInitiativeStore.getState().initiatives.has('INIT-001')).toBe(false);
			expect(useInitiativeStore.getState().initiatives.has('INIT-002')).toBe(true);
		});

		it('should clear selection if removed initiative was selected', () => {
			const initiatives = [
				createInitiative({ id: 'INIT-001' }),
				createInitiative({ id: 'INIT-002' }),
			];
			useInitiativeStore.getState().setInitiatives(initiatives);
			useInitiativeStore.getState().selectInitiative('INIT-001');

			useInitiativeStore.getState().removeInitiative('INIT-001');

			expect(useInitiativeStore.getState().currentInitiativeId).toBeNull();
		});
	});

	describe('selectInitiative', () => {
		it('should update currentInitiativeId', () => {
			useInitiativeStore.getState().selectInitiative('INIT-001');

			expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-001');
		});

		it('should allow UNASSIGNED_INITIATIVE special value', () => {
			useInitiativeStore.getState().selectInitiative(UNASSIGNED_INITIATIVE);

			expect(useInitiativeStore.getState().currentInitiativeId).toBe(UNASSIGNED_INITIATIVE);
		});

		it('should allow null (show all)', () => {
			useInitiativeStore.getState().selectInitiative('INIT-001');
			useInitiativeStore.getState().selectInitiative(null);

			expect(useInitiativeStore.getState().currentInitiativeId).toBeNull();
		});

		it('should sync to localStorage', () => {
			useInitiativeStore.getState().selectInitiative('INIT-001');

			expect(localStorage.getItem('orc_current_initiative_id')).toBe('INIT-001');
		});

		it('should push to browser history', () => {
			useInitiativeStore.getState().selectInitiative('INIT-001');

			expect(window.history.pushState).toHaveBeenCalled();
		});
	});

	describe('handlePopState', () => {
		it('should update selection from event state', () => {
			const event = new PopStateEvent('popstate', {
				state: { initiativeId: 'INIT-002' },
			});

			useInitiativeStore.getState().handlePopState(event);

			expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-002');
		});

		it('should fall back to URL param if event state is empty', () => {
			setMockSearch('?initiative=INIT-003');
			const event = new PopStateEvent('popstate', { state: null });

			useInitiativeStore.getState().handlePopState(event);

			expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-003');
		});
	});

	describe('initializeFromUrl', () => {
		it('should initialize from URL param', () => {
			setMockSearch('?initiative=INIT-url');

			useInitiativeStore.getState().initializeFromUrl();

			expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-url');
		});

		it('should fall back to localStorage if URL param is missing', () => {
			localStorage.setItem('orc_current_initiative_id', 'INIT-stored');

			useInitiativeStore.getState().initializeFromUrl();

			expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-stored');
		});

		it('should support UNASSIGNED_INITIATIVE from URL', () => {
			setMockSearch(`?initiative=${UNASSIGNED_INITIATIVE}`);

			useInitiativeStore.getState().initializeFromUrl();

			expect(useInitiativeStore.getState().currentInitiativeId).toBe(UNASSIGNED_INITIATIVE);
		});
	});

	describe('getInitiativesList', () => {
		it('should return initiatives as array', () => {
			const initiatives = [
				createInitiative({ id: 'INIT-001' }),
				createInitiative({ id: 'INIT-002' }),
			];
			useInitiativeStore.getState().setInitiatives(initiatives);

			const list = useInitiativeStore.getState().getInitiativesList();

			expect(list).toHaveLength(2);
			expect(Array.isArray(list)).toBe(true);
		});
	});

	describe('getCurrentInitiative', () => {
		it('should return current initiative by ID', () => {
			const initiatives = [
				createInitiative({ id: 'INIT-001', title: 'Initiative 1' }),
				createInitiative({ id: 'INIT-002', title: 'Initiative 2' }),
			];
			useInitiativeStore.getState().setInitiatives(initiatives);
			useInitiativeStore.setState({ currentInitiativeId: 'INIT-002' });

			const current = useInitiativeStore.getState().getCurrentInitiative();

			expect(current?.title).toBe('Initiative 2');
		});

		it('should return undefined for UNASSIGNED_INITIATIVE', () => {
			const initiatives = [createInitiative({ id: 'INIT-001' })];
			useInitiativeStore.getState().setInitiatives(initiatives);
			useInitiativeStore.setState({ currentInitiativeId: UNASSIGNED_INITIATIVE });

			const current = useInitiativeStore.getState().getCurrentInitiative();

			expect(current).toBeUndefined();
		});

		it('should return undefined when no selection', () => {
			const initiatives = [createInitiative({ id: 'INIT-001' })];
			useInitiativeStore.getState().setInitiatives(initiatives);
			useInitiativeStore.setState({ currentInitiativeId: null });

			const current = useInitiativeStore.getState().getCurrentInitiative();

			expect(current).toBeUndefined();
		});
	});

	describe('getInitiativeProgress', () => {
		it('should count tasks per initiative', () => {
			const tasks = [
				createTask({ id: 'TASK-001', initiative_id: 'INIT-001', status: 'completed' }),
				createTask({ id: 'TASK-002', initiative_id: 'INIT-001', status: 'running' }),
				createTask({ id: 'TASK-003', initiative_id: 'INIT-001', status: 'finished' }),
				createTask({ id: 'TASK-004', initiative_id: 'INIT-002', status: 'completed' }),
			];

			const progress = useInitiativeStore.getState().getInitiativeProgress(tasks);

			expect(progress.get('INIT-001')).toEqual({
				id: 'INIT-001',
				completed: 2, // completed + finished
				total: 3,
			});
			expect(progress.get('INIT-002')).toEqual({
				id: 'INIT-002',
				completed: 1,
				total: 1,
			});
		});

		it('should skip tasks without initiative_id', () => {
			const tasks = [
				createTask({ id: 'TASK-001', initiative_id: 'INIT-001', status: 'completed' }),
				createTask({ id: 'TASK-002', status: 'running' }), // No initiative_id
			];

			const progress = useInitiativeStore.getState().getInitiativeProgress(tasks);

			expect(progress.size).toBe(1);
			expect(progress.has('INIT-001')).toBe(true);
		});
	});

	describe('getInitiative', () => {
		it('should return initiative by ID', () => {
			const initiatives = [createInitiative({ id: 'INIT-001', title: 'Test' })];
			useInitiativeStore.getState().setInitiatives(initiatives);

			const initiative = useInitiativeStore.getState().getInitiative('INIT-001');

			expect(initiative?.title).toBe('Test');
		});

		it('should return undefined for non-existent ID', () => {
			const initiative = useInitiativeStore.getState().getInitiative('INIT-999');

			expect(initiative).toBeUndefined();
		});
	});

	describe('getInitiativeTitle', () => {
		it('should return title for existing initiative', () => {
			const initiatives = [createInitiative({ id: 'INIT-001', title: 'My Initiative' })];
			useInitiativeStore.getState().setInitiatives(initiatives);

			const title = useInitiativeStore.getState().getInitiativeTitle('INIT-001');

			expect(title).toBe('My Initiative');
		});

		it('should return ID as fallback for non-existent initiative', () => {
			const title = useInitiativeStore.getState().getInitiativeTitle('INIT-999');

			expect(title).toBe('INIT-999');
		});
	});

	describe('truncateInitiativeTitle helper', () => {
		it('should not truncate short titles', () => {
			expect(truncateInitiativeTitle('Short', 20)).toBe('Short');
		});

		it('should truncate long titles with ellipsis', () => {
			const result = truncateInitiativeTitle('This is a very long initiative title', 20);

			expect(result).toBe('This is a very long…');
			expect(result.length).toBe(20);
		});

		it('should respect custom max length', () => {
			const result = truncateInitiativeTitle('Medium length title', 10);

			expect(result).toBe('Medium le…');
			expect(result.length).toBe(10);
		});
	});

	describe('loading and error states', () => {
		it('should set loading state', () => {
			useInitiativeStore.getState().setLoading(true);
			expect(useInitiativeStore.getState().loading).toBe(true);
		});

		it('should set error state', () => {
			useInitiativeStore.getState().setError('Failed to load');
			expect(useInitiativeStore.getState().error).toBe('Failed to load');
		});

		it('should set hasLoaded state', () => {
			useInitiativeStore.getState().setHasLoaded(true);
			expect(useInitiativeStore.getState().hasLoaded).toBe(true);
		});
	});

	describe('reset', () => {
		it('should reset store to initial state', () => {
			const initiatives = [createInitiative({ id: 'INIT-001' })];
			useInitiativeStore.getState().setInitiatives(initiatives);
			useInitiativeStore.getState().selectInitiative('INIT-001');
			useInitiativeStore.getState().setLoading(true);
			useInitiativeStore.getState().setError('error');

			useInitiativeStore.getState().reset();

			expect(useInitiativeStore.getState().initiatives.size).toBe(0);
			expect(useInitiativeStore.getState().currentInitiativeId).toBeNull();
			expect(useInitiativeStore.getState().hasLoaded).toBe(false);
			expect(useInitiativeStore.getState().loading).toBe(false);
			expect(useInitiativeStore.getState().error).toBeNull();
		});
	});
});

describe('UNASSIGNED_INITIATIVE constant', () => {
	it('should be a special string value', () => {
		expect(UNASSIGNED_INITIATIVE).toBe('__unassigned__');
	});
});
