import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { AgentCard, type Agent, type IconColor } from './AgentCard';

// =============================================================================
// Test Fixtures
// =============================================================================

const createMockAgent = (overrides: Partial<Agent> = {}): Agent => ({
	id: 'test-agent',
	name: 'Test Agent',
	model: 'claude-sonnet-4-20250514',
	status: 'active',
	emoji: '🧠',
	iconColor: 'purple',
	stats: {
		tokensToday: 847000,
		tasksDone: 34,
		successRate: 94,
	},
	tools: ['File Read/Write', 'Bash', 'Web Search', 'MCP'],
	...overrides,
});

// =============================================================================
// Tests
// =============================================================================

describe('AgentCard', () => {
	describe('rendering', () => {
		it('renders agent name', () => {
			const agent = createMockAgent({ name: 'Primary Coder' });
			render(<AgentCard agent={agent} />);
			expect(screen.getByText('Primary Coder')).toBeInTheDocument();
		});

		it('renders agent model', () => {
			const agent = createMockAgent({ model: 'claude-haiku-3-5-20241022' });
			render(<AgentCard agent={agent} />);
			expect(screen.getByText('claude-haiku-3-5-20241022')).toBeInTheDocument();
		});

		it('renders status badge', () => {
			const agent = createMockAgent({ status: 'active' });
			render(<AgentCard agent={agent} />);
			expect(screen.getByText('active')).toBeInTheDocument();
		});

		it('renders idle status badge', () => {
			const agent = createMockAgent({ status: 'idle' });
			render(<AgentCard agent={agent} />);
			expect(screen.getByText('idle')).toBeInTheDocument();
		});

		it('renders agent emoji', () => {
			const agent = createMockAgent({ emoji: '⚡' });
			render(<AgentCard agent={agent} />);
			expect(screen.getByText('⚡')).toBeInTheDocument();
		});
	});

	describe('stats display', () => {
		it('displays all three stats with formatted values', () => {
			const agent = createMockAgent({
				stats: { tokensToday: 847000, tasksDone: 34, successRate: 94 },
			});
			render(<AgentCard agent={agent} />);

			// Tokens (formatted with K suffix)
			expect(screen.getByText('847K')).toBeInTheDocument();
			expect(screen.getByText('Tokens Today')).toBeInTheDocument();

			// Tasks Done
			expect(screen.getByText('34')).toBeInTheDocument();
			expect(screen.getByText('Tasks Done')).toBeInTheDocument();

			// Success Rate
			expect(screen.getByText('94%')).toBeInTheDocument();
			expect(screen.getByText('Success')).toBeInTheDocument();
		});

		it('displays custom tasksDoneLabel when provided', () => {
			const agent = createMockAgent({
				stats: { tokensToday: 256000, tasksDone: 8, successRate: 100, tasksDoneLabel: 'Reviews' },
			});
			render(<AgentCard agent={agent} />);
			expect(screen.getByText('Reviews')).toBeInTheDocument();
			expect(screen.queryByText('Tasks Done')).not.toBeInTheDocument();
		});

		it('formats large token numbers correctly', () => {
			const agent = createMockAgent({
				stats: { tokensToday: 1234567, tasksDone: 100, successRate: 99 },
			});
			render(<AgentCard agent={agent} />);
			expect(screen.getByText('1.23M')).toBeInTheDocument();
		});

		it('formats smaller token numbers correctly', () => {
			const agent = createMockAgent({
				stats: { tokensToday: 124000, tasksDone: 12, successRate: 91 },
			});
			render(<AgentCard agent={agent} />);
			expect(screen.getByText('124K')).toBeInTheDocument();
		});
	});

	describe('tools display', () => {
		it('renders tool badges correctly', () => {
			const agent = createMockAgent({
				tools: ['File Read', 'Bash', 'Git'],
			});
			render(<AgentCard agent={agent} />);

			expect(screen.getByText('File Read')).toBeInTheDocument();
			expect(screen.getByText('Bash')).toBeInTheDocument();
			expect(screen.getByText('Git')).toBeInTheDocument();
		});

		it('truncates tools with "+N more" when exceeding maxToolsDisplayed', () => {
			const agent = createMockAgent({
				tools: ['File Read', 'File Write', 'Bash', 'Web Search', 'Git', 'MCP'],
			});
			render(<AgentCard agent={agent} maxToolsDisplayed={4} />);

			// First 4 should be visible
			expect(screen.getByText('File Read')).toBeInTheDocument();
			expect(screen.getByText('File Write')).toBeInTheDocument();
			expect(screen.getByText('Bash')).toBeInTheDocument();
			expect(screen.getByText('Web Search')).toBeInTheDocument();

			// Last 2 should be truncated
			expect(screen.queryByText('Git')).not.toBeInTheDocument();
			expect(screen.queryByText('MCP')).not.toBeInTheDocument();

			// "+2 more" badge should appear
			expect(screen.getByText('+2 more')).toBeInTheDocument();
		});

		it('handles empty tools array without error', () => {
			const agent = createMockAgent({ tools: [] });
			const { container } = render(<AgentCard agent={agent} />);

			// Should not render tools section
			expect(container.querySelector('.agent-card-tools')).not.toBeInTheDocument();
		});

		it('does not show "+N more" when tools count equals maxToolsDisplayed', () => {
			const agent = createMockAgent({
				tools: ['Tool1', 'Tool2', 'Tool3', 'Tool4'],
			});
			render(<AgentCard agent={agent} maxToolsDisplayed={4} />);

			expect(screen.queryByText(/more/)).not.toBeInTheDocument();
		});
	});

	describe('active state', () => {
		it('applies agent-card-active class when isActive=true', () => {
			const agent = createMockAgent();
			const { container } = render(<AgentCard agent={agent} isActive />);
			const card = container.querySelector('.agent-card');
			expect(card).toHaveClass('agent-card-active');
		});

		it('does not apply agent-card-active class when isActive=false', () => {
			const agent = createMockAgent();
			const { container } = render(<AgentCard agent={agent} isActive={false} />);
			const card = container.querySelector('.agent-card');
			expect(card).not.toHaveClass('agent-card-active');
		});

		it('does not apply agent-card-active class by default', () => {
			const agent = createMockAgent();
			const { container } = render(<AgentCard agent={agent} />);
			const card = container.querySelector('.agent-card');
			expect(card).not.toHaveClass('agent-card-active');
		});
	});

	describe('click and keyboard interaction', () => {
		it('calls onSelect on click', () => {
			const agent = createMockAgent();
			const handleSelect = vi.fn();
			const { container } = render(<AgentCard agent={agent} onSelect={handleSelect} />);
			const card = container.querySelector('.agent-card')!;

			fireEvent.click(card);
			expect(handleSelect).toHaveBeenCalledTimes(1);
			expect(handleSelect).toHaveBeenCalledWith(agent);
		});

		it('calls onSelect on Enter key press', () => {
			const agent = createMockAgent();
			const handleSelect = vi.fn();
			const { container } = render(<AgentCard agent={agent} onSelect={handleSelect} />);
			const card = container.querySelector('.agent-card')!;

			fireEvent.keyDown(card, { key: 'Enter' });
			expect(handleSelect).toHaveBeenCalledTimes(1);
			expect(handleSelect).toHaveBeenCalledWith(agent);
		});

		it('calls onSelect on Space key press', () => {
			const agent = createMockAgent();
			const handleSelect = vi.fn();
			const { container } = render(<AgentCard agent={agent} onSelect={handleSelect} />);
			const card = container.querySelector('.agent-card')!;

			fireEvent.keyDown(card, { key: ' ' });
			expect(handleSelect).toHaveBeenCalledTimes(1);
		});

		it('does not call onSelect on other key presses', () => {
			const agent = createMockAgent();
			const handleSelect = vi.fn();
			const { container } = render(<AgentCard agent={agent} onSelect={handleSelect} />);
			const card = container.querySelector('.agent-card')!;

			fireEvent.keyDown(card, { key: 'Escape' });
			fireEvent.keyDown(card, { key: 'Tab' });
			expect(handleSelect).not.toHaveBeenCalled();
		});

		it('does not call onSelect when disabled', () => {
			const agent = createMockAgent({ disabled: true });
			const handleSelect = vi.fn();
			const { container } = render(<AgentCard agent={agent} onSelect={handleSelect} />);
			const card = container.querySelector('.agent-card')!;

			fireEvent.click(card);
			fireEvent.keyDown(card, { key: 'Enter' });
			expect(handleSelect).not.toHaveBeenCalled();
		});

		it('calls existing onKeyDown handler', () => {
			const agent = createMockAgent();
			const handleSelect = vi.fn();
			const handleKeyDown = vi.fn();
			const { container } = render(
				<AgentCard agent={agent} onSelect={handleSelect} onKeyDown={handleKeyDown} />
			);
			const card = container.querySelector('.agent-card')!;

			fireEvent.keyDown(card, { key: 'Enter' });
			expect(handleKeyDown).toHaveBeenCalled();
			expect(handleSelect).toHaveBeenCalled();
		});
	});

	describe('accessibility', () => {
		it('has role="button" when interactive', () => {
			const agent = createMockAgent();
			const { container } = render(<AgentCard agent={agent} onSelect={() => {}} />);
			const card = container.querySelector('.agent-card');
			expect(card).toHaveAttribute('role', 'button');
		});

		it('does not have role when not interactive', () => {
			const agent = createMockAgent();
			const { container } = render(<AgentCard agent={agent} />);
			const card = container.querySelector('.agent-card');
			expect(card).not.toHaveAttribute('role');
		});

		it('has tabIndex=0 when interactive', () => {
			const agent = createMockAgent();
			const { container } = render(<AgentCard agent={agent} onSelect={() => {}} />);
			const card = container.querySelector('.agent-card');
			expect(card).toHaveAttribute('tabIndex', '0');
		});

		it('does not have tabIndex when not interactive', () => {
			const agent = createMockAgent();
			const { container } = render(<AgentCard agent={agent} />);
			const card = container.querySelector('.agent-card');
			expect(card).not.toHaveAttribute('tabIndex');
		});

		it('has aria-pressed when interactive', () => {
			const agent = createMockAgent();
			const { container } = render(<AgentCard agent={agent} isActive onSelect={() => {}} />);
			const card = container.querySelector('.agent-card');
			expect(card).toHaveAttribute('aria-pressed', 'true');
		});

		it('has aria-pressed=false when not active but interactive', () => {
			const agent = createMockAgent();
			const { container } = render(
				<AgentCard agent={agent} isActive={false} onSelect={() => {}} />
			);
			const card = container.querySelector('.agent-card');
			expect(card).toHaveAttribute('aria-pressed', 'false');
		});

		it('has correct aria-label describing the agent', () => {
			const agent = createMockAgent({
				name: 'Test Agent',
				status: 'active',
				stats: { tokensToday: 847000, tasksDone: 34, successRate: 94 },
			});
			const { container } = render(<AgentCard agent={agent} />);
			const card = container.querySelector('.agent-card');
			expect(card).toHaveAttribute(
				'aria-label',
				'Test Agent agent, active, 847K tokens today, 34 tasks done, 94% success rate'
			);
		});

		it('has aria-disabled when agent is disabled', () => {
			const agent = createMockAgent({ disabled: true });
			const { container } = render(<AgentCard agent={agent} />);
			const card = container.querySelector('.agent-card');
			expect(card).toHaveAttribute('aria-disabled', 'true');
		});
	});

	describe('icon color variants', () => {
		const colors: IconColor[] = ['purple', 'blue', 'green', 'amber'];

		it.each(colors)('applies %s icon color class', (color) => {
			const agent = createMockAgent({ iconColor: color });
			const { container } = render(<AgentCard agent={agent} />);
			const icon = container.querySelector('.agent-card-icon');
			expect(icon).toHaveClass(`agent-card-icon-${color}`);
		});
	});

	describe('disabled state', () => {
		it('applies agent-card-disabled class when disabled', () => {
			const agent = createMockAgent({ disabled: true });
			const { container } = render(<AgentCard agent={agent} />);
			const card = container.querySelector('.agent-card');
			expect(card).toHaveClass('agent-card-disabled');
		});

		it('does not apply interactive class when disabled even with onSelect', () => {
			const agent = createMockAgent({ disabled: true });
			const { container } = render(<AgentCard agent={agent} onSelect={() => {}} />);
			const card = container.querySelector('.agent-card');
			expect(card).not.toHaveClass('agent-card-interactive');
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLDivElement>();
			const agent = createMockAgent();
			render(<AgentCard ref={ref} agent={agent} />);
			expect(ref.current).toBeInstanceOf(HTMLDivElement);
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const agent = createMockAgent();
			const { container } = render(<AgentCard agent={agent} className="custom-class" />);
			const card = container.querySelector('.agent-card');
			expect(card).toHaveClass('custom-class');
			expect(card).toHaveClass('agent-card');
		});
	});

	describe('HTML attributes', () => {
		it('passes through native div attributes', () => {
			const agent = createMockAgent();
			render(<AgentCard agent={agent} data-testid="test-card" title="Test tooltip" />);
			const card = screen.getByTestId('test-card');
			expect(card).toHaveAttribute('title', 'Test tooltip');
		});
	});
});
