/**
 * Integration test: ContextPanel → DiscussionPanel invocation wiring
 *
 * Verifies that ContextPanel actually renders DiscussionPanel's chat UI
 * when mode=discussion and threadId is provided. The existing unit test
 * only checks that the discussion tab has aria-selected=true — which
 * would pass even if DiscussionPanel was replaced with a stub or never
 * rendered.
 *
 * This test asserts the PRESENCE of DiscussionPanel-specific elements
 * (chat input, send button) — elements that only exist if ContextPanel
 * actually imports and renders DiscussionPanel.
 *
 * Deletion test: If you remove the DiscussionPanel import from
 * ContextPanel.tsx, these tests fail (no textbox or send button).
 *
 * Production path: ContextPanel → DiscussionPanel (mode=discussion + threadId)
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ContextPanel } from './ContextPanel';
import { AppShellProvider } from './AppShellContext';
import { EventProvider } from '@/hooks';
import { TooltipProvider } from '@/components/ui';
import { useThreadStore } from '@/stores/threadStore';

// Mock threadClient — DiscussionPanel uses it to send messages
vi.mock('@/lib/client', () => ({
	threadClient: {
		listThreads: vi.fn(),
		createThread: vi.fn(),
		getThread: vi.fn(),
		sendMessage: vi.fn(),
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
				<EventProvider autoConnect={false}>
					<AppShellProvider>
						{children}
					</AppShellProvider>
				</EventProvider>
			</TooltipProvider>
		</MemoryRouter>
	);
}

function renderWithProviders(ui: React.ReactElement) {
	return render(ui, { wrapper: TestWrapper });
}

beforeEach(() => {
	useThreadStore.getState().reset();
	localStorage.clear();
	vi.clearAllMocks();
});

afterEach(() => {
	vi.clearAllMocks();
	localStorage.clear();
});

// =============================================================================
// INVOCATION TESTS: ContextPanel actually renders DiscussionPanel
// =============================================================================

describe('ContextPanel → DiscussionPanel invocation', () => {
	it('should render DiscussionPanel chat input when mode is discussion with threadId', () => {
		renderWithProviders(
			<ContextPanel mode="discussion" threadId="thread-001" />
		);

		// The textbox comes from DiscussionPanel, not ContextPanel itself.
		// If ContextPanel doesn't render DiscussionPanel, this fails.
		expect(screen.getByRole('textbox')).toBeInTheDocument();
	});

	it('should render DiscussionPanel send button when mode is discussion with threadId', () => {
		renderWithProviders(
			<ContextPanel mode="discussion" threadId="thread-001" />
		);

		// Send button is a DiscussionPanel element — proves invocation, not registration.
		expect(screen.getByRole('button', { name: /send/i })).toBeInTheDocument();
	});

	it('should pass threadId to DiscussionPanel so it can send messages to correct thread', async () => {
		vi.mocked(threadClient.sendMessage).mockResolvedValue({
			userMessage: { id: BigInt(1), threadId: 'thread-042', role: 'user', content: 'test' },
			assistantMessage: { id: BigInt(2), threadId: 'thread-042', role: 'assistant', content: 'response' },
		} as never);

		renderWithProviders(
			<ContextPanel mode="discussion" threadId="thread-042" />
		);

		// Type a message and send — this exercises the full ContextPanel → DiscussionPanel → threadClient chain
		const input = screen.getByRole('textbox');
		fireEvent.change(input, { target: { value: 'Hello from context panel' } });
		fireEvent.click(screen.getByRole('button', { name: /send/i }));

		// Verify the message was sent to the correct thread
		await waitFor(() => {
			expect(threadClient.sendMessage).toHaveBeenCalledWith(
				expect.objectContaining({ threadId: 'thread-042' })
			);
		});
	});

	it('should NOT render chat input for non-discussion modes', () => {
		renderWithProviders(
			<ContextPanel mode="diff" />
		);

		// Diff mode should NOT show DiscussionPanel's chat input
		expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
	});

	it('should NOT render chat input when mode is discussion but no threadId', () => {
		renderWithProviders(
			<ContextPanel mode="discussion" />
		);

		// Without a threadId, DiscussionPanel should not render its chat UI
		// (ContextPanel should show "select a thread" instead)
		expect(screen.queryByRole('button', { name: /send/i })).not.toBeInTheDocument();
	});
});
