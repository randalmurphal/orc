import type { GetKnowledgeInsightsResponse } from '@/gen/orc/v1/knowledge_pb';

interface KnowledgeInsightsPanelProps {
	insights: GetKnowledgeInsightsResponse | null;
	loading: boolean;
}

export function KnowledgeInsightsPanel({ insights, loading }: KnowledgeInsightsPanelProps) {
	const hotFiles = insights?.hotFiles ?? [];
	const recurringPatterns = insights?.recurringPatterns ?? [];
	const constitutionUpdates = insights?.constitutionUpdates ?? [];
	const hasSections =
		hotFiles.length > 0 ||
		recurringPatterns.length > 0 ||
		constitutionUpdates.length > 0;

	return (
		<aside className="knowledge-insights-panel" aria-label="Knowledge insights">
			<h2>Insights</h2>
			{loading && <p className="knowledge-insights-panel__loading">Loading insights...</p>}

			{!loading && !hasSections && (
				<p className="knowledge-insights-panel__empty">No insights available yet.</p>
			)}

			{hotFiles.length > 0 && (
				<section>
					<h3>Hot Files</h3>
					<ul>
						{hotFiles.map((item) => (
							<li key={`${item.filePath}-${item.hitCount}`}>
								<span>{item.filePath}</span>
								<span>{item.hitCount}</span>
							</li>
						))}
					</ul>
				</section>
			)}

			{recurringPatterns.length > 0 && (
				<section>
					<h3>Recurring Patterns</h3>
					<ul>
						{recurringPatterns.map((item) => (
							<li key={`${item.name}-${item.memberCount}`}>
								<span>{item.name}</span>
								<span>{item.memberCount}</span>
							</li>
						))}
					</ul>
				</section>
			)}

			{constitutionUpdates.length > 0 && (
				<section>
					<h3>Constitution Updates</h3>
					<ul>
						{constitutionUpdates.map((item) => (
							<li key={`${item.title}-${item.source}`}>
								<p>{item.title}</p>
								<p>{item.summary}</p>
							</li>
						))}
					</ul>
				</section>
			)}
		</aside>
	);
}
