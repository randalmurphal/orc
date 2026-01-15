<script lang="ts">
	import { onMount } from 'svelte';
	import type { TestResultsInfo, Screenshot, TestSuite, TestResult } from '$lib/types';
	import { getTestResults, getScreenshotUrl, getHTMLReportUrl, getTraceUrl } from '$lib/api';

	interface Props {
		taskId: string;
	}

	let { taskId }: Props = $props();

	let results = $state<TestResultsInfo | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// For lightbox
	let lightboxImage = $state<string | null>(null);
	let lightboxFilename = $state<string | null>(null);

	// View toggle
	let activeTab = $state<'summary' | 'screenshots' | 'suites'>('summary');

	onMount(async () => {
		await loadResults();
	});

	async function loadResults() {
		loading = true;
		error = null;

		try {
			results = await getTestResults(taskId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load test results';
		} finally {
			loading = false;
		}
	}

	function openLightbox(filename: string) {
		lightboxImage = getScreenshotUrl(taskId, filename);
		lightboxFilename = filename;
	}

	function closeLightbox() {
		lightboxImage = null;
		lightboxFilename = null;
	}

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
			minute: '2-digit'
		});
	}

	// Computed values
	const passRate = $derived(
		results?.report?.summary
			? Math.round(
					(results.report.summary.passed / results.report.summary.total) * 100
				)
			: 0
	);

	const hasReport = $derived(results?.report != null);
	const hasScreenshots = $derived((results?.screenshots?.length ?? 0) > 0);
	const hasSuites = $derived((results?.report?.suites?.length ?? 0) > 0);
</script>

<div class="test-results-container">
	{#if loading}
		<div class="loading-state">
			<div class="spinner"></div>
			<span>Loading test results...</span>
		</div>
	{:else if error}
		<div class="error-message">{error}</div>
	{:else if !results?.has_results}
		<div class="empty-state">
			<svg
				xmlns="http://www.w3.org/2000/svg"
				width="48"
				height="48"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="1.5"
			>
				<path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z" />
				<polyline points="14 2 14 8 20 8" />
				<line x1="9" y1="15" x2="15" y2="15" />
			</svg>
			<h3>No Test Results</h3>
			<p>Playwright test results will appear here after running E2E tests.</p>
			<p class="hint">
				Set <code>ORC_TASK_ID</code> environment variable in your Playwright config to output
				results to this task.
			</p>
		</div>
	{:else}
		<!-- Tab navigation -->
		<div class="tabs">
			<button
				class="tab"
				class:active={activeTab === 'summary'}
				onclick={() => (activeTab = 'summary')}
			>
				Summary
			</button>
			{#if hasScreenshots}
				<button
					class="tab"
					class:active={activeTab === 'screenshots'}
					onclick={() => (activeTab = 'screenshots')}
				>
					Screenshots ({results.screenshots.length})
				</button>
			{/if}
			{#if hasSuites}
				<button
					class="tab"
					class:active={activeTab === 'suites'}
					onclick={() => (activeTab = 'suites')}
				>
					Test Suites
				</button>
			{/if}
		</div>

		<!-- Summary tab -->
		{#if activeTab === 'summary'}
			<div class="summary-view">
				{#if hasReport && results.report}
					<!-- Pass/Fail summary -->
					<div class="summary-cards">
						<div class="summary-card">
							<div class="summary-value passed">{results.report.summary.passed}</div>
							<div class="summary-label">Passed</div>
						</div>
						<div class="summary-card">
							<div class="summary-value failed">{results.report.summary.failed}</div>
							<div class="summary-label">Failed</div>
						</div>
						<div class="summary-card">
							<div class="summary-value skipped">{results.report.summary.skipped}</div>
							<div class="summary-label">Skipped</div>
						</div>
						<div class="summary-card">
							<div class="summary-value">{results.report.summary.total}</div>
							<div class="summary-label">Total</div>
						</div>
					</div>

					<!-- Pass rate bar -->
					<div class="pass-rate-container">
						<div class="pass-rate-header">
							<span class="pass-rate-label">Pass Rate</span>
							<span class="pass-rate-value">{passRate}%</span>
						</div>
						<div class="pass-rate-bar">
							<div
								class="pass-rate-fill"
								class:success={passRate === 100}
								class:warning={passRate >= 80 && passRate < 100}
								class:danger={passRate < 80}
								style="width: {passRate}%"
							></div>
						</div>
					</div>

					<!-- Metadata -->
					<div class="metadata">
						<div class="metadata-item">
							<span class="metadata-label">Framework</span>
							<span class="metadata-value">{results.report.framework}</span>
						</div>
						<div class="metadata-item">
							<span class="metadata-label">Duration</span>
							<span class="metadata-value">{formatDuration(results.report.duration)}</span>
						</div>
						{#if results.report.completed_at}
							<div class="metadata-item">
								<span class="metadata-label">Completed</span>
								<span class="metadata-value">{formatDate(results.report.completed_at)}</span>
							</div>
						{/if}
					</div>

					<!-- Coverage if available -->
					{#if results.report.coverage}
						<div class="coverage-section">
							<h4>Code Coverage</h4>
							<div class="coverage-bar-container">
								<div class="coverage-header">
									<span>Overall</span>
									<span>{results.report.coverage.percentage.toFixed(1)}%</span>
								</div>
								<div class="coverage-bar">
									<div
										class="coverage-fill"
										style="width: {results.report.coverage.percentage}%"
									></div>
								</div>
							</div>
							{#if results.report.coverage.lines}
								<div class="coverage-detail">
									<span class="coverage-detail-label">Lines</span>
									<span class="coverage-detail-value"
										>{results.report.coverage.lines.percent.toFixed(1)}%</span
									>
								</div>
							{/if}
							{#if results.report.coverage.branches}
								<div class="coverage-detail">
									<span class="coverage-detail-label">Branches</span>
									<span class="coverage-detail-value"
										>{results.report.coverage.branches.percent.toFixed(1)}%</span
									>
								</div>
							{/if}
							{#if results.report.coverage.functions}
								<div class="coverage-detail">
									<span class="coverage-detail-label">Functions</span>
									<span class="coverage-detail-value"
										>{results.report.coverage.functions.percent.toFixed(1)}%</span
									>
								</div>
							{/if}
						</div>
					{/if}
				{:else}
					<div class="no-report">
						<p>No structured report available. Screenshots and traces may still be available.</p>
					</div>
				{/if}

				<!-- Quick links -->
				<div class="quick-links">
					{#if results.has_html_report}
						<a
							href={getHTMLReportUrl(taskId)}
							target="_blank"
							rel="noopener"
							class="quick-link"
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="16"
								height="16"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
							>
								<path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z" />
								<polyline points="14 2 14 8 20 8" />
							</svg>
							View HTML Report
						</a>
					{/if}
					{#if results.has_traces && results.trace_files && results.trace_files.length > 0}
						<a
							href={getTraceUrl(taskId, results.trace_files[0])}
							target="_blank"
							rel="noopener"
							class="quick-link"
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="16"
								height="16"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
							>
								<circle cx="12" cy="12" r="10" />
								<polyline points="12 6 12 12 16 14" />
							</svg>
							Download Trace ({results.trace_files.length} file{results.trace_files.length > 1 ? 's' : ''})
						</a>
					{/if}
				</div>
			</div>
		{/if}

		<!-- Screenshots tab -->
		{#if activeTab === 'screenshots' && hasScreenshots}
			<div class="screenshots-view">
				<div class="screenshots-grid">
					{#each results.screenshots as screenshot}
						<div class="screenshot-card">
							<button
								class="screenshot-preview"
								onclick={() => openLightbox(screenshot.filename)}
								title="Click to enlarge"
							>
								<img
									src={getScreenshotUrl(taskId, screenshot.filename)}
									alt={screenshot.page_name}
									loading="lazy"
								/>
							</button>
							<div class="screenshot-info">
								<span class="screenshot-name" title={screenshot.page_name}
									>{screenshot.page_name}</span
								>
								<span class="screenshot-meta">{formatSize(screenshot.size)}</span>
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		<!-- Test Suites tab -->
		{#if activeTab === 'suites' && hasSuites && results.report}
			<div class="suites-view">
				{#each results.report.suites as suite}
					<div class="suite-card">
						<div class="suite-header">
							<span class="suite-name">{suite.name}</span>
							<span class="suite-count">
								{suite.tests.filter((t) => t.status === 'passed').length}/{suite.tests.length} passed
							</span>
						</div>
						<div class="suite-tests">
							{#each suite.tests as test}
								<div class="test-item" class:passed={test.status === 'passed'} class:failed={test.status === 'failed'} class:skipped={test.status === 'skipped'}>
									<span class="test-status-icon">
										{#if test.status === 'passed'}
											<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
												<polyline points="20 6 9 17 4 12" />
											</svg>
										{:else if test.status === 'failed'}
											<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
												<line x1="18" y1="6" x2="6" y2="18" />
												<line x1="6" y1="6" x2="18" y2="18" />
											</svg>
										{:else}
											<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
												<circle cx="12" cy="12" r="10" />
												<line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
											</svg>
										{/if}
									</span>
									<span class="test-name">{test.name}</span>
									<span class="test-duration">{formatDuration(test.duration)}</span>
								</div>
								{#if test.error}
									<div class="test-error">
										<pre>{test.error}</pre>
									</div>
								{/if}
							{/each}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	{/if}
</div>

<!-- Lightbox modal -->
{#if lightboxImage}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		class="lightbox"
		onclick={closeLightbox}
		onkeydown={(e) => e.key === 'Escape' && closeLightbox()}
		role="dialog"
		aria-modal="true"
		tabindex="-1"
	>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="lightbox-content" onclick={(e) => e.stopPropagation()} role="presentation">
			<button class="lightbox-close" onclick={closeLightbox} aria-label="Close">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="24"
					height="24"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
				>
					<line x1="18" y1="6" x2="6" y2="18" />
					<line x1="6" y1="6" x2="18" y2="18" />
				</svg>
			</button>
			<img src={lightboxImage} alt={lightboxFilename ?? 'Screenshot'} />
			{#if lightboxFilename}
				<div class="lightbox-filename">{lightboxFilename}</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.test-results-container {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	/* Loading state */
	.loading-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-3);
		padding: var(--space-8);
		color: var(--text-muted);
		font-size: var(--text-sm);
	}

	.spinner {
		width: 20px;
		height: 20px;
		border: 2px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	/* Error message */
	.error-message {
		padding: var(--space-3);
		background: var(--status-danger-bg);
		border: 1px solid var(--status-danger);
		border-radius: var(--radius-md);
		color: var(--status-danger);
		font-size: var(--text-sm);
	}

	/* Empty state */
	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-8);
		text-align: center;
		color: var(--text-muted);
	}

	.empty-state svg {
		opacity: 0.5;
	}

	.empty-state h3 {
		margin: 0;
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-secondary);
	}

	.empty-state p {
		margin: 0;
		font-size: var(--text-sm);
	}

	.empty-state .hint {
		font-size: var(--text-xs);
		opacity: 0.7;
	}

	.empty-state code {
		background: var(--bg-tertiary);
		padding: var(--space-1) var(--space-2);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
		font-size: var(--text-xs);
	}

	/* Tabs */
	.tabs {
		display: flex;
		gap: var(--space-1);
		border-bottom: 1px solid var(--border-default);
		padding-bottom: var(--space-2);
	}

	.tab {
		padding: var(--space-2) var(--space-3);
		background: none;
		border: none;
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--text-muted);
		cursor: pointer;
		transition: all 0.2s;
	}

	.tab:hover {
		color: var(--text-secondary);
		background: var(--bg-secondary);
	}

	.tab.active {
		color: var(--accent-primary);
		background: var(--accent-subtle);
		font-weight: var(--font-medium);
	}

	/* Summary view */
	.summary-view {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.summary-cards {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: var(--space-3);
	}

	.summary-card {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		padding: var(--space-3);
		text-align: center;
	}

	.summary-value {
		font-size: var(--text-2xl);
		font-weight: var(--font-bold);
		font-variant-numeric: tabular-nums;
	}

	.summary-value.passed {
		color: var(--status-success);
	}

	.summary-value.failed {
		color: var(--status-danger);
	}

	.summary-value.skipped {
		color: var(--text-muted);
	}

	.summary-label {
		font-size: var(--text-xs);
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
	}

	/* Pass rate bar */
	.pass-rate-container {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		padding: var(--space-3);
	}

	.pass-rate-header {
		display: flex;
		justify-content: space-between;
		margin-bottom: var(--space-2);
	}

	.pass-rate-label {
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}

	.pass-rate-value {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		font-variant-numeric: tabular-nums;
	}

	.pass-rate-bar {
		height: 8px;
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		overflow: hidden;
	}

	.pass-rate-fill {
		height: 100%;
		border-radius: var(--radius-full);
		transition: width 0.3s ease;
	}

	.pass-rate-fill.success {
		background: var(--status-success);
	}

	.pass-rate-fill.warning {
		background: var(--status-warning);
	}

	.pass-rate-fill.danger {
		background: var(--status-danger);
	}

	/* Metadata */
	.metadata {
		display: flex;
		gap: var(--space-4);
		flex-wrap: wrap;
	}

	.metadata-item {
		display: flex;
		gap: var(--space-2);
		font-size: var(--text-sm);
	}

	.metadata-label {
		color: var(--text-muted);
	}

	.metadata-value {
		color: var(--text-secondary);
		font-weight: var(--font-medium);
	}

	/* Coverage */
	.coverage-section {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		padding: var(--space-3);
	}

	.coverage-section h4 {
		margin: 0 0 var(--space-3) 0;
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-secondary);
	}

	.coverage-bar-container {
		margin-bottom: var(--space-3);
	}

	.coverage-header {
		display: flex;
		justify-content: space-between;
		font-size: var(--text-sm);
		margin-bottom: var(--space-1);
	}

	.coverage-bar {
		height: 6px;
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		overflow: hidden;
	}

	.coverage-fill {
		height: 100%;
		background: var(--accent-primary);
		border-radius: var(--radius-full);
	}

	.coverage-detail {
		display: flex;
		justify-content: space-between;
		font-size: var(--text-xs);
		padding: var(--space-1) 0;
		border-top: 1px solid var(--border-subtle);
	}

	.coverage-detail:first-of-type {
		border-top: none;
	}

	.coverage-detail-label {
		color: var(--text-muted);
	}

	.coverage-detail-value {
		font-variant-numeric: tabular-nums;
	}

	/* No report */
	.no-report {
		padding: var(--space-4);
		background: var(--bg-secondary);
		border-radius: var(--radius-md);
		color: var(--text-muted);
		font-size: var(--text-sm);
		text-align: center;
	}

	/* Quick links */
	.quick-links {
		display: flex;
		gap: var(--space-3);
		flex-wrap: wrap;
	}

	.quick-link {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--accent-primary);
		font-size: var(--text-sm);
		text-decoration: none;
		transition: all 0.2s;
	}

	.quick-link:hover {
		background: var(--accent-subtle);
		border-color: var(--accent-primary);
	}

	/* Screenshots view */
	.screenshots-view {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.screenshots-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
		gap: var(--space-3);
	}

	.screenshot-card {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		overflow: hidden;
	}

	.screenshot-preview {
		display: block;
		width: 100%;
		aspect-ratio: 16/9;
		padding: 0;
		border: none;
		background: var(--bg-tertiary);
		cursor: pointer;
	}

	.screenshot-preview img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.screenshot-info {
		padding: var(--space-2);
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.screenshot-name {
		font-size: var(--text-sm);
		color: var(--text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.screenshot-meta {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	/* Suites view */
	.suites-view {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.suite-card {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		overflow: hidden;
	}

	.suite-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border-bottom: 1px solid var(--border-subtle);
	}

	.suite-name {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.suite-count {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.suite-tests {
		padding: var(--space-2);
	}

	.test-item {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2);
		border-radius: var(--radius-sm);
	}

	.test-item.passed .test-status-icon {
		color: var(--status-success);
	}

	.test-item.failed .test-status-icon {
		color: var(--status-danger);
	}

	.test-item.skipped .test-status-icon {
		color: var(--text-muted);
	}

	.test-status-icon {
		flex-shrink: 0;
	}

	.test-name {
		flex: 1;
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}

	.test-duration {
		font-size: var(--text-xs);
		color: var(--text-muted);
		font-variant-numeric: tabular-nums;
	}

	.test-error {
		margin: 0 var(--space-2) var(--space-2) calc(var(--space-2) + 22px);
		padding: var(--space-2);
		background: var(--status-danger-bg);
		border-radius: var(--radius-sm);
		overflow-x: auto;
	}

	.test-error pre {
		margin: 0;
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--status-danger);
		white-space: pre-wrap;
		word-break: break-word;
	}

	/* Lightbox */
	.lightbox {
		position: fixed;
		inset: 0;
		z-index: 1000;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(0, 0, 0, 0.9);
		padding: var(--space-4);
	}

	.lightbox-content {
		position: relative;
		max-width: 90vw;
		max-height: 90vh;
		display: flex;
		flex-direction: column;
		align-items: center;
	}

	.lightbox-content img {
		max-width: 100%;
		max-height: calc(90vh - 60px);
		object-fit: contain;
		border-radius: var(--radius-md);
	}

	.lightbox-close {
		position: absolute;
		top: -40px;
		right: 0;
		padding: var(--space-2);
		background: none;
		border: none;
		color: white;
		cursor: pointer;
		opacity: 0.7;
	}

	.lightbox-close:hover {
		opacity: 1;
	}

	.lightbox-filename {
		margin-top: var(--space-3);
		color: white;
		font-size: var(--text-sm);
		opacity: 0.7;
	}

	/* Responsive */
	@media (max-width: 640px) {
		.summary-cards {
			grid-template-columns: repeat(2, 1fr);
		}

		.screenshots-grid {
			grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
		}
	}
</style>
