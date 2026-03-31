package config

// ResolveExportConfig returns the effective export configuration,
// applying preset overrides if a preset is specified.
func (c *StorageConfig) ResolveExportConfig() ExportConfig {
	if c.Export.Preset == "" {
		return c.Export
	}

	result := c.Export
	switch c.Export.Preset {
	case ExportPresetMinimal:
		result.TaskDefinition = true
		result.FinalState = false
		result.Transcripts = false
		result.ContextSummary = false
	case ExportPresetStandard:
		result.TaskDefinition = true
		result.FinalState = true
		result.Transcripts = false
		result.ContextSummary = true
	case ExportPresetFull:
		result.TaskDefinition = true
		result.FinalState = true
		result.Transcripts = true
		result.ContextSummary = true
	}
	return result
}

// ShouldExport returns true if any export is enabled and the master toggle is on.
func (c *StorageConfig) ShouldExport() bool {
	if !c.Export.Enabled {
		return false
	}
	resolved := c.ResolveExportConfig()
	return resolved.TaskDefinition || resolved.FinalState ||
		resolved.Transcripts || resolved.ContextSummary
}
