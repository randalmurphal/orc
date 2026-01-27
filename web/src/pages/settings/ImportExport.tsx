/**
 * Import/Export settings page (/settings/import-export)
 * Provides UI for exporting tasks as tar.gz and importing from uploaded files.
 */

import { useState, useCallback } from 'react';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { useDocumentTitle } from '@/hooks';
import './ImportExport.css';

interface ExportOptions {
	allTasks: boolean;
	includeTranscripts: boolean;
	includeInitiatives: boolean;
	minimal: boolean;
}

interface ImportResult {
	tasks_imported: number;
	tasks_skipped: number;
	initiatives_imported: number;
	initiatives_skipped: number;
	errors?: string[];
	dry_run: boolean;
}

export function ImportExportPage() {
	useDocumentTitle('Import / Export');

	// Export state
	const [exportOptions, setExportOptions] = useState<ExportOptions>({
		allTasks: true,
		includeTranscripts: true,
		includeInitiatives: true,
		minimal: false,
	});
	const [exporting, setExporting] = useState(false);
	const [exportError, setExportError] = useState<string | null>(null);

	// Import state
	const [importFile, setImportFile] = useState<File | null>(null);
	const [importing, setImporting] = useState(false);
	const [importResult, setImportResult] = useState<ImportResult | null>(null);
	const [importError, setImportError] = useState<string | null>(null);
	const [dryRun, setDryRun] = useState(false);

	// Handle export
	const handleExport = useCallback(async () => {
		try {
			setExporting(true);
			setExportError(null);

			const response = await fetch('/api/export', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify({
					all_tasks: exportOptions.allTasks,
					include_transcripts: exportOptions.includeTranscripts,
					include_initiatives: exportOptions.includeInitiatives,
					minimal: exportOptions.minimal,
				}),
			});

			if (!response.ok) {
				const text = await response.text();
				throw new Error(text || 'Export failed');
			}

			// Get filename from Content-Disposition header
			const disposition = response.headers.get('Content-Disposition');
			let filename = 'orc-export.tar.gz';
			if (disposition) {
				const match = disposition.match(/filename="?([^"]+)"?/);
				if (match) {
					filename = match[1];
				}
			}

			// Download the file
			const blob = await response.blob();
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = filename;
			document.body.appendChild(a);
			a.click();
			document.body.removeChild(a);
			URL.revokeObjectURL(url);
		} catch (err) {
			setExportError(err instanceof Error ? err.message : 'Export failed');
		} finally {
			setExporting(false);
		}
	}, [exportOptions]);

	// Handle file selection
	const handleFileChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
		const file = e.target.files?.[0];
		if (file) {
			setImportFile(file);
			setImportResult(null);
			setImportError(null);
		}
	}, []);

	// Handle import
	const handleImport = useCallback(async () => {
		if (!importFile) return;

		try {
			setImporting(true);
			setImportError(null);
			setImportResult(null);

			const formData = new FormData();
			formData.append('file', importFile);

			const url = dryRun ? '/api/import?dry_run=true' : '/api/import';
			const response = await fetch(url, {
				method: 'POST',
				body: formData,
			});

			if (!response.ok) {
				const text = await response.text();
				throw new Error(text || 'Import failed');
			}

			const result: ImportResult = await response.json();
			setImportResult(result);
		} catch (err) {
			setImportError(err instanceof Error ? err.message : 'Import failed');
		} finally {
			setImporting(false);
		}
	}, [importFile, dryRun]);

	// Handle option changes
	const handleOptionChange = useCallback((key: keyof ExportOptions) => {
		setExportOptions(prev => {
			const newOptions = { ...prev, [key]: !prev[key] };
			// If minimal is enabled, disable includeTranscripts
			if (key === 'minimal' && newOptions.minimal) {
				newOptions.includeTranscripts = false;
			}
			// If includeTranscripts is enabled, disable minimal
			if (key === 'includeTranscripts' && newOptions.includeTranscripts) {
				newOptions.minimal = false;
			}
			return newOptions;
		});
	}, []);

	return (
		<div className="page import-export-page">
			{/* Export Section */}
			<section className="import-export-section">
				<div className="import-export-header">
					<div>
						<h3>Export</h3>
						<p className="import-export-description">
							Export tasks and initiatives as a tar.gz archive for backup or transfer.
						</p>
					</div>
				</div>

				{exportError && (
					<div className="import-export-error">
						<Icon name="alert-circle" size={16} />
						{exportError}
					</div>
				)}

				<div className="export-options">
					<label className="export-option">
						<input
							type="checkbox"
							checked={exportOptions.allTasks}
							onChange={() => handleOptionChange('allTasks')}
							data-testid="export-all-tasks"
						/>
						<span>All tasks</span>
					</label>

					<label className="export-option">
						<input
							type="checkbox"
							checked={exportOptions.includeTranscripts}
							onChange={() => handleOptionChange('includeTranscripts')}
							disabled={exportOptions.minimal}
							data-testid="export-transcripts"
						/>
						<span>Include transcripts</span>
						<span className="option-hint">(Larger file size)</span>
					</label>

					<label className="export-option">
						<input
							type="checkbox"
							checked={exportOptions.includeInitiatives}
							onChange={() => handleOptionChange('includeInitiatives')}
							data-testid="export-initiatives"
						/>
						<span>Include initiatives</span>
					</label>

					<label className="export-option">
						<input
							type="checkbox"
							checked={exportOptions.minimal}
							onChange={() => handleOptionChange('minimal')}
							data-testid="export-minimal"
						/>
						<span>Minimal export</span>
						<span className="option-hint">(Excludes transcripts)</span>
					</label>
				</div>

				<Button
					variant="primary"
					onClick={handleExport}
					disabled={exporting || !exportOptions.allTasks}
					leftIcon={<Icon name="download" size={14} />}
					data-testid="export-button"
				>
					{exporting ? 'Exporting...' : 'Export'}
				</Button>
			</section>

			{/* Import Section */}
			<section className="import-export-section">
				<div className="import-export-header">
					<div>
						<h3>Import</h3>
						<p className="import-export-description">
							Import tasks and initiatives from a tar.gz archive.
							Uses smart merge: newer items overwrite older ones.
						</p>
					</div>
				</div>

				{importError && (
					<div className="import-export-error">
						<Icon name="alert-circle" size={16} />
						{importError}
					</div>
				)}

				{importResult && (
					<div className={`import-export-result ${importResult.dry_run ? 'dry-run' : ''}`}>
						<Icon name={importResult.dry_run ? 'info' : 'check-circle'} size={16} />
						<div>
							{importResult.dry_run && <strong>Preview (dry run):</strong>}
							<ul>
								<li>Tasks: {importResult.tasks_imported} imported, {importResult.tasks_skipped} skipped</li>
								<li>Initiatives: {importResult.initiatives_imported} imported, {importResult.initiatives_skipped} skipped</li>
							</ul>
							{importResult.errors && importResult.errors.length > 0 && (
								<div className="import-errors">
									<strong>Errors:</strong>
									<ul>
										{importResult.errors.map((err, i) => (
											<li key={i}>{err}</li>
										))}
									</ul>
								</div>
							)}
						</div>
					</div>
				)}

				<div className="import-file-section">
					<input
						type="file"
						accept=".tar.gz,.tgz"
						onChange={handleFileChange}
						className="import-file-input"
						id="import-file-input"
						data-testid="import-file-input"
					/>
					<label htmlFor="import-file-input" className="import-file-label">
						<Icon name="upload" size={20} />
						{importFile ? importFile.name : 'Choose a tar.gz file'}
					</label>
				</div>

				<div className="import-options">
					<label className="export-option">
						<input
							type="checkbox"
							checked={dryRun}
							onChange={() => setDryRun(!dryRun)}
							data-testid="import-dry-run"
						/>
						<span>Dry run</span>
						<span className="option-hint">(Preview without importing)</span>
					</label>
				</div>

				<Button
					variant="primary"
					onClick={handleImport}
					disabled={importing || !importFile}
					leftIcon={<Icon name="upload" size={14} />}
					data-testid="import-button"
				>
					{importing ? 'Importing...' : dryRun ? 'Preview Import' : 'Import'}
				</Button>
			</section>

			{/* Info Section */}
			<div className="import-export-info">
				<h4>How it works</h4>
				<ul>
					<li>
						<strong>Export</strong> creates a tar.gz archive containing YAML files for each task and initiative.
					</li>
					<li>
						<strong>Import</strong> uses smart merge logic: items with newer timestamps overwrite older ones.
					</li>
					<li>
						<strong>Dry run</strong> shows what would be imported without making changes.
					</li>
					<li>
						Running tasks imported from another machine are automatically set to "paused" status.
					</li>
				</ul>
			</div>
		</div>
	);
}
