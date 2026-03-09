import { create } from '@bufbuild/protobuf';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useDocumentTitle } from '@/hooks';
import { knowledgeClient, threadClient } from '@/lib/client';
import { useCurrentProjectId } from '@/stores';
import { useThreadStore } from '@/stores/threadStore';
import {
	GetKnowledgeInsightsRequestSchema,
	GetKnowledgeInsightsResponse,
	GetKnowledgeStatusRequestSchema,
	KnowledgeStatusSchema,
	KnowledgeStatus,
	QueryKnowledgeRequestSchema,
	type KnowledgeResult,
} from '@/gen/orc/v1/knowledge_pb';
import { SendThreadMessageRequestSchema } from '@/gen/orc/v1/thread_pb';
import { KnowledgeResultCard } from '@/components/knowledge/KnowledgeResultCard';
import { KnowledgeInsightsPanel } from '@/components/knowledge/KnowledgeInsightsPanel';
import { KnowledgeEmptyState } from '@/components/knowledge/KnowledgeEmptyState';
import './KnowledgePage.css';

type PresetOption = 'standard' | 'fast' | 'deep' | 'graph-first' | 'recency';

const PRESET_OPTIONS: { value: PresetOption; label: string }[] = [
	{ value: 'standard', label: 'standard' },
	{ value: 'fast', label: 'fast' },
	{ value: 'deep', label: 'deep' },
	{ value: 'graph-first', label: 'graph-first' },
	{ value: 'recency', label: 'recency' },
];

function presetToApiValue(preset: PresetOption): string {
	if (preset === 'graph-first') {
		return 'graph_first';
	}
	return preset;
}

function buildDiscussionContext(result: KnowledgeResult): string {
	const lines = [
		'Please discuss this knowledge result with me.',
		`Title: ${result.title || result.id}`,
		`Type: ${result.type}`,
		`Score: ${result.score.toFixed(2)}`,
	];

	if (result.filePath) {
		lines.push(`File: ${result.filePath}`);
	}
	if (result.startLine > 0) {
		lines.push(`Line: ${result.startLine}`);
	}
	if (result.summary) {
		lines.push(`Summary: ${result.summary}`);
	}
	if (result.content) {
		lines.push(`Content: ${result.content.slice(0, 1200)}`);
	}

	return lines.join('\n');
}

function defaultStatus(): KnowledgeStatus {
	return create(KnowledgeStatusSchema, {});
}

export function KnowledgePage() {
	useDocumentTitle('Knowledge');

	const projectId = useCurrentProjectId();
	const createThread = useThreadStore((state) => state.createThread);

	const [query, setQuery] = useState('');
	const [preset, setPreset] = useState<PresetOption>('standard');
	const [results, setResults] = useState<KnowledgeResult[]>([]);
	const [tokensUsed, setTokensUsed] = useState(0);
	const [status, setStatus] = useState<KnowledgeStatus>(defaultStatus);
	const [insights, setInsights] = useState<GetKnowledgeInsightsResponse | null>(null);
	const [statusLoading, setStatusLoading] = useState(true);
	const [insightsLoading, setInsightsLoading] = useState(false);
	const [searching, setSearching] = useState(false);
	const [statusError, setStatusError] = useState<string | null>(null);
	const [searchError, setSearchError] = useState<string | null>(null);
	const [discussionError, setDiscussionError] = useState<string | null>(null);
	const [discussingResultId, setDiscussingResultId] = useState<string | null>(null);

	const loadInsights = useCallback(async () => {
		if (!projectId) {
			setInsights(null);
			return;
		}

		setInsightsLoading(true);
		try {
			const response = await knowledgeClient.getInsights(
				create(GetKnowledgeInsightsRequestSchema, { projectId })
			);
			setInsights(response);
		} catch {
			setInsights(null);
		} finally {
			setInsightsLoading(false);
		}
	}, [projectId]);

	const loadStatus = useCallback(async () => {
		setStatusLoading(true);
		setStatusError(null);

		try {
			const response = await knowledgeClient.getStatus(
				create(GetKnowledgeStatusRequestSchema, { projectId: projectId ?? '' })
			);
			const nextStatus = response.status ?? defaultStatus();
			setStatus(nextStatus);

			if (nextStatus.running) {
				await loadInsights();
			} else {
				setInsights(null);
			}
		} catch (err) {
			setStatus(defaultStatus());
			setStatusError(err instanceof Error ? err.message : 'Failed to load knowledge status');
			setInsights(null);
		} finally {
			setStatusLoading(false);
		}
	}, [loadInsights, projectId]);

	useEffect(() => {
		void loadStatus();
	}, [loadStatus]);

	const handleSearch = useCallback(async () => {
		const trimmed = query.trim();
		if (trimmed === '' || !status.running) {
			return;
		}

		setSearching(true);
		setSearchError(null);

		try {
			const response = await knowledgeClient.query(
				create(QueryKnowledgeRequestSchema, {
					projectId: projectId ?? '',
					query: trimmed,
					preset: presetToApiValue(preset),
					limit: 20,
					maxTokens: 4000,
				})
			);
			setResults(response.results);
			setTokensUsed(response.tokensUsed);
		} catch (err) {
			setResults([]);
			setTokensUsed(0);
			setSearchError(err instanceof Error ? err.message : 'Knowledge query failed');
		} finally {
			setSearching(false);
		}
	}, [preset, projectId, query, status.running]);

	const handleDiscuss = useCallback(async (result: KnowledgeResult) => {
		if (!projectId) {
			return;
		}

		setDiscussionError(null);
		setDiscussingResultId(result.id);

		try {
			const threadTitle = `Knowledge: ${result.title || result.filePath || result.id}`;
			const thread = await createThread(projectId, threadTitle);
			if (!thread) {
				throw new Error('thread creation failed');
			}

			await threadClient.sendMessage(
				create(SendThreadMessageRequestSchema, {
					projectId,
					threadId: thread.id,
					content: buildDiscussionContext(result),
				})
			);
		} catch (err) {
			setDiscussionError(err instanceof Error ? err.message : 'Failed to start discussion');
		} finally {
			setDiscussingResultId(null);
		}
	}, [createThread, projectId]);

	const resultSummary = useMemo(() => {
		if (results.length === 0) {
			return 'No results yet';
		}
		return `${results.length} results · ${tokensUsed} tokens`;
	}, [results.length, tokensUsed]);

	if (statusLoading) {
		return (
			<div className="knowledge-page__loading">
				Loading knowledge status...
			</div>
		);
	}

	if (statusError) {
		return (
			<div className="knowledge-page__error">
				<p>{statusError}</p>
				<button type="button" onClick={() => void loadStatus()}>
					Retry
				</button>
			</div>
		);
	}

	if (!status.running) {
		return <KnowledgeEmptyState />;
	}

	return (
		<div className="knowledge-page">
			<section className="knowledge-page__main">
				<header className="knowledge-page__header">
					<h1>Knowledge Exploration</h1>
					<p>Search the indexed graph for implementation context and decisions.</p>
				</header>

				<div className="knowledge-page__controls">
					<input
						type="text"
						value={query}
						placeholder="Ask the knowledge graph about this codebase"
						onChange={(event) => setQuery(event.target.value)}
						onKeyDown={(event) => {
							if (event.key === 'Enter') {
								void handleSearch();
							}
						}}
						aria-label="Knowledge query input"
					/>
					<select
						value={preset}
						onChange={(event) => setPreset(event.target.value as PresetOption)}
						aria-label="Knowledge search preset"
					>
						{PRESET_OPTIONS.map((option) => (
							<option key={option.value} value={option.value}>
								{option.label}
							</option>
						))}
					</select>
					<button type="button" onClick={() => void handleSearch()} disabled={searching}>
						{searching ? 'Searching...' : 'Search'}
					</button>
				</div>

				<div className="knowledge-page__summary">{resultSummary}</div>

				{searchError && <div className="knowledge-page__alert">{searchError}</div>}
				{discussionError && <div className="knowledge-page__alert">{discussionError}</div>}

				<div className="knowledge-page__results">
					{results.map((result) => (
						<KnowledgeResultCard
							key={result.id}
							result={result}
							discussing={discussingResultId === result.id}
							onDiscuss={handleDiscuss}
						/>
					))}
				</div>
			</section>

			<KnowledgeInsightsPanel
				insights={insights}
				loading={insightsLoading}
			/>
		</div>
	);
}
