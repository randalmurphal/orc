// Package automation provides trigger-based automation for orc tasks.
package automation

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// CountEvaluator evaluates count-based triggers.
// Fires when a metric (tasks_completed, phases_completed, etc.) reaches threshold.
type CountEvaluator struct{}

func (e *CountEvaluator) Type() TriggerType {
	return TriggerTypeCount
}

func (e *CountEvaluator) Evaluate(ctx context.Context, trigger *Trigger, event *Event, svc *Service) (bool, string, error) {
	// Count triggers only respond to completion events
	if event.Type != EventTaskCompleted && event.Type != EventPhaseCompleted {
		return false, "", nil
	}

	// Check weight filter
	if len(trigger.Condition.Weights) > 0 && event.Weight != "" {
		if !slices.Contains(trigger.Condition.Weights, event.Weight) {
			return false, "", nil
		}
	}

	// Check category filter
	if len(trigger.Condition.Categories) > 0 && event.Category != "" {
		if !slices.Contains(trigger.Condition.Categories, event.Category) {
			return false, "", nil
		}
	}

	// Determine metric to check
	metric := trigger.Condition.Metric
	if metric == "" {
		metric = "tasks_completed"
	}

	// Check if this event contributes to the metric
	shouldCount := false
	switch metric {
	case "tasks_completed":
		shouldCount = event.Type == EventTaskCompleted
	case "large_tasks_completed":
		shouldCount = event.Type == EventTaskCompleted &&
			(event.Weight == "large" || event.Weight == "greenfield")
	case "phases_completed":
		shouldCount = event.Type == EventPhaseCompleted
	}

	if !shouldCount {
		return false, "", nil
	}

	// Atomically increment counter and get new value
	// This prevents race conditions between increment and threshold check
	count, err := svc.db.IncrementAndGetCounter(ctx, trigger.ID, metric)
	if err != nil {
		return false, "", fmt.Errorf("increment and get counter: %w", err)
	}

	if count >= trigger.Condition.Threshold {
		// Reset counter after firing
		if err := svc.db.ResetCounter(ctx, trigger.ID, metric); err != nil {
			svc.logger.Warn("error resetting counter after trigger",
				"trigger", trigger.ID,
				"metric", metric,
				"error", err)
		}
		return true, fmt.Sprintf("%d %s reached threshold of %d", count, metric, trigger.Condition.Threshold), nil
	}

	return false, "", nil
}

// InitiativeEvaluator evaluates initiative-based triggers.
// Fires on initiative events (completed, started).
type InitiativeEvaluator struct{}

func (e *InitiativeEvaluator) Type() TriggerType {
	return TriggerTypeInitiative
}

func (e *InitiativeEvaluator) Evaluate(ctx context.Context, trigger *Trigger, event *Event, svc *Service) (bool, string, error) {
	// Check if event matches the trigger's event condition
	if trigger.Condition.Event == "" {
		return false, "", nil
	}

	if event.Type != trigger.Condition.Event {
		return false, "", nil
	}

	// Check filter conditions
	if len(trigger.Condition.Filter) > 0 {
		for key, value := range trigger.Condition.Filter {
			if event.Metadata == nil {
				return false, "", nil
			}
			if event.Metadata[key] != value {
				return false, "", nil
			}
		}
	}

	reason := fmt.Sprintf("event %s occurred", event.Type)
	if event.TaskID != "" {
		reason += fmt.Sprintf(" for %s", event.TaskID)
	}

	return true, reason, nil
}

// EventEvaluator evaluates event-based triggers.
// Fires on specific events (pr_merged, task_completed, etc.)
type EventEvaluator struct{}

func (e *EventEvaluator) Type() TriggerType {
	return TriggerTypeEvent
}

func (e *EventEvaluator) Evaluate(ctx context.Context, trigger *Trigger, event *Event, svc *Service) (bool, string, error) {
	// Check if event matches
	if trigger.Condition.Event == "" {
		return false, "", nil
	}

	if event.Type != trigger.Condition.Event {
		return false, "", nil
	}

	// Check weight filter in filter map
	if len(trigger.Condition.Filter) > 0 {
		if weights, ok := trigger.Condition.Filter["weights"]; ok {
			// Filter weights is a comma-separated string
			// For simplicity, check if event weight is mentioned
			if event.Weight != "" && weights != "" {
				if !containsWeight(weights, event.Weight) {
					return false, "", nil
				}
			}
		}

		// Check other filter conditions
		for key, value := range trigger.Condition.Filter {
			if key == "weights" {
				continue // Already handled
			}
			if event.Metadata == nil {
				return false, "", nil
			}
			if event.Metadata[key] != value {
				return false, "", nil
			}
		}
	}

	reason := fmt.Sprintf("event %s", event.Type)
	if event.TaskID != "" {
		reason += fmt.Sprintf(" for %s", event.TaskID)
	}

	return true, reason, nil
}

// containsWeight checks if a comma-separated weights string contains a weight.
func containsWeight(weights, weight string) bool {
	if weights == "" || weight == "" {
		return false
	}
	parts := strings.Split(weights, ",")
	for _, p := range parts {
		if strings.TrimSpace(p) == weight {
			return true
		}
	}
	return false
}

// ThresholdEvaluator evaluates threshold-based triggers.
// Fires when a metric crosses a value.
type ThresholdEvaluator struct{}

func (e *ThresholdEvaluator) Type() TriggerType {
	return TriggerTypeThreshold
}

func (e *ThresholdEvaluator) Evaluate(ctx context.Context, trigger *Trigger, event *Event, svc *Service) (bool, string, error) {
	// Threshold triggers check metrics after relevant events
	// For now, check after any task completion that might have generated metrics
	if event.Type != EventTaskCompleted && event.Type != EventPhaseCompleted {
		return false, "", nil
	}

	// Get latest metric value
	metric, err := svc.db.GetLatestMetric(ctx, trigger.Condition.Metric)
	if err != nil {
		// No metric recorded yet, don't fire
		return false, "", nil
	}

	// Evaluate condition
	var shouldFire bool
	switch trigger.Condition.Operator {
	case "lt", "<":
		shouldFire = metric.Value < trigger.Condition.Value
	case "gt", ">":
		shouldFire = metric.Value > trigger.Condition.Value
	case "eq", "=", "==":
		shouldFire = metric.Value == trigger.Condition.Value
	case "lte", "<=":
		shouldFire = metric.Value <= trigger.Condition.Value
	case "gte", ">=":
		shouldFire = metric.Value >= trigger.Condition.Value
	default:
		return false, "", fmt.Errorf("unknown operator: %s", trigger.Condition.Operator)
	}

	if !shouldFire {
		return false, "", nil
	}

	reason := fmt.Sprintf("%s is %.2f (threshold: %s %.2f)",
		trigger.Condition.Metric,
		metric.Value,
		trigger.Condition.Operator,
		trigger.Condition.Value)

	return true, reason, nil
}

// ScheduleEvaluator evaluates schedule-based triggers.
// Only available in team mode with a persistent server.
type ScheduleEvaluator struct{}

func (e *ScheduleEvaluator) Type() TriggerType {
	return TriggerTypeSchedule
}

func (e *ScheduleEvaluator) Evaluate(ctx context.Context, trigger *Trigger, event *Event, svc *Service) (bool, string, error) {
	// Schedule evaluation is handled by a separate scheduler
	// This evaluator is called from the scheduler, not from events
	// For now, return false - the scheduler will directly call fireTrigger
	return false, "", nil
}
