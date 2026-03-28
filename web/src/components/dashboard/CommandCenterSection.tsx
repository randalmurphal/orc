import { useState, type ReactNode } from 'react';
import { Icon } from '@/components/ui';

export interface CommandCenterSectionProps {
	title: string;
	count: number;
	emptyState: string;
	children?: ReactNode;
	defaultExpanded?: boolean;
}

export function CommandCenterSection({
	title,
	count,
	emptyState,
	children,
	defaultExpanded = true,
}: CommandCenterSectionProps) {
	const [expanded, setExpanded] = useState(defaultExpanded);

	return (
		<section className="command-center-section" aria-label={title}>
			<button
				type="button"
				className="command-center-section__header"
				aria-expanded={expanded}
				onClick={() => setExpanded((current) => !current)}
			>
				<span className="command-center-section__title-group">
					<span className="command-center-section__title">{title}</span>
					<span className="command-center-section__count">{count}</span>
				</span>
				<Icon
					name={expanded ? 'chevron-up' : 'chevron-down'}
					size={16}
					className="command-center-section__chevron"
				/>
			</button>

			{expanded ? (
				count > 0 ? (
					<div className="command-center-section__body">{children}</div>
				) : (
					<div className="command-center-section__empty">{emptyState}</div>
				)
			) : null}
		</section>
	);
}
