/**
 * QueueColumn component with swimlane layout
 *
 * Displays queued tasks grouped by initiative in collapsible swimlanes:
 * - Column header with "Queue" title, indicator dot, and task count badge
 * - Swimlanes sorted: active initiatives first, then by task count (descending)
 * - "Unassigned" swimlane at bottom for tasks without initiative_id
 * - Scrollable content area with custom thin scrollbar
 * - Empty state when no tasks
 */

import { useMemo } from 'react';
import { Swimlane } from './Swimlane';
import type { Task, Initiative } from '@/lib/types';
import './QueueColumn.css';

export interface QueueColumnProps {
	/** Tasks filtered to queued status */
	tasks: Task[];
	/** Initiatives for grouping and display */
	initiatives: Initiative[];
	/** Controlled collapse state - set of initiative IDs that are collapsed */
	collapsedSwimlanes?: Set<string>;
	/** Callback when swimlane collapse is toggled */
	onToggleSwimlane?: (id: string) => void;
	/** Callback when a task is clicked */
	onTaskClick?: (task: Task) => void;
	/** Callback for task context menu (right-click) */
	onContextMenu?: (task: Task, e: React.MouseEvent) => void;
}

interface SwimlaneGroup {
	initiative: Initiative | null;
	tasks: Task[];
}

/**
 * Groups tasks by initiative and sorts swimlanes:
 * 1. Active initiatives first
 * 2. Then by task count (descending)
 * 3. Unassigned group at the end
 */
function groupAndSortTasks(
	tasks: Task[],
	initiatives: Initiative[]
): SwimlaneGroup[] {
	// Create lookup map for initiatives
	const initiativeMap = new Map<string, Initiative>();
	for (const init of initiatives) {
		initiativeMap.set(init.id, init);
	}

	// Group tasks by initiative_id
	const groups = new Map<string | null, Task[]>();
	for (const task of tasks) {
		const key = task.initiative_id ?? null;
		const existing = groups.get(key) ?? [];
		existing.push(task);
		groups.set(key, existing);
	}

	// Convert to array of SwimlaneGroup
	const swimlaneGroups: SwimlaneGroup[] = [];
	let unassignedGroup: SwimlaneGroup | null = null;

	for (const [initiativeId, groupTasks] of groups) {
		if (initiativeId === null) {
			// Save unassigned for later (goes at end)
			unassignedGroup = { initiative: null, tasks: groupTasks };
		} else {
			const initiative = initiativeMap.get(initiativeId);
			if (initiative) {
				swimlaneGroups.push({ initiative, tasks: groupTasks });
			} else {
				// Initiative not found - treat as unassigned
				if (!unassignedGroup) {
					unassignedGroup = { initiative: null, tasks: [] };
				}
				unassignedGroup.tasks.push(...groupTasks);
			}
		}
	}

	// Sort: active initiatives first, then by task count (descending)
	swimlaneGroups.sort((a, b) => {
		// Active initiatives first
		const aActive = a.initiative?.status === 'active' ? 0 : 1;
		const bActive = b.initiative?.status === 'active' ? 0 : 1;
		if (aActive !== bActive) return aActive - bActive;

		// Then by task count descending
		return b.tasks.length - a.tasks.length;
	});

	// Add unassigned at the end if it exists
	if (unassignedGroup && unassignedGroup.tasks.length > 0) {
		swimlaneGroups.push(unassignedGroup);
	}

	return swimlaneGroups;
}

export function QueueColumn({
	tasks,
	initiatives,
	collapsedSwimlanes,
	onToggleSwimlane,
	onTaskClick,
	onContextMenu,
}: QueueColumnProps) {
	// Group and sort tasks into swimlanes
	const swimlaneGroups = useMemo(
		() => groupAndSortTasks(tasks, initiatives),
		[tasks, initiatives]
	);

	const totalCount = tasks.length;

	// Handle toggle for a swimlane
	const handleToggle = (id: string) => {
		onToggleSwimlane?.(id);
	};

	return (
		<div
			className="queue-column"
			role="region"
			aria-label="Queue column"
		>
			{/* Column header */}
			<div className="queue-column-header">
				<div className="queue-column-title">
					<span className="queue-column-indicator" aria-hidden="true" />
					<span>Queue</span>
				</div>
				<span className="queue-column-count" aria-label={`${totalCount} tasks`}>
					{totalCount}
				</span>
			</div>

			{/* Column body - scrollable */}
			<div className="queue-column-body">
				{totalCount === 0 ? (
					<div className="queue-column-empty">No queued tasks</div>
				) : (
					swimlaneGroups.map((group) => {
						const swimlaneId = group.initiative?.id ?? 'unassigned';
						const isCollapsed = collapsedSwimlanes?.has(swimlaneId) ?? false;

						return (
							<Swimlane
								key={swimlaneId}
								initiative={group.initiative}
								tasks={group.tasks}
								isCollapsed={isCollapsed}
								onToggle={() => handleToggle(swimlaneId)}
								onTaskClick={onTaskClick}
								onContextMenu={onContextMenu}
							/>
						);
					})
				)}
			</div>
		</div>
	);
}
