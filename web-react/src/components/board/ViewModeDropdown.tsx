/**
 * ViewModeDropdown component
 *
 * Dropdown to toggle between flat and swimlane board views.
 */

import { useState, useCallback, useRef, useEffect } from 'react';
import { Icon } from '@/components/ui/Icon';
import type { BoardViewMode } from './Board';
import './ViewModeDropdown.css';

interface ViewOption {
	id: BoardViewMode;
	label: string;
	description: string;
}

const VIEW_OPTIONS: ViewOption[] = [
	{ id: 'flat', label: 'Flat', description: 'All tasks in columns' },
	{ id: 'swimlane', label: 'By Initiative', description: 'Grouped by initiative' },
];

interface ViewModeDropdownProps {
	value: BoardViewMode;
	onChange: (mode: BoardViewMode) => void;
	disabled?: boolean;
}

export function ViewModeDropdown({ value, onChange, disabled }: ViewModeDropdownProps) {
	const [isOpen, setIsOpen] = useState(false);
	const dropdownRef = useRef<HTMLDivElement>(null);

	const currentOption = VIEW_OPTIONS.find((opt) => opt.id === value) ?? VIEW_OPTIONS[0];

	const handleToggle = useCallback(() => {
		if (!disabled) {
			setIsOpen((prev) => !prev);
		}
	}, [disabled]);

	const handleSelect = useCallback(
		(mode: BoardViewMode) => {
			onChange(mode);
			setIsOpen(false);
		},
		[onChange]
	);

	const handleKeydown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Escape') {
				setIsOpen(false);
			}
		},
		[]
	);

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

	return (
		<div
			className={`view-mode-dropdown ${disabled ? 'disabled' : ''}`}
			ref={dropdownRef}
			onKeyDown={handleKeydown}
		>
			<button
				type="button"
				className="dropdown-trigger"
				onClick={handleToggle}
				aria-expanded={isOpen}
				aria-haspopup="listbox"
				disabled={disabled}
			>
				<Icon name="layout" size={16} />
				<span className="trigger-text">{currentOption.label}</span>
				<Icon name="chevron-down" size={14} className={`chevron ${isOpen ? 'open' : ''}`} />
			</button>

			{isOpen && (
				<div className="dropdown-menu" role="listbox">
					{VIEW_OPTIONS.map((option) => (
						<button
							key={option.id}
							type="button"
							className={`dropdown-item ${option.id === value ? 'selected' : ''}`}
							onClick={() => handleSelect(option.id)}
							role="option"
							aria-selected={option.id === value}
						>
							<span className="indicator-dot" />
							<div className="item-content">
								<span className="item-label">{option.label}</span>
								<span className="item-description">{option.description}</span>
							</div>
						</button>
					))}
				</div>
			)}
		</div>
	);
}
