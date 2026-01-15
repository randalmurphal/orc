import { useState, useEffect, useCallback } from 'react';
import type { TestResultsInfo } from '@/lib/types';
import { getTestResults, getScreenshotUrl, getHTMLReportUrl, getTraceUrl } from '@/lib/api';
import { Icon } from '@/components/ui/Icon';
import './TestResultsTab.css';

interface TestResultsTabProps {
	taskId: string;
}

type TabId = 'summary' | 'screenshots' | 'suites';

function formatSize(bytes: number): string {
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
	return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDuration(ms: number): string {
	if (ms < 1000) return `${ms}ms`;
	if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
	return `${(ms / 60000).toFixed(1)}m`;
}

function formatDate(dateStr: string): string {
	const date = new Date(dateStr);
	return date.toLocaleDateString(undefined, {
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
	});
}

export function TestResultsTab({ taskId }: TestResultsTabProps) {
	const [results, setResults] = useState<TestResultsInfo | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [activeTab, setActiveTab] = useState<TabId>('summary');
	const [lightboxImage, setLightboxImage] = useState<string | null>(null);
	const [lightboxFilename, setLightboxFilename] = useState<string | null>(null);

	useEffect(() => {
		async function loadResults() {
			setLoading(true);
			setError(null);

			try {
				const data = await getTestResults(taskId);
				setResults(data);
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Failed to load test results');
			} finally {
				setLoading(false);
			}
		}

		loadResults();
	}, [taskId]);

	const openLightbox = useCallback(
		(filename: string) => {
			setLightboxImage(getScreenshotUrl(taskId, filename));
			setLightboxFilename(filename);
		},
		[taskId]
	);

	const closeLightbox = useCallback(() => {
		setLightboxImage(null);
		setLightboxFilename(null);
	}, []);

	// Handle escape key for lightbox
	useEffect(() => {
		const handleKeyDown = (e: KeyboardEvent) => {
			if (e.key === 'Escape' && lightboxImage) {
				closeLightbox();
			}
		};
		document.addEventListener('keydown', handleKeyDown);
		return () => document.removeEventListener('keydown', handleKeyDown);
	}, [lightboxImage, closeLightbox]);

	// Computed values
	const passRate =
		results?.report?.summary
			? Math.round((results.report.summary.passed / results.report.summary.total) * 100)
			: 0;

	const hasReport = results?.report != null;
	const hasScreenshots = (results?.screenshots?.length ?? 0) > 0;
	const hasSuites = (results?.report?.suites?.length ?? 0) > 0;

	if (loading) {
		return (
			<div className="test-results-container">
				<div className="loading-state">
					<div className="spinner" />
					<span>Loading test results...</span>
				</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="test-results-container">
				<div className="error-message">{error}</div>
			</div>
		);
	}

	if (!results?.has_results) {
		return (
			<div className="test-results-container">
				<div className="empty-state">
					<Icon name="file-text" size={48} />
					<h3>No Test Results</h3>
					<p>Playwright test results will appear here after running E2E tests.</p>
					<p className="hint">
						Set <code>ORC_TASK_ID</code> environment variable in your Playwright config to output
						results to this task.
					</p>
				</div>
			</div>
		);
	}

	return (
		<div className="test-results-container">
			{/* Tab navigation */}
			<div className="tabs">
				<button
					className={`tab ${activeTab === 'summary' ? 'active' : ''}`}
					onClick={() => setActiveTab('summary')}
				>
					Summary
				</button>
				{hasScreenshots && (
					<button
						className={`tab ${activeTab === 'screenshots' ? 'active' : ''}`}
						onClick={() => setActiveTab('screenshots')}
					>
						Screenshots ({results.screenshots?.length})
					</button>
				)}
				{hasSuites && (
					<button
						className={`tab ${activeTab === 'suites' ? 'active' : ''}`}
						onClick={() => setActiveTab('suites')}
					>
						Test Suites
					</button>
				)}
			</div>

			{/* Summary tab */}
			{activeTab === 'summary' && (
				<div className="summary-view">
					{hasReport && results.report ? (
						<>
							{/* Pass/Fail summary */}
							<div className="summary-cards">
								<div className="summary-card">
									<div className="summary-value passed">{results.report.summary.passed}</div>
									<div className="summary-label">Passed</div>
								</div>
								<div className="summary-card">
									<div className="summary-value failed">{results.report.summary.failed}</div>
									<div className="summary-label">Failed</div>
								</div>
								<div className="summary-card">
									<div className="summary-value skipped">{results.report.summary.skipped}</div>
									<div className="summary-label">Skipped</div>
								</div>
								<div className="summary-card">
									<div className="summary-value">{results.report.summary.total}</div>
									<div className="summary-label">Total</div>
								</div>
							</div>

							{/* Pass rate bar */}
							<div className="pass-rate-container">
								<div className="pass-rate-header">
									<span className="pass-rate-label">Pass Rate</span>
									<span className="pass-rate-value">{passRate}%</span>
								</div>
								<div className="pass-rate-bar">
									<div
										className={`pass-rate-fill ${passRate === 100 ? 'success' : passRate >= 80 ? 'warning' : 'danger'}`}
										style={{ width: `${passRate}%` }}
									/>
								</div>
							</div>

							{/* Metadata */}
							<div className="metadata">
								<div className="metadata-item">
									<span className="metadata-label">Framework</span>
									<span className="metadata-value">{results.report.framework}</span>
								</div>
								<div className="metadata-item">
									<span className="metadata-label">Duration</span>
									<span className="metadata-value">{formatDuration(results.report.duration)}</span>
								</div>
								{results.report.completed_at && (
									<div className="metadata-item">
										<span className="metadata-label">Completed</span>
										<span className="metadata-value">
											{formatDate(results.report.completed_at)}
										</span>
									</div>
								)}
							</div>

							{/* Coverage if available */}
							{results.report.coverage && (
								<div className="coverage-section">
									<h4>Code Coverage</h4>
									<div className="coverage-bar-container">
										<div className="coverage-header">
											<span>Overall</span>
											<span>{results.report.coverage.percentage.toFixed(1)}%</span>
										</div>
										<div className="coverage-bar">
											<div
												className="coverage-fill"
												style={{ width: `${results.report.coverage.percentage}%` }}
											/>
										</div>
									</div>
									{results.report.coverage.lines && (
										<div className="coverage-detail">
											<span className="coverage-detail-label">Lines</span>
											<span className="coverage-detail-value">
												{results.report.coverage.lines.percent.toFixed(1)}%
											</span>
										</div>
									)}
									{results.report.coverage.branches && (
										<div className="coverage-detail">
											<span className="coverage-detail-label">Branches</span>
											<span className="coverage-detail-value">
												{results.report.coverage.branches.percent.toFixed(1)}%
											</span>
										</div>
									)}
									{results.report.coverage.functions && (
										<div className="coverage-detail">
											<span className="coverage-detail-label">Functions</span>
											<span className="coverage-detail-value">
												{results.report.coverage.functions.percent.toFixed(1)}%
											</span>
										</div>
									)}
								</div>
							)}
						</>
					) : (
						<div className="no-report">
							<p>No structured report available. Screenshots and traces may still be available.</p>
						</div>
					)}

					{/* Quick links */}
					<div className="quick-links">
						{results.has_html_report && (
							<a
								href={getHTMLReportUrl(taskId)}
								target="_blank"
								rel="noopener noreferrer"
								className="quick-link"
							>
								<Icon name="file-text" size={16} />
								View HTML Report
							</a>
						)}
						{results.has_traces &&
							results.trace_files &&
							results.trace_files.length > 0 && (
								<a
									href={getTraceUrl(taskId, results.trace_files[0])}
									target="_blank"
									rel="noopener noreferrer"
									className="quick-link"
								>
									<Icon name="clock" size={16} />
									Download Trace ({results.trace_files.length} file
									{results.trace_files.length > 1 ? 's' : ''})
								</a>
							)}
					</div>
				</div>
			)}

			{/* Screenshots tab */}
			{activeTab === 'screenshots' && hasScreenshots && results.screenshots && (
				<div className="screenshots-view">
					<div className="screenshots-grid">
						{results.screenshots.map((screenshot) => (
							<div key={screenshot.filename} className="screenshot-card">
								<button
									className="screenshot-preview"
									onClick={() => openLightbox(screenshot.filename)}
									title="Click to enlarge"
								>
									<img
										src={getScreenshotUrl(taskId, screenshot.filename)}
										alt={screenshot.page_name}
										loading="lazy"
									/>
								</button>
								<div className="screenshot-info">
									<span className="screenshot-name" title={screenshot.page_name}>
										{screenshot.page_name}
									</span>
									<span className="screenshot-meta">{formatSize(screenshot.size)}</span>
								</div>
							</div>
						))}
					</div>
				</div>
			)}

			{/* Test Suites tab */}
			{activeTab === 'suites' && hasSuites && results.report && (
				<div className="suites-view">
					{results.report.suites.map((suite, idx) => (
						<div key={idx} className="suite-card">
							<div className="suite-header">
								<span className="suite-name">{suite.name}</span>
								<span className="suite-count">
									{suite.tests.filter((t) => t.status === 'passed').length}/{suite.tests.length}{' '}
									passed
								</span>
							</div>
							<div className="suite-tests">
								{suite.tests.map((test, testIdx) => (
									<div key={testIdx}>
										<div className={`test-item ${test.status}`}>
											<span className="test-status-icon">
												{test.status === 'passed' && <Icon name="check" size={14} />}
												{test.status === 'failed' && <Icon name="x" size={14} />}
												{test.status === 'skipped' && <Icon name="slash" size={14} />}
											</span>
											<span className="test-name">{test.name}</span>
											<span className="test-duration">{formatDuration(test.duration)}</span>
										</div>
										{test.error && (
											<div className="test-error">
												<pre>{test.error}</pre>
											</div>
										)}
									</div>
								))}
							</div>
						</div>
					))}
				</div>
			)}

			{/* Lightbox modal */}
			{lightboxImage && (
				<div
					className="lightbox"
					onClick={closeLightbox}
					role="dialog"
					aria-modal="true"
					tabIndex={-1}
				>
					<div className="lightbox-content" onClick={(e) => e.stopPropagation()}>
						<button className="lightbox-close" onClick={closeLightbox} aria-label="Close">
							<Icon name="x" size={24} />
						</button>
						<img src={lightboxImage} alt={lightboxFilename ?? 'Screenshot'} />
						{lightboxFilename && <div className="lightbox-filename">{lightboxFilename}</div>}
					</div>
				</div>
			)}
		</div>
	);
}
