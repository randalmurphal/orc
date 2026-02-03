/**
 * Event handlers - Route incoming events to the appropriate stores
 *
 * This module processes typed Event payloads (discriminated unions)
 * and dispatches updates to Zustand stores.
 */

import type { Event } from '@/gen/orc/v1/events_pb';
import { ActivityState } from '@/gen/orc/v1/events_pb';
import { useTaskStore, useInitiativeStore, useSessionStore, useUIStore, toast } from '@/stores';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { create } from '@bufbuild/protobuf';
import { PendingDecisionSchema } from '@/gen/orc/v1/decision_pb';
import { TaskSchema, TaskStatus, TaskQueue, TaskPriority, TaskCategory, PhaseStatus, PhaseStateSchema } from '@/gen/orc/v1/task_pb';
import { InitiativeSchema, InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import type { PhaseStatus as UIPhaseStatus } from '@/components/workflow-editor/nodes';
import { estimatePhaseCompletion } from '@/lib/utils/progressEstimation';
import type { SessionMetrics, PhaseProgress } from '@/components/common/RealTimeMetrics';
import type { Task, ExecutionState } from '@/gen/orc/v1/task_pb';

/**
 * Interface for the subset of TaskStore methods used by event handlers
 */
interface TaskStoreActions {
	getRunningTasks: () => Task[];
	getTaskState: (taskId: string) => ExecutionState | undefined;
	updateSessionMetrics: (taskId: string, metrics: SessionMetrics) => void;
	updatePhaseProgress: (taskId: string, progress: PhaseProgress) => void;
}

/**
 * Metrics payload from session_metrics events
 */
interface GlobalMetrics {
	totalTokens: number;
	estimatedCostUsd: number;
	inputTokens: number;
	outputTokens: number;
	durationSeconds: number | bigint;
}

/**
 * Convert proto ActivityState enum to string format expected by components
 */
function getActivityStateString(activity: ActivityState): string {
	switch (activity) {
		case ActivityState.IDLE:
			return 'idle';
		case ActivityState.WAITING_API:
			return 'waiting_api';
		case ActivityState.STREAMING:
			return 'streaming';
		case ActivityState.RUNNING_TOOL:
			return 'running_tool';
		case ActivityState.PROCESSING:
			return 'processing';
		case ActivityState.SPEC_ANALYZING:
			return 'spec_analyzing';
		case ActivityState.SPEC_WRITING:
			return 'spec_writing';
		case ActivityState.UNSPECIFIED:
		default:
			return 'unknown_activity';
	}
}

/**
 * Update task-specific session metrics for running tasks
 */
function updateTaskSpecificMetrics(taskStore: TaskStoreActions, globalMetrics: GlobalMetrics): void {
	// Get running tasks and distribute metrics proportionally
	const runningTasks = taskStore.getRunningTasks();

	if (runningTasks.length === 0) {
		return;
	}

	// For now, divide metrics equally among running tasks
	// In a real implementation, this might be more sophisticated
	const tasksRunning = runningTasks.length;
	const tokensPerTask = Math.floor(globalMetrics.totalTokens / tasksRunning);
	const costPerTask = globalMetrics.estimatedCostUsd / tasksRunning;

	runningTasks.forEach((task: Task) => {
		const taskMetrics: SessionMetrics = {
			totalTokens: tokensPerTask,
			estimatedCostUSD: costPerTask,
			inputTokens: Math.floor(globalMetrics.inputTokens / tasksRunning),
			outputTokens: Math.floor(globalMetrics.outputTokens / tasksRunning),
			durationSeconds: Number(globalMetrics.durationSeconds),
			tasksRunning: 1 // This task specifically
		};

		taskStore.updateSessionMetrics(task.id, taskMetrics);
	});
}

/**
 * Update phase progress data when activity changes
 */
function updatePhaseProgressFromActivity(taskStore: TaskStoreActions, taskId: string, phaseId: string, activity: string): void {
	const existingState = taskStore.getTaskState(taskId);

	if (!existingState) {
		return;
	}

	const phaseState = existingState.phases[phaseId];
	if (!phaseState) {
		return;
	}

	const iterations = phaseState.iterations || 1;
	const phaseStartTime = phaseState.startedAt?.seconds ?
		Number(phaseState.startedAt.seconds) * 1000 : Date.now();

	// Compute progress estimation
	const estimatedCompletion = estimatePhaseCompletion(activity, phaseStartTime);

	const phaseProgress: PhaseProgress = {
		iterations,
		currentActivity: activity,
		estimatedCompletion
	};

	taskStore.updatePhaseProgress(taskId, phaseProgress);
}

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

			// Update ExecutionState phases for real-time progress
			const existingState = taskStore.getTaskState(taskId);
			if (existingState) {
				// Create proper PhaseState proto object
				const phaseState = create(PhaseStateSchema, {
					status,
					iterations: iteration,
					...(error && { error }),
				});

				const updatedState = {
					...existingState,
					phases: {
						...existingState.phases,
						[phaseName]: phaseState
					}
				};
				taskStore.updateTaskState(taskId, updatedState);
			}

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
			// Activity is proto ActivityState enum - convert to string for store
			const activityString = getActivityStateString(activity);
			taskStore.updateTaskActivity(taskId, phaseId, activity);

			// Compute and update phase progress when activity changes
			updatePhaseProgressFromActivity(taskStore, taskId, phaseId, activityString);
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

			// Update global session metrics
			sessionStore.updateFromMetricsEvent({
				durationSeconds: metrics.durationSeconds,
				totalTokens: metrics.totalTokens,
				estimatedCostUsd: metrics.estimatedCostUsd,
				inputTokens: metrics.inputTokens,
				outputTokens: metrics.outputTokens,
				tasksRunning: metrics.tasksRunning,
				isPaused: metrics.isPaused,
			});

			// Update task-specific metrics for running tasks
			updateTaskSpecificMetrics(taskStore, metrics);
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

