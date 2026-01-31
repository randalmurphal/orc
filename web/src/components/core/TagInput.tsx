/**
 * TagInput - Chip-style tag input component.
 *
 * Allows adding tags via Enter/comma, displays as removable chips,
 * and supports backspace-to-remove-last.
 */

import { useState, useCallback, useRef, forwardRef, type KeyboardEvent, type ChangeEvent } from 'react';
import './TagInput.css';

export interface TagInputProps {
	/** Current list of tags */
	tags: string[];
	/** Callback when tags change */
	onChange: (tags: string[]) => void;
	/** Placeholder text for the input */
	placeholder?: string;
	/** Whether the input is disabled */
	disabled?: boolean;
}

export const TagInput = forwardRef<HTMLInputElement, TagInputProps>(
	({ tags, onChange, placeholder, disabled = false }, ref) => {
		const [inputValue, setInputValue] = useState('');
		const internalRef = useRef<HTMLInputElement>(null);
		const inputRef = (ref as React.RefObject<HTMLInputElement>) || internalRef;

		const addTag = useCallback(
			(value: string) => {
				const trimmed = value.trim();
				if (!trimmed) return;
				if (tags.includes(trimmed)) return;
				onChange([...tags, trimmed]);
			},
			[tags, onChange]
		);

		const handleKeyDown = useCallback(
			(e: KeyboardEvent<HTMLInputElement>) => {
				if (e.key === 'Enter') {
					e.preventDefault();
					addTag(inputValue);
					setInputValue('');
				} else if (e.key === 'Backspace' && inputValue === '' && tags.length > 0) {
					onChange(tags.slice(0, -1));
				}
			},
			[inputValue, tags, onChange, addTag]
		);

		const handleChange = useCallback(
			(e: ChangeEvent<HTMLInputElement>) => {
				const value = e.target.value;
				if (value.includes(',')) {
					const parts = value.split(',');
					for (const part of parts) {
						addTag(part);
					}
					setInputValue('');
				} else {
					setInputValue(value);
				}
			},
			[addTag]
		);

		const removeTag = useCallback(
			(index: number) => {
				onChange(tags.filter((_, i) => i !== index));
			},
			[tags, onChange]
		);

		return (
			<div className="tag-input">
				<div className="tag-input__chips">
					{tags.map((tag, index) => (
						<span key={tag} className="tag-input__chip" data-tag={tag} title={tag}>
							<span className="tag-input__chip-text">{tag}</span>
							{!disabled && (
								<button
									type="button"
									className="tag-input__chip-remove"
									onClick={() => removeTag(index)}
									aria-label={`Remove ${tag}`}
								>
									×
								</button>
							)}
						</span>
					))}
				</div>
				<input
					ref={inputRef}
					type="text"
					className="tag-input__input"
					value={inputValue}
					onChange={handleChange}
					onKeyDown={handleKeyDown}
					placeholder={placeholder}
					disabled={disabled}
				/>
			</div>
		);
	}
);

TagInput.displayName = 'TagInput';
