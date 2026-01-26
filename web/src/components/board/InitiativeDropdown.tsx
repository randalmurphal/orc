/**
 * InitiativeDropdown component
 *
 * Dropdown to filter tasks by initiative.
 * Options: All initiatives, Unassigned, and each initiative with task count.
 * Uses Radix Select for accessibility (keyboard navigation, typeahead, ARIA).
 */

import { useMemo } from 'react';
import * as Select from '@radix-ui/react-select';
import { Icon } from '@/components/ui/Icon';
import { Tooltip } from '@/components/ui/Tooltip';
import { useInitiatives, UNASSIGNED_INITIATIVE } from '@/stores';
import type { Task } from '@/gen/orc/v1/task_pb';
import { InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import './InitiativeDropdown.css';

interface InitiativeDropdownProps {
	currentInitiativeId: string | null;
	onSelect: (id: string | null) => void;
	tasks: Task[];
}

// Internal value for "all initiatives" since Radix Select requires string values
const ALL_INITIATIVES_VALUE = '__all__';

export function InitiativeDropdown({
	currentInitiativeId,
	onSelect,
	tasks,
}: InitiativeDropdownProps) {
	const initiatives = useInitiatives();

	// Calculate task counts per initiative
	const taskCounts = useMemo(() => {
		const counts: Record<string, number> = { unassigned: 0 };
		for (const init of initiatives) {
			counts[init.id] = 0;
		}
		for (const task of tasks) {
			if (task.initiativeId && counts[task.initiativeId] !== undefined) {
				counts[task.initiativeId]++;
			} else {
				counts['unassigned']++;
			}
		}
		return counts;
	}, [tasks, initiatives]);

	// Sort initiatives: active first, then by title
	const sortedInitiatives = useMemo(() => {
		return [...initiatives].sort((a, b) => {
			// Active first
			if (a.status === InitiativeStatus.ACTIVE && b.status !== InitiativeStatus.ACTIVE) return -1;
			if (b.status === InitiativeStatus.ACTIVE && a.status !== InitiativeStatus.ACTIVE) return 1;
			// Then by title
			return a.title.localeCompare(b.title);
		});
	}, [initiatives]);

	// Convert external value (null for all) to internal Select value
	const selectValue = currentInitiativeId === null ? ALL_INITIATIVES_VALUE : currentInitiativeId;

	// Get display label for trigger
	const displayLabel = useMemo(() => {
		if (!currentInitiativeId) return 'All initiatives';
		if (currentInitiativeId === UNASSIGNED_INITIATIVE) return 'Unassigned';
		const init = initiatives.find((i) => i.id === currentInitiativeId);
		return init ? truncateTitle(init.title, 24) : currentInitiativeId;
	}, [currentInitiativeId, initiatives]);

	// Get full title for tooltip
	const fullTitle = useMemo(() => {
		if (!currentInitiativeId) return 'All initiatives';
		if (currentInitiativeId === UNASSIGNED_INITIATIVE) return 'Unassigned';
		const init = initiatives.find((i) => i.id === currentInitiativeId);
		return init ? init.title : currentInitiativeId;
	}, [currentInitiativeId, initiatives]);

	// Handle selection change
	const handleValueChange = (value: string) => {
		if (value === ALL_INITIATIVES_VALUE) {
			onSelect(null);
		} else {
			onSelect(value);
		}
	};

	const isActive = currentInitiativeId !== null;

	return (
		<div className="initiative-dropdown">
			<Select.Root value={selectValue} onValueChange={handleValueChange}>
				<Tooltip content={fullTitle} side="bottom">
					<Select.Trigger
						className={`dropdown-trigger ${isActive ? 'active' : ''}`}
						aria-label="Filter by initiative"
					>
						<Icon name="layers" size={16} />
						<span className="trigger-text">{displayLabel}</span>
						<Select.Icon className="chevron">
							<Icon name="chevron-down" size={14} />
						</Select.Icon>
					</Select.Trigger>
				</Tooltip>

				<Select.Portal>
					<Select.Content className="dropdown-menu" position="popper" sideOffset={4}>
						<Select.Viewport className="dropdown-viewport">
							{/* All initiatives */}
							<Select.Item value={ALL_INITIATIVES_VALUE} className="dropdown-item">
								<span className="item-label">All initiatives</span>
							</Select.Item>

							{/* Unassigned */}
							<Select.Item value={UNASSIGNED_INITIATIVE} className="dropdown-item">
								<span className="item-label">Unassigned</span>
								<span className="item-count">{taskCounts['unassigned']}</span>
							</Select.Item>

							{sortedInitiatives.length > 0 && (
								<Select.Separator className="dropdown-divider" />
							)}

							{/* Initiative list */}
							{sortedInitiatives.map((init) => (
								<Select.Item
									key={init.id}
									value={init.id}
									className="dropdown-item"
									title={init.title}
								>
									<span className="item-label">{truncateTitle(init.title, 24)}</span>
									<span className="item-count">{taskCounts[init.id] || 0}</span>
								</Select.Item>
							))}
						</Select.Viewport>
					</Select.Content>
				</Select.Portal>
			</Select.Root>
		</div>
	);
}

function truncateTitle(title: string, maxLength: number): string {
	if (title.length <= maxLength) return title;
	return title.slice(0, maxLength - 1) + '\u2026';
}
