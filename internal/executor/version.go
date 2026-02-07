package executor

import "runtime/debug"

// VersionInfo holds build and version metadata for the executor.
type VersionInfo struct {
	Version   string // Module version or "(devel)" for local builds
	GoVersion string // Go toolchain version
	VCSRev    string // VCS revision (git commit hash)
	VCSDirty  bool   // Whether the working tree had uncommitted changes
}

// GetVersionInfo returns build version information read from the Go runtime.
func GetVersionInfo() VersionInfo {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return VersionInfo{Version: "unknown"}
	}

	vi := VersionInfo{
		Version:   info.Main.Version,
		GoVersion: info.GoVersion,
	}

	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			vi.VCSRev = s.Value
		case "vcs.modified":
			vi.VCSDirty = s.Value == "true"
		}
	}

	return vi
}
