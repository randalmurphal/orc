import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { ImportExportPage } from './ImportExport';

// Mock fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Mock URL.createObjectURL and URL.revokeObjectURL
const mockCreateObjectURL = vi.fn(() => 'blob:test');
const mockRevokeObjectURL = vi.fn();
global.URL.createObjectURL = mockCreateObjectURL;
global.URL.revokeObjectURL = mockRevokeObjectURL;

describe('ImportExportPage', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	// ==========================================================================
	// SC-3: Frontend renders export options and import upload
	// ==========================================================================

	describe('export section', () => {
		it('renders export section with title and description', () => {
			render(<ImportExportPage />);

			expect(screen.getByRole('heading', { name: 'Export' })).toBeInTheDocument();
			expect(screen.getByText(/Export tasks and initiatives/)).toBeInTheDocument();
		});

		it('renders export options checkboxes', () => {
			render(<ImportExportPage />);

			expect(screen.getByTestId('export-all-tasks')).toBeInTheDocument();
			expect(screen.getByTestId('export-transcripts')).toBeInTheDocument();
			expect(screen.getByTestId('export-initiatives')).toBeInTheDocument();
			expect(screen.getByTestId('export-minimal')).toBeInTheDocument();
		});

		it('renders export button', () => {
			render(<ImportExportPage />);

			const exportButton = screen.getByTestId('export-button');
			expect(exportButton).toBeInTheDocument();
			expect(exportButton).toHaveTextContent('Export');
		});

		it('all tasks checkbox is checked by default', () => {
			render(<ImportExportPage />);

			const checkbox = screen.getByTestId('export-all-tasks');
			expect(checkbox).toBeChecked();
		});

		it('export button triggers API call', async () => {
			// Mock successful export response
			const mockBlob = new Blob(['test data'], { type: 'application/gzip' });
			mockFetch.mockResolvedValueOnce({
				ok: true,
				headers: new Headers({
					'Content-Disposition': 'attachment; filename="orc-export.tar.gz"',
				}),
				blob: () => Promise.resolve(mockBlob),
			});

			render(<ImportExportPage />);

			const exportButton = screen.getByTestId('export-button');
			fireEvent.click(exportButton);

			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith('/api/export', expect.objectContaining({
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
				}));
			});
		});

		it('minimal option disables transcripts', () => {
			render(<ImportExportPage />);

			const minimalCheckbox = screen.getByTestId('export-minimal');
			const transcriptsCheckbox = screen.getByTestId('export-transcripts');

			// Initially transcripts is not disabled
			expect(transcriptsCheckbox).not.toBeDisabled();

			// Check minimal
			fireEvent.click(minimalCheckbox);

			// Now transcripts should be disabled
			expect(transcriptsCheckbox).toBeDisabled();
		});
	});

	describe('import section', () => {
		it('renders import section with title and description', () => {
			render(<ImportExportPage />);

			expect(screen.getByRole('heading', { name: 'Import' })).toBeInTheDocument();
			expect(screen.getByText(/Import tasks and initiatives/)).toBeInTheDocument();
		});

		it('renders file upload input', () => {
			render(<ImportExportPage />);

			expect(screen.getByTestId('import-file-input')).toBeInTheDocument();
			expect(screen.getByText('Choose a tar.gz file')).toBeInTheDocument();
		});

		it('renders import button', () => {
			render(<ImportExportPage />);

			const importButton = screen.getByTestId('import-button');
			expect(importButton).toBeInTheDocument();
			expect(importButton).toHaveTextContent('Import');
		});

		it('import button is disabled when no file selected', () => {
			render(<ImportExportPage />);

			const importButton = screen.getByTestId('import-button');
			expect(importButton).toBeDisabled();
		});

		it('renders dry run checkbox', () => {
			render(<ImportExportPage />);

			const dryRunCheckbox = screen.getByTestId('import-dry-run');
			expect(dryRunCheckbox).toBeInTheDocument();
		});

		it('selecting file enables import button', async () => {
			render(<ImportExportPage />);

			const fileInput = screen.getByTestId('import-file-input');
			const importButton = screen.getByTestId('import-button');

			// Create a mock file
			const file = new File(['test content'], 'test-export.tar.gz', {
				type: 'application/gzip',
			});

			// Simulate file selection
			fireEvent.change(fileInput, { target: { files: [file] } });

			await waitFor(() => {
				expect(importButton).not.toBeDisabled();
			});
		});

		it('import with file calls API', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({
					tasks_imported: 1,
					tasks_skipped: 0,
					initiatives_imported: 0,
					initiatives_skipped: 0,
					dry_run: false,
				}),
			});

			render(<ImportExportPage />);

			// Select file
			const fileInput = screen.getByTestId('import-file-input');
			const file = new File(['test content'], 'test-export.tar.gz', {
				type: 'application/gzip',
			});
			fireEvent.change(fileInput, { target: { files: [file] } });

			// Click import
			const importButton = screen.getByTestId('import-button');
			await waitFor(() => expect(importButton).not.toBeDisabled());
			fireEvent.click(importButton);

			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith('/api/import', expect.objectContaining({
					method: 'POST',
				}));
			});
		});

		it('dry run option changes API URL', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({
					tasks_imported: 1,
					tasks_skipped: 0,
					initiatives_imported: 0,
					initiatives_skipped: 0,
					dry_run: true,
				}),
			});

			render(<ImportExportPage />);

			// Select file
			const fileInput = screen.getByTestId('import-file-input');
			const file = new File(['test content'], 'test-export.tar.gz', {
				type: 'application/gzip',
			});
			fireEvent.change(fileInput, { target: { files: [file] } });

			// Enable dry run
			const dryRunCheckbox = screen.getByTestId('import-dry-run');
			fireEvent.click(dryRunCheckbox);

			// Click import
			const importButton = screen.getByTestId('import-button');
			await waitFor(() => expect(importButton).not.toBeDisabled());
			fireEvent.click(importButton);

			await waitFor(() => {
				expect(mockFetch).toHaveBeenCalledWith('/api/import?dry_run=true', expect.anything());
			});
		});

		it('shows import results after successful import', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({
					tasks_imported: 3,
					tasks_skipped: 1,
					initiatives_imported: 2,
					initiatives_skipped: 0,
					dry_run: false,
				}),
			});

			render(<ImportExportPage />);

			// Select file
			const fileInput = screen.getByTestId('import-file-input');
			const file = new File(['test content'], 'test-export.tar.gz', {
				type: 'application/gzip',
			});
			fireEvent.change(fileInput, { target: { files: [file] } });

			// Click import
			const importButton = screen.getByTestId('import-button');
			await waitFor(() => expect(importButton).not.toBeDisabled());
			fireEvent.click(importButton);

			await waitFor(() => {
				expect(screen.getByText(/Tasks: 3 imported, 1 skipped/)).toBeInTheDocument();
				expect(screen.getByText(/Initiatives: 2 imported, 0 skipped/)).toBeInTheDocument();
			});
		});
	});

	describe('info section', () => {
		it('renders info section with how it works', () => {
			render(<ImportExportPage />);

			expect(screen.getByRole('heading', { name: 'How it works' })).toBeInTheDocument();
			expect(screen.getByText(/creates a tar.gz archive/)).toBeInTheDocument();
			expect(screen.getByText(/smart merge logic/)).toBeInTheDocument();
		});
	});

	describe('error handling', () => {
		it('shows error message when export fails', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: false,
				text: () => Promise.resolve('Export failed: internal error'),
			});

			render(<ImportExportPage />);

			const exportButton = screen.getByTestId('export-button');
			fireEvent.click(exportButton);

			await waitFor(() => {
				expect(screen.getByText(/Export failed/)).toBeInTheDocument();
			});
		});

		it('shows error message when import fails', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: false,
				text: () => Promise.resolve('Import failed: invalid file'),
			});

			render(<ImportExportPage />);

			// Select file
			const fileInput = screen.getByTestId('import-file-input');
			const file = new File(['test content'], 'test-export.tar.gz', {
				type: 'application/gzip',
			});
			fireEvent.change(fileInput, { target: { files: [file] } });

			// Click import
			const importButton = screen.getByTestId('import-button');
			await waitFor(() => expect(importButton).not.toBeDisabled());
			fireEvent.click(importButton);

			await waitFor(() => {
				expect(screen.getByText(/Import failed/)).toBeInTheDocument();
			});
		});
	});
});
