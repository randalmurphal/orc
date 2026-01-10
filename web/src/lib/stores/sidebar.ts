import { writable } from 'svelte/store';
import { browser } from '$app/environment';

const STORAGE_KEY = 'orc-sidebar-pinned';

function createSidebarStore() {
	// Initialize from localStorage if available
	const initialValue = browser
		? localStorage.getItem(STORAGE_KEY) === 'true'
		: false;

	const { subscribe, set, update } = writable(initialValue);

	return {
		subscribe,
		toggle: () => {
			update((pinned) => {
				const newValue = !pinned;
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

export const sidebarPinned = createSidebarStore();
