package variable

import (
	"regexp"
)

// varPattern matches {{VARIABLE_NAME}} patterns.
// Same pattern as used in RenderTemplate for consistency.
var varPattern = regexp.MustCompile(`\{\{([A-Z_][A-Z0-9_]*)\}\}`)

// interpolateString replaces {{VAR}} patterns with values from vars.
// Missing variables are replaced with empty string.
func interpolateString(s string, vars VariableSet) string {
	if s == "" {
		return s
	}
	return varPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-2]
		if value, ok := vars[name]; ok {
			return value
		}
		return ""
	})
}

// Interpolate methods for each config type.
// These are called after parsing the config but before resolution.

// Interpolate replaces {{VAR}} patterns in ScriptConfig fields.
func (c *ScriptConfig) Interpolate(vars VariableSet) {
	c.Path = interpolateString(c.Path, vars)
	c.WorkDir = interpolateString(c.WorkDir, vars)
	for i, arg := range c.Args {
		c.Args[i] = interpolateString(arg, vars)
	}
}

// Interpolate replaces {{VAR}} patterns in APIConfig fields.
func (c *APIConfig) Interpolate(vars VariableSet) {
	c.URL = interpolateString(c.URL, vars)
	for k, v := range c.Headers {
		c.Headers[k] = interpolateString(v, vars)
	}
	// Note: JQFilter is NOT interpolated - it uses gjson syntax which conflicts with {{}}
}

// Interpolate replaces {{VAR}} patterns in EnvConfig fields.
func (c *EnvConfig) Interpolate(vars VariableSet) {
	c.Var = interpolateString(c.Var, vars)
	c.Default = interpolateString(c.Default, vars)
}

// Interpolate replaces {{VAR}} patterns in PhaseOutputConfig fields.
func (c *PhaseOutputConfig) Interpolate(vars VariableSet) {
	c.Phase = interpolateString(c.Phase, vars)
}

// Interpolate replaces {{VAR}} patterns in PromptFragmentConfig fields.
func (c *PromptFragmentConfig) Interpolate(vars VariableSet) {
	c.Path = interpolateString(c.Path, vars)
}

// Interpolate replaces {{VAR}} patterns in StaticConfig fields.
func (c *StaticConfig) Interpolate(vars VariableSet) {
	c.Value = interpolateString(c.Value, vars)
}
