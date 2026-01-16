import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';

// Storage keys
const THEME_KEY = 'orc-theme';
const SIDEBAR_DEFAULT_KEY = 'orc-sidebar-default';
const BOARD_VIEW_MODE_KEY = 'orc-board-view-mode';
const DATE_FORMAT_KEY = 'orc-date-format';

// Types
export type Theme = 'dark' | 'light';
export type SidebarDefault = 'expanded' | 'collapsed';
export type BoardViewMode = 'flat' | 'swimlane';
export type DateFormat = 'relative' | 'absolute' | 'absolute24';

export interface Preferences {
	theme: Theme;
	sidebarDefault: SidebarDefault;
	boardViewMode: BoardViewMode;
	dateFormat: DateFormat;
}

export interface PreferencesStore extends Preferences {
	// Actions
	setTheme: (theme: Theme) => void;
	setSidebarDefault: (sidebarDefault: SidebarDefault) => void;
	setBoardViewMode: (boardViewMode: BoardViewMode) => void;
	setDateFormat: (dateFormat: DateFormat) => void;
	resetToDefaults: () => void;
}

// Default values
const defaultPreferences: Preferences = {
	theme: 'dark',
	sidebarDefault: 'expanded',
	boardViewMode: 'flat',
	dateFormat: 'relative',
};

// localStorage helpers
function getStoredTheme(): Theme {
	if (typeof window === 'undefined') return defaultPreferences.theme;
	try {
		const stored = localStorage.getItem(THEME_KEY);
		if (stored === 'light' || stored === 'dark') return stored;
		return defaultPreferences.theme;
	} catch {
		return defaultPreferences.theme;
	}
}

function setStoredTheme(theme: Theme): void {
	if (typeof window === 'undefined') return;
	try {
		localStorage.setItem(THEME_KEY, theme);
	} catch {
		// Ignore localStorage errors
	}
}

function getStoredSidebarDefault(): SidebarDefault {
	if (typeof window === 'undefined') return defaultPreferences.sidebarDefault;
	try {
		const stored = localStorage.getItem(SIDEBAR_DEFAULT_KEY);
		if (stored === 'expanded' || stored === 'collapsed') return stored;
		return defaultPreferences.sidebarDefault;
	} catch {
		return defaultPreferences.sidebarDefault;
	}
}

function setStoredSidebarDefault(sidebarDefault: SidebarDefault): void {
	if (typeof window === 'undefined') return;
	try {
		localStorage.setItem(SIDEBAR_DEFAULT_KEY, sidebarDefault);
	} catch {
		// Ignore localStorage errors
	}
}

function getStoredBoardViewMode(): BoardViewMode {
	if (typeof window === 'undefined') return defaultPreferences.boardViewMode;
	try {
		const stored = localStorage.getItem(BOARD_VIEW_MODE_KEY);
		if (stored === 'flat' || stored === 'swimlane') return stored;
		return defaultPreferences.boardViewMode;
	} catch {
		return defaultPreferences.boardViewMode;
	}
}

function setStoredBoardViewMode(boardViewMode: BoardViewMode): void {
	if (typeof window === 'undefined') return;
	try {
		localStorage.setItem(BOARD_VIEW_MODE_KEY, boardViewMode);
	} catch {
		// Ignore localStorage errors
	}
}

function getStoredDateFormat(): DateFormat {
	if (typeof window === 'undefined') return defaultPreferences.dateFormat;
	try {
		const stored = localStorage.getItem(DATE_FORMAT_KEY);
		if (stored === 'relative' || stored === 'absolute' || stored === 'absolute24') {
			return stored;
		}
		return defaultPreferences.dateFormat;
	} catch {
		return defaultPreferences.dateFormat;
	}
}

function setStoredDateFormat(dateFormat: DateFormat): void {
	if (typeof window === 'undefined') return;
	try {
		localStorage.setItem(DATE_FORMAT_KEY, dateFormat);
	} catch {
		// Ignore localStorage errors
	}
}

// Apply theme to document
function applyTheme(theme: Theme): void {
	if (typeof document === 'undefined') return;
	if (theme === 'light') {
		document.documentElement.setAttribute('data-theme', 'light');
	} else {
		document.documentElement.removeAttribute('data-theme');
	}
}

// Clear all preference storage keys
function clearStoredPreferences(): void {
	if (typeof window === 'undefined') return;
	try {
		localStorage.removeItem(THEME_KEY);
		localStorage.removeItem(SIDEBAR_DEFAULT_KEY);
		localStorage.removeItem(BOARD_VIEW_MODE_KEY);
		localStorage.removeItem(DATE_FORMAT_KEY);
	} catch {
		// Ignore localStorage errors
	}
}

export const usePreferencesStore = create<PreferencesStore>()(
	subscribeWithSelector((set) => {
		// Initialize with stored values
		const storedTheme = getStoredTheme();
		// Apply theme immediately on store creation
		applyTheme(storedTheme);

		return {
			theme: storedTheme,
			sidebarDefault: getStoredSidebarDefault(),
			boardViewMode: getStoredBoardViewMode(),
			dateFormat: getStoredDateFormat(),

			setTheme: (theme: Theme) => {
				setStoredTheme(theme);
				applyTheme(theme);
				set({ theme });
			},

			setSidebarDefault: (sidebarDefault: SidebarDefault) => {
				setStoredSidebarDefault(sidebarDefault);
				set({ sidebarDefault });
			},

			setBoardViewMode: (boardViewMode: BoardViewMode) => {
				setStoredBoardViewMode(boardViewMode);
				set({ boardViewMode });
			},

			setDateFormat: (dateFormat: DateFormat) => {
				setStoredDateFormat(dateFormat);
				set({ dateFormat });
			},

			resetToDefaults: () => {
				clearStoredPreferences();
				applyTheme(defaultPreferences.theme);
				set(defaultPreferences);
			},
		};
	})
);

// Selector hooks
export const useTheme = () => usePreferencesStore((state) => state.theme);
export const useSidebarDefault = () => usePreferencesStore((state) => state.sidebarDefault);
export const useBoardViewMode = () => usePreferencesStore((state) => state.boardViewMode);
export const useDateFormat = () => usePreferencesStore((state) => state.dateFormat);

// Export storage keys for testing
export const STORAGE_KEYS = {
	THEME: THEME_KEY,
	SIDEBAR_DEFAULT: SIDEBAR_DEFAULT_KEY,
	BOARD_VIEW_MODE: BOARD_VIEW_MODE_KEY,
	DATE_FORMAT: DATE_FORMAT_KEY,
} as const;

// Export defaults for testing
export { defaultPreferences };
