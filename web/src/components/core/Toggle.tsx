/**
 * Toggle component for boolean on/off switches.
 * Provides accessible toggle functionality with hidden checkbox for form compatibility.
 */

import { forwardRef, type InputHTMLAttributes, type ChangeEvent, useId } from 'react';
import './Toggle.css';

export type ToggleSize = 'sm' | 'md';

export interface ToggleProps
	extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size' | 'type' | 'onChange'> {
	/** Whether the toggle is on */
	checked?: boolean;
	/** Callback when toggle state changes */
	onChange?: (checked: boolean, event: ChangeEvent<HTMLInputElement>) => void;
	/** Whether the toggle is disabled */
	disabled?: boolean;
	/** Toggle size */
	size?: ToggleSize;
}

/**
 * Toggle component for boolean settings.
 *
 * @example
 * // Basic toggle
 * <Toggle checked={isEnabled} onChange={setIsEnabled} />
 *
 * @example
 * // With label
 * <label>
 *   Enable feature
 *   <Toggle checked={isEnabled} onChange={setIsEnabled} />
 * </label>
 *
 * @example
 * // In a form
 * <Toggle name="autoSave" checked={autoSave} onChange={setAutoSave} />
 *
 * @example
 * // Small size
 * <Toggle size="sm" checked={isOn} onChange={setIsOn} />
 */
export const Toggle = forwardRef<HTMLInputElement, ToggleProps>(
	(
		{
			checked = false,
			onChange,
			disabled = false,
			size = 'md',
			className = '',
			id: providedId,
			...props
		},
		ref
	) => {
		const generatedId = useId();
		const inputId = providedId ?? generatedId;

		const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
			if (!disabled && onChange) {
				onChange(event.target.checked, event);
			}
		};

		const wrapperClasses = [
			'toggle',
			`toggle--${size}`,
			checked && 'toggle--on',
			disabled && 'toggle--disabled',
			className,
		]
			.filter(Boolean)
			.join(' ');

		return (
			<label className={wrapperClasses} htmlFor={inputId}>
				<span className="toggle__track">
					<span className="toggle__knob" />
				</span>
				<input
					ref={ref}
					type="checkbox"
					id={inputId}
					className="toggle__input"
					checked={checked}
					onChange={handleChange}
					disabled={disabled}
					role="switch"
					aria-checked={checked}
					{...props}
				/>
			</label>
		);
	}
);

Toggle.displayName = 'Toggle';
