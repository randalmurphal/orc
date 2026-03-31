import { useCallback, useRef, useState } from 'react';
import { VARIABLE_SUGGESTIONS } from './constants';

interface VariableTagInputProps {
	tags: string[];
	onChange: (tags: string[]) => void;
	suggestions?: string[];
	placeholder?: string;
}

export function VariableTagInput({
	tags,
	onChange,
	suggestions = VARIABLE_SUGGESTIONS,
	placeholder,
}: VariableTagInputProps) {
	const [inputValue, setInputValue] = useState('');
	const [showSuggestions, setShowSuggestions] = useState(false);
	const inputRef = useRef<HTMLInputElement>(null);

	const filteredSuggestions = suggestions.filter(
		(suggestion) =>
			!tags.includes(suggestion) &&
			suggestion.toLowerCase().includes(inputValue.toLowerCase()),
	);

	const addTag = useCallback(
		(value: string) => {
			const trimmed = value.trim().toUpperCase();
			if (!trimmed || tags.includes(trimmed)) {
				return;
			}
			onChange([...tags, trimmed]);
			setInputValue('');
		},
		[onChange, tags],
	);

	const removeTag = useCallback(
		(index: number) => {
			onChange(tags.filter((_, currentIndex) => currentIndex !== index));
		},
		[onChange, tags],
	);

	return (
		<div className="variable-tag-input">
			<div className="variable-tag-input__chips">
				{tags.map((tag, index) => (
					<span key={tag} className="variable-tag-input__chip" data-tag={tag}>
						<span className="label-text">{tag}</span>
						<button
							type="button"
							className="variable-tag-input__chip-remove"
							onClick={() => removeTag(index)}
							aria-label={`Remove ${tag}`}
						>
							×
						</button>
					</span>
				))}
			</div>
			<div className="variable-tag-input__input-wrapper">
				<input
					ref={inputRef}
					type="text"
					className="variable-tag-input__input"
					value={inputValue}
					onChange={(event) => {
						setInputValue(event.target.value);
						setShowSuggestions(true);
					}}
					onKeyDown={(event) => {
						if (event.key === 'Enter') {
							event.preventDefault();
							addTag(inputValue);
							return;
						}
						if (event.key === 'Backspace' && inputValue === '' && tags.length > 0) {
							onChange(tags.slice(0, -1));
							return;
						}
						if (event.key === 'Escape') {
							setShowSuggestions(false);
						}
					}}
					onFocus={() => setShowSuggestions(true)}
					onBlur={() => setTimeout(() => setShowSuggestions(false), 200)}
					placeholder={placeholder || 'Add variable...'}
					aria-label="Input Variables"
				/>
				{showSuggestions && filteredSuggestions.length > 0 && (
					<ul className="variable-tag-input__suggestions" role="listbox">
						{filteredSuggestions.map((suggestion) => (
							<li
								key={suggestion}
								role="option"
								aria-selected={false}
								className="variable-tag-input__suggestion"
								onMouseDown={(event) => {
									event.preventDefault();
									addTag(suggestion);
									setShowSuggestions(false);
									inputRef.current?.focus();
								}}
							>
								{suggestion}
							</li>
						))}
					</ul>
				)}
			</div>
		</div>
	);
}
