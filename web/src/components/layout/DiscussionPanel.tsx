import { useState, useRef, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import { SendThreadMessageRequestSchema, type ThreadMessage } from '@/gen/orc/v1/thread_pb';
import { threadClient } from '@/lib/client';
import './DiscussionPanel.css';

interface DiscussionPanelProps {
	threadId: string;
	projectId: string;
	messages?: ThreadMessage[];
}

const EMPTY_MESSAGES: ThreadMessage[] = [];

export function DiscussionPanel({ threadId, projectId, messages: initialMessages }: DiscussionPanelProps) {
	const stableMessages = initialMessages ?? EMPTY_MESSAGES;
	const [messages, setMessages] = useState<ThreadMessage[]>(stableMessages);
	const [input, setInput] = useState('');
	const [sending, setSending] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const messagesEndRef = useRef<HTMLDivElement>(null);

	// Sync initial messages when prop changes (stable reference prevents loops)
	useEffect(() => {
		setMessages(stableMessages);
	}, [stableMessages]);

	// Scroll to bottom on new messages
	useEffect(() => {
		messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
	}, [messages]);

	const sendMessage = useCallback(async () => {
		const content = input.trim();
		if (!content || sending) return;

		setError(null);
		setSending(true);

		// Optimistic: add user message immediately
		const optimisticMsg = {
			id: BigInt(Date.now()),
			threadId,
			role: 'user',
			content,
		} as ThreadMessage;
		setMessages((prev) => [...prev, optimisticMsg]);
		setInput('');

		try {
			const response = await threadClient.sendMessage(
				create(SendThreadMessageRequestSchema, {
					projectId,
					threadId,
					content,
				})
			);

			// Replace optimistic message with real ones
			setMessages((prev) => {
				const withoutOptimistic = prev.filter((m) => m !== optimisticMsg);
				const newMessages = [...withoutOptimistic];
				if (response.userMessage) newMessages.push(response.userMessage);
				if (response.assistantMessage) newMessages.push(response.assistantMessage);
				return newMessages;
			});
		} catch {
			// Remove optimistic message and restore input
			setMessages((prev) => prev.filter((m) => m !== optimisticMsg));
			setInput(content);
			setError('Failed to send message. Try again.');
		} finally {
			setSending(false);
		}
	}, [input, sending, threadId, projectId]);

	const handleKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			sendMessage();
		}
	};

	const isDisabled = !input.trim() || sending;

	return (
		<div className="discussion-panel">
			<div className="discussion-panel__messages">
				{messages.map((msg, i) => (
					<div
						key={String(msg.id ?? i)}
						className={`discussion-panel__message discussion-panel__message--${msg.role}`}
					>
						<div className="discussion-panel__bubble">
							{msg.content}
						</div>
					</div>
				))}
				<div ref={messagesEndRef} />
			</div>

			{error && (
				<div className="discussion-panel__error">
					{error}
				</div>
			)}

			<div className="discussion-panel__input-area">
				<input
					type="text"
					value={input}
					onChange={(e) => setInput(e.target.value)}
					onKeyDown={handleKeyDown}
					placeholder="Type a message..."
					disabled={sending}
				/>
				<button
					onClick={sendMessage}
					disabled={isDisabled}
					aria-label="Send"
				>
					Send
				</button>
			</div>
		</div>
	);
}
