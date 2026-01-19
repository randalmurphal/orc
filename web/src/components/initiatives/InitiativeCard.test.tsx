import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import {
	InitiativeCard,
	extractEmoji,
	getStatusColor,
	getIconColor,
	formatTokens,
	formatCostDisplay,
	isPaused,
} from './InitiativeCard';
import type { Initiative, InitiativeStatus } from '../../lib/types';

// =============================================================================
// Test Fixtures
// =============================================================================

const mockInitiative: Initiative = {
	version: 1,
	id: 'INIT-001',
	title: '🎨 Frontend Polish & UX Audit',
	status: 'active',
	vision:
		'Comprehensive UI refresh including component library documentation and accessibility improvements',
	created_at: '2026-01-01T00:00:00Z',
	updated_at: '2026-01-15T00:00:00Z',
};

const mockPausedInitiative: Initiative = {
	...mockInitiative,
	id: 'INIT-002',
	title: '📊 Analytics Dashboard',
	status: 'archived',
	vision: 'Real-time metrics, charts, and exportable reports',
};

const mockCompletedInitiative: Initiative = {
	...mockInitiative,
	id: 'INIT-003',
	title: '⚡ Systems Reliability',
	status: 'completed',
	vision: 'Redis caching, connection pooling, circuit breakers',
};

// =============================================================================
// Utility Function Tests
// =============================================================================

describe('extractEmoji', () => {
	it('extracts emoji from start of string', () => {
		expect(extractEmoji('🎨 Frontend Polish')).toBe('🎨');
		expect(extractEmoji('⚡ Systems Reliability')).toBe('⚡');
	});

	it('extracts emoji from middle of string', () => {
		expect(extractEmoji('Frontend 🎨 Polish')).toBe('🎨');
	});

	it('returns default emoji when no emoji present', () => {
		expect(extractEmoji('Frontend Polish')).toBe('📋');
		expect(extractEmoji('')).toBe('📋');
	});

	it('returns default emoji for undefined', () => {
		expect(extractEmoji(undefined)).toBe('📋');
	});

	it('handles compound emojis', () => {
		expect(extractEmoji('🔐 Auth & Permissions')).toBe('🔐');
		expect(extractEmoji('📊 Analytics')).toBe('📊');
	});
});

describe('getStatusColor', () => {
	it('returns green for active status', () => {
		expect(getStatusColor('active')).toBe('green');
	});

	it('returns purple for completed status', () => {
		expect(getStatusColor('completed')).toBe('purple');
	});

	it('returns amber for archived status', () => {
		expect(getStatusColor('archived')).toBe('amber');
	});

	it('returns amber for draft status', () => {
		expect(getStatusColor('draft')).toBe('amber');
	});
});

describe('getIconColor', () => {
	it('returns correct colors for each status', () => {
		expect(getIconColor('active')).toBe('green');
		expect(getIconColor('completed')).toBe('purple');
		expect(getIconColor('archived')).toBe('amber');
		expect(getIconColor('draft')).toBe('amber');
	});
});

describe('formatTokens', () => {
	it('formats millions with M suffix', () => {
		expect(formatTokens(1_500_000)).toBe('1.5M');
		expect(formatTokens(2_000_000)).toBe('2M');
	});

	it('formats thousands with K suffix', () => {
		expect(formatTokens(127_000)).toBe('127K');
		expect(formatTokens(542_000)).toBe('542K');
		expect(formatTokens(891_000)).toBe('891K');
	});

	it('returns small numbers as-is', () => {
		expect(formatTokens(0)).toBe('0');
		expect(formatTokens(999)).toBe('999');
	});
});

describe('formatCostDisplay', () => {
	it('formats cost with $ prefix and 2 decimal places', () => {
		expect(formatCostDisplay(2.34)).toBe('$2.34');
		expect(formatCostDisplay(18.45)).toBe('$18.45');
		expect(formatCostDisplay(27.03)).toBe('$27.03');
		expect(formatCostDisplay(0)).toBe('$0.00');
	});
});

describe('isPaused', () => {
	it('returns true for archived status', () => {
		expect(isPaused('archived')).toBe(true);
	});

	it('returns true for draft status', () => {
		expect(isPaused('draft')).toBe(true);
	});

	it('returns false for active status', () => {
		expect(isPaused('active')).toBe(false);
	});

	it('returns false for completed status', () => {
		expect(isPaused('completed')).toBe(false);
	});
});

// =============================================================================
// InitiativeCard Component Tests
// =============================================================================

describe('InitiativeCard', () => {
	describe('rendering', () => {
		it('renders an article element with initiative-card class', () => {
			const { container } = render(
				<InitiativeCard initiative={mockInitiative} />
			);
			const card = container.querySelector('article');
			expect(card).toBeInTheDocument();
			expect(card).toHaveClass('initiative-card');
		});

		it('renders initiative title', () => {
			render(<InitiativeCard initiative={mockInitiative} />);
			expect(
				screen.getByRole('heading', {
					name: '🎨 Frontend Polish & UX Audit',
				})
			).toBeInTheDocument();
		});

		it('renders initiative description (vision)', () => {
			render(<InitiativeCard initiative={mockInitiative} />);
			expect(
				screen.getByText(
					'Comprehensive UI refresh including component library documentation and accessibility improvements'
				)
			).toBeInTheDocument();
		});

		it('renders status badge', () => {
			render(<InitiativeCard initiative={mockInitiative} />);
			expect(screen.getByText('Active')).toBeInTheDocument();
		});

		it('renders status badge with correct color class', () => {
			const { container } = render(
				<InitiativeCard initiative={mockInitiative} />
			);
			const badge = container.querySelector('.initiative-card-status');
			expect(badge).toHaveClass('initiative-card-status-green');
		});
	});

	describe('progress section', () => {
		it('displays correct progress fraction', () => {
			render(
				<InitiativeCard
					initiative={mockInitiative}
					completedTasks={15}
					totalTasks={20}
				/>
			);
			expect(screen.getByText('15 / 20 tasks')).toBeInTheDocument();
		});

		it('displays zero progress when no tasks', () => {
			render(
				<InitiativeCard
					initiative={mockInitiative}
					completedTasks={0}
					totalTasks={0}
				/>
			);
			expect(screen.getByText('0 / 0 tasks')).toBeInTheDocument();
		});

		it('renders progress bar with correct width', () => {
			const { container } = render(
				<InitiativeCard
					initiative={mockInitiative}
					completedTasks={15}
					totalTasks={20}
				/>
			);
			const progressFill = container.querySelector(
				'.initiative-card-progress-fill'
			);
			expect(progressFill).toHaveStyle({ width: '75%' });
		});

		it('renders progress bar at 0% when no tasks', () => {
			const { container } = render(
				<InitiativeCard
					initiative={mockInitiative}
					completedTasks={0}
					totalTasks={0}
				/>
			);
			const progressFill = container.querySelector(
				'.initiative-card-progress-fill'
			);
			expect(progressFill).toHaveStyle({ width: '0%' });
		});
	});

	describe('paused state', () => {
		it('applies paused opacity when status is archived', () => {
			const { container } = render(
				<InitiativeCard initiative={mockPausedInitiative} />
			);
			const card = container.querySelector('.initiative-card');
			expect(card).toHaveClass('initiative-card-paused');
		});

		it('applies paused opacity when status is draft', () => {
			const draftInitiative: Initiative = {
				...mockInitiative,
				status: 'draft',
			};
			const { container } = render(
				<InitiativeCard initiative={draftInitiative} />
			);
			const card = container.querySelector('.initiative-card');
			expect(card).toHaveClass('initiative-card-paused');
		});

		it('does not apply paused class for active initiatives', () => {
			const { container } = render(
				<InitiativeCard initiative={mockInitiative} />
			);
			const card = container.querySelector('.initiative-card');
			expect(card).not.toHaveClass('initiative-card-paused');
		});
	});

	describe('description truncation', () => {
		it('applies line-clamp CSS class to description', () => {
			const { container } = render(
				<InitiativeCard initiative={mockInitiative} />
			);
			const desc = container.querySelector('.initiative-card-desc');
			expect(desc).toBeInTheDocument();
			// The CSS class applies -webkit-line-clamp: 2
		});

		it('does not render description when vision is undefined', () => {
			const initiativeWithoutVision: Initiative = {
				...mockInitiative,
				vision: undefined,
			};
			const { container } = render(
				<InitiativeCard initiative={initiativeWithoutVision} />
			);
			const desc = container.querySelector('.initiative-card-desc');
			expect(desc).not.toBeInTheDocument();
		});
	});

	describe('missing emoji handling', () => {
		it('uses default icon when title has no emoji', () => {
			const initiativeNoEmoji: Initiative = {
				...mockInitiative,
				title: 'Frontend Polish',
				vision: 'Some description without emoji',
			};
			const { container } = render(
				<InitiativeCard initiative={initiativeNoEmoji} />
			);
			const icon = container.querySelector('.initiative-card-icon');
			expect(icon?.textContent).toBe('📋');
		});

		it('extracts emoji from title', () => {
			const { container } = render(
				<InitiativeCard initiative={mockInitiative} />
			);
			const icon = container.querySelector('.initiative-card-icon');
			expect(icon?.textContent).toBe('🎨');
		});
	});

	describe('status badge colors', () => {
		const statusTestCases: Array<{
			status: InitiativeStatus;
			expectedClass: string;
		}> = [
			{ status: 'active', expectedClass: 'initiative-card-status-green' },
			{
				status: 'completed',
				expectedClass: 'initiative-card-status-purple',
			},
			{
				status: 'archived',
				expectedClass: 'initiative-card-status-amber',
			},
			{ status: 'draft', expectedClass: 'initiative-card-status-amber' },
		];

		it.each(statusTestCases)(
			'displays $expectedClass for $status status',
			({ status, expectedClass }) => {
				const initiative: Initiative = { ...mockInitiative, status };
				const { container } = render(
					<InitiativeCard initiative={initiative} />
				);
				const badge = container.querySelector('.initiative-card-status');
				expect(badge).toHaveClass(expectedClass);
			}
		);
	});

	describe('onClick behavior', () => {
		it('calls onClick when card is clicked', () => {
			const handleClick = vi.fn();
			render(
				<InitiativeCard
					initiative={mockInitiative}
					onClick={handleClick}
				/>
			);
			const card = screen.getByRole('button');
			fireEvent.click(card);
			expect(handleClick).toHaveBeenCalledTimes(1);
		});

		it('has button role when onClick is provided', () => {
			render(
				<InitiativeCard
					initiative={mockInitiative}
					onClick={() => {}}
				/>
			);
			expect(screen.getByRole('button')).toBeInTheDocument();
		});

		it('does not have button role when onClick is not provided', () => {
			render(<InitiativeCard initiative={mockInitiative} />);
			expect(screen.queryByRole('button')).not.toBeInTheDocument();
		});

		it('is focusable when onClick is provided', () => {
			const { container } = render(
				<InitiativeCard
					initiative={mockInitiative}
					onClick={() => {}}
				/>
			);
			const card = container.querySelector('.initiative-card');
			expect(card).toHaveAttribute('tabIndex', '0');
		});

		it('calls onClick when Enter is pressed', () => {
			const handleClick = vi.fn();
			render(
				<InitiativeCard
					initiative={mockInitiative}
					onClick={handleClick}
				/>
			);
			const card = screen.getByRole('button');
			fireEvent.keyDown(card, { key: 'Enter' });
			expect(handleClick).toHaveBeenCalledTimes(1);
		});

		it('calls onClick when Space is pressed', () => {
			const handleClick = vi.fn();
			render(
				<InitiativeCard
					initiative={mockInitiative}
					onClick={handleClick}
				/>
			);
			const card = screen.getByRole('button');
			fireEvent.keyDown(card, { key: ' ' });
			expect(handleClick).toHaveBeenCalledTimes(1);
		});

		it('has clickable class when onClick is provided', () => {
			const { container } = render(
				<InitiativeCard
					initiative={mockInitiative}
					onClick={() => {}}
				/>
			);
			const card = container.querySelector('.initiative-card');
			expect(card).toHaveClass('initiative-card-clickable');
		});
	});

	describe('meta items', () => {
		it('renders time remaining when provided', () => {
			render(
				<InitiativeCard
					initiative={mockInitiative}
					estimatedTimeRemaining="Est. 2h remaining"
				/>
			);
			expect(screen.getByText('Est. 2h remaining')).toBeInTheDocument();
		});

		it('renders cost when provided', () => {
			render(
				<InitiativeCard initiative={mockInitiative} costSpent={18.45} />
			);
			expect(screen.getByText('$18.45 spent')).toBeInTheDocument();
		});

		it('renders tokens when provided', () => {
			render(
				<InitiativeCard
					initiative={mockInitiative}
					tokensUsed={542000}
				/>
			);
			expect(screen.getByText('542K tokens')).toBeInTheDocument();
		});

		it('does not render meta row when no meta items provided', () => {
			const { container } = render(
				<InitiativeCard initiative={mockInitiative} />
			);
			const meta = container.querySelector('.initiative-card-meta');
			expect(meta).not.toBeInTheDocument();
		});

		it('renders all meta items together', () => {
			render(
				<InitiativeCard
					initiative={mockInitiative}
					estimatedTimeRemaining="Est. 8h remaining"
					costSpent={2.34}
					tokensUsed={127000}
				/>
			);
			expect(screen.getByText('Est. 8h remaining')).toBeInTheDocument();
			expect(screen.getByText('$2.34 spent')).toBeInTheDocument();
			expect(screen.getByText('127K tokens')).toBeInTheDocument();
		});
	});

	describe('accessibility', () => {
		it('has appropriate aria-label', () => {
			const { container } = render(
				<InitiativeCard
					initiative={mockInitiative}
					completedTasks={15}
					totalTasks={20}
				/>
			);
			const card = container.querySelector('article');
			expect(card).toHaveAttribute(
				'aria-label',
				'Initiative: 🎨 Frontend Polish & UX Audit. Status: active. Progress: 15 of 20 tasks complete.'
			);
		});

		it('progress bar has correct ARIA attributes', () => {
			const { container } = render(
				<InitiativeCard
					initiative={mockInitiative}
					completedTasks={15}
					totalTasks={20}
				/>
			);
			const progressBar = container.querySelector(
				'.initiative-card-progress-bar'
			);
			expect(progressBar).toHaveAttribute('role', 'progressbar');
			expect(progressBar).toHaveAttribute('aria-valuenow', '75');
			expect(progressBar).toHaveAttribute('aria-valuemin', '0');
			expect(progressBar).toHaveAttribute('aria-valuemax', '100');
		});

		it('status badge has role="status"', () => {
			const { container } = render(
				<InitiativeCard initiative={mockInitiative} />
			);
			const badge = container.querySelector('.initiative-card-status');
			expect(badge).toHaveAttribute('role', 'status');
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLDivElement>();
			render(<InitiativeCard ref={ref} initiative={mockInitiative} />);
			expect(ref.current).toBeInstanceOf(HTMLElement);
			expect(ref.current?.tagName).toBe('ARTICLE');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(
				<InitiativeCard
					initiative={mockInitiative}
					className="custom-class"
				/>
			);
			const card = container.querySelector('.initiative-card');
			expect(card).toHaveClass('custom-class');
			expect(card).toHaveClass('initiative-card');
		});
	});

	describe('icon color variants', () => {
		it('applies correct icon color for active status', () => {
			const { container } = render(
				<InitiativeCard initiative={mockInitiative} />
			);
			const icon = container.querySelector('.initiative-card-icon');
			expect(icon).toHaveClass('initiative-card-icon-green');
		});

		it('applies correct icon color for completed status', () => {
			const { container } = render(
				<InitiativeCard initiative={mockCompletedInitiative} />
			);
			const icon = container.querySelector('.initiative-card-icon');
			expect(icon).toHaveClass('initiative-card-icon-purple');
		});

		it('applies correct icon color for archived status', () => {
			const { container } = render(
				<InitiativeCard initiative={mockPausedInitiative} />
			);
			const icon = container.querySelector('.initiative-card-icon');
			expect(icon).toHaveClass('initiative-card-icon-amber');
		});
	});

	describe('progress bar color variants', () => {
		it('uses green progress fill for active initiative', () => {
			const { container } = render(
				<InitiativeCard
					initiative={mockInitiative}
					completedTasks={5}
					totalTasks={10}
				/>
			);
			const fill = container.querySelector(
				'.initiative-card-progress-fill'
			);
			expect(fill).toHaveClass('initiative-card-progress-fill-green');
		});

		it('uses purple progress fill for completed initiative', () => {
			const { container } = render(
				<InitiativeCard
					initiative={mockCompletedInitiative}
					completedTasks={10}
					totalTasks={10}
				/>
			);
			const fill = container.querySelector(
				'.initiative-card-progress-fill'
			);
			expect(fill).toHaveClass('initiative-card-progress-fill-purple');
		});
	});
});
