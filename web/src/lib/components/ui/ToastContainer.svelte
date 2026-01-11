<script lang="ts">
	import { toast, type Toast } from '$lib/stores/toast.svelte';
	import { onMount, onDestroy } from 'svelte';
	import { fly, fade } from 'svelte/transition';

	let toasts = $state<Toast[]>([]);

	onMount(() => {
		const unsubscribe = toast.subscribe((current) => {
			toasts = current;
		});
		return unsubscribe;
	});

	function getIcon(type: Toast['type']) {
		switch (type) {
			case 'success':
				return `<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>`;
			case 'error':
				return `<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>`;
			case 'warning':
				return `<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`;
			case 'info':
				return `<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>`;
		}
	}
</script>

<div class="toast-container" role="region" aria-label="Notifications">
	{#each toasts as t (t.id)}
		<div
			class="toast toast-{t.type}"
			role="alert"
			in:fly={{ x: 50, duration: 200 }}
			out:fade={{ duration: 150 }}
		>
			<div class="toast-icon">
				{@html getIcon(t.type)}
			</div>
			<div class="toast-content">
				{#if t.title}
					<div class="toast-title">{t.title}</div>
				{/if}
				<div class="toast-message">{t.message}</div>
			</div>
			{#if t.dismissible}
				<button
					class="toast-dismiss"
					onclick={() => toast.dismiss(t.id)}
					aria-label="Dismiss notification"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<line x1="18" y1="6" x2="6" y2="18"/>
						<line x1="6" y1="6" x2="18" y2="18"/>
					</svg>
				</button>
			{/if}
		</div>
	{/each}
</div>

<style>
	.toast-container {
		position: fixed;
		top: var(--space-4);
		right: var(--space-4);
		z-index: 9999;
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		max-width: 380px;
		pointer-events: none;
	}

	.toast {
		display: flex;
		align-items: flex-start;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-4);
		background: var(--bg-elevated);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-lg);
		pointer-events: auto;
	}

	.toast-icon {
		flex-shrink: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		margin-top: 1px;
	}

	.toast-success .toast-icon {
		color: var(--status-success);
	}

	.toast-error .toast-icon {
		color: var(--status-danger);
	}

	.toast-warning .toast-icon {
		color: var(--status-warning);
	}

	.toast-info .toast-icon {
		color: var(--status-info);
	}

	.toast-content {
		flex: 1;
		min-width: 0;
	}

	.toast-title {
		font-weight: var(--font-semibold);
		font-size: var(--text-sm);
		color: var(--text-primary);
		margin-bottom: var(--space-0-5);
	}

	.toast-message {
		font-size: var(--text-sm);
		color: var(--text-secondary);
		line-height: 1.4;
		word-break: break-word;
	}

	.toast-dismiss {
		flex-shrink: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		margin: -2px -4px -2px 0;
		padding: 0;
		background: transparent;
		border: none;
		border-radius: var(--radius-sm);
		color: var(--text-muted);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.toast-dismiss:hover {
		background: var(--bg-tertiary);
		color: var(--text-secondary);
	}

	/* Type-specific left border accent */
	.toast::before {
		content: '';
		position: absolute;
		left: 0;
		top: 0;
		bottom: 0;
		width: 3px;
		border-radius: var(--radius-lg) 0 0 var(--radius-lg);
	}

	.toast {
		position: relative;
		overflow: hidden;
	}

	.toast-success::before {
		background: var(--status-success);
	}

	.toast-error::before {
		background: var(--status-danger);
	}

	.toast-warning::before {
		background: var(--status-warning);
	}

	.toast-info::before {
		background: var(--status-info);
	}
</style>
