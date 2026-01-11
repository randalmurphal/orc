<script lang="ts">
	interface Props {
		title: string;
		message: string;
		confirmLabel: string;
		confirmVariant?: 'primary' | 'warning' | 'danger';
		action?: 'run' | 'pause' | 'resume' | 'delete';
		loading?: boolean;
		onConfirm: () => void;
		onCancel: () => void;
	}

	let {
		title,
		message,
		confirmLabel,
		confirmVariant = 'primary',
		action,
		loading = false,
		onConfirm,
		onCancel
	}: Props = $props();

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			onCancel();
		}
		if (e.key === 'Enter' && !loading) {
			onConfirm();
		}
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) {
			onCancel();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	class="modal-backdrop"
	onclick={handleBackdropClick}
	onkeydown={handleKeydown}
	role="dialog"
	aria-modal="true"
	aria-labelledby="confirm-title"
	tabindex="-1"
>
	<div class="modal">
		<div class="modal-icon {confirmVariant}">
			{#if action === 'run'}
				<!-- Play icon for run -->
				<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<polygon points="5 3 19 12 5 21 5 3" />
				</svg>
			{:else if action === 'pause'}
				<!-- Pause icon -->
				<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<rect x="6" y="4" width="4" height="16" rx="1" />
					<rect x="14" y="4" width="4" height="16" rx="1" />
				</svg>
			{:else if action === 'resume'}
				<!-- Play/resume icon -->
				<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<polygon points="5 3 19 12 5 21 5 3" />
				</svg>
			{:else if action === 'delete' || confirmVariant === 'danger'}
				<!-- Warning triangle for danger/delete -->
				<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
					<line x1="12" y1="9" x2="12" y2="13" />
					<line x1="12" y1="17" x2="12.01" y2="17" />
				</svg>
			{:else}
				<!-- Question/confirm icon for generic confirmations -->
				<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<circle cx="12" cy="12" r="10" />
					<path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
					<line x1="12" y1="17" x2="12.01" y2="17" />
				</svg>
			{/if}
		</div>

		<h2 id="confirm-title">{title}</h2>
		<p>{message}</p>

		<div class="actions">
			<button class="cancel-btn" onclick={onCancel} disabled={loading}>
				Cancel
			</button>
			<button class="confirm-btn {confirmVariant}" onclick={onConfirm} disabled={loading}>
				{#if loading}
					<span class="spinner"></span>
					Processing...
				{:else}
					{confirmLabel}
				{/if}
			</button>
		</div>

		<div class="keyboard-hint">
			<kbd>Enter</kbd> to confirm
			<span class="separator">|</span>
			<kbd>Esc</kbd> to cancel
		</div>
	</div>
</div>

<style>
	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.7);
		backdrop-filter: blur(4px);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: var(--z-modal);
		animation: fade-in var(--duration-normal) var(--ease-out);
	}

	@keyframes fade-in {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	.modal {
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-xl);
		padding: var(--space-6);
		max-width: 400px;
		width: 90%;
		box-shadow: var(--shadow-2xl);
		text-align: center;
		animation: modal-in var(--duration-normal) var(--ease-out);
	}

	@keyframes modal-in {
		from {
			opacity: 0;
			transform: scale(0.95) translateY(-10px);
		}
		to {
			opacity: 1;
			transform: scale(1) translateY(0);
		}
	}

	.modal-icon {
		width: 48px;
		height: 48px;
		border-radius: var(--radius-full);
		display: flex;
		align-items: center;
		justify-content: center;
		margin: 0 auto var(--space-4);
	}

	.modal-icon.primary {
		background: var(--accent-subtle);
		color: var(--accent-primary);
	}

	.modal-icon.warning {
		background: var(--status-warning-bg);
		color: var(--status-warning);
	}

	.modal-icon.danger {
		background: var(--status-danger-bg);
		color: var(--status-danger);
	}

	.modal h2 {
		margin: 0 0 var(--space-2);
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		text-transform: none;
		letter-spacing: normal;
	}

	.modal p {
		margin: 0 0 var(--space-6);
		color: var(--text-secondary);
		font-size: var(--text-sm);
		line-height: var(--leading-relaxed);
	}

	.actions {
		display: flex;
		gap: var(--space-3);
		justify-content: center;
	}

	.cancel-btn,
	.confirm-btn {
		padding: var(--space-2-5) var(--space-5);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		border-radius: var(--radius-md);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
		min-width: 100px;
	}

	.cancel-btn {
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		color: var(--text-primary);
	}

	.cancel-btn:hover:not(:disabled) {
		background: var(--bg-surface);
		border-color: var(--border-strong);
	}

	.confirm-btn {
		border: none;
		color: white;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
	}

	.confirm-btn.primary {
		background: var(--accent-primary);
	}

	.confirm-btn.primary:hover:not(:disabled) {
		background: var(--accent-hover);
	}

	.confirm-btn.warning {
		background: var(--status-warning);
	}

	.confirm-btn.warning:hover:not(:disabled) {
		background: #d97706;
	}

	.confirm-btn.danger {
		background: var(--status-danger);
	}

	.confirm-btn.danger:hover:not(:disabled) {
		background: #dc2626;
	}

	.cancel-btn:disabled,
	.confirm-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.spinner {
		width: 14px;
		height: 14px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top-color: white;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.keyboard-hint {
		margin-top: var(--space-5);
		padding-top: var(--space-4);
		border-top: 1px solid var(--border-subtle);
		font-size: var(--text-xs);
		color: var(--text-muted);
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
	}

	.keyboard-hint kbd {
		font-family: var(--font-mono);
		font-size: var(--text-2xs);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		padding: var(--space-0-5) var(--space-1);
		box-shadow: 0 1px 0 var(--border-default);
	}

	.separator {
		color: var(--border-default);
	}
</style>
