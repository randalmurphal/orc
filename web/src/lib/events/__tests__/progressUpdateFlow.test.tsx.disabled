import { describe, it, expect, vi, beforeEach } from 'vitest';
import { EventService } from '../EventService';
import { taskStore } from '../../../stores/taskStore';
import { handleEvent } from '../handlers';
import { Event, EventPayload } from '../../../gen/orc/v1/events_pb';
import { Task, TaskStatus, ExecutionState, PhaseStatus } from '../../../gen/orc/v1/task_pb';

// Mock WebSocket and gRPC streaming
global.WebSocket = vi.fn().mockImplementation(() => ({
  send: vi.fn(),
  close: vi.fn(),
  readyState: WebSocket.OPEN,
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
}));

vi.mock('../../../stores/taskStore', () => ({
  taskStore: {
    updateTask: vi.fn(),
    updateTaskState: vi.fn(),
    updateTaskActivity: vi.fn(),
    updateTaskOutput: vi.fn(),
    getTask: vi.fn(),
    setTasks: vi.fn(),
  },
}));

vi.mock('../../../stores/sessionStore', () => ({
  sessionStore: {
    updateFromMetricsEvent: vi.fn(),
  },
}));

describe('Real-Time Progress Update Event Flow Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('SC-6.1: complete event flow from backend to UI state updates', async () => {
    const taskId = 'TASK-001';

    // Mock initial task state
    const mockTask = new Task({
      id: taskId,
      title: 'Test Task',
      status: TaskStatus.RUNNING,
      currentPhase: 'implement',
    });

    const mockExecutionState = new ExecutionState({
      currentIteration: 1,
      phases: {
        'implement': {
          status: PhaseStatus.PENDING,
          iterations: 1,
        }
      }
    });

    taskStore.getTask = vi.fn().mockReturnValue(mockTask);

    // 1. Simulate phase change event from backend
    const phaseChangeEvent = new Event({
      id: 'event-1',
      type: 'phase',
      taskId: taskId,
      timestamp: new Date().toISOString(),
      payload: {
        case: 'phaseChanged',
        value: {
          phase: 'review',
          status: 'running',
          commitSha: 'abc123',
        }
      }
    });

    // Handle the event
    await handleEvent(phaseChangeEvent);

    // Should update task current phase
    expect(taskStore.updateTask).toHaveBeenCalledWith(taskId, {
      currentPhase: 'review'
    });

    // 2. Simulate activity change event
    const activityEvent = new Event({
      id: 'event-2',
      type: 'activity',
      taskId: taskId,
      timestamp: new Date().toISOString(),
      payload: {
        case: 'activity',
        value: {
          phase: 'review',
          activity: 'spec_analyzing',
        }
      }
    });

    await handleEvent(activityEvent);

    // Should update task activity
    expect(taskStore.updateTaskActivity).toHaveBeenCalledWith(taskId, 'review', 'spec_analyzing');

    // 3. Simulate transcript event
    const transcriptEvent = new Event({
      id: 'event-3',
      type: 'transcript',
      taskId: taskId,
      timestamp: new Date().toISOString(),
      payload: {
        case: 'transcript',
        value: {
          line: '✓ Code review analysis completed',
          type: 'success',
        }
      }
    });

    await handleEvent(transcriptEvent);

    // Should update task output
    expect(taskStore.updateTaskOutput).toHaveBeenCalledWith(taskId, {
      line: '✓ Code review analysis completed',
      type: 'success',
      timestamp: expect.any(String),
    });

    // 4. Simulate token usage update
    const tokensEvent = new Event({
      id: 'event-4',
      type: 'tokens',
      taskId: taskId,
      timestamp: new Date().toISOString(),
      payload: {
        case: 'tokensUpdated',
        value: {
          input: 2500,
          output: 1800,
          total: 4300,
        }
      }
    });

    await handleEvent(tokensEvent);

    // Should update execution state tokens
    expect(taskStore.updateTaskState).toHaveBeenCalledWith(taskId, {
      tokens: {
        input: 2500,
        output: 1800,
        total: 4300,
      }
    });
  });

  it('SC-6.2: session metrics broadcast updates running task displays', async () => {
    const sessionStore = require('../../../stores/sessionStore').sessionStore;

    // Simulate session metrics event
    const sessionMetricsEvent = new Event({
      id: 'session-1',
      type: 'session_update',
      taskId: '*', // Global event
      timestamp: new Date().toISOString(),
      payload: {
        case: 'sessionMetrics',
        value: {
          durationSeconds: 300,
          totalTokens: 15420,
          estimatedCostUSD: 0.45,
          inputTokens: 8200,
          outputTokens: 7220,
          tasksRunning: 2,
          isPaused: false,
        }
      }
    });

    await handleEvent(sessionMetricsEvent);

    // Should update session store with new metrics
    expect(sessionStore.updateFromMetricsEvent).toHaveBeenCalledWith({
      durationSeconds: 300,
      totalTokens: 15420,
      estimatedCostUSD: 0.45,
      inputTokens: 8200,
      outputTokens: 7220,
      tasksRunning: 2,
      isPaused: false,
    });
  });

  it('SC-6.3: handles rapid event sequences without lost updates', async () => {
    const taskId = 'TASK-002';

    // Simulate rapid sequence of activity changes
    const events = [
      { activity: 'waiting_api', timestamp: 1000 },
      { activity: 'streaming', timestamp: 1100 },
      { activity: 'running_tool', timestamp: 1200 },
      { activity: 'streaming', timestamp: 1300 },
      { activity: 'idle', timestamp: 1400 },
    ];

    // Process events in rapid succession
    await Promise.all(
      events.map(async ({ activity, timestamp }) => {
        const event = new Event({
          id: `event-${timestamp}`,
          type: 'activity',
          taskId: taskId,
          timestamp: new Date(timestamp).toISOString(),
          payload: {
            case: 'activity',
            value: {
              phase: 'implement',
              activity: activity,
            }
          }
        });

        return handleEvent(event);
      })
    );

    // All events should be processed
    expect(taskStore.updateTaskActivity).toHaveBeenCalledTimes(5);

    // Final state should be the last event
    expect(taskStore.updateTaskActivity).toHaveBeenLastCalledWith(taskId, 'implement', 'idle');
  });

  it('SC-6.4: gracefully handles malformed or missing events', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    // Event with missing payload
    const malformedEvent = new Event({
      id: 'bad-event-1',
      type: 'phase',
      taskId: 'TASK-003',
      timestamp: new Date().toISOString(),
      // payload is missing
    });

    await handleEvent(malformedEvent);

    // Should log error but not crash
    expect(consoleSpy).toHaveBeenCalledWith(
      expect.stringContaining('Error handling event'),
      expect.any(Error)
    );

    // Should not call store methods with invalid data
    expect(taskStore.updateTask).not.toHaveBeenCalled();

    consoleSpy.mockRestore();
  });

  it('SC-6.5: maintains event order for same task across different event types', async () => {
    const taskId = 'TASK-004';
    let callOrder: string[] = [];

    // Mock store methods to track call order
    taskStore.updateTask = vi.fn().mockImplementation(() => {
      callOrder.push('updateTask');
    });

    taskStore.updateTaskActivity = vi.fn().mockImplementation(() => {
      callOrder.push('updateTaskActivity');
    });

    taskStore.updateTaskOutput = vi.fn().mockImplementation(() => {
      callOrder.push('updateTaskOutput');
    });

    // Send events in specific order with small time gaps
    const phaseEvent = new Event({
      id: 'event-phase',
      type: 'phase',
      taskId: taskId,
      timestamp: new Date(1000).toISOString(),
      payload: {
        case: 'phaseChanged',
        value: { phase: 'test', status: 'running' }
      }
    });

    const activityEvent = new Event({
      id: 'event-activity',
      type: 'activity',
      taskId: taskId,
      timestamp: new Date(1001).toISOString(),
      payload: {
        case: 'activity',
        value: { phase: 'test', activity: 'running_tool' }
      }
    });

    const outputEvent = new Event({
      id: 'event-output',
      type: 'transcript',
      taskId: taskId,
      timestamp: new Date(1002).toISOString(),
      payload: {
        case: 'transcript',
        value: { line: 'Test started', type: 'info' }
      }
    });

    // Process in order
    await handleEvent(phaseEvent);
    await handleEvent(activityEvent);
    await handleEvent(outputEvent);

    // Verify order was maintained
    expect(callOrder).toEqual(['updateTask', 'updateTaskActivity', 'updateTaskOutput']);
  });

  it('SC-6.6: handles WebSocket reconnection with state synchronization', async () => {
    const eventService = new EventService();
    const mockWebSocket = {
      send: vi.fn(),
      close: vi.fn(),
      readyState: WebSocket.OPEN,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    };

    // Mock WebSocket constructor to return our mock
    global.WebSocket = vi.fn().mockImplementation(() => mockWebSocket);

    // Simulate connection establishment
    await eventService.connect();

    // Simulate connection loss and reconnect
    mockWebSocket.readyState = WebSocket.CLOSED;

    // Reconnect should trigger state sync
    await eventService.reconnect();

    // Should send subscription requests for active tasks
    expect(mockWebSocket.send).toHaveBeenCalledWith(
      JSON.stringify({ type: 'subscribe', task_id: '*' })
    );
  });

  it('SC-6.7: filters events based on subscription preferences', async () => {
    const taskId = 'TASK-005';

    // Mock filtered subscription (only phase and activity events)
    const filteredEvents = ['phase', 'activity'];

    // Event that should be processed
    const phaseEvent = new Event({
      id: 'event-1',
      type: 'phase',
      taskId: taskId,
      timestamp: new Date().toISOString(),
      payload: {
        case: 'phaseChanged',
        value: { phase: 'implement', status: 'running' }
      }
    });

    // Event that should be filtered out
    const heartbeatEvent = new Event({
      id: 'event-2',
      type: 'heartbeat',
      taskId: taskId,
      timestamp: new Date().toISOString(),
      payload: {
        case: 'heartbeat',
        value: { phase: 'implement', iteration: 1 }
      }
    });

    // Process events with filter
    const shouldProcessPhase = filteredEvents.includes(phaseEvent.type);
    const shouldProcessHeartbeat = filteredEvents.includes(heartbeatEvent.type);

    if (shouldProcessPhase) {
      await handleEvent(phaseEvent);
    }
    if (shouldProcessHeartbeat) {
      await handleEvent(heartbeatEvent);
    }

    // Only phase event should be processed
    expect(taskStore.updateTask).toHaveBeenCalledWith(taskId, {
      currentPhase: 'implement'
    });

    // No heartbeat processing
    expect(taskStore.updateTaskActivity).not.toHaveBeenCalled();
  });
});