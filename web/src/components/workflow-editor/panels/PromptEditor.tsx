import { useState, useEffect, useRef, useCallback } from 'react';
import { workflowClient } from '@/lib/client';
import { PromptSource } from '@/gen/orc/v1/workflow_pb';
import './PromptEditor.css';

interface PromptEditorProps {
	phaseTemplateId: string;
	promptSource: PromptSource;
	promptContent?: string;
	readOnly: boolean;
}

type SaveStatus = 'idle' | 'saving' | 'saved' | 'error';

/**
 * Split prompt text into segments of plain text and {{VARIABLE}} badges.
 * Only matches uppercase variable names: {{[A-Z_][A-Z0-9_]*}}
 */
function parsePromptSegments(
	text: string,
): Array<{ type: 'text' | 'variable'; value: string }> {
	const segments: Array<{ type: 'text' | 'variable'; value: string }> = [];
	const regex = /\{\{([A-Z_][A-Z0-9_]*)\}\}/g;
	let lastIndex = 0;
	let match: RegExpExecArray | null;

	while ((match = regex.exec(text)) !== null) {
		if (match.index > lastIndex) {
			segments.push({ type: 'text', value: text.slice(lastIndex, match.index) });
		}
		segments.push({ type: 'variable', value: match[1] });
		lastIndex = match.index + match[0].length;
	}

	if (lastIndex < text.length) {
		segments.push({ type: 'text', value: text.slice(lastIndex) });
	}

	return segments;
}

/** Renders prompt content with {{VAR}} patterns highlighted as badges */
function HighlightedPrompt({ content }: { content: string }) {
	const segments = parsePromptSegments(content);

	return (
		<div className="prompt-editor-highlighted">
			{segments.map((seg, i) =>
				seg.type === 'variable' ? (
					<span key={i} className="prompt-editor-variable-badge" data-testid="variable-badge">
						{seg.value}
					</span>
				) : (
					<span key={i}>{seg.value}</span>
				),
			)}
		</div>
	);
}

export function PromptEditor({
	phaseTemplateId,
	promptSource,
	promptContent,
	readOnly,
}: PromptEditorProps) {
	const [fetchedContent, setFetchedContent] = useState<string | null>(null);
	const [loading, setLoading] = useState(false);
	const [fetchError, setFetchError] = useState<string | null>(null);
	const [editValue, setEditValue] = useState(promptContent ?? '');
	const [saveStatus, setSaveStatus] = useState<SaveStatus>('idle');
	const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
	const fetchIdRef = useRef(0);

	// Reset edit value when promptContent prop changes
	useEffect(() => {
		setEditValue(promptContent ?? '');
	}, [promptContent]);

	const fetchContent = useCallback(async () => {
		const fetchId = ++fetchIdRef.current;
		setLoading(true);
		setFetchError(null);
		try {
			const response = await workflowClient.getPromptContent({
				phaseTemplateId,
			});
			if (fetchId === fetchIdRef.current) {
				setFetchedContent(response.content);
			}
		} catch {
			if (fetchId === fetchIdRef.current) {
				setFetchError('Failed to load prompt content');
			}
		} finally {
			if (fetchId === fetchIdRef.current) {
				setLoading(false);
			}
		}
	}, [phaseTemplateId]);

	// Fetch for FILE source
	useEffect(() => {
		if (promptSource === PromptSource.FILE) {
			fetchContent();
		}
		const ref = fetchIdRef;
		return () => {
			// Invalidate any in-flight fetch on unmount or phaseTemplateId change
			ref.current++;
		};
	}, [promptSource, fetchContent]);

	// Cleanup debounce on unmount
	useEffect(() => {
		return () => {
			if (debounceRef.current) {
				clearTimeout(debounceRef.current);
			}
		};
	}, []);

	// Determine resolved content
	const resolvedContent =
		promptSource === PromptSource.FILE ? fetchedContent : (promptContent ?? '');

	// Handle loading state for FILE source
	if (promptSource === PromptSource.FILE && loading) {
		return (
			<div className="prompt-editor prompt-editor-loading">
				<span>Loading prompt content...</span>
			</div>
		);
	}

	// Handle error state for FILE source
	if (promptSource === PromptSource.FILE && fetchError) {
		return (
			<div className="prompt-editor prompt-editor-error-state">
				<span>{fetchError}</span>
				<button
					className="prompt-editor-retry-btn"
					onClick={() => fetchContent()}
				>
					Retry
				</button>
			</div>
		);
	}

	// Editable mode for custom templates - show textarea even if empty
	if (!readOnly) {
		const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
			const newValue = e.target.value;
			setEditValue(newValue);
			setSaveStatus('idle');

			if (debounceRef.current) {
				clearTimeout(debounceRef.current);
			}

			debounceRef.current = setTimeout(async () => {
				setSaveStatus('saving');
				try {
					await workflowClient.updatePhaseTemplate({
						id: phaseTemplateId,
						promptContent: newValue,
					});
					setSaveStatus('saved');
				} catch {
					setSaveStatus('error');
				}
			}, 500);
		};

		return (
			<div className="prompt-editor prompt-editor-editable">
				<textarea
					className="prompt-editor-textarea"
					value={editValue}
					onChange={handleChange}
				/>
				<div className="prompt-editor-save-status">
					{saveStatus === 'saving' && <span>Saving...</span>}
					{saveStatus === 'saved' && <span>Saved</span>}
					{saveStatus === 'error' && <span>Save failed</span>}
				</div>
			</div>
		);
	}

	// Handle empty content (read-only mode only)
	if (
		resolvedContent === '' ||
		resolvedContent === null ||
		resolvedContent === undefined
	) {
		return (
			<div className="prompt-editor prompt-editor-empty">
				<span>No prompt content configured</span>
			</div>
		);
	}

	// Read-only mode for built-in templates
	return (
		<div className="prompt-editor prompt-editor-readonly">
			<HighlightedPrompt content={resolvedContent} />
			<button className="prompt-editor-clone-btn">Clone Template</button>
		</div>
	);
}
