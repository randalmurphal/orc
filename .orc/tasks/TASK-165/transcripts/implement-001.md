# implement - Iteration 1

## Prompt

Implement the following feature/task:

**Task**: Phase 1: Create useWebSocket hook and WebSocketProvider

**Description**: ## Purpose
Port the WebSocket client to React with proper hook-based API.

## Implementation

### WebSocket Client (port from websocket.ts)
- OrcWebSocket class (keep similar structure)
- Connection lifecycle: connect, reconnect, disconnect
- Subscription model: global (*) vs task-specific
- Auto-reconnect with exponential backoff
- Ping/pong heartbeat (30s interval)
- Primary subscription restoration on reconnect

### React Integration

#### WebSocketProvider
- Context provider wrapping app
- Manages single WebSocket instance
- Exposes connection status

#### useWebSocket hook
```typescript
const { status, subscribe, unsubscribe } = useWebSocket()
```

#### useTaskSubscription hook
```typescript
const { state, transcript } = useTaskSubscription('TASK-001')
```

### Event Handling
- state -> update TaskStore
- transcript -> streaming to component
- task_created/updated/deleted -> update TaskStore
- error -> toast notification

## Success Criteria
- WebSocket connects and reconnects correctly
- Events update stores in real-time
- Connection status visible in UI
- No memory leaks on unmount



1. Plan your implementation approach
2. Implement the required changes
3. Write/update tests alongside code
4. Run tests and fix any failures

Keep iterating until implementation is complete and tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

Let me verify everything is complete by checking all the success criteria from the spec:

**✅ WebSocket connects and reconnects correctly**
- `OrcWebSocket` class with `connect()`, auto-reconnect with exponential backoff (1s, 2s, 4s... up to 5 attempts)
- Tests verify: `websocket.test.ts > reconnection` tests all pass

**✅ Events update stores in real-time**
- `useGlobalTaskEvents` hook handles task_created/updated/deleted/state events
- `useTaskSubscription` hook tracks state, transcript, tokens, finalize events per task
- `WebSocketProvider` has `onTaskEvent` callback for forwarding events

**✅ Connection status visible in UI**
- `ConnectionStatus` component in App.tsx shows connected/connecting/reconnecting/disconnected
- `useWebSocket` hook provides `status` and `isConnected`

**✅ No memory leaks on unmount**
- All event listeners return unsubscribe functions
- `useTaskSubscription` cleans up on taskId change and unmount
- `useGlobalTaskEvents` uses ref to avoid stale closures
- Provider doesn't disconnect on unmount (singleton pattern for shared usage)

All implementation is complete and tests pass (57/57). The phase is done!

<phase_complete>true</phase_complete>

---
Tokens: 851351 input, 2941 output, 80525 cache_creation, 770811 cache_read
Complete: true
Blocked: false
