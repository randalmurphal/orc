/**
 * Slider component for numeric range selection.
 * Provides a draggable track with visual value display and keyboard accessibility.
 */

import {
	forwardRef,
	useCallback,
	useEffect,
	useRef,
	useState,
	type HTMLAttributes,
	type KeyboardEvent,
	type MouseEvent,
	type TouchEvent,
} from 'react';
import './Slider.css';

export interface SliderProps extends Omit<HTMLAttributes<HTMLDivElement>, 'onChange'> {
	/** Current value */
	value: number;
	/** Callback when value changes */
	onChange: (value: number) => void;
	/** Minimum value (default 0) */
	min?: number;
	/** Maximum value (default 100) */
	max?: number;
	/** Step increment (default 1) */
	step?: number;
	/** Show value display to the right */
	showValue?: boolean;
	/** Custom value formatter (default: String) */
	formatValue?: (value: number) => string;
	/** Whether the slider is disabled */
	disabled?: boolean;
}

/**
 * Slider component for numeric range selection.
 *
 * @example
 * // Basic slider
 * <Slider value={50} onChange={setValue} />
 *
 * @example
 * // With custom range
 * <Slider value={5} onChange={setValue} min={0} max={10} />
 *
 * @example
 * // With step and value display
 * <Slider value={25} onChange={setValue} step={5} showValue />
 *
 * @example
 * // With custom formatter
 * <Slider value={50} onChange={setValue} showValue formatValue={v => `$${v}`} />
 */
export const Slider = forwardRef<HTMLDivElement, SliderProps>(
	(
		{
			value,
			onChange,
			min = 0,
			max = 100,
			step = 1,
			showValue = false,
			formatValue = String,
			disabled = false,
			className = '',
			...props
		},
		ref
	) => {
		const [isDragging, setIsDragging] = useState(false);
		const trackRef = useRef<HTMLDivElement>(null);

		// Clamp value to valid range and snap to step
		const snapToStep = useCallback(
			(rawValue: number): number => {
				const clamped = Math.min(Math.max(rawValue, min), max);
				const steps = Math.round((clamped - min) / step);
				const snapped = min + steps * step;
				// Round to step precision to avoid floating point errors
				const precision = String(step).split('.')[1]?.length ?? 0;
				const rounded = precision > 0 ? Number(snapped.toFixed(precision)) : snapped;
				// Ensure we don't exceed max due to floating point
				return Math.min(rounded, max);
			},
			[min, max, step]
		);

		// Calculate percentage for visual positioning
		const percentage = max > min ? ((value - min) / (max - min)) * 100 : 0;

		// Convert mouse/touch position to value
		const positionToValue = useCallback(
			(clientX: number): number => {
				if (!trackRef.current) return value;
				const rect = trackRef.current.getBoundingClientRect();
				const ratio = Math.min(Math.max((clientX - rect.left) / rect.width, 0), 1);
				const rawValue = min + ratio * (max - min);
				return snapToStep(rawValue);
			},
			[min, max, value, snapToStep]
		);

		// Mouse event handlers
		const handleMouseDown = useCallback(
			(event: MouseEvent<HTMLDivElement>) => {
				if (disabled) return;
				event.preventDefault();
				setIsDragging(true);
				const newValue = positionToValue(event.clientX);
				if (newValue !== value) {
					onChange(newValue);
				}
			},
			[disabled, positionToValue, value, onChange]
		);

		// Touch event handlers
		const handleTouchStart = useCallback(
			(event: TouchEvent<HTMLDivElement>) => {
				if (disabled) return;
				event.preventDefault();
				setIsDragging(true);
				const touch = event.touches[0];
				const newValue = positionToValue(touch.clientX);
				if (newValue !== value) {
					onChange(newValue);
				}
			},
			[disabled, positionToValue, value, onChange]
		);

		// Global mouse/touch move and up handlers
		useEffect(() => {
			if (!isDragging) return;

			const handleMouseMove = (event: globalThis.MouseEvent) => {
				const newValue = positionToValue(event.clientX);
				if (newValue !== value) {
					onChange(newValue);
				}
			};

			const handleTouchMove = (event: globalThis.TouchEvent) => {
				const touch = event.touches[0];
				const newValue = positionToValue(touch.clientX);
				if (newValue !== value) {
					onChange(newValue);
				}
			};

			const handleEnd = () => {
				setIsDragging(false);
			};

			document.addEventListener('mousemove', handleMouseMove);
			document.addEventListener('mouseup', handleEnd);
			document.addEventListener('touchmove', handleTouchMove);
			document.addEventListener('touchend', handleEnd);

			return () => {
				document.removeEventListener('mousemove', handleMouseMove);
				document.removeEventListener('mouseup', handleEnd);
				document.removeEventListener('touchmove', handleTouchMove);
				document.removeEventListener('touchend', handleEnd);
			};
		}, [isDragging, positionToValue, value, onChange]);

		// Keyboard navigation
		const handleKeyDown = useCallback(
			(event: KeyboardEvent<HTMLDivElement>) => {
				if (disabled) return;

				let newValue: number | null = null;

				switch (event.key) {
					case 'ArrowRight':
					case 'ArrowUp':
						newValue = snapToStep(value + step);
						break;
					case 'ArrowLeft':
					case 'ArrowDown':
						newValue = snapToStep(value - step);
						break;
					case 'Home':
						newValue = min;
						break;
					case 'End':
						newValue = max;
						break;
					case 'PageUp':
						newValue = snapToStep(value + step * 10);
						break;
					case 'PageDown':
						newValue = snapToStep(value - step * 10);
						break;
					default:
						return;
				}

				if (newValue !== null && newValue !== value) {
					event.preventDefault();
					onChange(newValue);
				}
			},
			[disabled, value, step, min, max, snapToStep, onChange]
		);

		const wrapperClasses = [
			'slider',
			isDragging && 'slider--dragging',
			disabled && 'slider--disabled',
			className,
		]
			.filter(Boolean)
			.join(' ');

		return (
			<div ref={ref} className={wrapperClasses} {...props}>
				<div
					ref={trackRef}
					className="slider__track"
					role="slider"
					tabIndex={disabled ? -1 : 0}
					aria-valuenow={value}
					aria-valuemin={min}
					aria-valuemax={max}
					aria-disabled={disabled}
					onMouseDown={handleMouseDown}
					onTouchStart={handleTouchStart}
					onKeyDown={handleKeyDown}
				>
					<div className="slider__fill" style={{ width: `${percentage}%` }} />
					<div className="slider__thumb" style={{ left: `${percentage}%` }} />
				</div>
				{showValue && <span className="slider__value">{formatValue(value)}</span>}
			</div>
		);
	}
);

Slider.displayName = 'Slider';
