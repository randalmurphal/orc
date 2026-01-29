import { useState, useMemo, type DragEvent } from 'react';
import { usePhaseTemplates } from '@/stores/workflowStore';
import { useToast } from '@/hooks/useToast';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import type { PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import {
	getCategoryForTemplate,
	filterTemplates,
	CATEGORY_ORDER,
	type CategoryName,
} from './categoryMap';
import './PhaseTemplatePalette.css';

interface PhaseTemplatePaletteProps {
	readOnly: boolean;
	workflowId: string;
}

function formatGateLabel(gt: GateType): string | null {
	switch (gt) {
		case GateType.HUMAN:
			return 'Human';
		case GateType.SKIP:
			return 'Skip';
		default:
			return null;
	}
}

function groupByCategory(templates: PhaseTemplate[]): Map<CategoryName, PhaseTemplate[]> {
	const groups = new Map<CategoryName, PhaseTemplate[]>();
	for (const t of templates) {
		const cat = getCategoryForTemplate(t.id);
		const list = groups.get(cat);
		if (list) {
			list.push(t);
		} else {
			groups.set(cat, [t]);
		}
	}
	return groups;
}

export function PhaseTemplatePalette({ readOnly, workflowId: _workflowId }: PhaseTemplatePaletteProps) {
	const allTemplates = usePhaseTemplates();
	const { toast } = useToast();
	const [query, setQuery] = useState('');
	const [collapsedCategories, setCollapsedCategories] = useState<Set<CategoryName>>(new Set());

	const filtered = useMemo(() => filterTemplates(allTemplates, query), [allTemplates, query]);
	const grouped = useMemo(() => groupByCategory(filtered), [filtered]);

	const toggleCategory = (cat: CategoryName) => {
		setCollapsedCategories((prev) => {
			const next = new Set(prev);
			if (next.has(cat)) {
				next.delete(cat);
			} else {
				next.add(cat);
			}
			return next;
		});
	};

	const handleDragStart = (e: DragEvent<HTMLDivElement>, templateId: string) => {
		if (readOnly) {
			e.preventDefault();
			toast({
				description: 'Clone this workflow to customize it',
			});
			return;
		}
		e.dataTransfer.setData('application/orc-phase-template', templateId);
		e.dataTransfer.effectAllowed = 'copy';
	};

	return (
		<div className="phase-palette">
			{readOnly && (
				<div className="phase-palette-banner">
					Clone to customize
				</div>
			)}
			<div className="phase-palette-search">
				<input
					type="search"
					className="phase-palette-search-input"
					placeholder="Search templates..."
					value={query}
					onChange={(e) => setQuery(e.target.value)}
				/>
			</div>
			<div className="phase-palette-list">
				{CATEGORY_ORDER.map((cat) => {
					const templates = grouped.get(cat);
					if (!templates || templates.length === 0) return null;
					const collapsed = collapsedCategories.has(cat);
					return (
						<div key={cat} className="phase-palette-category">
							<button
								type="button"
								className="phase-palette-category-header"
								onClick={() => toggleCategory(cat)}
								aria-expanded={!collapsed}
							>
								<span className="phase-palette-category-chevron">
									{collapsed ? '▸' : '▾'}
								</span>
								<span>{cat}</span>
								<span className="phase-palette-category-count">
									{templates.length}
								</span>
							</button>
							{!collapsed &&
								templates.map((t) => (
									<div
										key={t.id}
										className={`phase-palette-card${readOnly ? ' cursor-not-allowed' : ''}`}
										data-testid="template-card"
										draggable={!readOnly}
										onDragStart={(e) => handleDragStart(e, t.id)}
									>
										<div className="phase-palette-card-header">
											<span className="phase-palette-card-name">
												{t.name}
											</span>
											<code className="phase-palette-card-id">{t.id}</code>
										</div>
										{t.description && (
											<p className="phase-palette-card-desc">
												{t.description}
											</p>
										)}
										<div className="phase-palette-card-badges">
											{t.modelOverride && (
												<span className="phase-palette-badge phase-palette-badge--model">
													{t.modelOverride}
												</span>
											)}
											{formatGateLabel(t.gateType) && (
												<span className="phase-palette-badge phase-palette-badge--gate">
													{formatGateLabel(t.gateType)}
												</span>
											)}
										</div>
									</div>
								))}
						</div>
					);
				})}
			</div>
		</div>
	);
}
