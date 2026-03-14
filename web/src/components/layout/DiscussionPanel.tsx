import { useState, useRef, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import {
	AddThreadLinkRequestSchema,
	CreateThreadDecisionDraftRequestSchema,
	CreateThreadRecommendationDraftRequestSchema,
	GetThreadRequestSchema,
	PromoteThreadRecommendationDraftRequestSchema,
	SendThreadMessageRequestSchema,
	type ThreadDecisionDraft,
	type ThreadLink,
	type ThreadMessage,
	type ThreadRecommendationDraft,
} from '@/gen/orc/v1/thread_pb';
import { RecommendationKind } from '@/gen/orc/v1/recommendation_pb';
import { useEvents } from '@/hooks/useEvents';
import { threadClient } from '@/lib/client';
import { recommendationKindLabel } from '@/lib/recommendations';
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
const DEFAULT_LINK_TYPE = 'file';
const DEFAULT_RECOMMENDATION_KIND = 'follow_up';

export function DiscussionPanel({ threadId, projectId, messages: initialMessages }: DiscussionPanelProps) {
	const { onEvent } = useEvents();
	const stableMessages = initialMessages ?? EMPTY_MESSAGES;
	const [messages, setMessages] = useState<ThreadMessage[]>(stableMessages);
	const [links, setLinks] = useState<ThreadLink[]>(EMPTY_LINKS);
	const [recommendationDrafts, setRecommendationDrafts] = useState<ThreadRecommendationDraft[]>(EMPTY_RECOMMENDATION_DRAFTS);
	const [decisionDrafts, setDecisionDrafts] = useState<ThreadDecisionDraft[]>(EMPTY_DECISION_DRAFTS);
	const [threadTaskId, setThreadTaskId] = useState('');
	const [threadInitiativeId, setThreadInitiativeId] = useState('');
	const [input, setInput] = useState('');
	const [linkType, setLinkType] = useState(DEFAULT_LINK_TYPE);
	const [linkTargetId, setLinkTargetId] = useState('');
	const [linkTitle, setLinkTitle] = useState('');
	const [recommendationKind, setRecommendationKind] = useState(DEFAULT_RECOMMENDATION_KIND);
	const [recommendationTitle, setRecommendationTitle] = useState('');
	const [recommendationSummary, setRecommendationSummary] = useState('');
	const [recommendationAction, setRecommendationAction] = useState('');
	const [recommendationEvidence, setRecommendationEvidence] = useState('');
	const [decisionInitiativeId, setDecisionInitiativeId] = useState('');
	const [decisionText, setDecisionText] = useState('');
	const [decisionRationale, setDecisionRationale] = useState('');
	const [sending, setSending] = useState(false);
	const [addingLink, setAddingLink] = useState(false);
	const [creatingDraft, setCreatingDraft] = useState<'recommendation' | 'decision' | null>(null);
	const [loadingThread, setLoadingThread] = useState(true);
	const [promotingDraftId, setPromotingDraftId] = useState<string | null>(null);
	const [error, setError] = useState<string | null>(null);
	const messagesEndRef = useRef<HTMLDivElement>(null);
	const latestThreadRequestIdRef = useRef(0);
	const nextActionTokenRef = useRef(0);
	const sendingActionTokenRef = useRef(0);
	const addingLinkActionTokenRef = useRef(0);
	const creatingDraftActionTokenRef = useRef(0);
	const promotingDraftActionTokenRef = useRef(0);
	const decisionInitiativeDirtyRef = useRef(false);
	const currentThreadKeyRef = useRef(threadRequestKey(projectId, threadId));
	currentThreadKeyRef.current = threadRequestKey(projectId, threadId);

	const beginThreadRequest = useCallback(() => {
		const requestId = latestThreadRequestIdRef.current + 1;
		latestThreadRequestIdRef.current = requestId;
		return {
			requestId,
			threadKey: currentThreadKeyRef.current,
		};
	}, []);

	const beginActionToken = useCallback(() => {
		nextActionTokenRef.current += 1;
		return nextActionTokenRef.current;
	}, []);

	const isCurrentThreadRequest = useCallback((requestId: number, threadKey: string) => (
		latestThreadRequestIdRef.current === requestId && currentThreadKeyRef.current === threadKey
	), []);

	useEffect(() => {
		setMessages(initialMessages ?? EMPTY_MESSAGES);
		setLinks(EMPTY_LINKS);
		setRecommendationDrafts(EMPTY_RECOMMENDATION_DRAFTS);
		setDecisionDrafts(EMPTY_DECISION_DRAFTS);
		setThreadTaskId('');
		setThreadInitiativeId('');
		setInput('');
		setLinkType(DEFAULT_LINK_TYPE);
		setLinkTargetId('');
		setLinkTitle('');
		setRecommendationKind(DEFAULT_RECOMMENDATION_KIND);
		setRecommendationTitle('');
		setRecommendationSummary('');
		setRecommendationAction('');
		setRecommendationEvidence('');
		setDecisionInitiativeId('');
		setDecisionText('');
		setDecisionRationale('');
		setSending(false);
		setAddingLink(false);
		setCreatingDraft(null);
		setLoadingThread(true);
		setPromotingDraftId(null);
		setError(null);
		sendingActionTokenRef.current = 0;
		addingLinkActionTokenRef.current = 0;
		creatingDraftActionTokenRef.current = 0;
		promotingDraftActionTokenRef.current = 0;
		decisionInitiativeDirtyRef.current = false;
	}, [initialMessages, projectId, threadId]);

	useEffect(() => {
		setMessages(stableMessages);
	}, [stableMessages]);

	const syncThreadState = useCallback((thread?: {
		taskId?: string;
		initiativeId?: string;
		messages?: ThreadMessage[];
		links?: ThreadLink[];
		recommendationDrafts?: ThreadRecommendationDraft[];
		decisionDrafts?: ThreadDecisionDraft[];
	}) => {
		if (thread?.messages !== undefined) {
			setMessages(thread.messages);
		}
		setLinks(thread?.links ?? EMPTY_LINKS);
		setRecommendationDrafts(thread?.recommendationDrafts ?? EMPTY_RECOMMENDATION_DRAFTS);
		setDecisionDrafts(thread?.decisionDrafts ?? EMPTY_DECISION_DRAFTS);
		setThreadTaskId(thread?.taskId ?? '');
		setThreadInitiativeId(thread?.initiativeId ?? '');
		if (!decisionInitiativeDirtyRef.current) {
			setDecisionInitiativeId(thread?.initiativeId ?? '');
		}
	}, []);

	const applyThreadMutationState = useCallback((
		request: { requestId: number; threadKey: string },
		thread?: {
			taskId?: string;
			initiativeId?: string;
			messages?: ThreadMessage[];
			links?: ThreadLink[];
			recommendationDrafts?: ThreadRecommendationDraft[];
			decisionDrafts?: ThreadDecisionDraft[];
		}
	) => {
		if (!isCurrentThreadRequest(request.requestId, request.threadKey)) {
			return false;
		}
		syncThreadState(thread);
		return true;
	}, [isCurrentThreadRequest, syncThreadState]);

	const reloadThreadState = useCallback(async () => {
		const request = beginThreadRequest();
		try {
			const response = await threadClient.getThread(
				create(GetThreadRequestSchema, {
					projectId,
					threadId,
				})
			);
			if (!isCurrentThreadRequest(request.requestId, request.threadKey)) {
				return false;
			}
			syncThreadState(response.thread);
			setLoadingThread(false);
			return true;
		} catch (err) {
			if (!isCurrentThreadRequest(request.requestId, request.threadKey)) {
				return false;
			}
			throw err;
		}
	}, [beginThreadRequest, isCurrentThreadRequest, projectId, syncThreadState, threadId]);

	useEffect(() => {
		let cancelled = false;
		const loadThread = async () => {
			setLoadingThread(initialMessages === undefined);
			let shouldFinishLoading = true;
			try {
				shouldFinishLoading = await reloadThreadState();
			} catch (err) {
				if (!cancelled) {
					setError(withErrorDetails('Failed to load thread history. Try again.', err));
				}
			} finally {
				if (!cancelled && shouldFinishLoading) {
					setLoadingThread(false);
				}
			}
		};

		void loadThread();
		return () => {
			cancelled = true;
		};
	}, [initialMessages, reloadThreadState]);

	useEffect(() => onEvent((event) => {
		if (event.projectId && event.projectId !== projectId) {
			return;
		}
		if (event.payload.case !== 'threadUpdated') {
			return;
		}
		if (event.payload.value.threadId !== threadId) {
			return;
		}

		void reloadThreadState().catch((err) => {
			setError(withErrorDetails('Failed to refresh thread state. Try again.', err));
			setLoadingThread(false);
		});
	}), [onEvent, projectId, reloadThreadState, threadId]);

	// Scroll to bottom on new messages
	useEffect(() => {
		messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
	}, [messages]);

	const sendMessage = useCallback(async () => {
		const content = input.trim();
		if (!content || sending) return;

		const request = beginThreadRequest();
		const actionToken = beginActionToken();
		setError(null);
		setSending(true);
		sendingActionTokenRef.current = actionToken;

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
			if (!isCurrentThreadRequest(request.requestId, request.threadKey)) {
				return;
			}

			// Replace optimistic message with real ones
			setMessages((prev) => {
				const withoutOptimistic = prev.filter((m) => m !== optimisticMsg);
				return appendUniqueMessages(withoutOptimistic, [
					response.userMessage,
					response.assistantMessage,
				]);
			});
		} catch (err) {
			if (!isCurrentThreadRequest(request.requestId, request.threadKey)) {
				return;
			}
			// Remove optimistic message and restore input
			setMessages((prev) => prev.filter((m) => m !== optimisticMsg));
			setInput(content);
			setError(withErrorDetails('Failed to send message. Try again.', err));
		} finally {
			if (sendingActionTokenRef.current === actionToken) {
				sendingActionTokenRef.current = 0;
				setSending(false);
			}
		}
	}, [beginActionToken, beginThreadRequest, input, isCurrentThreadRequest, projectId, sending, threadId]);

	const promoteRecommendationDraft = useCallback(async (draftId: string) => {
		const request = beginThreadRequest();
		const actionToken = beginActionToken();
		promotingDraftActionTokenRef.current = actionToken;
		setPromotingDraftId(draftId);
		setError(null);
		try {
			const response = await threadClient.promoteRecommendationDraft(
				create(PromoteThreadRecommendationDraftRequestSchema, {
					projectId,
					threadId,
					draftId,
					promotedBy: '',
				})
			);
			applyThreadMutationState(request, response.thread);
		} catch (err) {
			if (isCurrentThreadRequest(request.requestId, request.threadKey)) {
				setError(withErrorDetails('Failed to promote recommendation draft. Try again.', err));
			}
		} finally {
			if (promotingDraftActionTokenRef.current === actionToken) {
				promotingDraftActionTokenRef.current = 0;
				setPromotingDraftId(null);
			}
		}
	}, [applyThreadMutationState, beginActionToken, beginThreadRequest, isCurrentThreadRequest, projectId, threadId]);

	const addLink = useCallback(async () => {
		const targetId = linkTargetId.trim();
		if (!targetId || addingLink) {
			return;
		}

		const request = beginThreadRequest();
		const actionToken = beginActionToken();
		addingLinkActionTokenRef.current = actionToken;
		setAddingLink(true);
		setError(null);
		try {
			const response = await threadClient.addLink(
				create(AddThreadLinkRequestSchema, {
					projectId,
					threadId,
					link: {
						linkType,
						targetId,
						title: linkTitle.trim(),
					},
				})
			);
			if (applyThreadMutationState(request, response.thread)) {
				setLinkTargetId('');
				setLinkTitle('');
			}
		} catch (err) {
			if (isCurrentThreadRequest(request.requestId, request.threadKey)) {
				setError(withErrorDetails('Failed to add linked context. Try again.', err));
			}
		} finally {
			if (addingLinkActionTokenRef.current === actionToken) {
				addingLinkActionTokenRef.current = 0;
				setAddingLink(false);
			}
		}
	}, [addingLink, applyThreadMutationState, beginActionToken, beginThreadRequest, isCurrentThreadRequest, linkTargetId, linkTitle, linkType, projectId, threadId]);

	const createRecommendationDraft = useCallback(async () => {
		if (creatingDraft !== null) {
			return;
		}

		const kind = recommendationKindFromValue(recommendationKind);
		if (
			kind === undefined ||
			!recommendationTitle.trim() ||
			!recommendationSummary.trim() ||
			!recommendationAction.trim() ||
			!recommendationEvidence.trim()
		) {
			setError('Recommendation drafts need kind, title, summary, proposed action, and evidence.');
			return;
		}

		const request = beginThreadRequest();
		const actionToken = beginActionToken();
		creatingDraftActionTokenRef.current = actionToken;
		setCreatingDraft('recommendation');
		setError(null);
		try {
			const response = await threadClient.createRecommendationDraft(
				create(CreateThreadRecommendationDraftRequestSchema, {
					projectId,
					threadId,
					draft: {
						kind,
						title: recommendationTitle.trim(),
						summary: recommendationSummary.trim(),
						proposedAction: recommendationAction.trim(),
						evidence: recommendationEvidence.trim(),
					},
				})
			);
			if (applyThreadMutationState(request, response.thread)) {
				setRecommendationKind(DEFAULT_RECOMMENDATION_KIND);
				setRecommendationTitle('');
				setRecommendationSummary('');
				setRecommendationAction('');
				setRecommendationEvidence('');
			}
		} catch (err) {
			if (isCurrentThreadRequest(request.requestId, request.threadKey)) {
				setError(withErrorDetails('Failed to create recommendation draft. Try again.', err));
			}
		} finally {
			if (creatingDraftActionTokenRef.current === actionToken) {
				creatingDraftActionTokenRef.current = 0;
				setCreatingDraft(null);
			}
		}
	}, [
		creatingDraft,
		applyThreadMutationState,
		beginActionToken,
		beginThreadRequest,
		isCurrentThreadRequest,
		projectId,
		recommendationAction,
		recommendationEvidence,
		recommendationKind,
		recommendationSummary,
		recommendationTitle,
		threadId,
	]);

	const createDecisionDraft = useCallback(async () => {
		if (creatingDraft !== null) {
			return;
		}
		if (!decisionText.trim()) {
			setError('Decision drafts need a decision.');
			return;
		}

		const request = beginThreadRequest();
		const actionToken = beginActionToken();
		creatingDraftActionTokenRef.current = actionToken;
		setCreatingDraft('decision');
		setError(null);
		try {
			const response = await threadClient.createDecisionDraft(
				create(CreateThreadDecisionDraftRequestSchema, {
					projectId,
					threadId,
					draft: {
						initiativeId: decisionInitiativeId.trim(),
						decision: decisionText.trim(),
						rationale: decisionRationale.trim(),
					},
				})
			);
			if (applyThreadMutationState(request, response.thread)) {
				setDecisionText('');
				setDecisionRationale('');
			}
		} catch (err) {
			if (isCurrentThreadRequest(request.requestId, request.threadKey)) {
				setError(withErrorDetails('Failed to create decision draft. Try again.', err));
			}
		} finally {
			if (creatingDraftActionTokenRef.current === actionToken) {
				creatingDraftActionTokenRef.current = 0;
				setCreatingDraft(null);
			}
		}
	}, [applyThreadMutationState, beginActionToken, beginThreadRequest, creatingDraft, decisionInitiativeId, decisionRationale, decisionText, isCurrentThreadRequest, projectId, threadId]);

	const handleKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			sendMessage();
		}
	};

	const isDisabled = !input.trim() || sending;

	return (
		<div className="discussion-panel">
			{!loadingThread && (
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
										<p className="discussion-panel__draft-detail">
											<strong>Next step</strong> Decision drafts stay in discussion until a human accepts them.
										</p>
									</article>
								))}
							</div>
						</div>
					)}

					<div className="discussion-panel__section">
						<div className="discussion-panel__section-label">Add linked context</div>
						<div className="discussion-panel__form-grid">
							<select
								aria-label="Link type"
								value={linkType}
								onChange={(e) => setLinkType(e.target.value)}
								disabled={addingLink}
							>
								<option value="task">Task</option>
								<option value="initiative">Initiative</option>
								<option value="recommendation">Recommendation</option>
								<option value="file">File</option>
								<option value="diff">Diff</option>
							</select>
							<input
								aria-label="Link target"
								value={linkTargetId}
								onChange={(e) => setLinkTargetId(e.target.value)}
								placeholder={linkTargetPlaceholder(linkType, threadTaskId, threadInitiativeId)}
								disabled={addingLink}
							/>
							<input
								aria-label="Link title"
								value={linkTitle}
								onChange={(e) => setLinkTitle(e.target.value)}
								placeholder="Optional title"
								disabled={addingLink}
							/>
							<button
								type="button"
								className="discussion-panel__secondary-button"
								onClick={addLink}
								disabled={addingLink || !linkTargetId.trim()}
							>
								Add Link
							</button>
						</div>
					</div>

					<div className="discussion-panel__section">
						<div className="discussion-panel__section-label">Create recommendation draft</div>
						<div className="discussion-panel__form-grid">
							<select
								aria-label="Recommendation kind"
								value={recommendationKind}
								onChange={(e) => setRecommendationKind(e.target.value)}
								disabled={creatingDraft !== null}
							>
								<option value="follow_up">Follow-up</option>
								<option value="cleanup">Cleanup</option>
								<option value="risk">Risk</option>
								<option value="decision_request">Decision request</option>
							</select>
							<input
								aria-label="Recommendation title"
								value={recommendationTitle}
								onChange={(e) => setRecommendationTitle(e.target.value)}
								placeholder="Draft title"
								disabled={creatingDraft !== null}
							/>
							<textarea
								aria-label="Recommendation summary"
								value={recommendationSummary}
								onChange={(e) => setRecommendationSummary(e.target.value)}
								placeholder="Summary"
								disabled={creatingDraft !== null}
								rows={2}
							/>
							<textarea
								aria-label="Recommendation proposed action"
								value={recommendationAction}
								onChange={(e) => setRecommendationAction(e.target.value)}
								placeholder="Proposed action"
								disabled={creatingDraft !== null}
								rows={2}
							/>
							<textarea
								aria-label="Recommendation evidence"
								value={recommendationEvidence}
								onChange={(e) => setRecommendationEvidence(e.target.value)}
								placeholder="Evidence"
								disabled={creatingDraft !== null}
								rows={2}
							/>
							<button
								type="button"
								className="discussion-panel__secondary-button"
								onClick={createRecommendationDraft}
								disabled={creatingDraft !== null}
							>
								Create Recommendation Draft
							</button>
						</div>
					</div>

					<div className="discussion-panel__section">
						<div className="discussion-panel__section-label">Create decision draft</div>
						<div className="discussion-panel__form-grid">
							<input
								aria-label="Decision initiative"
								value={decisionInitiativeId}
								onChange={(e) => {
									decisionInitiativeDirtyRef.current = true;
									setDecisionInitiativeId(e.target.value);
								}}
								placeholder={threadInitiativeId || 'Initiative ID'}
								disabled={creatingDraft !== null}
							/>
							<input
								aria-label="Decision text"
								value={decisionText}
								onChange={(e) => setDecisionText(e.target.value)}
								placeholder="Decision"
								disabled={creatingDraft !== null}
							/>
							<textarea
								aria-label="Decision rationale"
								value={decisionRationale}
								onChange={(e) => setDecisionRationale(e.target.value)}
								placeholder="Rationale"
								disabled={creatingDraft !== null}
								rows={2}
							/>
							<button
								type="button"
								className="discussion-panel__secondary-button"
								onClick={createDecisionDraft}
								disabled={creatingDraft !== null}
							>
								Create Decision Draft
							</button>
						</div>
					</div>
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

function recommendationKindFromValue(value: string): RecommendationKind | undefined {
	switch (value) {
		case 'cleanup':
			return RecommendationKind.CLEANUP;
		case 'risk':
			return RecommendationKind.RISK;
		case 'decision_request':
			return RecommendationKind.DECISION_REQUEST;
		case 'follow_up':
			return RecommendationKind.FOLLOW_UP;
		default:
			return undefined;
	}
}

function linkTargetPlaceholder(linkType: string, threadTaskId: string, threadInitiativeId: string): string {
	switch (linkType) {
		case 'task':
			return threadTaskId || 'TASK-001';
		case 'initiative':
			return threadInitiativeId || 'INIT-001';
		case 'recommendation':
			return 'REC-001';
		case 'diff':
			return `${threadTaskId || 'TASK-001'}:path/to/file.tsx`;
		case 'file':
		default:
			return 'path/to/file.tsx';
	}
}

function withErrorDetails(prefix: string, err: unknown): string {
	if (err instanceof Error && err.message) {
		return `${prefix} ${err.message}`;
	}
	return prefix;
}

function threadRequestKey(projectId: string, threadId: string): string {
	return `${projectId}:${threadId}`;
}

function appendUniqueMessages(existing: ThreadMessage[], additions: Array<ThreadMessage | undefined>): ThreadMessage[] {
	const seen = new Set(existing.map(messageKey));
	const next = [...existing];
	for (const message of additions) {
		if (!message) {
			continue;
		}
		const key = messageKey(message);
		if (seen.has(key)) {
			continue;
		}
		seen.add(key);
		next.push(message);
	}
	return next;
}

function messageKey(message: ThreadMessage): string {
	if (message.id !== undefined && message.id !== null) {
		return `id:${String(message.id)}`;
	}
	return `${message.role}:${message.content}`;
}
