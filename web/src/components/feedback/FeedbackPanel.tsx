/**
 * FeedbackPanel component for providing real-time feedback to agents during task execution
 *
 * Features:
 * - Add general feedback, inline code comments, approval, and direction changes
 * - Three timing options: Send Now (pauses task), Send When Done (queued), Save for Later (manual)
 * - List of pending feedback with delete option
 * - Send all queued feedback at once
 * - Form validation and error handling
 *
 * Integration:
 * - Appears in TaskDetail page for running/active tasks
 * - Connects to FeedbackService gRPC endpoints
 * - Triggers task pause when "Send Now" is selected
 */

import { useState, useEffect, useCallback } from 'react';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { FeedbackType, FeedbackTiming } from '@/gen/orc/v1/feedback_pb';
import type { Feedback } from '@/gen/orc/v1/feedback_pb';

export interface FeedbackPanelProps {
	/** Task ID for the feedback */
	taskId: string;
	/** Project ID for the feedback */
	projectId: string;
	/** Whether the task is currently running (affects form availability) */
	isTaskRunning?: boolean;
	/** Callback when feedback is added successfully */
	onFeedbackAdded?: (feedback: Feedback) => void;
	/** Callback when feedback send triggers task pause */
	onTaskPaused?: () => void;
	/** Error handler */
	onError?: (error: string) => void;
}

interface FeedbackFormData {
	type: FeedbackType;
	text: string;
	timing: FeedbackTiming;
	file: string;
	line: number;
}

const initialFormData: FeedbackFormData = {
	type: FeedbackType.GENERAL,
	text: '',
	timing: FeedbackTiming.WHEN_DONE,
	file: '',
	line: 0,
};

const FEEDBACK_TYPE_LABELS: Record<FeedbackType, string> = {
	[FeedbackType.UNSPECIFIED]: 'Select type',
	[FeedbackType.GENERAL]: 'General',
	[FeedbackType.INLINE]: 'Inline comment',
	[FeedbackType.APPROVAL]: 'Approval',
	[FeedbackType.DIRECTION]: 'Direction change',
};

const FEEDBACK_TIMING_LABELS: Record<FeedbackTiming, string> = {
	[FeedbackTiming.UNSPECIFIED]: 'Select timing',
	[FeedbackTiming.NOW]: 'Send Now',
	[FeedbackTiming.WHEN_DONE]: 'Send When Done',
	[FeedbackTiming.MANUAL]: 'Save for Later',
};

export function FeedbackPanel({
	taskId,
	projectId,
	isTaskRunning = false,
	onFeedbackAdded,
	onTaskPaused,
	onError,
}: FeedbackPanelProps) {
	const [formData, setFormData] = useState<FeedbackFormData>(initialFormData);
	const [pendingFeedback, setPendingFeedback] = useState<Feedback[]>([]);
	const [isSubmitting, setIsSubmitting] = useState(false);
	const [isSending, setIsSending] = useState(false);
	const [isLoading, setIsLoading] = useState(true);

	// Load existing pending feedback
	const loadPendingFeedback = useCallback(async () => {
		try {
			const { feedbackClient } = await import('@/lib/client');
			const response = await feedbackClient.listFeedback({
				projectId,
				taskId,
				excludeReceived: true,
			});
			setPendingFeedback(response.feedback);
		} catch (error) {
			console.error('Failed to load pending feedback:', error);
			onError?.('Failed to load existing feedback');
		} finally {
			setIsLoading(false);
		}
	}, [taskId, projectId, onError]);

	useEffect(() => {
		loadPendingFeedback();
	}, [loadPendingFeedback]);

	const handleInputChange = useCallback((field: keyof FeedbackFormData, value: string | FeedbackType | FeedbackTiming | number) => {
		setFormData(prev => ({ ...prev, [field]: value }));
	}, []);

	const validateForm = useCallback(() => {
		if (!formData.text.trim()) {
			onError?.('Feedback text is required');
			return false;
		}

		if (formData.type === FeedbackType.UNSPECIFIED) {
			onError?.('Please select a feedback type');
			return false;
		}

		if (formData.timing === FeedbackTiming.UNSPECIFIED) {
			onError?.('Please select timing');
			return false;
		}

		if (formData.type === FeedbackType.INLINE) {
			if (!formData.file.trim()) {
				onError?.('File is required for inline comments');
				return false;
			}
			if (formData.line <= 0) {
				onError?.('Line number is required for inline comments');
				return false;
			}
		}

		return true;
	}, [formData, onError]);

	const handleSubmit = useCallback(async (e: React.FormEvent) => {
		e.preventDefault();

		if (!validateForm()) {
			return;
		}

		setIsSubmitting(true);
		try {
			const { feedbackClient, taskClient } = await import('@/lib/client');

			// Add feedback
			const response = await feedbackClient.addFeedback({
				projectId,
				taskId,
				type: formData.type,
				text: formData.text.trim(),
				timing: formData.timing,
				file: formData.file.trim(),
				line: formData.line,
			});

			// If timing is NOW and task is running, pause the task
			if (formData.timing === FeedbackTiming.NOW && isTaskRunning) {
				try {
					await taskClient.pauseTask({ projectId, taskId });
					onTaskPaused?.();
				} catch (pauseError) {
					console.error('Failed to pause task:', pauseError);
					// Don't fail the feedback creation, just warn
					onError?.('Feedback added but failed to pause task');
				}
			}

			// Success
			onFeedbackAdded?.(response.feedback!);

			// Reset form and reload pending feedback
			setFormData(initialFormData);
			await loadPendingFeedback();

		} catch (error) {
			console.error('Failed to add feedback:', error);
			onError?.('Failed to add feedback');
		} finally {
			setIsSubmitting(false);
		}
	}, [formData, validateForm, projectId, taskId, isTaskRunning, onFeedbackAdded, onTaskPaused, onError, loadPendingFeedback]);

	const handleSendAll = useCallback(async () => {
		setIsSending(true);
		try {
			const { feedbackClient } = await import('@/lib/client');
			await feedbackClient.sendFeedback({ projectId, taskId });

			// Reload pending feedback (should be empty now)
			await loadPendingFeedback();
		} catch (error) {
			console.error('Failed to send feedback:', error);
			onError?.('Failed to send feedback');
		} finally {
			setIsSending(false);
		}
	}, [projectId, taskId, onError, loadPendingFeedback]);

	const handleDeleteFeedback = useCallback(async (feedbackId: string) => {
		try {
			const { feedbackClient } = await import('@/lib/client');
			await feedbackClient.deleteFeedback({ projectId, taskId, feedbackId });

			// Reload pending feedback
			await loadPendingFeedback();
		} catch (error) {
			console.error('Failed to delete feedback:', error);
			onError?.('Failed to delete feedback');
		}
	}, [projectId, taskId, onError, loadPendingFeedback]);

	const showInlineFields = formData.type === FeedbackType.INLINE;
	const hasPermissionToSubmit = isTaskRunning || formData.timing === FeedbackTiming.MANUAL;

	return (
		<div role="region" aria-label="Feedback to Agent" className="feedback-panel">
			<div className="feedback-panel-header">
				<Icon name="message-circle" className="w-4 h-4" />
				<h3>Feedback to Agent</h3>
				{pendingFeedback.length > 0 && (
					<span className="feedback-count">{pendingFeedback.length}</span>
				)}
			</div>

			{/* Pending feedback list */}
			{!isLoading && pendingFeedback.length > 0 && (
				<div className="pending-feedback">
					<div className="pending-feedback-header">
						<span>Pending feedback ({pendingFeedback.length}):</span>
						<Button
							variant="ghost"
							size="sm"
							onClick={handleSendAll}
							disabled={isSending}
							className="send-all-button"
						>
							{isSending ? 'Sending...' : 'Send All'}
						</Button>
					</div>
					<div className="feedback-list">
						{pendingFeedback.map((feedback) => (
							<div key={feedback.id} className="feedback-item">
								<div className="feedback-item-header">
									<Icon
										name={feedback.file ? "pin" : "message-square"}
										className="w-3 h-3"
									/>
									<span className="feedback-type">
										{FEEDBACK_TYPE_LABELS[feedback.type]}
									</span>
									{feedback.file && (
										<span className="feedback-location">
											{feedback.file}:{feedback.line}
										</span>
									)}
									<Button
										variant="ghost"
										size="sm"
										onClick={() => handleDeleteFeedback(feedback.id)}
										className="delete-button"
										aria-label="Delete feedback"
									>
										<Icon name="x" className="w-3 h-3" />
									</Button>
								</div>
								<p className="feedback-text">{feedback.text}</p>
							</div>
						))}
					</div>
				</div>
			)}

			{/* Feedback form */}
			<form onSubmit={handleSubmit} className="feedback-form">
				<div className="form-row">
					<div className="form-group">
						<label htmlFor="feedback-type">Type</label>
						<select
							id="feedback-type"
							value={formData.type}
							onChange={(e) => handleInputChange('type', parseInt(e.target.value) as FeedbackType)}
							className="form-select"
						>
							{Object.entries(FEEDBACK_TYPE_LABELS).map(([value, label]) => (
								<option key={value} value={value}>
									{label}
								</option>
							))}
						</select>
					</div>

					<div className="form-group">
						<label htmlFor="feedback-timing">Timing</label>
						<select
							id="feedback-timing"
							value={formData.timing}
							onChange={(e) => handleInputChange('timing', parseInt(e.target.value) as FeedbackTiming)}
							className="form-select"
							aria-label="Timing"
						>
							{Object.entries(FEEDBACK_TIMING_LABELS).map(([value, label]) => (
								<option key={value} value={value}>
									{label}
								</option>
							))}
						</select>
					</div>
				</div>

				{/* Inline comment fields */}
				{showInlineFields && (
					<div className="form-row">
						<div className="form-group">
							<label htmlFor="feedback-file">File</label>
							<input
								id="feedback-file"
								type="text"
								value={formData.file}
								onChange={(e) => handleInputChange('file', e.target.value)}
								placeholder="e.g. src/main.go"
								className="form-input"
							/>
						</div>
						<div className="form-group">
							<label htmlFor="feedback-line">Line</label>
							<input
								id="feedback-line"
								type="number"
								value={formData.line || ''}
								onChange={(e) => handleInputChange('line', parseInt(e.target.value) || 0)}
								min="1"
								placeholder="Line number"
								className="form-input"
							/>
						</div>
					</div>
				)}

				<div className="form-group">
					<label htmlFor="feedback-text">Feedback Text</label>
					<textarea
						id="feedback-text"
						value={formData.text}
						onChange={(e) => handleInputChange('text', e.target.value)}
						placeholder="Add a note..."
						rows={3}
						className="form-textarea"
						aria-label="feedback text"
					/>
				</div>

				<div className="form-actions">
					<Button
						type="submit"
						disabled={isSubmitting || !hasPermissionToSubmit}
						className="submit-button"
						role="button"
						aria-label="add feedback"
					>
						{isSubmitting ? 'Adding...' : 'Add Feedback'}
					</Button>

					{formData.timing === FeedbackTiming.NOW && (
						<span className="timing-warning">
							<Icon name="alert-triangle" className="w-4 h-4" />
							Will pause task
						</span>
					)}
				</div>
			</form>

			{!isTaskRunning && formData.timing !== FeedbackTiming.MANUAL && (
				<div className="disabled-notice">
					<Icon name="info" className="w-4 h-4" />
					<span>Feedback can only be sent to running tasks</span>
				</div>
			)}
		</div>
	);
}

export default FeedbackPanel;