import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { InitiativeCard } from './InitiativeCard';
import { extractEmoji, getStatusColor, getIconColor, isPaused } from './initiative-utils';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import { createMockInitiative } from '@/test/factories';

// =============================================================================
// Test Helpers
// =============================================================================

function renderWithTooltips(ui: React.ReactElement) {
	return render(<TooltipProvider>{ui}</TooltipProvider>);
}

// =============================================================================
// Test Fixtures
// =============================================================================

const mockInitiative = createMockInitiative({
	id: 'INIT-001',
	title: '🎨 Frontend Polish & UX Audit',
	status: InitiativeStatus.ACTIVE,
	vision:
		'Comprehensive UI refresh including component library documentation and accessibility improvements',
});

const mockPausedInitiative = createMockInitiative({
	id: 'INIT-002',
	title: '📊 Analytics Dashboard',
	status: InitiativeStatus.ARCHIVED,
	vision: 'Real-time metrics, charts, and exportable reports',
});

const mockCompletedInitiative = createMockInitiative({
	id: 'INIT-003',
	title: '⚡ Systems Reliability',
	status: InitiativeStatus.COMPLETED,
	vision: 'Redis caching, connection pooling, circuit breakers',
});

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
		expect(getStatusColor(InitiativeStatus.ACTIVE)).toBe('green');
	});

	it('returns purple for completed status', () => {
		expect(getStatusColor(InitiativeStatus.COMPLETED)).toBe('purple');
	});

	it('returns amber for archived status', () => {
		expect(getStatusColor(InitiativeStatus.ARCHIVED)).toBe('amber');
	});

	it('returns amber for draft status', () => {
		expect(getStatusColor(InitiativeStatus.DRAFT)).toBe('amber');
	});
});

describe('getIconColor', () => {
	it('returns correct colors for each status', () => {
		expect(getIconColor(InitiativeStatus.ACTIVE)).toBe('green');
		expect(getIconColor(InitiativeStatus.COMPLETED)).toBe('purple');
		expect(getIconColor(InitiativeStatus.ARCHIVED)).toBe('amber');
		expect(getIconColor(InitiativeStatus.DRAFT)).toBe('amber');
	});
});

// Format utility tests are in @/lib/format.test.ts

describe('isPaused', () => {
	it('returns true for archived status', () => {
		expect(isPaused(InitiativeStatus.ARCHIVED)).toBe(true);
	});

	it('returns true for draft status', () => {
		expect(isPaused(InitiativeStatus.DRAFT)).toBe(true);
	});

	it('returns false for active status', () => {
		expect(isPaused(InitiativeStatus.ACTIVE)).toBe(false);
	});

	it('returns false for completed status', () => {
		expect(isPaused(InitiativeStatus.COMPLETED)).toBe(false);
	});
});

// =============================================================================
// InitiativeCard Component Tests
// =============================================================================

describe('InitiativeCard', () => {
	describe('rendering', () => {
		it('renders an article element with initiative-card class', () => {
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={mockInitiative} />
			);
			const card = container.querySelector('article');
			expect(card).toBeInTheDocument();
			expect(card).toHaveClass('initiative-card');
		});

		it('renders initiative title', () => {
			renderWithTooltips(<InitiativeCard initiative={mockInitiative} />);
			expect(
				screen.getByRole('heading', {
					name: '🎨 Frontend Polish & UX Audit',
				})
			).toBeInTheDocument();
		});

		it('renders initiative description (vision)', () => {
			renderWithTooltips(<InitiativeCard initiative={mockInitiative} />);
			expect(
				screen.getByText(
					'Comprehensive UI refresh including component library documentation and accessibility improvements'
				)
			).toBeInTheDocument();
		});

		it('renders status badge', () => {
			renderWithTooltips(<InitiativeCard initiative={mockInitiative} />);
			expect(screen.getByText('Active')).toBeInTheDocument();
		});

		it('renders status badge with correct color class', () => {
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={mockInitiative} />
			);
			const badge = container.querySelector('.initiative-card-status');
			expect(badge).toHaveClass('initiative-card-status-green');
		});
	});

	describe('progress section', () => {
		it('displays correct progress fraction', () => {
			renderWithTooltips(
				<InitiativeCard
					initiative={mockInitiative}
					completedTasks={15}
					totalTasks={20}
				/>
			);
			expect(screen.getByText('15 / 20 tasks')).toBeInTheDocument();
		});

		it('displays zero progress when no tasks', () => {
			renderWithTooltips(
				<InitiativeCard
					initiative={mockInitiative}
					completedTasks={0}
					totalTasks={0}
				/>
			);
			expect(screen.getByText('0 / 0 tasks')).toBeInTheDocument();
		});

		it('renders progress bar with correct width', () => {
			const { container } = renderWithTooltips(
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
			const { container } = renderWithTooltips(
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
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={mockPausedInitiative} />
			);
			const card = container.querySelector('.initiative-card');
			expect(card).toHaveClass('initiative-card-paused');
		});

		it('applies paused opacity when status is draft', () => {
			const draftInitiative = createMockInitiative({
				...mockInitiative,
				status: InitiativeStatus.DRAFT,
			});
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={draftInitiative} />
			);
			const card = container.querySelector('.initiative-card');
			expect(card).toHaveClass('initiative-card-paused');
		});

		it('does not apply paused class for active initiatives', () => {
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={mockInitiative} />
			);
			const card = container.querySelector('.initiative-card');
			expect(card).not.toHaveClass('initiative-card-paused');
		});
	});

	describe('description truncation', () => {
		it('applies line-clamp CSS class to description', () => {
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={mockInitiative} />
			);
			const desc = container.querySelector('.initiative-card-desc');
			expect(desc).toBeInTheDocument();
			// The CSS class applies -webkit-line-clamp: 2
		});

		it('does not render description when vision is undefined', () => {
			const initiativeWithoutVision = createMockInitiative({
				...mockInitiative,
				vision: undefined,
			});
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={initiativeWithoutVision} />
			);
			const desc = container.querySelector('.initiative-card-desc');
			expect(desc).not.toBeInTheDocument();
		});
	});

	describe('missing emoji handling', () => {
		it('uses default icon when title has no emoji', () => {
			const initiativeNoEmoji = createMockInitiative({
				...mockInitiative,
				title: 'Frontend Polish',
				vision: 'Some description without emoji',
			});
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={initiativeNoEmoji} />
			);
			const icon = container.querySelector('.initiative-card-icon');
			expect(icon?.textContent).toBe('📋');
		});

		it('extracts emoji from title', () => {
			const { container } = renderWithTooltips(
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
			{ status: InitiativeStatus.ACTIVE, expectedClass: 'initiative-card-status-green' },
			{
				status: InitiativeStatus.COMPLETED,
				expectedClass: 'initiative-card-status-purple',
			},
			{
				status: InitiativeStatus.ARCHIVED,
				expectedClass: 'initiative-card-status-amber',
			},
			{ status: InitiativeStatus.DRAFT, expectedClass: 'initiative-card-status-amber' },
		];

		it.each(statusTestCases)(
			'displays $expectedClass for $status status',
			({ status, expectedClass }) => {
				const initiative = createMockInitiative({ ...mockInitiative, status });
				const { container } = renderWithTooltips(
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
			renderWithTooltips(
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
			renderWithTooltips(
				<InitiativeCard
					initiative={mockInitiative}
					onClick={() => {}}
				/>
			);
			expect(screen.getByRole('button')).toBeInTheDocument();
		});

		it('does not have button role when onClick is not provided', () => {
			renderWithTooltips(<InitiativeCard initiative={mockInitiative} />);
			expect(screen.queryByRole('button')).not.toBeInTheDocument();
		});

		it('is focusable when onClick is provided', () => {
			const { container } = renderWithTooltips(
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
			renderWithTooltips(
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
			renderWithTooltips(
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
			const { container } = renderWithTooltips(
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
			renderWithTooltips(
				<InitiativeCard
					initiative={mockInitiative}
					estimatedTimeRemaining="Est. 2h remaining"
				/>
			);
			expect(screen.getByText('Est. 2h remaining')).toBeInTheDocument();
		});

		it('renders cost when provided', () => {
			renderWithTooltips(
				<InitiativeCard initiative={mockInitiative} costSpent={18.45} />
			);
			expect(screen.getByText('$18.45 spent')).toBeInTheDocument();
		});

		it('renders tokens when provided', () => {
			renderWithTooltips(
				<InitiativeCard
					initiative={mockInitiative}
					tokensUsed={542000}
				/>
			);
			expect(screen.getByText('542K tokens')).toBeInTheDocument();
		});

		it('does not render meta row when no meta items provided', () => {
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={mockInitiative} />
			);
			const meta = container.querySelector('.initiative-card-meta');
			expect(meta).not.toBeInTheDocument();
		});

		it('renders all meta items together', () => {
			renderWithTooltips(
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
			const { container } = renderWithTooltips(
				<InitiativeCard
					initiative={mockInitiative}
					completedTasks={15}
					totalTasks={20}
				/>
			);
			const card = container.querySelector('article');
			expect(card).toHaveAttribute(
				'aria-label',
				`Initiative: 🎨 Frontend Polish & UX Audit. Status: ${InitiativeStatus.ACTIVE}. Progress: 15 of 20 tasks complete.`
			);
		});

		it('progress bar has correct ARIA attributes', () => {
			const { container } = renderWithTooltips(
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
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={mockInitiative} />
			);
			const badge = container.querySelector('.initiative-card-status');
			expect(badge).toHaveAttribute('role', 'status');
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLDivElement>();
			renderWithTooltips(<InitiativeCard ref={ref} initiative={mockInitiative} />);
			expect(ref.current).toBeInstanceOf(HTMLElement);
			expect(ref.current?.tagName).toBe('ARTICLE');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = renderWithTooltips(
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
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={mockInitiative} />
			);
			const icon = container.querySelector('.initiative-card-icon');
			expect(icon).toHaveClass('initiative-card-icon-green');
		});

		it('applies correct icon color for completed status', () => {
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={mockCompletedInitiative} />
			);
			const icon = container.querySelector('.initiative-card-icon');
			expect(icon).toHaveClass('initiative-card-icon-purple');
		});

		it('applies correct icon color for archived status', () => {
			const { container } = renderWithTooltips(
				<InitiativeCard initiative={mockPausedInitiative} />
			);
			const icon = container.querySelector('.initiative-card-icon');
			expect(icon).toHaveClass('initiative-card-icon-amber');
		});
	});

	describe('progress bar color variants', () => {
		it('uses green progress fill for active initiative', () => {
			const { container } = renderWithTooltips(
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
			const { container } = renderWithTooltips(
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
