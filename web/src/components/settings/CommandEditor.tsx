/**
 * CommandEditor component - markdown editor for editing slash command files.
 * Features syntax highlighting overlay, line numbers, dirty state tracking,
 * and keyboard shortcuts (Ctrl+S to save, Escape to cancel).
 */

import {
	type ChangeEvent,
	type KeyboardEvent,
	useCallback,
	useEffect,
	useMemo,
	useRef,
	useState,
} from 'react';
import { Button } from '../ui/Button';
import { Icon } from '../ui/Icon';
import './CommandEditor.css';

export interface EditableCommand {
	id: string;
	name: string;
	path: string;
	content: string;
}

export interface CommandEditorProps {
	command: EditableCommand;
	onSave: (content: string) => Promise<void>;
	onCancel: () => void;
}

/**
 * Apply syntax highlighting to markdown content.
 * Returns HTML with spans for different markdown elements.
 */
function highlightMarkdown(content: string): string {
	// Escape HTML first to prevent XSS
	let html = content
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;');

	// H1 headers (# only) - muted comment style
	html = html.replace(
		/^(#\s.*)$/gm,
		'<span class="code-comment">$1</span>'
	);

	// H2-H6 headers (## through ######) - key/keyword style
	html = html.replace(
		/^(#{2,6}\s.*)$/gm,
		'<span class="code-key">$1</span>'
	);

	// Inline code (backticks)
	html = html.replace(
		/`([^`\n]+)`/g,
		'<span class="code-string">`$1`</span>'
	);

	// Code blocks (triple backticks) - preserve the block structure
	html = html.replace(
		/^(```[\s\S]*?```)$/gm,
		'<span class="md-code-block">$1</span>'
	);

	// Bold (**text** or __text__)
	html = html.replace(
		/(\*\*|__)([^*_]+)\1/g,
		'<span class="md-bold">$1$2$1</span>'
	);

	// Italic (*text* or _text_) - avoid matching bold markers
	html = html.replace(
		/(?<!\*)(\*)(?!\*)([^*\n]+)(?<!\*)(\*)(?!\*)/g,
		'<span class="md-italic">$1$2$3</span>'
	);
	html = html.replace(
		/(?<!_)(_)(?!_)([^_\n]+)(?<!_)(_)(?!_)/g,
		'<span class="md-italic">$1$2$3</span>'
	);

	// Lists (-, *, or numbered)
	html = html.replace(
		/^(\s*[-*+]\s)/gm,
		'<span class="md-list">$1</span>'
	);
	html = html.replace(
		/^(\s*\d+\.\s)/gm,
		'<span class="md-list">$1</span>'
	);

	return html;
}

/**
 * Generate line numbers for the given content.
 */
function getLineCount(content: string): number {
	return content.split('\n').length;
}

export function CommandEditor({ command, onSave, onCancel }: CommandEditorProps) {
	const [content, setContent] = useState(command.content);
	const [isSaving, setIsSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const textareaRef = useRef<HTMLTextAreaElement>(null);
	const highlightRef = useRef<HTMLPreElement>(null);

	// Determine if content has been modified
	const isDirty = content !== command.content;

	// Highlighted content for the overlay
	const highlightedContent = useMemo(() => highlightMarkdown(content), [content]);

	// Line count for line numbers
	const lineCount = useMemo(() => getLineCount(content), [content]);

	// Sync scroll between textarea and highlight overlay
	const handleScroll = useCallback(() => {
		if (textareaRef.current && highlightRef.current) {
			highlightRef.current.scrollTop = textareaRef.current.scrollTop;
			highlightRef.current.scrollLeft = textareaRef.current.scrollLeft;
		}
	}, []);

	// Handle content changes
	const handleChange = useCallback((e: ChangeEvent<HTMLTextAreaElement>) => {
		setContent(e.target.value);
		setError(null);
	}, []);

	// Handle save
	const handleSave = useCallback(async () => {
		if (isSaving) return;

		setIsSaving(true);
		setError(null);

		try {
			await onSave(content);
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Failed to save command';
			setError(message);
		} finally {
			setIsSaving(false);
		}
	}, [content, isSaving, onSave]);

	// Handle keyboard shortcuts
	const handleKeyDown = useCallback(
		(e: KeyboardEvent) => {
			if (e.key === 's' && (e.ctrlKey || e.metaKey)) {
				e.preventDefault();
				handleSave();
			} else if (e.key === 'Escape') {
				e.preventDefault();
				onCancel();
			}
		},
		[handleSave, onCancel]
	);

	// Focus textarea on mount
	useEffect(() => {
		textareaRef.current?.focus();
	}, []);

	// Calculate dynamic height based on content
	const minHeight = 300;
	const lineHeight = 20; // approximate line height
	const calculatedHeight = Math.max(minHeight, lineCount * lineHeight + 40);
	const maxHeight = 600;
	const editorHeight = Math.min(calculatedHeight, maxHeight);

	return (
		<div className="editor-container">
			<div className="editor-header">
				<div className="editor-header-left">
					<div className="editor-command-name">
						<Icon name="terminal" size={16} />
						<span>{command.name}</span>
					</div>
					<div className="editor-path">{command.path}</div>
				</div>
				<div className="editor-header-right">
					{isDirty && (
						<span className="editor-dirty" aria-label="Unsaved changes">
							<Icon name="circle" size={8} />
							<span>Unsaved</span>
						</span>
					)}
					<Button
						variant="ghost"
						size="sm"
						onClick={onCancel}
						aria-label="Cancel editing"
					>
						Cancel
					</Button>
					<Button
						variant="primary"
						size="sm"
						onClick={handleSave}
						loading={isSaving}
						disabled={isSaving}
						aria-label="Save command"
					>
						Save
					</Button>
				</div>
			</div>

			{error && (
				<div className="editor-error" role="alert">
					<Icon name="alert-circle" size={14} />
					<span>{error}</span>
				</div>
			)}

			<div className="editor-body" style={{ height: editorHeight }}>
				<div className="editor-line-numbers" aria-hidden="true">
					{Array.from({ length: lineCount }, (_, i) => (
						<div key={i + 1} className="editor-line-number">
							{i + 1}
						</div>
					))}
				</div>
				<div className="editor-content">
					<pre
						ref={highlightRef}
						className="editor-highlight"
						aria-hidden="true"
						dangerouslySetInnerHTML={{ __html: highlightedContent + '\n' }}
					/>
					<textarea
						ref={textareaRef}
						className="editor-textarea"
						value={content}
						onChange={handleChange}
						onScroll={handleScroll}
						onKeyDown={handleKeyDown}
						spellCheck={false}
						aria-label={`Edit ${command.name}`}
					/>
				</div>
			</div>
		</div>
	);
}
