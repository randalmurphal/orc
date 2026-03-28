import { useState, useEffect, useCallback, useRef } from 'react';
import { useCurrentProjectId } from '@/stores';
import { DiscussionPanel } from './DiscussionPanel';
import './ContextPanel.css';

export type ContextPanelMode = 'discussion' | 'diff' | 'terminal' | 'knowledge' | 'task';

const MODE_TABS: { id: ContextPanelMode; label: string }[] = [
	{ id: 'discussion', label: 'Discussion' },
	{ id: 'diff', label: 'Diff' },
	{ id: 'terminal', label: 'Terminal' },
	{ id: 'knowledge', label: 'Knowledge' },
	{ id: 'task', label: 'Task' },
];

const WIDTH_STORAGE_KEY = 'orc-context-panel-width';
const DEFAULT_WIDTH = 360;
const MIN_WIDTH = 280;

function getStoredWidth(): number {
	if (typeof window === 'undefined') return DEFAULT_WIDTH;
	try {
		const stored = localStorage.getItem(WIDTH_STORAGE_KEY);
		if (!stored) return DEFAULT_WIDTH;
		const parsed = parseInt(stored, 10);
		if (isNaN(parsed) || parsed < MIN_WIDTH) return DEFAULT_WIDTH;
		return parsed;
	} catch {
		return DEFAULT_WIDTH;
	}
}

interface ContextPanelProps {
	mode?: ContextPanelMode;
	onModeChange?: (mode: ContextPanelMode) => void;
	threadId?: string;
}

export function ContextPanel({ mode, onModeChange, threadId }: ContextPanelProps) {
	const projectId = useCurrentProjectId();
	const [width, setWidth] = useState(getStoredWidth);
	const isDragging = useRef(false);
	const startX = useRef(0);
	const startWidth = useRef(0);

	const handleMouseDown = useCallback((e: React.MouseEvent) => {
		isDragging.current = true;
		startX.current = e.clientX;
		startWidth.current = width;
		e.preventDefault();
	}, [width]);

	useEffect(() => {
		const handleMouseMove = (e: MouseEvent) => {
			if (!isDragging.current) return;
			const delta = startX.current - e.clientX;
			const newWidth = Math.max(MIN_WIDTH, startWidth.current + delta);
			setWidth(newWidth);
		};

		const handleMouseUp = () => {
			if (!isDragging.current) return;
			isDragging.current = false;
			try {
				localStorage.setItem(WIDTH_STORAGE_KEY, String(width));
			} catch {
				// Ignore
			}
		};

		document.addEventListener('mousemove', handleMouseMove);
		document.addEventListener('mouseup', handleMouseUp);
		return () => {
			document.removeEventListener('mousemove', handleMouseMove);
			document.removeEventListener('mouseup', handleMouseUp);
		};
	}, [width]);

	const renderContent = () => {
		if (!mode) {
			return (
				<div className="context-panel__empty">
					Select a thread or action
				</div>
			);
		}

		if (mode === 'discussion') {
			if (!threadId) {
				return (
					<div className="context-panel__empty">
						Select a thread to start a discussion
					</div>
				);
			}
			return (
				<DiscussionPanel
					key={threadId}
					threadId={threadId}
					projectId={projectId ?? ''}
				/>
			);
		}

		return (
			<div className="context-panel__placeholder">
				{mode.charAt(0).toUpperCase() + mode.slice(1)} panel coming soon
			</div>
		);
	};

	return (
		<aside
			className="context-panel"
			role="complementary"
			aria-label="Context Panel"
			style={{ width: `${width}px` }}
		>
			<div
				className="context-panel__resize-handle"
				onMouseDown={handleMouseDown}
			/>

			<div className="context-panel__tabs" role="tablist">
				{MODE_TABS.map((tab) => (
					<button
						key={tab.id}
						role="tab"
						aria-selected={mode === tab.id}
						className={`context-panel__tab${mode === tab.id ? ' context-panel__tab--active' : ''}`}
						onClick={() => onModeChange?.(tab.id)}
					>
						{tab.label}
					</button>
				))}
			</div>

			<div className="context-panel__content">
				{renderContent()}
			</div>
		</aside>
	);
}
