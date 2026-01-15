import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ClaudeMd } from './ClaudeMd';
import * as api from '@/lib/api';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	getClaudeMD: vi.fn(),
	updateClaudeMD: vi.fn(),
	getClaudeMDHierarchy: vi.fn(),
}));

describe('ClaudeMd', () => {
	const mockHierarchy: api.ClaudeMDHierarchy = {
		global: { content: '# Global instructions', path: '~/.claude/CLAUDE.md' },
		user: { content: '# User instructions', path: '~/CLAUDE.md' },
		project: { content: '# Project instructions', path: './CLAUDE.md' },
	};

	const mockClaudeMD = { content: '# Project instructions', path: './CLAUDE.md' };

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.getClaudeMDHierarchy).mockResolvedValue(mockHierarchy);
		vi.mocked(api.getClaudeMD).mockResolvedValue(mockClaudeMD);
	});

	const renderClaudeMd = (initialPath: string = '/environment/claudemd') => {
		return render(
			<MemoryRouter initialEntries={[initialPath]}>
				<Routes>
					<Route path="/environment/claudemd" element={<ClaudeMd />} />
				</Routes>
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading state initially', async () => {
			vi.mocked(api.getClaudeMDHierarchy).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve(mockHierarchy), 100)
					)
			);

			renderClaudeMd();
			expect(screen.getByText('Loading CLAUDE.md...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.getClaudeMDHierarchy).mockRejectedValue(
				new Error('Failed to load')
			);

			renderClaudeMd();

			await waitFor(() => {
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});
	});

	describe('header', () => {
		it('displays page title for project scope', async () => {
			renderClaudeMd();

			await waitFor(() => {
				expect(screen.getByText('CLAUDE.md')).toBeInTheDocument();
			});
		});

		it('displays subtitle for project scope', async () => {
			renderClaudeMd();

			await waitFor(() => {
				expect(
					screen.getByText('Project instructions for Claude')
				).toBeInTheDocument();
			});
		});

		it('displays global title when scope is global', async () => {
			renderClaudeMd('/environment/claudemd?scope=global');

			await waitFor(() => {
				expect(screen.getByRole('heading', { name: 'Global CLAUDE.md' })).toBeInTheDocument();
			});
		});

		it('displays user title when scope is user', async () => {
			renderClaudeMd('/environment/claudemd?scope=user');

			await waitFor(() => {
				expect(screen.getByRole('heading', { name: 'User CLAUDE.md' })).toBeInTheDocument();
			});
		});
	});

	describe('source selector', () => {
		it('shows all three source options', async () => {
			renderClaudeMd();

			await waitFor(() => {
				expect(screen.getByText('Global')).toBeInTheDocument();
				expect(screen.getByText('User')).toBeInTheDocument();
				expect(screen.getByText('Project')).toBeInTheDocument();
			});
		});

		it('shows source paths', async () => {
			renderClaudeMd();

			await waitFor(() => {
				expect(screen.getByText('~/.claude/CLAUDE.md')).toBeInTheDocument();
				expect(screen.getByText('~/CLAUDE.md')).toBeInTheDocument();
				expect(screen.getByText('./CLAUDE.md')).toBeInTheDocument();
			});
		});

		it('defaults to project selected', async () => {
			renderClaudeMd();

			await waitFor(() => {
				const projectButton = screen.getByRole('button', { name: /project/i });
				expect(projectButton).toHaveClass('selected');
			});
		});

		it('switches to global source when clicked', async () => {
			renderClaudeMd();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: /global/i }));
			});

			await waitFor(() => {
				expect(api.getClaudeMD).toHaveBeenCalledWith('global');
			});
		});

		it('switches to user source when clicked', async () => {
			renderClaudeMd();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: /user/i }));
			});

			await waitFor(() => {
				expect(api.getClaudeMD).toHaveBeenCalledWith('user');
			});
		});

		it('shows New badge for sources without content', async () => {
			vi.mocked(api.getClaudeMDHierarchy).mockResolvedValue({
				global: null,
				user: null,
				project: { content: '# test', path: './CLAUDE.md' },
			});

			renderClaudeMd();

			await waitFor(() => {
				const badges = screen.getAllByText('New');
				expect(badges.length).toBe(2); // Global and User don't have content
			});
		});
	});

	describe('editor', () => {
		it('displays content in textarea', async () => {
			renderClaudeMd();

			await waitFor(() => {
				const textarea = screen.getByRole('textbox');
				expect(textarea).toHaveValue('# Project instructions');
			});
		});

		it('updates content when user types', async () => {
			renderClaudeMd();

			await waitFor(() => {
				const textarea = screen.getByRole('textbox');
				fireEvent.change(textarea, { target: { value: '# Updated content' } });
				expect(textarea).toHaveValue('# Updated content');
			});
		});

		it('shows editor header with source label', async () => {
			renderClaudeMd();

			await waitFor(() => {
				expect(screen.getByText('Project (./CLAUDE.md)')).toBeInTheDocument();
			});
		});

		it('shows global label in editor header', async () => {
			renderClaudeMd('/environment/claudemd?scope=global');

			await waitFor(() => {
				expect(
					screen.getByText('Global (~/.claude/CLAUDE.md)')
				).toBeInTheDocument();
			});
		});
	});

	describe('save functionality', () => {
		it('shows save button', async () => {
			renderClaudeMd();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
			});
		});

		it('calls updateClaudeMD with correct params when save clicked', async () => {
			vi.mocked(api.updateClaudeMD).mockResolvedValue({});

			renderClaudeMd();

			await waitFor(() => {
				const textarea = screen.getByRole('textbox');
				fireEvent.change(textarea, { target: { value: '# New content' } });
				fireEvent.click(screen.getByRole('button', { name: 'Save' }));
			});

			await waitFor(() => {
				expect(api.updateClaudeMD).toHaveBeenCalledWith('# New content', 'project');
			});
		});

		it('shows Saving... text while saving', async () => {
			vi.mocked(api.updateClaudeMD).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve({}), 100)
					)
			);

			renderClaudeMd();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: 'Save' }));
			});

			expect(screen.getByText('Saving...')).toBeInTheDocument();
		});

		it('shows success message after save', async () => {
			vi.mocked(api.updateClaudeMD).mockResolvedValue({});

			renderClaudeMd();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: 'Save' }));
			});

			await waitFor(() => {
				expect(
					screen.getByText('CLAUDE.md saved successfully')
				).toBeInTheDocument();
			});
		});

		it('shows error message when save fails', async () => {
			vi.mocked(api.updateClaudeMD).mockRejectedValue(new Error('Save failed'));

			renderClaudeMd();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: 'Save' }));
			});

			await waitFor(() => {
				expect(screen.getByText('Save failed')).toBeInTheDocument();
			});
		});

		it('refreshes hierarchy after successful save', async () => {
			vi.mocked(api.updateClaudeMD).mockResolvedValue({});

			renderClaudeMd();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: 'Save' }));
			});

			await waitFor(() => {
				// getClaudeMDHierarchy called once on load and once after save
				expect(api.getClaudeMDHierarchy).toHaveBeenCalledTimes(2);
			});
		});
	});

	describe('help text', () => {
		it('shows hierarchy order explanation', async () => {
			renderClaudeMd();

			await waitFor(() => {
				expect(
					screen.getByText(/CLAUDE.md files are applied in order/i)
				).toBeInTheDocument();
			});
		});
	});
});
