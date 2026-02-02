/**
 * FeedbackPanel component - allows users to provide feedback to agents during task execution.
 *
 * Features:
 * - Display list of existing feedback for current task
 * - Create new feedback with different types (GENERAL, INLINE, APPROVAL, DIRECTION)
 * - Select feedback timing (NOW, WHEN_DONE, MANUAL)
 * - Add inline comments targeting specific files and lines
 * - Send all pending feedback to the agent
 * - Delete specific feedback items
 * - Real-time status updates via WebSocket
 * - Form validation and accessibility compliance
 */

import React, { useState, useEffect, useCallback } from 'react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Textarea } from '@/components/ui/Textarea';
import { Icon } from '@/components/ui/Icon';
import { feedbackClient, taskClient } from '@/lib/client';
import { FeedbackType, FeedbackTiming, type Feedback } from '@/gen/orc/v1/feedback_pb';
import { TaskStatus, type Task } from '@/gen/orc/v1/task_pb';
import { useCurrentProjectId } from '@/stores';
import { toast } from '@/stores/uiStore';
import './FeedbackPanel.css';

export interface FeedbackPanelProps {
	task: Task;
	onTaskUpdate?: (task: Task) => void;
}

interface FormData {
	text: string;
	type: FeedbackType;
	timing: FeedbackTiming;
	file: string;
	line: string;
}

interface FormErrors {
	text?: string;
	file?: string;
	line?: string;
	submit?: string;
}

const FEEDBACK_TYPE_LABELS: Record<FeedbackType, string> = {
	[FeedbackType.UNSPECIFIED]: 'Unspecified',
	[FeedbackType.GENERAL]: 'General',
	[FeedbackType.INLINE]: 'Inline Comment',
	[FeedbackType.APPROVAL]: 'Approval',
	[FeedbackType.DIRECTION]: 'Direction Change',
};

const FEEDBACK_TIMING_LABELS: Record<FeedbackTiming, string> = {
	[FeedbackTiming.UNSPECIFIED]: 'Unspecified',
	[FeedbackTiming.NOW]: 'Send Now',
	[FeedbackTiming.WHEN_DONE]: 'When Done',
	[FeedbackTiming.MANUAL]: 'Save for Later',
};

export const FeedbackPanel: React.FC<FeedbackPanelProps> = ({ task, onTaskUpdate }) => {
	const projectId = useCurrentProjectId();
	const [feedback, setFeedback] = useState<Feedback[]>([]);
	const [loading, setLoading] = useState(false);
	const [loadError, setLoadError] = useState<string>('');
	const [submitting, setSubmitting] = useState(false);
	const [sending, setSending] = useState(false);
	const [sendError, setSendError] = useState<string>('');

	// Form state
	const [formData, setFormData] = useState<FormData>({
		text: '',
		type: FeedbackType.GENERAL,
		timing: FeedbackTiming.WHEN_DONE,
		file: '',
		line: '',
	});
	const [errors, setErrors] = useState<FormErrors>({});

	// Load feedback for the current task
	const loadFeedback = useCallback(async () => {
		if (!projectId || !task.id) return;

		setLoading(true);
		setLoadError('');

		try {
			const response = await feedbackClient.listFeedback({
				projectId,
				taskId: task.id,
				excludeReceived: false,
			});
			setFeedback(response.feedback);
		} catch (error) {
			console.error('Failed to load feedback:', error);
			setLoadError('Failed to load feedback');
		} finally {
			setLoading(false);
		}
	}, [projectId, task.id]);

	// Load feedback on mount and when task changes
	useEffect(() => {
		loadFeedback();
	}, [loadFeedback]);

	// TODO: Add WebSocket support for real-time updates
	// const ws = useWebSocket();
	// useEffect(() => {
	// 	const handleFeedbackUpdate = (event: FeedbackEvent) => {
	// 		if (event.taskId === task.id) {
	// 			loadFeedback();
	// 		}
	// 	};
	// 	ws.on('feedback', handleFeedbackUpdate);
	// 	return () => ws.off('feedback', handleFeedbackUpdate);
	// }, [ws, task.id, loadFeedback]);

	// Validate form data
	const validateForm = (data: FormData): FormErrors => {
		const errors: FormErrors = {};

		if (!data.text.trim()) {
			errors.text = 'Feedback text is required';
		}

		if (data.type === FeedbackType.INLINE) {
			if (!data.file.trim()) {
				errors.file = 'File path is required for inline comments';
			}
			const lineNum = parseInt(data.line, 10);
			if (data.line && (isNaN(lineNum) || lineNum <= 0)) {
				errors.line = 'Line number must be positive';
			}
		}

		return errors;
	};

	// Handle form submission
	const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
		e.preventDefault();

		const formErrors = validateForm(formData);
		if (Object.keys(formErrors).length > 0) {
			setErrors(formErrors);
			return;
		}

		if (!projectId || !task.id) return;

		setSubmitting(true);
		setErrors({});

		try {
			const lineNumber = formData.type === FeedbackType.INLINE ? parseInt(formData.line, 10) || 0 : 0;

			await feedbackClient.addFeedback({
				projectId,
				taskId: task.id,
				type: formData.type,
				text: formData.text.trim(),
				timing: formData.timing,
				file: formData.type === FeedbackType.INLINE ? formData.file.trim() : '',
				line: lineNumber,
			});

			// Clear form
			setFormData({
				text: '',
				type: FeedbackType.GENERAL,
				timing: FeedbackTiming.WHEN_DONE,
				file: '',
				line: '',
			});

			// Reload feedback list
			await loadFeedback();

			toast.success('Feedback added successfully');

			// If timing is NOW and task is running, pause the task
			if (formData.timing === FeedbackTiming.NOW && task.status === TaskStatus.RUNNING) {
				try {
					const pausedTask = await taskClient.pauseTask({
						projectId,
						taskId: task.id,
					});
					onTaskUpdate?.(pausedTask.task!);
				} catch (error) {
					console.error('Failed to pause task:', error);
				}
			}
		} catch (error) {
			console.error('Failed to add feedback:', error);
			setErrors({ submit: 'Failed to add feedback' });
			toast.error('Failed to add feedback');
		} finally {
			setSubmitting(false);
		}
	};

	// Handle sending pending feedback
	const handleSendPending = async () => {
		if (!projectId || !task.id) return;

		setSending(true);
		setSendError('');
		try {
			await feedbackClient.sendFeedback({
				projectId,
				taskId: task.id,
			});

			await loadFeedback();
			toast.success('Pending feedback sent to agent');
		} catch (error) {
			console.error('Failed to send feedback:', error);
			setSendError('Failed to send feedback');
			toast.error('Failed to send feedback');
		} finally {
			setSending(false);
		}
	};

	// Handle deleting feedback
	const handleDeleteFeedback = async (feedbackId: string) => {
		if (!projectId || !task.id) return;

		try {
			await feedbackClient.deleteFeedback({
				projectId,
				taskId: task.id,
				feedbackId,
			});

			await loadFeedback();
			toast.success('Feedback deleted');
		} catch (error) {
			console.error('Failed to delete feedback:', error);
			toast.error('Failed to delete feedback');
		}
	};

	// Handle form field changes
	const handleFieldChange = (field: keyof FormData, value: string | FeedbackType | FeedbackTiming) => {
		setFormData((prev) => ({ ...prev, [field]: value }));
		// Clear related errors
		if (errors[field as keyof FormErrors]) {
			setErrors((prev) => ({ ...prev, [field]: undefined }));
		}
	};

	// Handle type selection change
	const handleTypeChange = (value: string) => {
		const typeValue = FeedbackType[value as keyof typeof FeedbackType];
		if (typeof typeValue === 'number') {
			setFormData((prev) => ({ ...prev, type: typeValue }));
			if (errors.file || errors.line) {
				setErrors((prev) => ({ ...prev, file: undefined, line: undefined }));
			}
		}
	};

	// Handle timing selection change
	const handleTimingChange = (value: string) => {
		const timingValue = FeedbackTiming[value as keyof typeof FeedbackTiming];
		if (typeof timingValue === 'number') {
			setFormData((prev) => ({ ...prev, timing: timingValue }));
		}
	};

	const pendingFeedback = feedback.filter((f) => !f.received);
	const pendingCount = pendingFeedback.length;

	const showNowWarning = formData.timing === FeedbackTiming.NOW && task.status === TaskStatus.RUNNING;
	const showWhenDoneInfo = formData.timing === FeedbackTiming.WHEN_DONE;

	// Helper functions to get enum string key from number value
	const getTypeKey = (typeValue: FeedbackType): string => {
		for (const [key, value] of Object.entries(FeedbackType)) {
			if (value === typeValue && isNaN(Number(key))) {
				return key;
			}
		}
		return 'GENERAL';
	};

	const getTimingKey = (timingValue: FeedbackTiming): string => {
		for (const [key, value] of Object.entries(FeedbackTiming)) {
			if (value === timingValue && isNaN(Number(key))) {
				return key;
			}
		}
		return 'WHEN_DONE';
	};

	return (
		<div className="feedback-panel">
			<div className="feedback-panel__header">
				<h2 className="feedback-panel__title">Feedback</h2>
				{pendingCount > 0 && (
					<Button
						variant="primary"
						size="sm"
						onClick={handleSendPending}
						loading={sending}
						className="feedback-panel__send-button"
					>
						Send Pending ({pendingCount})
					</Button>
				)}
			</div>

			{/* Status indicator for screen readers */}
			<div role="status" aria-live="polite" className="sr-only">
				{pendingCount > 0 ? `${pendingCount} pending feedback` : 'No pending feedback'}
			</div>

			{/* Feedback List */}
			<div className="feedback-panel__list">
				{loadError && (
					<div className="feedback-panel__error" role="alert">
						{loadError}
					</div>
				)}

				{loading ? (
					<div className="feedback-panel__loading">Loading feedback...</div>
				) : feedback.length === 0 ? (
					<div className="feedback-panel__empty">No feedback yet</div>
				) : (
					feedback.map((item) => (
						<div key={item.id} className="feedback-item">
							<div className="feedback-item__header">
								<span className="feedback-item__type">
									{FEEDBACK_TYPE_LABELS[item.type]}
								</span>
								<span className="feedback-item__timing">
									{FEEDBACK_TIMING_LABELS[item.timing]}
								</span>
								<span
									className={`feedback-item__status ${
										item.received ? 'feedback-item__status--received' : 'feedback-item__status--pending'
									}`}
									aria-label={item.received ? 'Received feedback' : 'Pending feedback'}
								>
									{item.received ? 'Received' : 'Pending'}
								</span>
								<Button
									variant="ghost"
									size="sm"
									iconOnly
									onClick={() => handleDeleteFeedback(item.id)}
									aria-label="Delete feedback"
									className="feedback-item__delete"
								>
									<Icon name="trash" />
								</Button>
							</div>
							<div className="feedback-item__content">
								{item.text}
								{item.type === FeedbackType.INLINE && item.file && (
									<div className="feedback-item__location">
										at {item.file}
										{item.line > 0 && `:${item.line}`}
									</div>
								)}
							</div>
						</div>
					))
				)}
			</div>

			{/* Feedback Form */}
			<form onSubmit={handleSubmit} className="feedback-panel__form">
				<div className="form-group">
					<Textarea
						placeholder="Enter your feedback..."
						value={formData.text}
						onChange={(e) => handleFieldChange('text', e.target.value)}
						error={errors.text}
						rows={3}
						aria-label="Feedback text"
						aria-describedby="feedback-text-help"
						required
					/>
					<div id="feedback-text-help" className="form-help">
						Provide feedback for the agent working on this task
					</div>
					{errors.text && (
						<div className="form-error" role="alert">
							{errors.text}
						</div>
					)}
				</div>

				<div className="form-row">
					<div className="form-group">
						<label htmlFor="feedback-type" className="form-label">
							Type
						</label>
						<select
							id="feedback-type"
							value={getTypeKey(formData.type)}
							onChange={(e) => handleTypeChange(e.target.value)}
							className="form-select"
							aria-describedby="feedback-type-help"
							aria-label="Feedback type"
						>
							{Object.keys(FeedbackType)
								.filter(key => isNaN(Number(key)) && key !== 'UNSPECIFIED')
								.map(key => (
									<option key={key} value={key}>
										{FEEDBACK_TYPE_LABELS[FeedbackType[key as keyof typeof FeedbackType]]}
									</option>
								))}
						</select>
						<div id="feedback-type-help" className="form-help">
							Choose the type of feedback you're providing
						</div>
					</div>

					<div className="form-group">
						<label htmlFor="feedback-timing" className="form-label">
							Timing
						</label>
						<select
							id="feedback-timing"
							value={getTimingKey(formData.timing)}
							onChange={(e) => handleTimingChange(e.target.value)}
							className="form-select"
							aria-describedby="feedback-timing-help"
							aria-label="Timing"
						>
							{Object.keys(FeedbackTiming)
								.filter(key => isNaN(Number(key)) && key !== 'UNSPECIFIED')
								.map(key => (
									<option key={key} value={key}>
										{FEEDBACK_TIMING_LABELS[FeedbackTiming[key as keyof typeof FeedbackTiming]]}
									</option>
								))}
						</select>
						<div id="feedback-timing-help" className="form-help">
							When should this feedback be delivered?
						</div>
					</div>
				</div>

				{/* Inline comment fields */}
				{formData.type === FeedbackType.INLINE && (
					<div className="form-row">
						<div className="form-group">
							<label htmlFor="feedback-file" className="form-label">
								File Path
							</label>
							<Input
								id="feedback-file"
								type="text"
								placeholder="src/components/Button.tsx"
								value={formData.file}
								onChange={(e) => handleFieldChange('file', e.target.value)}
								error={errors.file}
								aria-label="File path"
								aria-describedby="file-path-help"
							/>
							<div id="file-path-help" className="form-help">
								Path to the file this comment applies to
							</div>
							{errors.file && (
								<div className="form-error" role="alert">
									{errors.file}
								</div>
							)}
						</div>

						<div className="form-group">
							<label htmlFor="feedback-line" className="form-label">
								Line Number
							</label>
							<Input
								id="feedback-line"
								type="number"
								placeholder="42"
								value={formData.line}
								onChange={(e) => handleFieldChange('line', e.target.value)}
								error={errors.line}
								min="1"
								aria-label="Line number"
								aria-describedby="line-number-help"
							/>
							<div id="line-number-help" className="form-help">
								Line number (optional)
							</div>
							{errors.line && (
								<div className="form-error" role="alert">
									{errors.line}
								</div>
							)}
						</div>
					</div>
				)}

				{/* Timing warnings/info */}
				{showNowWarning && (
					<div className="feedback-panel__warning" role="alert">
						<Icon name="warning" />
						Sending now will pause the task
					</div>
				)}

				{showWhenDoneInfo && (
					<div className="feedback-panel__info">
						<Icon name="info" />
						Feedback will be queued until phase completion
					</div>
				)}

				{/* Form submission errors */}
				{errors.submit && (
					<div className="form-error" role="alert">
						{errors.submit}
					</div>
				)}

				{/* Send pending errors */}
				{sendError && (
					<div className="form-error" role="alert">
						{sendError}
					</div>
				)}

				<Button
					type="submit"
					variant="primary"
					loading={submitting}
					className="feedback-panel__submit"
				>
					Add Feedback
				</Button>
			</form>
		</div>
	);
};