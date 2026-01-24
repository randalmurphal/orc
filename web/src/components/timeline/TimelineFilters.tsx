/**
 * TimelineFilters component - Filter dropdown for timeline events.
 *
 * Provides filtering by:
 * - Event types (with checkboxes grouped by category)
 * - Task ID (dropdown)
 * - Initiative ID (dropdown)
 *
 * Features:
 * - Active filter indicator badge
 * - Clear all filters button
 * - Keyboard navigation
 * - URL parameter sync (managed by parent)
 */

import { useState, useCallback, useRef, useEffect } from 'react';
import { Icon } from '@/components/ui';
import './TimelineFilters.css';

// Event type categories for grouping
const EVENT_TYPE_CATEGORIES = {
	'Phase Events': ['phase_started', 'phase_completed', 'phase_failed'],
	'Task Events': ['task_created', 'task_started', 'task_paused', 'task_completed'],
	'Activity Events': ['activity_changed', 'token_update'],
	'Alerts': ['error_occurred', 'warning_issued', 'gate_decision'],
};

// Labels for event types
const EVENT_TYPE_LABELS: Record<string, string> = {
	phase_started: 'Phase Started',
	phase_completed: 'Phase Completed',
	phase_failed: 'Phase Failed',
	task_created: 'Task Created',
	task_started: 'Task Started',
	task_paused: 'Task Paused',
	task_completed: 'Task Completed',
	activity_changed: 'Activity Changed',
	error_occurred: 'Error Occurred',
	warning_issued: 'Warning Issued',
	token_update: 'Token Update',
	gate_decision: 'Gate Decision',
};

// All event types as flat array
const ALL_EVENT_TYPES = Object.values(EVENT_TYPE_CATEGORIES).flat();

export interface TimelineFiltersProps {
	/** Currently selected event types */
	selectedTypes: string[];
	/** Currently selected task ID */
	selectedTaskId?: string;
	/** Currently selected initiative ID */
	selectedInitiativeId?: string;
	/** Available tasks for filtering */
	tasks: Array<{ id: string; title: string }>;
	/** Available initiatives for filtering */
	initiatives: Array<{ id: string; title: string }>;
	/** Callback when event types change */
	onTypesChange: (types: string[]) => void;
	/** Callback when task filter changes */
	onTaskChange: (taskId: string | undefined) => void;
	/** Callback when initiative filter changes */
	onInitiativeChange: (initiativeId: string | undefined) => void;
	/** Callback to clear all filters */
	onClearAll: () => void;
}

/**
 * TimelineFilters provides a dropdown interface for filtering timeline events.
 */
export function TimelineFilters({
	selectedTypes,
	selectedTaskId,
	selectedInitiativeId,
	tasks,
	initiatives,
	onTypesChange,
	onTaskChange,
	onInitiativeChange,
	onClearAll,
}: TimelineFiltersProps) {
	const [isOpen, setIsOpen] = useState(false);
	const menuRef = useRef<HTMLDivElement>(null);
	const buttonRef = useRef<HTMLButtonElement>(null);

	// Calculate active filter count
	const activeFilterCount =
		selectedTypes.length +
		(selectedTaskId ? 1 : 0) +
		(selectedInitiativeId ? 1 : 0);

	const hasFilters = activeFilterCount > 0;

	// Handle type checkbox change
	const handleTypeToggle = useCallback(
		(type: string) => {
			if (selectedTypes.includes(type)) {
				onTypesChange(selectedTypes.filter((t) => t !== type));
			} else {
				onTypesChange([...selectedTypes, type]);
			}
		},
		[selectedTypes, onTypesChange]
	);

	// Handle select all event types
	const handleSelectAll = useCallback(() => {
		onTypesChange([...ALL_EVENT_TYPES]);
	}, [onTypesChange]);

	// Handle clear type filter
	const handleClearTypes = useCallback(() => {
		onTypesChange([]);
	}, [onTypesChange]);

	// Handle keyboard navigation
	const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
		if (e.key === 'Escape') {
			setIsOpen(false);
			buttonRef.current?.focus();
		}
	}, []);

	// Close dropdown on outside click or Escape key
	useEffect(() => {
		if (!isOpen) return;

		const handleClickOutside = (e: MouseEvent) => {
			if (
				menuRef.current &&
				!menuRef.current.contains(e.target as Node) &&
				buttonRef.current &&
				!buttonRef.current.contains(e.target as Node)
			) {
				setIsOpen(false);
			}
		};

		const handleEscapeKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				setIsOpen(false);
				buttonRef.current?.focus();
			}
		};

		document.addEventListener('mousedown', handleClickOutside);
		document.addEventListener('keydown', handleEscapeKey);
		return () => {
			document.removeEventListener('mousedown', handleClickOutside);
			document.removeEventListener('keydown', handleEscapeKey);
		};
	}, [isOpen]);

	return (
		<div className="timeline-filters">
			{/* Filter Button */}
			<button
				ref={buttonRef}
				type="button"
				className={`timeline-filters-button ${hasFilters ? 'timeline-filters-button--active' : ''}`}
				onClick={() => setIsOpen(!isOpen)}
				aria-expanded={isOpen}
				aria-haspopup="menu"
				aria-label="Filter events"
			>
				<Icon name="sliders" size={16} />
				<span>Filter</span>
				{hasFilters && (
					<span className="timeline-filters-badge filter-badge">{activeFilterCount}</span>
				)}
			</button>

			{/* Dropdown Menu */}
			{isOpen && (
				<div
					ref={menuRef}
					className="timeline-filters-menu"
					role="menu"
					onKeyDown={handleKeyDown}
				>
					{/* Event Types Section */}
					<div className="timeline-filters-section">
						<div className="timeline-filters-section-header">
							<span>Event Types</span>
							<div className="timeline-filters-section-actions">
								<button
									type="button"
									onClick={handleSelectAll}
									className="timeline-filters-action"
									aria-label="Select all event types"
								>
									Select All
								</button>
								{selectedTypes.length > 0 && (
									<button
										type="button"
										onClick={handleClearTypes}
										className="timeline-filters-action"
										aria-label="Clear type filter"
									>
										Clear
									</button>
								)}
							</div>
						</div>

						{/* Event type categories */}
						{Object.entries(EVENT_TYPE_CATEGORIES).map(([category, types]) => (
							<div key={category} className="timeline-filters-category">
								<span className="timeline-filters-category-label">{category}</span>
								<div className="timeline-filters-checkboxes">
									{types.map((type) => (
										<label key={type} className="timeline-filters-checkbox">
											<input
												type="checkbox"
												checked={selectedTypes.includes(type)}
												onChange={() => handleTypeToggle(type)}
											/>
											<span>{EVENT_TYPE_LABELS[type]}</span>
										</label>
									))}
								</div>
							</div>
						))}
					</div>

					{/* Divider */}
					<div className="timeline-filters-divider" />

					{/* Task Filter */}
					<div className="timeline-filters-section">
						<label className="timeline-filters-select-label" htmlFor="task-filter-select">
							Task
						</label>
						<div className="timeline-filters-select-wrapper">
							<select
								id="task-filter-select"
								className="timeline-filters-select"
								value={selectedTaskId || ''}
								onChange={(e) =>
									onTaskChange(e.target.value || undefined)
								}
							>
								<option value="">All Tasks</option>
								{tasks.map((task) => (
									<option key={task.id} value={task.id}>
										{task.id}
									</option>
								))}
							</select>
						</div>
					</div>

					{/* Initiative Filter */}
					<div className="timeline-filters-section">
						<label className="timeline-filters-select-label" htmlFor="initiative-filter-select">
							Initiative
						</label>
						<div className="timeline-filters-select-wrapper">
							<select
								id="initiative-filter-select"
								className="timeline-filters-select"
								value={selectedInitiativeId || ''}
								onChange={(e) =>
									onInitiativeChange(e.target.value || undefined)
								}
							>
								<option value="">All Initiatives</option>
								{initiatives.map((initiative) => (
									<option key={initiative.id} value={initiative.id}>
										{initiative.title}
									</option>
								))}
							</select>
						</div>
					</div>

					{/* Clear All */}
					{hasFilters && (
						<>
							<div className="timeline-filters-divider" />
							<button
								type="button"
								className="timeline-filters-clear-all"
								onClick={() => {
									onClearAll();
									setIsOpen(false);
								}}
								aria-label="Clear all filters"
							>
								<Icon name="x" size={14} />
								<span>Clear All Filters</span>
							</button>
						</>
					)}
				</div>
			)}
		</div>
	);
}
