<script lang="ts">
	import { onMount } from 'svelte';
	import { createPR, getPR, mergePR, getPRChecks, type GetPRResponse, type GetChecksResponse } from '$lib/api';
	import type { PR, CheckRun, CheckSummary, TaskStatus } from '$lib/types';

	let {
		taskId,
		taskBranch,
		taskStatus
	}: {
		taskId: string;
		taskBranch: string;
		taskStatus: TaskStatus;
	} = $props();

	let pr = $state<PR | null>(null);
	let checks = $state<CheckRun[]>([]);
	let checkSummary = $state<CheckSummary | null>(null);
	let loading = $state(false);
	let creating = $state(false);
	let merging = $state(false);
	let error = $state<string | null>(null);

	// Only show for completed tasks with a branch
	const shouldShow = $derived(
		taskStatus === 'completed' && taskBranch && taskBranch.length > 0
	);

	onMount(() => {
		if (shouldShow) {
			loadPRData();
		}
	});

	// Reload when task status changes to completed
	$effect(() => {
		if (shouldShow && !pr && !loading) {
			loadPRData();
		}
	});

	async function loadPRData() {
		if (!taskBranch) return;

		loading = true;
		error = null;

		try {
			const response = await getPR(taskId);
			pr = response.pr;
			checks = response.checks || [];

			// Get check summary
			const checksResponse = await getPRChecks(taskId);
			checkSummary = checksResponse.summary;
		} catch (e) {
			// PR doesn't exist yet - that's fine
			if (e instanceof Error && e.message.includes('no PR found')) {
				pr = null;
			} else if (e instanceof Error && e.message.includes('not authenticated')) {
				error = 'GitHub CLI not authenticated';
			} else {
				// Silently ignore other errors - PR might not exist
				pr = null;
			}
		} finally {
			loading = false;
		}
	}

	async function handleCreatePR() {
		creating = true;
		error = null;

		try {
			const response = await createPR(taskId);
			pr = response.pr;
			// Reload to get full PR details with checks
			await loadPRData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create PR';
		} finally {
			creating = false;
		}
	}

	async function handleMergePR() {
		if (!pr || !confirm(`Merge PR #${pr.number}?`)) return;

		merging = true;
		error = null;

		try {
			await mergePR(taskId, { method: 'squash', delete_branch: true });
			// Reload to show merged state
			await loadPRData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to merge PR';
		} finally {
			merging = false;
		}
	}

	function getCheckStatusIcon(check: CheckRun): string {
		if (check.status !== 'completed') return 'pending';
		switch (check.conclusion) {
			case 'success': return 'success';
			case 'failure': case 'timed_out': return 'failure';
			case 'neutral': case 'skipped': case 'cancelled': return 'neutral';
			default: return 'pending';
		}
	}
</script>

{#if shouldShow}
	<div class="pr-actions">
		{#if loading}
			<div class="pr-loading">
				<div class="spinner"></div>
				<span>Checking PR status...</span>
			</div>
		{:else if error}
			<div class="pr-error">
				<span class="error-icon">!</span>
				<span>{error}</span>
				<button class="btn-retry" onclick={loadPRData}>Retry</button>
			</div>
		{:else if !pr}
			<!-- No PR exists yet -->
			<button class="btn-create-pr" onclick={handleCreatePR} disabled={creating}>
				{#if creating}
					<div class="spinner small"></div>
					Creating PR...
				{:else}
					<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<circle cx="18" cy="18" r="3"/>
						<circle cx="6" cy="6" r="3"/>
						<path d="M13 6h3a2 2 0 0 1 2 2v7"/>
						<line x1="6" y1="9" x2="6" y2="21"/>
					</svg>
					Create Pull Request
				{/if}
			</button>
		{:else}
			<!-- PR exists -->
			<div class="pr-info">
				<a href={pr.html_url} target="_blank" rel="noopener noreferrer" class="pr-link">
					<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<circle cx="18" cy="18" r="3"/>
						<circle cx="6" cy="6" r="3"/>
						<path d="M13 6h3a2 2 0 0 1 2 2v7"/>
						<line x1="6" y1="9" x2="6" y2="21"/>
					</svg>
					PR #{pr.number}
				</a>

				<span class="pr-state" class:open={pr.state === 'open'} class:merged={pr.state === 'merged'} class:closed={pr.state === 'closed'}>
					{pr.state}
				</span>

				{#if checkSummary && checkSummary.total > 0}
					<div class="checks-summary" class:all-passed={checkSummary.failed === 0 && checkSummary.pending === 0}>
						{#if checkSummary.pending > 0}
							<span class="check-pending">{checkSummary.pending} pending</span>
						{/if}
						{#if checkSummary.failed > 0}
							<span class="check-failed">{checkSummary.failed} failed</span>
						{/if}
						{#if checkSummary.passed > 0}
							<span class="check-passed">{checkSummary.passed} passed</span>
						{/if}
					</div>
				{/if}

				{#if pr.state === 'open'}
					{#if pr.mergeable}
						<button class="btn-merge" onclick={handleMergePR} disabled={merging}>
							{#if merging}
								<div class="spinner small"></div>
								Merging...
							{:else}
								<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<circle cx="18" cy="18" r="3"/>
									<circle cx="6" cy="6" r="3"/>
									<path d="M6 21V9a9 9 0 0 0 9 9"/>
								</svg>
								Merge
							{/if}
						</button>
					{:else}
						<span class="conflict-warning">
							<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
								<path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
								<line x1="12" y1="9" x2="12" y2="13"/>
								<line x1="12" y1="17" x2="12.01" y2="17"/>
							</svg>
							Conflicts
						</span>
					{/if}
				{/if}
			</div>
		{/if}
	</div>
{/if}

<style>
	.pr-actions {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.pr-loading {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	.pr-error {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-sm);
		color: var(--status-danger);
	}

	.error-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 16px;
		background: var(--status-danger);
		color: white;
		border-radius: 50%;
		font-size: 10px;
		font-weight: bold;
	}

	.btn-retry {
		padding: var(--space-1) var(--space-2);
		font-size: var(--text-xs);
		background: transparent;
		border: 1px solid currentColor;
		border-radius: var(--radius-sm);
		color: inherit;
		cursor: pointer;
	}

	.btn-create-pr {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		background: var(--accent-primary);
		border: none;
		border-radius: var(--radius-md);
		color: white;
		cursor: pointer;
		transition: background 0.15s ease;
	}

	.btn-create-pr:hover:not(:disabled) {
		background: var(--accent-primary-hover);
	}

	.btn-create-pr:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.pr-info {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}

	.pr-link {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--accent-primary);
		text-decoration: none;
	}

	.pr-link:hover {
		text-decoration: underline;
	}

	.pr-state {
		padding: var(--space-0-5) var(--space-2);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		text-transform: uppercase;
		border-radius: var(--radius-full);
	}

	.pr-state.open {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.pr-state.merged {
		background: var(--status-info-bg);
		color: var(--status-info);
	}

	.pr-state.closed {
		background: var(--status-danger-bg);
		color: var(--status-danger);
	}

	.checks-summary {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-xs);
	}

	.check-pending {
		color: var(--status-warning);
	}

	.check-failed {
		color: var(--status-danger);
	}

	.check-passed {
		color: var(--status-success);
	}

	.btn-merge {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-1-5) var(--space-3);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		background: var(--status-success);
		border: none;
		border-radius: var(--radius-md);
		color: white;
		cursor: pointer;
		transition: background 0.15s ease;
	}

	.btn-merge:hover:not(:disabled) {
		filter: brightness(1.1);
	}

	.btn-merge:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.conflict-warning {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-1) var(--space-2);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		background: var(--status-warning-bg);
		border-radius: var(--radius-md);
		color: var(--status-warning);
	}

	.spinner {
		width: 16px;
		height: 16px;
		border: 2px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	.spinner.small {
		width: 12px;
		height: 12px;
		border-width: 1.5px;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
