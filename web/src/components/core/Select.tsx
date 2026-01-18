/**
 * Select dropdown component matching the mockup design from example_ui/agents.html.
 * Built on Radix Select primitive for accessibility (keyboard navigation, ARIA, typeahead).
 */

import { forwardRef } from 'react';
import * as RadixSelect from '@radix-ui/react-select';
import './Select.css';

export interface SelectOption {
	/** The value stored/returned when this option is selected */
	value: string;
	/** The display text shown to the user */
	label: string;
	/** Optional: disable this option */
	disabled?: boolean;
}

export interface SelectProps {
	/** Currently selected value */
	value?: string;
	/** Callback when selection changes */
	onChange?: (value: string) => void;
	/** Available options */
	options: SelectOption[];
	/** Placeholder text when no value is selected */
	placeholder?: string;
	/** Disable the select */
	disabled?: boolean;
	/** Accessible label for screen readers */
	'aria-label'?: string;
	/** ID of element that labels this select */
	'aria-labelledby'?: string;
	/** Additional CSS classes */
	className?: string;
	/** Name attribute for form submission */
	name?: string;
	/** Whether the select is required */
	required?: boolean;
}

/**
 * Styled Select dropdown component.
 *
 * @example
 * // Basic usage
 * <Select
 *   value={selectedModel}
 *   onChange={setSelectedModel}
 *   options={[
 *     { value: 'sonnet', label: 'Claude Sonnet 4' },
 *     { value: 'opus', label: 'Claude Opus 4.5' },
 *   ]}
 *   placeholder="Select a model"
 * />
 *
 * @example
 * // With disabled option
 * <Select
 *   value={value}
 *   onChange={setValue}
 *   options={[
 *     { value: 'a', label: 'Option A' },
 *     { value: 'b', label: 'Option B', disabled: true },
 *   ]}
 * />
 */
export const Select = forwardRef<HTMLButtonElement, SelectProps>(
	(
		{
			value,
			onChange,
			options,
			placeholder = 'Select...',
			disabled = false,
			'aria-label': ariaLabel,
			'aria-labelledby': ariaLabelledBy,
			className = '',
			name,
			required,
		},
		ref
	) => {
		const selectedOption = options.find((opt) => opt.value === value);

		return (
			<RadixSelect.Root
				value={value}
				onValueChange={onChange}
				disabled={disabled}
				name={name}
				required={required}
			>
				<RadixSelect.Trigger
					ref={ref}
					className={`select-trigger ${className}`.trim()}
					aria-label={ariaLabel}
					aria-labelledby={ariaLabelledBy}
				>
					<RadixSelect.Value placeholder={placeholder}>
						{selectedOption?.label}
					</RadixSelect.Value>
					<RadixSelect.Icon className="select-icon">
						<ChevronDownIcon />
					</RadixSelect.Icon>
				</RadixSelect.Trigger>

				<RadixSelect.Portal>
					<RadixSelect.Content
						className="select-content"
						position="popper"
						sideOffset={4}
						align="start"
					>
						<RadixSelect.Viewport className="select-viewport">
							{options.map((option) => (
								<RadixSelect.Item
									key={option.value}
									value={option.value}
									className="select-item"
									disabled={option.disabled}
								>
									<RadixSelect.ItemText>{option.label}</RadixSelect.ItemText>
								</RadixSelect.Item>
							))}
						</RadixSelect.Viewport>
					</RadixSelect.Content>
				</RadixSelect.Portal>
			</RadixSelect.Root>
		);
	}
);

Select.displayName = 'Select';

/** Chevron down icon for the select trigger */
function ChevronDownIcon() {
	return (
		<svg
			width="12"
			height="12"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
		>
			<path d="M6 9l6 6 6-6" />
		</svg>
	);
}
