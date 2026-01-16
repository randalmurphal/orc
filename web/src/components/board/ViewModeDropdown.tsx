/**
 * ViewModeDropdown component
 *
 * Dropdown to toggle between flat and swimlane board views.
 * Uses Radix Select for accessibility (keyboard navigation, typeahead, ARIA).
 */

import * as Select from '@radix-ui/react-select';
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
	const currentOption = VIEW_OPTIONS.find((opt) => opt.id === value) ?? VIEW_OPTIONS[0];

	const handleValueChange = (newValue: string) => {
		onChange(newValue as BoardViewMode);
	};

	return (
		<div className={`view-mode-dropdown ${disabled ? 'disabled' : ''}`}>
			<Select.Root value={value} onValueChange={handleValueChange} disabled={disabled}>
				<Select.Trigger className="dropdown-trigger" aria-label="Select view mode">
					<Icon name="layout" size={16} />
					<span className="trigger-text">{currentOption.label}</span>
					<Select.Icon className="chevron">
						<Icon name="chevron-down" size={14} />
					</Select.Icon>
				</Select.Trigger>

				<Select.Portal>
					<Select.Content className="dropdown-menu" position="popper" sideOffset={4}>
						<Select.Viewport className="dropdown-viewport">
							{VIEW_OPTIONS.map((option) => (
								<Select.Item
									key={option.id}
									value={option.id}
									className="dropdown-item"
								>
									<span className="indicator-dot" />
									<div className="item-content">
										<Select.ItemText>{option.label}</Select.ItemText>
										<span className="item-description">{option.description}</span>
									</div>
								</Select.Item>
							))}
						</Select.Viewport>
					</Select.Content>
				</Select.Portal>
			</Select.Root>
		</div>
	);
}
