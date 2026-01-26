/**
 * DependencyDropdown component
 *
 * Dropdown to filter tasks by dependency status.
 * Options: All tasks, Ready, Blocked, No dependencies
 * Uses Radix Select for accessibility (keyboard navigation, typeahead, ARIA).
 */

import { useMemo } from 'react';
import * as Select from '@radix-ui/react-select';
import { Icon } from '@/components/ui/Icon';
import {
	useCurrentDependencyStatus,
	useDependencyStore,
	DEPENDENCY_OPTIONS,
	type DependencyStatusFilter,
} from '@/stores';
import type { Task } from '@/gen/orc/v1/task_pb';
import { DependencyStatus } from '@/gen/orc/v1/task_pb';
import './DependencyDropdown.css';

interface DependencyDropdownProps {
	tasks: Task[];
}

export function DependencyDropdown({ tasks }: DependencyDropdownProps) {
	const selectedStatus = useCurrentDependencyStatus();
	const selectDependencyStatus = useDependencyStore((state) => state.selectDependencyStatus);

	// Count tasks by dependency status
	const statusCounts = useMemo(() => {
		const counts: Record<DependencyStatusFilter, number> = {
			all: tasks.length,
			blocked: 0,
			ready: 0,
			none: 0,
		};
		for (const task of tasks) {
			if (task.dependencyStatus === DependencyStatus.BLOCKED) counts.blocked++;
			else if (task.dependencyStatus === DependencyStatus.READY) counts.ready++;
			else if (task.dependencyStatus === DependencyStatus.NONE) counts.none++;
		}
		return counts;
	}, [tasks]);

	// Get count for a specific filter
	const getCount = (value: DependencyStatusFilter): number => {
		return statusCounts[value] ?? 0;
	};

	// Handle selection
	const handleValueChange = (value: string) => {
		const filter = value as DependencyStatusFilter;
		selectDependencyStatus(filter === 'all' ? null : filter);
	};

	// Get display text for current selection
	const displayText = useMemo(() => {
		const option = DEPENDENCY_OPTIONS.find((o) => o.value === selectedStatus);
		return option?.label ?? 'All tasks';
	}, [selectedStatus]);

	const isActive = selectedStatus !== 'all';

	return (
		<div className="dependency-dropdown">
			<Select.Root value={selectedStatus} onValueChange={handleValueChange}>
				<Select.Trigger
					className={`dropdown-trigger ${isActive ? 'active' : ''}`}
					aria-label="Filter by dependency status"
				>
					<span className="trigger-text">{displayText}</span>
					<Select.Icon className="chevron">
						<Icon name="chevron-down" size={14} />
					</Select.Icon>
				</Select.Trigger>

				<Select.Portal>
					<Select.Content className="dropdown-menu" position="popper" sideOffset={4}>
						<Select.Viewport className="dropdown-viewport">
							{DEPENDENCY_OPTIONS.map((option) => {
								const count = getCount(option.value);
								return (
									<Select.Item
										key={option.value}
										value={option.value}
										className="dropdown-item"
									>
										<span className="item-indicator">
											<span className="indicator-dot" />
										</span>
										<Select.ItemText>
											<span className="item-label">{option.label}</span>
										</Select.ItemText>
										{option.value !== 'all' && (
											<span className="item-count">{count}</span>
										)}
									</Select.Item>
								);
							})}
						</Select.Viewport>
					</Select.Content>
				</Select.Portal>
			</Select.Root>
		</div>
	);
}
