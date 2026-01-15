import { describe, it, expect, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useSearchParams } from 'react-router-dom';
import { UrlParamSync } from './UrlParamSync';
import { useProjectStore, useInitiativeStore } from '@/stores';

// Test component that displays current search params
function ParamsDisplay() {
	const [searchParams] = useSearchParams();
	return (
		<div data-testid="params">
			<span data-testid="project">{searchParams.get('project') ?? 'null'}</span>
			<span data-testid="initiative">{searchParams.get('initiative') ?? 'null'}</span>
		</div>
	);
}

function TestWrapper({
	initialPath = '/',
	children,
}: {
	initialPath?: string;
	children?: React.ReactNode;
}) {
	return (
		<MemoryRouter initialEntries={[initialPath]}>
			<Routes>
				<Route
					path="/*"
					element={
						<>
							<UrlParamSync />
							<ParamsDisplay />
							{children}
						</>
					}
				/>
			</Routes>
		</MemoryRouter>
	);
}

describe('UrlParamSync', () => {
	beforeEach(() => {
		// Reset stores to initial state
		useProjectStore.setState({
			projects: [],
			currentProjectId: null,
			loading: false,
			error: null,
			_isHandlingPopState: false,
		});
		useInitiativeStore.setState({
			initiatives: new Map(),
			currentInitiativeId: null,
			loading: false,
			error: null,
			hasLoaded: false,
			_isHandlingPopState: false,
		});
	});

	describe('URL -> Store sync', () => {
		it('syncs project param from URL to store', async () => {
			render(<TestWrapper initialPath="/?project=test-project" />);

			await waitFor(() => {
				expect(useProjectStore.getState().currentProjectId).toBe('test-project');
			});
		});

		it('syncs initiative param from URL to store on root route', async () => {
			render(<TestWrapper initialPath="/?initiative=INIT-001" />);

			await waitFor(() => {
				expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-001');
			});
		});

		it('syncs initiative param from URL to store on board route', async () => {
			render(<TestWrapper initialPath="/board?initiative=INIT-002" />);

			await waitFor(() => {
				expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-002');
			});
		});

		it('syncs multiple params from URL', async () => {
			render(<TestWrapper initialPath="/?project=proj-1&initiative=INIT-001" />);

			await waitFor(() => {
				expect(useProjectStore.getState().currentProjectId).toBe('proj-1');
				expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-001');
			});
		});
	});

	describe('Store state reflects URL params', () => {
		it('store currentProjectId matches URL project param', async () => {
			render(<TestWrapper initialPath="/?project=my-project" />);

			await waitFor(() => {
				const storeId = useProjectStore.getState().currentProjectId;
				expect(storeId).toBe('my-project');
			});
		});

		it('store currentInitiativeId matches URL initiative param', async () => {
			render(<TestWrapper initialPath="/?initiative=INIT-TEST" />);

			await waitFor(() => {
				const storeId = useInitiativeStore.getState().currentInitiativeId;
				expect(storeId).toBe('INIT-TEST');
			});
		});

		it('handles empty URL params', async () => {
			render(<TestWrapper initialPath="/" />);

			await waitFor(() => {
				expect(useProjectStore.getState().currentProjectId).toBeNull();
				expect(useInitiativeStore.getState().currentInitiativeId).toBeNull();
			});
		});
	});

	describe('Initiative sync restrictions', () => {
		it('does not sync initiative on dashboard route', async () => {
			render(<TestWrapper initialPath="/dashboard?initiative=INIT-001" />);

			// Wait a bit to ensure no sync happens
			await new Promise((resolve) => setTimeout(resolve, 50));

			// Dashboard doesn't support initiative param, so store should remain null
			expect(useInitiativeStore.getState().currentInitiativeId).toBeNull();
		});

		it('syncs initiative on root route', async () => {
			render(<TestWrapper initialPath="/?initiative=INIT-001" />);

			await waitFor(() => {
				expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-001');
			});
		});

		it('syncs initiative on board route', async () => {
			render(<TestWrapper initialPath="/board?initiative=INIT-001" />);

			await waitFor(() => {
				expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-001');
			});
		});
	});

	describe('Deep linking support', () => {
		it('correctly initializes store from deep link URL', async () => {
			// Simulate opening a deep link with multiple params
			render(<TestWrapper initialPath="/?project=deep-project&initiative=INIT-DEEP" />);

			await waitFor(() => {
				expect(useProjectStore.getState().currentProjectId).toBe('deep-project');
				expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-DEEP');
			});
		});

		it('handles task detail route params', async () => {
			// Task detail doesn't sync project/initiative from URL to store
			// but the URL params should be readable by the page
			render(<TestWrapper initialPath="/tasks/TASK-001?tab=transcript" />);

			// UrlParamSync doesn't handle tab param, but it shouldn't interfere
			await waitFor(() => {
				// Store should not be affected by task detail route
				expect(useProjectStore.getState().currentProjectId).toBeNull();
			});
		});
	});
});
