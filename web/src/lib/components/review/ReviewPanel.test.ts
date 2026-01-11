import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import ReviewPanel from './ReviewPanel.svelte';
import type { ReviewComment } from '$lib/types';
import * as api from '$lib/api';

// Mock the API module
vi.mock('$lib/api', () => ({
	getReviewComments: vi.fn(),
	createReviewComment: vi.fn(),
	updateReviewComment: vi.fn(),
	deleteReviewComment: vi.fn(),
	triggerReviewRetry: vi.fn()
}));

// Mock the toast store
vi.mock('$lib/stores/toast.svelte', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
		info: vi.fn()
	}
}));

describe('ReviewPanel', () => {
	const mockGetReviewComments = api.getReviewComments as ReturnType<typeof vi.fn>;
	const mockCreateReviewComment = api.createReviewComment as ReturnType<typeof vi.fn>;
	const mockUpdateReviewComment = api.updateReviewComment as ReturnType<typeof vi.fn>;
	const mockDeleteReviewComment = api.deleteReviewComment as ReturnType<typeof vi.fn>;
	const mockTriggerReviewRetry = api.triggerReviewRetry as ReturnType<typeof vi.fn>;

	beforeEach(() => {
		vi.clearAllMocks();
		mockGetReviewComments.mockResolvedValue([]);
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	function createMockComment(overrides: Partial<ReviewComment> = {}): ReviewComment {
		return {
			id: 'comment-1',
			task_id: 'TASK-001',
			review_round: 1,
			content: 'Test comment',
			severity: 'issue',
			status: 'open',
			created_at: '2025-01-01T00:00:00Z',
			...overrides
		};
	}

	describe('loading state', () => {
		it('shows loading skeleton while fetching comments', async () => {
			mockGetReviewComments.mockImplementation(() => new Promise(() => {}));
			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			// Should show skeleton loading elements
			const skeletons = document.querySelectorAll('.skeleton');
			expect(skeletons.length).toBeGreaterThan(0);
		});

		it('loads comments on mount', async () => {
			mockGetReviewComments.mockResolvedValue([]);
			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(mockGetReviewComments).toHaveBeenCalledWith('TASK-001');
			});
		});
	});

	describe('selectedCommentId tracking with j/k navigation', () => {
		it('selects first comment on j key when none selected', async () => {
			const comments = [
				createMockComment({ id: 'c1', content: 'Navigation test comment one' }),
				createMockComment({ id: 'c2', content: 'Navigation test comment two' }),
				createMockComment({ id: 'c3', content: 'Navigation test comment three' })
			];
			mockGetReviewComments.mockResolvedValue(comments);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			// Wait for loading to complete
			await waitFor(() => {
				const skeletons = document.querySelectorAll('.skeleton');
				expect(skeletons.length).toBe(0);
			});

			// Press j to select first comment
			await fireEvent.keyDown(window, { key: 'j' });

			// The first comment should be selected (has .selected class)
			await waitFor(() => {
				const selectedWrapper = document.querySelector('.comment-wrapper.selected');
				expect(selectedWrapper).toBeInTheDocument();
			});
		});

		it('moves selection down with j key', async () => {
			const comments = [
				createMockComment({ id: 'c1', content: 'Test navigation down first' }),
				createMockComment({ id: 'c2', content: 'Test navigation down second' }),
				createMockComment({ id: 'c3', content: 'Test navigation down third' })
			];
			mockGetReviewComments.mockResolvedValue(comments);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			// Wait for loading to complete
			await waitFor(() => {
				const skeletons = document.querySelectorAll('.skeleton');
				expect(skeletons.length).toBe(0);
			});

			// Select first
			await fireEvent.keyDown(window, { key: 'j' });

			// Move to second
			await fireEvent.keyDown(window, { key: 'j' });

			// The wrapper containing Second comment should be selected
			await waitFor(() => {
				const selectedWrappers = document.querySelectorAll('.comment-wrapper.selected');
				expect(selectedWrappers.length).toBe(1);
			});
		});

		it('moves selection up with k key', async () => {
			const comments = [
				createMockComment({ id: 'c1', content: 'Test navigation up first item' }),
				createMockComment({ id: 'c2', content: 'Test navigation up second item' })
			];
			mockGetReviewComments.mockResolvedValue(comments);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			// Wait for loading to complete
			await waitFor(() => {
				const skeletons = document.querySelectorAll('.skeleton');
				expect(skeletons.length).toBe(0);
			});

			// Navigate to second comment
			await fireEvent.keyDown(window, { key: 'j' });
			await fireEvent.keyDown(window, { key: 'j' });

			// Navigate back up
			await fireEvent.keyDown(window, { key: 'k' });

			await waitFor(() => {
				const selectedWrappers = document.querySelectorAll('.comment-wrapper.selected');
				expect(selectedWrappers.length).toBe(1);
			});
		});

		it('does not go below last comment with j key', async () => {
			const comments = [
				createMockComment({ id: 'c1', content: 'Single comment boundary test' })
			];
			mockGetReviewComments.mockResolvedValue(comments);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			// Wait for loading to complete
			await waitFor(() => {
				const skeletons = document.querySelectorAll('.skeleton');
				expect(skeletons.length).toBe(0);
			});

			// Select first
			await fireEvent.keyDown(window, { key: 'j' });
			// Try to go past the end
			await fireEvent.keyDown(window, { key: 'j' });
			await fireEvent.keyDown(window, { key: 'j' });

			// Should still have exactly one selected
			await waitFor(() => {
				const selectedWrappers = document.querySelectorAll('.comment-wrapper.selected');
				expect(selectedWrappers.length).toBe(1);
			});
		});

		it('does not go above first comment with k key', async () => {
			const comments = [
				createMockComment({ id: 'c1', content: 'Upper boundary test first' }),
				createMockComment({ id: 'c2', content: 'Upper boundary test second' })
			];
			mockGetReviewComments.mockResolvedValue(comments);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			// Wait for loading to complete
			await waitFor(() => {
				const skeletons = document.querySelectorAll('.skeleton');
				expect(skeletons.length).toBe(0);
			});

			// Select first
			await fireEvent.keyDown(window, { key: 'j' });
			// Try to go above first
			await fireEvent.keyDown(window, { key: 'k' });
			await fireEvent.keyDown(window, { key: 'k' });
			await fireEvent.keyDown(window, { key: 'k' });

			// Should still have selection
			await waitFor(() => {
				const selectedWrappers = document.querySelectorAll('.comment-wrapper.selected');
				expect(selectedWrappers.length).toBe(1);
			});
		});

		it('ignores j/k when focus is in input field', async () => {
			const comments = [createMockComment({ id: 'c1', content: 'Input focus test comment' })];
			mockGetReviewComments.mockResolvedValue(comments);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			// Wait for loading to complete
			await waitFor(() => {
				const skeletons = document.querySelectorAll('.skeleton');
				expect(skeletons.length).toBe(0);
			});

			// Open the comment form
			await fireEvent.keyDown(window, { key: 'n' });

			await waitFor(() => {
				const formHeader = document.querySelector('.form-header h3');
				expect(formHeader).toBeInTheDocument();
			});

			// Get the textarea
			const textarea = screen.getByPlaceholderText(/describe the issue/i);

			// Fire keydown from within the textarea - should not navigate
			await fireEvent.keyDown(textarea, { key: 'j' });

			// No comment should be selected since we're in a text field
			const selectedWrappers = document.querySelectorAll('.comment-wrapper.selected');
			expect(selectedWrappers.length).toBe(0);
		});
	});

	describe('comments grouped correctly', () => {
		it('groups comments by file path', async () => {
			const comments = [
				createMockComment({ id: 'c1', file_path: 'src/grouped-app.ts', content: 'Comment on app' }),
				createMockComment({ id: 'c2', file_path: 'src/grouped-app.ts', content: 'Another on app' }),
				createMockComment({ id: 'c3', file_path: 'src/grouped-utils.ts', content: 'Comment on utils' })
			];
			mockGetReviewComments.mockResolvedValue(comments);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			// Wait for loading to complete
			await waitFor(() => {
				const skeletons = document.querySelectorAll('.skeleton');
				expect(skeletons.length).toBe(0);
			});

			// File sections should exist
			const fileSections = document.querySelectorAll('.file-section');
			expect(fileSections.length).toBeGreaterThanOrEqual(2);
		});

		it('shows general section for comments without file path', async () => {
			const comments = [
				createMockComment({ id: 'c1', file_path: undefined, content: 'No file path comment' }),
				createMockComment({ id: 'c2', file_path: 'src/app.ts', content: 'With file path comment' })
			];
			mockGetReviewComments.mockResolvedValue(comments);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			// Wait for loading to complete
			await waitFor(() => {
				const skeletons = document.querySelectorAll('.skeleton');
				expect(skeletons.length).toBe(0);
			});

			// Should have file-header elements for both General and the file path
			const fileHeaders = document.querySelectorAll('.file-header');
			expect(fileHeaders.length).toBeGreaterThanOrEqual(1);
		});

		it('separates open and resolved comments', async () => {
			const comments = [
				createMockComment({ id: 'c1', status: 'open', content: 'Open comment' }),
				createMockComment({ id: 'c2', status: 'resolved', content: 'Resolved comment' })
			];
			mockGetReviewComments.mockResolvedValue(comments);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				// Resolved section should show count
				expect(screen.getByText(/Resolved \(1\)/)).toBeInTheDocument();
			});
		});
	});

	describe('filter functionality', () => {
		it('filters comments by severity', async () => {
			const comments = [
				createMockComment({ id: 'c1', severity: 'blocker', content: 'This is blocker comment' }),
				createMockComment({ id: 'c2', severity: 'issue', content: 'This is issue comment' }),
				createMockComment({ id: 'c3', severity: 'suggestion', content: 'This is suggestion comment' })
			];
			mockGetReviewComments.mockResolvedValue(comments);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			// Wait for loading to complete - skeleton disappears
			await waitFor(() => {
				const skeletons = document.querySelectorAll('.skeleton');
				expect(skeletons.length).toBe(0);
			});

			// All filter tabs should be visible
			const filterTabs = document.querySelectorAll('.filter-tab');
			expect(filterTabs.length).toBeGreaterThanOrEqual(4);
		});

		it('shows filter counts correctly', async () => {
			const comments = [
				createMockComment({ id: 'c1', severity: 'blocker', content: 'Blocker comment 1' }),
				createMockComment({ id: 'c2', severity: 'blocker', content: 'Blocker comment 2' }),
				createMockComment({ id: 'c3', severity: 'issue', content: 'Issue comment 3' }),
				createMockComment({ id: 'c4', severity: 'suggestion', content: 'Suggestion comment 4' })
			];
			mockGetReviewComments.mockResolvedValue(comments);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			// Wait for loading to complete - skeleton disappears
			await waitFor(() => {
				const skeletons = document.querySelectorAll('.skeleton');
				expect(skeletons.length).toBe(0);
			});

			// Find filter tabs by their class
			const filterTabs = document.querySelectorAll('.filter-tab');
			expect(filterTabs.length).toBeGreaterThanOrEqual(4);

			// All tab should show 4
			const allTab = Array.from(filterTabs).find((t) => t.textContent?.includes('All'));
			expect(allTab?.textContent).toContain('4');
		});
	});

	describe('keyboard shortcuts', () => {
		it('opens comment form with n key', async () => {
			mockGetReviewComments.mockResolvedValue([]);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(mockGetReviewComments).toHaveBeenCalled();
			});

			// Wait for loading to finish
			await waitFor(() => {
				expect(screen.getByText('No open comments')).toBeInTheDocument();
			});

			await fireEvent.keyDown(window, { key: 'n' });

			await waitFor(() => {
				// CommentForm has a header with h3 "Add Comment"
				const formHeader = document.querySelector('.form-header h3');
				expect(formHeader).toBeInTheDocument();
				expect(formHeader?.textContent).toContain('Add Comment');
			});
		});

		it('closes comment form with Escape key', async () => {
			mockGetReviewComments.mockResolvedValue([]);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(mockGetReviewComments).toHaveBeenCalled();
			});

			// Wait for loading to finish
			await waitFor(() => {
				expect(screen.getByText('No open comments')).toBeInTheDocument();
			});

			// Open form
			await fireEvent.keyDown(window, { key: 'n' });

			await waitFor(() => {
				const formHeader = document.querySelector('.form-header h3');
				expect(formHeader).toBeInTheDocument();
			});

			// Close form
			await fireEvent.keyDown(window, { key: 'Escape' });

			await waitFor(() => {
				const formHeader = document.querySelector('.form-header h3');
				expect(formHeader).not.toBeInTheDocument();
			});
		});
	});

	describe('empty state', () => {
		it('shows empty state when no comments', async () => {
			mockGetReviewComments.mockResolvedValue([]);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByText('No open comments')).toBeInTheDocument();
			});
		});

		it('shows add comment button in empty state', async () => {
			mockGetReviewComments.mockResolvedValue([]);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /add a comment/i })).toBeInTheDocument();
			});
		});
	});

	describe('retry functionality', () => {
		it('shows retry button when there are open comments', async () => {
			mockGetReviewComments.mockResolvedValue([createMockComment()]);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /send.*comment.*to agent/i })).toBeInTheDocument();
			});
		});

		it('shows blocker warning when blockers exist', async () => {
			mockGetReviewComments.mockResolvedValue([
				createMockComment({ severity: 'blocker' })
			]);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.getByText(/contains blockers/i)).toBeInTheDocument();
			});
		});

		it('does not show retry button when no open comments', async () => {
			mockGetReviewComments.mockResolvedValue([
				createMockComment({ status: 'resolved' })
			]);

			render(ReviewPanel, { props: { taskId: 'TASK-001' } });

			await waitFor(() => {
				expect(screen.queryByRole('button', { name: /send.*to agent/i })).not.toBeInTheDocument();
			});
		});
	});
});
