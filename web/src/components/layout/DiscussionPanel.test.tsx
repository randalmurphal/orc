import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { DiscussionPanel } from './DiscussionPanel';
import { TooltipProvider } from '@/components/ui';
import {
	ThreadDecisionDraftSchema,
	ThreadLinkSchema,
	ThreadMessageSchema,
	ThreadRecommendationDraftSchema,
	ThreadSchema,
} from '@/gen/orc/v1/thread_pb';
import { RecommendationKind } from '@/gen/orc/v1/recommendation_pb';
import { createTimestamp } from '@/test/factories';

// Mock threadClient for send message tests
vi.mock('@/lib/client', () => ({
	threadClient: {
		sendMessage: vi.fn(),
		getThread: vi.fn(),
		promoteRecommendationDraft: vi.fn(),
		promoteDecisionDraft: vi.fn(),
	},
}));

import { threadClient } from '@/lib/client';

// =============================================================================
// TEST UTILITIES
// =============================================================================

function TestWrapper({ children }: { children: React.ReactNode }) {
	return (
		<MemoryRouter>
			<TooltipProvider delayDuration={0}>
				{children}
			</TooltipProvider>
		</MemoryRouter>
	);
}

function renderWithProviders(ui: React.ReactElement) {
	return render(ui, { wrapper: TestWrapper });
}

function createMockMessage(overrides: Record<string, unknown> = {}) {
	return create(ThreadMessageSchema, {
		id: BigInt(1),
		threadId: 'thread-001',
		role: 'user',
		content: 'Hello world',
		createdAt: createTimestamp('2024-01-01T00:00:00Z'),
		...overrides,
	});
}

function createMockThread(overrides: Record<string, unknown> = {}) {
	return create(ThreadSchema, {
		id: 'thread-001',
		title: 'Test Thread',
		status: 'active',
		taskId: '',
		initiativeId: '',
		sessionId: '',
		fileContext: '',
		createdAt: createTimestamp('2024-01-01T00:00:00Z'),
		updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
		messages: [],
		links: [],
		recommendationDrafts: [],
		decisionDrafts: [],
		...overrides,
	});
}

beforeEach(() => {
	vi.clearAllMocks();
	vi.mocked(threadClient.getThread).mockResolvedValue({
		thread: createMockThread(),
	} as never);
});

afterEach(() => {
	vi.clearAllMocks();
});

// =============================================================================
// SC-7: Discussion mode renders chat interface with send/receive
// =============================================================================

describe('DiscussionPanel chat interface (SC-7)', () => {
	it('should render message input field', () => {
		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		expect(screen.getByRole('textbox')).toBeInTheDocument();
	});

	it('should render send button', () => {
		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		expect(screen.getByRole('button', { name: /send/i })).toBeInTheDocument();
	});

	it('should display existing messages', () => {
		const messages = [
			createMockMessage({ id: BigInt(1), role: 'user', content: 'What is the plan?' }),
			createMockMessage({ id: BigInt(2), role: 'assistant', content: 'Here is the plan...' }),
		];

		renderWithProviders(
			<DiscussionPanel
				threadId="thread-001"
				projectId="proj-001"
				messages={messages}
			/>
		);

		expect(screen.getByText('What is the plan?')).toBeInTheDocument();
		expect(screen.getByText('Here is the plan...')).toBeInTheDocument();
	});

	it('should load thread history when opened without initial messages', async () => {
		vi.mocked(threadClient.getThread).mockResolvedValue({
			thread: createMockThread({
				messages: [
					createMockMessage({ id: BigInt(1), role: 'user', content: 'Restored message' }),
					createMockMessage({ id: BigInt(2), role: 'assistant', content: 'Restored reply' }),
				],
			}),
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		await waitFor(() => {
			expect(screen.getByText('Restored message')).toBeInTheDocument();
			expect(screen.getByText('Restored reply')).toBeInTheDocument();
		});
	});

	it('should render thread context links and drafts from loaded thread state', async () => {
		vi.mocked(threadClient.getThread).mockResolvedValue({
			thread: createMockThread({
				links: [
					create(ThreadLinkSchema, { id: BigInt(1), threadId: 'thread-001', linkType: 'task', targetId: 'TASK-001', title: 'TASK-001' }),
				],
				recommendationDrafts: [
					create(ThreadRecommendationDraftSchema, {
						id: 'TRD-001',
						threadId: 'thread-001',
						kind: RecommendationKind.FOLLOW_UP,
						title: 'Follow up on workspace',
						summary: 'The promotion path needs a regression test.',
						proposedAction: 'Add a regression test.',
						evidence: 'No test covered this flow.',
						status: 'draft',
					}),
				],
				decisionDrafts: [
					create(ThreadDecisionDraftSchema, {
						id: 'TDD-001',
						threadId: 'thread-001',
						initiativeId: 'INIT-001',
						decision: 'Keep recommendations human-gated',
						rationale: 'Automatic backlog creation is noisy.',
						status: 'draft',
					}),
				],
			}),
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		await waitFor(() => {
			expect(screen.getByText('Current context')).toBeInTheDocument();
			expect(screen.getByText('Follow up on workspace')).toBeInTheDocument();
			expect(screen.getByText('Keep recommendations human-gated')).toBeInTheDocument();
		});
	});

	it('should distinguish user and assistant messages visually', () => {
		const messages = [
			createMockMessage({ id: BigInt(1), role: 'user', content: 'User message' }),
			createMockMessage({ id: BigInt(2), role: 'assistant', content: 'Assistant message' }),
		];

		renderWithProviders(
			<DiscussionPanel
				threadId="thread-001"
				projectId="proj-001"
				messages={messages}
			/>
		);

		const userMsg = screen.getByText('User message').closest('[class*="message"]');
		const assistantMsg = screen.getByText('Assistant message').closest('[class*="message"]');

		expect(userMsg?.className).toMatch(/user/);
		expect(assistantMsg?.className).toMatch(/assistant/);
	});

	it('should send message via ThreadService.SendMessage on submit', async () => {
		const userMessage = createMockMessage({
			id: BigInt(3),
			role: 'user',
			content: 'Hello',
		});
		const assistantMessage = createMockMessage({
			id: BigInt(4),
			role: 'assistant',
			content: 'Hi there!',
		});

		vi.mocked(threadClient.sendMessage).mockResolvedValue({
			userMessage,
			assistantMessage,
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		const input = screen.getByRole('textbox');
		fireEvent.change(input, { target: { value: 'Hello' } });
		fireEvent.click(screen.getByRole('button', { name: /send/i }));

		await waitFor(() => {
			expect(threadClient.sendMessage).toHaveBeenCalledWith(
				expect.objectContaining({
					threadId: 'thread-001',
					projectId: 'proj-001',
					content: 'Hello',
				})
			);
		});
	});

	it('should display user message immediately after sending', async () => {
		vi.mocked(threadClient.sendMessage).mockResolvedValue({
			userMessage: createMockMessage({ id: BigInt(3), role: 'user', content: 'Hello' }),
			assistantMessage: createMockMessage({ id: BigInt(4), role: 'assistant', content: 'Hi' }),
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		const input = screen.getByRole('textbox');
		fireEvent.change(input, { target: { value: 'Hello' } });
		fireEvent.click(screen.getByRole('button', { name: /send/i }));

		// User message should appear immediately (optimistic)
		await waitFor(() => {
			expect(screen.getByText('Hello')).toBeInTheDocument();
		});
	});

	it('should display assistant response after API returns', async () => {
		vi.mocked(threadClient.sendMessage).mockResolvedValue({
			userMessage: createMockMessage({ id: BigInt(3), role: 'user', content: 'Hello' }),
			assistantMessage: createMockMessage({ id: BigInt(4), role: 'assistant', content: 'Hi there!' }),
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		const input = screen.getByRole('textbox');
		fireEvent.change(input, { target: { value: 'Hello' } });
		fireEvent.click(screen.getByRole('button', { name: /send/i }));

		await waitFor(() => {
			expect(screen.getByText('Hi there!')).toBeInTheDocument();
		});
	});

	it('should clear input after successful send', async () => {
		vi.mocked(threadClient.sendMessage).mockResolvedValue({
			userMessage: createMockMessage({ id: BigInt(3), role: 'user', content: 'Hello' }),
			assistantMessage: createMockMessage({ id: BigInt(4), role: 'assistant', content: 'Hi' }),
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		const input = screen.getByRole('textbox') as HTMLInputElement;
		fireEvent.change(input, { target: { value: 'Hello' } });
		fireEvent.click(screen.getByRole('button', { name: /send/i }));

		await waitFor(() => {
			expect(input.value).toBe('');
		});
	});

	it('should send message on Enter key press', async () => {
		vi.mocked(threadClient.sendMessage).mockResolvedValue({
			userMessage: createMockMessage({ id: BigInt(3), role: 'user', content: 'Enter test' }),
			assistantMessage: createMockMessage({ id: BigInt(4), role: 'assistant', content: 'Ok' }),
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		const input = screen.getByRole('textbox');
		fireEvent.change(input, { target: { value: 'Enter test' } });
		fireEvent.keyDown(input, { key: 'Enter' });

		await waitFor(() => {
			expect(threadClient.sendMessage).toHaveBeenCalled();
		});
	});

	it('should disable send button when input is empty', () => {
		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		const sendButton = screen.getByRole('button', { name: /send/i });
		expect(sendButton).toBeDisabled();
	});

	it('should disable input and button while sending', async () => {
		let resolvePromise: (value: unknown) => void;
		const pending = new Promise((resolve) => {
			resolvePromise = resolve;
		});
		vi.mocked(threadClient.sendMessage).mockReturnValue(pending as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		const input = screen.getByRole('textbox');
		fireEvent.change(input, { target: { value: 'Hello' } });
		fireEvent.click(screen.getByRole('button', { name: /send/i }));

		await waitFor(() => {
			expect(screen.getByRole('button', { name: /send/i })).toBeDisabled();
		});

		// Resolve the pending call
		resolvePromise!({
			userMessage: createMockMessage(),
			assistantMessage: createMockMessage(),
		});
	});

	it('should promote a recommendation draft from the workspace', async () => {
		vi.mocked(threadClient.getThread).mockResolvedValue({
			thread: createMockThread({
				recommendationDrafts: [
					create(ThreadRecommendationDraftSchema, {
						id: 'TRD-001',
						threadId: 'thread-001',
						kind: RecommendationKind.FOLLOW_UP,
						title: 'Follow up on workspace',
						summary: 'The promotion path needs a regression test.',
						proposedAction: 'Add a regression test.',
						evidence: 'No test covered this flow.',
						status: 'draft',
					}),
				],
			}),
		} as never);
		vi.mocked(threadClient.promoteRecommendationDraft).mockResolvedValue({
			thread: createMockThread({
				recommendationDrafts: [
					create(ThreadRecommendationDraftSchema, {
						id: 'TRD-001',
						threadId: 'thread-001',
						kind: RecommendationKind.FOLLOW_UP,
						title: 'Follow up on workspace',
						summary: 'The promotion path needs a regression test.',
						proposedAction: 'Add a regression test.',
						evidence: 'No test covered this flow.',
						status: 'promoted',
					}),
				],
			}),
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		await waitFor(() => {
			expect(screen.getByRole('button', { name: /promote to inbox/i })).toBeInTheDocument();
		});

		fireEvent.click(screen.getByRole('button', { name: /promote to inbox/i }));

		await waitFor(() => {
			expect(threadClient.promoteRecommendationDraft).toHaveBeenCalledWith(
				expect.objectContaining({
					projectId: 'proj-001',
					threadId: 'thread-001',
					draftId: 'TRD-001',
				})
			);
		});
	});
});

// =============================================================================
// FAILURE MODES: SendMessage error handling
// =============================================================================

describe('DiscussionPanel error handling (SC-7)', () => {
	it('should show inline error when SendMessage fails', async () => {
		vi.mocked(threadClient.sendMessage).mockRejectedValue(
			new Error('Network error')
		);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		const input = screen.getByRole('textbox');
		fireEvent.change(input, { target: { value: 'Hello' } });
		fireEvent.click(screen.getByRole('button', { name: /send/i }));

		await waitFor(() => {
			expect(screen.getByText(/failed to send message/i)).toBeInTheDocument();
		});
	});

	it('should keep message in input on send failure', async () => {
		vi.mocked(threadClient.sendMessage).mockRejectedValue(
			new Error('Network error')
		);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		const input = screen.getByRole('textbox') as HTMLInputElement;
		fireEvent.change(input, { target: { value: 'My important message' } });
		fireEvent.click(screen.getByRole('button', { name: /send/i }));

		await waitFor(() => {
			expect(input.value).toBe('My important message');
		});
	});

	it('should show retry option on send failure', async () => {
		vi.mocked(threadClient.sendMessage).mockRejectedValue(
			new Error('Network error')
		);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		const input = screen.getByRole('textbox');
		fireEvent.change(input, { target: { value: 'Hello' } });
		fireEvent.click(screen.getByRole('button', { name: /send/i }));

		await waitFor(() => {
			expect(screen.getByText(/try again/i)).toBeInTheDocument();
		});
	});
});

// =============================================================================
// EDGE CASES
// =============================================================================

describe('DiscussionPanel edge cases', () => {
	it('should show empty state when no messages exist', () => {
		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" messages={[]} />
		);

		// Should show some indication that this is a new conversation
		const input = screen.getByRole('textbox');
		expect(input).toBeInTheDocument();
	});

	it('should not submit empty or whitespace-only messages', () => {
		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		const input = screen.getByRole('textbox');
		fireEvent.change(input, { target: { value: '   ' } });
		fireEvent.click(screen.getByRole('button', { name: /send/i }));

		expect(threadClient.sendMessage).not.toHaveBeenCalled();
	});
});
