/**
 * FilesPanel Component Tests
 *
 * Tests for:
 * - Basic rendering
 * - Status badges (M, A, D, R)
 * - File icon differentiation (text vs binary)
 * - Click interactions
 * - Keyboard navigation
 * - Grouping by task
 * - Collapse/expand behavior
 * - "Show more" functionality
 * - Edge cases
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FilesPanel, ChangedFile, FileStatus } from './FilesPanel';

describe('FilesPanel', () => {
	const mockOnFileClick = vi.fn();

	beforeEach(() => {
		mockOnFileClick.mockClear();
	});

	describe('basic rendering', () => {
		it('renders panel section when files array is empty', () => {
			const { container } = render(
				<FilesPanel files={[]} onFileClick={mockOnFileClick} />
			);
			expect(container.querySelector('.panel-section')).toBeTruthy();
		});

		it('renders panel with header and file count', () => {
			const files: ChangedFile[] = [
				{ path: 'src/app.ts', status: 'modified' },
				{ path: 'src/index.ts', status: 'added' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(screen.getByText('Files Changed')).toBeInTheDocument();
			expect(screen.getByText('2')).toBeInTheDocument();
		});

		it('renders blue-themed header icon', () => {
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const iconContainer = document.querySelector('.panel-title-icon.blue');
			expect(iconContainer).toBeInTheDocument();
		});

		it('renders all files in the list', () => {
			const files: ChangedFile[] = [
				{ path: 'src/app.ts', status: 'modified' },
				{ path: 'src/index.ts', status: 'added' },
				{ path: 'src/old.ts', status: 'deleted' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(screen.getByText('src/app.ts')).toBeInTheDocument();
			expect(screen.getByText('src/index.ts')).toBeInTheDocument();
			expect(screen.getByText('src/old.ts')).toBeInTheDocument();
		});
	});

	describe('status badges', () => {
		const statuses: { status: FileStatus; badge: string; label: string }[] = [
			{ status: 'modified', badge: 'M', label: 'Modified' },
			{ status: 'added', badge: 'A', label: 'Added' },
			{ status: 'deleted', badge: 'D', label: 'Deleted' },
			{ status: 'renamed', badge: 'R', label: 'Renamed' },
		];

		statuses.forEach(({ status, badge, label }) => {
			it(`renders ${badge} badge for ${status} files`, () => {
				const files: ChangedFile[] = [{ path: 'src/file.ts', status }];

				render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

				const badgeElement = screen.getByText(badge);
				expect(badgeElement).toBeInTheDocument();
				expect(badgeElement).toHaveClass(`file-status`, status);
			});

			it(`has aria-label "${label}" for ${status} status`, () => {
				const files: ChangedFile[] = [{ path: 'src/file.ts', status }];

				render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

				const fileItem = screen.getByRole('button', {
					name: new RegExp(label, 'i'),
				});
				expect(fileItem).toBeInTheDocument();
			});
		});
	});

	describe('file icons', () => {
		it('shows text icon for text files', () => {
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const iconContainer = document.querySelector('.file-icon:not(.binary)');
			expect(iconContainer).toBeInTheDocument();
		});

		it('shows binary icon for image files', () => {
			const files: ChangedFile[] = [
				{ path: 'assets/logo.png', status: 'added' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const iconContainer = document.querySelector('.file-icon.binary');
			expect(iconContainer).toBeInTheDocument();
		});

		it('detects common binary extensions', () => {
			const binaryFiles = [
				'image.png',
				'photo.jpg',
				'icon.ico',
				'doc.pdf',
				'archive.zip',
				'font.woff2',
				'video.mp4',
				'data.sqlite',
			];

			binaryFiles.forEach((path) => {
				const { container, unmount } = render(
					<FilesPanel
						files={[{ path, status: 'modified' }]}
						onFileClick={mockOnFileClick}
					/>
				);

				expect(
					container.querySelector('.file-icon.binary'),
					`Expected ${path} to be detected as binary`
				).toBeInTheDocument();
				unmount();
			});
		});

		it('respects explicit binary flag', () => {
			const files: ChangedFile[] = [
				{ path: 'data.txt', status: 'modified', binary: true },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const iconContainer = document.querySelector('.file-icon.binary');
			expect(iconContainer).toBeInTheDocument();
		});

		it('respects explicit binary=false for extension-detected binary', () => {
			const files: ChangedFile[] = [
				{ path: 'data.png', status: 'modified', binary: false },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const iconContainer = document.querySelector('.file-icon:not(.binary)');
			expect(iconContainer).toBeInTheDocument();
		});
	});

	describe('click interactions', () => {
		it('calls onFileClick when file is clicked', async () => {
			const user = userEvent.setup();
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const fileItem = screen.getByRole('button', { name: /app\.ts/i });
			await user.click(fileItem);

			expect(mockOnFileClick).toHaveBeenCalledTimes(1);
			expect(mockOnFileClick).toHaveBeenCalledWith(files[0]);
		});

		it('passes correct file object on click', async () => {
			const user = userEvent.setup();
			const files: ChangedFile[] = [
				{ path: 'src/first.ts', status: 'modified' },
				{ path: 'src/second.ts', status: 'added', taskId: 'TASK-001' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const secondFile = screen.getByRole('button', { name: /second\.ts/i });
			await user.click(secondFile);

			expect(mockOnFileClick).toHaveBeenCalledWith(files[1]);
		});
	});

	describe('keyboard navigation', () => {
		it('supports Enter key to activate file', async () => {
			const user = userEvent.setup();
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const fileItem = screen.getByRole('button', { name: /app\.ts/i });
			fileItem.focus();
			await user.keyboard('{Enter}');

			expect(mockOnFileClick).toHaveBeenCalledTimes(1);
		});

		it('supports Space key to activate file', async () => {
			const user = userEvent.setup();
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const fileItem = screen.getByRole('button', { name: /app\.ts/i });
			fileItem.focus();
			await user.keyboard(' ');

			expect(mockOnFileClick).toHaveBeenCalledTimes(1);
		});

		it('file items are focusable', () => {
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const fileItem = screen.getByRole('button', { name: /app\.ts/i });
			expect(fileItem).toHaveAttribute('tabindex', '0');
		});
	});

	describe('collapse/expand', () => {
		it('starts expanded by default', () => {
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const header = screen.getByRole('button', { name: /Files Changed/i });
			expect(header).toHaveAttribute('aria-expanded', 'true');
		});

		it('collapses when header is clicked', async () => {
			const user = userEvent.setup();
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const header = screen.getByRole('button', { name: /Files Changed/i });
			await user.click(header);

			expect(header).toHaveAttribute('aria-expanded', 'false');
		});

		it('expands when collapsed header is clicked', async () => {
			const user = userEvent.setup();
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const header = screen.getByRole('button', { name: /Files Changed/i });
			await user.click(header); // collapse
			await user.click(header); // expand

			expect(header).toHaveAttribute('aria-expanded', 'true');
		});

		it('adds collapsed class when collapsed', async () => {
			const user = userEvent.setup();
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			const { container } = render(
				<FilesPanel files={files} onFileClick={mockOnFileClick} />
			);

			const header = screen.getByRole('button', { name: /Files Changed/i });
			await user.click(header);

			expect(container.querySelector('.files-panel')).toHaveClass('collapsed');
		});
	});

	describe('show more functionality', () => {
		it('shows all files when count is within maxVisible', () => {
			const files: ChangedFile[] = [
				{ path: 'src/a.ts', status: 'modified' },
				{ path: 'src/b.ts', status: 'modified' },
				{ path: 'src/c.ts', status: 'modified' },
			];

			render(
				<FilesPanel files={files} onFileClick={mockOnFileClick} maxVisible={5} />
			);

			expect(screen.getByText('src/a.ts')).toBeInTheDocument();
			expect(screen.getByText('src/b.ts')).toBeInTheDocument();
			expect(screen.getByText('src/c.ts')).toBeInTheDocument();
			expect(screen.queryByText(/more files/)).not.toBeInTheDocument();
		});

		it('shows "more files" link when count exceeds maxVisible', () => {
			const files: ChangedFile[] = [
				{ path: 'src/a.ts', status: 'modified' },
				{ path: 'src/b.ts', status: 'modified' },
				{ path: 'src/c.ts', status: 'modified' },
				{ path: 'src/d.ts', status: 'modified' },
			];

			render(
				<FilesPanel files={files} onFileClick={mockOnFileClick} maxVisible={2} />
			);

			expect(screen.getByText('src/a.ts')).toBeInTheDocument();
			expect(screen.getByText('src/b.ts')).toBeInTheDocument();
			expect(screen.queryByText('src/c.ts')).not.toBeInTheDocument();
			expect(screen.getByText('+ 2 more files')).toBeInTheDocument();
		});

		it('shows all files after clicking "more files"', async () => {
			const user = userEvent.setup();
			const files: ChangedFile[] = [
				{ path: 'src/a.ts', status: 'modified' },
				{ path: 'src/b.ts', status: 'modified' },
				{ path: 'src/c.ts', status: 'modified' },
				{ path: 'src/d.ts', status: 'modified' },
			];

			render(
				<FilesPanel files={files} onFileClick={mockOnFileClick} maxVisible={2} />
			);

			const moreButton = screen.getByText('+ 2 more files');
			await user.click(moreButton);

			expect(screen.getByText('src/c.ts')).toBeInTheDocument();
			expect(screen.getByText('src/d.ts')).toBeInTheDocument();
			expect(screen.queryByText(/more files/)).not.toBeInTheDocument();
		});

		it('calls onShowMore callback when provided', async () => {
			const user = userEvent.setup();
			const mockOnShowMore = vi.fn();
			const files: ChangedFile[] = [
				{ path: 'src/a.ts', status: 'modified' },
				{ path: 'src/b.ts', status: 'modified' },
				{ path: 'src/c.ts', status: 'modified' },
			];

			render(
				<FilesPanel
					files={files}
					onFileClick={mockOnFileClick}
					maxVisible={2}
					onShowMore={mockOnShowMore}
				/>
			);

			const moreButton = screen.getByText('+ 1 more files');
			await user.click(moreButton);

			expect(mockOnShowMore).toHaveBeenCalledTimes(1);
		});
	});

	describe('task grouping', () => {
		it('groups files by taskId when multiple tasks present', () => {
			const files: ChangedFile[] = [
				{ path: 'src/a.ts', status: 'modified', taskId: 'TASK-001' },
				{ path: 'src/b.ts', status: 'added', taskId: 'TASK-001' },
				{ path: 'src/c.ts', status: 'modified', taskId: 'TASK-002' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('TASK-002')).toBeInTheDocument();
		});

		it('does not show task headers for single task', () => {
			const files: ChangedFile[] = [
				{ path: 'src/a.ts', status: 'modified', taskId: 'TASK-001' },
				{ path: 'src/b.ts', status: 'added', taskId: 'TASK-001' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();
		});

		it('shows all files within each task group', () => {
			const files: ChangedFile[] = [
				{ path: 'src/a.ts', status: 'modified', taskId: 'TASK-001' },
				{ path: 'src/b.ts', status: 'added', taskId: 'TASK-001' },
				{ path: 'src/c.ts', status: 'modified', taskId: 'TASK-002' },
				{ path: 'src/d.ts', status: 'deleted', taskId: 'TASK-002' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			// All files should be visible
			expect(screen.getByText('src/a.ts')).toBeInTheDocument();
			expect(screen.getByText('src/b.ts')).toBeInTheDocument();
			expect(screen.getByText('src/c.ts')).toBeInTheDocument();
			expect(screen.getByText('src/d.ts')).toBeInTheDocument();
		});

		it('groups files with no taskId under "unknown"', () => {
			const files: ChangedFile[] = [
				{ path: 'src/a.ts', status: 'modified' },
				{ path: 'src/b.ts', status: 'added', taskId: 'TASK-001' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('unknown')).toBeInTheDocument();
		});
	});

	describe('file path display', () => {
		it('shows full file path', () => {
			const files: ChangedFile[] = [
				{ path: 'src/components/deep/nested/file.ts', status: 'modified' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(
				screen.getByText('src/components/deep/nested/file.ts')
			).toBeInTheDocument();
		});

		it('has title attribute with full path for tooltip', () => {
			const longPath = 'src/components/very/deep/nested/directory/structure/file.ts';
			const files: ChangedFile[] = [{ path: longPath, status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const pathElement = screen.getByText(longPath);
			expect(pathElement).toHaveAttribute('title', longPath);
		});

		it('uses monospace font for file path', () => {
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const pathElement = screen.getByText('src/app.ts');
			expect(pathElement).toHaveClass('file-path');
		});
	});

	describe('accessibility', () => {
		it('has proper aria-label for file items', () => {
			const files: ChangedFile[] = [
				{ path: 'src/app.ts', status: 'modified' },
				{ path: 'assets/logo.png', status: 'added' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(
				screen.getByRole('button', { name: /app\.ts, Modified/i })
			).toBeInTheDocument();
			expect(
				screen.getByRole('button', { name: /logo\.png, Added, binary file/i })
			).toBeInTheDocument();
		});

		it('has aria-label for file count badge', () => {
			const files: ChangedFile[] = [
				{ path: 'src/a.ts', status: 'modified' },
				{ path: 'src/b.ts', status: 'modified' },
				{ path: 'src/c.ts', status: 'modified' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const badge = screen.getByLabelText('3 files changed');
			expect(badge).toBeInTheDocument();
		});

		it('has aria-controls linking header to body', () => {
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const header = screen.getByRole('button', { name: /Files Changed/i });
			expect(header).toHaveAttribute('aria-controls', 'files-panel-body');
		});

		it('body has role="region"', () => {
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			const body = document.getElementById('files-panel-body');
			expect(body).toHaveAttribute('role', 'region');
		});
	});

	describe('edge cases', () => {
		it('handles files with special characters in path', () => {
			const files: ChangedFile[] = [
				{ path: 'src/file-name_v2.test.spec.ts', status: 'modified' },
				{ path: 'src/[slug]/page.tsx', status: 'added' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(
				screen.getByText('src/file-name_v2.test.spec.ts')
			).toBeInTheDocument();
			expect(screen.getByText('src/[slug]/page.tsx')).toBeInTheDocument();
		});

		it('handles single file correctly', () => {
			const files: ChangedFile[] = [{ path: 'src/app.ts', status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(screen.getByText('1')).toBeInTheDocument();
			expect(screen.getByText('src/app.ts')).toBeInTheDocument();
		});

		it('handles file without extension', () => {
			const files: ChangedFile[] = [
				{ path: 'Dockerfile', status: 'modified' },
				{ path: 'Makefile', status: 'added' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(screen.getByText('Dockerfile')).toBeInTheDocument();
			expect(screen.getByText('Makefile')).toBeInTheDocument();
		});

		it('handles deeply nested paths', () => {
			const deepPath =
				'src/components/features/dashboard/widgets/charts/LineChart.tsx';
			const files: ChangedFile[] = [{ path: deepPath, status: 'modified' }];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(screen.getByText(deepPath)).toBeInTheDocument();
		});

		it('handles root-level files', () => {
			const files: ChangedFile[] = [
				{ path: 'package.json', status: 'modified' },
			];

			render(<FilesPanel files={files} onFileClick={mockOnFileClick} />);

			expect(screen.getByText('package.json')).toBeInTheDocument();
		});

		it('handles large number of files', () => {
			const files: ChangedFile[] = Array.from({ length: 50 }, (_, i) => ({
				path: `src/file${i}.ts`,
				status: 'modified' as const,
			}));

			render(
				<FilesPanel files={files} onFileClick={mockOnFileClick} maxVisible={5} />
			);

			expect(screen.getByText('50')).toBeInTheDocument();
			expect(screen.getByText('+ 45 more files')).toBeInTheDocument();
		});
	});
});
