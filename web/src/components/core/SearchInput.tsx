/**
 * SearchInput component for search functionality.
 * Includes search icon on left, clear button on right when has value.
 */

import {
	forwardRef,
	useCallback,
	type InputHTMLAttributes,
	type KeyboardEvent,
	type ChangeEvent,
} from 'react';
import '../ui/Input.css';

export interface SearchInputProps
	extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'onChange'> {
	/** Controlled value */
	value?: string;
	/** Change handler */
	onChange?: (value: string) => void;
	/** Called when input is cleared (via clear button or Escape key) */
	onClear?: () => void;
	/** Additional class name for the wrapper */
	className?: string;
}

/**
 * SearchInput component with search icon and clear button.
 *
 * @example
 * // Controlled search input
 * const [query, setQuery] = useState('');
 * <SearchInput
 *   value={query}
 *   onChange={setQuery}
 *   placeholder="Search tasks..."
 * />
 *
 * @example
 * // With clear callback
 * <SearchInput
 *   value={query}
 *   onChange={setQuery}
 *   onClear={() => console.log('Cleared!')}
 *   placeholder="Search..."
 * />
 *
 * @example
 * // In a form with name attribute
 * <SearchInput name="search" placeholder="Search..." />
 */
export const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(
	(
		{
			value = '',
			onChange,
			onClear,
			className = '',
			placeholder = 'Search...',
			disabled,
			...props
		},
		ref
	) => {
		const hasValue = value.length > 0;

		const handleChange = useCallback(
			(e: ChangeEvent<HTMLInputElement>) => {
				onChange?.(e.target.value);
			},
			[onChange]
		);

		const handleClear = useCallback(() => {
			onChange?.('');
			onClear?.();
		}, [onChange, onClear]);

		const handleKeyDown = useCallback(
			(e: KeyboardEvent<HTMLInputElement>) => {
				if (e.key === 'Escape' && hasValue) {
					e.preventDefault();
					handleClear();
				}
			},
			[hasValue, handleClear]
		);

		const wrapperClasses = ['search-input', className]
			.filter(Boolean)
			.join(' ');

		return (
			<div className={wrapperClasses}>
				<span className="search-input__icon" aria-hidden="true">
					<svg
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						strokeWidth="2"
					>
						<circle cx="11" cy="11" r="8" />
						<path d="m21 21-4.35-4.35" />
					</svg>
				</span>
				<input
					ref={ref}
					type="text"
					className="input search-input__input"
					value={value}
					onChange={handleChange}
					onKeyDown={handleKeyDown}
					placeholder={placeholder}
					disabled={disabled}
					{...props}
				/>
				<button
					type="button"
					className={`search-input__clear ${hasValue ? 'search-input__clear--visible' : ''}`}
					onClick={handleClear}
					disabled={disabled || !hasValue}
					tabIndex={hasValue ? 0 : -1}
					aria-label="Clear search"
				>
					<svg
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						strokeWidth="2"
					>
						<path d="M18 6 6 18" />
						<path d="m6 6 12 12" />
					</svg>
				</button>
			</div>
		);
	}
);

SearchInput.displayName = 'SearchInput';
