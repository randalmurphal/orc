import { create } from '@bufbuild/protobuf';
import { describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { fireEvent, render, screen } from '@testing-library/react';
import { KnowledgeResultCard } from './KnowledgeResultCard';
import { KnowledgeResultSchema, KnowledgeResultType } from '@/gen/orc/v1/knowledge_pb';

function renderCard(resultType: KnowledgeResultType) {
	const base = create(KnowledgeResultSchema, {
		id: `result-${resultType}`,
		type: resultType,
		title: 'Knowledge Result',
		content: 'Detailed content',
		summary: 'Summary content',
		filePath: 'internal/executor/workflow_executor.go',
		startLine: 17,
		endLine: 31,
		score: 0.88,
		severity: 'high',
		status: 'open',
		initiativeId: 'INIT-003',
		initiativeTitle: 'Development OS',
		rationale: 'Consistency',
		memberCount: 9,
	});

	const onDiscuss = vi.fn();
	render(
		<MemoryRouter>
			<KnowledgeResultCard
				result={base}
				discussing={false}
				onDiscuss={onDiscuss}
			/>
		</MemoryRouter>
	);

	return { onDiscuss };
}

describe('KnowledgeResultCard', () => {
	it('renders code metadata with file path and line numbers', () => {
		renderCard(KnowledgeResultType.CODE);
		expect(screen.getByText('internal/executor/workflow_executor.go:17-31')).toBeInTheDocument();
	});

	it('renders finding metadata with severity and status', () => {
		renderCard(KnowledgeResultType.FINDING);
		expect(screen.getByText('high')).toBeInTheDocument();
		expect(screen.getByText('open')).toBeInTheDocument();
	});

	it('renders decision metadata with initiative link and rationale', () => {
		renderCard(KnowledgeResultType.DECISION);
		expect(screen.getByRole('link', { name: 'Development OS' })).toHaveAttribute('href', '/initiatives/INIT-003');
		expect(screen.getByText('Consistency')).toBeInTheDocument();
	});

	it('renders pattern metadata with member count', () => {
		renderCard(KnowledgeResultType.PATTERN);
		expect(screen.getByText('9 members')).toBeInTheDocument();
	});

	it('calls onDiscuss when clicking Discuss', () => {
		const { onDiscuss } = renderCard(KnowledgeResultType.CODE);
		fireEvent.click(screen.getByRole('button', { name: 'Discuss' }));
		expect(onDiscuss).toHaveBeenCalledTimes(1);
	});
});
