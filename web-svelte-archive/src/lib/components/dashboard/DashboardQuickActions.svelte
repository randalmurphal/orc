<script lang="ts">
	interface Props {
		onNewTask: () => void;
		onViewTasks: () => void;
	}

	let { onNewTask, onViewTasks }: Props = $props();
</script>

<section class="actions-section">
	<div class="quick-actions">
		<button class="action-btn primary" onclick={onNewTask}>
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<line x1="12" y1="5" x2="12" y2="19" />
				<line x1="5" y1="12" x2="19" y2="12" />
			</svg>
			New Task
		</button>
		<button class="action-btn" onclick={onViewTasks}>
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
				<line x1="9" y1="9" x2="15" y2="15" />
				<line x1="15" y1="9" x2="9" y2="15" />
			</svg>
			View All Tasks
		</button>
	</div>
</section>

<style>
	.quick-actions {
		display: flex;
		gap: var(--space-3);
	}

	.action-btn {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2-5) var(--space-4);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--text-secondary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.action-btn:hover {
		border-color: var(--accent-primary);
		color: var(--text-primary);
	}

	.action-btn.primary {
		background: var(--accent-primary);
		border-color: var(--accent-primary);
		color: var(--text-inverse);
	}

	.action-btn.primary:hover {
		background: var(--accent-secondary);
		border-color: var(--accent-secondary);
	}
</style>
