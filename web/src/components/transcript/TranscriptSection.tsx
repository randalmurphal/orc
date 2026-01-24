/**
 * TranscriptSection - A collapsible content block for the transcript viewer.
 *
 * Displays structured transcript content with expand/collapse functionality.
 * Supports different section types: phase, iteration, prompt, response, tool_call, error.
 */

import { useState, useCallback, type ReactNode, type KeyboardEvent } from 'react';
import { Icon, type IconName } from '@/components/ui/Icon';
import './TranscriptSection.css';

export type TranscriptSectionType =
	| 'phase'
	| 'iteration'
	| 'prompt'
	| 'response'
	| 'tool_call'
	| 'tool_result'
	| 'error'
	| 'system';

export interface TranscriptSectionProps {
	/** Section type determines styling and icon */
	type: TranscriptSectionType;
	/** Section title displayed in the header */
	title: string;
	/** Optional subtitle/metadata displayed next to title */
	subtitle?: string;
	/** Optional timestamp to display */
	timestamp?: string;
	/** Optional badge content (e.g., token count, status) */
	badge?: ReactNode;
	/** Section content (children) */
	children: ReactNode;
	/** Whether the section is initially expanded */
	defaultExpanded?: boolean;
	/** Controlled expanded state (makes component controlled) */
	expanded?: boolean;
	/** Callback when expand state changes */
	onExpandedChange?: (expanded: boolean) => void;
	/** Optional additional CSS class */
	className?: string;
	/** Optional test id for testing */
	testId?: string;
	/** Nesting depth for visual indentation */
	depth?: number;
}

// Configuration for section types
interface SectionConfig {
	icon: IconName;
	accentColor: string;
}

const SECTION_CONFIG: Record<TranscriptSectionType, SectionConfig> = {
	phase: {
		icon: 'layers',
		accentColor: 'var(--primary)',
	},
	iteration: {
		icon: 'rotate-ccw',
		accentColor: 'var(--cyan)',
	},
	prompt: {
		icon: 'user',
		accentColor: 'var(--primary)',
	},
	response: {
		icon: 'message-square',
		accentColor: 'var(--green)',
	},
	tool_call: {
		icon: 'terminal',
		accentColor: 'var(--amber)',
	},
	tool_result: {
		icon: 'file-text',
		accentColor: 'var(--orange)',
	},
	error: {
		icon: 'alert-triangle',
		accentColor: 'var(--red)',
	},
	system: {
		icon: 'settings',
		accentColor: 'var(--text-muted)',
	},
};

/**
 * TranscriptSection component for collapsible content blocks.
 *
 * @example
 * // Basic usage
 * <TranscriptSection type="phase" title="implement">
 *   <p>Phase content here</p>
 * </TranscriptSection>
 *
 * @example
 * // With all props
 * <TranscriptSection
 *   type="tool_call"
 *   title="Read"
 *   subtitle="file.ts"
 *   timestamp="12:34"
 *   badge={<span>150 tokens</span>}
 *   defaultExpanded={false}
 * >
 *   <pre>{toolOutput}</pre>
 * </TranscriptSection>
 *
 * @example
 * // Controlled mode
 * <TranscriptSection
 *   type="response"
 *   title="Assistant"
 *   expanded={isOpen}
 *   onExpandedChange={setIsOpen}
 * >
 *   {content}
 * </TranscriptSection>
 */
export function TranscriptSection({
	type,
	title,
	subtitle,
	timestamp,
	badge,
	children,
	defaultExpanded = true,
	expanded: controlledExpanded,
	onExpandedChange,
	className = '',
	testId,
	depth = 0,
}: TranscriptSectionProps) {
	// Internal expanded state for uncontrolled mode
	const [internalExpanded, setInternalExpanded] = useState(defaultExpanded);

	// Use controlled value if provided, otherwise use internal state
	const isControlled = controlledExpanded !== undefined;
	const isExpanded = isControlled ? controlledExpanded : internalExpanded;

	const config = SECTION_CONFIG[type];

	const toggleExpanded = useCallback(() => {
		const newValue = !isExpanded;
		if (!isControlled) {
			setInternalExpanded(newValue);
		}
		onExpandedChange?.(newValue);
	}, [isExpanded, isControlled, onExpandedChange]);

	const handleKeyDown = useCallback(
		(e: KeyboardEvent<HTMLButtonElement>) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				toggleExpanded();
			}
		},
		[toggleExpanded]
	);

	// Build class names
	const sectionClasses = [
		'transcript-section',
		`transcript-section--${type}`,
		isExpanded && 'transcript-section--expanded',
		depth > 0 && `transcript-section--depth-${Math.min(depth, 3)}`,
		className,
	]
		.filter(Boolean)
		.join(' ');

	// CSS custom property for accent color
	const style = {
		'--section-accent': config.accentColor,
	} as React.CSSProperties;

	return (
		<section
			className={sectionClasses}
			style={style}
			data-testid={testId}
			data-type={type}
			data-expanded={isExpanded}
		>
			<button
				type="button"
				className="transcript-section-header"
				onClick={toggleExpanded}
				onKeyDown={handleKeyDown}
				aria-expanded={isExpanded}
				aria-controls={`section-content-${title.replace(/\s+/g, '-').toLowerCase()}`}
			>
				{/* Expand/Collapse chevron */}
				<span className="transcript-section-chevron">
					<Icon name={isExpanded ? 'chevron-down' : 'chevron-right'} size={14} />
				</span>

				{/* Section icon */}
				<span className="transcript-section-icon">
					<Icon name={config.icon} size={14} />
				</span>

				{/* Title and subtitle */}
				<span className="transcript-section-title">{title}</span>
				{subtitle && <span className="transcript-section-subtitle">{subtitle}</span>}

				{/* Right side: badge and timestamp */}
				<span className="transcript-section-meta">
					{badge && <span className="transcript-section-badge">{badge}</span>}
					{timestamp && <span className="transcript-section-timestamp">{timestamp}</span>}
				</span>
			</button>

			{/* Content - only rendered when expanded */}
			{isExpanded && (
				<div
					className="transcript-section-content"
					id={`section-content-${title.replace(/\s+/g, '-').toLowerCase()}`}
				>
					{children}
				</div>
			)}
		</section>
	);
}
