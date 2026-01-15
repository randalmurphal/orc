/**
 * InitiativeDropdown component
 *
 * Dropdown to filter tasks by initiative.
 * Options: All initiatives, Unassigned, and each initiative with task count.
 */

import { useState, useCallback, useRef, useEffect, useMemo } from 'react';
import { Icon } from '@/components/ui/Icon';
import { useInitiatives, UNASSIGNED_INITIATIVE } from '@/stores';
import type { Task } from '@/lib/types';
import './InitiativeDropdown.css';

interface InitiativeDropdownProps {
	currentInitiativeId: string | null;
	onSelect: (id: string | null) => void;
	tasks: Task[];
}

export function InitiativeDropdown({
	currentInitiativeId,
	onSelect,
	tasks,
}: InitiativeDropdownProps) {
	const [isOpen, setIsOpen] = useState(false);
	const dropdownRef = useRef<HTMLDivElement>(null);
	const initiatives = useInitiatives();

	// Calculate task counts per initiative
	const taskCounts = useMemo(() => {
		const counts: Record<string, number> = { unassigned: 0 };
		for (const init of initiatives) {
			counts[init.id] = 0;
		}
		for (const task of tasks) {
			if (task.initiative_id && counts[task.initiative_id] !== undefined) {
				counts[task.initiative_id]++;
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
			if (a.status === 'active' && b.status !== 'active') return -1;
			if (b.status === 'active' && a.status !== 'active') return 1;
			// Then by title
			return a.title.localeCompare(b.title);
		});
	}, [initiatives]);

	// Get display label for current selection
	const displayLabel = useMemo(() => {
		if (!currentInitiativeId) return 'All initiatives';
		if (currentInitiativeId === UNASSIGNED_INITIATIVE) return 'Unassigned';
		const init = initiatives.find((i) => i.id === currentInitiativeId);
		return init ? truncateTitle(init.title, 24) : currentInitiativeId;
	}, [currentInitiativeId, initiatives]);

	const handleToggle = useCallback(() => {
		setIsOpen((prev) => !prev);
	}, []);

	const handleSelect = useCallback(
		(id: string | null) => {
			onSelect(id);
			setIsOpen(false);
		},
		[onSelect]
	);

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

	const isActive = currentInitiativeId !== null;

	return (
		<div className="initiative-dropdown" ref={dropdownRef} onKeyDown={handleKeydown}>
			<button
				type="button"
				className={`dropdown-trigger ${isActive ? 'active' : ''}`}
				onClick={handleToggle}
				aria-expanded={isOpen}
				aria-haspopup="listbox"
			>
				<Icon name="layers" size={16} />
				<span className="trigger-label">{displayLabel}</span>
				<Icon name="chevron-down" size={14} className={`chevron ${isOpen ? 'open' : ''}`} />
			</button>

			{isOpen && (
				<div className="dropdown-menu" role="listbox">
					{/* All initiatives */}
					<button
						type="button"
						className={`dropdown-option ${currentInitiativeId === null ? 'selected' : ''}`}
						onClick={() => handleSelect(null)}
						role="option"
						aria-selected={currentInitiativeId === null}
					>
						<span className="option-label">All initiatives</span>
					</button>

					{/* Unassigned */}
					<button
						type="button"
						className={`dropdown-option ${currentInitiativeId === UNASSIGNED_INITIATIVE ? 'selected' : ''}`}
						onClick={() => handleSelect(UNASSIGNED_INITIATIVE)}
						role="option"
						aria-selected={currentInitiativeId === UNASSIGNED_INITIATIVE}
					>
						<span className="option-label">Unassigned</span>
						<span className="option-count">{taskCounts['unassigned']}</span>
					</button>

					{sortedInitiatives.length > 0 && <div className="dropdown-divider" />}

					{/* Initiative list */}
					{sortedInitiatives.map((init) => (
						<button
							key={init.id}
							type="button"
							className={`dropdown-option ${currentInitiativeId === init.id ? 'selected' : ''}`}
							onClick={() => handleSelect(init.id)}
							role="option"
							aria-selected={currentInitiativeId === init.id}
							title={init.title}
						>
							<span className="option-label">{truncateTitle(init.title, 24)}</span>
							<span className="option-count">{taskCounts[init.id] || 0}</span>
						</button>
					))}
				</div>
			)}
		</div>
	);
}

function truncateTitle(title: string, maxLength: number): string {
	if (title.length <= maxLength) return title;
	return title.slice(0, maxLength - 1) + '\u2026';
}
