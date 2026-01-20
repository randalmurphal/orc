/**
 * ConfigEditor component - editable code/config file viewer with syntax highlighting.
 * Supports markdown, YAML, and JSON syntax highlighting with save functionality
 * and unsaved changes detection.
 */

import {
	type ChangeEvent,
	type KeyboardEvent,
	useCallback,
	useMemo,
	useRef,
	useState,
} from 'react';
import { Button } from '../ui/Button';
import { Icon } from '../ui/Icon';
import './ConfigEditor.css';

export type ConfigLanguage = 'markdown' | 'yaml' | 'json';

export interface ConfigEditorProps {
	filePath: string;
	content: string;
	onChange: (content: string) => void;
	onSave: () => void;
	language?: ConfigLanguage;
}

/**
 * Apply syntax highlighting to content based on language.
 * Returns HTML string with syntax highlighting spans.
 */
function highlightSyntax(content: string, language: ConfigLanguage): string {
	const escapeHtml = (text: string): string => {
		return text
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(/"/g, '&quot;')
			.replace(/'/g, '&#039;');
	};

	const escaped = escapeHtml(content);

	switch (language) {
		case 'markdown':
			return highlightMarkdown(escaped);
		case 'yaml':
			return highlightYaml(escaped);
		case 'json':
			return highlightJson(escaped);
		default:
			return escaped;
	}
}

function highlightMarkdown(content: string): string {
	const lines = content.split('\n');
	return lines
		.map((line) => {
			// Headers: ## Title
			if (/^#+\s/.test(line)) {
				return `<span class="code-key">${line}</span>`;
			}
			// Code blocks: ```
			if (/^```/.test(line)) {
				return `<span class="code-string">${line}</span>`;
			}
			// Full line comments at start of file or section dividers
			if (/^#\s/.test(line) && !/^##/.test(line)) {
				return `<span class="code-comment">${line}</span>`;
			}
			return line;
		})
		.join('\n');
}

function highlightYaml(content: string): string {
	const lines = content.split('\n');
	return lines
		.map((line) => {
			// Comments: # comment
			if (/^\s*#/.test(line)) {
				return `<span class="code-comment">${line}</span>`;
			}
			// Keys: key: or key:value (highlight key and colon)
			const keyMatch = line.match(/^(\s*)([a-zA-Z0-9_-]+)(:)(.*)$/);
			if (keyMatch) {
				const [, indent, key, colon, rest] = keyMatch;
				const highlightedRest = highlightYamlValue(rest);
				return `${indent}<span class="code-key">${key}${colon}</span>${highlightedRest}`;
			}
			return line;
		})
		.join('\n');
}

function highlightYamlValue(value: string): string {
	// Quoted strings
	if (/^\s*["'].*["']\s*$/.test(value)) {
		return `<span class="code-string">${value}</span>`;
	}
	// Inline comments
	const commentMatch = value.match(/^(.*)(\s+#.*)$/);
	if (commentMatch) {
		const [, val, comment] = commentMatch;
		return `${val}<span class="code-comment">${comment}</span>`;
	}
	return value;
}

function highlightJson(content: string): string {
	// First pass: Find all "key": "value" pairs and wrap the value
	// Pattern: "key": "value" where value comes after the colon
	let result = content.replace(
		/(&quot;)([^&]+)(&quot;)(\s*:\s*)(&quot;)([^&]*)(&quot;)/g,
		'<span class="code-key">$1$2$3$4</span><span class="code-string">$5$6$7</span>'
	);
	// Second pass: Handle "key": without string value (numbers, booleans, etc.)
	// Match quoted keys followed by colon that weren't already handled
	result = result.replace(
		/(&quot;)([^&]+)(&quot;)(\s*:)(?!<\/span>)/g,
		'<span class="code-key">$1$2$3$4</span>'
	);
	return result;
}

export function ConfigEditor({
	filePath,
	content,
	onChange,
	onSave,
	language = 'markdown',
}: ConfigEditorProps) {
	// Track the initial content from when the component first mounts
	const [initialContent] = useState(content);
	const textareaRef = useRef<HTMLTextAreaElement>(null);
	const highlightRef = useRef<HTMLDivElement>(null);

	// Track if content has been modified from initial state
	const isUnsaved = content !== initialContent;

	// Sync scroll position between textarea and highlight div
	const handleScroll = useCallback(() => {
		if (textareaRef.current && highlightRef.current) {
			highlightRef.current.scrollTop = textareaRef.current.scrollTop;
			highlightRef.current.scrollLeft = textareaRef.current.scrollLeft;
		}
	}, []);

	const handleChange = useCallback(
		(e: ChangeEvent<HTMLTextAreaElement>) => {
			onChange(e.target.value);
		},
		[onChange]
	);

	const handleKeyDown = useCallback(
		(e: KeyboardEvent<HTMLTextAreaElement>) => {
			// Save on Ctrl+S or Cmd+S
			if ((e.ctrlKey || e.metaKey) && e.key === 's') {
				e.preventDefault();
				onSave();
			}
			// Handle Tab key for indentation
			if (e.key === 'Tab') {
				e.preventDefault();
				const textarea = e.currentTarget;
				const start = textarea.selectionStart;
				const end = textarea.selectionEnd;
				const newValue = content.substring(0, start) + '\t' + content.substring(end);
				onChange(newValue);
				// Restore cursor position after React re-render
				requestAnimationFrame(() => {
					textarea.selectionStart = textarea.selectionEnd = start + 1;
				});
			}
		},
		[content, onChange, onSave]
	);

	const handleSaveClick = useCallback(() => {
		onSave();
	}, [onSave]);

	// Memoize highlighted content to avoid re-computation on every render
	const highlightedContent = useMemo(() => {
		return highlightSyntax(content, language);
	}, [content, language]);

	return (
		<div className="config-editor" data-testid="config-editor">
			<div className="config-editor-header">
				<div className="config-editor-header-left">
					<span className="config-editor-path" data-testid="config-editor-path">
						{filePath}
					</span>
					{isUnsaved && (
						<span
							className="config-editor-unsaved"
							data-testid="config-editor-unsaved"
							aria-label="Unsaved changes"
						>
							Modified
						</span>
					)}
				</div>
				<Button
					variant="ghost"
					size="sm"
					onClick={handleSaveClick}
					leftIcon={<Icon name="save" size={12} />}
					aria-label="Save changes"
					data-testid="config-editor-save"
				>
					Save
				</Button>
			</div>
			<div className="config-editor-content">
				<div
					ref={highlightRef}
					className="config-editor-highlight"
					aria-hidden="true"
					dangerouslySetInnerHTML={{ __html: highlightedContent + '\n' }}
				/>
				<textarea
					ref={textareaRef}
					className="config-editor-textarea"
					value={content}
					onChange={handleChange}
					onScroll={handleScroll}
					onKeyDown={handleKeyDown}
					spellCheck={false}
					aria-label={`Edit ${filePath}`}
					data-testid="config-editor-textarea"
				/>
			</div>
		</div>
	);
}
