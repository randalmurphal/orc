export function KnowledgeEmptyState() {
	return (
		<div className="knowledge-empty-state" role="status">
			<h1>Knowledge Layer Not Running</h1>
			<p>
				Run <code>orc knowledge start && orc index</code> to get started.
			</p>
		</div>
	);
}
