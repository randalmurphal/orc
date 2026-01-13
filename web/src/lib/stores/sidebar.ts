import { writable } from 'svelte/store';
import { browser } from '$app/environment';

const STORAGE_KEY = 'orc-sidebar-expanded';

function createSidebarStore() {
	// Initialize from localStorage if available, default to expanded
	const initialValue = browser
		? localStorage.getItem(STORAGE_KEY) !== 'false' // Default to true (expanded)
		: true;

	const { subscribe, set, update } = writable(initialValue);

	return {
		subscribe,
		toggle: () => {
			update((expanded) => {
				const newValue = !expanded;
				if (browser) {
					localStorage.setItem(STORAGE_KEY, String(newValue));
				}
				return newValue;
			});
		},
		set: (value: boolean) => {
			if (browser) {
				localStorage.setItem(STORAGE_KEY, String(value));
			}
			set(value);
		}
	};
}

export const sidebarExpanded = createSidebarStore();

// Keep the old export name for backwards compatibility during migration
// TODO: Remove this after all imports are updated
export const sidebarPinned = sidebarExpanded;
