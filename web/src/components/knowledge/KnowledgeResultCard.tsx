import { Link } from 'react-router-dom';
import { Icon } from '@/components/ui';
import type { IconName } from '@/components/ui/Icon';
import type { KnowledgeResult } from '@/gen/orc/v1/knowledge_pb';
import { KnowledgeResultType } from '@/gen/orc/v1/knowledge_pb';

interface KnowledgeResultCardProps {
	result: KnowledgeResult;
	discussing: boolean;
	onDiscuss: (result: KnowledgeResult) => void;
}

function resultTypeLabel(type: KnowledgeResultType): string {
	switch (type) {
		case KnowledgeResultType.CODE:
			return 'Code';
		case KnowledgeResultType.FINDING:
			return 'Finding';
		case KnowledgeResultType.DECISION:
			return 'Decision';
		case KnowledgeResultType.PATTERN:
			return 'Pattern';
		default:
			return 'Result';
	}
}

function resultTypeIcon(type: KnowledgeResultType): IconName {
	switch (type) {
		case KnowledgeResultType.CODE:
			return 'file-code';
		case KnowledgeResultType.FINDING:
			return 'alert-triangle';
		case KnowledgeResultType.DECISION:
			return 'clipboard';
		case KnowledgeResultType.PATTERN:
			return 'layers';
		default:
			return 'file-text';
	}
}

function lineRange(startLine: number, endLine: number): string {
	if (startLine <= 0 && endLine <= 0) {
		return '';
	}
	if (startLine > 0 && endLine > 0 && startLine !== endLine) {
		return `:${startLine}-${endLine}`;
	}
	if (startLine > 0) {
		return `:${startLine}`;
	}
	return '';
}

export function KnowledgeResultCard({ result, discussing, onDiscuss }: KnowledgeResultCardProps) {
	const typeLabel = resultTypeLabel(result.type);
	const fileRef = result.filePath ? `${result.filePath}${lineRange(result.startLine, result.endLine)}` : '';

	return (
		<article className="knowledge-result-card">
			<header className="knowledge-result-card__header">
				<div className="knowledge-result-card__type">
					<Icon name={resultTypeIcon(result.type)} size={14} />
					<span>{typeLabel}</span>
				</div>
				<div className="knowledge-result-card__score">
					Score {result.score.toFixed(2)}
				</div>
			</header>

			<div className="knowledge-result-card__body">
				<h3>{result.title || result.id}</h3>
				{fileRef && <p className="knowledge-result-card__file">{fileRef}</p>}
				<p className="knowledge-result-card__content">{result.summary || result.content}</p>

				{result.type === KnowledgeResultType.FINDING && (
					<div className="knowledge-result-card__meta">
						{result.severity && <span className="knowledge-result-card__badge">{result.severity}</span>}
						{result.status && <span className="knowledge-result-card__badge">{result.status}</span>}
					</div>
				)}

				{result.type === KnowledgeResultType.DECISION && (
					<div className="knowledge-result-card__meta knowledge-result-card__meta--decision">
						{result.initiativeId && (
							<Link to={`/initiatives/${result.initiativeId}`}>
								{result.initiativeTitle || result.initiativeId}
							</Link>
						)}
						{result.rationale && <span>{result.rationale}</span>}
					</div>
				)}

				{result.type === KnowledgeResultType.PATTERN && (
					<div className="knowledge-result-card__meta">
						<span className="knowledge-result-card__badge">
							{result.memberCount} members
						</span>
					</div>
				)}
			</div>

			<footer className="knowledge-result-card__footer">
				<button
					type="button"
					onClick={() => onDiscuss(result)}
					disabled={discussing}
				>
					Discuss
				</button>
			</footer>
		</article>
	);
}
