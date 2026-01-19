import { useEffect, useRef } from 'react';
import { useSearchParams, useLocation } from 'react-router-dom';
import { useProjectStore, useInitiativeStore } from '@/stores';

/**
 * Synchronizes URL search parameters with Zustand stores.
 *
 * Bidirectional sync:
 * - URL changes (navigation, back/forward) -> update stores
 * - Store changes (programmatic) -> update URL
 *
 * Parameters handled:
 * - project: Project ID (on all routes)
 * - initiative: Initiative filter (on /, /board, /initiatives)
 * - dependency_status: Dependency filter (on /, /board, /initiatives)
 */
export function UrlParamSync() {
	const [searchParams, setSearchParams] = useSearchParams();
	const location = useLocation();

	// Store selectors
	const currentProjectId = useProjectStore((state) => state.currentProjectId);
	const selectProject = useProjectStore((state) => state.selectProject);
	const currentInitiativeId = useInitiativeStore((state) => state.currentInitiativeId);
	const selectInitiative = useInitiativeStore((state) => state.selectInitiative);

	// Track if we're syncing from URL to prevent loops
	const isSyncingFromUrl = useRef(false);
	// Track if we're syncing from store to prevent loops
	const isSyncingFromStore = useRef(false);

	// Get URL params
	const urlProjectId = searchParams.get('project');
	const urlInitiativeId = searchParams.get('initiative');

	// Routes that support initiative param
	const supportsInitiative = location.pathname === '/' || location.pathname === '/board' || location.pathname === '/initiatives';

	// URL -> Store sync
	// Only sync FROM URL when URL explicitly has the parameter
	// Don't overwrite store state with null when URL param is absent
	useEffect(() => {
		if (isSyncingFromStore.current) return;

		isSyncingFromUrl.current = true;

		// Sync project - only if URL has explicit project param
		// (don't reset store to null when URL param is absent)
		if (urlProjectId && urlProjectId !== currentProjectId) {
			selectProject(urlProjectId);
		}

		// Sync initiative (only on supported routes) - only if URL has explicit initiative param
		if (supportsInitiative && urlInitiativeId && urlInitiativeId !== currentInitiativeId) {
			selectInitiative(urlInitiativeId);
		}

		// Use setTimeout to ensure the flag is cleared after React's batch update
		setTimeout(() => {
			isSyncingFromUrl.current = false;
		}, 0);
	}, [
		urlProjectId,
		urlInitiativeId,
		currentProjectId,
		currentInitiativeId,
		selectProject,
		selectInitiative,
		supportsInitiative,
	]);

	// Store -> URL sync
	useEffect(() => {
		if (isSyncingFromUrl.current) return;

		isSyncingFromStore.current = true;

		const newParams = new URLSearchParams(searchParams);
		let changed = false;

		// Sync project to URL
		if (currentProjectId && currentProjectId !== urlProjectId) {
			newParams.set('project', currentProjectId);
			changed = true;
		} else if (!currentProjectId && urlProjectId) {
			newParams.delete('project');
			changed = true;
		}

		// Sync initiative to URL (only on supported routes)
		if (supportsInitiative) {
			if (currentInitiativeId && currentInitiativeId !== urlInitiativeId) {
				newParams.set('initiative', currentInitiativeId);
				changed = true;
			} else if (!currentInitiativeId && urlInitiativeId) {
				newParams.delete('initiative');
				changed = true;
			}
		}

		if (changed) {
			setSearchParams(newParams, { replace: true });
		}

		setTimeout(() => {
			isSyncingFromStore.current = false;
		}, 0);
	}, [
		currentProjectId,
		currentInitiativeId,
		searchParams,
		setSearchParams,
		urlProjectId,
		urlInitiativeId,
		supportsInitiative,
	]);

	// Render nothing - this is a sync-only component
	return null;
}
