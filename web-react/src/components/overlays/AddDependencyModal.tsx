/**
 * AddDependencyModal - Search and add task dependencies
 *
 * Features:
 * - Search tasks by ID or title
 * - Filter out current task and already-selected dependencies
 * - Status indicators for each task
 * - Loading, error, and empty states
 */

import { useState, useEffect, useMemo } from 'react';
import { Modal } from './Modal';
import { Icon } from '@/components/ui/Icon';
import { listTasks, type PaginatedTasks } from '@/lib/api';
import type { Task } from '@/lib/types';
import './AddDependencyModal.css';

interface AddDependencyModalProps {
	open: boolean;
	onClose: () => void;
	onSelect: (taskId: string) => void;
	type: 'blocker' | 'related';
	currentTaskId: string;
	existingBlockers: string[];
	existingRelated: string[];
}

function getStatusIcon(status: string): { icon: string; className: string } {
	if (status === 'completed' || status === 'finished') {
		return { icon: '✓', className: 'status-completed' };
	}
	if (status === 'running') {
		return { icon: '●', className: 'status-running' };
	}
	return { icon: '○', className: 'status-pending' };
}

function getStatusLabel(status: string): string {
	switch (status) {
		case 'completed':
		case 'finished':
			return 'Completed';
		case 'running':
			return 'Running';
		case 'paused':
			return 'Paused';
		case 'blocked':
			return 'Blocked';
		case 'failed':
			return 'Failed';
		default:
			return 'Pending';
	}
}

export function AddDependencyModal({
	open,
	onClose,
	onSelect,
	type,
	currentTaskId,
	existingBlockers,
	existingRelated,
}: AddDependencyModalProps) {
	const [tasks, setTasks] = useState<Task[]>([]);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [searchQuery, setSearchQuery] = useState('');

	// Load tasks when modal opens
	useEffect(() => {
		if (open) {
			setSearchQuery('');
			loadTasks();
		}
	}, [open]);

	async function loadTasks() {
		setLoading(true);
		setError(null);

		try {
			const result = await listTasks();
			// Handle both array and paginated response
			setTasks(Array.isArray(result) ? result : (result as PaginatedTasks).tasks);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load tasks');
		} finally {
			setLoading(false);
		}
	}

	// Filter tasks to exclude current task and already selected
	const filteredTasks = useMemo(() => {
		const excludeIds = new Set([currentTaskId]);

		// Exclude already selected depending on type
		if (type === 'blocker') {
			existingBlockers.forEach((id) => excludeIds.add(id));
		} else {
			existingRelated.forEach((id) => excludeIds.add(id));
		}

		return tasks
			.filter((t) => !excludeIds.has(t.id))
			.filter((t) => {
				if (!searchQuery) return true;
				const query = searchQuery.toLowerCase();
				return t.id.toLowerCase().includes(query) || t.title.toLowerCase().includes(query);
			});
	}, [tasks, currentTaskId, type, existingBlockers, existingRelated, searchQuery]);

	const modalTitle = type === 'blocker' ? 'Add Blocking Task' : 'Add Related Task';
	const helpText =
		type === 'blocker'
			? 'Select a task that must be completed before this one'
			: 'Select a task that is related to this one';

	return (
		<Modal open={open} onClose={onClose} title={modalTitle} size="md">
			<div className="add-dependency-modal">
				<p className="help-text">{helpText}</p>

				{/* Search input */}
				<div className="search-box">
					<Icon name="search" size={16} />
					<input
						type="text"
						placeholder="Search by ID or title..."
						value={searchQuery}
						onChange={(e) => setSearchQuery(e.target.value)}
						disabled={loading}
					/>
				</div>

				{/* Content */}
				{loading ? (
					<div className="loading-state" role="status" aria-live="polite">
						<div className="spinner" aria-hidden="true" />
						<span>Loading tasks...</span>
					</div>
				) : error ? (
					<div className="error-state" role="alert">
						<span className="error-icon" aria-hidden="true">!</span>
						<span>{error}</span>
						<button type="button" className="btn-retry" onClick={loadTasks}>
							Retry
						</button>
					</div>
				) : filteredTasks.length === 0 ? (
					<div className="empty-state">
						{searchQuery ? (
							<p>No tasks matching "{searchQuery}"</p>
						) : (
							<p>No available tasks to add</p>
						)}
					</div>
				) : (
					<ul className="task-list">
						{filteredTasks.map((task) => {
							const statusInfo = getStatusIcon(task.status);
							return (
								<li key={task.id}>
									<button
										type="button"
										className="task-item"
										onClick={() => onSelect(task.id)}
									>
										<div className="task-main">
											<span
												className={`status-icon ${statusInfo.className}`}
												title={getStatusLabel(task.status)}
											>
												{statusInfo.icon}
											</span>
											<span className="task-id">{task.id}</span>
											<span className="task-title">{task.title}</span>
										</div>
										<div className="task-meta">
											<span className="task-status">
												{getStatusLabel(task.status)}
											</span>
										</div>
									</button>
								</li>
							);
						})}
					</ul>
				)}

				{/* Actions */}
				<div className="modal-actions">
					<button type="button" className="btn-cancel" onClick={onClose}>
						Cancel
					</button>
				</div>
			</div>
		</Modal>
	);
}
