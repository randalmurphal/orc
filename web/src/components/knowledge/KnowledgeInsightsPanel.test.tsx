import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { KnowledgeInsightsPanel } from './KnowledgeInsightsPanel';
import { GetKnowledgeInsightsResponseSchema } from '@/gen/orc/v1/knowledge_pb';

describe('KnowledgeInsightsPanel', () => {
	it('renders hot files, recurring patterns, and constitution updates sections', () => {
		const insights = create(GetKnowledgeInsightsResponseSchema, {
			hotFiles: [{ filePath: 'internal/api/server.go', hitCount: 5, summary: 'hot file' }],
			recurringPatterns: [{ name: 'Pipeline fan-out', memberCount: 3, summary: 'pattern' }],
			constitutionUpdates: [{ title: 'Guardrail Update', summary: 'constitution', source: '.orc/CONSTITUTION.md' }],
		});

		render(<KnowledgeInsightsPanel insights={insights} loading={false} />);

		expect(screen.getByRole('heading', { name: 'Hot Files' })).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'Recurring Patterns' })).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'Constitution Updates' })).toBeInTheDocument();
	});

	it('hides empty sections instead of rendering blank placeholders', () => {
		const insights = create(GetKnowledgeInsightsResponseSchema, {
			hotFiles: [{ filePath: 'internal/api/server.go', hitCount: 2, summary: '' }],
		});

		render(<KnowledgeInsightsPanel insights={insights} loading={false} />);

		expect(screen.getByRole('heading', { name: 'Hot Files' })).toBeInTheDocument();
		expect(screen.queryByRole('heading', { name: 'Recurring Patterns' })).not.toBeInTheDocument();
		expect(screen.queryByRole('heading', { name: 'Constitution Updates' })).not.toBeInTheDocument();
	});
});
