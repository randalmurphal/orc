/**
 * DependencyDropdown component
 *
 * Dropdown to filter tasks by dependency status.
 * Options: All tasks, Ready, Blocked, No dependencies
 */

import { useState, useCallback, useRef, useEffect, useMemo } from 'react';
import { Icon } from '@/components/ui/Icon';
import {
	useCurrentDependencyStatus,
	useDependencyStore,
	DEPENDENCY_OPTIONS,
	type DependencyStatusFilter,
} from '@/stores';
import type { Task } from '@/lib/types';
import './DependencyDropdown.css';

interface DependencyDropdownProps {
	tasks: Task[];
}

export function DependencyDropdown({ tasks }: DependencyDropdownProps) {
	const [isOpen, setIsOpen] = useState(false);
	const dropdownRef = useRef<HTMLDivElement>(null);
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
			if (task.dependency_status === 'blocked') counts.blocked++;
			else if (task.dependency_status === 'ready') counts.ready++;
			else if (task.dependency_status === 'none') counts.none++;
		}
		return counts;
	}, [tasks]);

	// Get count for a specific filter
	const getCount = (value: DependencyStatusFilter): number => {
		return statusCounts[value] ?? 0;
	};

	// Handle selection
	const handleSelect = useCallback(
		(value: DependencyStatusFilter) => {
			selectDependencyStatus(value === 'all' ? null : value);
			setIsOpen(false);
		},
		[selectDependencyStatus]
	);

	// Get display text for current selection
	const displayText = useMemo(() => {
		const option = DEPENDENCY_OPTIONS.find((o) => o.value === selectedStatus);
		return option?.label ?? 'All tasks';
	}, [selectedStatus]);

	// Toggle dropdown
	const handleToggle = useCallback(() => {
		setIsOpen((prev) => !prev);
	}, []);

	// Keyboard handler
	const handleKeydown = useCallback((e: React.KeyboardEvent) => {
		if (e.key === 'Escape') {
			setIsOpen(false);
		}
	}, []);

	// Close on click outside
	useEffect(() => {
		if (!isOpen) return;

		const handleClickOutside = (e: MouseEvent) => {
			if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
				setIsOpen(false);
			}
		};

		document.addEventListener('mousedown', handleClickOutside);
		return () => document.removeEventListener('mousedown', handleClickOutside);
	}, [isOpen]);

	const isActive = selectedStatus !== 'all';

	return (
		<div className="dependency-dropdown" ref={dropdownRef} onKeyDown={handleKeydown}>
			<button
				type="button"
				className={`dropdown-trigger ${isActive ? 'active' : ''}`}
				onClick={handleToggle}
				aria-haspopup="listbox"
				aria-expanded={isOpen}
			>
				<span className="trigger-text">{displayText}</span>
				<Icon name={isOpen ? 'chevron-up' : 'chevron-down'} size={14} />
			</button>

			{isOpen && (
				<div className="dropdown-menu" role="listbox">
					{DEPENDENCY_OPTIONS.map((option) => {
						const count = getCount(option.value);
						return (
							<button
								key={option.value}
								type="button"
								className={`dropdown-item ${selectedStatus === option.value ? 'selected' : ''}`}
								onClick={() => handleSelect(option.value)}
								role="option"
								aria-selected={selectedStatus === option.value}
							>
								<span className="item-indicator">
									{selectedStatus === option.value ? (
										<span className="indicator-dot filled" />
									) : (
										<span className="indicator-dot" />
									)}
								</span>
								<span className="item-label">{option.label}</span>
								{option.value !== 'all' && (
									<span className="item-count">{count}</span>
								)}
							</button>
						);
					})}
				</div>
			)}
		</div>
	);
}
