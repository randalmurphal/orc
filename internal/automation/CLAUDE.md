# Automation Package

Trigger-based automation for orc tasks. Enables automatic execution of maintenance
and workflow tasks based on configurable conditions.

## Overview

The automation system fires triggers based on:
- **Count-based**: After N tasks/phases complete
- **Initiative-based**: On initiative events (completed, started)
- **Event-based**: On specific events (pr_merged, task_completed)
- **Threshold-based**: When metrics cross values (coverage < 80%)
- **Schedule-based**: Cron expressions (team mode only)

## Key Types

| Type | Purpose |
|------|---------|
| `Trigger` | Trigger definition with condition, action, cooldown |
| `Execution` | Record of a trigger firing |
| `Template` | Automation task template |
| `Counter` | Task count tracking for count-based triggers |
| `Metric` | Metric values for threshold-based triggers |
| `Notification` | User notification for pending/failed tasks |
| `Stats` | Automation statistics |

## Trigger Types

| Type | Fires When | Solo Mode | Team Mode |
|------|------------|-----------|-----------|
| `count` | N tasks complete | Yes | Yes |
| `initiative` | Initiative events | Yes | Yes |
| `event` | Specific events | Yes | Yes |
| `threshold` | Metric crosses value | Yes | Yes |
| `schedule` | Cron expression | No | Yes |

## Execution Modes

| Mode | Behavior |
|------|----------|
| `auto` | Fire and execute without prompts |
| `approval` | Create pending task, require human approval |
| `notify` | Only notify, human creates task manually |

## Cooldown

Cooldowns prevent trigger storms. In solo mode, cooldowns are task-count based:

```yaml
cooldown: 5 tasks  # Don't retrigger for 5 more task completions
```

In team mode, time-based cooldowns are also available:

```yaml
cooldown: 2h  # Don't retrigger for 2 hours
```

## Database Tables

| Table | Purpose |
|-------|---------|
| `automation_triggers` | Trigger definitions and state |
| `trigger_executions` | Execution history |
| `trigger_counters` | Count tracking per metric |
| `trigger_metrics` | Threshold metric values |
| `notifications` | User notifications |

## Usage

```go
import "github.com/randalmurphal/orc/internal/automation"

// Create a trigger
trigger := &automation.Trigger{
    ID:          "style-normalization",
    Type:        automation.TriggerTypeCount,
    Description: "Normalize code style after changes",
    Enabled:     true,
    Mode:        automation.ModeAuto,
    Condition: automation.Condition{
        Metric:    "tasks_completed",
        Threshold: 5,
    },
    Action: automation.Action{
        Template: "style-normalization",
        Priority: "low",
    },
    Cooldown: automation.Cooldown{Tasks: 5},
}
```

## Files

| File | Purpose |
|------|---------|
| `types.go` | Core types and enums |
| `service.go` | Trigger service with event handling and cooldowns |
| `evaluators.go` | Trigger evaluators (count, initiative, event, threshold, schedule) |
| `db.go` | Database operations via ProjectDBAdapter |

## Service Architecture

The automation service:
1. Receives events from executor (task/phase completion, PR merged, etc.)
2. Evaluates enabled triggers against incoming events
3. Respects cooldowns (global and per-trigger, task-count or time-based)
4. Fires matching triggers based on execution mode (auto/approval/notify)

```go
import "github.com/randalmurphal/orc/internal/automation"

// Create service with database adapter
adapter := automation.NewProjectDBAdapter(pdb)
svc := automation.NewService(cfg, adapter, logger)

// Handle event from executor
event := &automation.Event{
    Type:     automation.EventTaskCompleted,
    TaskID:   "TASK-001",
    Weight:   "medium",
    Category: "feature",
}
svc.HandleEvent(ctx, event)
```

## Evaluator Interface

Custom evaluators can be registered:

```go
type Evaluator interface {
    Type() TriggerType
    Evaluate(ctx, trigger, event, svc) (shouldFire bool, reason string, err error)
}

svc.RegisterEvaluator(&MyCustomEvaluator{})
```

## Event Types

| Event | Description |
|-------|-------------|
| `task_completed` | Task finished successfully |
| `task_failed` | Task execution failed |
| `phase_completed` | Single phase completed |
| `phase_failed` | Phase execution failed |
| `pr_merged` | Pull request merged |
| `pr_approved` | Pull request approved |
| `initiative_completed` | Initiative marked complete |
| `initiative_started` | Initiative activated |
