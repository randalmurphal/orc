/**
 * Toggle component for boolean on/off switches.
 * Provides accessible toggle functionality with hidden checkbox for form compatibility.
 */

import {
	forwardRef,
	type InputHTMLAttributes,
	type ChangeEvent,
	type KeyboardEvent,
	useId,
	useRef,
	useImperativeHandle,
} from 'react';
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
			'aria-label': ariaLabel,
			...props
		},
		ref
	) => {
		const generatedId = useId();
		const inputId = providedId ?? generatedId;
		const inputRef = useRef<HTMLInputElement>(null);

		// Expose the input element via the forwarded ref
		useImperativeHandle(ref, () => inputRef.current as HTMLInputElement);

		const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
			if (!disabled && onChange) {
				onChange(event.target.checked, event);
			}
		};

		const handleKeyDown = (event: KeyboardEvent<HTMLLabelElement>) => {
			if (event.key === ' ' || event.key === 'Enter') {
				event.preventDefault();
				if (inputRef.current && !disabled) {
					inputRef.current.click();
				}
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
			<label
				className={wrapperClasses}
				htmlFor={inputId}
				role="switch"
				aria-checked={checked}
				aria-label={ariaLabel}
				aria-disabled={disabled || undefined}
				tabIndex={disabled ? -1 : 0}
				onKeyDown={handleKeyDown}
			>
				<span className="toggle__track">
					<span className="toggle__knob" />
				</span>
				<input
					ref={inputRef}
					type="checkbox"
					id={inputId}
					className="toggle__input"
					checked={checked}
					onChange={handleChange}
					disabled={disabled}
					tabIndex={-1}
					aria-hidden="true"
					{...props}
				/>
			</label>
		);
	}
);

Toggle.displayName = 'Toggle';
