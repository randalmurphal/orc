<script lang="ts">
	import { goto } from '$app/navigation';
	import StatusIndicator from '$lib/components/ui/StatusIndicator.svelte';
	import type { Task, TaskPriority, TaskQueue } from '$lib/types';
	import { PRIORITY_CONFIG } from '$lib/types';
	import { updateTask } from '$lib/api';
	import { updateTask as updateTaskInStore } from '$lib/stores/tasks';
	import { getInitiativeBadgeTitle } from '$lib/stores/initiatives';

	interface Props {
		task: Task;
		onAction: (taskId: string, action: 'run' | 'pause' | 'resume') => Promise<void>;
		onTaskClick?: (task: Task) => void;
		onInitiativeClick?: (initiativeId: string) => void;
	}

	let { task, onAction, onTaskClick, onInitiativeClick }: Props = $props();

	let actionLoading = $state(false);
	let isDragging = $state(false);
	let showQuickMenu = $state(false);
	let quickMenuLoading = $state(false);

	// Get priority config with fallback to normal
	const priority = $derived((task.priority || 'normal') as TaskPriority);
	const priorityConfig = $derived(PRIORITY_CONFIG[priority]);
	const showPriority = $derived(priority !== 'normal'); // Only show non-normal priorities
	const queue = $derived((task.queue || 'active') as TaskQueue);
	const isBacklog = $derived(queue === 'backlog');

	function handleDragStart(e: DragEvent) {
		if (e.dataTransfer) {
			e.dataTransfer.setData('application/json', JSON.stringify(task));
			e.dataTransfer.effectAllowed = 'move';
		}
		isDragging = true;
	}

	function handleDragEnd() {
		isDragging = false;
	}

	/**
	 * Button actions execute immediately without confirmation.
	 * This is intentional - drag-drop requires confirmation because the action
	 * is less explicit (you're moving to a column), but button clicks are
	 * direct and clear about what action will occur.
	 */
	async function handleAction(action: 'run' | 'pause' | 'resume', e: MouseEvent) {
		e.stopPropagation();
		e.preventDefault();
		actionLoading = true;
		try {
			await onAction(task.id, action);
		} finally {
			actionLoading = false;
		}
	}

	function openTask(e: MouseEvent) {
		// Don't navigate if clicking on action buttons or quick menu
		const target = e.target as HTMLElement;
		if (target.closest('.actions') || target.closest('.quick-menu')) {
			return;
		}
		// For running tasks, show transcript modal if callback provided
		if (task.status === 'running' && onTaskClick) {
			onTaskClick(task);
			return;
		}
		goto(`/tasks/${task.id}`);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			goto(`/tasks/${task.id}`);
		}
		if (e.key === 'Escape' && showQuickMenu) {
			showQuickMenu = false;
		}
	}

	function toggleQuickMenu(e: MouseEvent) {
		e.stopPropagation();
		e.preventDefault();
		showQuickMenu = !showQuickMenu;
	}

	function closeQuickMenu() {
		showQuickMenu = false;
	}

	async function setQueue(newQueue: TaskQueue) {
		if (newQueue === queue) {
			showQuickMenu = false;
			return;
		}
		quickMenuLoading = true;
		try {
			const updated = await updateTask(task.id, { queue: newQueue });
			updateTaskInStore(task.id, updated);
		} catch (e) {
			console.error('Failed to update queue:', e);
		} finally {
			quickMenuLoading = false;
			showQuickMenu = false;
		}
	}

	async function setPriority(newPriority: TaskPriority) {
		if (newPriority === priority) {
			showQuickMenu = false;
			return;
		}
		quickMenuLoading = true;
		try {
			const updated = await updateTask(task.id, { priority: newPriority });
			updateTaskInStore(task.id, updated);
		} catch (e) {
			console.error('Failed to update priority:', e);
		} finally {
			quickMenuLoading = false;
			showQuickMenu = false;
		}
	}

	const weightConfig: Record<string, { color: string; bg: string }> = {
		trivial: { color: 'var(--weight-trivial)', bg: 'rgba(107, 114, 128, 0.15)' },
		small: { color: 'var(--weight-small)', bg: 'var(--status-success-bg)' },
		medium: { color: 'var(--weight-medium)', bg: 'var(--status-info-bg)' },
		large: { color: 'var(--weight-large)', bg: 'var(--status-warning-bg)' },
		greenfield: { color: 'var(--weight-greenfield)', bg: 'var(--accent-subtle)' }
	};

	const weight = $derived(weightConfig[task.weight] || weightConfig.small);
	const isRunning = $derived(task.status === 'running');
	const initiativeBadge = $derived(task.initiative_id ? getInitiativeBadgeTitle(task.initiative_id) : null);

	function handleInitiativeClick(e: MouseEvent) {
		e.stopPropagation();
		e.preventDefault();
		if (task.initiative_id && onInitiativeClick) {
			onInitiativeClick(task.initiative_id);
		}
	}

	function formatDate(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		const diffHours = Math.floor(diffMins / 60);
		const diffDays = Math.floor(diffHours / 24);

		if (diffMins < 1) return 'just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		if (diffHours < 24) return `${diffHours}h ago`;
		if (diffDays < 7) return `${diffDays}d ago`;
		return date.toLocaleDateString();
	}
</script>

<div
	class="task-card"
	class:dragging={isDragging}
	class:running={isRunning}
	draggable="true"
	ondragstart={handleDragStart}
	ondragend={handleDragEnd}
	onclick={openTask}
	onkeydown={handleKeydown}
	role="button"
	tabindex="0"
>
	<div class="card-header">
		<div class="header-left">
			<span class="task-id">{task.id}</span>
			{#if showPriority}
				<span
					class="priority-badge"
					class:critical={priority === 'critical'}
					class:high={priority === 'high'}
					class:low={priority === 'low'}
					style:color={priorityConfig.color}
					title="{priorityConfig.label} priority"
				>
					{#if priority === 'critical'}
						<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
							<circle cx="12" cy="12" r="10" />
							<line x1="12" y1="8" x2="12" y2="12" />
							<line x1="12" y1="16" x2="12.01" y2="16" />
						</svg>
					{:else if priority === 'high'}
						<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
							<polyline points="18 15 12 9 6 15" />
						</svg>
					{:else if priority === 'low'}
						<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
							<polyline points="6 9 12 15 18 9" />
						</svg>
					{/if}
				</span>
			{/if}
		</div>
		<StatusIndicator status={task.status} size="sm" />
	</div>

	<h3 class="task-title">{task.title}</h3>

	{#if task.description}
		<p class="task-description">{task.description}</p>
	{/if}

	{#if task.current_phase}
		<div class="task-phase">
			<span class="phase-label">Phase:</span>
			<span class="phase-value">{task.current_phase}</span>
		</div>
	{/if}

	<div class="card-footer">
		<div class="footer-left">
			<span
				class="weight-badge"
				style:color={weight.color}
				style:background={weight.bg}
			>
				{task.weight}
			</span>
			{#if task.is_blocked}
				<span
					class="blocked-badge"
					title="Blocked by {task.unmet_blockers?.join(', ')}"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
						<circle cx="12" cy="12" r="10" />
						<line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
					</svg>
					Blocked
				</span>
			{/if}
			{#if initiativeBadge}
				<button
					class="initiative-badge"
					onclick={handleInitiativeClick}
					title={initiativeBadge.full}
					type="button"
				>
					{initiativeBadge.display}
				</button>
			{/if}
			<span class="updated-time">{formatDate(task.updated_at)}</span>
		</div>

		<div class="actions">
			{#if task.status === 'created' || task.status === 'planned'}
				<button
					class="action-btn run"
					onclick={(e) => handleAction('run', e)}
					disabled={actionLoading}
					title="Run task"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="none">
						<polygon points="5 3 19 12 5 21 5 3" />
					</svg>
				</button>
			{:else if task.status === 'running'}
				<button
					class="action-btn pause"
					onclick={(e) => handleAction('pause', e)}
					disabled={actionLoading}
					title="Pause task"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="none">
						<rect x="6" y="4" width="4" height="16" rx="1" />
						<rect x="14" y="4" width="4" height="16" rx="1" />
					</svg>
				</button>
			{:else if task.status === 'paused'}
				<button
					class="action-btn resume"
					onclick={(e) => handleAction('resume', e)}
					disabled={actionLoading}
					title="Resume task"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="none">
						<polygon points="5 3 19 12 5 21 5 3" />
					</svg>
				</button>
			{/if}

			<!-- Quick menu for queue/priority -->
			<div class="quick-menu">
				<button
					class="action-btn more"
					onclick={toggleQuickMenu}
					title="Quick actions"
					aria-expanded={showQuickMenu}
					aria-haspopup="true"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="none">
						<circle cx="12" cy="5" r="2" />
						<circle cx="12" cy="12" r="2" />
						<circle cx="12" cy="19" r="2" />
					</svg>
				</button>

				{#if showQuickMenu}
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div class="quick-menu-backdrop" onclick={closeQuickMenu} onkeydown={(e) => e.key === 'Escape' && closeQuickMenu()}></div>
					<div class="quick-menu-dropdown" role="menu">
						{#if quickMenuLoading}
							<div class="menu-loading">
								<div class="spinner"></div>
							</div>
						{:else}
							<!-- Queue section -->
							<div class="menu-section">
								<div class="menu-label">Queue</div>
								<button
									class="menu-item"
									class:selected={queue === 'active'}
									onclick={() => setQueue('active')}
									role="menuitem"
								>
									<span class="menu-icon active-icon"></span>
									Active
								</button>
								<button
									class="menu-item"
									class:selected={queue === 'backlog'}
									onclick={() => setQueue('backlog')}
									role="menuitem"
								>
									<span class="menu-icon backlog-icon"></span>
									Backlog
								</button>
							</div>

							<div class="menu-divider"></div>

							<!-- Priority section -->
							<div class="menu-section">
								<div class="menu-label">Priority</div>
								<button
									class="menu-item"
									class:selected={priority === 'critical'}
									onclick={() => setPriority('critical')}
									role="menuitem"
								>
									<span class="menu-icon priority-icon" style:background="var(--status-error)"></span>
									Critical
								</button>
								<button
									class="menu-item"
									class:selected={priority === 'high'}
									onclick={() => setPriority('high')}
									role="menuitem"
								>
									<span class="menu-icon priority-icon" style:background="var(--status-warning)"></span>
									High
								</button>
								<button
									class="menu-item"
									class:selected={priority === 'normal'}
									onclick={() => setPriority('normal')}
									role="menuitem"
								>
									<span class="menu-icon priority-icon" style:background="var(--text-muted)"></span>
									Normal
								</button>
								<button
									class="menu-item"
									class:selected={priority === 'low'}
									onclick={() => setPriority('low')}
									role="menuitem"
								>
									<span class="menu-icon priority-icon" style:background="var(--text-disabled)"></span>
									Low
								</button>
							</div>
						{/if}
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>

<style>
	.task-card {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		padding: var(--space-3);
		cursor: grab;
		transition:
			border-color var(--duration-fast) var(--ease-out),
			box-shadow var(--duration-fast) var(--ease-out),
			transform var(--duration-fast) var(--ease-out),
			opacity var(--duration-fast) var(--ease-out);
	}

	.task-card:hover {
		border-color: var(--border-strong);
		box-shadow: var(--shadow-md);
	}

	.task-card:focus-visible {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.task-card:active {
		cursor: grabbing;
	}

	.task-card.dragging {
		opacity: 0.5;
		transform: rotate(2deg) scale(1.02);
		box-shadow: var(--shadow-lg);
	}

	.task-card.running {
		border-color: var(--accent-primary);
		border-width: 2px;
		background: linear-gradient(
			135deg,
			var(--bg-secondary) 0%,
			color-mix(in srgb, var(--accent-primary) 5%, var(--bg-secondary)) 100%
		);
		animation: card-pulse 2s ease-in-out infinite;
	}

	@keyframes card-pulse {
		0%,
		100% {
			box-shadow:
				0 0 0 0 var(--accent-glow),
				0 2px 8px rgba(139, 92, 246, 0.15);
		}
		50% {
			box-shadow:
				0 0 0 4px var(--accent-glow),
				0 4px 16px rgba(139, 92, 246, 0.25);
		}
	}

	.card-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: var(--space-2);
	}

	.card-header .header-left {
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
	}

	.task-id {
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-muted);
		letter-spacing: var(--tracking-wide);
	}

	.priority-badge {
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.priority-badge.critical {
		animation: priority-pulse 1.5s ease-in-out infinite;
	}

	@keyframes priority-pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.6; }
	}

	.task-title {
		margin: 0 0 var(--space-1);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
		line-height: var(--leading-snug);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		text-transform: none;
		letter-spacing: normal;
	}

	.task-description {
		margin: 0 0 var(--space-2);
		font-size: var(--text-xs);
		color: var(--text-secondary);
		line-height: var(--leading-relaxed);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		white-space: pre-wrap;
	}

	.task-phase {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		margin-bottom: var(--space-3);
		font-size: var(--text-xs);
	}

	.phase-label {
		color: var(--text-muted);
	}

	.phase-value {
		color: var(--accent-primary);
		font-weight: var(--font-medium);
	}

	.card-footer {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.footer-left {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.weight-badge {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		padding: var(--space-0-5) var(--space-1-5);
		border-radius: var(--radius-sm);
	}

	.blocked-badge {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		padding: var(--space-0-5) var(--space-1-5);
		border-radius: var(--radius-sm);
		background: var(--status-danger-bg);
		color: var(--status-danger);
	}

	.initiative-badge {
		font-size: var(--text-2xs);
		font-weight: var(--font-medium);
		letter-spacing: var(--tracking-wide);
		padding: var(--space-0-5) var(--space-1-5);
		border-radius: var(--radius-sm);
		background: var(--bg-tertiary);
		color: var(--text-secondary);
		border: 1px solid var(--border-subtle);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.initiative-badge:hover {
		background: var(--bg-surface);
		border-color: var(--border-default);
		color: var(--text-primary);
	}

	.updated-time {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.actions {
		display: flex;
		gap: var(--space-1);
	}

	.action-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		padding: 0;
		border: none;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.action-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.action-btn.run {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.action-btn.run:hover:not(:disabled) {
		background: var(--status-success);
		color: white;
	}

	.action-btn.pause {
		background: var(--status-warning-bg);
		color: var(--status-warning);
	}

	.action-btn.pause:hover:not(:disabled) {
		background: var(--status-warning);
		color: white;
	}

	.action-btn.resume {
		background: var(--status-info-bg);
		color: var(--status-info);
	}

	.action-btn.resume:hover:not(:disabled) {
		background: var(--status-info);
		color: white;
	}

	.action-btn.more {
		background: var(--bg-tertiary);
		color: var(--text-muted);
	}

	.action-btn.more:hover {
		background: var(--bg-secondary);
		color: var(--text-primary);
		border: 1px solid var(--border-default);
	}

	/* Quick menu */
	.quick-menu {
		position: relative;
	}

	.quick-menu-backdrop {
		position: fixed;
		inset: 0;
		z-index: 100;
	}

	.quick-menu-dropdown {
		position: absolute;
		right: 0;
		top: 100%;
		margin-top: var(--space-1);
		min-width: 140px;
		background: var(--bg-primary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		box-shadow: var(--shadow-lg);
		z-index: 101;
		overflow: hidden;
	}

	.menu-section {
		padding: var(--space-1);
	}

	.menu-label {
		padding: var(--space-1) var(--space-2);
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--text-muted);
	}

	.menu-item {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		width: 100%;
		padding: var(--space-1-5) var(--space-2);
		background: transparent;
		border: none;
		border-radius: var(--radius-sm);
		font-size: var(--text-sm);
		color: var(--text-primary);
		cursor: pointer;
		text-align: left;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.menu-item:hover {
		background: var(--bg-tertiary);
	}

	.menu-item.selected {
		background: var(--accent-subtle);
		color: var(--accent-primary);
	}

	.menu-icon {
		width: 8px;
		height: 8px;
		border-radius: var(--radius-full);
		flex-shrink: 0;
	}

	.menu-icon.active-icon {
		background: var(--accent-primary);
	}

	.menu-icon.backlog-icon {
		background: var(--text-muted);
		border: 1px dashed var(--border-default);
	}

	/* .menu-icon.priority-icon - background color set inline via style attribute */

	.menu-divider {
		height: 1px;
		background: var(--border-subtle);
		margin: var(--space-1) 0;
	}

	.menu-loading {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: var(--space-4);
	}

	.menu-loading .spinner {
		width: 16px;
		height: 16px;
		border: 2px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}
</style>
