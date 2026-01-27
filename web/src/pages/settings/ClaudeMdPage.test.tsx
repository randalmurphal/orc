/**
 * Tests for ClaudeMdPage - CLAUDE.md editor with preview
 *
 * Success Criteria Coverage:
 * - SC-1: Editor loads/saves CLAUDE.md for selected scope (tests 1-4)
 * - SC-2: Split-view with editor and rendered markdown preview (tests 5-7)
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ClaudeMdPage } from './ClaudeMdPage';
import { configClient } from '@/lib/client';
import { SettingsScope } from '@/gen/orc/v1/config_pb';
import type { ClaudeMd } from '@/gen/orc/v1/config_pb';

// Mock the configClient
vi.mock('@/lib/client', () => ({
	configClient: {
		getClaudeMd: vi.fn(),
		updateClaudeMd: vi.fn(),
	},
}));

const mockProjectFile: Partial<ClaudeMd> = {
	path: '/project/CLAUDE.md',
	content: '# Project Instructions\n\nBuild quality software.\n\n## Rules\n\n- Write tests\n- Handle errors',
	scope: SettingsScope.PROJECT,
	$typeName: 'orc.v1.ClaudeMd' as const,
};

const mockGlobalFile: Partial<ClaudeMd> = {
	path: '/home/user/CLAUDE.md',
	content: '# Global Instructions\n\nGeneral guidelines.',
	scope: SettingsScope.GLOBAL,
	$typeName: 'orc.v1.ClaudeMd' as const,
};

describe('ClaudeMdPage', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	// ==========================================================================
	// SC-1: Editor loads CLAUDE.md for selected scope and saves changes via API
	// ==========================================================================

	describe('SC-1: Load and save CLAUDE.md', () => {
		it('fetches CLAUDE.md files on mount via getClaudeMd API', async () => {
			vi.mocked(configClient.getClaudeMd).mockResolvedValue({
				files: [mockProjectFile, mockGlobalFile] as ClaudeMd[],
				$typeName: 'orc.v1.GetClaudeMdResponse',
			});

			render(<ClaudeMdPage />);

			await waitFor(() => {
				expect(configClient.getClaudeMd).toHaveBeenCalledTimes(1);
			});
		});

		it('displays Project scope content by default when available', async () => {
			vi.mocked(configClient.getClaudeMd).mockResolvedValue({
				files: [mockProjectFile, mockGlobalFile] as ClaudeMd[],
				$typeName: 'orc.v1.GetClaudeMdResponse',
			});

			render(<ClaudeMdPage />);

			await waitFor(() => {
				// Editor should contain the project content
				const textarea = screen.getByTestId('config-editor-textarea');
				expect(textarea).toHaveValue(mockProjectFile.content);
			});
		});

		it('switches to Global scope content when Global tab is clicked', async () => {
			const user = userEvent.setup();
			vi.mocked(configClient.getClaudeMd).mockResolvedValue({
				files: [mockProjectFile, mockGlobalFile] as ClaudeMd[],
				$typeName: 'orc.v1.GetClaudeMdResponse',
			});

			render(<ClaudeMdPage />);

			// Wait for initial load
			await waitFor(() => {
				expect(screen.getByTestId('config-editor-textarea')).toBeInTheDocument();
			});

			// Click Global tab using userEvent for better Radix compatibility
			const globalTab = screen.getByRole('tab', { name: /global/i });
			await user.click(globalTab);

			// Editor should now show global content
			await waitFor(() => {
				const textarea = screen.getByTestId('config-editor-textarea');
				expect(textarea).toHaveValue(mockGlobalFile.content);
			});
		});

		it('calls updateClaudeMd API with correct scope when saving', async () => {
			vi.mocked(configClient.getClaudeMd).mockResolvedValue({
				files: [mockProjectFile, mockGlobalFile] as ClaudeMd[],
				$typeName: 'orc.v1.GetClaudeMdResponse',
			});
			vi.mocked(configClient.updateClaudeMd).mockResolvedValue({
				claudeMd: mockProjectFile as ClaudeMd,
				$typeName: 'orc.v1.UpdateClaudeMdResponse',
			});

			render(<ClaudeMdPage />);

			await waitFor(() => {
				expect(screen.getByTestId('config-editor-textarea')).toBeInTheDocument();
			});

			// Click save button
			const saveButton = screen.getByTestId('config-editor-save');
			fireEvent.click(saveButton);

			await waitFor(() => {
				expect(configClient.updateClaudeMd).toHaveBeenCalledWith(
					expect.objectContaining({
						scope: SettingsScope.PROJECT,
						content: mockProjectFile.content,
					})
				);
			});
		});
	});

	// ==========================================================================
	// SC-2: Split-view with ConfigEditor and rendered markdown preview
	// ==========================================================================

	describe('SC-2: Split-view with editor and preview', () => {
		it('renders both editor pane and preview pane', async () => {
			vi.mocked(configClient.getClaudeMd).mockResolvedValue({
				files: [mockProjectFile] as ClaudeMd[],
				$typeName: 'orc.v1.GetClaudeMdResponse',
			});

			render(<ClaudeMdPage />);

			await waitFor(() => {
				// Editor pane should exist
				expect(screen.getByTestId('claudemd-editor-pane')).toBeInTheDocument();
				// Preview pane should exist
				expect(screen.getByTestId('claudemd-preview-pane')).toBeInTheDocument();
			});
		});

		it('renders markdown headings as HTML h1/h2 elements in preview', async () => {
			vi.mocked(configClient.getClaudeMd).mockResolvedValue({
				files: [mockProjectFile] as ClaudeMd[],
				$typeName: 'orc.v1.GetClaudeMdResponse',
			});

			render(<ClaudeMdPage />);

			await waitFor(() => {
				const previewPane = screen.getByTestId('claudemd-preview-pane');
				// H1 should render "Project Instructions"
				const h1 = previewPane.querySelector('h1');
				expect(h1).toHaveTextContent('Project Instructions');
				// H2 should render "Rules"
				const h2 = previewPane.querySelector('h2');
				expect(h2).toHaveTextContent('Rules');
			});
		});

		it('renders markdown lists as HTML ul/li elements in preview', async () => {
			vi.mocked(configClient.getClaudeMd).mockResolvedValue({
				files: [mockProjectFile] as ClaudeMd[],
				$typeName: 'orc.v1.GetClaudeMdResponse',
			});

			render(<ClaudeMdPage />);

			await waitFor(() => {
				const previewPane = screen.getByTestId('claudemd-preview-pane');
				// Should have list items
				const listItems = previewPane.querySelectorAll('li');
				expect(listItems.length).toBeGreaterThan(0);
				expect(listItems[0]).toHaveTextContent('Write tests');
			});
		});
	});

	// ==========================================================================
	// Edge cases
	// ==========================================================================

	describe('Edge cases', () => {
		it('shows empty state when no CLAUDE.md files exist', async () => {
			vi.mocked(configClient.getClaudeMd).mockResolvedValue({
				files: [],
				$typeName: 'orc.v1.GetClaudeMdResponse',
			});

			render(<ClaudeMdPage />);

			await waitFor(() => {
				expect(screen.getByTestId('claudemd-empty-state')).toBeInTheDocument();
			});
		});

		it('handles API errors gracefully', async () => {
			vi.mocked(configClient.getClaudeMd).mockRejectedValue(new Error('Network error'));

			render(<ClaudeMdPage />);

			await waitFor(() => {
				expect(screen.getByTestId('claudemd-error')).toBeInTheDocument();
			});
		});
	});
});
