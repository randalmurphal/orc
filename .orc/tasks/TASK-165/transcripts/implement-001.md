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

All 198 tests pass. Let me create a summary of what was implemented:

## Summary

I've successfully implemented the WebSocket client for the React 19 frontend migration:

### Files Created

1. **`web-react/src/lib/websocket.ts`** - OrcWebSocket class
   - Connection lifecycle (connect, reconnect, disconnect)
   - Subscription model (global `*` vs task-specific)
   - Auto-reconnect with exponential backoff (max 5 attempts)
   - Ping/pong heartbeat (30s interval)
   - Primary subscription restoration on reconnect

2. **`web-react/src/hooks/useWebSocket.tsx`** - React integration
   - `WebSocketProvider` - Context provider wrapping the app
   - `useWebSocket` - Access WebSocket functionality
   - `useTaskSubscription` - Subscribe to task events with streaming transcript
   - `useConnectionStatus` - Simple connection status hook

3. **`web-react/src/hooks/index.ts`** - Clean exports

4. **`web-react/src/lib/websocket.test.ts`** - 42 tests for OrcWebSocket class

5. **`web-react/src/hooks/useWebSocket.test.tsx`** - 32 tests for React hooks

### Files Modified

1. **`web-react/src/lib/types.ts`** - Added WebSocket types (WSEventType, WSEvent, WSMessage, etc.)
2. **`web-react/CLAUDE.md`** - Added comprehensive WebSocket hooks documentation

### Key Features

- **Real-time updates**: Events update TaskStore and UIStore automatically
- **Store integration**: `state` events → TaskStore, connection status → UIStore
- **Event handling**: task_created/updated/deleted, state, transcript, phase, tokens, finalize, error
- **Toast notifications**: Errors shown via toast, task deleted notification
- **Memory safe**: Proper cleanup on unmount, no memory leaks

### Test Coverage

- 198 tests passing (74 new WebSocket tests)
- OrcWebSocket class: connection, subscription, commands, reconnection, heartbeat
- React hooks: provider, context, event handling, store integration

<phase_complete>true</phase_complete>

---
Tokens: 4311546 input, 32101 output, 101065 cache_creation, 4201370 cache_read
Complete: true
Blocked: false
