import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
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
		addLink: vi.fn(),
		createRecommendationDraft: vi.fn(),
		createDecisionDraft: vi.fn(),
		sendMessage: vi.fn(),
		getThread: vi.fn(),
		promoteRecommendationDraft: vi.fn(),
		promoteDecisionDraft: vi.fn(),
	},
}));

let eventHandler: ((event: {
	projectId?: string;
	payload: { case?: string; value?: { threadId?: string; updateType?: string } };
}) => void) | undefined;

vi.mock('@/hooks/useEvents', () => ({
	useEvents: () => ({
		status: 'connected',
		subscribe: vi.fn(),
		subscribeGlobal: vi.fn(),
		disconnect: vi.fn(),
		isConnected: vi.fn(() => true),
		onEvent: vi.fn((handler: typeof eventHandler) => {
			eventHandler = handler;
			return () => {
				if (eventHandler === handler) {
					eventHandler = undefined;
				}
			};
		}),
	}),
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

function createDeferred<T>() {
	let resolvePromise: ((value: T | PromiseLike<T>) => void) | undefined;
	let rejectPromise: ((reason?: unknown) => void) | undefined;
	const promise = new Promise<T>((resolve, reject) => {
		resolvePromise = resolve;
		rejectPromise = reject;
	});
	return {
		promise,
		resolve(value: T) {
			if (resolvePromise === undefined) {
				throw new Error('Deferred promise resolved before initialization');
			}
			resolvePromise(value);
		},
		reject(reason?: unknown) {
			if (rejectPromise === undefined) {
				throw new Error('Deferred promise rejected before initialization');
			}
			rejectPromise(reason);
		},
	};
}

beforeEach(() => {
	vi.clearAllMocks();
	eventHandler = undefined;
	vi.mocked(threadClient.getThread).mockReturnValue(new Promise(() => {}) as never);
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
				taskId: 'TASK-001',
				initiativeId: 'INIT-001',
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
			expect(screen.getByRole('button', { name: /add link/i })).toBeInTheDocument();
		});
	});

	it('should reload full thread context even when initial messages are provided', async () => {
		vi.mocked(threadClient.getThread).mockResolvedValue({
			thread: createMockThread({
				links: [
					create(ThreadLinkSchema, {
						id: BigInt(1),
						threadId: 'thread-001',
						linkType: 'diff',
						targetId: 'TASK-001:web/src/components/layout/DiscussionPanel.tsx',
						title: 'DiscussionPanel diff',
					}),
				],
			}),
		} as never);

		renderWithProviders(
			<DiscussionPanel
				threadId="thread-001"
				projectId="proj-001"
				messages={[createMockMessage({ content: 'Preloaded message' })]}
			/>
		);

		await waitFor(() => {
			expect(threadClient.getThread).toHaveBeenCalledWith(
				expect.objectContaining({
					projectId: 'proj-001',
					threadId: 'thread-001',
				})
			);
			expect(screen.getByText('DiscussionPanel diff')).toBeInTheDocument();
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

		await act(async () => {
			resolvePromise!({
				userMessage: createMockMessage(),
				assistantMessage: createMockMessage({ id: BigInt(2), role: 'assistant' }),
			});
			await Promise.resolve();
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

	it('should add typed context links from the workspace', async () => {
		vi.mocked(threadClient.getThread).mockResolvedValue({
			thread: createMockThread(),
		} as never);
		vi.mocked(threadClient.addLink).mockResolvedValue({
			thread: createMockThread({
				links: [
					create(ThreadLinkSchema, {
						id: BigInt(2),
						threadId: 'thread-001',
						linkType: 'file',
						targetId: 'web/src/components/layout/DiscussionPanel.tsx',
						title: 'DiscussionPanel.tsx',
					}),
				],
			}),
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		await waitFor(() => {
			expect(threadClient.getThread).toHaveBeenCalled();
		});

		fireEvent.change(screen.getByLabelText('Link target'), {
			target: { value: 'web/src/components/layout/DiscussionPanel.tsx' },
		});
		fireEvent.change(screen.getByLabelText('Link title'), {
			target: { value: 'DiscussionPanel.tsx' },
		});
		fireEvent.click(screen.getByRole('button', { name: /add link/i }));

		await waitFor(() => {
			expect(threadClient.addLink).toHaveBeenCalledWith(
				expect.objectContaining({
					projectId: 'proj-001',
					threadId: 'thread-001',
					link: expect.objectContaining({
						linkType: 'file',
						targetId: 'web/src/components/layout/DiscussionPanel.tsx',
						title: 'DiscussionPanel.tsx',
					}),
				})
			);
			expect(screen.getByText('DiscussionPanel.tsx')).toBeInTheDocument();
		});
	});

	it('should create a recommendation draft from the workspace', async () => {
		vi.mocked(threadClient.getThread).mockResolvedValue({
			thread: createMockThread(),
		} as never);
		vi.mocked(threadClient.createRecommendationDraft).mockResolvedValue({
			thread: createMockThread({
				recommendationDrafts: [
					create(ThreadRecommendationDraftSchema, {
						id: 'TRD-002',
						threadId: 'thread-001',
						kind: RecommendationKind.RISK,
						title: 'Investigate reload gap',
						summary: 'The panel should always reload full thread state.',
						proposedAction: 'Keep the full-thread fetch on reopen.',
						evidence: 'Initial messages skipped the fetch before this change.',
						status: 'draft',
					}),
				],
			}),
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		await waitFor(() => {
			expect(threadClient.getThread).toHaveBeenCalled();
		});

		fireEvent.change(screen.getByLabelText('Recommendation kind'), {
			target: { value: 'risk' },
		});
		fireEvent.change(screen.getByLabelText('Recommendation title'), {
			target: { value: 'Investigate reload gap' },
		});
		fireEvent.change(screen.getByLabelText('Recommendation summary'), {
			target: { value: 'The panel should always reload full thread state.' },
		});
		fireEvent.change(screen.getByLabelText('Recommendation proposed action'), {
			target: { value: 'Keep the full-thread fetch on reopen.' },
		});
		fireEvent.change(screen.getByLabelText('Recommendation evidence'), {
			target: { value: 'Initial messages skipped the fetch before this change.' },
		});
		fireEvent.click(screen.getByRole('button', { name: /create recommendation draft/i }));

		await waitFor(() => {
			expect(threadClient.createRecommendationDraft).toHaveBeenCalledWith(
				expect.objectContaining({
					projectId: 'proj-001',
					threadId: 'thread-001',
					draft: expect.objectContaining({
						title: 'Investigate reload gap',
					}),
				})
			);
			expect(screen.getByText('Investigate reload gap')).toBeInTheDocument();
		});
	});

	it('should create a decision draft from the workspace', async () => {
		vi.mocked(threadClient.getThread).mockResolvedValue({
			thread: createMockThread({
				initiativeId: 'INIT-001',
			}),
		} as never);
		vi.mocked(threadClient.createDecisionDraft).mockResolvedValue({
			thread: createMockThread({
				initiativeId: 'INIT-001',
				decisionDrafts: [
					create(ThreadDecisionDraftSchema, {
						id: 'TDD-002',
						threadId: 'thread-001',
						initiativeId: 'INIT-001',
						decision: 'Keep thread context persisted',
						rationale: 'Reopen should restore the real workspace state.',
						status: 'draft',
					}),
				],
			}),
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		await waitFor(() => {
			expect(threadClient.getThread).toHaveBeenCalled();
		});

		fireEvent.change(screen.getByLabelText('Decision text'), {
			target: { value: 'Keep thread context persisted' },
		});
		fireEvent.change(screen.getByLabelText('Decision rationale'), {
			target: { value: 'Reopen should restore the real workspace state.' },
		});
		fireEvent.click(screen.getByRole('button', { name: /create decision draft/i }));

		await waitFor(() => {
			expect(threadClient.createDecisionDraft).toHaveBeenCalledWith(
				expect.objectContaining({
					projectId: 'proj-001',
					threadId: 'thread-001',
					draft: expect.objectContaining({
						initiativeId: 'INIT-001',
						decision: 'Keep thread context persisted',
					}),
				})
			);
			expect(screen.getByText('Keep thread context persisted')).toBeInTheDocument();
		});
	});

	it('should keep decision drafts in draft-only state', async () => {
		vi.mocked(threadClient.getThread).mockResolvedValue({
			thread: createMockThread({
				decisionDrafts: [
					create(ThreadDecisionDraftSchema, {
						id: 'TDD-003',
						threadId: 'thread-001',
						initiativeId: 'INIT-001',
						decision: 'Keep recommendations human-gated',
						rationale: 'Decision drafts should not write initiative history directly.',
						status: 'draft',
					}),
				],
			}),
		} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		await waitFor(() => {
			expect(screen.getByText('Keep recommendations human-gated')).toBeInTheDocument();
		});

		expect(screen.queryByRole('button', { name: /promote decision/i })).not.toBeInTheDocument();
		expect(screen.getByText(/decision drafts stay in discussion until a human accepts them/i)).toBeInTheDocument();
	});

	it('should reset thread-local state when switching threads', async () => {
		const firstThread = createDeferred<unknown>();
		const secondThread = createDeferred<unknown>();
		vi.mocked(threadClient.getThread)
			.mockReturnValueOnce(firstThread.promise as never)
			.mockReturnValueOnce(secondThread.promise as never);

		const { rerender } = renderWithProviders(
			<DiscussionPanel
				threadId="thread-001"
				projectId="proj-001"
				messages={[createMockMessage({ id: BigInt(1), content: 'Thread one message' })]}
			/>
		);

		expect(screen.getByText('Thread one message')).toBeInTheDocument();

		fireEvent.change(screen.getByLabelText('Decision initiative'), {
			target: { value: 'CUSTOM-INIT' },
		});

		rerender(
			<DiscussionPanel threadId="thread-002" projectId="proj-001" />
		);

		expect(screen.queryByText('Thread one message')).not.toBeInTheDocument();
		expect(screen.getByText(/loading thread history/i)).toBeInTheDocument();

		await act(async () => {
			secondThread.resolve({
				thread: createMockThread({
					id: 'thread-002',
					initiativeId: 'INIT-002',
					messages: [
						createMockMessage({ id: BigInt(2), threadId: 'thread-002', content: 'Thread two message' }),
					],
				}),
			});
			await Promise.resolve();
		});

		await waitFor(() => {
			expect(screen.getByText('Thread two message')).toBeInTheDocument();
			expect((screen.getByLabelText('Decision initiative') as HTMLInputElement).value).toBe('INIT-002');
		});

		await act(async () => {
			firstThread.resolve({
				thread: createMockThread({
					id: 'thread-001',
					initiativeId: 'INIT-001',
					messages: [
						createMockMessage({ id: BigInt(3), content: 'Late thread one message' }),
					],
				}),
			});
			await Promise.resolve();
		});

		expect(screen.queryByText('Late thread one message')).not.toBeInTheDocument();
		expect(screen.getByText('Thread two message')).toBeInTheDocument();
		expect((screen.getByLabelText('Decision initiative') as HTMLInputElement).value).toBe('INIT-002');
	});

	it('should refresh when a matching threadUpdated event arrives', async () => {
		vi.mocked(threadClient.getThread)
			.mockResolvedValueOnce({
				thread: createMockThread({
					links: [],
				}),
			} as never)
			.mockResolvedValueOnce({
				thread: createMockThread({
					links: [
						create(ThreadLinkSchema, {
							id: BigInt(9),
							threadId: 'thread-001',
							linkType: 'file',
							targetId: 'docs/notes.md',
							title: 'notes.md',
						}),
					],
				}),
			} as never);

		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		await waitFor(() => {
			expect(threadClient.getThread).toHaveBeenCalledTimes(1);
		});

		await act(async () => {
			eventHandler?.({
				projectId: 'proj-001',
				payload: {
					case: 'threadUpdated',
					value: {
						threadId: 'thread-001',
						updateType: 'link_added',
					},
				},
			});
		});

		await waitFor(() => {
			expect(threadClient.getThread).toHaveBeenCalledTimes(2);
			expect(screen.getByText('notes.md')).toBeInTheDocument();
		});
	});

	it('should ignore threadUpdated events for other projects', async () => {
		renderWithProviders(
			<DiscussionPanel threadId="thread-001" projectId="proj-001" />
		);

		await waitFor(() => {
			expect(threadClient.getThread).toHaveBeenCalledTimes(1);
		});

		eventHandler?.({
			projectId: 'proj-002',
			payload: {
				case: 'threadUpdated',
				value: {
					threadId: 'thread-001',
					updateType: 'link_added',
				},
			},
		});

		await waitFor(() => {
			expect(threadClient.getThread).toHaveBeenCalledTimes(1);
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
			expect(screen.getByText(/failed to send message\. try again\. network error/i)).toBeInTheDocument();
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
		const input = screen.getByPlaceholderText('Type a message...');
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
