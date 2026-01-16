import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Preferences } from './Preferences';
import { usePreferencesStore, defaultPreferences } from '@/stores';

// Mock the date formatting to get consistent results
vi.mock('@/lib/formatDate', () => ({
	formatDate: vi.fn((date, format) => {
		if (!date) return 'Never';
		if (format === 'relative') return '3h ago';
		if (format === 'absolute') return 'Jan 16, 3:45 PM';
		if (format === 'absolute24') return 'Jan 16, 15:45';
		return String(date);
	}),
}));

describe('Preferences', () => {
	beforeEach(() => {
		// Reset store and localStorage
		localStorage.clear();
		usePreferencesStore.setState(defaultPreferences);
		document.documentElement.removeAttribute('data-theme');
	});

	const renderPreferences = () => {
		return render(
			<MemoryRouter>
				<Preferences />
			</MemoryRouter>
		);
	};

	describe('page rendering', () => {
		it('renders page title', () => {
			renderPreferences();
			expect(screen.getByRole('heading', { name: 'Preferences', level: 2 })).toBeInTheDocument();
		});

		it('renders description text', () => {
			renderPreferences();
			expect(screen.getByText(/customize your orc experience/i)).toBeInTheDocument();
		});

		it('renders all section headers', () => {
			renderPreferences();
			expect(screen.getByRole('heading', { name: /appearance/i })).toBeInTheDocument();
			expect(screen.getByRole('heading', { name: /layout/i })).toBeInTheDocument();
			expect(screen.getByRole('heading', { name: /date & time/i })).toBeInTheDocument();
		});
	});

	describe('theme controls', () => {
		it('renders Dark and Light theme buttons', () => {
			renderPreferences();
			expect(screen.getByRole('button', { name: /dark/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /light/i })).toBeInTheDocument();
		});

		it('shows Dark as selected by default', () => {
			renderPreferences();
			const darkBtn = screen.getByRole('button', { name: /dark/i });
			expect(darkBtn).toHaveAttribute('aria-pressed', 'true');
		});

		it('switches to Light theme when clicked', () => {
			renderPreferences();
			const lightBtn = screen.getByRole('button', { name: /light/i });

			fireEvent.click(lightBtn);

			expect(usePreferencesStore.getState().theme).toBe('light');
			expect(lightBtn).toHaveAttribute('aria-pressed', 'true');
		});

		it('applies theme to document', () => {
			renderPreferences();
			const lightBtn = screen.getByRole('button', { name: /light/i });

			fireEvent.click(lightBtn);

			expect(document.documentElement.getAttribute('data-theme')).toBe('light');
		});
	});

	describe('sidebar default controls', () => {
		it('renders Expanded and Collapsed buttons', () => {
			renderPreferences();
			expect(screen.getByRole('button', { name: /expanded/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /collapsed/i })).toBeInTheDocument();
		});

		it('shows Expanded as selected by default', () => {
			renderPreferences();
			const expandedBtn = screen.getByRole('button', { name: /expanded/i });
			expect(expandedBtn).toHaveAttribute('aria-pressed', 'true');
		});

		it('switches to Collapsed when clicked', () => {
			renderPreferences();
			const collapsedBtn = screen.getByRole('button', { name: /collapsed/i });

			fireEvent.click(collapsedBtn);

			expect(usePreferencesStore.getState().sidebarDefault).toBe('collapsed');
			expect(collapsedBtn).toHaveAttribute('aria-pressed', 'true');
		});
	});

	describe('board view mode controls', () => {
		it('renders Flat and Swimlane buttons', () => {
			renderPreferences();
			expect(screen.getByRole('button', { name: /^flat$/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /swimlane/i })).toBeInTheDocument();
		});

		it('shows Flat as selected by default', () => {
			renderPreferences();
			const flatBtn = screen.getByRole('button', { name: /^flat$/i });
			expect(flatBtn).toHaveAttribute('aria-pressed', 'true');
		});

		it('switches to Swimlane when clicked', () => {
			renderPreferences();
			const swimlaneBtn = screen.getByRole('button', { name: /swimlane/i });

			fireEvent.click(swimlaneBtn);

			expect(usePreferencesStore.getState().boardViewMode).toBe('swimlane');
			expect(swimlaneBtn).toHaveAttribute('aria-pressed', 'true');
		});
	});

	describe('date format controls', () => {
		it('renders date format dropdown', () => {
			renderPreferences();
			expect(screen.getByRole('combobox', { name: /date format/i })).toBeInTheDocument();
		});

		it('shows Relative as selected by default', () => {
			renderPreferences();
			const select = screen.getByRole('combobox', { name: /date format/i });
			expect(select).toHaveValue('relative');
		});

		it('has all three date format options', () => {
			renderPreferences();
			const select = screen.getByRole('combobox', { name: /date format/i });
			const options = select.querySelectorAll('option');

			expect(options).toHaveLength(3);
			expect(options[0]).toHaveValue('relative');
			expect(options[1]).toHaveValue('absolute');
			expect(options[2]).toHaveValue('absolute24');
		});

		it('changes date format when selected', () => {
			renderPreferences();
			const select = screen.getByRole('combobox', { name: /date format/i });

			fireEvent.change(select, { target: { value: 'absolute24' } });

			expect(usePreferencesStore.getState().dateFormat).toBe('absolute24');
		});

		it('shows date format preview', () => {
			renderPreferences();
			expect(screen.getByText(/preview/i)).toBeInTheDocument();
			expect(screen.getByText(/3 hours ago/i)).toBeInTheDocument();
			expect(screen.getByText(/5 days ago/i)).toBeInTheDocument();
		});
	});

	describe('reset to defaults', () => {
		it('renders reset button', () => {
			renderPreferences();
			expect(screen.getByRole('button', { name: /reset to defaults/i })).toBeInTheDocument();
		});

		it('resets all preferences when clicked', () => {
			// Set non-default values
			usePreferencesStore.getState().setTheme('light');
			usePreferencesStore.getState().setSidebarDefault('collapsed');
			usePreferencesStore.getState().setBoardViewMode('swimlane');
			usePreferencesStore.getState().setDateFormat('absolute24');

			renderPreferences();
			const resetBtn = screen.getByRole('button', { name: /reset to defaults/i });

			fireEvent.click(resetBtn);

			const state = usePreferencesStore.getState();
			expect(state.theme).toBe('dark');
			expect(state.sidebarDefault).toBe('expanded');
			expect(state.boardViewMode).toBe('flat');
			expect(state.dateFormat).toBe('relative');
		});

		it('updates UI after reset', () => {
			// Set non-default values
			usePreferencesStore.getState().setTheme('light');

			renderPreferences();
			const resetBtn = screen.getByRole('button', { name: /reset to defaults/i });

			fireEvent.click(resetBtn);

			// Dark button should now be selected
			const darkBtn = screen.getByRole('button', { name: /dark/i });
			expect(darkBtn).toHaveAttribute('aria-pressed', 'true');
		});
	});

	describe('accessibility', () => {
		it('has proper section landmarks', () => {
			renderPreferences();
			// Check for aria-labelledby on sections
			const sections = screen.getAllByRole('region');
			expect(sections).toHaveLength(3); // Appearance, Layout, Date & Time
		});

		it('toggle groups have proper role and labels', () => {
			renderPreferences();
			const themeGroup = screen.getByRole('group', { name: /theme selection/i });
			const sidebarGroup = screen.getByRole('group', { name: /sidebar default state/i });
			const boardGroup = screen.getByRole('group', { name: /board view mode/i });

			expect(themeGroup).toBeInTheDocument();
			expect(sidebarGroup).toBeInTheDocument();
			expect(boardGroup).toBeInTheDocument();
		});

		it('date format select has accessible label', () => {
			renderPreferences();
			const select = screen.getByRole('combobox', { name: /date format/i });
			expect(select).toBeInTheDocument();
		});
	});
});
