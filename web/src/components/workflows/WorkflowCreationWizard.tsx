/**
 * WorkflowCreationWizard - Guided 3-step workflow creation wizard.
 *
 * Steps:
 * 1. Intent Selection - Choose workflow type (Build, Review, Test, Document, Custom)
 * 2. Name & Details - Enter workflow name, auto-generated ID, optional description
 * 3. Phase Selection - Choose phases based on intent recommendations
 *
 * Features:
 * - Step indicator showing current progress
 * - Navigation with Back/Next buttons
 * - Skip to Editor for experienced users
 * - Auto-generated workflow ID from name
 * - Intent-based phase pre-selection
 */

import { useState, useCallback, useEffect, useMemo } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button, Icon, type IconName } from '@/components/ui';
import { workflowClient } from '@/lib/client';
import { type Workflow, type PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import { getRecommendedPhases, slugifyWorkflowId, type WorkflowIntent } from './workflowWizardUtils';
import { PROVIDERS, PROVIDER_MODELS } from '@/lib/providerUtils';
import './WorkflowCreationWizard.css';

export interface WorkflowCreationWizardProps {
	open: boolean;
	onClose: () => void;
	onCreated: (workflow: Workflow) => void;
	onSkipToEditor: () => void;
}

const INTENT_OPTIONS: { id: WorkflowIntent; label: string; icon: IconName }[] = [
	{ id: 'build', label: 'Build', icon: 'code' },
	{ id: 'review', label: 'Review', icon: 'search' },
	{ id: 'test', label: 'Test', icon: 'check-circle' },
	{ id: 'document', label: 'Document', icon: 'file-text' },
	{ id: 'custom', label: 'Custom', icon: 'settings' },
];

type WizardStep = 1 | 2 | 3;

export function WorkflowCreationWizard({
	open,
	onClose,
	onCreated,
	onSkipToEditor,
}: WorkflowCreationWizardProps) {
	// Current step
	const [step, setStep] = useState<WizardStep>(1);

	// Step 1: Intent
	const [selectedIntent, setSelectedIntent] = useState<WorkflowIntent | null>(null);

	// Step 2: Name & Details
	const [name, setName] = useState('');
	const [id, setId] = useState('');
	const [description, setDescription] = useState('');
	const [defaultProvider, setDefaultProvider] = useState('');
	const [defaultModel, setDefaultModel] = useState('');
	const [idManuallySet, setIdManuallySet] = useState(false);
	const [showOptional, setShowOptional] = useState(false);

	// Step 3: Phases
	const [phaseTemplates, setPhaseTemplates] = useState<PhaseTemplate[]>([]);
	const [selectedPhases, setSelectedPhases] = useState<Set<string>>(new Set());
	const [phasesLoaded, setPhasesLoaded] = useState(false);

	// Submission state
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);

	// Define callbacks before useEffects that reference them
	const loadPhaseTemplates = useCallback(async () => {
		try {
			const response = await workflowClient.listPhaseTemplates({});
			setPhaseTemplates(response.templates);
			setPhasesLoaded(true);
		} catch (err) {
			console.error('Failed to load phase templates:', err);
			setPhaseTemplates([]);
			setPhasesLoaded(true);
		}
	}, []);

	// Reset form when modal opens
	useEffect(() => {
		if (open) {
			setStep(1);
			setSelectedIntent(null);
			setName('');
			setId('');
			setDescription('');
			setDefaultProvider('');
			setDefaultModel('');
			setIdManuallySet(false);
			setShowOptional(false);
			setSelectedPhases(new Set());
			setPhasesLoaded(false);
			setError(null);
		}
	}, [open]);

	// Load phase templates when entering step 3
	useEffect(() => {
		if (step === 3 && !phasesLoaded) {
			loadPhaseTemplates();
		}
	}, [step, phasesLoaded, loadPhaseTemplates]);

	// Auto-generate ID from name unless manually set
	useEffect(() => {
		if (!idManuallySet && name) {
			setId(slugifyWorkflowId(name));
		}
	}, [name, idManuallySet]);

	// Pre-select recommended phases when entering step 3
	useEffect(() => {
		if (step === 3 && selectedIntent && phasesLoaded) {
			const recommended = getRecommendedPhases(selectedIntent);
			setSelectedPhases(new Set(recommended));
		}
	}, [step, selectedIntent, phasesLoaded]);

	const handleIdChange = useCallback((value: string) => {
		// When user manually edits, accept their input as-is (no slugification)
		// This allows users to type exact IDs like "custom-id"
		setId(value);
		setIdManuallySet(true);
	}, []);

	const handlePhaseToggle = useCallback((phaseId: string) => {
		setSelectedPhases((prev) => {
			const next = new Set(prev);
			if (next.has(phaseId)) {
				next.delete(phaseId);
			} else {
				next.add(phaseId);
			}
			return next;
		});
	}, []);

	const handleNext = useCallback(() => {
		if (step < 3) {
			setStep((s) => (s + 1) as WizardStep);
		}
	}, [step]);

	const handleBack = useCallback(() => {
		if (step > 1) {
			setStep((s) => (s - 1) as WizardStep);
		}
	}, [step]);

	const handleCreate = useCallback(async () => {
		if (!id.trim() || selectedPhases.size === 0) return;

		setSaving(true);
		setError(null);

		try {
			const workflowId = id.trim();

			// Step 1: Create the workflow
			const createResponse = await workflowClient.createWorkflow({
				id: workflowId,
				name: name.trim() || undefined,
				description: description.trim() || undefined,
				defaultProvider: defaultProvider || undefined,
				defaultModel: defaultModel || undefined,
			});

			if (!createResponse.workflow) {
				throw new Error('Failed to create workflow');
			}

			// Step 2: Add phases in order
			// Build ordered list of phase template IDs based on their order in phaseTemplates
			const orderedPhaseIds = phaseTemplates
				.filter((t) => selectedPhases.has(t.id))
				.map((t) => t.id);

			for (let i = 0; i < orderedPhaseIds.length; i++) {
				await workflowClient.addPhase({
					workflowId,
					phaseTemplateId: orderedPhaseIds[i],
					sequence: i,
				});
			}

			// Return the created workflow - the editor will load the full details
			onCreated(createResponse.workflow);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to create workflow');
		} finally {
			setSaving(false);
		}
	}, [id, name, description, defaultProvider, defaultModel, selectedPhases, phaseTemplates, onCreated]);

	// Recommended phases for current intent
	const recommendedPhases = useMemo(() => {
		if (!selectedIntent) return [];
		return getRecommendedPhases(selectedIntent);
	}, [selectedIntent]);

	// Validation
	const canProceedStep1 = selectedIntent !== null;
	const canProceedStep2 = name.trim().length > 0;
	const canCreate = selectedPhases.size > 0;

	return (
		<Modal
			open={open}
			onClose={onClose}
			title="New Workflow"
			size="md"
			ariaLabel="Create workflow wizard"
		>
			<div className="wizard-content">
				{/* Step indicator */}
				<div className="wizard-step-indicator" data-testid="step-indicator">
					<div
						className={`wizard-step ${step >= 1 ? 'active' : ''} ${step > 1 ? 'completed' : ''}`}
						data-testid="step-1"
					>
						<span className="wizard-step-number">{step > 1 ? '✓' : '1'}</span>
					</div>
					<div className="wizard-step-connector" />
					<div
						className={`wizard-step ${step >= 2 ? 'active' : ''} ${step > 2 ? 'completed' : ''}`}
						data-testid="step-2"
					>
						<span className="wizard-step-number">{step > 2 ? '✓' : '2'}</span>
					</div>
					<div className="wizard-step-connector" />
					<div
						className={`wizard-step ${step >= 3 ? 'active' : ''}`}
						data-testid="step-3"
					>
						<span className="wizard-step-number">3</span>
					</div>
				</div>

				{/* Step 1: Intent Selection */}
				{step === 1 && (
					<>
						<div className="wizard-step-header">
							<h3 className="wizard-step-title">What kind of workflow?</h3>
							<p className="wizard-step-label">Step 1 of 3</p>
						</div>
						<div className="wizard-intent-grid">
							{INTENT_OPTIONS.map((intent) => (
								<button
									key={intent.id}
									type="button"
									className={`wizard-intent-button ${selectedIntent === intent.id ? 'selected' : ''}`}
									onClick={() => setSelectedIntent(intent.id)}
								>
									<Icon name={intent.icon} size={24} className="intent-icon" />
									<span className="intent-label">{intent.label}</span>
								</button>
							))}
						</div>
					</>
				)}

				{/* Step 2: Name & Details */}
				{step === 2 && (
					<>
						<div className="wizard-step-header">
							<h3 className="wizard-step-title">Name your workflow</h3>
							<p className="wizard-step-label">Step 2 of 3</p>
						</div>
						<div className="wizard-form">
							<div className="form-group">
								<label htmlFor="wizard-name" className="form-label">
									Name <span className="form-required">*</span>
								</label>
								<input
									id="wizard-name"
									type="text"
									className="form-input"
									value={name}
									onChange={(e) => setName(e.target.value)}
									placeholder="My Custom Workflow"
									autoFocus
								/>
							</div>

							<div className="form-group">
								<label htmlFor="wizard-id" className="form-label">
									ID
								</label>
								<input
									id="wizard-id"
									type="text"
									className="form-input"
									value={id}
									onChange={(e) => handleIdChange(e.target.value)}
									placeholder="my-custom-workflow"
								/>
								<span className="form-help">
									Auto-generated from name (lowercase, hyphens)
								</span>
							</div>

							<div className="form-group">
								<label htmlFor="wizard-description" className="form-label">
									Description
								</label>
								<textarea
									id="wizard-description"
									className="form-textarea"
									value={description}
									onChange={(e) => setDescription(e.target.value)}
									placeholder="Describe what this workflow does..."
									rows={2}
								/>
							</div>

							<div className="wizard-optional-section">
								<button
									type="button"
									className="wizard-optional-trigger"
									onClick={() => setShowOptional(!showOptional)}
								>
									<Icon name={showOptional ? 'chevron-down' : 'chevron-right'} size={16} />
									<span>Optional settings</span>
								</button>
								{showOptional && (
									<div className="wizard-optional-content">
										<div className="form-group">
											<label htmlFor="wizard-provider" className="form-label">
												Default Provider
											</label>
											<select
												id="wizard-provider"
												className="form-select"
												value={defaultProvider}
												onChange={(e) => {
													setDefaultProvider(e.target.value);
													setDefaultModel('');
												}}
											>
												<option value="">Claude (default)</option>
												{PROVIDERS.map((p) => (
													<option key={p.value} value={p.value}>
														{p.label}
													</option>
												))}
											</select>
										</div>
										<div className="form-group">
											<label htmlFor="wizard-model" className="form-label">
												Default Model
											</label>
											{(PROVIDER_MODELS[defaultProvider || 'claude'] ?? []).length > 0 ? (
												<select
													id="wizard-model"
													className="form-select"
													value={defaultModel}
													onChange={(e) => setDefaultModel(e.target.value)}
												>
													<option value="">Default (inherit)</option>
													{(PROVIDER_MODELS[defaultProvider || 'claude'] ?? []).map((m) => (
														<option key={m.value} value={m.value}>
															{m.label}
														</option>
													))}
												</select>
											) : (
												<input
													id="wizard-model"
													type="text"
													className="form-input"
													value={defaultModel}
													onChange={(e) => setDefaultModel(e.target.value)}
													placeholder="Type model name..."
												/>
											)}
										</div>
									</div>
								)}
							</div>
						</div>
					</>
				)}

				{/* Step 3: Phase Selection */}
				{step === 3 && (
					<>
						<div className="wizard-step-header">
							<h3 className="wizard-step-title">Choose your phases</h3>
							<p className="wizard-step-label">Step 3 of 3</p>
						</div>
						<div className="wizard-phases">
							{selectedIntent && selectedIntent !== 'custom' && (
								<div className="wizard-phases-recommended">
									<span className="wizard-phases-recommended-label">
										Recommended for {selectedIntent.charAt(0).toUpperCase() + selectedIntent.slice(1)}
									</span>
								</div>
							)}
							<div className="wizard-phase-list">
								{phaseTemplates.map((template) => {
									const isRecommended = recommendedPhases.includes(template.id);
									const isSelected = selectedPhases.has(template.id);
									return (
										<label
											key={template.id}
											className={`wizard-phase-item ${isRecommended ? 'recommended' : ''}`}
										>
											<div className="wizard-phase-checkbox">
												<input
													type="checkbox"
													checked={isSelected}
													onChange={() => handlePhaseToggle(template.id)}
													aria-label={template.name || template.id}
												/>
											</div>
											<div className="wizard-phase-info">
												<span className="wizard-phase-name">
													{template.name || template.id}
												</span>
											</div>
										</label>
									);
								})}
							</div>
						</div>
					</>
				)}

				{/* Error message */}
				{error && (
					<div className="wizard-error" role="alert">
						<Icon name="alert-circle" size={14} />
						<span>{error}</span>
					</div>
				)}

				{/* Navigation */}
				<div className="wizard-actions">
					<div className="wizard-actions-left">
						<Button variant="ghost" onClick={onClose}>
							Cancel
						</Button>
						{step === 1 && (
							<Button variant="ghost" onClick={onSkipToEditor}>
								Skip to Editor
							</Button>
						)}
					</div>
					<div className="wizard-actions-right">
						{step > 1 && (
							<Button variant="ghost" onClick={handleBack}>
								Back
							</Button>
						)}
						{step < 3 && (
							<Button
								variant="primary"
								onClick={handleNext}
								disabled={step === 1 ? !canProceedStep1 : !canProceedStep2}
							>
								Next
							</Button>
						)}
						{step === 3 && (
							<Button
								variant="primary"
								onClick={handleCreate}
								disabled={saving || !canCreate}
								loading={saving}
							>
								{saving ? 'Creating...' : 'Create & Open Editor'}
							</Button>
						)}
					</div>
				</div>
			</div>
		</Modal>
	);
}
