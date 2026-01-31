package workflow

// Source indicates where a workflow or phase template came from.
type Source string

const (
	SourcePersonalGlobal Source = "personal_global" // ~/.orc/workflows/ or ~/.orc/phases/
	SourceProjectLocal   Source = "project_local"   // .orc/local/workflows/ or .orc/local/phases/
	SourceProject        Source = "project"         // .orc/workflows/ or .orc/phases/
	SourceEmbedded       Source = "embedded"        // Embedded in binary
	SourceDatabase       Source = "database"        // Legacy: loaded from database (for migration)
)

// SourcePriority returns the priority of a source (lower = higher priority).
func SourcePriority(s Source) int {
	switch s {
	case SourcePersonalGlobal:
		return 1
	case SourceProjectLocal:
		return 2
	case SourceProject:
		return 3
	case SourceEmbedded:
		return 5
	case SourceDatabase:
		return 6
	default:
		return 99
	}
}

// SourceDisplayName returns a human-readable name for the source.
func SourceDisplayName(s Source) string {
	switch s {
	case SourcePersonalGlobal:
		return "Personal (~/.orc/)"
	case SourceProjectLocal:
		return "Local (.orc/local/)"
	case SourceProject:
		return "Project (.orc/)"
	case SourceEmbedded:
		return "Embedded (built-in)"
	case SourceDatabase:
		return "Database (legacy)"
	default:
		return string(s)
	}
}

// IsEditable returns true if the source allows editing (file-based sources).
func (s Source) IsEditable() bool {
	switch s {
	case SourcePersonalGlobal, SourceProjectLocal, SourceProject:
		return true
	default:
		return false
	}
}
