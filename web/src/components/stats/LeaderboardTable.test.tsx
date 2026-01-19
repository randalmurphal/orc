import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { LeaderboardTable } from './LeaderboardTable';

describe('LeaderboardTable', () => {
	const sampleItems = [
		{ rank: 1, name: 'Systems Reliability', value: '89 tasks' },
		{ rank: 2, name: 'Auth & Permissions', value: '67 tasks' },
		{ rank: 3, name: 'Frontend Polish', value: '54 tasks' },
		{ rank: 4, name: 'Analytics Dashboard', value: '37 tasks' },
	];

	describe('rendering', () => {
		it('renders with sample data', () => {
			const { container } = render(
				<LeaderboardTable title="Most Active Initiatives" items={sampleItems} />
			);

			expect(container.querySelector('.leaderboard-table')).toBeInTheDocument();
			expect(screen.getByText('Most Active Initiatives')).toBeInTheDocument();
		});

		it('displays title in header', () => {
			render(<LeaderboardTable title="Test Title" items={sampleItems} />);

			expect(screen.getByText('Test Title')).toBeInTheDocument();
		});

		it('displays all items with ranks, names, and values', () => {
			render(
				<LeaderboardTable title="Most Active Initiatives" items={sampleItems} />
			);

			// Check ranks
			expect(screen.getByText('1')).toBeInTheDocument();
			expect(screen.getByText('2')).toBeInTheDocument();
			expect(screen.getByText('3')).toBeInTheDocument();
			expect(screen.getByText('4')).toBeInTheDocument();

			// Check names
			expect(screen.getByText('Systems Reliability')).toBeInTheDocument();
			expect(screen.getByText('Auth & Permissions')).toBeInTheDocument();
			expect(screen.getByText('Frontend Polish')).toBeInTheDocument();
			expect(screen.getByText('Analytics Dashboard')).toBeInTheDocument();

			// Check values
			expect(screen.getByText('89 tasks')).toBeInTheDocument();
			expect(screen.getByText('67 tasks')).toBeInTheDocument();
			expect(screen.getByText('54 tasks')).toBeInTheDocument();
			expect(screen.getByText('37 tasks')).toBeInTheDocument();
		});

		it('renders rows with correct structure', () => {
			const { container } = render(
				<LeaderboardTable title="Test" items={sampleItems} />
			);

			const rows = container.querySelectorAll('.leaderboard-table-row');
			expect(rows).toHaveLength(4);

			// Each row should have rank, name, and value
			const firstRow = rows[0];
			expect(firstRow.querySelector('.leaderboard-table-rank')).toBeInTheDocument();
			expect(firstRow.querySelector('.leaderboard-table-name')).toBeInTheDocument();
			expect(firstRow.querySelector('.leaderboard-table-value')).toBeInTheDocument();
		});
	});

	describe('item limit', () => {
		it('limits display to 4 items', () => {
			const manyItems = [
				{ rank: 1, name: 'Item 1', value: '100' },
				{ rank: 2, name: 'Item 2', value: '90' },
				{ rank: 3, name: 'Item 3', value: '80' },
				{ rank: 4, name: 'Item 4', value: '70' },
				{ rank: 5, name: 'Item 5', value: '60' },
				{ rank: 6, name: 'Item 6', value: '50' },
			];

			const { container } = render(
				<LeaderboardTable title="Test" items={manyItems} />
			);

			const rows = container.querySelectorAll('.leaderboard-table-row');
			expect(rows).toHaveLength(4);

			// Should not show items 5 and 6
			expect(screen.queryByText('Item 5')).not.toBeInTheDocument();
			expect(screen.queryByText('Item 6')).not.toBeInTheDocument();
		});
	});

	describe('View all button', () => {
		it('shows View all button when onViewAll is provided', () => {
			const handleViewAll = vi.fn();
			render(
				<LeaderboardTable
					title="Test"
					items={sampleItems}
					onViewAll={handleViewAll}
				/>
			);

			expect(screen.getByText('View all')).toBeInTheDocument();
		});

		it('hides View all button when onViewAll is not provided', () => {
			render(<LeaderboardTable title="Test" items={sampleItems} />);

			expect(screen.queryByText('View all')).not.toBeInTheDocument();
		});

		it('calls onViewAll when clicked', () => {
			const handleViewAll = vi.fn();
			render(
				<LeaderboardTable
					title="Test"
					items={sampleItems}
					onViewAll={handleViewAll}
				/>
			);

			fireEvent.click(screen.getByText('View all'));
			expect(handleViewAll).toHaveBeenCalledTimes(1);
		});
	});

	describe('empty state', () => {
		it('shows "No data" message when items array is empty', () => {
			const { container } = render(
				<LeaderboardTable title="Empty Table" items={[]} />
			);

			expect(screen.getByText('No data')).toBeInTheDocument();
			expect(container.querySelector('.leaderboard-table-empty')).toBeInTheDocument();
		});

		it('does not show any rows when empty', () => {
			const { container } = render(
				<LeaderboardTable title="Empty Table" items={[]} />
			);

			const rows = container.querySelectorAll('.leaderboard-table-row');
			expect(rows).toHaveLength(0);
		});
	});

	describe('file path mode', () => {
		const fileItems = [
			{ rank: 1, name: 'src/lib/auth.ts', value: '34x' },
			{ rank: 2, name: 'src/components/Button.tsx', value: '28x' },
			{ rank: 3, name: 'src/stores/session.ts', value: '21x' },
			{ rank: 4, name: 'src/lib/redis.ts', value: '18x' },
		];

		it('applies file path styling when isFilePath is true', () => {
			const { container } = render(
				<LeaderboardTable title="Most Modified Files" items={fileItems} isFilePath />
			);

			const names = container.querySelectorAll('.leaderboard-table-name');
			names.forEach((name) => {
				expect(name).toHaveClass('leaderboard-table-name--path');
			});
		});

		it('does not apply file path styling by default', () => {
			const { container } = render(
				<LeaderboardTable title="Initiatives" items={sampleItems} />
			);

			const names = container.querySelectorAll('.leaderboard-table-name');
			names.forEach((name) => {
				expect(name).not.toHaveClass('leaderboard-table-name--path');
			});
		});

		it('renders file paths correctly', () => {
			render(
				<LeaderboardTable title="Most Modified Files" items={fileItems} isFilePath />
			);

			expect(screen.getByText('src/lib/auth.ts')).toBeInTheDocument();
			expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
		});
	});

	describe('rank badge', () => {
		it('supports single-digit ranks (1-9)', () => {
			const items = [{ rank: 1, name: 'Test', value: '10' }];
			render(<LeaderboardTable title="Test" items={items} />);

			expect(screen.getByText('1')).toBeInTheDocument();
		});

		it('supports double-digit ranks (10-99)', () => {
			const items = [
				{ rank: 10, name: 'Tenth', value: '5' },
				{ rank: 99, name: 'Last', value: '1' },
			];
			render(<LeaderboardTable title="Test" items={items} />);

			expect(screen.getByText('10')).toBeInTheDocument();
			expect(screen.getByText('99')).toBeInTheDocument();
		});
	});

	describe('truncation', () => {
		it('adds title attribute for full name on hover', () => {
			const items = [
				{
					rank: 1,
					name: 'This is a very long initiative name that should be truncated',
					value: '100',
				},
			];
			const { container } = render(
				<LeaderboardTable title="Test" items={items} />
			);

			const nameElement = container.querySelector('.leaderboard-table-name');
			expect(nameElement).toHaveAttribute(
				'title',
				'This is a very long initiative name that should be truncated'
			);
		});
	});

	describe('structure', () => {
		it('has header and body sections', () => {
			const { container } = render(
				<LeaderboardTable title="Test" items={sampleItems} />
			);

			expect(container.querySelector('.leaderboard-table-header')).toBeInTheDocument();
			expect(container.querySelector('.leaderboard-table-body')).toBeInTheDocument();
		});

		it('renders table as a card', () => {
			const { container } = render(
				<LeaderboardTable title="Test" items={sampleItems} />
			);

			expect(container.querySelector('.leaderboard-table')).toBeInTheDocument();
		});
	});
});
