<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		open: boolean;
		onClose: () => void;
		size?: 'sm' | 'md' | 'lg' | 'xl';
		title?: string;
		showClose?: boolean;
		children: Snippet;
	}

	let { open, onClose, size = 'md', title, showClose = true, children }: Props = $props();

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) {
			onClose();
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			onClose();
		}
	}

	const sizeClasses: Record<string, string> = {
		sm: 'max-width-sm',
		md: 'max-width-md',
		lg: 'max-width-lg',
		xl: 'max-width-xl'
	};
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="modal-backdrop"
		role="dialog"
		aria-modal="true"
		aria-labelledby={title ? 'modal-title' : undefined}
		tabindex="-1"
		onclick={handleBackdropClick}
		onkeydown={handleKeydown}
	>
		<div class="modal-content {sizeClasses[size]}">
			{#if title || showClose}
				<div class="modal-header">
					{#if title}
						<h2 id="modal-title" class="modal-title">{title}</h2>
					{/if}
					{#if showClose}
						<button
							class="modal-close"
							onclick={onClose}
							aria-label="Close modal"
							title="Close (Esc)"
						>
							<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
								<line x1="18" y1="6" x2="6" y2="18" />
								<line x1="6" y1="6" x2="18" y2="18" />
							</svg>
						</button>
					{/if}
				</div>
			{/if}
			<div class="modal-body">
				{@render children()}
			</div>
		</div>
	</div>
{/if}

<style>
	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.7);
		backdrop-filter: blur(4px);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding: var(--space-16) var(--space-4);
		z-index: 1000;
		animation: fade-in var(--duration-normal) var(--ease-out);
	}

	.modal-content {
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-xl);
		box-shadow: var(--shadow-2xl);
		width: 100%;
		max-height: calc(100vh - var(--space-32));
		overflow: hidden;
		display: flex;
		flex-direction: column;
		animation: modal-content-in var(--duration-normal) var(--ease-out);
	}

	/* Size variants */
	.max-width-sm {
		max-width: 400px;
	}

	.max-width-md {
		max-width: 560px;
	}

	.max-width-lg {
		max-width: 720px;
	}

	.max-width-xl {
		max-width: 900px;
	}

	/* Header */
	.modal-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-4) var(--space-5);
		border-bottom: 1px solid var(--border-subtle);
		flex-shrink: 0;
	}

	.modal-title {
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.modal-close {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-muted);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.modal-close:hover {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	.modal-close:focus-visible {
		outline: none;
		box-shadow: 0 0 0 2px var(--accent-glow);
	}

	/* Body */
	.modal-body {
		flex: 1;
		overflow-y: auto;
	}
</style>
