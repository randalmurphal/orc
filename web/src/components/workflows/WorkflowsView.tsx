/**
 * WorkflowsView container component - displays workflows and phase templates
 * with management actions.
 */

import { useState, useEffect, useCallback } from 'react';
import type { Workflow, PhaseTemplate, DefinitionSource } from '@/gen/orc/v1/workflow_pb';
import { workflowClient } from '@/lib/client';
import { useWorkflowStore } from '@/stores/workflowStore';
import { WorkflowCard } from './WorkflowCard';
import { PhaseTemplateCard } from './PhaseTemplateCard';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import './WorkflowsView.css';

export interface WorkflowsViewProps {
	className?: string;
}

function WorkflowCardSkeleton() {
	return (
		<article className="workflow-card-skeleton" aria-hidden="true">
			<div className="workflow-card-skeleton-header">
				<div className="workflow-card-skeleton-icon" />
				<div className="workflow-card-skeleton-info">
					<div className="workflow-card-skeleton-title" />
					<div className="workflow-card-skeleton-id" />
				</div>
				<div className="workflow-card-skeleton-badge" />
			</div>
			<div className="workflow-card-skeleton-description" />
			<div className="workflow-card-skeleton-stats">
				<div className="workflow-card-skeleton-stat" />
				<div className="workflow-card-skeleton-stat" />
			</div>
		</article>
	);
}

function WorkflowsViewSkeleton() {
	return (
		<div className="workflows-view-grid" aria-busy="true" aria-label="Loading workflows">
			<WorkflowCardSkeleton />
			<WorkflowCardSkeleton />
			<WorkflowCardSkeleton />
		</div>
	);
}

function WorkflowsViewEmpty() {
	return (
		<div className="workflows-view-empty" role="status">
			<div className="workflows-view-empty-icon">
				<Icon name="git-branch" size={32} />
			</div>
			<h2 className="workflows-view-empty-title">No custom workflows</h2>
			<p className="workflows-view-empty-desc">
				Clone a built-in workflow or create a new one to customize your task execution.
			</p>
		</div>
	);
}

interface WorkflowsViewErrorProps {
	error: string;
	onRetry: () => void;
}

function WorkflowsViewError({ error, onRetry }: WorkflowsViewErrorProps) {
	return (
		<div className="workflows-view-error" role="alert">
			<div className="workflows-view-error-icon">
				<Icon name="alert-circle" size={24} />
			</div>
			<h2 className="workflows-view-error-title">Failed to load workflows</h2>
			<p className="workflows-view-error-desc">{error}</p>
			<Button variant="secondary" onClick={onRetry}>
				Retry
			</Button>
		</div>
	);
}

/**
 * WorkflowsView displays all workflows and phase templates.
 */
export function WorkflowsView({ className = '' }: WorkflowsViewProps) {
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [phaseCounts, setPhaseCounts] = useState<Record<string, number>>({});
	const [workflowSources, setWorkflowSources] = useState<Record<string, DefinitionSource>>({});
	const [phaseSources, setPhaseSources] = useState<Record<string, DefinitionSource>>({});
	const { workflows, phaseTemplates, setWorkflows, setPhaseTemplates, refreshKey } = useWorkflowStore();

	const loadData = useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			const [workflowsRes, templatesRes] = await Promise.all([
				workflowClient.listWorkflows({ includeBuiltin: true }),
				workflowClient.listPhaseTemplates({ includeBuiltin: true }),
			]);
			// Convert sources to plain objects
			const wfSources: Record<string, DefinitionSource> = {};
			for (const [key, value] of Object.entries(workflowsRes.sources)) {
				wfSources[key] = value;
			}
			const phSources: Record<string, DefinitionSource> = {};
			for (const [key, value] of Object.entries(templatesRes.sources)) {
				phSources[key] = value;
			}
			setWorkflows(workflowsRes.workflows, wfSources);
			setPhaseTemplates(templatesRes.templates, phSources);
			setWorkflowSources(wfSources);
			setPhaseSources(phSources);
			// Convert Map to plain object for phase counts
			const counts: Record<string, number> = {};
			for (const [key, value] of Object.entries(workflowsRes.phaseCounts)) {
				counts[key] = value;
			}
			setPhaseCounts(counts);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load workflows');
		} finally {
			setLoading(false);
		}
	}, [setWorkflows, setPhaseTemplates]);

	useEffect(() => {
		loadData();
	}, [loadData, refreshKey]);

	const handleSelectWorkflow = useCallback((workflow: Workflow) => {
		window.dispatchEvent(new CustomEvent('orc:select-workflow', { detail: { workflow } }));
	}, []);

	const handleCloneWorkflow = useCallback((workflow: Workflow) => {
		window.dispatchEvent(new CustomEvent('orc:clone-workflow', { detail: { workflow } }));
	}, []);

	const handleSelectPhaseTemplate = useCallback((template: PhaseTemplate) => {
		window.dispatchEvent(
			new CustomEvent('orc:select-phase-template', {
				detail: { template, source: phaseSources[template.id] }
			})
		);
	}, [phaseSources]);

	const handleAddWorkflow = useCallback(() => {
		window.dispatchEvent(new CustomEvent('orc:add-workflow'));
	}, []);

	// Separate built-in and custom workflows
	const builtinWorkflows = workflows.filter((wf) => wf.isBuiltin);
	const customWorkflows = workflows.filter((wf) => !wf.isBuiltin);

	// Separate built-in and custom phase templates
	const builtinPhases = phaseTemplates.filter((pt) => pt.isBuiltin);
	const customPhases = phaseTemplates.filter((pt) => !pt.isBuiltin);

	const classes = ['workflows-view', className].filter(Boolean).join(' ');

	return (
		<div className={classes}>
			<header className="workflows-view-header">
				<div className="workflows-view-header-text">
					<h1 className="workflows-view-title">Workflows</h1>
					<p className="workflows-view-subtitle">
						Composable task execution plans with configurable phases
					</p>
				</div>
				<Button
					variant="primary"
					leftIcon={<Icon name="plus" size={12} />}
					onClick={handleAddWorkflow}
				>
					New Workflow
				</Button>
			</header>

			<div className="workflows-view-content">
				{/* Built-in Workflows Section */}
				<section className="workflows-view-section">
					<div className="workflows-view-section-header">
						<h2 className="section-title">Built-in Workflows</h2>
						<p className="section-subtitle">Default workflow templates (clone to customize)</p>
					</div>

					{loading && <WorkflowsViewSkeleton />}

					{!loading && error && <WorkflowsViewError error={error} onRetry={loadData} />}

					{!loading && !error && builtinWorkflows.length > 0 && (
						<div className="workflows-view-grid">
							{builtinWorkflows.map((workflow) => (
								<WorkflowCard
									key={workflow.id}
									workflow={workflow}
									phaseCount={phaseCounts[workflow.id]}
									source={workflowSources[workflow.id]}
									onSelect={handleSelectWorkflow}
									onClone={handleCloneWorkflow}
								/>
							))}
						</div>
					)}
				</section>

				{/* Custom Workflows Section */}
				<section className="workflows-view-section">
					<div className="workflows-view-section-header">
						<h2 className="section-title">Custom Workflows</h2>
						<p className="section-subtitle">Your customized workflow configurations</p>
					</div>

					{!loading && !error && customWorkflows.length === 0 && <WorkflowsViewEmpty />}

					{!loading && !error && customWorkflows.length > 0 && (
						<div className="workflows-view-grid">
							{customWorkflows.map((workflow) => (
								<WorkflowCard
									key={workflow.id}
									workflow={workflow}
									phaseCount={phaseCounts[workflow.id]}
									source={workflowSources[workflow.id]}
									onSelect={handleSelectWorkflow}
									onClone={handleCloneWorkflow}
								/>
							))}
						</div>
					)}
				</section>

				{/* Phase Templates Section */}
				<section className="workflows-view-section">
					<div className="workflows-view-section-header">
						<h2 className="section-title">Phase Templates</h2>
						<p className="section-subtitle">
							Reusable phase definitions ({builtinPhases.length} built-in,{' '}
							{customPhases.length} custom)
						</p>
					</div>

					{!loading && !error && phaseTemplates.length > 0 && (
						<div className="workflows-view-grid workflows-view-grid-small">
							{phaseTemplates.map((template) => (
								<PhaseTemplateCard
									key={template.id}
									template={template}
									source={phaseSources[template.id]}
									onSelect={handleSelectPhaseTemplate}
								/>
							))}
						</div>
					)}
				</section>
			</div>
		</div>
	);
}
