/**
 * TranscriptSearch - Search input with navigation controls for transcript search.
 */

import { useCallback, type KeyboardEvent, type ChangeEvent } from 'react';
import { Icon } from '@/components/ui/Icon';
import './TranscriptSearch.css';

export interface TranscriptSearchProps {
	/** Current search value */
	value: string;
	/** Called when search value changes */
	onChange: (value: string) => void;
	/** Number of search results found */
	resultCount: number;
	/** Current result index (-1 if no results) */
	currentIndex: number;
	/** Called to navigate to next result */
	onNext: () => void;
	/** Called to navigate to previous result */
	onPrev: () => void;
}

export function TranscriptSearch({
	value,
	onChange,
	resultCount,
	currentIndex,
	onNext,
	onPrev,
}: TranscriptSearchProps) {
	const handleChange = useCallback(
		(e: ChangeEvent<HTMLInputElement>) => {
			onChange(e.target.value);
		},
		[onChange]
	);

	const handleKeyDown = useCallback(
		(e: KeyboardEvent<HTMLInputElement>) => {
			if (e.key === 'Enter') {
				e.preventDefault();
				if (e.shiftKey) {
					onPrev();
				} else {
					onNext();
				}
			} else if (e.key === 'Escape') {
				e.preventDefault();
				onChange('');
			}
		},
		[onNext, onPrev, onChange]
	);

	const handleClear = useCallback(() => {
		onChange('');
	}, [onChange]);

	return (
		<div className="transcript-search">
			<div className="search-input-container">
				<Icon name="search" size={14} className="search-icon" />
				<input
					type="text"
					className="search-input"
					placeholder="Search transcripts..."
					value={value}
					onChange={handleChange}
					onKeyDown={handleKeyDown}
					aria-label="Search transcripts"
				/>
				{value && (
					<button
						className="search-clear-btn"
						onClick={handleClear}
						title="Clear search"
						aria-label="Clear search"
					>
						<Icon name="x" size={12} />
					</button>
				)}
			</div>

			{value && (
				<div className="search-results">
					<span className="search-count">
						{resultCount > 0 ? `${currentIndex + 1}/${resultCount}` : 'No results'}
					</span>
					<div className="search-nav">
						<button
							className="search-nav-btn"
							onClick={onPrev}
							disabled={resultCount === 0}
							title="Previous result (Shift+Enter)"
							aria-label="Previous result"
						>
							<Icon name="chevron-up" size={14} />
						</button>
						<button
							className="search-nav-btn"
							onClick={onNext}
							disabled={resultCount === 0}
							title="Next result (Enter)"
							aria-label="Next result"
						>
							<Icon name="chevron-down" size={14} />
						</button>
					</div>
				</div>
			)}
		</div>
	);
}
