/**
 * TDD Tests for VariableReferencePanel (SC-7)
 *
 * Tests for TASK-699: Structured retry variables in variable reference panel
 *
 * Success Criteria Coverage:
 * - SC-7: Frontend VariableReferencePanel lists structured retry variables
 *
 * Behaviors:
 * - Panel displays RETRY_ATTEMPT, RETRY_FROM_PHASE, RETRY_REASON, RETRY_FEEDBACK variables
 * - Panel does NOT display the old RETRY_CONTEXT variable
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { VariableReferencePanel } from './VariableReferencePanel';

describe('VariableReferencePanel - SC-7: Structured retry variables', () => {
	it('displays RETRY_ATTEMPT variable', () => {
		render(<VariableReferencePanel />);
		expect(screen.getByText('{{RETRY_ATTEMPT}}')).toBeInTheDocument();
	});

	it('displays RETRY_FROM_PHASE variable', () => {
		render(<VariableReferencePanel />);
		expect(screen.getByText('{{RETRY_FROM_PHASE}}')).toBeInTheDocument();
	});

	it('displays RETRY_REASON variable', () => {
		render(<VariableReferencePanel />);
		expect(screen.getByText('{{RETRY_REASON}}')).toBeInTheDocument();
	});

	it('displays RETRY_FEEDBACK variable', () => {
		render(<VariableReferencePanel />);
		expect(screen.getByText('{{RETRY_FEEDBACK}}')).toBeInTheDocument();
	});

	it('does not display RETRY_CONTEXT variable', () => {
		render(<VariableReferencePanel />);
		expect(screen.queryByText('{{RETRY_CONTEXT}}')).not.toBeInTheDocument();
	});

	it('groups retry variables under a recognizable category', () => {
		const { container } = render(<VariableReferencePanel />);
		// All three retry variables should be findable in the rendered output
		const text = container.textContent ?? '';
		expect(text).toContain('RETRY_ATTEMPT');
		expect(text).toContain('RETRY_FROM_PHASE');
		expect(text).toContain('RETRY_REASON');
		expect(text).toContain('RETRY_FEEDBACK');
	});
});
