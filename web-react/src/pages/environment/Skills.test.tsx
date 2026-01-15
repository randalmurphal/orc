import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { Skills } from './Skills';
import * as api from '@/lib/api';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	listSkills: vi.fn(),
	getSkill: vi.fn(),
	createSkill: vi.fn(),
	updateSkill: vi.fn(),
	deleteSkill: vi.fn(),
}));

describe('Skills', () => {
	const mockSkillsList: api.SkillInfo[] = [
		{ name: 'python-style', description: 'Python coding standards', path: '.claude/skills/python-style' },
		{ name: 'testing', description: 'Testing best practices', path: '.claude/skills/testing' },
	];

	const mockSkill: api.Skill = {
		name: 'python-style',
		description: 'Python coding standards',
		content: '# Python Style Guide\n\nFollow PEP8...',
		allowed_tools: ['Read', 'Edit'],
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.listSkills).mockResolvedValue(mockSkillsList);
		vi.mocked(api.getSkill).mockResolvedValue(mockSkill);
	});

	const renderSkills = (initialPath: string = '/environment/skills') => {
		return render(
			<MemoryRouter initialEntries={[initialPath]}>
				<Routes>
					<Route path="/environment/skills" element={<Skills />} />
				</Routes>
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading state initially', async () => {
			vi.mocked(api.listSkills).mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(() => resolve(mockSkillsList), 100)
					)
			);

			renderSkills();
			expect(screen.getByText('Loading skills...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.listSkills).mockRejectedValue(new Error('Failed to load'));

			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});
	});

	describe('header', () => {
		it('displays page title for project scope', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('Claude Code Skills')).toBeInTheDocument();
			});
		});

		it('displays global title when scope is global', async () => {
			renderSkills('/environment/skills?scope=global');

			await waitFor(() => {
				expect(screen.getByText('Global Claude Code Skills')).toBeInTheDocument();
			});
		});

		it('displays subtitle with path', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByText(/manage skills in .claude\/skills\//i)).toBeInTheDocument();
			});
		});

		it('shows New Skill button', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Skill' })).toBeInTheDocument();
			});
		});
	});

	describe('scope toggle', () => {
		it('shows Project and Global links', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByRole('link', { name: 'Project' })).toBeInTheDocument();
				expect(screen.getByRole('link', { name: 'Global' })).toBeInTheDocument();
			});
		});

		it('highlights Project when no scope param', async () => {
			renderSkills();

			await waitFor(() => {
				const projectLink = screen.getByRole('link', { name: 'Project' });
				expect(projectLink).toHaveClass('active');
			});
		});

		it('highlights Global when scope=global', async () => {
			renderSkills('/environment/skills?scope=global');

			await waitFor(() => {
				const globalLink = screen.getByRole('link', { name: 'Global' });
				expect(globalLink).toHaveClass('active');
			});
		});
	});

	describe('skill list', () => {
		it('displays list of skills', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
				expect(screen.getByText('testing')).toBeInTheDocument();
			});
		});

		it('shows skill descriptions', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('Python coding standards')).toBeInTheDocument();
				expect(screen.getByText('Testing best practices')).toBeInTheDocument();
			});
		});

		it('shows empty message when no skills', async () => {
			vi.mocked(api.listSkills).mockResolvedValue([]);

			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('No skills configured')).toBeInTheDocument();
			});
		});
	});

	describe('selecting a skill', () => {
		it('loads skill details when clicked', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('python-style'));

			await waitFor(() => {
				expect(api.getSkill).toHaveBeenCalledWith('python-style', undefined);
			});
		});

		it('loads skill with global scope param', async () => {
			renderSkills('/environment/skills?scope=global');

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('python-style'));

			await waitFor(() => {
				expect(api.getSkill).toHaveBeenCalledWith('python-style', 'global');
			});
		});

		it('shows skill form with populated fields', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('python-style'));

			await waitFor(() => {
				expect(screen.getByDisplayValue('python-style')).toBeInTheDocument();
				expect(screen.getByDisplayValue('Python coding standards')).toBeInTheDocument();
				expect(screen.getByDisplayValue('Read, Edit')).toBeInTheDocument();
			});
		});

		it('shows Delete button for selected skill', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('python-style'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});
		});

		it('highlights selected skill in list', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('python-style'));

			// Wait for the API call to complete and state to update
			await waitFor(() => {
				expect(api.getSkill).toHaveBeenCalledWith('python-style', undefined);
			});

			await waitFor(() => {
				const skillButton = screen.getByRole('button', { name: /python-style/i });
				expect(skillButton).toHaveClass('selected');
			});
		});
	});

	describe('creating a skill', () => {
		it('shows empty form when New Skill clicked', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Skill' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Skill' }));

			await waitFor(() => {
				expect(screen.getByRole('heading', { name: 'New Skill' })).toBeInTheDocument();
				expect(screen.getByLabelText('Name')).toHaveValue('');
				expect(screen.getByLabelText('Description')).toHaveValue('');
			});
		});

		it('shows Create button for new skill', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Skill' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Skill' }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
			});
		});

		it('calls createSkill with form data', async () => {
			vi.mocked(api.createSkill).mockResolvedValue({});

			renderSkills();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Skill' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Skill' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'new-skill' } });
			fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'A new skill' } });
			fireEvent.change(screen.getByLabelText('Content (Markdown)'), { target: { value: '# Content' } });
			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(api.createSkill).toHaveBeenCalledWith(
					{
						name: 'new-skill',
						description: 'A new skill',
						content: '# Content',
						allowed_tools: undefined,
					},
					undefined
				);
			});
		});

		it('shows validation error when name is empty', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Skill' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Skill' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Description')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'desc' } });
			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(screen.getByText('Name and description are required')).toBeInTheDocument();
			});
		});

		it('shows success message after create', async () => {
			vi.mocked(api.createSkill).mockResolvedValue({});

			renderSkills();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'New Skill' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'New Skill' }));

			await waitFor(() => {
				expect(screen.getByLabelText('Name')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'new-skill' } });
			fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'desc' } });
			fireEvent.click(screen.getByRole('button', { name: 'Create' }));

			await waitFor(() => {
				expect(screen.getByText('Skill created successfully')).toBeInTheDocument();
			});
		});
	});

	describe('updating a skill', () => {
		it('shows Update button for existing skill', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('python-style'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Update' })).toBeInTheDocument();
			});
		});

		it('calls updateSkill with form data', async () => {
			vi.mocked(api.updateSkill).mockResolvedValue({});

			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('python-style'));

			await waitFor(() => {
				expect(screen.getByLabelText('Description')).toBeInTheDocument();
			});

			fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'Updated description' } });
			fireEvent.click(screen.getByRole('button', { name: 'Update' }));

			await waitFor(() => {
				expect(api.updateSkill).toHaveBeenCalledWith(
					'python-style',
					expect.objectContaining({ description: 'Updated description' }),
					undefined
				);
			});
		});

		it('shows success message after update', async () => {
			vi.mocked(api.updateSkill).mockResolvedValue({});

			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('python-style'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Update' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Update' }));

			await waitFor(() => {
				expect(screen.getByText('Skill updated successfully')).toBeInTheDocument();
			});
		});
	});

	describe('deleting a skill', () => {
		it('calls deleteSkill when delete clicked and confirmed', async () => {
			vi.mocked(api.deleteSkill).mockResolvedValue({});
			vi.stubGlobal('confirm', vi.fn(() => true));

			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('python-style'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			await waitFor(() => {
				expect(api.deleteSkill).toHaveBeenCalledWith('python-style', undefined);
			});

			vi.unstubAllGlobals();
		});

		it('does not delete when confirm cancelled', async () => {
			vi.stubGlobal('confirm', vi.fn(() => false));

			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('python-style'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			expect(api.deleteSkill).not.toHaveBeenCalled();

			vi.unstubAllGlobals();
		});

		it('shows success message after delete', async () => {
			vi.mocked(api.deleteSkill).mockResolvedValue({});
			vi.stubGlobal('confirm', vi.fn(() => true));

			renderSkills();

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByText('python-style'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

			await waitFor(() => {
				expect(screen.getByText('Skill deleted successfully')).toBeInTheDocument();
			});

			vi.unstubAllGlobals();
		});
	});

	describe('no selection state', () => {
		it('shows help message when no skill selected', async () => {
			renderSkills();

			await waitFor(() => {
				expect(
					screen.getByText('Select a skill from the list or create a new one.')
				).toBeInTheDocument();
			});
		});

		it('shows hint about skill usage', async () => {
			renderSkills();

			await waitFor(() => {
				expect(screen.getByText(/skills are reusable prompts/i)).toBeInTheDocument();
			});
		});
	});
});
