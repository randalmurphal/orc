import { useMemo } from 'react';
import type { WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';
import './VariableReferencePanel.css';

// Built-in variables available in all workflows
const BUILTIN_VARIABLES = {
	'Task Context': [
		{ name: 'TASK_ID', description: 'Task identifier (e.g., TASK-001)' },
		{ name: 'TASK_TITLE', description: 'Task title' },
		{ name: 'TASK_DESCRIPTION', description: 'Task description' },
		{ name: 'TASK_CATEGORY', description: 'Task category (feature, bug, etc.)' },
		{ name: 'WEIGHT', description: 'Task weight (trivial, small, medium, large)' },
	],
	'Execution Context': [
		{ name: 'PHASE', description: 'Current phase ID' },
		{ name: 'ITERATION', description: 'Current iteration number' },
	],
	'Retry Context': [
		{ name: 'RETRY_ATTEMPT', description: 'Retry attempt number (e.g., 2, 3)' },
		{ name: 'RETRY_FROM_PHASE', description: 'Phase that triggered the retry' },
		{ name: 'RETRY_REASON', description: 'Reason the retry was triggered' },
	],
	'Git Context': [
		{ name: 'WORKTREE_PATH', description: 'Path to git worktree' },
		{ name: 'PROJECT_ROOT', description: 'Project root directory' },
		{ name: 'TASK_BRANCH', description: 'Git branch for this task' },
		{ name: 'TARGET_BRANCH', description: 'Branch to merge into' },
	],
	'Project Detection': [
		{ name: 'LANGUAGE', description: 'Primary programming language' },
		{ name: 'HAS_FRONTEND', description: 'Whether project has frontend' },
		{ name: 'HAS_TESTS', description: 'Whether project has tests' },
		{ name: 'FRAMEWORKS', description: 'Detected frameworks' },
	],
	'Commands': [
		{ name: 'TEST_COMMAND', description: 'Project test command' },
		{ name: 'LINT_COMMAND', description: 'Project lint command' },
		{ name: 'BUILD_COMMAND', description: 'Project build command' },
	],
};


interface VariableReferencePanelProps {
	workflowDetails?: WorkflowWithDetails | null;
	onVariableClick?: (varName: string) => void;
	collapsed?: boolean;
}

export function VariableReferencePanel({
	workflowDetails,
	onVariableClick,
	collapsed = false,
}: VariableReferencePanelProps) {
	// Get custom workflow variables
	const workflowVariables = useMemo(() => {
		return workflowDetails?.variables ?? [];
	}, [workflowDetails]);

	// Derive named phase output variables from template metadata
	const namedPhaseOutputVars = useMemo(() => {
		const phases = workflowDetails?.phases ?? [];
		const seen = new Set<string>();
		const vars: { name: string; description: string }[] = [];

		for (const p of phases) {
			const varName = p.template?.outputVarName;
			if (varName && !seen.has(varName)) {
				seen.add(varName);
				vars.push({
					name: varName,
					description: `Output from ${p.template?.name || p.phaseTemplateId} phase`,
				});
			}
		}
		return vars;
	}, [workflowDetails]);

	// Generic OUTPUT_<PHASE_ID> variables for all phases
	const genericPhaseOutputVars = useMemo(() => {
		const phases = workflowDetails?.phases ?? [];
		return phases.map((p) => ({
			name: `OUTPUT_${p.phaseTemplateId.toUpperCase().replace(/-/g, '_')}`,
			description: `Raw output from ${p.phaseTemplateId} phase`,
		}));
	}, [workflowDetails]);

	const handleCopy = (varName: string) => {
		navigator.clipboard.writeText(`{{${varName}}}`);
		onVariableClick?.(varName);
	};

	if (collapsed) {
		return (
			<div className="variable-reference-collapsed">
				<span className="variable-reference-collapsed-label">Variables</span>
			</div>
		);
	}

	return (
		<div className="variable-reference-panel">
			<h4 className="variable-reference-title">Variable Reference</h4>
			<p className="variable-reference-hint">
				Click to copy. Use <code>{`{{VAR_NAME}}`}</code> in prompts.
			</p>

			{/* Workflow Variables */}
			{workflowVariables.length > 0 && (
				<VariableSection title="Workflow Variables">
					{workflowVariables.map((v) => (
						<VariableChip
							key={v.name}
							name={v.name}
							description={v.description || undefined}
							onClick={() => handleCopy(v.name)}
						/>
					))}
				</VariableSection>
			)}

			{/* Phase Output Variables — derived from template metadata */}
			{(namedPhaseOutputVars.length > 0 || genericPhaseOutputVars.length > 0) && (
				<VariableSection title="Phase Outputs">
					{namedPhaseOutputVars.map((v) => (
						<VariableChip
							key={v.name}
							name={v.name}
							description={v.description}
							onClick={() => handleCopy(v.name)}
						/>
					))}
					{genericPhaseOutputVars.map((v) => (
						<VariableChip
							key={v.name}
							name={v.name}
							description={v.description}
							onClick={() => handleCopy(v.name)}
							secondary
						/>
					))}
				</VariableSection>
			)}

			{/* Built-in Variables by Category */}
			{Object.entries(BUILTIN_VARIABLES).map(([category, vars]) => (
				<VariableSection key={category} title={category}>
					{vars.map((v) => (
						<VariableChip
							key={v.name}
							name={v.name}
							description={v.description}
							onClick={() => handleCopy(v.name)}
						/>
					))}
				</VariableSection>
			))}
		</div>
	);
}

// ─── Sub-components ────────────────────────────────────────────────────────

interface VariableSectionProps {
	title: string;
	children: React.ReactNode;
}

function VariableSection({ title, children }: VariableSectionProps) {
	return (
		<div className="variable-reference-section">
			<h5 className="variable-reference-section-title">{title}</h5>
			<div className="variable-reference-chips">{children}</div>
		</div>
	);
}

interface VariableChipProps {
	name: string;
	description?: string;
	onClick: () => void;
	secondary?: boolean;
}

function VariableChip({ name, description, onClick, secondary }: VariableChipProps) {
	return (
		<button
			type="button"
			className={`variable-chip ${secondary ? 'variable-chip--secondary' : ''}`}
			onClick={onClick}
			title={description || name}
		>
			<code>{`{{${name}}}`}</code>
		</button>
	);
}
