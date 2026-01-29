import { useEffect, useState, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { workflowClient } from '@/lib/client';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { WorkflowCanvas } from './WorkflowCanvas';
import './WorkflowEditorPage.css';

export function WorkflowEditorPage() {
	const { id } = useParams<{ id: string }>();
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const workflowDetails = useWorkflowEditorStore((s) => s.workflowDetails);
	const readOnly = useWorkflowEditorStore((s) => s.readOnly);
	const loadFromWorkflow = useWorkflowEditorStore((s) => s.loadFromWorkflow);
	const reset = useWorkflowEditorStore((s) => s.reset);

	const fetchWorkflow = useCallback(async () => {
		if (!id) return;
		setLoading(true);
		setError(null);
		try {
			const response = await workflowClient.getWorkflow({ id });
			if (!response.workflow) {
				setError('Workflow not found');
				return;
			}
			loadFromWorkflow(response.workflow);
		} catch (err) {
			const message =
				err instanceof Error ? err.message : 'Failed to load workflow';
			if (message.includes('not found') || message.includes('404')) {
				setError('Workflow not found');
			} else {
				setError(message);
			}
		} finally {
			setLoading(false);
		}
	}, [id, loadFromWorkflow]);

	useEffect(() => {
		fetchWorkflow();
		return () => reset();
	}, [fetchWorkflow, reset]);

	if (loading) {
		return (
			<div className="workflow-editor-page">
				<div className="workflow-editor-loading">Loading workflow...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="workflow-editor-page">
				<div className="workflow-editor-error">
					<h2>{error}</h2>
					<p>
						{error === 'Workflow not found'
							? 'The requested workflow does not exist.'
							: 'Something went wrong loading this workflow.'}
					</p>
					<Link to="/workflows">Back to Workflows</Link>
					{error !== 'Workflow not found' && (
						<button onClick={fetchWorkflow}>Retry</button>
					)}
				</div>
			</div>
		);
	}

	const workflowName = workflowDetails?.workflow?.name ?? 'Workflow';

	return (
		<div className="workflow-editor-page">
			<div className="workflow-editor-header">
				<nav className="workflow-editor-breadcrumb">
					<Link to="/workflows">Workflows</Link>
					<span className="workflow-editor-breadcrumb-separator">/</span>
					<span className="workflow-editor-breadcrumb-current">
						{workflowName}
					</span>
				</nav>
				{readOnly && (
					<span className="workflow-editor-badge">
						Read-only &middot; Clone to customize
					</span>
				)}
			</div>
			<div className="workflow-editor-body">
				<aside className="workflow-editor-palette">
					<span>Phase Palette</span>
					<span>(coming soon)</span>
				</aside>
				<div className="workflow-editor-canvas">
					<WorkflowCanvas />
				</div>
				<aside className="workflow-editor-inspector">
					<span>Phase Inspector</span>
					<span>(coming soon)</span>
				</aside>
			</div>
		</div>
	);
}
