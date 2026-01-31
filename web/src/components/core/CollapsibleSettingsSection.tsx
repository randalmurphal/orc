/**
 * CollapsibleSettingsSection - Collapsible section with header badge and chevron.
 *
 * Used in EditPhaseTemplateModal for claude_config settings sections.
 * Renders collapsed by default, expands on click, shows badge count of active items.
 */

import { useState, useCallback, type ReactNode } from 'react';
import './CollapsibleSettingsSection.css';

export interface CollapsibleSettingSectionProps {
	/** Section title displayed in the header */
	title: string;
	/** Number of active items (shown as badge) */
	badgeCount: number;
	/** Custom badge text (overrides badgeCount display, always shown) */
	badgeText?: string;
	/** Section content (shown when expanded) */
	children: ReactNode;
	/** Whether the section starts expanded */
	defaultExpanded?: boolean;
	/** Whether the section is disabled (for built-in templates) */
	disabled?: boolean;
}

export function CollapsibleSettingsSection({
	title,
	badgeCount,
	badgeText,
	children,
	defaultExpanded = false,
	disabled = false,
}: CollapsibleSettingSectionProps) {
	const [expanded, setExpanded] = useState(defaultExpanded);

	const handleToggle = useCallback(() => {
		if (!disabled) {
			setExpanded((prev) => !prev);
		}
	}, [disabled]);

	return (
		<div
			className={`settings-section ${disabled ? 'settings-section--disabled' : ''}`}
			data-testid="collapsible-section"
		>
			<button
				type="button"
				className="settings-section__header"
				onClick={handleToggle}
				disabled={disabled}
				aria-expanded={expanded}
			>
				<span className={`settings-section__chevron ${expanded ? 'settings-section__chevron--expanded' : ''}`}>
					▸
				</span>
				<span className="settings-section__title">{title}</span>
				{!expanded && (badgeText !== undefined ? (
					<span className="settings-section__badge">{badgeText}</span>
				) : badgeCount > 0 ? (
					<span className="settings-section__badge">{badgeCount}</span>
				) : null)}
			</button>
			{expanded && (
				<div className="settings-section__body">
					{children}
				</div>
			)}
		</div>
	);
}
