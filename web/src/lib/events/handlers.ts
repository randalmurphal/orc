/**
 * Event handlers - Route incoming events to the appropriate stores
 *
 * This module processes typed Event payloads (discriminated unions)
 * and dispatches updates to Zustand stores.
 */

import type { Event } from '@/gen/orc/v1/events_pb';
import { useTaskStore, useInitiativeStore, useSessionStore, useUIStore, toast } from '@/stores';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { create } from '@bufbuild/protobuf';
import { PendingDecisionSchema } from '@/gen/orc/v1/decision_pb';
import { TaskSchema, TaskStatus, TaskQueue, TaskPriority, TaskCategory, PhaseStatus } from '@/gen/orc/v1/task_pb';
import { InitiativeSchema, InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import type { PhaseStatus as UIPhaseStatus } from '@/components/workflow-editor/nodes';

/**
 * Map proto PhaseStatus to UI PhaseStatus string
 * Used for updating workflow editor nodes from events
 *
 * AMENDMENT AMEND-001: Proto PhaseStatus only has UNSPECIFIED(0), PENDING(1), COMPLETED(3), SKIPPED(7)
 * Values RUNNING, FAILED, BLOCKED were removed - these are now derived from context:
 * - 'running': derived when this phase is the current running phase
 * - 'failed': derived when the phase has an error message
 * - 'blocked': derived from gate blocking conditions (future)
 *
 * @param protoStatus - The PhaseStatus from the proto event
 * @param isCurrentPhase - Whether this phase is the current running phase in a RUNNING run
 * @param hasError - Whether this phase has an error
 */
function mapPhaseStatus(
	protoStatus: PhaseStatus,
	isCurrentPhase: boolean = false,
	hasError: boolean = false
): UIPhaseStatus {
	// Derive failed status from error
	if (hasError) {
		return 'failed';
	}

	// Check proto status for completed/skipped
	switch (protoStatus) {
		case PhaseStatus.COMPLETED:
			return 'completed';
		case PhaseStatus.SKIPPED:
			return 'skipped';
		case PhaseStatus.PENDING:
		case PhaseStatus.UNSPECIFIED:
		default:
			// If this is the current phase in a running run, it's "running"
			// Otherwise it's "pending"
			return isCurrentPhase ? 'running' : 'pending';
	}
}

/**
 * Handle an incoming event by dispatching to the appropriate store.
 *
 * Events use discriminated unions via `event.payload.case`, providing
 * type-safe access to the payload value.
 */
export function handleEvent(event: Event): void {
	const taskStore = useTaskStore.getState();
	const initiativeStore = useInitiativeStore.getState();

	switch (event.payload.case) {
		case 'taskCreated': {
			// TaskCreatedEvent has partial info - add minimal task to store
			const { taskId, title, weight, initiativeId } = event.payload.value;
			// Check if task already exists to avoid duplicates
			const existingTask = taskStore.getTask(taskId);
			if (!existingTask) {
				// Create minimal task with required fields from the event
				const task = create(TaskSchema, {
					id: taskId,
					title,
					weight,
					initiativeId,
					status: TaskStatus.CREATED,
					queue: TaskQueue.ACTIVE,
					priority: TaskPriority.NORMAL,
					category: TaskCategory.FEATURE,
					branch: '',
					blockedBy: [],
					relatedTo: [],
				});
				taskStore.addTask(task);
			}
			break;
		}

		case 'taskUpdated': {
			const { taskId, task } = event.payload.value;
			if (task) {
				// Proto types match directly - no conversion needed
				taskStore.updateTask(taskId, {
					currentPhase: task.currentPhase,
					status: task.status,
				});
			}
			break;
		}

		case 'taskDeleted': {
			const { taskId } = event.payload.value;
			taskStore.removeTask(taskId);
			toast.info(`Task ${taskId} deleted`);
			break;
		}

		case 'phaseChanged': {
			const { taskId, phaseName, status, iteration, error } = event.payload.value;
			// Update task store with current phase
			taskStore.updateTask(taskId, {
				currentPhase: phaseName,
			});

			// TASK-639: Also update workflow editor store if this event matches active run
			const editorStore = useWorkflowEditorStore.getState();
			const activeRun = editorStore.activeRun;
			if (activeRun?.run?.taskId === taskId) {
				// Derive whether this is the current running phase
				// (PENDING status + being the current phase in a RUNNING run = running)
				const isCurrentPhase =
					status === PhaseStatus.PENDING && activeRun.run.currentPhase === phaseName;
				const hasError = !!error;

				// Update node status using context-aware mapping
				const uiStatus = mapPhaseStatus(status, isCurrentPhase, hasError);
				editorStore.updateNodeStatus(phaseName, uiStatus, {
					iterations: iteration,
				});

				// Update edge animations: animate edges when phase is running
				if (uiStatus === 'running') {
					editorStore.updateEdgesForActivePhase(phaseName);
				} else if (uiStatus === 'completed' || uiStatus === 'failed') {
					// When a phase completes/fails, clear edge animations
					// (next phase will start and set them again if needed)
					editorStore.updateEdgesForActivePhase(null);
				}
			}
			break;
		}

		case 'tokensUpdated': {
			const { taskId, tokens } = event.payload.value;
			if (tokens) {
				const existingState = taskStore.getTaskState(taskId);
				if (existingState) {
					// Proto types match directly - no conversion needed
					taskStore.updateTaskState(taskId, {
						...existingState,
						tokens,
					});
				}
			}
			break;
		}

		case 'activity': {
			const { taskId, phaseId, activity } = event.payload.value;
			// Activity is proto ActivityState enum - matches store directly
			taskStore.updateTaskActivity(taskId, phaseId, activity);
			break;
		}

		case 'initiativeCreated': {
			const { initiativeId, title } = event.payload.value;
			// Check if initiative already exists to avoid duplicates
			const existingInitiative = initiativeStore.getInitiative(initiativeId);
			if (!existingInitiative) {
				// Create minimal initiative with required fields from the event
				const initiative = create(InitiativeSchema, {
					id: initiativeId,
					title,
					status: InitiativeStatus.ACTIVE,
					decisions: [],
					contextFiles: [],
					blockedBy: [],
				});
				initiativeStore.addInitiative(initiative);
			}
			break;
		}

		case 'initiativeUpdated': {
			const { initiativeId } = event.payload.value;
			console.warn(`Initiative updated: ${initiativeId}`);
			break;
		}

		case 'initiativeDeleted': {
			const { initiativeId } = event.payload.value;
			initiativeStore.removeInitiative(initiativeId);
			toast.info(`Initiative ${initiativeId} deleted`);
			break;
		}

		case 'decisionRequired': {
			const uiStore = useUIStore.getState();
			const eventData = event.payload.value;
			// Convert event to PendingDecision proto
			// Note: DecisionRequiredEvent doesn't include options - they come from API if needed
			const decision = create(PendingDecisionSchema, {
				id: eventData.decisionId,
				taskId: eventData.taskId,
				taskTitle: eventData.taskTitle,
				phase: eventData.phase,
				gateType: eventData.gateType,
				question: eventData.question,
				context: eventData.context,
				requestedAt: eventData.requestedAt,
				options: [], // Options fetched via API when needed
			});
			uiStore.addPendingDecision(decision);
			toast.warning(`Decision required: ${eventData.taskTitle} - ${eventData.question}`);
			break;
		}

		case 'decisionResolved': {
			const uiStore = useUIStore.getState();
			const { decisionId, taskId, approved } = event.payload.value;
			uiStore.removePendingDecision(decisionId);
			const action = approved ? 'approved' : 'rejected';
			toast.info(`Decision ${action} for task ${taskId}`);
			break;
		}

		case 'filesChanged': {
			// File change events - could update UI indicator
			break;
		}

		case 'sessionUpdate': {
			// SessionInfo contains Claude session metadata (id, model, status)
			const { session } = event.payload.value;
			if (session) {
				console.warn(`Session update: ${session.id} - ${session.status}`);
			}
			break;
		}

		case 'sessionMetrics': {
			// Aggregate session metrics (tokens, cost, etc.)
			const sessionStore = useSessionStore.getState();
			const metrics = event.payload.value;
			sessionStore.updateFromMetricsEvent({
				durationSeconds: metrics.durationSeconds,
				totalTokens: metrics.totalTokens,
				estimatedCostUsd: metrics.estimatedCostUsd,
				inputTokens: metrics.inputTokens,
				outputTokens: metrics.outputTokens,
				tasksRunning: metrics.tasksRunning,
				isPaused: metrics.isPaused,
			});
			break;
		}

		case 'error': {
			const { error, phase } = event.payload.value;
			const message = phase ? `[${phase}] ${error}` : error;
			toast.error(message);
			break;
		}

		case 'warning': {
			const { message, phase } = event.payload.value;
			const msg = phase ? `[${phase}] ${message}` : message;
			toast.warning(msg);
			break;
		}

		case 'heartbeat': {
			// Connection health check - no action needed
			break;
		}

		case undefined: {
			console.warn('Event with undefined payload case:', event);
			break;
		}

		default: {
			// Exhaustiveness check - cast to never to catch unhandled cases
			const _exhaustive: never = event.payload;
			void _exhaustive; // Suppress unused variable warning
			console.warn('Unhandled event payload case:', event.payload);
		}
	}
}
