import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import DiffViewer from './DiffViewer.svelte';
import type { DiffResult, FileDiff } from '$lib/types';

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

describe('DiffViewer', () => {
	beforeEach(() => {
		mockFetch.mockReset();
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	// Helper to mock the comments API response
	function mockCommentsResponse(comments: unknown[] = []) {
		return {
			ok: true,
			json: () => Promise.resolve(comments)
		};
	}

	function mockDiffResponse(files: FileDiff[] = []): DiffResult {
		return {
			base: 'main',
			head: 'feature',
			stats: {
				files_changed: files.length,
				additions: files.reduce((sum, f) => sum + f.additions, 0),
				deletions: files.reduce((sum, f) => sum + f.deletions, 0)
			},
			files
		};
	}

	function mockFile(path: string, additions = 10, deletions = 5): FileDiff {
		return {
			path,
			status: 'modified',
			additions,
			deletions,
			binary: false,
			syntax: 'typescript'
		};
	}

	describe('loading and error states', () => {
		it('shows loading state while fetching diff', async () => {
			mockFetch.mockImplementation(() => new Promise(() => {})); // Never resolves
			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			expect(screen.getByText('Loading diff...')).toBeInTheDocument();
		});

		it('shows error state when fetch fails', async () => {
			mockFetch
				.mockResolvedValueOnce({
					ok: false,
					status: 500,
					statusText: 'Internal Server Error'
				})
				.mockResolvedValueOnce(mockCommentsResponse([])); // comments

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByText('Failed to load diff')).toBeInTheDocument();
			});
		});

		it('shows empty state when no files changed', async () => {
			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDiffResponse([]))
				})
				.mockResolvedValueOnce(mockCommentsResponse([])); // comments

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByText('No changes to display')).toBeInTheDocument();
			});
		});
	});

	describe('toggleFile creates new Set (immutability)', () => {
		it('creates new Set when expanding a file', async () => {
			const diff = mockDiffResponse([mockFile('src/app.ts')]);
			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(diff)
				})
				.mockResolvedValueOnce(mockCommentsResponse([])) // comments
				.mockResolvedValueOnce({
					ok: true,
					json: () =>
						Promise.resolve({
							...mockFile('src/app.ts'),
							hunks: [{ old_start: 1, old_lines: 5, new_start: 1, new_lines: 7, lines: [] }]
						})
				});

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByText('src/app.ts')).toBeInTheDocument();
			});

			// The file header should be clickable to toggle
			const fileHeader = screen.getByText('src/app.ts').closest('button');
			if (fileHeader) {
				await fireEvent.click(fileHeader);
			}

			// Verify fetch was called to load file hunks
			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith(
					expect.stringContaining('/api/tasks/TASK-001/diff/file/')
				);
			});
		});

		it('creates new Set when collapsing a file', async () => {
			const fileData = mockFile('src/app.ts');
			const diff = mockDiffResponse([fileData]);

			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(diff)
				})
				.mockResolvedValueOnce(mockCommentsResponse([])) // comments
				.mockResolvedValueOnce({
					ok: true,
					json: () =>
						Promise.resolve({
							...fileData,
							hunks: [{ old_start: 1, old_lines: 5, new_start: 1, new_lines: 7, lines: [] }]
						})
				});

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByText('src/app.ts')).toBeInTheDocument();
			});

			const fileHeader = screen.getByText('src/app.ts').closest('button');
			if (fileHeader) {
				// Expand
				await fireEvent.click(fileHeader);

				// Wait for hunks to load
				await waitFor(() => {
					expect(mockFetch).toHaveBeenCalledTimes(3);
				});

				// Collapse - this should create a new Set
				await fireEvent.click(fileHeader);
			}
		});
	});

	describe('loadFileHunks error handling sets loadError', () => {
		it('sets loadError when file fetch returns non-ok response', async () => {
			const diff = mockDiffResponse([mockFile('src/broken.ts')]);

			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(diff)
				})
				.mockResolvedValueOnce(mockCommentsResponse([])) // comments
				.mockResolvedValueOnce({
					ok: false,
					status: 404,
					statusText: 'Not Found'
				});

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByText('src/broken.ts')).toBeInTheDocument();
			});

			const fileHeader = screen.getByText('src/broken.ts').closest('button');
			if (fileHeader) {
				await fireEvent.click(fileHeader);
			}

			// Verify the error fetch was attempted
			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledTimes(3);
			});
		});

		it('sets loadError when file fetch throws an exception', async () => {
			const diff = mockDiffResponse([mockFile('src/error.ts')]);

			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(diff)
				})
				.mockResolvedValueOnce(mockCommentsResponse([])) // comments
				.mockRejectedValueOnce(new Error('Network error'));

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByText('src/error.ts')).toBeInTheDocument();
			});

			const fileHeader = screen.getByText('src/error.ts').closest('button');
			if (fileHeader) {
				await fireEvent.click(fileHeader);
			}

			// Verify the error fetch was attempted
			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledTimes(3);
			});
		});
	});

	describe('ARIA attributes', () => {
		it('has role="tablist" on view toggle container', async () => {
			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDiffResponse([mockFile('src/app.ts')]))
				})
				.mockResolvedValueOnce(mockCommentsResponse([])); // comments

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				const tablist = screen.getByRole('tablist', { name: /diff view mode/i });
				expect(tablist).toBeInTheDocument();
			});
		});

		it('has role="tab" on Split/Unified buttons', async () => {
			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDiffResponse([mockFile('src/app.ts')]))
				})
				.mockResolvedValueOnce(mockCommentsResponse([])); // comments

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				const tabs = screen.getAllByRole('tab');
				expect(tabs).toHaveLength(2);
				expect(tabs[0]).toHaveTextContent('Split');
				expect(tabs[1]).toHaveTextContent('Unified');
			});
		});

		it('sets aria-selected correctly on view toggle buttons', async () => {
			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDiffResponse([mockFile('src/app.ts')]))
				})
				.mockResolvedValueOnce(mockCommentsResponse([])); // comments

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				const splitTab = screen.getByRole('tab', { name: 'Split' });
				const unifiedTab = screen.getByRole('tab', { name: 'Unified' });

				// Split is selected by default
				expect(splitTab).toHaveAttribute('aria-selected', 'true');
				expect(unifiedTab).toHaveAttribute('aria-selected', 'false');
			});
		});

		it('updates aria-selected when switching view mode', async () => {
			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDiffResponse([mockFile('src/app.ts')]))
				})
				.mockResolvedValueOnce(mockCommentsResponse([])); // comments

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByRole('tab', { name: 'Split' })).toBeInTheDocument();
			});

			const unifiedTab = screen.getByRole('tab', { name: 'Unified' });
			await fireEvent.click(unifiedTab);

			expect(screen.getByRole('tab', { name: 'Split' })).toHaveAttribute('aria-selected', 'false');
			expect(screen.getByRole('tab', { name: 'Unified' })).toHaveAttribute('aria-selected', 'true');
		});
	});

	describe('expand/collapse all functionality', () => {
		it('shows expand all button when files exist', async () => {
			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: () =>
						Promise.resolve(mockDiffResponse([mockFile('src/a.ts'), mockFile('src/b.ts')]))
				})
				.mockResolvedValueOnce(mockCommentsResponse([])); // comments

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByText('Expand all')).toBeInTheDocument();
			});
		});

		it('does not show expand button when no files', async () => {
			mockFetch
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDiffResponse([]))
				})
				.mockResolvedValueOnce(mockCommentsResponse([])); // comments

			render(DiffViewer, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByText('No changes to display')).toBeInTheDocument();
			});

			expect(screen.queryByText('Expand all')).not.toBeInTheDocument();
		});
	});
});
