/**
 * BlockedPanel component for right panel showing blocked tasks
 *
 * Displays tasks that are blocked by dependencies with:
 * - Orange-themed section header with blocked icon and count
 * - Task ID (monospace), title (truncated), blocking reason
 * - Action buttons: Skip (bypass block), Force (run despite block with confirmation)
 *
 * Reference: example_ui/board.html (.blocked-item class)
 */

import { useState, useCallback } from 'react';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/overlays/Modal';
import type { Task } from '@/gen/orc/v1/task_pb';
import './BlockedPanel.css';

export interface BlockedPanelProps {
	/** Blocked tasks to display */
	tasks: Task[];
	/** Callback when Skip button is clicked - bypasses the block */
	onSkip: (taskId: string) => void;
	/** Callback when Force button is clicked - runs despite block */
	onForce: (taskId: string) => void;
}

/**
 * Formats blocking reasons with code formatting for task IDs.
 * Wraps task IDs (TASK-XXX, INIT-XXX) in <code> tags.
 */
function formatBlockingReason(blockers: string[]): React.ReactNode {
	if (blockers.length === 0) {
		return 'Unknown blocker';
	}

	if (blockers.length === 1) {
		const blocker = blockers[0];
		// Check if it looks like a task/initiative ID
		if (/^(TASK|INIT)-\d+$/i.test(blocker)) {
			return (
				<>
					Waiting for <code>{blocker}</code>
				</>
			);
		}
		return blocker;
	}

	// Multiple blockers - format as a list
	return (
		<ul className="blocked-reason-list">
			{blockers.map((blocker) => (
				<li key={blocker}>
					{/^(TASK|INIT)-\d+$/i.test(blocker) ? <code>{blocker}</code> : blocker}
				</li>
			))}
		</ul>
	);
}

/**
 * BlockedPanel displays tasks blocked by dependencies with actions to skip or force.
 */
export function BlockedPanel({ tasks, onSkip, onForce }: BlockedPanelProps) {
	const [collapsed, setCollapsed] = useState(false);
	const [confirmingForce, setConfirmingForce] = useState<string | null>(null);

	const handleToggle = useCallback(() => {
		setCollapsed((prev) => !prev);
	}, []);

	const handleSkip = useCallback(
		(taskId: string) => {
			onSkip(taskId);
		},
		[onSkip]
	);

	const handleForceClick = useCallback((taskId: string) => {
		setConfirmingForce(taskId);
	}, []);

	const handleForceConfirm = useCallback(() => {
		if (confirmingForce) {
			onForce(confirmingForce);
			setConfirmingForce(null);
		}
	}, [confirmingForce, onForce]);

	const handleForceCancel = useCallback(() => {
		setConfirmingForce(null);
	}, []);

	const taskCount = tasks.length;
	const confirmingTask = confirmingForce
		? tasks.find((t) => t.id === confirmingForce)
		: null;

	return (
		<>
			<div className={`blocked-panel panel-section ${collapsed ? 'collapsed' : ''}`}>
				<button
					className="panel-header"
					onClick={handleToggle}
					aria-expanded={!collapsed}
					aria-controls="blocked-panel-body"
				>
					<div className="panel-title">
						<div className="panel-title-icon orange">
							<Icon name="alert-circle" size={12} />
						</div>
						<span>Blocked</span>
					</div>
					<span className="panel-badge orange" aria-label={`${taskCount} blocked tasks`}>
						{taskCount}
					</span>
					<Icon
						name={collapsed ? 'chevron-right' : 'chevron-down'}
						size={12}
						className="panel-chevron"
					/>
				</button>

				<div id="blocked-panel-body" className="panel-body" role="region">
					{tasks.map((task) => (
						<div key={task.id} className="blocked-item">
							<div className="blocked-header">
								<div className="blocked-icon">
									<Icon name="alert-circle" size={10} />
								</div>
								<div className="blocked-content">
									<div className="blocked-id">{task.id}</div>
									<div className="blocked-title" title={task.title}>
										{task.title}
									</div>
								</div>
							</div>

							<div className="blocked-reason">
								{formatBlockingReason(task.unmetBlockers || task.blockedBy || [])}
							</div>

							<div className="blocked-actions">
								<Button
									variant="ghost"
									size="sm"
									onClick={() => handleSkip(task.id)}
									aria-label={`Skip block for ${task.id}`}
								>
									Skip
								</Button>
								<Button
									variant="ghost"
									size="sm"
									onClick={() => handleForceClick(task.id)}
									aria-label={`Force run ${task.id}`}
								>
									Force
								</Button>
							</div>
						</div>
					))}
				</div>
			</div>

			{/* Force confirmation modal */}
			<Modal
				open={confirmingForce !== null}
				onClose={handleForceCancel}
				size="sm"
				title="Force Run Task?"
			>
				<div className="blocked-force-confirm">
					<p className="blocked-force-message">
						Are you sure you want to force run{' '}
						<code>{confirmingTask?.id}</code>?
					</p>
					<p className="blocked-force-warning">
						This task is blocked by incomplete dependencies. Running it now may
						cause errors or unexpected behavior.
					</p>
					<div className="blocked-force-actions">
						<Button variant="secondary" onClick={handleForceCancel}>
							Cancel
						</Button>
						<Button variant="danger" onClick={handleForceConfirm}>
							Force Run
						</Button>
					</div>
				</div>
			</Modal>
		</>
	);
}
