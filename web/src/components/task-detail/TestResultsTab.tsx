import { useState, useEffect, useCallback } from 'react';
import { taskClient } from '@/lib/client';
import { create } from '@bufbuild/protobuf';
import {
	type TestResultsInfo,
	type TestResult,
	GetTestResultsRequestSchema,
	TestResultStatus,
} from '@/gen/orc/v1/task_pb';
import { timestampToDate } from '@/lib/time';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { useCurrentProjectId } from '@/stores';
import './TestResultsTab.css';

interface TestResultsTabProps {
	taskId: string;
}

type TabId = 'summary' | 'screenshots' | 'suites';

function formatSize(bytes: number | bigint): string {
	const numBytes = typeof bytes === 'bigint' ? Number(bytes) : bytes;
	if (numBytes < 1024) return `${numBytes} B`;
	if (numBytes < 1024 * 1024) return `${(numBytes / 1024).toFixed(1)} KB`;
	return `${(numBytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDuration(ms: number | bigint): string {
	const numMs = typeof ms === 'bigint' ? Number(ms) : ms;
	if (numMs < 1000) return `${numMs}ms`;
	if (numMs < 60000) return `${(numMs / 1000).toFixed(1)}s`;
	return `${(numMs / 60000).toFixed(1)}m`;
}

function formatDate(date: Date | null): string {
	if (!date) return '';
	return date.toLocaleDateString(undefined, {
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
	});
}

// URL helpers (use /files/ endpoint for binary file serving)
function getScreenshotUrl(taskId: string, filename: string): string {
	return `/files/tasks/${taskId}/test-results/screenshots/${encodeURIComponent(filename)}`;
}

function getHTMLReportUrl(taskId: string): string {
	return `/files/tasks/${taskId}/test-results/html-report`;
}

function getTraceUrl(taskId: string, filename: string): string {
	return `/files/tasks/${taskId}/test-results/traces/${encodeURIComponent(filename)}`;
}

// Helper to check test status
function testStatusMatches(test: TestResult, status: TestResultStatus): boolean {
	return test.status === status;
}

export function TestResultsTab({ taskId }: TestResultsTabProps) {
	const projectId = useCurrentProjectId();
	const [results, setResults] = useState<TestResultsInfo | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [activeTab, setActiveTab] = useState<TabId>('summary');
	const [lightboxImage, setLightboxImage] = useState<string | null>(null);
	const [lightboxFilename, setLightboxFilename] = useState<string | null>(null);

	useEffect(() => {
		async function loadResults() {
			if (!projectId) return;
			setLoading(true);
			setError(null);

			try {
				const response = await taskClient.getTestResults(
					create(GetTestResultsRequestSchema, { projectId, taskId })
				);
				setResults(response.results ?? null);
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Failed to load test results');
			} finally {
				setLoading(false);
			}
		}

		loadResults();
	}, [projectId, taskId]);

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

	if (!results?.hasResults) {
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
				<Button
					variant={activeTab === 'summary' ? 'primary' : 'ghost'}
					size="sm"
					className={`tab ${activeTab === 'summary' ? 'active' : ''}`}
					onClick={() => setActiveTab('summary')}
				>
					Summary
				</Button>
				{hasScreenshots && (
					<Button
						variant={activeTab === 'screenshots' ? 'primary' : 'ghost'}
						size="sm"
						className={`tab ${activeTab === 'screenshots' ? 'active' : ''}`}
						onClick={() => setActiveTab('screenshots')}
					>
						Screenshots ({results.screenshots?.length})
					</Button>
				)}
				{hasSuites && (
					<Button
						variant={activeTab === 'suites' ? 'primary' : 'ghost'}
						size="sm"
						className={`tab ${activeTab === 'suites' ? 'active' : ''}`}
						onClick={() => setActiveTab('suites')}
					>
						Test Suites
					</Button>
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
									<div className="summary-value passed">{results.report.summary?.passed ?? 0}</div>
									<div className="summary-label">Passed</div>
								</div>
								<div className="summary-card">
									<div className="summary-value failed">{results.report.summary?.failed ?? 0}</div>
									<div className="summary-label">Failed</div>
								</div>
								<div className="summary-card">
									<div className="summary-value skipped">{results.report.summary?.skipped ?? 0}</div>
									<div className="summary-label">Skipped</div>
								</div>
								<div className="summary-card">
									<div className="summary-value">{results.report.summary?.total ?? 0}</div>
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
									<span className="metadata-value">{formatDuration(results.report.durationMs)}</span>
								</div>
								{results.report.completedAt && (
									<div className="metadata-item">
										<span className="metadata-label">Completed</span>
										<span className="metadata-value">
											{formatDate(timestampToDate(results.report.completedAt))}
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
						{results.hasHtmlReport && (
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
						{results.hasTraces &&
							results.traceFiles &&
							results.traceFiles.length > 0 && (
								<a
									href={getTraceUrl(taskId, results.traceFiles[0])}
									target="_blank"
									rel="noopener noreferrer"
									className="quick-link"
								>
									<Icon name="clock" size={16} />
									Download Trace ({results.traceFiles.length} file
									{results.traceFiles.length > 1 ? 's' : ''})
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
								<Button
									variant="ghost"
									className="screenshot-preview"
									onClick={() => openLightbox(screenshot.filename)}
									title="Click to enlarge"
									aria-label={`Preview ${screenshot.pageName}`}
								>
									<img
										src={getScreenshotUrl(taskId, screenshot.filename)}
										alt={screenshot.pageName}
										loading="lazy"
									/>
								</Button>
								<div className="screenshot-info">
									<span className="screenshot-name" title={screenshot.pageName}>
										{screenshot.pageName}
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
									{suite.tests.filter((t) => testStatusMatches(t, TestResultStatus.PASSED)).length}/{suite.tests.length}{' '}
									passed
								</span>
							</div>
							<div className="suite-tests">
								{suite.tests.map((test, testIdx) => (
									<div key={testIdx}>
										<div className={`test-item ${test.status === TestResultStatus.PASSED ? 'passed' : test.status === TestResultStatus.FAILED ? 'failed' : 'skipped'}`}>
											<span className="test-status-icon">
												{test.status === TestResultStatus.PASSED && <Icon name="check" size={14} />}
												{test.status === TestResultStatus.FAILED && <Icon name="x" size={14} />}
												{test.status === TestResultStatus.SKIPPED && <Icon name="slash" size={14} />}
											</span>
											<span className="test-name">{test.name}</span>
											<span className="test-duration">{formatDuration(test.durationMs)}</span>
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
						<Button variant="ghost" iconOnly className="lightbox-close" onClick={closeLightbox} aria-label="Close">
							<Icon name="x" size={24} />
						</Button>
						<img src={lightboxImage} alt={lightboxFilename ?? 'Screenshot'} />
						{lightboxFilename && <div className="lightbox-filename">{lightboxFilename}</div>}
					</div>
				</div>
			)}
		</div>
	);
}
