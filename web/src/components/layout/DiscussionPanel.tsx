import { useState, useRef, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import {
	GetThreadRequestSchema,
	PromoteThreadDecisionDraftRequestSchema,
	PromoteThreadRecommendationDraftRequestSchema,
	SendThreadMessageRequestSchema,
	type ThreadDecisionDraft,
	type ThreadLink,
	type ThreadMessage,
	type ThreadRecommendationDraft,
} from '@/gen/orc/v1/thread_pb';
import { RecommendationKind } from '@/gen/orc/v1/recommendation_pb';
import { threadClient } from '@/lib/client';
import './DiscussionPanel.css';

interface DiscussionPanelProps {
	threadId: string;
	projectId: string;
	messages?: ThreadMessage[];
}

const EMPTY_MESSAGES: ThreadMessage[] = [];
const EMPTY_LINKS: ThreadLink[] = [];
const EMPTY_RECOMMENDATION_DRAFTS: ThreadRecommendationDraft[] = [];
const EMPTY_DECISION_DRAFTS: ThreadDecisionDraft[] = [];

export function DiscussionPanel({ threadId, projectId, messages: initialMessages }: DiscussionPanelProps) {
	const stableMessages = initialMessages ?? EMPTY_MESSAGES;
	const [messages, setMessages] = useState<ThreadMessage[]>(stableMessages);
	const [links, setLinks] = useState<ThreadLink[]>(EMPTY_LINKS);
	const [recommendationDrafts, setRecommendationDrafts] = useState<ThreadRecommendationDraft[]>(EMPTY_RECOMMENDATION_DRAFTS);
	const [decisionDrafts, setDecisionDrafts] = useState<ThreadDecisionDraft[]>(EMPTY_DECISION_DRAFTS);
	const [input, setInput] = useState('');
	const [sending, setSending] = useState(false);
	const [loadingThread, setLoadingThread] = useState(initialMessages === undefined);
	const [promotingDraftId, setPromotingDraftId] = useState<string | null>(null);
	const [error, setError] = useState<string | null>(null);
	const messagesEndRef = useRef<HTMLDivElement>(null);

	// Sync initial messages when prop changes (stable reference prevents loops)
	useEffect(() => {
		setMessages(stableMessages);
	}, [stableMessages]);

	const syncThreadState = useCallback((thread?: {
		messages?: ThreadMessage[];
		links?: ThreadLink[];
		recommendationDrafts?: ThreadRecommendationDraft[];
		decisionDrafts?: ThreadDecisionDraft[];
	}) => {
		setMessages(thread?.messages ?? EMPTY_MESSAGES);
		setLinks(thread?.links ?? EMPTY_LINKS);
		setRecommendationDrafts(thread?.recommendationDrafts ?? EMPTY_RECOMMENDATION_DRAFTS);
		setDecisionDrafts(thread?.decisionDrafts ?? EMPTY_DECISION_DRAFTS);
	}, []);

	useEffect(() => {
		if (initialMessages !== undefined) {
			setLoadingThread(false);
			return;
		}

		let cancelled = false;
		const loadThread = async () => {
			setLoadingThread(true);
			try {
				const response = await threadClient.getThread(
					create(GetThreadRequestSchema, {
						projectId,
						threadId,
					})
				);
				if (cancelled) {
					return;
				}
				syncThreadState(response.thread);
				setError(null);
			} catch {
				if (!cancelled) {
					setError('Failed to load thread history. Try again.');
				}
			} finally {
				if (!cancelled) {
					setLoadingThread(false);
				}
			}
		};

		void loadThread();
		return () => {
			cancelled = true;
		};
	}, [initialMessages, projectId, syncThreadState, threadId]);

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

	const promoteRecommendationDraft = useCallback(async (draftId: string) => {
		setPromotingDraftId(draftId);
		setError(null);
		try {
			const response = await threadClient.promoteRecommendationDraft(
				create(PromoteThreadRecommendationDraftRequestSchema, {
					projectId,
					threadId,
					draftId,
					promotedBy: 'operator',
				})
			);
			syncThreadState(response.thread);
		} catch {
			setError('Failed to promote recommendation draft. Try again.');
		} finally {
			setPromotingDraftId(null);
		}
	}, [projectId, syncThreadState, threadId]);

	const promoteDecisionDraft = useCallback(async (draftId: string) => {
		setPromotingDraftId(draftId);
		setError(null);
		try {
			const response = await threadClient.promoteDecisionDraft(
				create(PromoteThreadDecisionDraftRequestSchema, {
					projectId,
					threadId,
					draftId,
					promotedBy: 'operator',
				})
			);
			syncThreadState(response.thread);
		} catch {
			setError('Failed to promote decision draft. Try again.');
		} finally {
			setPromotingDraftId(null);
		}
	}, [projectId, syncThreadState, threadId]);

	const handleKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			sendMessage();
		}
	};

	const isDisabled = !input.trim() || sending;

	return (
		<div className="discussion-panel">
			{(links.length > 0 || recommendationDrafts.length > 0 || decisionDrafts.length > 0) && (
				<div className="discussion-panel__context">
					{links.length > 0 && (
						<div className="discussion-panel__section">
							<div className="discussion-panel__section-label">Current context</div>
							<div className="discussion-panel__link-list">
								{links.map((link) => (
									<span key={`${link.linkType}:${link.targetId}`} className="discussion-panel__link-pill">
										<span className="discussion-panel__link-type">{link.linkType}</span>
										<span>{link.title || link.targetId}</span>
									</span>
								))}
							</div>
						</div>
					)}

					{recommendationDrafts.length > 0 && (
						<div className="discussion-panel__section">
							<div className="discussion-panel__section-label">Recommendation drafts</div>
							<div className="discussion-panel__draft-list">
								{recommendationDrafts.map((draft) => (
									<article key={draft.id} className="discussion-panel__draft-card">
										<div className="discussion-panel__draft-meta">
											<span>{recommendationKindLabel(draft.kind)}</span>
											<span>{draft.status}</span>
										</div>
										<h3>{draft.title}</h3>
										<p>{draft.summary}</p>
										<p className="discussion-panel__draft-detail">
											<strong>Proposed action</strong> {draft.proposedAction}
										</p>
										<button
											type="button"
											onClick={() => promoteRecommendationDraft(draft.id)}
											disabled={promotingDraftId === draft.id || draft.status !== 'draft'}
										>
											Promote to Inbox
										</button>
									</article>
								))}
							</div>
						</div>
					)}

					{decisionDrafts.length > 0 && (
						<div className="discussion-panel__section">
							<div className="discussion-panel__section-label">Decision drafts</div>
							<div className="discussion-panel__draft-list">
								{decisionDrafts.map((draft) => (
									<article key={draft.id} className="discussion-panel__draft-card">
										<div className="discussion-panel__draft-meta">
											<span>{draft.initiativeId || 'thread initiative'}</span>
											<span>{draft.status}</span>
										</div>
										<h3>{draft.decision}</h3>
										{draft.rationale && <p>{draft.rationale}</p>}
										<button
											type="button"
											onClick={() => promoteDecisionDraft(draft.id)}
											disabled={promotingDraftId === draft.id || draft.status !== 'draft'}
										>
											Promote Decision
										</button>
									</article>
								))}
							</div>
						</div>
					)}
				</div>
			)}

			<div className="discussion-panel__messages">
				{loadingThread && messages.length === 0 && (
					<div className="discussion-panel__empty">Loading thread history...</div>
				)}
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
				{!loadingThread && messages.length === 0 && (
					<div className="discussion-panel__empty">No messages yet. Start the thread here.</div>
				)}
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

function recommendationKindLabel(kind: RecommendationKind): string {
	switch (kind) {
		case RecommendationKind.CLEANUP:
			return 'cleanup';
		case RecommendationKind.RISK:
			return 'risk';
		case RecommendationKind.DECISION_REQUEST:
			return 'decision request';
		default:
			return 'follow-up';
	}
}
