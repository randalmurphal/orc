package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// GetValue retrieves a config value by dot-separated path (e.g., "gates.default_type").
// Returns the value as a string and any error encountered.
func (c *Config) GetValue(path string) (string, error) {
	v, err := getValueByPath(reflect.ValueOf(c), path)
	if err != nil {
		return "", err
	}
	return formatValue(v), nil
}

// SetValue sets a config value by dot-separated path.
// The value is parsed based on the target field's type.
func (c *Config) SetValue(path, value string) error {
	return setValueByPath(reflect.ValueOf(c).Elem(), path, value)
}

// getValueByPath traverses a reflect.Value by dot-separated path.
func getValueByPath(v reflect.Value, path string) (reflect.Value, error) {
	if path == "" {
		return v, nil
	}

	parts := strings.SplitN(path, ".", 2)
	fieldName := parts[0]
	remaining := ""
	if len(parts) > 1 {
		remaining = parts[1]
	}

	// Dereference pointer
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}, fmt.Errorf("nil pointer at %s", fieldName)
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("expected struct, got %s", v.Kind())
	}

	// Find field by yaml tag or name
	field := findFieldByTag(v, fieldName)
	if !field.IsValid() {
		return reflect.Value{}, fmt.Errorf("unknown config key: %s", fieldName)
	}

	if remaining == "" {
		return field, nil
	}

	return getValueByPath(field, remaining)
}

// setValueByPath sets a value at the given path.
func setValueByPath(v reflect.Value, path, value string) error {
	parts := strings.SplitN(path, ".", 2)
	fieldName := parts[0]
	remaining := ""
	if len(parts) > 1 {
		remaining = parts[1]
	}

	// Dereference pointer
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return fmt.Errorf("nil pointer at %s", fieldName)
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", v.Kind())
	}

	// Find field by yaml tag or name
	field := findFieldByTag(v, fieldName)
	if !field.IsValid() {
		return fmt.Errorf("unknown config key: %s", fieldName)
	}

	if !field.CanSet() {
		return fmt.Errorf("cannot set field: %s", fieldName)
	}

	if remaining != "" {
		return setValueByPath(field, remaining, value)
	}

	return setFieldValue(field, value)
}

// findFieldByTag finds a struct field by its yaml tag or name.
func findFieldByTag(v reflect.Value, name string) reflect.Value {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Check yaml tag
		yamlTag := field.Tag.Get("yaml")
		if yamlTag != "" {
			tagName := strings.Split(yamlTag, ",")[0]
			if tagName == name {
				return v.Field(i)
			}
		}

		// Check field name (case-insensitive)
		if strings.EqualFold(field.Name, name) {
			return v.Field(i)
		}
	}

	return reflect.Value{}
}

// setFieldValue sets a field to the parsed value.
func setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int64:
		// Handle time.Duration specially
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid duration %q: %w", value, err)
			}
			field.SetInt(int64(d))
		} else {
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer %q: %w", value, err)
			}
			field.SetInt(i)
		}
	case reflect.Bool:
		b := parseBool(value)
		field.SetBool(b)
	case reflect.Slice:
		// Handle string slices (comma-separated)
		if field.Type().Elem().Kind() == reflect.String {
			parts := strings.Split(value, ",")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			field.Set(reflect.ValueOf(parts))
		} else {
			return fmt.Errorf("unsupported slice type: %s", field.Type())
		}
	case reflect.Map:
		return fmt.Errorf("map fields must be set via config file, not CLI")
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
	return nil
}

// formatValue formats a reflect.Value as a string.
func formatValue(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}

	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int64:
		// Handle time.Duration specially
		if v.Type() == reflect.TypeOf(time.Duration(0)) {
			return time.Duration(v.Int()).String()
		}
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Slice:
		if v.Len() == 0 {
			return "[]"
		}
		var parts []string
		for i := 0; i < v.Len(); i++ {
			parts = append(parts, formatValue(v.Index(i)))
		}
		return strings.Join(parts, ", ")
	case reflect.Map:
		if v.Len() == 0 {
			return "{}"
		}
		var parts []string
		iter := v.MapRange()
		for iter.Next() {
			parts = append(parts, fmt.Sprintf("%v: %s", iter.Key().Interface(), formatValue(iter.Value())))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case reflect.Struct:
		return fmt.Sprintf("%+v", v.Interface())
	case reflect.Ptr:
		if v.IsNil() {
			return "<nil>"
		}
		return formatValue(v.Elem())
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}

// AllConfigPaths returns all known config paths.
func AllConfigPaths() []string {
	return []string{
		"version",
		"profile",
		"model",
		"fallback_model",
		"max_iterations",
		"timeout",
		"branch_prefix",
		"commit_prefix",
		"claude_path",
		"dangerously_skip_permissions",
		"templates_dir",
		"enable_checkpoints",
		"gates.default_type",
		"gates.auto_approve_on_success",
		"gates.retry_on_failure",
		"gates.max_retries",
		"gates.phase_overrides",
		"gates.weight_overrides",
		"retry.enabled",
		"retry.max_retries",
		"retry.retry_map",
		"worktree.enabled",
		"worktree.dir",
		"worktree.cleanup_on_complete",
		"worktree.cleanup_on_fail",
		"completion.action",
		"completion.target_branch",
		"completion.delete_branch",
		"completion.pr.title",
		"completion.pr.body_template",
		"completion.pr.labels",
		"completion.pr.reviewers",
		"completion.pr.draft",
		"completion.pr.team_reviewers",
		"completion.pr.assignees",
		"completion.pr.maintainer_can_modify",
		"completion.pr.auto_merge",
		"completion.pr.auto_approve",
		"completion.ci.wait_for_ci",
		"completion.ci.ci_timeout",
		"completion.ci.poll_interval",
		"completion.ci.merge_on_ci_pass",
		"completion.ci.merge_method",
		"completion.ci.merge_commit_template",
		"completion.ci.squash_commit_template",
		"completion.ci.verify_sha_on_merge",
		"completion.finalize.enabled",
		"completion.finalize.auto_trigger",
		"completion.finalize.sync.strategy",
		"completion.finalize.conflict_resolution.enabled",
		"completion.finalize.conflict_resolution.instructions",
		"completion.finalize.risk_assessment.enabled",
		"completion.finalize.risk_assessment.re_review_threshold",
		"completion.finalize.gates.pre_merge",
		"execution.use_session_execution",
		"execution.session_persistence",
		"execution.checkpoint_interval",
		"budget.threshold_usd",
		"budget.alert_on_exceed",
		"budget.pause_on_exceed",
		"pool.enabled",
		"pool.config_path",
		"server.host",
		"server.port",
		"server.auth.enabled",
		"server.auth.type",
		"team.name",
		"team.activity_logging",
		"team.task_claiming",
		"team.visibility",
		"team.mode",
		"team.server_url",
		"identity.initials",
		"identity.display_name",
		"identity.email",
		"task_id.mode",
		"task_id.prefix_source",
		"database.driver",
		"database.sqlite.path",
		"database.sqlite.global_path",
		"database.postgres.host",
		"database.postgres.port",
		"database.postgres.database",
		"database.postgres.user",
		"database.postgres.password",
		"database.postgres.ssl_mode",
		"database.postgres.pool_max",
	}
}
