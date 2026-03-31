import { useCallback, useRef } from 'react';

interface PromptEditorProps {
	value: string;
	onChange: (value: string) => void;
	placeholder?: string;
}

export function PromptEditor({ value, onChange, placeholder }: PromptEditorProps) {
	const textareaRef = useRef<HTMLTextAreaElement>(null);
	const highlightRef = useRef<HTMLDivElement>(null);

	const renderHighlightedText = useCallback((text: string) => {
		const parts: React.ReactNode[] = [];
		const regex = /(\{\{[A-Z_][A-Z0-9_]*\}\})/g;
		let lastIndex = 0;
		let match: RegExpExecArray | null;

		while ((match = regex.exec(text)) !== null) {
			if (match.index > lastIndex) {
				parts.push(
					<span key={`text-${lastIndex}`} className="prompt-editor-text">
						{text.slice(lastIndex, match.index)}
					</span>,
				);
			}
			parts.push(
				<span
					key={`var-${match.index}`}
					className="prompt-editor-highlight variable-highlight"
					data-variable-highlight
				>
					{match[1]}
				</span>,
			);
			lastIndex = regex.lastIndex;
		}

		if (lastIndex < text.length) {
			parts.push(
				<span key={`text-${lastIndex}`} className="prompt-editor-text">
					{text.slice(lastIndex)}
				</span>,
			);
		}

		return parts;
	}, []);

	return (
		<div className="prompt-editor">
			<div ref={highlightRef} className="prompt-editor__highlight-overlay" aria-hidden="true">
				{renderHighlightedText(value)}
			</div>
			<textarea
				ref={textareaRef}
				className="prompt-editor__textarea"
				value={value}
				onChange={(event) => onChange(event.target.value)}
				onScroll={() => {
					if (!textareaRef.current || !highlightRef.current) {
						return;
					}
					highlightRef.current.scrollTop = textareaRef.current.scrollTop;
					highlightRef.current.scrollLeft = textareaRef.current.scrollLeft;
				}}
				placeholder={placeholder || 'Enter your prompt template...'}
				aria-label="Prompt Content"
				rows={8}
			/>
		</div>
	);
}
